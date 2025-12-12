package deployment

import (
	"testing"

	"github.com/opsorch/opsorch-core/deployment"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"token": "ghp_test_token",
				"owner": "testorg",
				"repo":  "testrepo",
			},
			wantErr: false,
		},
		{
			name: "missing token",
			config: map[string]any{
				"owner": "testorg",
				"repo":  "testrepo",
			},
			wantErr: true,
		},
		{
			name: "missing owner",
			config: map[string]any{
				"token": "ghp_test_token",
				"repo":  "testrepo",
			},
			wantErr: true,
		},
		{
			name: "missing repo",
			config: map[string]any{
				"token": "ghp_test_token",
				"owner": "testorg",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Errorf("New() returned nil provider")
			}
		})
	}
}

func TestProviderRegistration(t *testing.T) {
	// Test that the provider is registered
	constructor, ok := deployment.LookupProvider("github")
	if !ok {
		t.Errorf("GitHub deployment provider not registered")
	}
	if constructor == nil {
		t.Errorf("GitHub deployment provider constructor is nil")
	}
}

func TestNormalizeStatus(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		status     string
		conclusion string
		expected   string
	}{
		{"queued", "", "queued"},
		{"in_progress", "", "running"},
		{"completed", "success", "success"},
		{"completed", "failure", "failed"},
		{"completed", "cancelled", "cancelled"},
		{"completed", "skipped", "cancelled"},
		{"completed", "timed_out", "failed"},
		{"completed", "action_required", "failed"},
		{"completed", "unknown", "failed"},
		{"unknown", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status+"_"+tt.conclusion, func(t *testing.T) {
			result := p.normalizeStatus(tt.status, tt.conclusion)
			if result != tt.expected {
				t.Errorf("normalizeStatus(%s, %s) = %s, want %s", tt.status, tt.conclusion, result, tt.expected)
			}
		})
	}
}

func TestExtractEnvironment(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		workflowName string
		branch       string
		expected     string
	}{
		{"Deploy to Production", "main", "prod"},
		{"Deploy to Staging", "develop", "staging"},
		{"Test Deployment", "feature/test", "test"},
		{"Build", "main", "production"},
		{"Build", "master", "production"},
		{"Build", "develop", "dev"},
		{"Build", "dev", "dev"},
		{"Build", "feature/xyz", "development"},
	}

	for _, tt := range tests {
		t.Run(tt.workflowName+"_"+tt.branch, func(t *testing.T) {
			result := p.extractEnvironment(tt.workflowName, tt.branch)
			if result != tt.expected {
				t.Errorf("extractEnvironment(%s, %s) = %s, want %s", tt.workflowName, tt.branch, result, tt.expected)
			}
		})
	}
}
