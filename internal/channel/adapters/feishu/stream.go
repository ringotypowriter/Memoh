package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/memohai/memoh/internal/channel"
)

const (
	feishuStreamThinkingText  = "Thinking..."
	feishuStreamToolHintText  = "Calling tools..."
	feishuStreamPatchInterval = 700 * time.Millisecond
	feishuStreamMaxRunes      = 8000
)

type feishuOutboundStream struct {
	adapter       *FeishuAdapter
	cfg           channel.ChannelConfig
	target        string
	reply         *channel.ReplyRef
	client        *lark.Client
	receiveID     string
	receiveType   string
	cardMessageID string
	textBuffer    strings.Builder
	attachments   []channel.Attachment
	lastPatchedAt time.Time
	lastPatched   string
	patchInterval time.Duration
	closed        atomic.Bool
	ensureCardFn  func(context.Context, string) error
	patchCardFn   func(context.Context, string) error
	sendMessageFn func(context.Context, channel.Message) error
}

func (s *feishuOutboundStream) Push(ctx context.Context, event channel.StreamEvent) error {
	if s == nil || s.adapter == nil {
		return errors.New("feishu stream not configured")
	}
	if s.closed.Load() {
		return errors.New("feishu stream is closed")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	switch event.Type {
	case channel.StreamEventStatus:
		if event.Status == channel.StreamStatusStarted {
			return s.ensureStreamCard(ctx, feishuStreamThinkingText)
		}
		return nil
	case channel.StreamEventDelta:
		if event.Delta == "" || event.Phase == channel.StreamPhaseReasoning {
			return nil
		}
		s.textBuffer.WriteString(event.Delta)
		if err := s.ensureStreamCard(ctx, feishuStreamThinkingText); err != nil {
			return err
		}
		if time.Since(s.lastPatchedAt) < s.patchInterval && !strings.Contains(event.Delta, "\n") {
			return nil
		}
		return s.patchStreamCard(ctx, s.textBuffer.String())
	case channel.StreamEventToolCallStart:
		bufText := strings.TrimSpace(s.textBuffer.String())
		if s.cardMessageID != "" && bufText != "" {
			_ = s.patchStreamCard(ctx, bufText)
		}
		s.cardMessageID = ""
		s.lastPatched = ""
		s.lastPatchedAt = time.Time{}
		s.textBuffer.Reset()
		return nil
	case channel.StreamEventToolCallEnd:
		s.cardMessageID = ""
		s.lastPatched = ""
		s.lastPatchedAt = time.Time{}
		s.textBuffer.Reset()
		return nil
	case channel.StreamEventAttachment:
		if len(event.Attachments) == 0 {
			return nil
		}
		s.attachments = append(s.attachments, event.Attachments...)
		return nil
	case channel.StreamEventPhaseStart, channel.StreamEventPhaseEnd:
		return nil
	case channel.StreamEventAgentStart, channel.StreamEventAgentEnd, channel.StreamEventProcessingStarted, channel.StreamEventProcessingCompleted, channel.StreamEventProcessingFailed:
		return nil
	case channel.StreamEventFinal:
		msg := channel.Message{}
		if event.Final != nil {
			msg = event.Final.Message
		}
		bufText := strings.TrimSpace(s.textBuffer.String())
		mergedAttachments := channel.DeduplicateAttachmentsExact(append(append([]channel.Attachment(nil), s.attachments...), msg.Attachments...))
		if msg.IsEmpty() && bufText == "" && len(mergedAttachments) == 0 {
			return nil
		}
		finalText := bufText
		if finalText == "" {
			finalText = strings.TrimSpace(msg.PlainText())
		}
		var cardErr error
		if finalText != "" {
			if err := s.ensureStreamCard(ctx, feishuStreamThinkingText); err != nil {
				cardErr = err
			} else if err := s.patchStreamCard(ctx, finalText); err != nil {
				cardErr = err
			}
		}
		if len(mergedAttachments) > 0 {
			if err := s.flushBufferedAttachments(ctx, mergedAttachments, msg); err != nil {
				if cardErr != nil {
					return errors.Join(cardErr, err)
				}
				return err
			}
		}
		return cardErr
	case channel.StreamEventError:
		errText := strings.TrimSpace(event.Error)
		if errText == "" {
			return nil
		}
		var cardErr error
		if err := s.ensureStreamCard(ctx, feishuStreamThinkingText); err != nil {
			cardErr = err
		} else if err := s.patchStreamCard(ctx, "Error: "+errText); err != nil {
			cardErr = err
		}
		attachments := channel.DeduplicateAttachmentsExact(append([]channel.Attachment(nil), s.attachments...))
		if err := s.flushBufferedAttachments(ctx, attachments, channel.Message{}); err != nil {
			if cardErr != nil {
				return errors.Join(cardErr, err)
			}
			return err
		}
		return cardErr
	default:
		return nil
	}
}

func (s *feishuOutboundStream) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	s.closed.Store(true)
	return nil
}

