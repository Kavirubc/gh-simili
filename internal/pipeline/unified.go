package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/embedding"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/llm"
	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/processor"
	"github.com/Kavirubc/gh-simili/internal/transfer"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// UnifiedProcessor handles the complete issue processing pipeline:
// 1. Similarity search
// 2. Triage (labels, quality, duplicates)
// 3. Transfer (with delayed actions support)
// 4. Indexing to vector DB
type UnifiedProcessor struct {
	cfg            *config.Config
	gh             *github.Client
	transferClient *github.Client
	embedder       *embedding.FallbackProvider
	vdb            *vectordb.Client
	similarity     *processor.SimilarityFinder
	indexer        *processor.Indexer
	triageAgent    *triage.Agent
	llmProvider    llm.Provider
	dryRun         bool
	execute        bool
}

// UnifiedResult contains the complete result of unified processing
type UnifiedResult struct {
	IssueNumber     int                     `json:"issue_number"`
	Skipped         bool                    `json:"skipped,omitempty"`
	SkipReason      string                  `json:"skip_reason,omitempty"`
	SimilarFound    []vectordb.SearchResult `json:"similar_found,omitempty"`
	TriageResult    *triage.Result          `json:"triage_result,omitempty"`
	Transferred     bool                    `json:"transferred,omitempty"`
	TransferTarget  string                  `json:"transfer_target,omitempty"`
	CommentPosted   bool                    `json:"comment_posted,omitempty"`
	Indexed         bool                    `json:"indexed,omitempty"`
	ActionsExecuted int                     `json:"actions_executed,omitempty"`
	PendingAction   *pending.PendingAction  `json:"pending_action,omitempty"`
}

// NewUnifiedProcessor creates a new unified processor
func NewUnifiedProcessor(cfg *config.Config, dryRun bool, execute bool) (*UnifiedProcessor, error) {
	return NewUnifiedProcessorWithTransferToken(cfg, dryRun, execute, "")
}

// NewUnifiedProcessorWithTransferToken creates a unified processor with separate transfer token
func NewUnifiedProcessorWithTransferToken(cfg *config.Config, dryRun bool, execute bool, transferToken string) (*UnifiedProcessor, error) {
	gh, err := github.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Create transfer client with separate token if provided
	var transferClient *github.Client
	if transferToken != "" {
		transferClient, err = github.NewClientWithToken(transferToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create transfer client: %w", err)
		}
	} else {
		transferClient = gh
	}

	embedder, err := embedding.NewFallbackProvider(&cfg.Embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding provider: %w", err)
	}

	vdb, err := vectordb.NewClient(&cfg.Qdrant)
	if err != nil {
		embedder.Close()
		return nil, fmt.Errorf("failed to create vector DB client: %w", err)
	}

	indexer, err := processor.NewIndexer(cfg, dryRun)
	if err != nil {
		embedder.Close()
		vdb.Close()
		return nil, fmt.Errorf("failed to create indexer: %w", err)
	}

	similarity := processor.NewSimilarityFinder(cfg, embedder, vdb)

	// Create LLM provider for triage (optional - only if triage is enabled)
	var llmProvider llm.Provider
	var triageAgent *triage.Agent
	if cfg.Triage.Enabled {
		llmProvider, err = createLLMProvider(&cfg.Triage.LLM)
		if err != nil {
			log.Printf("Warning: failed to create LLM provider for triage: %v", err)
		} else {
			triageAgent = triage.NewAgentWithGitHub(cfg, llmProvider, similarity, gh)
		}
	}

	return &UnifiedProcessor{
		cfg:            cfg,
		gh:             gh,
		transferClient: transferClient,
		embedder:       embedder,
		vdb:            vdb,
		similarity:     similarity,
		indexer:        indexer,
		triageAgent:    triageAgent,
		llmProvider:    llmProvider,
		dryRun:         dryRun,
		execute:        execute,
	}, nil
}

// createLLMProvider creates an LLM provider based on config
func createLLMProvider(cfg *config.LLMConfig) (llm.Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("LLM API key not configured")
	}
	switch cfg.Provider {
	case "gemini":
		return llm.NewGeminiProvider(cfg.APIKey, cfg.Model)
	case "openai":
		return llm.NewOpenAIProvider(cfg.APIKey, cfg.Model)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}

