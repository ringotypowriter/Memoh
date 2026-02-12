package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/memohai/memoh/internal/bots"
)

const (
	mcpCheckTimeout = 8 * time.Second
)

// ConnectionChecker implements bots.RuntimeChecker for MCP connections.
type ConnectionChecker struct {
	logger      *slog.Logger
	connections *ConnectionService
	gateway     *ToolGatewayService
}

// NewConnectionChecker creates an MCP runtime checker.
func NewConnectionChecker(log *slog.Logger, connections *ConnectionService, gateway *ToolGatewayService) *ConnectionChecker {
	if log == nil {
		log = slog.Default()
	}
	return &ConnectionChecker{
		logger:      log.With(slog.String("checker", "mcp")),
		connections: connections,
		gateway:     gateway,
	}
}

// CheckBot probes each active MCP connection for a bot and returns check results.
func (c *ConnectionChecker) CheckBot(ctx context.Context, botID string) []bots.BotCheck {
	if c.connections == nil {
		return nil
	}
	items, err := c.connections.ListActiveByBot(ctx, botID)
	if err != nil {
		c.logger.Warn("mcp checker: list connections failed",
			slog.String("bot_id", botID), slog.Any("error", err))
		return nil
	}
	if len(items) == 0 {
		return nil
	}

	checks := make([]bots.BotCheck, 0, len(items))
	for _, conn := range items {
		check := c.probeConnection(ctx, botID, conn)
		checks = append(checks, check)
	}
	return checks
}

func (c *ConnectionChecker) probeConnection(ctx context.Context, botID string, conn Connection) bots.BotCheck {
	checkKey := "mcp." + sanitizeCheckKey(conn.Name)
	check := bots.BotCheck{
		CheckKey: checkKey,
		Status:   bots.BotCheckStatusUnknown,
		Summary:  fmt.Sprintf("MCP server %q is being checked.", conn.Name),
		Metadata: map[string]any{
			"connection_id": conn.ID,
			"name":          conn.Name,
			"type":          conn.Type,
		},
	}

	if c.gateway == nil {
		check.Status = bots.BotCheckStatusWarn
		check.Summary = fmt.Sprintf("MCP server %q cannot be checked.", conn.Name)
		check.Detail = "tool gateway not available"
		return check
	}

	probeCtx, cancel := context.WithTimeout(ctx, mcpCheckTimeout)
	defer cancel()

	session := ToolSessionContext{BotID: botID}
	tools, err := c.gateway.ListTools(probeCtx, session)
	if err != nil {
		check.Status = bots.BotCheckStatusError
		check.Summary = fmt.Sprintf("MCP server %q is not reachable.", conn.Name)
		check.Detail = err.Error()
		return check
	}

	// Count tools belonging to this connection (prefixed with connection name).
	prefix := sanitizeCheckKey(conn.Name) + "."
	toolCount := 0
	for _, t := range tools {
		if strings.HasPrefix(t.Name, prefix) || t.Name == conn.Name {
			toolCount++
		}
	}

	if toolCount > 0 {
		check.Status = bots.BotCheckStatusOK
		check.Summary = fmt.Sprintf("MCP server %q is healthy (%d tools).", conn.Name, toolCount)
		check.Metadata["tool_count"] = toolCount
	} else {
		check.Status = bots.BotCheckStatusWarn
		check.Summary = fmt.Sprintf("MCP server %q is reachable but no tools found.", conn.Name)
		check.Detail = "The server responded but exposed no tools."
	}
	return check
}

func sanitizeCheckKey(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "unknown"
	}
	b := strings.Builder{}
	for _, ch := range raw {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			b.WriteRune(ch)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_-")
}
