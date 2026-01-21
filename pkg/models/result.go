package models

// SearchResult represents a similar issue found via vector search
type SearchResult struct {
	Issue Issue   `json:"issue"`
	Score float64 `json:"score"` // Similarity score (0-1)
}

// IndexStats contains statistics from an indexing operation
type IndexStats struct {
	TotalIssues   int `json:"total_issues"`
	Indexed       int `json:"indexed"`
	Skipped       int `json:"skipped"`
	Errors        int `json:"errors"`
	DurationMs    int `json:"duration_ms"`
}

// ProcessResult contains the result of processing a single issue
type ProcessResult struct {
	IssueNumber     int            `json:"issue_number"`
	SimilarFound    []SearchResult `json:"similar_found,omitempty"`
	CommentPosted   bool           `json:"comment_posted"`
	Transferred     bool           `json:"transferred"`
	TransferTarget  string         `json:"transfer_target,omitempty"`
	Skipped         bool           `json:"skipped"`
	SkipReason      string         `json:"skip_reason,omitempty"`
	Error           string         `json:"error,omitempty"`
}
