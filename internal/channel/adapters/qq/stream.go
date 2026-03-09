package qq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/memohai/memoh/internal/channel"
)

type qqOutboundStream struct {
	target string
	reply  *channel.ReplyRef
	send   func(context.Context, channel.OutboundMessage) error

	closed         atomic.Bool
	mu             sync.Mutex
	buffer         strings.Builder
	attachments    []channel.Attachment
	sentText       bool
	toolSentKeys   map[string]struct{}
	failedToolKeys map[string]struct{}
}

func (a *QQAdapter) OpenStream(_ context.Context, cfg channel.ChannelConfig, target string, opts channel.StreamOptions) (channel.OutboundStream, error) {
	return &qqOutboundStream{
		target: target,
		reply:  opts.Reply,
		send: func(ctx context.Context, msg channel.OutboundMessage) error {
			if msg.Target == "" {
				msg.Target = target
			}
			if msg.Message.Reply == nil && opts.Reply != nil {
				msg.Message.Reply = opts.Reply
			}
			return a.Send(ctx, cfg, msg)
		},
	}, nil
}

func (s *qqOutboundStream) Push(ctx context.Context, event channel.StreamEvent) error {
	if s == nil || s.send == nil {
		return errors.New("qq stream not configured")
	}
	if s.closed.Load() {
		return errors.New("qq stream is closed")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	switch event.Type {
	case channel.StreamEventStatus,
		channel.StreamEventPhaseStart,
		channel.StreamEventPhaseEnd,
		channel.StreamEventToolCallStart,
		channel.StreamEventAgentStart,
		channel.StreamEventAgentEnd,
		channel.StreamEventProcessingStarted,
		channel.StreamEventProcessingCompleted,
		channel.StreamEventProcessingFailed:
		return nil
	case channel.StreamEventDelta:
		if event.Phase == channel.StreamPhaseReasoning || event.Delta == "" {
			return nil
		}
		s.mu.Lock()
		s.buffer.WriteString(event.Delta)
		s.mu.Unlock()
		return nil
	case channel.StreamEventToolCallEnd:
		s.rememberToolCallAttachmentOutcome(event.ToolCall)
		return nil
	case channel.StreamEventAttachment:
		if len(event.Attachments) == 0 {
			return nil
		}
		s.mu.Lock()
		s.attachments = append(s.attachments, event.Attachments...)
		s.mu.Unlock()
		return nil
	case channel.StreamEventError:
		errText := strings.TrimSpace(event.Error)
		if errText == "" {
			return nil
		}
		return s.flush(ctx, channel.Message{
			Text: "Error: " + errText,
		})
	case channel.StreamEventFinal:
		if event.Final == nil {
			return errors.New("qq stream final payload is required")
		}
		return s.flush(ctx, event.Final.Message)
	default:
		return nil
	}
}

func (s *qqOutboundStream) Close(ctx context.Context) error {
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

func (s *qqOutboundStream) flush(ctx context.Context, msg channel.Message) error {
	s.mu.Lock()
	bufferedText := strings.TrimSpace(s.buffer.String())
	bufferedAttachments := append([]channel.Attachment(nil), s.attachments...)
	alreadySentText := s.sentText
	s.buffer.Reset()
	s.attachments = nil
	s.mu.Unlock()

	if bufferedText != "" {
		msg.Text = bufferedText
		msg.Parts = nil
		if msg.Format == "" {
			msg.Format = channel.MessageFormatPlain
		}
	} else if alreadySentText && len(bufferedAttachments) == 0 && len(msg.Attachments) == 0 && strings.TrimSpace(msg.PlainText()) != "" {
		return nil
	}
	toolSentKeys := s.mergedToolSentKeys(msg.Metadata)
	if len(bufferedAttachments) > 0 || len(msg.Attachments) > 0 {
		msg.Attachments = deduplicateQQAttachmentsExact(append(
			filterQQToolDuplicateAttachments(bufferedAttachments, toolSentKeys),
			filterQQToolDuplicateAttachments(msg.Attachments, toolSentKeys)...,
		))
	}
	if msg.Reply == nil && s.reply != nil {
		msg.Reply = s.reply
	}
	if msg.IsEmpty() {
		return nil
	}
	if err := s.send(ctx, channel.OutboundMessage{
		Target:  s.target,
		Message: msg,
	}); err != nil {
		return err
	}
	if strings.TrimSpace(msg.PlainText()) != "" {
		s.mu.Lock()
		s.sentText = true
		s.mu.Unlock()
	}
	return nil
}

func (s *qqOutboundStream) mergedToolSentKeys(metadata map[string]any) map[string]struct{} {
	metaKeys := parseToolSentAttachmentKeys(metadata)
	if len(s.toolSentKeys) == 0 {
		return metaKeys
	}
	merged := make(map[string]struct{}, len(s.toolSentKeys)+len(metaKeys))
	for k := range s.toolSentKeys {
		merged[k] = struct{}{}
	}
	for k := range metaKeys {
		merged[k] = struct{}{}
	}
	for k := range s.failedToolKeys {
		delete(merged, k)
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func (s *qqOutboundStream) rememberToolCallAttachmentOutcome(toolCall *channel.StreamToolCall) {
	if s == nil || toolCall == nil {
		return
	}
	keys := qqToolCallAttachmentKeys(toolCall, s.target)
	if len(keys) == 0 {
		return
	}
	success := qqToolCallSucceeded(toolCall.Result)
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

type qqSendToolArgs struct {
	Platform          string           `json:"platform"`
	Target            string           `json:"target"`
	ChannelIdentityID string           `json:"channel_identity_id"`
	Attachments       any              `json:"attachments"`
	Message           *channel.Message `json:"message"`
}

func qqToolCallAttachmentKeys(toolCall *channel.StreamToolCall, currentTarget string) []string {
	if toolCall == nil {
		return nil
	}
	name := strings.TrimSpace(toolCall.Name)
	if name != "send" && name != "send_message" {
		return nil
	}
	var args qqSendToolArgs
	if !decodeQQSendToolArgs(toolCall.Input, &args) {
		return nil
	}
	if !qqToolCallTargetsCurrentStream(args, currentTarget) {
		return nil
	}
	seen := make(map[string]struct{})
	var keys []string
	if args.Message != nil {
		for _, att := range args.Message.Attachments {
			appendUniqueQQAttachmentKeys(&keys, seen, qqAttachmentMatchKeys(att))
		}
	}
	for _, item := range normalizeQQToolAttachmentInputs(args.Attachments) {
		appendUniqueQQAttachmentKeys(&keys, seen, qqToolAttachmentInputKeys(item))
	}
	return keys
}

func decodeQQSendToolArgs(raw any, out *qqSendToolArgs) bool {
	if raw == nil || out == nil {
		return false
	}
	data, err := json.Marshal(raw)
	if err != nil || len(data) == 0 {
		return false
	}
	return json.Unmarshal(data, out) == nil
}

func qqToolCallTargetsCurrentStream(args qqSendToolArgs, currentTarget string) bool {
	platform := strings.ToLower(strings.TrimSpace(args.Platform))
	if platform != "" && platform != "qq" {
		return false
	}
	target := strings.TrimSpace(args.Target)
	if target == "" {
		return strings.TrimSpace(args.ChannelIdentityID) == ""
	}
	return normalizeTarget(target) == normalizeTarget(currentTarget)
}

func qqToolCallSucceeded(result any) bool {
	value, ok := result.(map[string]any)
	if !ok {
		return true
	}
	if raw, exists := value["isError"]; exists {
		switch v := raw.(type) {
		case bool:
			return !v
		case string:
			return strings.ToLower(strings.TrimSpace(v)) != "true"
		}
	}
	return true
}

func normalizeQQToolAttachmentInputs(raw any) []any {
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

func qqToolAttachmentInputKeys(raw any) []string {
	switch value := raw.(type) {
	case string:
		ref := strings.TrimSpace(value)
		if ref == "" {
			return nil
		}
		keys := []string{"ref:" + ref}
		if sk := qqExtractStorageKey(ref); sk != "" {
			keys = append(keys, "storage:"+sk)
		}
		return keys
	case map[string]any:
		seen := make(map[string]struct{})
		var keys []string
		if contentHash := strings.TrimSpace(fmt.Sprint(value["content_hash"])); contentHash != "" && contentHash != "<nil>" {
			appendUniqueQQAttachmentKeys(&keys, seen, []string{"hash:" + contentHash})
		}
		if path := strings.TrimSpace(fmt.Sprint(value["path"])); path != "" && path != "<nil>" {
			appendUniqueQQAttachmentKeys(&keys, seen, qqRefKeys(path))
		}
		if url := strings.TrimSpace(fmt.Sprint(value["url"])); url != "" && url != "<nil>" {
			appendUniqueQQAttachmentKeys(&keys, seen, qqRefKeys(url))
		}
		return keys
	default:
		return nil
	}
}

func qqRefKeys(ref string) []string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	keys := []string{"ref:" + ref}
	if sk := qqExtractStorageKey(ref); sk != "" {
		keys = append(keys, "storage:"+sk)
	}
	return keys
}

func appendUniqueQQAttachmentKeys(dst *[]string, seen map[string]struct{}, keys []string) {
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

func parseToolSentAttachmentKeys(metadata map[string]any) map[string]struct{} {
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
			key := strings.TrimSpace(itemToString(item))
			if key != "" {
				keys[key] = struct{}{}
			}
		}
	case []string:
		for _, item := range value {
			key := strings.TrimSpace(item)
			if key != "" {
				keys[key] = struct{}{}
			}
		}
	case string:
		key := strings.TrimSpace(value)
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return keys
}

func filterQQToolDuplicateAttachments(attachments []channel.Attachment, toolSentKeys map[string]struct{}) []channel.Attachment {
	if len(attachments) == 0 || len(toolSentKeys) == 0 {
		return attachments
	}
	filtered := make([]channel.Attachment, 0, len(attachments))
	for _, att := range attachments {
		if qqAttachmentMatchesToolSentKeys(att, toolSentKeys) {
			continue
		}
		filtered = append(filtered, att)
	}
	return filtered
}

func qqAttachmentMatchesToolSentKeys(att channel.Attachment, toolSentKeys map[string]struct{}) bool {
	for _, key := range qqAttachmentMatchKeys(att) {
		if _, ok := toolSentKeys[key]; ok {
			return true
		}
	}
	return false
}

func qqAttachmentMatchKeys(att channel.Attachment) []string {
	keys := make([]string, 0, 4)
	if contentHash := strings.TrimSpace(att.ContentHash); contentHash != "" {
		keys = append(keys, "hash:"+contentHash)
	}
	if storageKey := qqAttachmentStorageKey(att); storageKey != "" {
		keys = append(keys, "storage:"+storageKey)
	}
	if ref := strings.TrimSpace(att.URL); ref != "" {
		keys = append(keys, "ref:"+ref)
		if storageKey := qqExtractStorageKey(ref); storageKey != "" {
			keys = append(keys, "storage:"+storageKey)
		}
	}
	if platformKey := strings.TrimSpace(att.PlatformKey); platformKey != "" {
		keys = append(keys, "platform:"+platformKey)
	}
	return keys
}

func qqAttachmentStorageKey(att channel.Attachment) string {
	if att.Metadata == nil {
		return ""
	}
	if storageKey, ok := att.Metadata["storage_key"].(string); ok {
		return strings.TrimSpace(storageKey)
	}
	return ""
}

func qqExtractStorageKey(ref string) string {
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

func deduplicateQQAttachmentsExact(attachments []channel.Attachment) []channel.Attachment {
	if len(attachments) < 2 {
		return attachments
	}
	seen := make(map[string]struct{}, len(attachments))
	deduped := make([]channel.Attachment, 0, len(attachments))
	for _, att := range attachments {
		key := qqExactAttachmentKey(att)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, att)
	}
	return deduped
}

func qqExactAttachmentKey(att channel.Attachment) string {
	var b strings.Builder
	b.Grow(256)
	b.WriteString(strings.TrimSpace(string(att.Type)))
	b.WriteString("|")
	b.WriteString(strings.TrimSpace(att.URL))
	b.WriteString("|")
	b.WriteString(strings.TrimSpace(att.PlatformKey))
	b.WriteString("|")
	b.WriteString(strings.TrimSpace(att.ContentHash))
	b.WriteString("|")
	b.WriteString(strings.TrimSpace(att.Base64))
	b.WriteString("|")
	b.WriteString(strings.TrimSpace(att.Name))
	b.WriteString("|")
	b.WriteString(strings.TrimSpace(att.Mime))
	b.WriteString("|")
	b.WriteString(strings.TrimSpace(att.Caption))
	b.WriteString("|")
	b.WriteString(strconv.FormatInt(att.Size, 10))
	b.WriteString("|")
	b.WriteString(strconv.FormatInt(att.DurationMs, 10))
	b.WriteString("|")
	b.WriteString(strconv.Itoa(att.Width))
	b.WriteString("|")
	b.WriteString(strconv.Itoa(att.Height))
	b.WriteString("|")
	b.WriteString(strings.TrimSpace(att.ThumbnailURL))
	return b.String()
}

func itemToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