func (s *feishuOutboundStream) ensureStreamCard(ctx context.Context, text string) error {
	if s.ensureCardFn != nil {
		return s.ensureCardFn(ctx, text)
	}
	return s.ensureCard(ctx, text)
}

func (s *feishuOutboundStream) patchStreamCard(ctx context.Context, text string) error {
	if s.patchCardFn != nil {
		return s.patchCardFn(ctx, text)
	}
	return s.patchCard(ctx, text)
}

func (s *feishuOutboundStream) sendStreamMessage(ctx context.Context, msg channel.Message) error {
	if s.sendMessageFn != nil {
		return s.sendMessageFn(ctx, msg)
	}
	return s.adapter.Send(ctx, s.cfg, channel.OutboundMessage{
		Target:  s.target,
		Message: msg,
	})
}

func (s *feishuOutboundStream) flushBufferedAttachments(ctx context.Context, attachments []channel.Attachment, base channel.Message) error {
	if len(attachments) == 0 {
		return nil
	}
	pending := append([]channel.Attachment(nil), attachments...)
	s.attachments = append([]channel.Attachment(nil), pending...)
	for len(pending) > 0 {
		media := base
		media.Attachments = []channel.Attachment{pending[0]}
		media.Format = ""
		media.Text = ""
		media.Parts = nil
		media.Actions = nil
		media.Reply = nil
		if err := s.sendStreamMessage(ctx, media); err != nil {
			return err
		}
		pending = pending[1:]
		s.attachments = append([]channel.Attachment(nil), pending...)
	}
	return nil
}

func (s *feishuOutboundStream) ensureCard(ctx context.Context, text string) error {
	if strings.TrimSpace(s.cardMessageID) != "" {
		return nil
	}
	if s.client == nil {
		return errors.New("feishu client not configured")
	}
	content, err := buildFeishuStreamCardContent(text)
	if err != nil {
		return err
	}
	if s.reply != nil && strings.TrimSpace(s.reply.MessageID) != "" {
		replyReq := larkim.NewReplyMessageReqBuilder().
			MessageId(strings.TrimSpace(s.reply.MessageID)).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				Content(content).
				MsgType(larkim.MsgTypeInteractive).
				Uuid(uuid.NewString()).
				Build()).
			Build()
		replyResp, err := s.client.Im.Message.Reply(ctx, replyReq)
		if err != nil {
			return err
		}
		if replyResp == nil || !replyResp.Success() {
			code, msg := 0, ""
			if replyResp != nil {
				code, msg = replyResp.Code, replyResp.Msg
			}
			return fmt.Errorf("feishu stream reply failed: %s (code: %d)", msg, code)
		}
		if replyResp.Data == nil || replyResp.Data.MessageId == nil || strings.TrimSpace(*replyResp.Data.MessageId) == "" {
			return errors.New("feishu stream reply failed: empty message id")
		}
		s.cardMessageID = strings.TrimSpace(*replyResp.Data.MessageId)
		s.lastPatched = normalizeFeishuStreamText(text)
		s.lastPatchedAt = time.Now()
		return nil
	}
	createReq := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(s.receiveType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(s.receiveID).
			MsgType(larkim.MsgTypeInteractive).
			Content(content).
			Uuid(uuid.NewString()).
			Build()).
		Build()
	createResp, err := s.client.Im.Message.Create(ctx, createReq)
	if err != nil {
		return err
	}
	if createResp == nil || !createResp.Success() {
		code, msg := 0, ""
		if createResp != nil {
			code, msg = createResp.Code, createResp.Msg
		}
		return fmt.Errorf("feishu stream create failed: %s (code: %d)", msg, code)
	}
	if createResp.Data == nil || createResp.Data.MessageId == nil || strings.TrimSpace(*createResp.Data.MessageId) == "" {
		return errors.New("feishu stream create failed: empty message id")
	}
	s.cardMessageID = strings.TrimSpace(*createResp.Data.MessageId)
	s.lastPatched = normalizeFeishuStreamText(text)
	s.lastPatchedAt = time.Now()
	return nil
}

