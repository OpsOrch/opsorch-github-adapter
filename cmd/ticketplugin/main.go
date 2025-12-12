package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-github-adapter/ticket"
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
		fmt.Println("opsorch-github-ticket-plugin v1.0.0")
		return
	}

	var provider *ticket.Provider

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
			p, err := ticket.New(req.Config)
			if err != nil {
				writeErr(err)
				continue
			}
			if githubProvider, ok := p.(*ticket.Provider); ok {
				provider = githubProvider
			} else {
				writeErr(fmt.Errorf("failed to create GitHub ticket provider"))
				continue
			}
		}

		ctx := context.Background()

		switch req.Method {
		case "ticket.query":
			var query schema.TicketQuery
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

		case "ticket.get":
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

		case "ticket.create":
			var input schema.CreateTicketInput
			if err := json.Unmarshal(req.Payload, &input); err != nil {
				writeErr(err)
				continue
			}
			result, err := provider.Create(ctx, input)
			if err != nil {
				writeErr(err)
				continue
			}
			writeOK(result)

		case "ticket.update":
			var payload struct {
				ID    string                   `json:"id"`
				Input schema.UpdateTicketInput `json:"input"`
			}
			if err := json.Unmarshal(req.Payload, &payload); err != nil {
				writeErr(err)
				continue
			}
			result, err := provider.Update(ctx, payload.ID, payload.Input)
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
