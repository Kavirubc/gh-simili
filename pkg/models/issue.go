package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Issue represents a GitHub issue with its metadata
type Issue struct {
	Org       string    `json:"org"`
	Repo      string    `json:"repo"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"` // "open" or "closed"
	Labels    []string  `json:"labels"`
	Author    string    `json:"author"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FullRepo returns the full repository name (org/repo)
func (i *Issue) FullRepo() string {
	return fmt.Sprintf("%s/%s", i.Org, i.Repo)
}

// UUID generates a deterministic UUID based on org/repo#number
func (i *Issue) UUID() string {
	return IssueUUID(i.Org, i.Repo, i.Number)
}

// BodyHash returns a SHA256 hash of the body for change detection
func (i *Issue) BodyHash() string {
	h := sha256.Sum256([]byte(i.Body))
	return hex.EncodeToString(h[:])
}

// IssueUUID generates a deterministic UUID from issue identity
func IssueUUID(org, repo string, number int) string {
	data := fmt.Sprintf("%s/%s#%d", org, repo, number)
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(data)).String()
}
