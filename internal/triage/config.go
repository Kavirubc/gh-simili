package triage

import (
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// Result contains the complete triage analysis
type Result struct {
	Labels      []LabelResult    `json:"labels,omitempty"`
	Quality     *QualityResult   `json:"quality,omitempty"`
	Duplicate   *DuplicateResult `json:"duplicate,omitempty"`
	Actions     []Action         `json:"actions"`
	Error       string           `json:"error,omitempty"`
}

// LabelResult contains classification result for a single label
type LabelResult struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

// QualityResult contains issue quality assessment
type QualityResult struct {
	Score    float64  `json:"score"`
	Missing  []string `json:"missing,omitempty"`
	Feedback string   `json:"feedback,omitempty"`
}

// DuplicateResult contains duplicate detection result
type DuplicateResult struct {
	IsDuplicate bool           `json:"is_duplicate"`
	Similarity  float64        `json:"similarity"`
	Original    *models.Issue  `json:"original,omitempty"`
	ShouldClose bool           `json:"should_close"`
}

// Action represents an action to take on the issue
type Action struct {
	Type    ActionType `json:"type"`
	Label   string     `json:"label,omitempty"`
	Comment string     `json:"comment,omitempty"`
	Reason  string     `json:"reason,omitempty"`
}

// ActionType represents the type of action
type ActionType string

const (
	ActionAddLabel    ActionType = "add_label"
	ActionRemoveLabel ActionType = "remove_label"
	ActionComment     ActionType = "comment"
	ActionClose       ActionType = "close"
)

// IssueContext contains all information about an issue for triage
type IssueContext struct {
	Issue        *models.Issue   `json:"issue"`
	SimilarIssues []models.Issue `json:"similar_issues,omitempty"`
}
