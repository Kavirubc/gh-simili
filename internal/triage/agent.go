package triage

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/llm"
	"github.com/Kavirubc/gh-simili/internal/processor"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// Agent orchestrates issue triage operations
type Agent struct {
	cfg        *config.Config
	llm        llm.Provider
	classifier *Classifier
	quality    *QualityChecker
	duplicate  *DuplicateChecker
	similarity *processor.SimilarityFinder
}

// NewAgent creates a new triage agent
func NewAgent(cfg *config.Config, llmProvider llm.Provider, similarity *processor.SimilarityFinder) *Agent {
	return &Agent{
		cfg:        cfg,
		llm:        llmProvider,
		classifier: NewClassifier(llmProvider, &cfg.Triage.Classifier),
		quality:    NewQualityChecker(llmProvider, &cfg.Triage.Quality),
		duplicate:  NewDuplicateChecker(&cfg.Triage.Duplicate),
		similarity: similarity,
	}
}

// NewAgentWithGitHub creates a new triage agent with GitHub client for delayed actions
func NewAgentWithGitHub(cfg *config.Config, llmProvider llm.Provider, similarity *processor.SimilarityFinder, gh *github.Client) *Agent {
	return &Agent{
		cfg:        cfg,
		llm:        llmProvider,
		classifier: NewClassifier(llmProvider, &cfg.Triage.Classifier),
		quality:    NewQualityChecker(llmProvider, &cfg.Triage.Quality),
		duplicate:  NewDuplicateCheckerWithDelayedActions(&cfg.Triage.Duplicate, gh, cfg),
		similarity: similarity,
	}
}

// Triage performs full triage analysis on an issue
func (a *Agent) Triage(ctx context.Context, issue *models.Issue) (*Result, error) {
	result := &Result{
		Actions: []Action{},
	}

	// Step 1: Find similar issues
	similarIssues, err := a.similarity.FindSimilar(ctx, issue, true)
	if err != nil {
		log.Printf("Warning: failed to find similar issues: %v", err)
	}

	// Step 2: Check for duplicates
	if a.cfg.Triage.Duplicate.Enabled && len(similarIssues) > 0 {
		dupResult := a.duplicate.Check(similarIssues)
		result.Duplicate = dupResult

		if dupResult.IsDuplicate {
			result.Actions = append(result.Actions, a.duplicate.GetActions(dupResult)...)
			// If it's a high-confidence duplicate, skip other analysis
			if dupResult.ShouldClose {
				return result, nil
			}
		}
	}

	// Step 3: Classify labels
	if a.cfg.Triage.Classifier.Enabled {
		labels, err := a.classifier.Classify(ctx, issue)
		if err != nil {
			log.Printf("Warning: label classification failed: %v", err)
		} else {
			result.Labels = labels
			result.Actions = append(result.Actions, a.labelsToActions(labels)...)
		}
	}

	// Step 4: Check quality
	if a.cfg.Triage.Quality.Enabled {
		qualityResult, err := a.quality.Check(ctx, issue)
		if err != nil {
			log.Printf("Warning: quality check failed: %v", err)
		} else {
			result.Quality = qualityResult
			if a.quality.NeedsInfo(qualityResult) {
				result.Actions = append(result.Actions, a.qualityToActions(qualityResult)...)
			}
		}
	}

	// Step 5: Build and add triage summary comment
	summaryComment := a.buildSummaryComment(result, similarIssues, issue)
	result.Actions = append(result.Actions, Action{
		Type:    ActionComment,
		Comment: summaryComment,
		Reason:  "triage summary",
	})

	return result, nil
}

// labelsToActions converts label results to actions
func (a *Agent) labelsToActions(labels []LabelResult) []Action {
	var actions []Action
	for _, l := range labels {
		actions = append(actions, Action{
			Type:   ActionAddLabel,
			Label:  l.Label,
			Reason: l.Reason,
		})
	}
	return actions
}

