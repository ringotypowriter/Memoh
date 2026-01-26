package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/memohai/memoh/internal/chat"
)

// ProviderLLMClient uses chat.Provider to make LLM calls for memory operations
type ProviderLLMClient struct {
	provider chat.Provider
	model    string
}

// NewProviderLLMClient creates a new LLM client that uses chat.Provider
func NewProviderLLMClient(provider chat.Provider, model string) *ProviderLLMClient {
	if model == "" {
		model = "gpt-4.1-nano-2025-04-14"
	}
	return &ProviderLLMClient{
		provider: provider,
		model:    model,
	}
}

// Extract extracts facts from messages using the provider
func (c *ProviderLLMClient) Extract(ctx context.Context, req ExtractRequest) (ExtractResponse, error) {
	if len(req.Messages) == 0 {
		return ExtractResponse{}, fmt.Errorf("messages is required")
	}
	
	parsedMessages := parseMessages(formatMessages(req.Messages))
	systemPrompt, userPrompt := getFactRetrievalMessages(parsedMessages)
	
	// Call provider with JSON mode
	temp := float32(0)
	result, err := c.provider.Chat(ctx, chat.Request{
		Model:       c.model,
		Temperature: &temp,
		ResponseFormat: &chat.ResponseFormat{
			Type: "json_object",
		},
		Messages: []chat.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return ExtractResponse{}, err
	}

	content := result.Message.Content
	var parsed ExtractResponse
	if err := json.Unmarshal([]byte(removeCodeBlocks(content)), &parsed); err != nil {
		return ExtractResponse{}, err
	}
	return parsed, nil
}

// Decide decides what actions to take based on facts and existing memories
func (c *ProviderLLMClient) Decide(ctx context.Context, req DecideRequest) (DecideResponse, error) {
	if len(req.Facts) == 0 {
		return DecideResponse{}, fmt.Errorf("facts is required")
	}
	
	retrieved := make([]map[string]string, 0, len(req.Candidates))
	for _, candidate := range req.Candidates {
		retrieved = append(retrieved, map[string]string{
			"id":   candidate.ID,
			"text": candidate.Memory,
		})
	}
	
	prompt := getUpdateMemoryMessages(retrieved, req.Facts)
	
	// Call provider with JSON mode
	temp := float32(0)
	result, err := c.provider.Chat(ctx, chat.Request{
		Model:       c.model,
		Temperature: &temp,
		ResponseFormat: &chat.ResponseFormat{
			Type: "json_object",
		},
		Messages: []chat.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return DecideResponse{}, err
	}

	content := result.Message.Content
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(removeCodeBlocks(content)), &raw); err != nil {
		return DecideResponse{}, err
	}

	memoryItems := normalizeMemoryItems(raw["memory"])
	actions := make([]DecisionAction, 0, len(memoryItems))
	for _, item := range memoryItems {
		event := strings.ToUpper(asString(item["event"]))
		if event == "" {
			event = "ADD"
		}
		if event == "NONE" {
			continue
		}

		text := asString(item["text"])
		if text == "" {
			text = asString(item["fact"])
		}
		if strings.TrimSpace(text) == "" {
			continue
		}

		actions = append(actions, DecisionAction{
			Event:     event,
			ID:        normalizeID(item["id"]),
			Text:      text,
			OldMemory: asString(item["old_memory"]),
		})
	}
	return DecideResponse{Actions: actions}, nil
}

