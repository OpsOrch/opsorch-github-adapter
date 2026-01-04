package ticket

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/opsorch/opsorch-core/orcherr"
	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-core/ticket"
)

// Provider implements the ticket.Provider interface for GitHub Issues.
type Provider struct {
	client *github.Client
	config Config
}

// Config holds the configuration for the GitHub ticket provider.
type Config struct {
	Token        string `json:"token"`        // GitHub personal access token
	Owner        string `json:"owner"`        // Repository owner (user or organization)
	Repo         string `json:"repo"`         // Repository name
	DefaultState string `json:"defaultState"` // Default state for new issues (open/closed)
}

// New creates a new GitHub ticket provider.
func New(cfg map[string]any) (ticket.Provider, error) {
	var config Config

	// Parse token
	if token, ok := cfg["token"].(string); ok {
		config.Token = token
	} else {
		return nil, fmt.Errorf("token is required")
	}

	// Parse owner
	if owner, ok := cfg["owner"].(string); ok {
		config.Owner = owner
	} else {
		return nil, fmt.Errorf("owner is required")
	}

	// Parse repo
	if repo, ok := cfg["repo"].(string); ok {
		config.Repo = repo
	} else {
		return nil, fmt.Errorf("repo is required")
	}

	// Parse default state (optional)
	if state, ok := cfg["defaultState"].(string); ok {
		config.DefaultState = state
	} else {
		config.DefaultState = "open"
	}

	// Create GitHub client
	client := github.NewTokenClient(context.Background(), config.Token)

	return &Provider{
		client: client,
		config: config,
	}, nil
}

