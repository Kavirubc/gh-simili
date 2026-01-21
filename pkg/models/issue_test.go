package models

import (
	"testing"
)

func TestIssueUUID(t *testing.T) {
	tests := []struct {
		org    string
		repo   string
		number int
	}{
		{"myorg", "myrepo", 123},
		{"other", "repo", 456},
		{"test", "test", 1},
	}

	for _, tt := range tests {
		t.Run(tt.org+"/"+tt.repo, func(t *testing.T) {
			// UUID should be deterministic
			uuid1 := IssueUUID(tt.org, tt.repo, tt.number)
			uuid2 := IssueUUID(tt.org, tt.repo, tt.number)

			if uuid1 != uuid2 {
				t.Errorf("IssueUUID not deterministic: %v != %v", uuid1, uuid2)
			}

			// UUID should be valid format
			if len(uuid1) != 36 {
				t.Errorf("IssueUUID invalid length: %d", len(uuid1))
			}
		})
	}
}

func TestIssue_FullRepo(t *testing.T) {
	issue := &Issue{
		Org:  "myorg",
		Repo: "myrepo",
	}

	if issue.FullRepo() != "myorg/myrepo" {
		t.Errorf("FullRepo() = %v, want myorg/myrepo", issue.FullRepo())
	}
}

func TestIssue_BodyHash(t *testing.T) {
	issue := &Issue{Body: "test body content"}

	hash1 := issue.BodyHash()
	hash2 := issue.BodyHash()

	// Hash should be deterministic
	if hash1 != hash2 {
		t.Errorf("BodyHash not deterministic")
	}

	// Hash should be 64 chars (SHA256 hex)
	if len(hash1) != 64 {
		t.Errorf("BodyHash invalid length: %d", len(hash1))
	}

	// Different body should produce different hash
	issue.Body = "different body"
	hash3 := issue.BodyHash()
	if hash1 == hash3 {
		t.Errorf("Different body produced same hash")
	}
}
