package channel

import (
	"encoding/json"
	"strconv"
	"strings"
)

// DeduplicateAttachmentsExact collapses only exact attachment payload duplicates.
// Attachments with different type/caption/name metadata remain distinct.
func DeduplicateAttachmentsExact(attachments []Attachment) []Attachment {
	if len(attachments) < 2 {
		return attachments
	}
	result := make([]Attachment, 0, len(attachments))
	seen := make(map[string]struct{}, len(attachments))
	for i, att := range attachments {
		key, ok := ExactAttachmentKey(att)
		if !ok {
			key = "index:" + strconv.Itoa(i)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, att)
	}
	return result
}

// ExactAttachmentKey returns a stable key for exact attachment comparisons.
// The bool reports whether the attachment could be normalized successfully.
func ExactAttachmentKey(att Attachment) (string, bool) {
	normalized := att
	normalized.URL = strings.TrimSpace(normalized.URL)
	normalized.PlatformKey = strings.TrimSpace(normalized.PlatformKey)
	normalized.SourcePlatform = strings.TrimSpace(normalized.SourcePlatform)
	normalized.ContentHash = strings.TrimSpace(normalized.ContentHash)
	normalized.Base64 = strings.TrimSpace(normalized.Base64)
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.Mime = strings.TrimSpace(normalized.Mime)
	normalized.Caption = strings.TrimSpace(normalized.Caption)

	data, err := json.Marshal(normalized)
	if err != nil {
		return "", false
	}
	return string(data), true
}