// Query returns tickets (GitHub Issues) matching the given filters.
func (p *Provider) Query(ctx context.Context, query schema.TicketQuery) ([]schema.Ticket, error) {
	opts := &github.IssueListByRepoOptions{
		ListOptions: github.ListOptions{
			PerPage: 100, // GitHub's max per page
		},
	}

	// Apply limit
	if query.Limit > 0 && query.Limit < 100 {
		opts.PerPage = query.Limit
	}

	// Apply status filter
	if len(query.Statuses) > 0 {
		// GitHub Issues only support "open" or "closed"
		for _, status := range query.Statuses {
			switch strings.ToLower(status) {
			case "open", "new", "in_progress":
				opts.State = "open"
			case "closed", "resolved", "done":
				opts.State = "closed"
			}
		}
	}

	// Apply assignee filter from scope
	if query.Scope.Team != "" {
		opts.Assignee = query.Scope.Team
	}

	// Apply labels from metadata
	if labels, ok := query.Metadata["labels"].([]string); ok {
		opts.Labels = labels
	}

	issues, _, err := p.client.Issues.ListByRepo(ctx, p.config.Owner, p.config.Repo, opts)
	if err != nil {
		return nil, p.wrapError(err)
	}

	tickets := make([]schema.Ticket, 0, len(issues))
	for _, issue := range issues {
		// Skip pull requests (GitHub API includes them in issues)
		if issue.PullRequestLinks != nil {
			continue
		}

		ticket := p.convertIssueToTicket(issue)
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// Get returns a single ticket by its ID.
func (p *Provider) Get(ctx context.Context, id string) (schema.Ticket, error) {
	issueNumber, err := strconv.Atoi(id)
	if err != nil {
		return schema.Ticket{}, &orcherr.OpsOrchError{
			Code:    "bad_request",
			Message: fmt.Sprintf("invalid issue number: %s", id),
		}
	}

	issue, _, err := p.client.Issues.Get(ctx, p.config.Owner, p.config.Repo, issueNumber)
	if err != nil {
		return schema.Ticket{}, p.wrapError(err)
	}

	return p.convertIssueToTicket(issue), nil
}

// Create creates a new ticket (GitHub Issue).
func (p *Provider) Create(ctx context.Context, input schema.CreateTicketInput) (schema.Ticket, error) {
	issueRequest := &github.IssueRequest{
		Title: &input.Title,
		Body:  &input.Description,
	}

	// Set assignees from fields if provided
	if assignees, ok := input.Fields["assignees"].([]string); ok && len(assignees) > 0 {
		issueRequest.Assignees = &assignees
	}

	// Set labels from metadata
	if labels, ok := input.Metadata["labels"].([]string); ok {
		issueRequest.Labels = &labels
	}

	issue, _, err := p.client.Issues.Create(ctx, p.config.Owner, p.config.Repo, issueRequest)
	if err != nil {
		return schema.Ticket{}, p.wrapError(err)
	}

	return p.convertIssueToTicket(issue), nil
}

// Update updates an existing ticket.
func (p *Provider) Update(ctx context.Context, id string, input schema.UpdateTicketInput) (schema.Ticket, error) {
	issueNumber, err := strconv.Atoi(id)
	if err != nil {
		return schema.Ticket{}, &orcherr.OpsOrchError{
			Code:    "bad_request",
			Message: fmt.Sprintf("invalid issue number: %s", id),
		}
	}

	issueRequest := &github.IssueRequest{}

	// Update title if provided
	if input.Title != nil && *input.Title != "" {
		issueRequest.Title = input.Title
	}

	// Update description if provided
	if input.Description != nil && *input.Description != "" {
		issueRequest.Body = input.Description
	}

	// Update status if provided
	if input.Status != nil && *input.Status != "" {
		switch strings.ToLower(*input.Status) {
		case "open", "new", "in_progress":
			state := "open"
			issueRequest.State = &state
		case "closed", "resolved", "done":
			state := "closed"
			issueRequest.State = &state
		}
	}

	// Update assignees if provided
	if input.Assignees != nil && len(*input.Assignees) > 0 {
		issueRequest.Assignees = input.Assignees
	}

	issue, _, err := p.client.Issues.Edit(ctx, p.config.Owner, p.config.Repo, issueNumber, issueRequest)
	if err != nil {
		return schema.Ticket{}, p.wrapError(err)
	}

	return p.convertIssueToTicket(issue), nil
}

// convertIssueToTicket converts a GitHub Issue to a normalized Ticket.
func (p *Provider) convertIssueToTicket(issue *github.Issue) schema.Ticket {
	ticket := schema.Ticket{
		ID:          strconv.Itoa(issue.GetNumber()),
		Title:       issue.GetTitle(),
		Description: issue.GetBody(),
		Status:      p.normalizeStatus(issue.GetState()),
		URL:         issue.GetHTMLURL(),
		CreatedAt:   issue.GetCreatedAt().Time,
		UpdatedAt:   issue.GetUpdatedAt().Time,
		Fields: map[string]any{
			"url": issue.GetHTMLURL(),
		},
	}

	// Add assignees
	if len(issue.Assignees) > 0 {
		assignees := make([]string, len(issue.Assignees))
		for i, assignee := range issue.Assignees {
			assignees[i] = assignee.GetLogin()
		}
		ticket.Assignees = assignees
	}

	// Add reporter
	if user := issue.GetUser(); user != nil {
		ticket.Reporter = user.GetLogin()
	}

	// Add labels
	if len(issue.Labels) > 0 {
		labels := make([]string, len(issue.Labels))
		for i, label := range issue.Labels {
			labels[i] = label.GetName()
		}
		ticket.Fields["labels"] = labels
	}

	// Add milestone
	if milestone := issue.GetMilestone(); milestone != nil {
		ticket.Fields["milestone"] = milestone.GetTitle()
	}

	return ticket
}

// normalizeStatus converts GitHub issue state to normalized status.
func (p *Provider) normalizeStatus(state string) string {
	switch strings.ToLower(state) {
	case "open":
		return "open"
	case "closed":
		return "closed"
	default:
		return state
	}
}

// wrapError wraps GitHub API errors into OpsOrch errors.
func (p *Provider) wrapError(err error) error {
	if ghErr, ok := err.(*github.ErrorResponse); ok {
		switch ghErr.Response.StatusCode {
		case 401:
			return &orcherr.OpsOrchError{
				Code:    "unauthorized",
				Message: "GitHub API authentication failed",
			}
		case 403:
			return &orcherr.OpsOrchError{
				Code:    "forbidden",
				Message: "GitHub API access forbidden",
			}
		case 404:
			return &orcherr.OpsOrchError{
				Code:    "not_found",
				Message: "GitHub repository or issue not found",
			}
		case 422:
			return &orcherr.OpsOrchError{
				Code:    "bad_request",
				Message: fmt.Sprintf("GitHub API validation error: %s", ghErr.Message),
			}
		default:
			return &orcherr.OpsOrchError{
				Code:    "provider_error",
				Message: fmt.Sprintf("GitHub API error: %s", ghErr.Message),
			}
		}
	}

	return &orcherr.OpsOrchError{
		Code:    "provider_error",
		Message: fmt.Sprintf("GitHub API error: %v", err),
	}
}

func init() {
	ticket.RegisterProvider("github", New)
}
