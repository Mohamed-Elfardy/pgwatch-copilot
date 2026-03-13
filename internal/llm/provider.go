package llm

import "context"

// Message represents a single message in a conversation
type Message struct {
	Role    string // "user" or "assistant"
	Content string
}

// Response is what the LLM will return
type Response struct {
	Content string
}

// Provider is the common interface for all LLM providers (Gemini, OpenAI, Claude, etc.)
type Provider interface {
	Complete(ctx context.Context, system string, messages []Message) (*Response, error)
	Name() string
}
