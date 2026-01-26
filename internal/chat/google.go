package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/firebase/genkit/go/genkit"
)

// GoogleProvider wraps Genkit's Google AI provider
type GoogleProvider struct {
	g       *genkit.Genkit
	apiKey  string
	timeout time.Duration
}

func NewGoogleProvider(apiKey string, timeout time.Duration) (*GoogleProvider, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &GoogleProvider{
		apiKey:  apiKey,
		timeout: timeout,
	}, nil
}

func (p *GoogleProvider) Chat(ctx context.Context, req Request) (Result, error) {
	return Result{}, fmt.Errorf("google provider not yet implemented")
}

func (p *GoogleProvider) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, <-chan error) {
	chunkChan := make(chan StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)
		errChan <- fmt.Errorf("google streaming not yet implemented")
	}()

	return chunkChan, errChan
}
