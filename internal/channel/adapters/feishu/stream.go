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
	adapter        *FeishuAdapter
	cfg            channel.ChannelConfig
	target         string
	reply          *channel.ReplyRef
	send           func(context.Context, channel.OutboundMessage) error
	client         *lark.Client
	receiveID      string
	receiveType    string
	cardMessageID  string
	textBuffer     strings.Builder
	lastPatchedAt  time.Time
	lastPatched    string
	patchInterval  time.Duration
	attachments    []channel.Attachment
	toolSentKeys   map[string]struct{}
	failedToolKeys map[string]struct{}
	sawFinal       bool
	closed         atomic.Bool
}

func (s *feishuOutboundStream) Push(ctx context.Context, event channel.StreamEvent) error {
	if s == nil || (s.adapter == nil && s.send == nil) {
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
			return s.ensureCard(ctx, feishuStreamThinkingText)
		}
		if event.Status == channel.StreamStatusCompleted && !s.sawFinal {
			s.attachments = nil
		}
		return nil
	case channel.StreamEventDelta:
		if event.Delta == "" || event.Phase == channel.StreamPhaseReasoning {
			return nil
		}
		s.textBuffer.WriteString(event.Delta)
		if err := s.ensureCard(ctx, feishuStreamThinkingText); err != nil {
			return err
		}
		if time.Since(s.lastPatchedAt) < s.patchInterval && !strings.Contains(event.Delta, "\n") {
			return nil
		}
		return s.patchCard(ctx, s.textBuffer.String())
	case channel.StreamEventToolCallStart:
		bufText := strings.TrimSpace(s.textBuffer.String())
		if s.cardMessageID != "" && bufText != "" {
			_ = s.patchCard(ctx, bufText)
		}
		s.cardMessageID = ""
		s.lastPatched = ""
		s.lastPatchedAt = time.Time{}
		s.textBuffer.Reset()
		return nil
	case channel.StreamEventToolCallEnd:
		s.rememberToolCallAttachmentOutcome(event.ToolCall)
		s.cardMessageID = ""
		s.lastPatched = ""
		s.lastPatchedAt = time.Time{}
		s.textBuffer.Reset()
		return nil
	case channel.StreamEventAttachment:
		attachments := s.filterToolDuplicateAttachments(event.Attachments, nil)
		if len(attachments) == 0 {
			return nil
		}
		s.attachments = append(s.attachments, attachments...)
		return nil
	case channel.StreamEventPhaseStart, channel.StreamEventPhaseEnd:
		return nil
	case channel.StreamEventAgentStart, channel.StreamEventAgentEnd, channel.StreamEventProcessingStarted, channel.StreamEventProcessingCompleted, channel.StreamEventProcessingFailed:
		return nil
	case channel.StreamEventFinal:
		if event.Final == nil {
			return nil
		}
		s.sawFinal = true
		msg := event.Final.Message
		msg.Attachments = s.mergeBufferedAttachments(msg.Attachments, msg.Metadata)
		if msg.IsEmpty() {
			return nil
		}
		bufText := strings.TrimSpace(s.textBuffer.String())
		finalText := bufText
		if finalText == "" {
			finalText = strings.TrimSpace(msg.PlainText())
		}
		if finalText != "" {
			if err := s.ensureCard(ctx, feishuStreamThinkingText); err != nil {
				return err
			}
			if err := s.patchCard(ctx, finalText); err != nil {
				return err
			}
		}
		if len(msg.Attachments) > 0 {
			media := msg
			media.Format = ""
			media.Text = ""
			media.Parts = nil
			media.Actions = nil
			media.Reply = nil
			if err := s.sendMessage(ctx, channel.OutboundMessage{
				Target:  s.target,
				Message: media,
			}); err != nil {
				return err
			}
		}
		return nil
	case channel.StreamEventError:
		errText := strings.TrimSpace(event.Error)
		var patchErr error
		if errText != "" {
			if err := s.ensureCard(ctx, feishuStreamThinkingText); err != nil {
				patchErr = err
			} else if err := s.patchCard(ctx, "Error: "+errText); err != nil {
				patchErr = err
			}
		}
		return errors.Join(patchErr, s.flushBufferedAttachments(ctx, nil))
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

func (s *feishuOutboundStream) sendMessage(ctx context.Context, msg channel.OutboundMessage) error {
	if s == nil {
		return errors.New("feishu stream not configured")
	}
	if strings.TrimSpace(msg.Target) == "" {
		msg.Target = s.target
	}
	if s.send != nil {
		return s.send(ctx, msg)
	}
	if s.adapter == nil {
		return errors.New("feishu stream not configured")
	}
	return s.adapter.Send(ctx, s.cfg, msg)
}

func (s *feishuOutboundStream) mergeBufferedAttachments(attachments []channel.Attachment, metadata map[string]any) []channel.Attachment {
	buffered := s.filterToolDuplicateAttachments(s.attachments, metadata)
	current := s.filterToolDuplicateAttachments(attachments, metadata)
	if len(buffered) == 0 {
		s.attachments = nil
		return current
	}
	merged := make([]channel.Attachment, 0, len(buffered)+len(current))
	merged = append(merged, buffered...)
	merged = append(merged, current...)
	s.attachments = nil
	return merged
}

func (s *feishuOutboundStream) flushBufferedAttachments(ctx context.Context, metadata map[string]any) error {
	attachments := s.mergeBufferedAttachments(nil, metadata)
	if len(attachments) == 0 {
		return nil
	}
	return s.sendMessage(ctx, channel.OutboundMessage{
		Target: s.target,
		Message: channel.Message{
			Attachments: attachments,
		},
	})
}

func (s *feishuOutboundStream) rememberToolCallAttachmentOutcome(toolCall *channel.StreamToolCall) {
	if s == nil || toolCall == nil {
		return
	}
	keys := feishuToolCallAttachmentKeys(toolCall, s.target)
	if len(keys) == 0 {
		return
	}
	success := feishuToolCallSucceeded(toolCall.Result)
	if success {
		if s.toolSentKeys == nil {
			s.toolSentKeys = make(map[string]struct{}, len(keys))
		}
		for _, key := range keys {
			if key = strings.TrimSpace(key); key != "" {
				s.toolSentKeys[key] = struct{}{}
				if s.failedToolKeys != nil {
					delete(s.failedToolKeys, key)
				}
			}
		}
		return
	}
	if s.failedToolKeys == nil {
		s.failedToolKeys = make(map[string]struct{}, len(keys))
	}
	for _, key := range keys {
		if key = strings.TrimSpace(key); key != "" {
			if s.toolSentKeys != nil {
				if _, sent := s.toolSentKeys[key]; sent {
					continue
				}
			}
			s.failedToolKeys[key] = struct{}{}
		}
	}
}

func feishuToolCallSucceeded(result any) bool {
	value, ok := result.(map[string]any)
	if !ok {
		return true
	}
	if raw, exists := value["isError"]; exists {
		switch v := raw.(type) {
		case bool:
			if v {
				return false
			}
			return true
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "true":
				return false
			case "false":
				return true
			}
		}
	}
	return true
}

