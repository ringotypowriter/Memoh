package feishu

import (
	"context"
	"testing"

	"github.com/memohai/memoh/internal/channel"
)

func TestExtractReadableFromJSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "hello world", "hello world"},
		{"json with text", `{"text":"extracted"}`, "extracted"},
		{"json with message", `{"message":"ok"}`, "ok"},
		{"json with content", `{"content":"result"}`, "result"},
		{"invalid json", `{invalid`, `{invalid`},
		{"empty object", `{}`, `{}`},
		{"array of strings", `["first"]`, "first"},
		{"array empty", `[]`, `[]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractReadableFromJSON(tc.in)
			if got != tc.want {
				t.Errorf("extractReadableFromJSON(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFeishuOutboundStreamSkipsAttachmentsAlreadySentByMessageTool(t *testing.T) {
	t.Parallel()

	var sent []channel.OutboundMessage
	stream := &feishuOutboundStream{
		target: "chat_id:oc_current",
		send: func(_ context.Context, msg channel.OutboundMessage) error {
			sent = append(sent, msg)
			return nil
		},
	}

	ctx := context.Background()
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventToolCallStart,
		ToolCall: &channel.StreamToolCall{
			Name: "send",
			Input: map[string]any{
				"platform":    "feishu",
				"target":      "chat_id:oc_current",
				"attachments": []any{"/data/media/tool/file.txt"},
			},
		},
	}); err != nil {
		t.Fatalf("push tool_call_start: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventToolCallEnd,
		ToolCall: &channel.StreamToolCall{
			Name: "send",
			Input: map[string]any{
				"platform":    "feishu",
				"target":      "chat_id:oc_current",
				"attachments": []any{"/data/media/tool/file.txt"},
			},
			Result: map[string]any{
				"structuredContent": map[string]any{
					"ok": true,
				},
			},
		},
	}); err != nil {
		t.Fatalf("push tool_call_end: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventAttachment,
		Attachments: []channel.Attachment{{
			Type: channel.AttachmentFile,
			URL:  "/data/media/tool/file.txt",
		}},
	}); err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type:  channel.StreamEventFinal,
		Final: &channel.StreamFinalizePayload{},
		Metadata: map[string]any{
			"tool_sent_attachment_keys": []any{"ref:/data/media/tool/file.txt"},
		},
	}); err != nil {
		t.Fatalf("push final: %v", err)
	}

	if len(sent) != 0 {
		t.Fatalf("expected tool-sent attachment to be skipped, got %d sends", len(sent))
	}
}

func TestFeishuOutboundStreamBuffersAttachmentsBeforeCurrentConversationSendTool(t *testing.T) {
	t.Parallel()

	var sent []channel.OutboundMessage
	stream := &feishuOutboundStream{
		target: "chat_id:oc_current",
		send: func(_ context.Context, msg channel.OutboundMessage) error {
			sent = append(sent, msg)
			return nil
		},
	}

	ctx := context.Background()
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventAttachment,
		Attachments: []channel.Attachment{{
			Type: channel.AttachmentFile,
			URL:  "/data/media/tool/file.txt",
		}},
	}); err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventToolCallStart,
		ToolCall: &channel.StreamToolCall{
			Name: "send",
			Input: map[string]any{
				"platform":    "feishu",
				"target":      "chat_id:oc_current",
				"attachments": []any{"/data/media/tool/file.txt"},
			},
		},
	}); err != nil {
		t.Fatalf("push tool_call_start: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventToolCallEnd,
		ToolCall: &channel.StreamToolCall{
			Name: "send",
			Input: map[string]any{
				"platform":    "feishu",
				"target":      "chat_id:oc_current",
				"attachments": []any{"/data/media/tool/file.txt"},
			},
			Result: map[string]any{
				"structuredContent": map[string]any{
					"ok": true,
				},
			},
		},
	}); err != nil {
		t.Fatalf("push tool_call_end: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type:   channel.StreamEventStatus,
		Status: channel.StreamStatusCompleted,
	}); err != nil {
		t.Fatalf("push completed: %v", err)
	}

	if len(sent) != 0 {
		t.Fatalf("expected buffered attachment to be dropped after current-conversation send tool, got %d sends", len(sent))
	}
}

func TestFeishuOutboundStreamKeepsAttachmentsForOtherTargets(t *testing.T) {
	t.Parallel()

	var sent []channel.OutboundMessage
	stream := &feishuOutboundStream{
		target: "chat_id:oc_current",
		send: func(_ context.Context, msg channel.OutboundMessage) error {
			sent = append(sent, msg)
			return nil
		},
	}

	ctx := context.Background()
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventToolCallStart,
		ToolCall: &channel.StreamToolCall{
			Name: "send",
			Input: map[string]any{
				"platform":    "feishu",
				"target":      "chat_id:oc_other",
				"attachments": []any{"/data/media/tool/file.txt"},
			},
		},
	}); err != nil {
		t.Fatalf("push tool_call_start: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventToolCallEnd,
		ToolCall: &channel.StreamToolCall{
			Name: "send",
			Input: map[string]any{
				"platform":    "feishu",
				"target":      "chat_id:oc_other",
				"attachments": []any{"/data/media/tool/file.txt"},
			},
			Result: map[string]any{
				"structuredContent": map[string]any{
					"ok": true,
				},
			},
		},
	}); err != nil {
		t.Fatalf("push tool_call_end: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventAttachment,
		Attachments: []channel.Attachment{{
			Type: channel.AttachmentFile,
			URL:  "/data/media/tool/file.txt",
		}},
	}); err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type:  channel.StreamEventFinal,
		Final: &channel.StreamFinalizePayload{},
	}); err != nil {
		t.Fatalf("push final: %v", err)
	}

	if len(sent) != 1 {
		t.Fatalf("expected attachment for other target to be sent once, got %d sends", len(sent))
	}
	if len(sent[0].Message.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %d", len(sent[0].Message.Attachments))
	}
}

func TestFeishuOutboundStreamKeepsAttachmentsWhenMessageToolFails(t *testing.T) {
	t.Parallel()

	var sent []channel.OutboundMessage
	stream := &feishuOutboundStream{
		target: "chat_id:oc_current",
		send: func(_ context.Context, msg channel.OutboundMessage) error {
			sent = append(sent, msg)
			return nil
		},
	}

	ctx := context.Background()
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventToolCallStart,
		ToolCall: &channel.StreamToolCall{
			Name: "send",
			Input: map[string]any{
				"platform":    "feishu",
				"target":      "chat_id:oc_current",
				"attachments": []any{"/data/media/tool/file.txt"},
			},
		},
	}); err != nil {
		t.Fatalf("push tool_call_start: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventToolCallEnd,
		ToolCall: &channel.StreamToolCall{
			Name: "send",
			Input: map[string]any{
				"platform":    "feishu",
				"target":      "chat_id:oc_current",
				"attachments": []any{"/data/media/tool/file.txt"},
			},
			Result: map[string]any{
				"isError": true,
				"content": []any{
					map[string]any{
						"type": "text",
						"text": "send failed",
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("push tool_call_end: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type: channel.StreamEventAttachment,
		Attachments: []channel.Attachment{{
			Type: channel.AttachmentFile,
			URL:  "/data/media/tool/file.txt",
		}},
	}); err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	if err := stream.Push(ctx, channel.StreamEvent{
		Type:  channel.StreamEventFinal,
		Final: &channel.StreamFinalizePayload{},
	}); err != nil {
		t.Fatalf("push final: %v", err)
	}

	if len(sent) != 1 {
		t.Fatalf("expected failed tool call attachment to be sent once, got %d sends", len(sent))
	}
	if len(sent[0].Message.Attachments) != 1 {
		t.Fatalf("expected one attachment after failed tool call, got %d", len(sent[0].Message.Attachments))
	}
}