// qualityToActions converts quality result to actions
func (a *Agent) qualityToActions(qr *QualityResult) []Action {
	var actions []Action

	// Add needs-info label
	actions = append(actions, Action{
		Type:   ActionAddLabel,
		Label:  a.quality.GetNeedsInfoLabel(),
		Reason: "issue needs more information",
	})

	// Add feedback comment if available
	if qr.Feedback != "" {
		actions = append(actions, Action{
			Type:    ActionComment,
			Comment: qr.Feedback,
			Reason:  "request additional information",
		})
	}

	return actions
}

// buildSummaryComment creates a summary of triage actions
func (a *Agent) buildSummaryComment(result *Result, similarIssues []vectordb.SearchResult, issue *models.Issue) string {
	var sections []string

	// Header
	sections = append(sections, "## 🤖 Triage Summary\n")

	// Labels section
	if len(result.Labels) > 0 {
		var labelLines []string
		labelLines = append(labelLines, "### Labels Applied")
		for _, l := range result.Labels {
			labelLines = append(labelLines, fmt.Sprintf("- `%s` (%.0f%% confidence) - %s", l.Label, l.Confidence*100, l.Reason))
		}
		sections = append(sections, strings.Join(labelLines, "\n"))
	} else {
		sections = append(sections, "### Labels\nNo labels applied (no confident matches found)")
	}

	// Quality section
	if result.Quality != nil {
		qualityLine := fmt.Sprintf("### Quality Score: %.0f%%", result.Quality.Score*100)
		if len(result.Quality.Missing) > 0 {
			qualityLine += fmt.Sprintf("\n⚠️ Missing: %s", strings.Join(result.Quality.Missing, ", "))
		} else {
			qualityLine += "\n✅ Issue is well-documented"
		}
		sections = append(sections, qualityLine)
	}

	// Similar issues section
	if len(similarIssues) > 0 {
		crossRepo := processor.HasCrossRepoResults(similarIssues, issue.Org, issue.Repo)
		similarComment := processor.FormatSimilarityComment(similarIssues, crossRepo)
		if similarComment != "" {
			sections = append(sections, "### Similar Issues\n"+similarComment)
		}
	} else {
		sections = append(sections, "### Similar Issues\nNo similar issues found")
	}

	// Duplicate section
	if result.Duplicate != nil && result.Duplicate.IsDuplicate {
		dupLine := fmt.Sprintf("### ⚠️ Potential Duplicate\nSimilarity: %.0f%%", result.Duplicate.Similarity*100)
		if result.Duplicate.Original != nil {
			dupLine += fmt.Sprintf("\nOriginal: #%d - %s", result.Duplicate.Original.Number, result.Duplicate.Original.Title)
		}
		sections = append(sections, dupLine)
	}

	// Footer
	sections = append(sections, "\n---\n<sub>🤖 Powered by [Simili Triage](https://github.com/Kavirubc/gh-simili)</sub>")

	return strings.Join(sections, "\n\n")
}

// TriageWithSimilar performs triage with pre-fetched similar issues
func (a *Agent) TriageWithSimilar(ctx context.Context, issue *models.Issue, similarIssues []vectordb.SearchResult) (*Result, error) {
	result := &Result{
		Actions: []Action{},
	}

	// Check for duplicates
	if a.cfg.Triage.Duplicate.Enabled && len(similarIssues) > 0 {
		dupResult := a.duplicate.Check(similarIssues)
		result.Duplicate = dupResult

		if dupResult.IsDuplicate {
			result.Actions = append(result.Actions, a.duplicate.GetActions(dupResult)...)
			if dupResult.ShouldClose {
				return result, nil
			}
		}
	}

	// Classify labels
	if a.cfg.Triage.Classifier.Enabled {
		labels, err := a.classifier.Classify(ctx, issue)
		if err != nil {
			log.Printf("Warning: label classification failed: %v", err)
		} else {
			result.Labels = labels
			result.Actions = append(result.Actions, a.labelsToActions(labels)...)
		}
	}

	// Check quality
	if a.cfg.Triage.Quality.Enabled {
		qualityResult, err := a.quality.Check(ctx, issue)
		if err != nil {
			log.Printf("Warning: quality check failed: %v", err)
		} else {
			result.Quality = qualityResult
			if a.quality.NeedsInfo(qualityResult) {
				result.Actions = append(result.Actions, a.qualityToActions(qualityResult)...)
			}
		}
	}

	return result, nil
}