func (s *feishuOutboundStream) filterToolDuplicateAttachments(attachments []channel.Attachment, metadata map[string]any) []channel.Attachment {
	if len(attachments) == 0 {
		return attachments
	}
	keys := make(map[string]struct{}, len(s.toolSentKeys))
	for key := range s.toolSentKeys {
		keys[key] = struct{}{}
	}
	for key := range parseFeishuToolSentAttachmentKeys(metadata) {
		keys[key] = struct{}{}
	}
	for key := range s.failedToolKeys {
		delete(keys, key)
	}
	if len(keys) == 0 {
		return attachments
	}
	filtered := make([]channel.Attachment, 0, len(attachments))
	for _, att := range attachments {
		if feishuAttachmentMatchesToolSentKeys(att, keys) {
			continue
		}
		filtered = append(filtered, att)
	}
	return filtered
}

type feishuSendToolArgs struct {
	Platform          string           `json:"platform"`
	Target            string           `json:"target"`
	ChannelIdentityID string           `json:"channel_identity_id"`
	Attachments       any              `json:"attachments"`
	Message           *channel.Message `json:"message"`
}

func feishuToolCallAttachmentKeys(toolCall *channel.StreamToolCall, currentTarget string) []string {
	if toolCall == nil {
		return nil
	}
	name := strings.TrimSpace(toolCall.Name)
	if name != "send" && name != "send_message" {
		return nil
	}
	var args feishuSendToolArgs
	if !decodeFeishuSendToolArgs(toolCall.Input, &args) {
		return nil
	}
	if !feishuToolCallTargetsCurrentStream(args, currentTarget) {
		return nil
	}
	seen := make(map[string]struct{})
	keys := make([]string, 0)
	if args.Message != nil {
		for _, att := range args.Message.Attachments {
			appendUniqueFeishuAttachmentKeys(&keys, seen, feishuAttachmentMatchKeys(att))
		}
	}
	for _, item := range normalizeFeishuToolAttachmentInputs(args.Attachments) {
		appendUniqueFeishuAttachmentKeys(&keys, seen, feishuToolAttachmentInputKeys(item))
	}
	return keys
}

func decodeFeishuSendToolArgs(raw any, out *feishuSendToolArgs) bool {
	if raw == nil || out == nil {
		return false
	}
	data, err := json.Marshal(raw)
	if err != nil || len(data) == 0 {
		return false
	}
	return json.Unmarshal(data, out) == nil
}

