package container

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	mcpgw "github.com/memohai/memoh/internal/mcp"
)

// ReadResult contains the result of reading a file with pagination.
type ReadResult struct {
	Content              string
	LinesRead            int
	StartLine            int
	EndLine              int
	TotalLinesAvailable  int // -1 if unknown
	MaxLinesReached      bool
	MaxBytesReached      bool
	TruncatedLineNumbers []int
	EndOfFile            bool
}

// ReadFile reads a file inside the container with pagination support.
// It reads from line_offset (1-indexed) for up to n_lines lines.
// Limits: max 200 lines / 5KB per call (defined in prune.go).
func ReadFile(ctx context.Context, runner ExecRunner, botID, workDir, filePath string, lineOffset, nLines int) (*ReadResult, error) {
	if lineOffset < 1 {
		lineOffset = 1
	}
	if nLines < 1 {
		nLines = readMaxLines
	}
	if nLines > readMaxLines {
		nLines = readMaxLines
	}

	// Use sed to read specific line range efficiently.
	// sed -n '10,110p' file -> reads lines 10-110 (inclusive)
	endLine := lineOffset + nLines - 1
	sedCmd := fmt.Sprintf("sed -n '%d,%dp' %s", lineOffset, endLine, ShellQuote(filePath))

	result, err := runner.ExecWithCapture(ctx, mcpgw.ExecRequest{
		BotID:   botID,
		Command: []string{"/bin/sh", "-c", wrapWithCd(workDir, sedCmd)},
		WorkDir: workDir,
	})
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s", strings.TrimSpace(result.Stderr))
	}

	// Parse the output with line truncation.
	return parseReadOutput(result.Stdout, lineOffset, nLines, -1), nil
}

// parseReadOutput parses command output and applies line length limits.
func parseReadOutput(content string, startLine, requestedLines, totalLines int) *ReadResult {
	result := &ReadResult{
		StartLine:            startLine,
		TruncatedLineNumbers: []int{},
	}

	if content == "" {
		result.EndLine = startLine - 1
		// Empty result from sed means we've reached EOF (empty file or offset past end).
		result.EndOfFile = true
		result.TotalLinesAvailable = totalLines
		return result
	}

	var lines []string
	var nBytes int
	currentLine := startLine

	for i := 0; i < len(content); {
		if len(lines) >= readMaxLines {
			break
		}

		nextNewline := strings.IndexByte(content[i:], '\n')
		var line string
		if nextNewline < 0 {
			line = content[i:]
			i = len(content)
		} else {
			line = content[i : i+nextNewline]
			i += nextNewline + 1
		}

		// Apply max line length limit.
		wasTruncated := utf8.RuneCountInString(line) > readMaxLineLength
		truncatedLine := truncateLine(line, readMaxLineLength)
		if wasTruncated {
			result.TruncatedLineNumbers = append(result.TruncatedLineNumbers, currentLine)
		}

		// Format with line number like `cat -n`: 6-digit width, right-aligned, tab separator.
		formattedLine := fmt.Sprintf("%6d\t%s\n", currentLine, truncatedLine)

		// Check if adding this line would exceed max bytes.
		if nBytes+len(formattedLine) > readMaxBytes {
			result.MaxBytesReached = true
			break
		}

		lines = append(lines, formattedLine)
		nBytes += len(formattedLine)
		currentLine++
	}

	result.Content = strings.Join(lines, "")
	result.LinesRead = len(lines)
	result.EndLine = startLine + len(lines) - 1
	if result.EndLine < startLine {
		result.EndLine = startLine - 1
	}
	result.TotalLinesAvailable = totalLines
	if result.LinesRead >= readMaxLines {
		// Reaching max lines is only meaningful when there may be more data available.
		result.MaxLinesReached = totalLines < 0 || result.EndLine < totalLines
	}

	// Determine if we reached end of file.
	if totalLines >= 0 {
		result.EndOfFile = result.EndLine >= totalLines
	} else {
		// Without total lines info, assume EOF if we got fewer lines than requested.
		result.EndOfFile = len(lines) < requestedLines && !result.MaxBytesReached
	}

	return result
}

