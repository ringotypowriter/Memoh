package channel

import "testing"

func TestDeduplicateAttachmentsExact(t *testing.T) {
	attachments := []Attachment{
		{Type: AttachmentImage, URL: "https://example.com/a.png"},
		{Type: AttachmentImage, URL: "https://example.com/a.png"},
	}

	got := DeduplicateAttachmentsExact(attachments)
	if len(got) != 1 {
		t.Fatalf("expected 1 attachment after dedupe, got %d", len(got))
	}
}

func TestDeduplicateAttachmentsExactPreservesDistinctMetadata(t *testing.T) {
	attachments := []Attachment{
		{Type: AttachmentImage, URL: "https://example.com/a.png", Caption: "first"},
		{Type: AttachmentImage, URL: "https://example.com/a.png", Caption: "second"},
		{Type: AttachmentFile, URL: "https://example.com/a.png", Name: "report.png"},
	}

	got := DeduplicateAttachmentsExact(attachments)
	if len(got) != 3 {
		t.Fatalf("expected distinct attachments to be preserved, got %d", len(got))
	}
}
