package container

import textprune "github.com/memohai/memoh/internal/prune"

const (
	toolOutputHeadBytes = 6 * 1024
	toolOutputTailBytes = 2 * 1024
	toolOutputHeadLines = 180
	toolOutputTailLines = 50
)

func pruneToolOutputText(text, label string) string {
	return textprune.PruneWithEdges(text, label, textprune.Config{
		MaxBytes:  textprune.DefaultMaxBytes,
		MaxLines:  textprune.DefaultMaxLines,
		HeadBytes: toolOutputHeadBytes,
		TailBytes: toolOutputTailBytes,
		HeadLines: toolOutputHeadLines,
		TailLines: toolOutputTailLines,
		Marker:    textprune.DefaultMarker,
	})
}
