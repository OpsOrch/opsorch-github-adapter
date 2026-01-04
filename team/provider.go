package team

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/opsorch/opsorch-core/orcherr"
	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-core/team"
)

// Provider implements the team.Provider interface for GitHub Teams.
type Provider struct {
	client *github.Client
	config Config
}

// Config holds the configuration for the GitHub team provider.
type Config struct {
	Token        string `json:"token"`        // GitHub personal access token
	Organization string `json:"organization"` // GitHub organization name
}

// New creates a new GitHub team provider.
func New(cfg map[string]any) (team.Provider, error) {
	var config Config

	// Parse token
	if token, ok := cfg["token"].(string); ok {
		config.Token = token
	} else {
		return nil, fmt.Errorf("token is required")
	}

	// Parse organization
	if org, ok := cfg["organization"].(string); ok {
		config.Organization = org
	} else {
		return nil, fmt.Errorf("organization is required")
	}

	// Create GitHub client
	client := github.NewTokenClient(context.Background(), config.Token)

	return &Provider{
		client: client,
		config: config,
	}, nil
}

// Query returns teams matching the given filters.
func (p *Provider) Query(ctx context.Context, query schema.TeamQuery) ([]schema.Team, error) {
	opts := &github.ListOptions{
		PerPage: 100, // GitHub's max per page
	}

	teams, _, err := p.client.Teams.ListTeams(ctx, p.config.Organization, opts)
	if err != nil {
		return nil, p.wrapError(err)
	}

	var result []schema.Team
	for _, team := range teams {
		normalizedTeam := p.convertTeamToSchema(team)

		// Filter by name if specified
		if query.Name != "" && !strings.Contains(strings.ToLower(normalizedTeam.Name), strings.ToLower(query.Name)) {
			continue
		}

		// Filter by tags if specified
		if len(query.Tags) > 0 {
			match := true
			for key, value := range query.Tags {
				if normalizedTeam.Tags[key] != value {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		result = append(result, normalizedTeam)
	}

	return result, nil
}

// Get returns a single team by its ID.
func (p *Provider) Get(ctx context.Context, id string) (schema.Team, error) {
	teamID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		// Try by slug if ID parsing fails
		team, _, err := p.client.Teams.GetTeamBySlug(ctx, p.config.Organization, id)
		if err != nil {
			return schema.Team{}, p.wrapError(err)
		}
		return p.convertTeamToSchema(team), nil
	}

	// Get organization ID first
	org, _, err := p.client.Organizations.Get(ctx, p.config.Organization)
	if err != nil {
		return schema.Team{}, p.wrapError(err)
	}

	team, _, err := p.client.Teams.GetTeamByID(ctx, org.GetID(), teamID)
	if err != nil {
		return schema.Team{}, p.wrapError(err)
	}

	return p.convertTeamToSchema(team), nil
}

// Members returns the members of a team.
func (p *Provider) Members(ctx context.Context, teamID string) ([]schema.TeamMember, error) {
	id, err := strconv.ParseInt(teamID, 10, 64)
	if err != nil {
		// Try by slug if ID parsing fails
		team, _, err := p.client.Teams.GetTeamBySlug(ctx, p.config.Organization, teamID)
		if err != nil {
			return nil, p.wrapError(err)
		}
		id = team.GetID()
	}

	opts := &github.TeamListTeamMembersOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// Get organization ID first
	org, _, err := p.client.Organizations.Get(ctx, p.config.Organization)
	if err != nil {
		return nil, p.wrapError(err)
	}

	members, _, err := p.client.Teams.ListTeamMembersByID(ctx, org.GetID(), id, opts)
	if err != nil {
		return nil, p.wrapError(err)
	}

	var result []schema.TeamMember
	for _, member := range members {
		// Get detailed user info to get email and name
		user, _, err := p.client.Users.Get(ctx, member.GetLogin())
		if err != nil {
			// If we can't get detailed info, use basic info
			result = append(result, schema.TeamMember{
				ID:     member.GetLogin(),
				Name:   member.GetLogin(),
				Handle: member.GetLogin(),
				Role:   "member", // Default role
				Metadata: map[string]any{
					"github_id":  member.GetID(),
					"avatar_url": member.GetAvatarURL(),
					"html_url":   member.GetHTMLURL(),
					"site_admin": member.GetSiteAdmin(),
					"type":       member.GetType(),
				},
			})
			continue
		}

		// Get team membership to determine role
		membership, _, err := p.client.Teams.GetTeamMembershipByID(ctx, org.GetID(), id, member.GetLogin())
		role := "member"
		if err == nil && membership != nil {
			role = membership.GetRole() // "member" or "maintainer"
		}

		result = append(result, schema.TeamMember{
			ID:     member.GetLogin(),
			Name:   user.GetName(),
			Email:  user.GetEmail(),
			Handle: member.GetLogin(),
			Role:   p.normalizeRole(role),
			Metadata: map[string]any{
				"github_id":    member.GetID(),
				"avatar_url":   member.GetAvatarURL(),
				"html_url":     member.GetHTMLURL(),
				"site_admin":   member.GetSiteAdmin(),
				"type":         member.GetType(),
				"company":      user.GetCompany(),
				"location":     user.GetLocation(),
				"bio":          user.GetBio(),
				"blog":         user.GetBlog(),
				"twitter":      user.GetTwitterUsername(),
				"public_repos": user.GetPublicRepos(),
				"followers":    user.GetFollowers(),
				"following":    user.GetFollowing(),
			},
		})
	}

	return result, nil
}

// convertTeamToSchema converts a GitHub Team to a normalized Team.
func (p *Provider) convertTeamToSchema(team *github.Team) schema.Team {
	// Use team ID as primary identifier, with slug as fallback
	id := strconv.FormatInt(team.GetID(), 10)
	if team.GetSlug() != "" {
		id = team.GetSlug()
	}

	normalizedTeam := schema.Team{
		ID:   id,
		Name: team.GetName(),
		URL:  team.GetHTMLURL(),
		Tags: map[string]string{
			"provider":   "github",
			"privacy":    team.GetPrivacy(),
			"permission": team.GetPermission(),
		},
		Metadata: map[string]any{
			"github_id":        team.GetID(),
			"slug":             team.GetSlug(),
			"description":      team.GetDescription(),
			"privacy":          team.GetPrivacy(),
			"permission":       team.GetPermission(),
			"html_url":         team.GetHTMLURL(),
			"members_url":      team.GetMembersURL(),
			"repositories_url": team.GetRepositoriesURL(),
			"members_count":    team.GetMembersCount(),
			"repos_count":      team.GetReposCount(),
		},
	}

	// Handle parent team if it exists
	if parent := team.GetParent(); parent != nil {
		normalizedTeam.Parent = strconv.FormatInt(parent.GetID(), 10)
		if parent.GetSlug() != "" {
			normalizedTeam.Parent = parent.GetSlug()
		}
	}

	// Add organization info to tags
	normalizedTeam.Tags["organization"] = p.config.Organization

	return normalizedTeam
}

// normalizeRole converts GitHub team roles to standard roles.
func (p *Provider) normalizeRole(role string) string {
	switch strings.ToLower(role) {
	case "maintainer":
		return "owner"
	case "member":
		return "member"
	default:
		return "member"
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
				Message: "GitHub API access forbidden - check token permissions",
			}
		case 404:
			return &orcherr.OpsOrchError{
				Code:    "not_found",
				Message: "GitHub organization or team not found",
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
	team.RegisterProvider("github", New)
}
