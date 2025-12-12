package deployment

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/opsorch/opsorch-core/deployment"
	"github.com/opsorch/opsorch-core/orcherr"
	"github.com/opsorch/opsorch-core/schema"
)

// Provider implements the deployment.Provider interface for GitHub Actions.
type Provider struct {
	client *github.Client
	config Config
}

// Config holds the configuration for the GitHub deployment provider.
type Config struct {
	Token string `json:"token"` // GitHub personal access token
	Owner string `json:"owner"` // Repository owner (user or organization)
	Repo  string `json:"repo"`  // Repository name
}

// New creates a new GitHub deployment provider.
func New(cfg map[string]any) (deployment.Provider, error) {
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

	// Create GitHub client
	client := github.NewTokenClient(context.Background(), config.Token)

	return &Provider{
		client: client,
		config: config,
	}, nil
}

// Query returns deployments (GitHub Actions workflow runs) matching the given filters.
func (p *Provider) Query(ctx context.Context, query schema.DeploymentQuery) ([]schema.Deployment, error) {
	opts := &github.ListWorkflowRunsOptions{
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
		// Map OpsOrch statuses to GitHub workflow run statuses
		for _, status := range query.Statuses {
			switch strings.ToLower(status) {
			case "queued":
				opts.Status = "queued"
			case "running", "in_progress":
				opts.Status = "in_progress"
			case "success", "completed":
				opts.Status = "completed"
			case "failed":
				opts.Status = "completed" // We'll filter by conclusion later
			case "cancelled":
				opts.Status = "completed" // We'll filter by conclusion later
			}
		}
	}

	// Apply branch filter from metadata
	if branch, ok := query.Metadata["branch"].(string); ok {
		opts.Branch = branch
	}

	// Apply actor filter from metadata
	if actor, ok := query.Metadata["actor"].(string); ok {
		opts.Actor = actor
	}

	// Apply event filter from metadata
	if event, ok := query.Metadata["event"].(string); ok {
		opts.Event = event
	}

	runs, _, err := p.client.Actions.ListRepositoryWorkflowRuns(ctx, p.config.Owner, p.config.Repo, opts)
	if err != nil {
		return nil, p.wrapError(err)
	}

	deployments := make([]schema.Deployment, 0, len(runs.WorkflowRuns))
	for _, run := range runs.WorkflowRuns {
		// Apply conclusion filter if status filter was specified
		if len(query.Statuses) > 0 {
			normalizedStatus := p.normalizeStatus(run.GetStatus(), run.GetConclusion())
			found := false
			for _, status := range query.Statuses {
				if strings.EqualFold(normalizedStatus, status) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		deployment := p.convertWorkflowRunToDeployment(run)

		// Apply service filter from scope
		if query.Scope.Service != "" && deployment.Service != query.Scope.Service {
			continue
		}

		// Apply environment filter from scope
		if query.Scope.Environment != "" && deployment.Environment != query.Scope.Environment {
			continue
		}

		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

// Get returns a single deployment by its ID (workflow run ID).
func (p *Provider) Get(ctx context.Context, id string) (schema.Deployment, error) {
	runID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return schema.Deployment{}, &orcherr.OpsOrchError{
			Code:    "bad_request",
			Message: fmt.Sprintf("invalid workflow run ID: %s", id),
		}
	}

	run, _, err := p.client.Actions.GetWorkflowRunByID(ctx, p.config.Owner, p.config.Repo, runID)
	if err != nil {
		return schema.Deployment{}, p.wrapError(err)
	}

	return p.convertWorkflowRunToDeployment(run), nil
}

// convertWorkflowRunToDeployment converts a GitHub workflow run to a normalized Deployment.
func (p *Provider) convertWorkflowRunToDeployment(run *github.WorkflowRun) schema.Deployment {
	deployment := schema.Deployment{
		ID:     strconv.FormatInt(run.GetID(), 10),
		Status: p.normalizeStatus(run.GetStatus(), run.GetConclusion()),
		URL:    run.GetHTMLURL(),
		Fields: map[string]any{
			"workflow_name": run.GetName(),
			"branch":        run.GetHeadBranch(),
			"commit":        run.GetHeadSHA(),
		},
	}

	// Set timestamps
	if createdAt := run.GetCreatedAt(); !createdAt.IsZero() {
		deployment.StartedAt = createdAt.Time
	}
	if updatedAt := run.GetUpdatedAt(); !updatedAt.IsZero() {
		deployment.FinishedAt = updatedAt.Time
	}

	// Extract service name from repository name
	deployment.Service = p.config.Repo

	// Try to extract environment from workflow name or branch
	deployment.Environment = p.extractEnvironment(run.GetName(), run.GetHeadBranch())

	// Extract version from head SHA (short form)
	if headSHA := run.GetHeadSHA(); headSHA != "" && len(headSHA) >= 7 {
		deployment.Version = headSHA[:7]
	}

	// Add actor information (just the login name)
	if actor := run.GetActor(); actor != nil {
		deployment.Actor = map[string]any{
			"login": actor.GetLogin(),
		}
	}

	// Add commit message if available
	if headCommit := run.GetHeadCommit(); headCommit != nil {
		deployment.Fields["commit_message"] = headCommit.GetMessage()
	}

	return deployment
}

// normalizeStatus converts GitHub workflow run status and conclusion to normalized status.
func (p *Provider) normalizeStatus(status, conclusion string) string {
	switch strings.ToLower(status) {
	case "queued":
		return "queued"
	case "in_progress":
		return "running"
	case "completed":
		switch strings.ToLower(conclusion) {
		case "success":
			return "success"
		case "failure":
			return "failed"
		case "cancelled":
			return "cancelled"
		case "skipped":
			return "cancelled"
		case "timed_out":
			return "failed"
		case "action_required":
			return "failed"
		default:
			return "failed"
		}
	default:
		return status
	}
}

// extractEnvironment tries to extract environment from workflow name or branch.
func (p *Provider) extractEnvironment(workflowName, branch string) string {
	// Common environment patterns
	envPatterns := []string{"prod", "production", "staging", "stage", "dev", "development", "test", "testing"}

	// Check workflow name first
	workflowLower := strings.ToLower(workflowName)
	for _, env := range envPatterns {
		if strings.Contains(workflowLower, env) {
			return env
		}
	}

	// Check branch name
	branchLower := strings.ToLower(branch)
	for _, env := range envPatterns {
		if strings.Contains(branchLower, env) {
			return env
		}
	}

	// Default mappings
	switch branchLower {
	case "main", "master":
		return "production"
	case "develop", "dev":
		return "development"
	default:
		return "development" // Default fallback
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
				Message: "GitHub repository or workflow run not found",
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
	deployment.RegisterProvider("github", New)
}
