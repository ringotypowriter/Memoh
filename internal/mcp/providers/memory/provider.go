package memory

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/memohai/memoh/internal/chat"
	mcpgw "github.com/memohai/memoh/internal/mcp"
	mem "github.com/memohai/memoh/internal/memory"
)

const (
	toolSearchMemory       = "search_memory"
	defaultMemoryToolLimit = 8
	maxMemoryToolLimit     = 50
)

type MemorySearcher interface {
	Search(ctx context.Context, req mem.SearchRequest) (mem.SearchResponse, error)
}

type ChatAccessor interface {
	Get(ctx context.Context, chatID string) (chat.Chat, error)
	GetSettings(ctx context.Context, chatID string) (chat.Settings, error)
	IsParticipant(ctx context.Context, chatID, channelIdentityID string) (bool, error)
}

type AdminChecker interface {
	IsAdmin(ctx context.Context, channelIdentityID string) (bool, error)
}

type Executor struct {
	searcher     MemorySearcher
	chatAccessor ChatAccessor
	adminChecker AdminChecker
	logger       *slog.Logger
}

func NewExecutor(log *slog.Logger, searcher MemorySearcher, chatAccessor ChatAccessor, adminChecker AdminChecker) *Executor {
	if log == nil {
		log = slog.Default()
	}
	return &Executor{
		searcher:     searcher,
		chatAccessor: chatAccessor,
		adminChecker: adminChecker,
		logger:       log.With(slog.String("provider", "memory_tool")),
	}
}

func (p *Executor) ListTools(ctx context.Context, session mcpgw.ToolSessionContext) ([]mcpgw.ToolDescriptor, error) {
	if p.searcher == nil || p.chatAccessor == nil {
		return []mcpgw.ToolDescriptor{}, nil
	}
	return []mcpgw.ToolDescriptor{
		{
			Name:        toolSearchMemory,
			Description: "Search for memories relevant to the current chat",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The query to search memories",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of memory results",
					},
				},
				"required": []string{"query"},
			},
		},
	}, nil
}

func (p *Executor) CallTool(ctx context.Context, session mcpgw.ToolSessionContext, toolName string, arguments map[string]any) (map[string]any, error) {
	if toolName != toolSearchMemory {
		return nil, mcpgw.ErrToolNotFound
	}
	if p.searcher == nil || p.chatAccessor == nil {
		return mcpgw.BuildToolErrorResult("memory service not available"), nil
	}

	query := mcpgw.StringArg(arguments, "query")
	if query == "" {
		return mcpgw.BuildToolErrorResult("query is required"), nil
	}
	botID := strings.TrimSpace(session.BotID)
	chatID := strings.TrimSpace(session.ChatID)
	channelIdentityID := strings.TrimSpace(session.ChannelIdentityID)
	if botID == "" || chatID == "" {
		return mcpgw.BuildToolErrorResult("bot_id and chat_id are required"), nil
	}

	limit := defaultMemoryToolLimit
	if value, ok, err := mcpgw.IntArg(arguments, "limit"); err != nil {
		return mcpgw.BuildToolErrorResult(err.Error()), nil
	} else if ok {
		limit = value
	}
	if limit <= 0 {
		limit = defaultMemoryToolLimit
	}
	if limit > maxMemoryToolLimit {
		limit = maxMemoryToolLimit
	}

	chatObj, err := p.chatAccessor.Get(ctx, chatID)
	if err != nil {
		return mcpgw.BuildToolErrorResult("chat not found"), nil
	}
	if strings.TrimSpace(chatObj.BotID) != botID {
		return mcpgw.BuildToolErrorResult("bot mismatch"), nil
	}
	if channelIdentityID != "" {
		allowed, err := p.canAccessChat(ctx, chatID, channelIdentityID)
		if err != nil {
			return mcpgw.BuildToolErrorResult(err.Error()), nil
		}
		if !allowed {
			return mcpgw.BuildToolErrorResult("not a chat participant"), nil
		}
	}

	settings, err := p.chatAccessor.GetSettings(ctx, chatID)
	if err != nil {
		return mcpgw.BuildToolErrorResult(err.Error()), nil
	}

	type memoryScope struct {
		namespace string
		scopeID   string
	}
	scopes := make([]memoryScope, 0, 3)
	if settings.EnableChatMemory {
		scopes = append(scopes, memoryScope{namespace: "chat", scopeID: chatID})
	}
	if settings.EnablePrivateMemory && channelIdentityID != "" {
		scopes = append(scopes, memoryScope{namespace: "private", scopeID: channelIdentityID})
	}
	if settings.EnablePublicMemory {
		scopes = append(scopes, memoryScope{namespace: "public", scopeID: botID})
	}
	if len(scopes) == 0 {
		scopes = append(scopes, memoryScope{namespace: "chat", scopeID: chatID})
	}

	allResults := make([]mem.MemoryItem, 0, len(scopes)*limit)
	for _, scope := range scopes {
		resp, err := p.searcher.Search(ctx, mem.SearchRequest{
			Query: query,
			BotID: botID,
			Limit: limit,
			Filters: map[string]any{
				"namespace": scope.namespace,
				"scopeId":   scope.scopeID,
				"botId":     botID,
			},
		})
		if err != nil {
			p.logger.Warn("memory search namespace failed", slog.String("namespace", scope.namespace), slog.Any("error", err))
			continue
		}
		allResults = append(allResults, resp.Results...)
	}

	allResults = deduplicateMemoryItems(allResults)
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})
	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	results := make([]map[string]any, 0, len(allResults))
	for _, item := range allResults {
		results = append(results, map[string]any{
			"id":     item.ID,
			"memory": item.Memory,
			"score":  item.Score,
		})
	}

	return mcpgw.BuildToolSuccessResult(map[string]any{
		"query":   query,
		"total":   len(results),
		"results": results,
	}), nil
}

func (p *Executor) canAccessChat(ctx context.Context, chatID, channelIdentityID string) (bool, error) {
	if p.adminChecker != nil {
		isAdmin, err := p.adminChecker.IsAdmin(ctx, channelIdentityID)
		if err != nil {
			return false, err
		}
		if isAdmin {
			return true, nil
		}
	}
	return p.chatAccessor.IsParticipant(ctx, chatID, channelIdentityID)
}

func deduplicateMemoryItems(items []mem.MemoryItem) []mem.MemoryItem {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]mem.MemoryItem, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = strings.TrimSpace(item.Memory)
		}
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, item)
	}
	return result
}
