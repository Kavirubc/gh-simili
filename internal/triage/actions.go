package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// Executor executes triage actions
type Executor struct {
	client *github.Client
	dryRun bool
}

// NewExecutor creates a new action executor
func NewExecutor(client *github.Client, dryRun bool) *Executor {
	return &Executor{
		client: client,
		dryRun: dryRun,
	}
}

// Execute performs all actions in a triage result
func (e *Executor) Execute(ctx context.Context, issue *models.Issue, result *Result) error {
	for _, action := range result.Actions {
		if err := e.executeAction(ctx, issue, action); err != nil {
			log.Printf("Error executing action %s: %v", action.Type, err)
			// Continue with other actions
		}
	}
	return nil
}

// executeAction performs a single action
func (e *Executor) executeAction(ctx context.Context, issue *models.Issue, action Action) error {
	log.Printf("Executing action: %s (reason: %s)", action.Type, action.Reason)

	if e.dryRun {
		log.Printf("[DRY RUN] Would execute: %s", action.Type)
		return nil
	}

	switch action.Type {
	case ActionAddLabel:
		return e.client.AddLabels(ctx, issue.Org, issue.Repo, issue.Number, []string{action.Label})

	case ActionRemoveLabel:
		return e.client.RemoveLabel(ctx, issue.Org, issue.Repo, issue.Number, action.Label)

	case ActionComment:
		return e.client.PostComment(ctx, issue.Org, issue.Repo, issue.Number, action.Comment)

	case ActionClose:
		return e.client.CloseIssue(ctx, issue.Org, issue.Repo, issue.Number, "not_planned")

	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// ExecuteSelective executes only specific action types
func (e *Executor) ExecuteSelective(ctx context.Context, issue *models.Issue, result *Result, allowedTypes []ActionType) error {
	allowed := make(map[ActionType]bool)
	for _, t := range allowedTypes {
		allowed[t] = true
	}

	for _, action := range result.Actions {
		if !allowed[action.Type] {
			continue
		}
		if err := e.executeAction(ctx, issue, action); err != nil {
			log.Printf("Error executing action %s: %v", action.Type, err)
		}
	}
	return nil
}

// WriteOutput writes the triage result to a file
func WriteOutput(result *Result, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// ReadOutput reads a triage result from a file
func ReadOutput(path string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read output: %w", err)
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &result, nil
}

// FilterActions filters result actions by type
func FilterActions(result *Result, types ...ActionType) []Action {
	typeSet := make(map[ActionType]bool)
	for _, t := range types {
		typeSet[t] = true
	}

	var filtered []Action
	for _, a := range result.Actions {
		if typeSet[a.Type] {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// HasAction checks if result contains a specific action type
func HasAction(result *Result, actionType ActionType) bool {
	for _, a := range result.Actions {
		if a.Type == actionType {
			return true
		}
	}
	return false
}
