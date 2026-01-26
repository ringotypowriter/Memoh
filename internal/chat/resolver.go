package chat

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/memohai/memoh/internal/db/sqlc"
	"github.com/memohai/memoh/internal/models"
)

const (
	ProviderOpenAI       = "openai"
	ProviderOpenAICompat = "openai-compat"
	ProviderAnthropic    = "anthropic"
	ProviderGoogle       = "google"
	ProviderOllama       = "ollama"
)

// Provider interface for chat providers
type Provider interface {
	Chat(ctx context.Context, req Request) (Result, error)
	StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, <-chan error)
}

// Resolver resolves chat models and providers
type Resolver struct {
	modelsService *models.Service
	queries       *sqlc.Queries
	timeout       time.Duration
}

// NewResolver creates a new chat resolver
func NewResolver(modelsService *models.Service, queries *sqlc.Queries, timeout time.Duration) *Resolver {
	return &Resolver{
		modelsService: modelsService,
		queries:       queries,
		timeout:       timeout,
	}
}

// Chat performs a chat completion
func (r *Resolver) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	// Select model and provider
	selected, provider, err := r.selectChatModel(ctx, req.Model, req.Provider)
	if err != nil {
		return ChatResponse{}, err
	}

	// Create internal request
	internalReq := Request{
		Messages: req.Messages,
		Model:    selected.ModelID,
		Provider: strings.ToLower(provider.ClientType),
	}

	// Add system prompt
	if len(internalReq.Messages) > 0 && internalReq.Messages[0].Role != "system" {
		systemPrompt := SystemPrompt(PromptParams{
			Date:               time.Now(),
			Locale:             "en-US",
			Language:           "Same as user input",
			MaxContextLoadTime: 24 * 60, // 24 hours
			Platforms:          []string{},
			CurrentPlatform:    "api",
		})
		internalReq.Messages = append([]Message{
			{Role: "system", Content: systemPrompt},
		}, internalReq.Messages...)
	}

	// Create provider instance
	providerInst, err := r.createProvider(provider)
	if err != nil {
		return ChatResponse{}, err
	}

	// Execute chat
	result, err := providerInst.Chat(ctx, internalReq)
	if err != nil {
		return ChatResponse{}, err
	}

	return ChatResponse{
		Message:      result.Message,
		Model:        result.Model,
		Provider:     result.Provider,
		FinishReason: result.FinishReason,
		Usage:        result.Usage,
	}, nil
}

// StreamChat performs a streaming chat completion
func (r *Resolver) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamChunk, <-chan error) {
	chunkChan := make(chan StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		// Select model and provider
		selected, provider, err := r.selectChatModel(ctx, req.Model, req.Provider)
		if err != nil {
			errChan <- err
			return
		}

		// Create internal request
		internalReq := Request{
			Messages: req.Messages,
			Model:    selected.ModelID,
			Provider: strings.ToLower(provider.ClientType),
		}

		// Add system prompt
		if len(internalReq.Messages) > 0 && internalReq.Messages[0].Role != "system" {
			systemPrompt := SystemPrompt(PromptParams{
				Date:               time.Now(),
				Locale:             "en-US",
				Language:           "Same as user input",
				MaxContextLoadTime: 24 * 60, // 24 hours
				Platforms:          []string{},
				CurrentPlatform:    "api",
			})
			internalReq.Messages = append([]Message{
				{Role: "system", Content: systemPrompt},
			}, internalReq.Messages...)
		}

		// Create provider instance
		providerInst, err := r.createProvider(provider)
		if err != nil {
			errChan <- err
			return
		}

		// Execute streaming chat
		providerChunkChan, providerErrChan := providerInst.StreamChat(ctx, internalReq)

		// Forward chunks and errors
		for {
			select {
			case chunk, ok := <-providerChunkChan:
				if !ok {
					return
				}
				chunkChan <- chunk
			case err := <-providerErrChan:
				if err != nil {
					errChan <- err
				}
				return
			}
		}
	}()

	return chunkChan, errChan
}

