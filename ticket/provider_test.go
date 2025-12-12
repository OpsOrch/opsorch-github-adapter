package ticket

import (
	"testing"

	"github.com/opsorch/opsorch-core/ticket"
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
	constructor, ok := ticket.LookupProvider("github")
	if !ok {
		t.Errorf("GitHub ticket provider not registered")
	}
	if constructor == nil {
		t.Errorf("GitHub ticket provider constructor is nil")
	}
}

func TestNormalizeStatus(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		input    string
		expected string
	}{
		{"open", "open"},
		{"closed", "closed"},
		{"OPEN", "open"},
		{"CLOSED", "closed"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := p.normalizeStatus(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeStatus(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
