package logging

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestDebugLogsAppearOnlyInVerboseMode(t *testing.T) {
	var stderr bytes.Buffer
	logger := New(&stderr, "", false)
	logger.Debug("hidden debug")
	logger.Info("visible info")

	output := stderr.String()
	if strings.Contains(output, "hidden debug") {
		t.Fatalf("debug log should not appear when verbose=false: %s", output)
	}
	if !strings.Contains(output, "visible info") {
		t.Fatalf("info log should appear: %s", output)
	}
}

func TestSensitiveValuesAreRedacted(t *testing.T) {
	var stderr bytes.Buffer
	logger := New(&stderr, "", true)
	logger.Info("config resolved", "api_key", "super-secret", "secret_env", "GEMINI_API_KEY")

	output := stderr.String()
	if strings.Contains(output, "super-secret") {
		t.Fatalf("secret value leaked into logs: %s", output)
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("expected redacted marker in logs: %s", output)
	}
}

func TestFileLoggingWritesToActivePath(t *testing.T) {
	storePath := t.TempDir()
	var stderr bytes.Buffer
	logger := New(&stderr, storePath, true)
	defer logger.Close()

	logger.Info("hello file log")
	bytes, err := os.ReadFile(ActiveLogPath(storePath))
	if err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}
	if !strings.Contains(string(bytes), "hello file log") {
		t.Fatalf("expected file log content, got %s", string(bytes))
	}
}
