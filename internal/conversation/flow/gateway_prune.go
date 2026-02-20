package flow

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/memohai/memoh/internal/conversation"
)

const (
	// Prune long tool payloads per message to keep gateway requests within provider limits,
	// while preserving as much surrounding context as possible.
	gatewayToolResultMaxChars  = 64 * 1024
	gatewayToolResultHeadChars = 32 * 1024
	gatewayToolResultTailChars = 8 * 1024

	gatewayToolArgsMaxChars  = 16 * 1024
	gatewayToolArgsHeadChars = 8 * 1024
	gatewayToolArgsTailChars = 2 * 1024

	gatewayToolPayloadPrunedMarker = "[memoh pruned]"
)

func pruneHistoryForGateway(messages []messageWithUsage) []messageWithUsage {
	if len(messages) == 0 {
		return messages
	}
	out := make([]messageWithUsage, 0, len(messages))
	staleUsage := false
	for _, item := range messages {
		msg, changed := pruneMessageForGateway(item.Message)
		if changed {
			item.Message = msg
			staleUsage = true
		}
		if staleUsage {
			item.UsageInputTokens = nil
		}
		out = append(out, item)
	}
	return out
}

func pruneMessagesForGateway(messages []conversation.ModelMessage) []conversation.ModelMessage {
	if len(messages) == 0 {
		return messages
	}
	out := make([]conversation.ModelMessage, 0, len(messages))
	for _, msg := range messages {
		pruned, _ := pruneMessageForGateway(msg)
		out = append(out, pruned)
	}
	return out
}

func pruneMessageForGateway(msg conversation.ModelMessage) (conversation.ModelMessage, bool) {
	changed := false
	if strings.EqualFold(strings.TrimSpace(msg.Role), "tool") {
		msg2, did := pruneToolMessage(msg)
		if did {
			msg = msg2
			changed = true
		}
	}
	if len(msg.ToolCalls) > 0 {
		calls, did := pruneToolCalls(msg.ToolCalls)
		if did {
			msg.ToolCalls = calls
			changed = true
		}
	}
	return msg, changed
}

func pruneToolCalls(calls []conversation.ToolCall) ([]conversation.ToolCall, bool) {
	changed := false
	out := make([]conversation.ToolCall, len(calls))
	for i, call := range calls {
		out[i] = call
		args := call.Function.Arguments
		if args == "" || len(args) <= gatewayToolArgsMaxChars {
			continue
		}
		out[i].Function.Arguments = pruneStringEdges(
			args,
			gatewayToolArgsHeadChars,
			gatewayToolArgsTailChars,
			"tool arguments",
		)
		changed = true
	}
	return out, changed
}

func pruneToolMessage(msg conversation.ModelMessage) (conversation.ModelMessage, bool) {
	// Vercel AI SDK schema requires tool messages to carry an array of tool-result parts.
	// Prune outputs inside those parts (preserving shape) so the gateway prompt remains valid.
	if pruned, ok := pruneToolResultParts(msg.Content); ok {
		msg.Content = pruned
		return msg, true
	}

	// Backward-compat: tool messages may have been persisted as plain strings.
	text := msg.TextContent()
	if len(text) <= gatewayToolResultMaxChars {
		return msg, false
	}
	msg.Content = conversation.NewTextContent(pruneStringEdges(
		text,
		gatewayToolResultHeadChars,
		gatewayToolResultTailChars,
		"tool result",
	))
	return msg, true
}

type toolResultPart struct {
	Type       string     `json:"type"`
	ToolCallID string     `json:"toolCallId"`
	ToolName   string     `json:"toolName"`
	Output     toolOutput `json:"output"`
}

type toolOutput struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value,omitempty"`
}

