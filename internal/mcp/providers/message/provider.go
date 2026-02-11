package message

import (
	"context"
	"log/slog"
	"strings"

	"github.com/memohai/memoh/internal/channel"
	mcpgw "github.com/memohai/memoh/internal/mcp"
)

const toolSendMessage = "send_message"

type Sender interface {
	Send(ctx context.Context, botID string, channelType channel.ChannelType, req channel.SendRequest) error
}

type ChannelTypeResolver interface {
	ParseChannelType(raw string) (channel.ChannelType, error)
}

type Executor struct {
	sender   Sender
	resolver ChannelTypeResolver
	logger   *slog.Logger
}

func NewExecutor(log *slog.Logger, sender Sender, resolver ChannelTypeResolver) *Executor {
	if log == nil {
		log = slog.Default()
	}
	return &Executor{
		sender:   sender,
		resolver: resolver,
		logger:   log.With(slog.String("provider", "message_tool")),
	}
}

func (p *Executor) ListTools(ctx context.Context, session mcpgw.ToolSessionContext) ([]mcpgw.ToolDescriptor, error) {
	if p.sender == nil || p.resolver == nil {
		return []mcpgw.ToolDescriptor{}, nil
	}
	return []mcpgw.ToolDescriptor{
		{
			Name:        toolSendMessage,
			Description: "Send a message to a channel or session",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"bot_id": map[string]any{
						"type":        "string",
						"description": "Bot ID, optional and defaults to current bot",
					},
					"platform": map[string]any{
						"type":        "string",
						"description": "Channel platform name",
					},
					"target": map[string]any{
						"type":        "string",
						"description": "Channel target (chat/group/thread ID)",
					},
					"channel_identity_id": map[string]any{
						"type":        "string",
						"description": "Target identity ID when direct target is absent",
					},
					"to_user_id": map[string]any{
						"type":        "string",
						"description": "Alias for channel_identity_id",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "Message text content",
					},
				},
				"required": []string{"message"},
			},
		},
	}, nil
}

func (p *Executor) CallTool(ctx context.Context, session mcpgw.ToolSessionContext, toolName string, arguments map[string]any) (map[string]any, error) {
	if toolName != toolSendMessage {
		return nil, mcpgw.ErrToolNotFound
	}
	if p.sender == nil || p.resolver == nil {
		return mcpgw.BuildToolErrorResult("message service not available"), nil
	}

	botID := mcpgw.FirstStringArg(arguments, "bot_id")
	if botID == "" {
		botID = strings.TrimSpace(session.BotID)
	}
	if botID == "" {
		return mcpgw.BuildToolErrorResult("bot_id is required"), nil
	}
	if strings.TrimSpace(session.BotID) != "" && botID != strings.TrimSpace(session.BotID) {
		return mcpgw.BuildToolErrorResult("bot_id mismatch"), nil
	}

	platform := mcpgw.FirstStringArg(arguments, "platform")
	if platform == "" {
		platform = strings.TrimSpace(session.CurrentPlatform)
	}
	if platform == "" {
		return mcpgw.BuildToolErrorResult("platform is required"), nil
	}
	channelType, err := p.resolver.ParseChannelType(platform)
	if err != nil {
		return mcpgw.BuildToolErrorResult(err.Error()), nil
	}

	messageText := mcpgw.FirstStringArg(arguments, "message")
	if messageText == "" {
		return mcpgw.BuildToolErrorResult("message is required"), nil
	}

	target := mcpgw.FirstStringArg(arguments, "target")
	if target == "" {
		target = strings.TrimSpace(session.ReplyTarget)
	}
	channelIdentityID := mcpgw.FirstStringArg(arguments, "channel_identity_id", "to_user_id")
	if target == "" && channelIdentityID == "" {
		return mcpgw.BuildToolErrorResult("target or channel_identity_id is required"), nil
	}

	sendReq := channel.SendRequest{
		Target:            target,
		ChannelIdentityID: channelIdentityID,
		Message: channel.Message{
			Text: messageText,
		},
	}
	if err := p.sender.Send(ctx, botID, channelType, sendReq); err != nil {
		p.logger.Warn("send message failed", slog.Any("error", err), slog.String("bot_id", botID), slog.String("platform", platform))
		return mcpgw.BuildToolErrorResult(err.Error()), nil
	}

	payload := map[string]any{
		"ok":                  true,
		"bot_id":              botID,
		"platform":            channelType.String(),
		"target":              target,
		"channel_identity_id": channelIdentityID,
		"instruction":         "Message delivered successfully. You have completed your response. Please STOP now and do not call any more tools.",
	}
	return mcpgw.BuildToolSuccessResult(payload), nil
}