func feishuToolCallTargetsCurrentStream(args feishuSendToolArgs, currentTarget string) bool {
	platform := strings.ToLower(strings.TrimSpace(args.Platform))
	if platform != "" && platform != "feishu" && platform != "lark" {
		return false
	}
	target := strings.TrimSpace(args.Target)
	if target == "" {
		return strings.TrimSpace(args.ChannelIdentityID) == ""
	}
	return normalizeTarget(target) == normalizeTarget(currentTarget)
}

func normalizeFeishuToolAttachmentInputs(raw any) []any {
	switch value := raw.(type) {
	case nil:
		return nil
	case []any:
		return value
	case []string:
		items := make([]any, 0, len(value))
		for _, item := range value {
			items = append(items, item)
		}
		return items
	case string, map[string]any:
		return []any{value}
	default:
		return nil
	}
}

func feishuToolAttachmentInputKeys(raw any) []string {
	switch value := raw.(type) {
	case string:
		return feishuAttachmentReferenceKeys(value)
	case map[string]any:
		seen := make(map[string]struct{})
		keys := make([]string, 0, 4)
		if contentHash := strings.TrimSpace(fmt.Sprint(value["content_hash"])); contentHash != "" && contentHash != "<nil>" {
			appendUniqueFeishuAttachmentKeys(&keys, seen, []string{"hash:" + contentHash})
		}
		if path := strings.TrimSpace(fmt.Sprint(value["path"])); path != "" && path != "<nil>" {
			appendUniqueFeishuAttachmentKeys(&keys, seen, feishuAttachmentReferenceKeys(path))
		}
		if url := strings.TrimSpace(fmt.Sprint(value["url"])); url != "" && url != "<nil>" {
			appendUniqueFeishuAttachmentKeys(&keys, seen, feishuAttachmentReferenceKeys(url))
		}
		return keys
	default:
		return nil
	}
}

func appendUniqueFeishuAttachmentKeys(dst *[]string, seen map[string]struct{}, keys []string) {
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		*dst = append(*dst, key)
	}
}

func parseFeishuToolSentAttachmentKeys(metadata map[string]any) map[string]struct{} {
	if len(metadata) == 0 {
		return nil
	}
	raw, ok := metadata["tool_sent_attachment_keys"]
	if !ok || raw == nil {
		return nil
	}
	keys := make(map[string]struct{})
	switch value := raw.(type) {
	case []any:
		for _, item := range value {
			if key := strings.TrimSpace(fmt.Sprint(item)); key != "" && key != "<nil>" {
				keys[key] = struct{}{}
			}
		}
	case []string:
		for _, item := range value {
			if key := strings.TrimSpace(item); key != "" {
				keys[key] = struct{}{}
			}
		}
	case string:
		if key := strings.TrimSpace(value); key != "" {
			keys[key] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return keys
}

func feishuAttachmentMatchesToolSentKeys(att channel.Attachment, keys map[string]struct{}) bool {
	for _, key := range feishuAttachmentMatchKeys(att) {
		if _, ok := keys[key]; ok {
			return true
		}
	}
	return false
}

func feishuAttachmentMatchKeys(att channel.Attachment) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0, 4)
	if contentHash := strings.TrimSpace(att.ContentHash); contentHash != "" {
		appendUniqueFeishuAttachmentKeys(&keys, seen, []string{"hash:" + contentHash})
	}
	if storageKey := feishuAttachmentStorageKey(att); storageKey != "" {
		appendUniqueFeishuAttachmentKeys(&keys, seen, []string{"storage:" + storageKey})
	}
	if ref := strings.TrimSpace(att.URL); ref != "" {
		appendUniqueFeishuAttachmentKeys(&keys, seen, feishuAttachmentReferenceKeys(ref))
	}
	if platformKey := strings.TrimSpace(att.PlatformKey); platformKey != "" {
		appendUniqueFeishuAttachmentKeys(&keys, seen, []string{"platform:" + platformKey})
	}
	return keys
}

func feishuAttachmentStorageKey(att channel.Attachment) string {
	if att.Metadata == nil {
		return ""
	}
	storageKey, _ := att.Metadata["storage_key"].(string)
	return strings.TrimSpace(storageKey)
}

func feishuAttachmentReferenceKeys(ref string) []string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	seen := make(map[string]struct{})
	keys := make([]string, 0, 3)
	appendUniqueFeishuAttachmentKeys(&keys, seen, []string{"ref:" + ref})
	if storageKey := feishuExtractStorageKey(ref); storageKey != "" {
		appendUniqueFeishuAttachmentKeys(&keys, seen, []string{"storage:" + storageKey})
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		appendUniqueFeishuAttachmentKeys(&keys, seen, []string{"url:" + ref})
	}
	return keys
}

func feishuExtractStorageKey(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	marker := "/data/media/"
	idx := strings.Index(ref, marker)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(ref[idx+len(marker):])
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
