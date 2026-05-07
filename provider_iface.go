package enno

import "context"

type Request struct {
	SystemPrompt string
	Messages     []Message
	Tools        []Tool
}

type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
}
