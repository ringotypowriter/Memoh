package discord

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/memohai/memoh/internal/channel"
)

type fakeProcessingStatusSession struct {
	typingErr   error
	reactionErr error
}

func (s *fakeProcessingStatusSession) ChannelTyping(channelID string, options ...discordgo.RequestOption) error {
	return s.typingErr
}

func (s *fakeProcessingStatusSession) MessageReactionAdd(channelID, messageID, emoji string, options ...discordgo.RequestOption) error {
	return s.reactionErr
}

func TestStartProcessingStatus_ReturnsHandleWithoutTypingErrorWhenReactionAdded(t *testing.T) {
	t.Parallel()

	session := &fakeProcessingStatusSession{
		typingErr: errors.New("typing failed"),
	}

	handle, err := startProcessingStatus(session, "chat-1", "msg-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if handle.Token != processingBusyReactionEmoji {
		t.Fatalf("unexpected handle token: %q", handle.Token)
	}
}

func TestStartProcessingStatus_ReturnsTypingErrorWhenNoReactionHandle(t *testing.T) {
	t.Parallel()

	typingErr := errors.New("typing failed")
	session := &fakeProcessingStatusSession{
		typingErr: typingErr,
	}

	handle, err := startProcessingStatus(session, "chat-1", "")
	if !errors.Is(err, typingErr) {
		t.Fatalf("expected typing error, got %v", err)
	}
	if handle != (channel.ProcessingStatusHandle{}) {
		t.Fatalf("expected empty handle, got %+v", handle)
	}
}

func TestStartProcessingStatus_ReturnsReactionErrorWhenReactionFails(t *testing.T) {
	t.Parallel()

	reactionErr := errors.New("reaction failed")
	session := &fakeProcessingStatusSession{
		reactionErr: reactionErr,
	}

	handle, err := startProcessingStatus(session, "chat-1", "msg-1")
	if !errors.Is(err, reactionErr) {
		t.Fatalf("expected reaction error, got %v", err)
	}
	if handle != (channel.ProcessingStatusHandle{}) {
		t.Fatalf("expected empty handle, got %+v", handle)
	}
}
