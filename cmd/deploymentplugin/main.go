package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-github-adapter/deployment"
)

type rpcRequest struct {
	Method  string          `json:"method"`
	Config  map[string]any  `json:"config"`
	Payload json.RawMessage `json:"payload"`
}

type rpcResponse struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("opsorch-github-deployment-plugin v1.0.0")
		return
	}

	var provider *deployment.Provider

	dec := json.NewDecoder(os.Stdin)
	for {
		var req rpcRequest
		if err := dec.Decode(&req); err != nil {
			if err.Error() == "EOF" {
				return
			}
			writeErr(err)
			return
		}

		// Initialize provider if not already done
		if provider == nil {
			p, err := deployment.New(req.Config)
			if err != nil {
				writeErr(err)
				continue
			}
			if githubProvider, ok := p.(*deployment.Provider); ok {
				provider = githubProvider
			} else {
				writeErr(fmt.Errorf("failed to create GitHub deployment provider"))
				continue
			}
		}

		ctx := context.Background()

		switch req.Method {
		case "deployment.query":
			var query schema.DeploymentQuery
			if err := json.Unmarshal(req.Payload, &query); err != nil {
				writeErr(err)
				continue
			}
			result, err := provider.Query(ctx, query)
			if err != nil {
				writeErr(err)
				continue
			}
			writeOK(result)

		case "deployment.get":
			var payload struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(req.Payload, &payload); err != nil {
				writeErr(err)
				continue
			}
			result, err := provider.Get(ctx, payload.ID)
			if err != nil {
				writeErr(err)
				continue
			}
			writeOK(result)

		default:
			writeErr(fmt.Errorf("unknown method: %s", req.Method))
		}
	}
}

func writeOK(result any) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(rpcResponse{Result: result})
}

func writeErr(err error) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(rpcResponse{Error: err.Error()})
}