// Close releases all resources
func (up *UnifiedProcessor) Close() error {
	var errs []error

	if up.llmProvider != nil {
		up.llmProvider.Close()
	}
	if up.indexer != nil {
		if err := up.indexer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if up.embedder != nil {
		up.embedder.Close()
	}
	if up.vdb != nil {
		if err := up.vdb.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing resources: %v", errs)
	}
	return nil
}

// ProcessEvent processes a GitHub Action event through the unified pipeline
func (up *UnifiedProcessor) ProcessEvent(ctx context.Context, eventPath string) (*UnifiedResult, error) {
	event, err := github.ParseEventFile(eventPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}

	if !event.IsIssueEvent() {
		return &UnifiedResult{
			Skipped:    true,
			SkipReason: "not an issue event",
		}, nil
	}

	issue := event.ToIssue()
	if issue == nil {
		return nil, fmt.Errorf("failed to parse issue from event")
	}

	// Only process opened events in unified flow
	if !event.IsOpenedEvent() {
		return &UnifiedResult{
			IssueNumber: issue.Number,
			Skipped:     true,
			SkipReason:  fmt.Sprintf("action '%s' not supported in unified flow (use 'process' for edits/closes)", event.Action),
		}, nil
	}

	return up.ProcessIssue(ctx, issue)
}

// ProcessIssue processes a single issue through the unified pipeline
func (up *UnifiedProcessor) ProcessIssue(ctx context.Context, issue *models.Issue) (*UnifiedResult, error) {
	result := &UnifiedResult{IssueNumber: issue.Number}

	// Step 1: Check if repo is enabled
	repoConfig := up.cfg.GetRepoConfig(issue.Org, issue.Repo)
	if repoConfig == nil || !repoConfig.Enabled {
		result.Skipped = true
		result.SkipReason = "repository not enabled"
		return result, nil
	}

	// Step 2: Check cooldown
	skip, err := up.gh.ShouldSkipComment(ctx, issue.Org, issue.Repo, issue.Number, up.cfg.Defaults.CommentCooldownHours)
	if err != nil {
		return nil, fmt.Errorf("failed to check cooldown: %w", err)
	}
	if skip {
		result.Skipped = true
		result.SkipReason = "cooldown active"
		return result, nil
	}

	// Step 3: Ensure collection exists
	collection := vectordb.CollectionName(issue.Org)
	if !up.dryRun {
		if err := up.vdb.EnsureCollection(ctx, collection); err != nil {
			return nil, fmt.Errorf("failed to ensure collection: %w", err)
		}
	}

	// Step 4: Find similar issues
	similarIssues, err := up.similarity.FindSimilar(ctx, issue, true)
	if err != nil {
		log.Printf("Warning: similarity search failed: %v", err)
	}
	if len(similarIssues) > 0 {
		result.SimilarFound = similarIssues
	}

	// Step 5: Check transfer rules FIRST
	var transferTarget string
	var transferRule *config.TransferRule
	var skipDuplicateCheck bool
	if len(repoConfig.TransferRules) > 0 {
		matcher := transfer.NewRuleMatcher(repoConfig.TransferRules)
		transferTarget, transferRule = matcher.Match(issue)
	}

	// Step 6: If transfer matched, store it but continue processing
	if transferTarget != "" {
		log.Printf("Transfer rule matched: %s -> %s", issue.Repo, transferTarget)
		result.TransferTarget = transferTarget
		skipDuplicateCheck = true // Skip duplicate detection for transfers

		// Prepare pending action if delayed actions enabled
		if up.cfg.Defaults.DelayedActions.Enabled {
			delayHours := up.cfg.Defaults.DelayedActions.DelayHours
			expiresAt := time.Now().Add(time.Duration(delayHours) * time.Hour)
			result.PendingAction = &pending.PendingAction{
				Type:        pending.ActionTypeTransfer,
				Org:         issue.Org,
				Repo:        issue.Repo,
				IssueNumber: issue.Number,
				Target:      transferTarget,
				ScheduledAt: time.Now(),
				ExpiresAt:   expiresAt,
			}
		}
	}

	// Step 7: Run triage analysis (labels, quality, duplicates)
	if up.triageAgent != nil {
		// Skip duplicate check if transfer matched
		var triageResult *triage.Result
		var err error
		if skipDuplicateCheck {
			triageResult, err = up.triageAgent.TriageWithoutDuplicates(ctx, issue, similarIssues)
		} else {
			triageResult, err = up.triageAgent.TriageWithSimilar(ctx, issue, similarIssues)
		}
		if err != nil {
			log.Printf("Warning: triage failed: %v", err)
		} else {
			result.TriageResult = triageResult

			// Prepare pending close if duplicate and confirmed
			if triageResult.Duplicate != nil && triageResult.Duplicate.IsDuplicate && triageResult.Duplicate.ShouldClose &&
				up.cfg.Defaults.DelayedActions.Enabled && result.PendingAction == nil {
				delayHours := up.cfg.Defaults.DelayedActions.DelayHours
				expiresAt := time.Now().Add(time.Duration(delayHours) * time.Hour)
				result.PendingAction = &pending.PendingAction{
					Type:        pending.ActionTypeClose,
					Org:         issue.Org,
					Repo:        issue.Repo,
					IssueNumber: issue.Number,
					Target:      triageResult.Duplicate.Original.URL,
					ScheduledAt: time.Now(),
					ExpiresAt:   expiresAt,
				}
			}
		}
	}

	// Step 8: Build and post unified comment
	comment := up.buildUnifiedComment(result, similarIssues, issue)
	var commentID int
	if comment != "" && up.execute && !up.dryRun {
		id, err := up.gh.PostCommentWithID(ctx, issue.Org, issue.Repo, issue.Number, comment)
		if err != nil {
			log.Printf("Warning: failed to post unified comment: %v", err)
		} else {
			result.CommentPosted = true
			commentID = id
		}
	}

	// Step 8.5: Execute transfer if matched (after posting unified comment)
	if result.TransferTarget != "" && up.execute && !up.dryRun {
		executor := transfer.NewExecutor(up.transferClient, up.gh, up.vdb, up.cfg, up.dryRun)
		// Use silent scheduling if unified comment was posted
		if result.CommentPosted {
			if err := executor.ScheduleTransferSilent(ctx, issue, result.TransferTarget, commentID); err != nil {
				log.Printf("Warning: failed to schedule transfer: %v", err)
			} else {
				result.Transferred = true
			}
		} else {
			if err := executor.Transfer(ctx, issue, result.TransferTarget, transferRule); err != nil {
				log.Printf("Warning: failed to schedule transfer: %v", err)
			} else {
				result.Transferred = true
			}
		}
	}

	// Step 9: Execute triage actions (labels, close)
	if result.TriageResult != nil && up.execute && !up.dryRun {
		// Filter out comment actions (we already posted unified comment)
		actionsToExecute := filterNonCommentActions(result.TriageResult.Actions)

		// Create executor with delayed action support if enabled
		var executor *triage.Executor
		if up.cfg.Defaults.DelayedActions.Enabled {
			duplicateChecker := triage.NewDuplicateCheckerWithDelayedActions(&up.cfg.Triage.Duplicate, up.gh, up.cfg)

			// If it's a duplicate that should be closed, use silent scheduling if unified comment was posted
			if result.TriageResult.Duplicate != nil && result.TriageResult.Duplicate.IsDuplicate &&
				result.TriageResult.Duplicate.ShouldClose && result.CommentPosted {

				if err := duplicateChecker.ScheduleCloseSilent(ctx, issue, result.TriageResult.Duplicate.Original.URL, commentID); err != nil {
					log.Printf("Warning: failed to schedule close: %v", err)
				}
				// Remove close action from list as we handled it silently
				actionsToExecute = filterCloseActions(actionsToExecute)
			}

			executor = triage.NewExecutorWithDelayedActions(up.gh, up.cfg, duplicateChecker, up.dryRun)
		} else {
			executor = triage.NewExecutor(up.gh, up.dryRun)
		}

		// Create a filtered result for execution
		filteredResult := &triage.Result{
			Labels:    result.TriageResult.Labels,
			Quality:   result.TriageResult.Quality,
			Duplicate: result.TriageResult.Duplicate,
			Actions:   actionsToExecute,
		}

		if err := executor.Execute(ctx, issue, filteredResult); err != nil {
			log.Printf("Warning: failed to execute triage actions: %v", err)
		} else {
			result.ActionsExecuted = len(actionsToExecute)
		}
	}

	// Step 10: Index the issue (skip if duplicate should be closed OR transferred)
	shouldIndex := true
	if result.TriageResult != nil && result.TriageResult.Duplicate != nil && result.TriageResult.Duplicate.ShouldClose {
		shouldIndex = false
		log.Printf("Skipping indexing: issue will be closed as duplicate")
	}
	if result.TransferTarget != "" {
		shouldIndex = false
		log.Printf("Skipping indexing: issue will be transferred")
	}

	if shouldIndex && !up.dryRun {
		if err := up.indexer.IndexSingleIssue(ctx, issue); err != nil {
			log.Printf("Warning: failed to index issue: %v", err)
		} else {
			result.Indexed = true
		}
	}

	return result, nil
}

// filterNonCommentActions removes comment actions from the list
func filterNonCommentActions(actions []triage.Action) []triage.Action {
	filtered := make([]triage.Action, 0, len(actions))
	for _, a := range actions {
		if a.Type != triage.ActionComment {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// filterCloseActions removes close actions from the list
func filterCloseActions(actions []triage.Action) []triage.Action {
	filtered := make([]triage.Action, 0, len(actions))
	for _, a := range actions {
		if a.Type != triage.ActionClose {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// buildUnifiedComment creates a single comment combining similarity and triage results
func (up *UnifiedProcessor) buildUnifiedComment(result *UnifiedResult, similarIssues []vectordb.SearchResult, issue *models.Issue) string {
	if len(similarIssues) == 0 && result.TriageResult == nil && result.TransferTarget == "" {
		return ""
	}

	var sections []string

	// Header
	sections = append(sections, "## 🤖 Issue Intelligence Summary\n")
	sections = append(sections, "Thanks for opening this issue! Here's what I found:\n")

	// Similar issues section
	if len(similarIssues) > 0 {
		crossRepo := processor.HasCrossRepoResults(similarIssues, issue.Org, issue.Repo)
		sections = append(sections, up.formatSimilarIssuesSection(similarIssues, crossRepo))
	}

	// Triage results
	if result.TriageResult != nil {
		// Labels section
		if len(result.TriageResult.Labels) > 0 {
			var labelLines []string
			labelLines = append(labelLines, "### 🏷️ Suggested Labels")
			for _, l := range result.TriageResult.Labels {
				labelLines = append(labelLines, fmt.Sprintf("- `%s` (%.0f%% confidence) - %s", l.Label, l.Confidence*100, l.Reason))
			}
			sections = append(sections, strings.Join(labelLines, "\n"))
		}

		// Quality section
		if result.TriageResult.Quality != nil {
			qualityLine := fmt.Sprintf("### 📊 Quality Score: %.0f%%", result.TriageResult.Quality.Score*100)
			if len(result.TriageResult.Quality.Missing) > 0 {
				qualityLine += fmt.Sprintf("\n⚠️ Missing: %s", strings.Join(result.TriageResult.Quality.Missing, ", "))
			} else {
				qualityLine += "\n✅ Issue is well-documented"
			}
			sections = append(sections, qualityLine)
		}

		// Duplicate section
		if result.TriageResult.Duplicate != nil && result.TriageResult.Duplicate.IsDuplicate {
			dupLine := fmt.Sprintf("### ⚠️ Potential Duplicate\nSimilarity: %.0f%%", result.TriageResult.Duplicate.Similarity*100)
			if result.TriageResult.Duplicate.Original != nil {
				dupLine += fmt.Sprintf("\nOriginal: [#%d - %s](%s)",
					result.TriageResult.Duplicate.Original.Number,
					truncateString(result.TriageResult.Duplicate.Original.Title, 50),
					result.TriageResult.Duplicate.Original.URL)
			}
			sections = append(sections, dupLine)
		}
	}

	// Transfer section
	if result.TransferTarget != "" {
		sections = append(sections, up.formatTransferSection(result.TransferTarget, result.PendingAction))
	}

	// Footer
	footer := "\n---\n<sub>🤖 Powered by [Simili](https://github.com/Kavirubc/gh-simili)</sub>"
	if result.PendingAction != nil {
		metadata, err := pending.FormatPendingActionMetadata(result.PendingAction)
		if err == nil {
			footer = "\n\n" + metadata + footer
		}
	}
	sections = append(sections, footer)

	return strings.Join(sections, "\n\n")
}

// formatSimilarIssuesSection formats the similar issues as a table
func (up *UnifiedProcessor) formatSimilarIssuesSection(results []vectordb.SearchResult, crossRepo bool) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### 🔍 Related Issues\n\n")

	if crossRepo {
		sb.WriteString("| Issue | Repository | Similarity | Status |\n")
		sb.WriteString("|-------|------------|------------|--------|\n")
	} else {
		sb.WriteString("| Issue | Similarity | Status |\n")
		sb.WriteString("|-------|------------|--------|\n")
	}

	for _, r := range results {
		status := "🟢 Open"
		if r.Issue.State == "closed" {
			status = "🔴 Closed"
		}

		title := truncateString(r.Issue.Title, 50)
		link := fmt.Sprintf("[#%d - %s](%s)", r.Issue.Number, title, r.Issue.URL)
		similarity := fmt.Sprintf("%.0f%%", r.Score*100)

		if crossRepo {
			repo := fmt.Sprintf("%s/%s", r.Issue.Org, r.Issue.Repo)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", link, repo, similarity, status))
		} else {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", link, similarity, status))
		}
	}

	sb.WriteString("\nIf any of these address your problem, please let us know!")

	return sb.String()
}

// formatTransferSection formats the transfer suggestion
func (up *UnifiedProcessor) formatTransferSection(target string, action *pending.PendingAction) string {
	var sb strings.Builder
	sb.WriteString("### 🔄 Transfer Suggestion\n\n")
	sb.WriteString(fmt.Sprintf("This issue appears to belong in **%s**.\n\n", target))

	if up.cfg.Defaults.DelayedActions.Enabled && action != nil {
		deadline := action.ExpiresAt.Format("2006-01-02 15:04 MST")
		delayHours := up.cfg.Defaults.DelayedActions.DelayHours
		sb.WriteString(fmt.Sprintf("**This issue will be transferred in %d hours.**\n\n", delayHours))
		sb.WriteString("**React to this comment:**\n")
		sb.WriteString(fmt.Sprintf("- 👍 (%s) to approve and proceed with transfer\n", up.cfg.Defaults.DelayedActions.ApproveReaction))
		sb.WriteString(fmt.Sprintf("- 👎 (%s) to cancel this transfer\n\n", up.cfg.Defaults.DelayedActions.CancelReaction))
		sb.WriteString(fmt.Sprintf("**Deadline**: %s\n\n", deadline))
		sb.WriteString("If no reaction is provided, the transfer will proceed automatically.")
	} else {
		sb.WriteString("Transfer will be executed immediately.")
	}

	return sb.String()
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// PrintUnifiedResult outputs the processing result to stdout
func PrintUnifiedResult(result *UnifiedResult) {
	fmt.Println("\n=== Unified Processing Result ===")
	fmt.Printf("Issue: #%d\n", result.IssueNumber)

	if result.Skipped {
		fmt.Printf("Skipped: %s\n", result.SkipReason)
		return
	}

	if len(result.SimilarFound) > 0 {
		fmt.Printf("Similar Issues Found: %d\n", len(result.SimilarFound))
	}

	if result.TransferTarget != "" {
		status := "scheduled"
		if result.Transferred {
			status = "executed"
		}
		fmt.Printf("Transfer to %s: %s\n", result.TransferTarget, status)
	}

	if result.TriageResult != nil {
		if len(result.TriageResult.Labels) > 0 {
			fmt.Println("Labels:")
			for _, l := range result.TriageResult.Labels {
				fmt.Printf("  - %s (%.0f%%)\n", l.Label, l.Confidence*100)
			}
		}
		if result.TriageResult.Quality != nil {
			fmt.Printf("Quality Score: %.0f%%\n", result.TriageResult.Quality.Score*100)
		}
		if result.TriageResult.Duplicate != nil && result.TriageResult.Duplicate.IsDuplicate {
			fmt.Printf("Duplicate: %.0f%% similar to #%d\n",
				result.TriageResult.Duplicate.Similarity*100,
				result.TriageResult.Duplicate.Original.Number)
		}
	}

	if result.CommentPosted {
		fmt.Println("Comment: posted")
	}

	if result.Indexed {
		fmt.Println("Index: updated")
	}

	if result.ActionsExecuted > 0 {
		fmt.Printf("Actions Executed: %d\n", result.ActionsExecuted)
	}
}
