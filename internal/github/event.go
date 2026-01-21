package github

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
)

// Event represents a GitHub webhook event
type Event struct {
	Action string       `json:"action"`
	Issue  *EventIssue  `json:"issue"`
	Repo   *EventRepo   `json:"repository"`
	Sender *EventSender `json:"sender"`
}

// EventIssue represents issue data in an event
type EventIssue struct {
	Number  int          `json:"number"`
	Title   string       `json:"title"`
	Body    string       `json:"body"`
	State   string       `json:"state"`
	HTMLURL string       `json:"html_url"`
	User    *EventSender `json:"user"`
	Labels  []Label      `json:"labels"`
}

// EventRepo represents repository data in an event
type EventRepo struct {
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
	Name string `json:"name"`
}

// EventSender represents the user who triggered the event
type EventSender struct {
	Login string `json:"login"`
}

// ParseEventFile reads and parses a GitHub event JSON file
func ParseEventFile(path string) (*Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read event file: %w", err)
	}

	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to parse event JSON: %w", err)
	}

	return &event, nil
}

// ToIssue converts event issue to models.Issue
func (e *Event) ToIssue() *models.Issue {
	if e.Issue == nil || e.Repo == nil {
		return nil
	}

	labels := make([]string, len(e.Issue.Labels))
	for i, l := range e.Issue.Labels {
		labels[i] = l.Name
	}

	author := ""
	if e.Issue.User != nil {
		author = e.Issue.User.Login
	}

	return &models.Issue{
		Org:    e.Repo.Owner.Login,
		Repo:   e.Repo.Name,
		Number: e.Issue.Number,
		Title:  e.Issue.Title,
		Body:   e.Issue.Body,
		State:  e.Issue.State,
		Labels: labels,
		Author: author,
		URL:    e.Issue.HTMLURL,
	}
}

// IsIssueEvent checks if this is an issue event
func (e *Event) IsIssueEvent() bool {
	return e.Issue != nil
}

// IsOpenedEvent checks if this is an issue opened event
func (e *Event) IsOpenedEvent() bool {
	return e.Action == "opened"
}

// IsEditedEvent checks if this is an issue edited event
func (e *Event) IsEditedEvent() bool {
	return e.Action == "edited"
}

// IsClosedEvent checks if this is an issue closed event
func (e *Event) IsClosedEvent() bool {
	return e.Action == "closed"
}

// IsDeletedEvent checks if this is an issue deleted event
func (e *Event) IsDeletedEvent() bool {
	return e.Action == "deleted"
}

// IsReopenedEvent checks if this is an issue reopened event
func (e *Event) IsReopenedEvent() bool {
	return e.Action == "reopened"
}
