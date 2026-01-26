package memory

import "context"

// LLM is the interface for LLM operations needed by memory service
type LLM interface {
	Extract(ctx context.Context, req ExtractRequest) (ExtractResponse, error)
	Decide(ctx context.Context, req DecideRequest) (DecideResponse, error)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AddRequest struct {
	Message  string                 `json:"message,omitempty"`
	Messages []Message              `json:"messages,omitempty"`
	UserID   string                 `json:"user_id,omitempty"`
	AgentID  string                 `json:"agent_id,omitempty"`
	RunID    string                 `json:"run_id,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
	Infer    *bool                  `json:"infer,omitempty"`
}

type SearchRequest struct {
	Query   string                 `json:"query"`
	UserID  string                 `json:"user_id,omitempty"`
	AgentID string                 `json:"agent_id,omitempty"`
	RunID   string                 `json:"run_id,omitempty"`
	Limit   int                    `json:"limit,omitempty"`
	Filters map[string]interface{} `json:"filters,omitempty"`
	Sources []string               `json:"sources,omitempty"`
}

type UpdateRequest struct {
	MemoryID string `json:"memory_id"`
	Memory   string `json:"memory"`
}

type GetAllRequest struct {
	UserID  string `json:"user_id,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	RunID   string `json:"run_id,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type DeleteAllRequest struct {
	UserID  string `json:"user_id,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	RunID   string `json:"run_id,omitempty"`
}

type EmbedInput struct {
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
}

type EmbedUpsertRequest struct {
	Type     string                 `json:"type"`
	Provider string                 `json:"provider,omitempty"`
	Model    string                 `json:"model,omitempty"`
	Input    EmbedInput             `json:"input"`
	Source   string                 `json:"source,omitempty"`
	UserID   string                 `json:"user_id,omitempty"`
	AgentID  string                 `json:"agent_id,omitempty"`
	RunID    string                 `json:"run_id,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
}

type EmbedUpsertResponse struct {
	Item       MemoryItem `json:"item"`
	Provider   string     `json:"provider"`
	Model      string     `json:"model"`
	Dimensions int        `json:"dimensions"`
}

type MemoryItem struct {
	ID        string                 `json:"id"`
	Memory    string                 `json:"memory"`
	Hash      string                 `json:"hash,omitempty"`
	CreatedAt string                 `json:"createdAt,omitempty"`
	UpdatedAt string                 `json:"updatedAt,omitempty"`
	Score     float64                `json:"score,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	UserID    string                 `json:"userId,omitempty"`
	AgentID   string                 `json:"agentId,omitempty"`
	RunID     string                 `json:"runId,omitempty"`
}

type SearchResponse struct {
	Results   []MemoryItem `json:"results"`
	Relations []any        `json:"relations,omitempty"`
}

type DeleteResponse struct {
	Message string `json:"message"`
}

type ExtractRequest struct {
	Messages []Message              `json:"messages"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ExtractResponse struct {
	Facts []string `json:"facts"`
}

type CandidateMemory struct {
	ID       string                 `json:"id"`
	Memory   string                 `json:"memory"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type DecideRequest struct {
	Facts      []string               `json:"facts"`
	Candidates []CandidateMemory      `json:"candidates"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type DecisionAction struct {
	Event     string `json:"event"`
	ID        string `json:"id,omitempty"`
	Text      string `json:"text"`
	OldMemory string `json:"old_memory,omitempty"`
}

type DecideResponse struct {
	Actions []DecisionAction `json:"actions"`
}
