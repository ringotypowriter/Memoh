package feishu

import (
	"context"
	"errors"
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

func TestFeishuOutboundStreamBuffersAttachments(t *testing.T) {
	t.Parallel()

	stream := &feishuOutboundStream{
		adapter: &FeishuAdapter{},
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

func TestFeishuOutboundStreamErrorFlushesBufferedAttachments(t *testing.T) {
	t.Parallel()

	flushed := 0
	stream := &feishuOutboundStream{
		adapter: &FeishuAdapter{},
		ensureCardFn: func(context.Context, string) error {
			return nil
		},
		patchCardFn: func(context.Context, string) error {
			return nil
		},
		sendMessageFn: func(_ context.Context, msg channel.Message) error {
			flushed += len(msg.Attachments)
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

func TestFeishuOutboundStreamFinalStillFlushesAttachmentsWhenCardPatchFails(t *testing.T) {
	t.Parallel()

	flushed := 0
	stream := &feishuOutboundStream{
		adapter: &FeishuAdapter{},
		ensureCardFn: func(context.Context, string) error {
			return nil
		},
		patchCardFn: func(context.Context, string) error {
			return errors.New("patch failed")
		},
		sendMessageFn: func(_ context.Context, msg channel.Message) error {
			flushed += len(msg.Attachments)
			return nil
		},
	}

	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:        channel.StreamEventAttachment,
		Attachments: []channel.Attachment{{Type: channel.AttachmentImage, URL: "https://example.com/a.png"}},
	}); err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	err := stream.Push(context.Background(), channel.StreamEvent{
		Type: channel.StreamEventFinal,
		Final: &channel.StreamFinalizePayload{
			Message: channel.Message{Text: "done"},
		},
	})
	if err == nil {
		t.Fatal("expected card patch error to be returned")
	}
	if flushed != 1 {
		t.Fatalf("expected buffered attachment to flush even when card patch fails, got %d", flushed)
	}
}

func TestFeishuOutboundStreamErrorStillFlushesAttachmentsWhenCardPatchFails(t *testing.T) {
	t.Parallel()

	flushed := 0
	stream := &feishuOutboundStream{
		adapter: &FeishuAdapter{},
		ensureCardFn: func(context.Context, string) error {
			return nil
		},
		patchCardFn: func(context.Context, string) error {
			return errors.New("patch failed")
		},
		sendMessageFn: func(_ context.Context, msg channel.Message) error {
			flushed += len(msg.Attachments)
			return nil
		},
	}

	if err := stream.Push(context.Background(), channel.StreamEvent{
		Type:        channel.StreamEventAttachment,
		Attachments: []channel.Attachment{{Type: channel.AttachmentImage, URL: "https://example.com/a.png"}},
	}); err != nil {
		t.Fatalf("push attachment: %v", err)
	}
	err := stream.Push(context.Background(), channel.StreamEvent{
		Type:  channel.StreamEventError,
		Error: "boom",
	})
	if err == nil {
		t.Fatal("expected card patch error to be returned")
	}
	if flushed != 1 {
		t.Fatalf("expected buffered attachment to flush even when error card patch fails, got %d", flushed)
	}
}

func TestFeishuOutboundStreamErrorOnlyRetriesRemainingAttachmentsAfterPartialFinalFailure(t *testing.T) {
	t.Parallel()

	failed := false
	var sent []string
	stream := &feishuOutboundStream{
		adapter: &FeishuAdapter{},
		ensureCardFn: func(context.Context, string) error {
			return nil
		},
		patchCardFn: func(context.Context, string) error {
			return nil
		},
		sendMessageFn: func(_ context.Context, msg channel.Message) error {
			if len(msg.Attachments) != 1 {
				t.Fatalf("expected one attachment per flush attempt, got %d", len(msg.Attachments))
			}
			att := msg.Attachments[0]
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
