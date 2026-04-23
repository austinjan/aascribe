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

func TestInitCreatesExpectedLayout(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer

	status := Run([]string{"--store", storePath, "init"}, &stdout)
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

	status := Run([]string{"--store", storePath, "init"}, &stdout)
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
	status := Run([]string{"--store", storePath, "init", "--force"}, &stdout)
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

	status := Run([]string{"--format", "text", "--store", storePath, "init"}, &stdout)
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

	status := Run([]string{"--store", storePath, "list"}, &stdout)
	if status != 1 {
		t.Fatalf("expected runtime error status, got %d", status)
	}
	if !strings.Contains(stdout.String(), "NOT_IMPLEMENTED") {
		t.Fatalf("expected NOT_IMPLEMENTED output, got %s", stdout.String())
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
