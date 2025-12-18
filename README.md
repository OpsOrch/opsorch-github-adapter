# OpsOrch GitHub Adapter

[![Go Version](https://img.shields.io/github/go-mod/go-version/opsorch/opsorch-github-adapter)](https://github.com/opsorch/opsorch-github-adapter/blob/main/go.mod)
[![License](https://img.shields.io/github/license/opsorch/opsorch-github-adapter)](https://github.com/opsorch/opsorch-github-adapter/blob/main/LICENSE)

The OpsOrch GitHub Adapter provides integration with GitHub for ticket management (GitHub Issues), deployment tracking (GitHub Actions), and team management (GitHub Teams). This adapter implements multiple OpsOrch capabilities in a single package.

## Features

### Ticket Provider (GitHub Issues)
- Query GitHub Issues with filters
- Get individual issues by number
- Create new issues
- Update existing issues
- Support for labels, assignees, and milestones
- Automatic status normalization

### Deployment Provider (GitHub Actions)
- Query GitHub Actions workflow runs
- Get individual workflow runs by ID
- Filter by status, branch, actor, and event
- Automatic environment detection
- Rich metadata including commit information and actor details

### Team Provider (GitHub Teams)
- Query GitHub Teams with filters
- Get individual teams by ID or slug
- Get team members with roles and detailed information
- Support for nested team hierarchies
- Automatic role normalization (maintainer → owner)

## Installation

### As In-Process Provider

Add the adapter to your OpsOrch Core binary:

```go
import (
    _ "github.com/opsorch/opsorch-github-adapter"
)
```

### As Plugin

Build the plugin binaries:

```bash
make plugins
```

This creates:
- `bin/ticketplugin` - GitHub Issues plugin
- `bin/deploymentplugin` - GitHub Actions plugin
- `bin/teamplugin` - GitHub Teams plugin

## Configuration

### Ticket Provider (GitHub Issues)

```bash
# In-process provider
OPSORCH_TICKET_PROVIDER=github
OPSORCH_TICKET_CONFIG='{
  "token": "ghp_your_github_token",
  "owner": "your-org",
  "repo": "your-repo",
  "defaultState": "open"
}'

# Plugin provider
OPSORCH_TICKET_PLUGIN=/path/to/bin/ticketplugin
OPSORCH_TICKET_CONFIG='{
  "token": "ghp_your_github_token",
  "owner": "your-org", 
  "repo": "your-repo"
}'
```

### Deployment Provider (GitHub Actions)

```bash
# In-process provider
OPSORCH_DEPLOYMENT_PROVIDER=github
OPSORCH_DEPLOYMENT_CONFIG='{
  "token": "ghp_your_github_token",
  "owner": "your-org",
  "repo": "your-repo"
}'

# Plugin provider
OPSORCH_DEPLOYMENT_PLUGIN=/path/to/bin/deploymentplugin
OPSORCH_DEPLOYMENT_CONFIG='{
  "token": "ghp_your_github_token",
  "owner": "your-org",
  "repo": "your-repo"
}'
```

### Team Provider (GitHub Teams)

```bash
# In-process provider
OPSORCH_TEAM_PROVIDER=github
OPSORCH_TEAM_CONFIG='{
  "token": "ghp_your_github_token",
  "organization": "your-org"
}'

# Plugin provider
OPSORCH_TEAM_PLUGIN=/path/to/bin/teamplugin
OPSORCH_TEAM_CONFIG='{
  "token": "ghp_your_github_token",
  "organization": "your-org"
}'
```

### Configuration Fields

| Field | Required | Provider | Description |
|-------|----------|----------|-------------|
| `token` | Yes | All | GitHub personal access token |
| `owner` | Yes | Ticket, Deployment | Repository owner (user or organization) |
| `repo` | Yes | Ticket, Deployment | Repository name |
| `organization` | Yes | Team | GitHub organization name |
| `defaultState` | No | Ticket | Default state for new issues |

### GitHub Token Permissions

Your GitHub token needs the following scopes:

**For Ticket Provider:**
- `repo` (for private repositories) or `public_repo` (for public repositories)
- `issues:write` (to create and update issues)

**For Deployment Provider:**
- `repo` (for private repositories) or `public_repo` (for public repositories)
- `actions:read` (to read workflow runs)

**For Team Provider:**
- `read:org` (to read organization teams)
- `read:user` (to read team member details)

## Usage Examples

### Query GitHub Issues

```bash
curl -X POST http://localhost:8080/tickets/query \
  -H "Content-Type: application/json" \
  -d '{
    "statuses": ["open"],
    "metadata": {
      "labels": ["bug", "high-priority"]
    },
    "limit": 10
  }'
```

### Create GitHub Issue

```bash
curl -X POST http://localhost:8080/tickets \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Bug in user authentication",
    "description": "Users are unable to log in with valid credentials",
    "assignee": "developer-username",
    "metadata": {
      "labels": ["bug", "urgent"]
    }
  }'
```

### Query GitHub Actions Deployments

```bash
curl -X POST http://localhost:8080/deployments/query \
  -H "Content-Type: application/json" \
  -d '{
    "statuses": ["success", "failed"],
    "scope": {
      "environment": "production"
    },
    "metadata": {
      "branch": "main",
      "event": "push"
    },
    "limit": 20
  }'
```

### Get Specific Deployment

```bash
curl http://localhost:8080/deployments/1234567890
```

### Query GitHub Teams

```bash
curl -X POST http://localhost:8080/teams/query \
  -H "Content-Type: application/json" \
  -d '{
    "name": "backend",
    "tags": {
      "privacy": "closed"
    }
  }'
```

### Get Team Details

```bash
curl http://localhost:8080/teams/engineering
```

### Get Team Members

```bash
curl http://localhost:8080/teams/engineering/members
```

## Data Mapping

### GitHub Issues → OpsOrch Tickets

| GitHub Field | OpsOrch Field | Notes |
|--------------|---------------|-------|
| `number` | `id` | Issue number as string |
| `title` | `title` | Issue title |
| `body` | `description` | Issue description |
| `state` | `status` | Normalized to "open"/"closed" |
| `assignee.login` | `assignee` | Primary assignee |
| `user.login` | `reporter` | Issue creator |
| `created_at` | `createdAt` | Creation timestamp |
| `updated_at` | `updatedAt` | Last update timestamp |
| `html_url` | `fields.url` | GitHub issue URL |
| `labels` | `fields.labels` | Issue labels |

### GitHub Actions → OpsOrch Deployments

| GitHub Field | OpsOrch Field | Notes |
|--------------|---------------|-------|
| `id` | `id` | Workflow run ID as string |
| `name` | `fields.workflow_name` | Workflow name |
| `head_sha` | `version` | Short commit SHA |
| `status`/`conclusion` | `status` | Normalized status |
| `created_at` | `startedAt` | Run start time |
| `updated_at` | `finishedAt` | Run completion time |
| `html_url` | `url` | GitHub Actions run URL |
| `actor` | `actor` | User who triggered the run |
| `head_branch` | `fields.branch` | Source branch |

### GitHub Teams → OpsOrch Teams

| GitHub Field | OpsOrch Field | Notes |
|--------------|---------------|-------|
| `id` | `id` | Team ID as string (or slug if available) |
| `name` | `name` | Team name |
| `parent.id` | `parent` | Parent team ID (for nested teams) |
| `privacy` | `tags.privacy` | Team privacy level |
| `permission` | `tags.permission` | Team permission level |
| `slug` | `metadata.slug` | Team slug |
| `description` | `metadata.description` | Team description |
| `members_count` | `metadata.members_count` | Number of team members |

### GitHub Team Members → OpsOrch Team Members

| GitHub Field | OpsOrch Field | Notes |
|--------------|---------------|-------|
| `login` | `id` | GitHub username |
| `name` | `name` | User's display name |
| `email` | `email` | User's email address |
| `login` | `handle` | GitHub username |
| `role` | `role` | Normalized role (maintainer → owner) |

## Environment Detection

The deployment provider automatically detects environments based on:

1. **Workflow name** - Looks for keywords like "prod", "staging", "dev"
2. **Branch name** - Maps common branch patterns:
   - `main`/`master` → `production`
   - `develop`/`dev` → `development`
   - Contains "prod" → `production`
   - Contains "stage" → `staging`

## Error Handling

The adapter maps GitHub API errors to OpsOrch error codes:

| GitHub Status | OpsOrch Code | Description |
|---------------|--------------|-------------|
| 401 | `unauthorized` | Invalid or expired token |
| 403 | `forbidden` | Insufficient permissions |
| 404 | `not_found` | Repository or resource not found |
| 422 | `bad_request` | Validation error |
| Other | `provider_error` | Generic GitHub API error |

## Development

### Building

```bash
# Build everything
make all

# Build just the library
make build

# Build just the plugins
make plugins
```

### Testing

**Unit Tests:**
```bash
make test
```

**Integration Tests:**

Integration tests run against a real GitHub repository and require authentication.

**Prerequisites:**
- GitHub personal access token with appropriate permissions
- Access to a GitHub repository for testing (you need write access to create issues)
- **For meaningful deployment tests**: Repository should have GitHub Actions enabled with some workflow runs
- **For comprehensive ticket tests**: Repository should have some existing issues (tests will create their own)

**Setup:**
```bash
# Set required environment variables
export GITHUB_TOKEN="ghp_your_github_token_here"
export GITHUB_OWNER="your-org"          # Optional, defaults to "opsorch" (just the org/username)
export GITHUB_REPO="your-test-repo"     # Optional, defaults to "opsorch-github-adapter" (just the repo name)

# Example for testing against github.com/mycompany/my-project:
# export GITHUB_OWNER="mycompany"
# export GITHUB_REPO="my-project"

# Run all integration tests
make integ

# Or run specific capability tests
make integ-ticket      # Test GitHub Issues integration
make integ-deployment  # Test GitHub Actions integration
make integ-team        # Test GitHub Teams integration
```

**What the tests do:**
- **Ticket tests**: Query existing issues, create/update/close test issues, test filtering by status and labels
- **Deployment tests**: Query workflow runs, test filtering by status/environment/branch, validate metadata extraction
- **Team tests**: Query organization teams, get team details, retrieve team members with roles

**Expected behavior:**
- Tests create temporary issues that are automatically closed and cleaned up
- Cleanup happens both during normal test completion and via defer if tests fail
- Tests only read existing workflow runs (no new workflows are triggered)
- All tests should pass if credentials and repository access are correct
- No test artifacts should remain visible in GitHub after test completion

**Required GitHub Token Permissions:**
- `repo` scope (for private repos) or `public_repo` (for public repos)
- `issues:write` (to create/update test issues)
- `actions:read` (to read workflow runs)
- `read:org` (to read organization teams)
- `read:user` (to read team member details)

**Setting Up Test Data:**

For comprehensive testing, your repository should have:

1. **GitHub Actions workflows** (for deployment tests):
   ```bash
   mkdir -p .github/workflows
   cat > .github/workflows/ci.yml << 'EOF'
   name: CI
   on: [push, pull_request]
   jobs:
     test:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
         - run: echo "Hello World"
   EOF
   git add .github/workflows/ci.yml
   git commit -m "Add CI workflow"
   git push
   ```

2. **Some existing issues** (optional - tests will create their own):
   - Create a few issues manually, or let the integration tests create them
   - Tests automatically clean up any issues they create

**Note:** Tests are designed to work with empty repositories but provide more comprehensive validation when test data is available.

**Test Coverage:**

*Ticket Integration Tests:*
- Provider initialization and configuration validation
- Query existing issues with various filters (status, labels, text search)
- Create new issues with labels and metadata
- Get specific issues by ID
- Update issue titles and status
- Error handling for invalid issue IDs
- Automatic cleanup of test issues

*Deployment Integration Tests:*
- Provider initialization and configuration validation
- Query workflow runs with various filters (status, environment, branch)
- Get specific deployments by workflow run ID
- Validate environment extraction from workflow names and branches
- Test metadata field mapping (workflow name, branch, commit info)
- Error handling for invalid workflow run IDs
- Verify actor information and timestamps

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint
```

## Troubleshooting

### Common Issues

**Authentication Error:**
```
GitHub API authentication failed
```
- Verify your token is correct and not expired
- Check token permissions include required scopes

**Repository Not Found:**
```
GitHub repository or issue not found
```
- Verify owner and repo names are correct
- Check token has access to the repository

**Rate Limiting:**
```
GitHub API error: rate limit exceeded
```
- Use authenticated requests (higher rate limits)
- Implement request throttling in your application

### Debug Mode

Enable debug logging in OpsOrch Core:

```bash
OPSORCH_LOG_LEVEL=debug go run ./cmd/opsorch
```

## License

Apache 2.0