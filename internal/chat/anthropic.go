package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/firebase/genkit/go/genkit"
)

// AnthropicProvider wraps Genkit's Anthropic provider
type AnthropicProvider struct {
	g       *genkit.Genkit
	apiKey  string
	timeout time.Duration
}

func NewAnthropicProvider(apiKey string, timeout time.Duration) (*AnthropicProvider, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &AnthropicProvider{
		apiKey:  apiKey,
		timeout: timeout,
	}, nil
}

func (p *AnthropicProvider) Chat(ctx context.Context, req Request) (Result, error) {
	return Result{}, fmt.Errorf("anthropic provider not yet implemented")
}

func (p *AnthropicProvider) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, <-chan error) {
	chunkChan := make(chan StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)
		errChan <- fmt.Errorf("anthropic streaming not yet implemented")
	}()

	return chunkChan, errChan
}
