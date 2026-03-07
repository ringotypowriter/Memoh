package discord

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/memohai/memoh/internal/channel"
)

func TestDiscordOutboundStreamBuffersAttachments(t *testing.T) {
	t.Parallel()

	stream := &discordOutboundStream{
		adapter: &DiscordAdapter{},
	}

	err := stream.Push(context.Background(), channel.StreamEvent{
		Type:        channel.StreamEventAttachment,
		Attachments: []channel.Attachment{{Type: channel.AttachmentImage, URL: "https://example.com/a.png"}},
	})
	if err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	if got := len(stream.attachments); got != 1 {
		t.Fatalf("expected buffered attachment, got %d", got)
	}
}

func TestDiscordOutboundStreamErrorFlushesBufferedAttachments(t *testing.T) {
	t.Parallel()

	flushed := 0
	stream := &discordOutboundStream{
		adapter: &DiscordAdapter{},
		finalizeFn: func(string) error {
			return nil
		},
		sendFn: func(_ context.Context, _ channel.Attachment) error {
			flushed++
			return nil
		},
	}

	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:        channel.StreamEventAttachment,
		Attachments: []channel.Attachment{{Type: channel.AttachmentImage, URL: "https://example.com/a.png"}},
	}); err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:  channel.StreamEventError,
		Error: "boom",
	}); err != nil {
		t.Fatalf("push error: %v", err)
	}
	if flushed != 1 {
		t.Fatalf("expected buffered attachment to flush on error, got %d", flushed)
	}
}

func TestDiscordOutboundStreamFinalPrefersExplicitFinalText(t *testing.T) {
	t.Parallel()

	var finalized string
	stream := &discordOutboundStream{
		adapter:    &DiscordAdapter{},
		lastUpdate: time.Now(),
		finalizeFn: func(text string) error {
			finalized = text
			return nil
		},
	}

	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:  channel.StreamEventDelta,
		Delta: "stale draft",
	}); err != nil {
		t.Fatalf("push delta: %v", err)
	}
	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:     channel.StreamEventToolCallStart,
		ToolCall: &channel.StreamToolCall{Name: "send"},
	}); err != nil {
		t.Fatalf("push tool call start: %v", err)
	}
	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:  channel.StreamEventFinal,
		Final: &channel.StreamFinalizePayload{Message: channel.Message{Text: "resolved answer"}},
	}); err != nil {
		t.Fatalf("push final: %v", err)
	}
	if finalized != "resolved answer" {
		t.Fatalf("expected explicit final text, got %q", finalized)
	}
}

func TestDiscordOutboundStreamErrorOnlyRetriesRemainingAttachmentsAfterPartialFinalFailure(t *testing.T) {
	t.Parallel()

	failed := false
	var sent []string
	stream := &discordOutboundStream{
		adapter: &DiscordAdapter{},
		finalizeFn: func(string) error {
			return nil
		},
		sendFn: func(_ context.Context, att channel.Attachment) error {
			if att.URL == "https://example.com/b.png" && !failed {
				failed = true
				return errors.New("send failed")
			}
			sent = append(sent, att.URL)
			return nil
		},
	}

	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type: channel.StreamEventAttachment,
		Attachments: []channel.Attachment{
			{Type: channel.AttachmentImage, URL: "https://example.com/a.png"},
			{Type: channel.AttachmentImage, URL: "https://example.com/b.png"},
		},
	}); err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:  channel.StreamEventFinal,
		Final: &channel.StreamFinalizePayload{Message: channel.Message{Text: "done"}},
	}); err == nil {
		t.Fatal("expected final send to fail")
	}
	if got := len(stream.attachments); got != 1 {
		t.Fatalf("expected one remaining buffered attachment after partial failure, got %d", got)
	}
	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:  channel.StreamEventError,
		Error: "boom",
	}); err != nil {
		t.Fatalf("push error: %v", err)
	}
	if len(sent) != 2 {
		t.Fatalf("expected exactly two successful attachment deliveries, got %d (%v)", len(sent), sent)
	}
	if sent[0] != "https://example.com/a.png" || sent[1] != "https://example.com/b.png" {
		t.Fatalf("unexpected successful attachment order: %v", sent)
	}
}
