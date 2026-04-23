package logging

import (
	"strings"
	"testing"
)

func TestFormatLineQuotesValuesContainingEquals(t *testing.T) {
	line := formatLine(LevelError, "parse failed", "error_code", "INVALID_ARGUMENTS", "error_message", "No subcommand provided.")

	if !strings.Contains(line, `error_code=INVALID_ARGUMENTS`) {
		t.Fatalf("expected error_code field, got %q", line)
	}
	if !strings.Contains(line, `error_message="No subcommand provided."`) {
		t.Fatalf("expected quoted error_message field, got %q", line)
	}
}