// FormatReadResult formats a ReadResult into the final output string.
func FormatReadResult(r *ReadResult) string {
	var buf bytes.Buffer

	if r.Content != "" {
		buf.WriteString(r.Content)
		// Ensure trailing newline if content doesn't end with one.
		if !strings.HasSuffix(r.Content, "\n") {
			buf.WriteByte('\n')
		}
	}

	// Build status message.
	var messages []string

	if r.LinesRead == 0 {
		if r.StartLine > 1 {
			messages = append(messages, fmt.Sprintf("No lines read from file (starting from line %d).", r.StartLine))
		} else {
			messages = append(messages, "File is empty.")
		}
	} else {
		if r.StartLine == r.EndLine {
			messages = append(messages, fmt.Sprintf("Read 1 line (line %d).", r.StartLine))
		} else {
			messages = append(messages, fmt.Sprintf("Read %d lines (%d-%d).",
				r.LinesRead, r.StartLine, r.EndLine))
		}
	}

	if r.MaxLinesReached {
		messages = append(messages, fmt.Sprintf("Limit %d lines reached.", readMaxLines))
	}
	if r.MaxBytesReached {
		messages = append(messages, fmt.Sprintf("Limit %d bytes reached.", readMaxBytes))
	}
	if r.EndOfFile {
		if !r.MaxLinesReached && !r.MaxBytesReached {
			messages = append(messages, "End of file.")
		}
	} else if r.TotalLinesAvailable > 0 {
		messages = append(messages, fmt.Sprintf("Total %d lines. Continue with line_offset=%d.",
			r.TotalLinesAvailable, r.EndLine+1))
	} else if !r.MaxLinesReached && !r.MaxBytesReached {
		// Unknown total but not EOF - suggest continue anyway
		messages = append(messages, fmt.Sprintf("Continue with line_offset=%d if more content exists.", r.EndLine+1))
	}

	if len(r.TruncatedLineNumbers) > 0 {
		messages = append(messages, fmt.Sprintf("Truncated: %s.", formatTruncatedLines(r.TruncatedLineNumbers)))
	}

	// Write status line.
	if len(messages) > 0 {
		buf.WriteString("\n")
		for i, msg := range messages {
			if i > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(msg)
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// ReadFileSimple reads an entire file without pagination (for backward compatibility/internal use).
// Suitable for small files only; applies pruning.
func ReadFileSimple(ctx context.Context, runner ExecRunner, botID, workDir, filePath string) (string, error) {
	result, err := runner.ExecWithCapture(ctx, mcpgw.ExecRequest{
		BotID:   botID,
		Command: []string{"/bin/sh", "-c", wrapWithCd(workDir, "cat "+ShellQuote(filePath))},
		WorkDir: workDir,
	})
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("%s", strings.TrimSpace(result.Stderr))
	}
	return pruneReadOutput(result.Stdout), nil
}

// CountLines counts lines in a file efficiently.
func CountLines(ctx context.Context, runner ExecRunner, botID, workDir, filePath string) (int, error) {
	// Use NR instead of wc -l so files without trailing '\n' are counted correctly.
	lineCountCmd := "awk 'END {print NR}' " + ShellQuote(filePath)
	result, err := runner.ExecWithCapture(ctx, mcpgw.ExecRequest{
		BotID:   botID,
		Command: []string{"/bin/sh", "-c", wrapWithCd(workDir, lineCountCmd)},
		WorkDir: workDir,
	})
	if err != nil {
		return 0, err
	}
	if result.ExitCode != 0 {
		return 0, fmt.Errorf("%s", strings.TrimSpace(result.Stderr))
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(result.Stdout), "%d", &count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// IsTextFile checks if a file appears to be a text file by checking for null bytes.
func IsTextFile(ctx context.Context, runner ExecRunner, botID, workDir, filePath string) (bool, error) {
	result, err := runner.ExecWithCapture(ctx, mcpgw.ExecRequest{
		BotID:   botID,
		Command: []string{"/bin/sh", "-c", wrapWithCd(workDir, "head -c 8192 "+ShellQuote(filePath))},
		WorkDir: workDir,
	})
	if err != nil {
		return false, err
	}
	if result.ExitCode != 0 {
		return false, fmt.Errorf("%s", strings.TrimSpace(result.Stderr))
	}

	return !bytes.Contains([]byte(result.Stdout), []byte{0}), nil
}