func pruneToolResultParts(content json.RawMessage) (json.RawMessage, bool) {
	if len(content) == 0 {
		return nil, false
	}
	var parts []json.RawMessage
	if err := json.Unmarshal(content, &parts); err != nil || len(parts) == 0 {
		return nil, false
	}

	changed := false
	out := make([]json.RawMessage, 0, len(parts))
	for _, raw := range parts {
		var part toolResultPart
		if err := json.Unmarshal(raw, &part); err != nil || part.Type != "tool-result" {
			out = append(out, raw)
			continue
		}

		pruned, didPrune := pruneToolOutput(part.Output)
		if !didPrune {
			out = append(out, raw)
			continue
		}

		part.Output = pruned
		rebuilt, err := json.Marshal(part)
		if err != nil {
			out = append(out, raw)
			continue
		}
		out = append(out, json.RawMessage(rebuilt))
		changed = true
	}

	if !changed {
		return nil, false
	}
	rebuilt, err := json.Marshal(out)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(rebuilt), true
}

func pruneToolOutput(output toolOutput) (toolOutput, bool) {
	switch output.Type {
	case "text", "error-text":
		var s string
		if err := json.Unmarshal(output.Value, &s); err != nil || len(s) <= gatewayToolResultMaxChars {
			return output, false
		}
		s = pruneStringEdges(s, gatewayToolResultHeadChars, gatewayToolResultTailChars, "tool result")
		data, err := json.Marshal(s)
		if err != nil {
			return output, false
		}
		output.Value = data
		return output, true

	case "json", "error-json":
		if len(output.Value) <= gatewayToolResultMaxChars {
			return output, false
		}
		pruned := pruneStringEdges(
			string(output.Value),
			gatewayToolResultHeadChars,
			gatewayToolResultTailChars,
			"tool result (json)",
		)
		data, err := json.Marshal(pruned)
		if err != nil {
			return output, false
		}
		output.Value = data
		return output, true

	case "content":
		// Best-effort: prune any large text items inside the content array.
		// If parsing fails, keep the original output to avoid breaking schema.
		var items []map[string]any
		if err := json.Unmarshal(output.Value, &items); err != nil {
			return output, false
		}
		didPrune := false
		for i := range items {
			if items[i]["type"] != "text" {
				continue
			}
			textAny, ok := items[i]["text"]
			if !ok {
				continue
			}
			text, ok := textAny.(string)
			if !ok || len(text) <= gatewayToolResultMaxChars {
				continue
			}
			items[i]["text"] = pruneStringEdges(text, gatewayToolResultHeadChars, gatewayToolResultTailChars, "tool result (content)")
			didPrune = true
		}
		if !didPrune {
			return output, false
		}
		data, err := json.Marshal(items)
		if err != nil {
			return output, false
		}
		output.Value = data
		return output, true

	default:
		return output, false
	}
}

func pruneStringEdges(s string, headChars, tailChars int, label string) string {
	if headChars < 0 {
		headChars = 0
	}
	if tailChars < 0 {
		tailChars = 0
	}
	if headChars+tailChars <= 0 || len(s) == 0 {
		return fmt.Sprintf("%s %s omitted (len=%d)", gatewayToolPayloadPrunedMarker, label, len(s))
	}
	if len(s) <= headChars+tailChars {
		return s
	}
	head := safeUTF8Prefix(s, minInt(headChars, len(s)))
	tail := ""
	if tailChars > 0 {
		tail = safeUTF8Suffix(s, minInt(tailChars, len(s)))
	}
	return fmt.Sprintf(
		"%s %s too long (len=%d), showing head/tail\n\n%s\n\n[...snip...]\n\n%s",
		gatewayToolPayloadPrunedMarker,
		label,
		len(s),
		head,
		tail,
	)
}

func safeUTF8Prefix(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) == 0 {
		return ""
	}
	if maxBytes >= len(s) {
		return s
	}
	cut := maxBytes
	for cut > 0 && cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if cut <= 0 {
		return ""
	}
	return s[:cut]
}

func safeUTF8Suffix(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) == 0 {
		return ""
	}
	if maxBytes >= len(s) {
		return s
	}
	start := len(s) - maxBytes
	if start < 0 {
		start = 0
	}
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	if start >= len(s) {
		return ""
	}
	return s[start:]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
