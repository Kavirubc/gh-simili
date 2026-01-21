package transfer

import (
	"testing"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
)

func TestRuleMatcher_Match_Labels(t *testing.T) {
	rules := []config.TransferRule{
		{
			Match:    config.MatchCondition{Labels: []string{"backend", "api"}},
			Target:   "org/backend-service",
			Priority: 1,
		},
	}

	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name      string
		issue     *models.Issue
		wantMatch bool
	}{
		{
			name:      "matches label",
			issue:     &models.Issue{Labels: []string{"backend"}},
			wantMatch: true,
		},
		{
			name:      "matches one of multiple labels",
			issue:     &models.Issue{Labels: []string{"bug", "api"}},
			wantMatch: true,
		},
		{
			name:      "no matching labels",
			issue:     &models.Issue{Labels: []string{"frontend", "bug"}},
			wantMatch: false,
		},
		{
			name:      "empty labels",
			issue:     &models.Issue{Labels: []string{}},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, _ := matcher.Match(tt.issue)
			gotMatch := target != ""
			if gotMatch != tt.wantMatch {
				t.Errorf("Match() = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestRuleMatcher_Match_TitleContains(t *testing.T) {
	rules := []config.TransferRule{
		{
			Match:    config.MatchCondition{TitleContains: []string{"frontend", "UI"}},
			Target:   "org/web-app",
			Priority: 1,
		},
	}

	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name      string
		title     string
		wantMatch bool
	}{
		{
			name:      "matches title keyword",
			title:     "Fix UI button alignment",
			wantMatch: true,
		},
		{
			name:      "case insensitive match",
			title:     "FRONTEND issue with forms",
			wantMatch: true,
		},
		{
			name:      "no match",
			title:     "Database migration failed",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &models.Issue{Title: tt.title}
			target, _ := matcher.Match(issue)
			gotMatch := target != ""
			if gotMatch != tt.wantMatch {
				t.Errorf("Match() = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestRuleMatcher_Match_Priority(t *testing.T) {
	rules := []config.TransferRule{
		{
			Match:    config.MatchCondition{Labels: []string{"backend"}},
			Target:   "org/backend-service",
			Priority: 2,
		},
		{
			Match:    config.MatchCondition{Labels: []string{"backend"}, TitleContains: []string{"urgent"}},
			Target:   "org/urgent-backend",
			Priority: 1,
		},
	}

	matcher := NewRuleMatcher(rules)

	// Issue matches both rules - should match higher priority (lower number)
	issue := &models.Issue{
		Labels: []string{"backend"},
		Title:  "Urgent: API timeout",
	}

	target, _ := matcher.Match(issue)
	if target != "org/urgent-backend" {
		t.Errorf("Match() = %v, want org/urgent-backend", target)
	}
}

func TestRuleMatcher_Match_ANDLogic(t *testing.T) {
	// Multiple conditions in same rule = AND logic
	rules := []config.TransferRule{
		{
			Match: config.MatchCondition{
				Labels:        []string{"backend"},
				TitleContains: []string{"database"},
			},
			Target:   "org/data-platform",
			Priority: 1,
		},
	}

	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name      string
		issue     *models.Issue
		wantMatch bool
	}{
		{
			name:      "matches all conditions",
			issue:     &models.Issue{Labels: []string{"backend"}, Title: "Database connection issue"},
			wantMatch: true,
		},
		{
			name:      "only matches labels",
			issue:     &models.Issue{Labels: []string{"backend"}, Title: "API timeout"},
			wantMatch: false,
		},
		{
			name:      "only matches title",
			issue:     &models.Issue{Labels: []string{"frontend"}, Title: "Database error"},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, _ := matcher.Match(tt.issue)
			gotMatch := target != ""
			if gotMatch != tt.wantMatch {
				t.Errorf("Match() = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}