func (s *feishuOutboundStream) patchCard(ctx context.Context, text string) error {
	if strings.TrimSpace(s.cardMessageID) == "" {
		return errors.New("feishu stream card message not initialized")
	}
	contentText := normalizeFeishuStreamText(text)
	if contentText == s.lastPatched {
		return nil
	}
	content, err := buildFeishuStreamCardContent(contentText)
	if err != nil {
		return err
	}
	patchReq := larkim.NewPatchMessageReqBuilder().
		MessageId(strings.TrimSpace(s.cardMessageID)).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(content).
			Build()).
		Build()
	patchResp, err := s.client.Im.Message.Patch(ctx, patchReq)
	if err != nil {
		return err
	}
	if patchResp == nil || !patchResp.Success() {
		code, msg := 0, ""
		if patchResp != nil {
			code, msg = patchResp.Code, patchResp.Msg
		}
		return fmt.Errorf("feishu stream patch failed: %s (code: %d)", msg, code)
	}
	s.lastPatched = contentText
	s.lastPatchedAt = time.Now()
	return nil
}

// extractReadableFromJSON tries to extract human-readable text from JSON-like content.
// Returns the original text if not JSON or extraction fails.
func extractReadableFromJSON(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return text
	}
	first := strings.TrimLeft(trimmed, " \t\n\r")
	if (len(first) > 0 && first[0] != '{' && first[0] != '[') || len(first) < 2 {
		return text
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		var arr []any
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return text
		}
		if len(arr) == 0 {
			return text
		}
		if s, ok := arr[0].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
		return text
	}
	for _, key := range []string{"text", "message", "content", "result", "output", "response", "answer"} {
		if v, ok := raw[key]; ok && v != nil {
			switch val := v.(type) {
			case string:
				if strings.TrimSpace(val) != "" {
					return val
				}
			case map[string]any:
				if b, err := json.Marshal(val); err == nil {
					return string(b)
				}
			}
		}
	}
	return text
}

func buildFeishuStreamCardContent(text string) (string, error) {
	content := normalizeFeishuStreamText(extractReadableFromJSON(text))
	body := processFeishuCardMarkdown(content)
	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
			"enable_forward":   true,
			"update_multi":     true,
		},
		"elements": []map[string]any{
			{
				"tag": "div",
				"fields": []map[string]any{
					{
						"is_short": false,
						"text": map[string]any{
							"tag":     "lark_md",
							"content": body,
						},
					},
				},
			},
		},
	}
	data, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

var feishuCardHeadingPrefix = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)

// processFeishuCardMarkdown normalizes markdown for Feishu card lark_md (e.g. ATX headings to bold).
func processFeishuCardMarkdown(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = feishuCardHeadingPrefix.ReplaceAllStringFunc(s, func(m string) string {
		parts := feishuCardHeadingPrefix.FindStringSubmatch(m)
		if len(parts) == 2 {
			return "**" + parts[1] + "**"
		}
		return m
	})
	return s
}

func normalizeFeishuStreamText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return feishuStreamThinkingText
	}
	runes := []rune(trimmed)
	if len(runes) <= feishuStreamMaxRunes {
		return trimmed
	}
	return "...\n" + string(runes[len(runes)-feishuStreamMaxRunes:])
}