// selectChatModel selects a chat model based on the request
func (r *Resolver) selectChatModel(ctx context.Context, modelID, providerType string) (models.GetResponse, sqlc.LlmProvider, error) {
	if r.modelsService == nil {
		return models.GetResponse{}, sqlc.LlmProvider{}, errors.New("models service not configured")
	}

	modelID = strings.TrimSpace(modelID)
	providerType = strings.ToLower(strings.TrimSpace(providerType))

	// If no model specified, try to get default chat model
	if modelID == "" && providerType == "" {
		defaultModel, err := r.modelsService.GetByEnableAs(ctx, models.EnableAsChat)
		if err == nil {
			provider, err := r.fetchProvider(ctx, defaultModel.LlmProviderID)
			if err != nil {
				return models.GetResponse{}, sqlc.LlmProvider{}, err
			}
			return defaultModel, provider, nil
		}
	}

	// List available models
	var candidates []models.GetResponse
	var err error
	if providerType != "" {
		candidates, err = r.modelsService.ListByClientType(ctx, models.ClientType(providerType))
	} else {
		candidates, err = r.modelsService.ListByType(ctx, models.ModelTypeChat)
	}
	if err != nil {
		return models.GetResponse{}, sqlc.LlmProvider{}, err
	}

	// Filter chat models
	filtered := make([]models.GetResponse, 0, len(candidates))
	for _, model := range candidates {
		if model.Type != models.ModelTypeChat {
			continue
		}
		filtered = append(filtered, model)
	}
	if len(filtered) == 0 {
		return models.GetResponse{}, sqlc.LlmProvider{}, errors.New("no chat models available")
	}

	// If model specified, find it
	if modelID != "" {
		for _, model := range filtered {
			if model.ModelID == modelID {
				provider, err := r.fetchProvider(ctx, model.LlmProviderID)
				if err != nil {
					return models.GetResponse{}, sqlc.LlmProvider{}, err
				}
				return model, provider, nil
			}
		}
		return models.GetResponse{}, sqlc.LlmProvider{}, errors.New("chat model not found")
	}

	// Return first available model
	selected := filtered[0]
	provider, err := r.fetchProvider(ctx, selected.LlmProviderID)
	if err != nil {
		return models.GetResponse{}, sqlc.LlmProvider{}, err
	}
	return selected, provider, nil
}

// fetchProvider fetches provider information from database
func (r *Resolver) fetchProvider(ctx context.Context, providerID string) (sqlc.LlmProvider, error) {
	if r.queries == nil {
		return sqlc.LlmProvider{}, errors.New("llm provider queries not configured")
	}
	if strings.TrimSpace(providerID) == "" {
		return sqlc.LlmProvider{}, errors.New("llm provider id missing")
	}
	parsed, err := uuid.Parse(providerID)
	if err != nil {
		return sqlc.LlmProvider{}, err
	}
	pgID := pgtype.UUID{Valid: true}
	copy(pgID.Bytes[:], parsed[:])
	return r.queries.GetLlmProviderByID(ctx, pgID)
}

// createProvider creates a provider instance based on configuration
func (r *Resolver) createProvider(provider sqlc.LlmProvider) (Provider, error) {
	clientType := strings.ToLower(strings.TrimSpace(provider.ClientType))
	timeout := r.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	switch clientType {
	case ProviderOpenAI, ProviderOpenAICompat:
		if strings.TrimSpace(provider.ApiKey) == "" {
			return nil, errors.New("openai api key is required")
		}
		p, err := NewOpenAIProvider(provider.ApiKey, provider.BaseUrl, timeout)
		if err != nil {
			return nil, err
		}
		return p, nil
	case ProviderAnthropic:
		if strings.TrimSpace(provider.ApiKey) == "" {
			return nil, errors.New("anthropic api key is required")
		}
		p, err := NewAnthropicProvider(provider.ApiKey, timeout)
		if err != nil {
			return nil, err
		}
		return p, nil
	case ProviderGoogle:
		if strings.TrimSpace(provider.ApiKey) == "" {
			return nil, errors.New("google api key is required")
		}
		p, err := NewGoogleProvider(provider.ApiKey, timeout)
		if err != nil {
			return nil, err
		}
		return p, nil
	case ProviderOllama:
		p, err := NewOllamaProvider(provider.BaseUrl, timeout)
		if err != nil {
			return nil, err
		}
		return p, nil
	default:
		return nil, errors.New("unsupported provider type: " + clientType)
	}
}
