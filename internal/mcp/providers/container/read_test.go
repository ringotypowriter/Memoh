package container

import (
	"context"
	"strings"
	"testing"

	mcpgw "github.com/memohai/memoh/internal/mcp"
)

type scriptedReadRunner struct {
	handler func(req mcpgw.ExecRequest) (*mcpgw.ExecWithCaptureResult, error)
	calls   []mcpgw.ExecRequest
}

func (r *scriptedReadRunner) ExecWithCapture(ctx context.Context, req mcpgw.ExecRequest) (*mcpgw.ExecWithCaptureResult, error) {
	r.calls = append(r.calls, req)
	return r.handler(req)
}

func TestParseReadOutput_LongSingleLine(t *testing.T) {
	longLine := strings.Repeat("a", 2*1024*1024) // 2MB single line without '\n'

	result := parseReadOutput(longLine, 1, readMaxLines, 1)

	if result.LinesRead != 1 {
		t.Fatalf("LinesRead = %d, want 1", result.LinesRead)
	}
	if result.EndLine != 1 {
		t.Fatalf("EndLine = %d, want 1", result.EndLine)
	}
	if !result.EndOfFile {
		t.Fatalf("EndOfFile = false, want true")
	}
	if result.MaxBytesReached {
		t.Fatalf("MaxBytesReached = true, want false")
	}
	if len(result.TruncatedLineNumbers) != 1 || result.TruncatedLineNumbers[0] != 1 {
		t.Fatalf("TruncatedLineNumbers = %v, want [1]", result.TruncatedLineNumbers)
	}
	if !strings.Contains(result.Content, "\t"+strings.Repeat("a", readMaxLineLength)+"...\n") {
		t.Fatalf("content does not contain expected truncated output, got: %q", result.Content)
	}
}

func TestParseReadOutput_TruncationMarkerForNearThresholdLine(t *testing.T) {
	// 1001 ASCII chars: truncation happens, but output becomes 1003 chars due to "...".
	// This verifies truncation tracking doesn't rely on byte-length shrinkage.
	line := strings.Repeat("x", readMaxLineLength+1)

	result := parseReadOutput(line, 1, readMaxLines, 1)

	if len(result.TruncatedLineNumbers) != 1 || result.TruncatedLineNumbers[0] != 1 {
		t.Fatalf("TruncatedLineNumbers = %v, want [1]", result.TruncatedLineNumbers)
	}

	formatted := FormatReadResult(result)
	if !strings.Contains(formatted, "Truncated: 1.") {
		t.Fatalf("formatted output missing truncation marker, got: %q", formatted)
	}
}

func TestParseReadOutput_EmptyContentWithoutTotalMarksEOF(t *testing.T) {
	result := parseReadOutput("", 401, readMaxLines, -1)

	if !result.EndOfFile {
		t.Fatalf("EndOfFile = false, want true")
	}
	if result.LinesRead != 0 {
		t.Fatalf("LinesRead = %d, want 0", result.LinesRead)
	}

	formatted := FormatReadResult(result)
	if strings.Contains(formatted, "Continue with line_offset=") {
		t.Fatalf("formatted output should not contain continuation hint, got: %q", formatted)
	}
}

func TestReadFile_DoesNotScanWholeFileForTotalLines(t *testing.T) {
	runner := &scriptedReadRunner{}
	runner.handler = func(req mcpgw.ExecRequest) (*mcpgw.ExecWithCaptureResult, error) {
		cmd := strings.Join(req.Command, " ")
		switch {
		case strings.Contains(cmd, "sed -n"):
			return &mcpgw.ExecWithCaptureResult{Stdout: strings.Repeat("line\n", readMaxLines), ExitCode: 0}, nil
		default:
			t.Fatalf("unexpected command: %q", cmd)
			return nil, nil
		}
	}

	result, err := ReadFile(context.Background(), runner, "bot-1", "/data", "test.txt", 201, 200)
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalLinesAvailable != -1 {
		t.Fatalf("TotalLinesAvailable = %d, want -1", result.TotalLinesAvailable)
	}
	if result.EndOfFile {
		t.Fatalf("EndOfFile = true, want false")
	}
	if result.LinesRead != 200 {
		t.Fatalf("LinesRead = %d, want 200", result.LinesRead)
	}

	for _, req := range runner.calls {
		cmd := strings.Join(req.Command, " ")
		if strings.Contains(cmd, "awk 'END {print NR}'") || strings.Contains(cmd, "wc -l") {
			t.Fatalf("unexpected full-file line-count command: %q", cmd)
		}
	}
}
