package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/opsorch/opsorch-core/schema"
	coreteam "github.com/opsorch/opsorch-core/team"
	"github.com/opsorch/opsorch-github-adapter/team"
)

// PluginRequest represents an incoming RPC request.
type PluginRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// PluginResponse represents an outgoing RPC response.
type PluginResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *PluginError    `json:"error,omitempty"`
}

// PluginError represents an error in the plugin response.
type PluginError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func main() {
	// Read configuration from environment
	configJSON := os.Getenv("OPSORCH_TEAM_CONFIG")
	if configJSON == "" {
		log.Fatal("OPSORCH_TEAM_CONFIG environment variable is required")
	}

	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	// Create the GitHub team provider
	provider, err := team.New(config)
	if err != nil {
		log.Fatalf("Failed to create GitHub team provider: %v", err)
	}

	// Process RPC requests from stdin
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req PluginRequest
		if err := decoder.Decode(&req); err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Printf("Failed to decode request: %v", err)
			continue
		}

		response := handleRequest(provider, req)
		if err := encoder.Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

func handleRequest(provider coreteam.Provider, req PluginRequest) PluginResponse {
	ctx := context.Background()

	switch req.Method {
	case "team.query":
		var query schema.TeamQuery
		if err := json.Unmarshal(req.Params, &query); err != nil {
			return PluginResponse{
				Error: &PluginError{
					Code:    "bad_request",
					Message: fmt.Sprintf("Invalid query parameters: %v", err),
				},
			}
		}

		teams, err := provider.Query(ctx, query)
		if err != nil {
			return PluginResponse{
				Error: &PluginError{
					Code:    "provider_error",
					Message: err.Error(),
				},
			}
		}

		result, _ := json.Marshal(teams)
		return PluginResponse{Result: result}

	case "team.get":
		var params struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return PluginResponse{
				Error: &PluginError{
					Code:    "bad_request",
					Message: fmt.Sprintf("Invalid parameters: %v", err),
				},
			}
		}

		team, err := provider.Get(ctx, params.ID)
		if err != nil {
			return PluginResponse{
				Error: &PluginError{
					Code:    "provider_error",
					Message: err.Error(),
				},
			}
		}

		result, _ := json.Marshal(team)
		return PluginResponse{Result: result}

	case "team.members":
		var params struct {
			TeamID string `json:"teamID"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return PluginResponse{
				Error: &PluginError{
					Code:    "bad_request",
					Message: fmt.Sprintf("Invalid parameters: %v", err),
				},
			}
		}

		members, err := provider.Members(ctx, params.TeamID)
		if err != nil {
			return PluginResponse{
				Error: &PluginError{
					Code:    "provider_error",
					Message: err.Error(),
				},
			}
		}

		result, _ := json.Marshal(members)
		return PluginResponse{Result: result}

	default:
		return PluginResponse{
			Error: &PluginError{
				Code:    "method_not_found",
				Message: fmt.Sprintf("Unknown method: %s", req.Method),
			},
		}
	}
}
