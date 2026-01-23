package triage

import (
	"context"
	"log"

	"github.com/Kavirubc/gh-simili/internal/config"
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

	// Step 5: Add similarity comment if relevant matches found
	if len(similarIssues) > 0 && !isDuplicateClose(result.Actions) {
		crossRepo := processor.HasCrossRepoResults(similarIssues, issue.Org, issue.Repo)
		comment := processor.FormatSimilarityComment(similarIssues, crossRepo)
		if comment != "" {
			result.Actions = append(result.Actions, Action{
				Type:    ActionComment,
				Comment: comment,
				Reason:  "found similar issues",
			})
		}
	}

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

// isDuplicateClose checks if actions include closing as duplicate
func isDuplicateClose(actions []Action) bool {
	for _, a := range actions {
		if a.Type == ActionClose {
			return true
		}
	}
	return false
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
