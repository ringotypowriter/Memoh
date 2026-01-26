package chat

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // user, assistant, system
	Content string `json:"content"`
}

// ChatRequest represents an incoming chat request
type ChatRequest struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model,omitempty"`    // optional: specific model to use
	Provider string    `json:"provider,omitempty"` // optional: specific provider to use
	Stream   bool      `json:"stream,omitempty"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
	Message      Message `json:"message"`
	Model        string  `json:"model"`
	Provider     string  `json:"provider"`
	FinishReason string  `json:"finish_reason,omitempty"`
	Usage        Usage   `json:"usage,omitempty"`
}

// StreamChunk represents a chunk in streaming response
type StreamChunk struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	Model        string `json:"model,omitempty"`
	Provider     string `json:"provider,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Request is the internal request structure
type Request struct {
	Messages       []Message
	Model          string
	Provider       string
	Temperature    *float32          // optional temperature
	ResponseFormat *ResponseFormat   // optional response format
	MaxTokens      *int              // optional max tokens
}

// ResponseFormat specifies the format of the response
type ResponseFormat struct {
	Type string `json:"type"` // "json_object" or "text"
}

// Result is the internal result structure
type Result struct {
	Message      Message
	Model        string
	Provider     string
	FinishReason string
	Usage        Usage
}
