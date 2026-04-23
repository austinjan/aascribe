package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/store"
)

func TestParseInitStoreAndForce(t *testing.T) {
	parsed, err := cli.Parse([]string{"--store", "/tmp/demo", "init", "--force"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	if parsed.Store != "/tmp/demo" {
		t.Fatalf("expected explicit store, got %q", parsed.Store)
	}
	cmd, ok := parsed.Command.(cli.InitCommand)
	if !ok || !cmd.Force {
		t.Fatalf("expected init command with force=true, got %#v", parsed.Command)
	}
}

func TestParseDefaultFormatJSON(t *testing.T) {
	parsed, err := cli.Parse([]string{"list"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	if parsed.Format != cli.FormatJSON {
		t.Fatalf("expected default json format, got %q", parsed.Format)
	}
}

func TestResolveStorePathPrefersExplicitFlag(t *testing.T) {
	temp := t.TempDir()
	explicit := filepath.Join(temp, "explicit-store")
	envStore := filepath.Join(temp, "env-store")
	t.Setenv("AASCRIBE_STORE", envStore)

	resolved, err := store.ResolveStorePath(explicit)
	if err != nil {
		t.Fatalf("expected resolve success, got %v", err)
	}
	if resolved != explicit {
		t.Fatalf("expected explicit store %q, got %q", explicit, resolved)
	}
}

func TestResolveStorePathUsesEnvironmentWhenPresent(t *testing.T) {
	temp := t.TempDir()
	envStore := filepath.Join(temp, "env-store")
	t.Setenv("AASCRIBE_STORE", envStore)

	resolved, err := store.ResolveStorePath("")
	if err != nil {
		t.Fatalf("expected resolve success, got %v", err)
	}
	if resolved != envStore {
		t.Fatalf("expected env store %q, got %q", envStore, resolved)
	}
}

func TestResolveStorePathDefaultsToWorkingDirectoryDataMemory(t *testing.T) {
	temp := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory, got %v", err)
	}
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("failed to switch working directory: %v", err)
	}
	resolvedWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected resolved working directory, got %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}()

	t.Setenv("AASCRIBE_STORE", "")

	resolved, err := store.ResolveStorePath("")
	if err != nil {
		t.Fatalf("expected resolve success, got %v", err)
	}
	expected := filepath.Join(resolvedWD, "data", "memory")
	if resolved != expected {
		t.Fatalf("expected default store %q, got %q", expected, resolved)
	}
}

func TestInitCreatesExpectedLayout(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--store", storePath, "init"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d with output %s", status, stdout.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	data := payload["data"].(map[string]any)
	if data["store"] != storePath {
		t.Fatalf("expected store path %q, got %#v", storePath, data["store"])
	}
	if data["created"] != true {
		t.Fatalf("expected created=true, got %#v", data["created"])
	}
	assertExists(t, filepath.Join(storePath, "short_term"))
	assertExists(t, filepath.Join(storePath, "long_term"))
	assertExists(t, filepath.Join(storePath, "index"))
	assertExists(t, filepath.Join(storePath, "cache"))
	assertExists(t, filepath.Join(storePath, "layout.json"))
}

func TestInitFailsWithoutForceWhenStoreExists(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--store", storePath, "init"}, &stdout, &stderr)
	if status != 1 {
		t.Fatalf("expected runtime error status, got %d", status)
	}
	if !strings.Contains(stdout.String(), "STORE_ALREADY_EXISTS") {
		t.Fatalf("expected STORE_ALREADY_EXISTS output, got %s", stdout.String())
	}
}

func TestInitReinitializesWhenForceIsSet(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	assertNoError(t, os.MkdirAll(filepath.Join(storePath, "short_term"), 0o755))
	assertNoError(t, os.WriteFile(filepath.Join(storePath, "short_term", "old.txt"), []byte("stale"), 0o644))
	assertNoError(t, os.WriteFile(filepath.Join(storePath, "layout.json"), []byte(`{"layout_version":"old"}`), 0o644))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--store", storePath, "init", "--force"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d with output %s", status, stdout.String())
	}
	if _, err := os.Stat(filepath.Join(storePath, "short_term", "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected old managed file to be removed, got err=%v", err)
	}
}

func TestInitReportsTextOutput(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--format", "text", "--store", storePath, "init"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	expected := "Initialized aascribe store at " + storePath
	if strings.TrimSpace(stdout.String()) != expected {
		t.Fatalf("expected %q, got %q", expected, strings.TrimSpace(stdout.String()))
	}
}

func TestStubbedCommandReturnsNotImplemented(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	assertNoError(t, os.MkdirAll(storePath, 0o755))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--store", storePath, "list"}, &stdout, &stderr)
	if status != 1 {
		t.Fatalf("expected runtime error status, got %d", status)
	}
	if !strings.Contains(stdout.String(), "NOT_IMPLEMENTED") {
		t.Fatalf("expected NOT_IMPLEMENTED output, got %s", stdout.String())
	}
}

func TestVerboseLogsGoToStderrWithoutCorruptingStdoutJSON(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--verbose", "--store", storePath, "init"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout should remain valid json: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "command started") {
		t.Fatalf("expected lifecycle log on stderr, got %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "level=debug") && !strings.Contains(stderr.String(), "level=info") {
		t.Fatalf("expected structured log levels on stderr, got %s", stderr.String())
	}
}

func TestLogsPathReturnsExpectedFilePath(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--store", storePath, "logs", "path"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	data := payload["data"].(map[string]any)
	expected := filepath.Join(storePath, "logs", "aascribe.log")
	if data["path"] != expected {
		t.Fatalf("expected path %q, got %#v", expected, data["path"])
	}
}

func TestLogsExportCopiesLogFile(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	exportPath := filepath.Join(t.TempDir(), "exported", "aascribe.log")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--store", storePath, "init"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected init success, got %d", status)
	}
	stdout.Reset()
	stderr.Reset()
	status = Run([]string{"--store", storePath, "list"}, &stdout, &stderr)
	if status != 1 {
		t.Fatalf("expected stubbed list failure to still produce logs, got %d", status)
	}
	stdout.Reset()
	stderr.Reset()

	status = Run([]string{"--store", storePath, "logs", "export", "--output", exportPath}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected export success, got %d with stdout=%s stderr=%s", status, stdout.String(), stderr.String())
	}
	bytes, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("expected exported log file, got %v", err)
	}
	if len(bytes) == 0 {
		t.Fatalf("expected exported log file to have content")
	}
}

func TestLogsClearTruncatesLogFile(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--store", storePath, "init"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected init success, got %d", status)
	}
	stdout.Reset()
	stderr.Reset()
	status = Run([]string{"--store", storePath, "list"}, &stdout, &stderr)
	if status != 1 {
		t.Fatalf("expected stubbed list failure to still produce logs, got %d", status)
	}
	logPath := filepath.Join(storePath, "logs", "aascribe.log")
	infoBefore, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("expected log file to exist, got %v", err)
	}
	if infoBefore.Size() == 0 {
		t.Fatalf("expected non-empty log file before clear")
	}
	stdout.Reset()
	stderr.Reset()

	status = Run([]string{"--store", storePath, "logs", "clear", "--force"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected clear success, got %d", status)
	}
	infoAfter, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("expected log file to exist after clear, got %v", err)
	}
	if infoAfter.Size() != 0 {
		t.Fatalf("expected truncated log file, got size %d", infoAfter.Size())
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
