package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/firebase/genkit/go/genkit"
)

// OllamaProvider wraps Genkit's Ollama provider
type OllamaProvider struct {
	g       *genkit.Genkit
	baseURL string
	timeout time.Duration
}

func NewOllamaProvider(baseURL string, timeout time.Duration) (*OllamaProvider, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &OllamaProvider{
		baseURL: baseURL,
		timeout: timeout,
	}, nil
}

func (p *OllamaProvider) Chat(ctx context.Context, req Request) (Result, error) {
	return Result{}, fmt.Errorf("ollama provider not yet implemented")
}

func (p *OllamaProvider) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, <-chan error) {
	chunkChan := make(chan StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)
		errChan <- fmt.Errorf("ollama streaming not yet implemented")
	}()

	return chunkChan, errChan
}
