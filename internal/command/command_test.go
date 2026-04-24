package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/config"
	"github.com/austinjan/aascribe/internal/index"
	"github.com/austinjan/aascribe/internal/llm"
)

func TestDescribeWithFallbackUsesLocalSummaryWithoutConfig(t *testing.T) {
	storePath := t.TempDir()
	filePath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := describeWithFallback(storePath, cli.DescribeCommand{
		File:   filePath,
		Length: "medium",
		Focus:  "",
	})
	if err != nil {
		t.Fatalf("expected fallback success, got %v", err)
	}
	if result.Summary == "" {
		t.Fatalf("expected local summary, got %#v", result)
	}
}

func TestDescribeWithFallbackUsesPromptRunnerWhenConfigured(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
`)
	t.Setenv("GEMINI_API_KEY", "test-secret")

	filePath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	original := promptRunner
	promptRunner = func(resolved *config.Resolved, prompt string) (*llm.TextResponse, error) {
		if !strings.Contains(prompt, "Summary length target: short") {
			t.Fatalf("expected short length in prompt, got %q", prompt)
		}
		if !strings.Contains(prompt, "Prioritize this focus area: testing.") {
			t.Fatalf("expected focus in prompt, got %q", prompt)
		}
		return &llm.TextResponse{Text: "LLM generated summary."}, nil
	}
	defer func() {
		promptRunner = original
	}()

	result, err := describeWithFallback(storePath, cli.DescribeCommand{
		File:   filePath,
		Length: "short",
		Focus:  "testing",
	})
	if err != nil {
		t.Fatalf("expected configured describe success, got %v", err)
	}
	if result.Summary != "LLM generated summary." {
		t.Fatalf("expected LLM summary, got %#v", result)
	}
}

func TestRunIndexUsesPromptRunnerWhenConfigured(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
`)
	t.Setenv("GEMINI_API_KEY", "test-secret")

	root := t.TempDir()
	filePath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	original := promptRunner
	promptRunner = func(resolved *config.Resolved, prompt string) (*llm.TextResponse, error) {
		if !strings.Contains(prompt, "File path:") || !strings.Contains(prompt, "notes.txt") {
			t.Fatalf("expected file path in prompt, got %q", prompt)
		}
		return &llm.TextResponse{Text: "LLM index summary."}, nil
	}
	defer func() {
		promptRunner = original
	}()

	result, err := runIndex(storePath, cli.IndexCommand{
		Path:        root,
		Depth:       1,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected index success, got %v", err)
	}

	tree, ok := result.Data.(*index.PathIndexTree)
	if !ok {
		t.Fatalf("expected PathIndexTree, got %T", result.Data)
	}
	if len(tree.Tree.Children) != 1 || tree.Tree.Children[0].Summary != "LLM index summary." {
		t.Fatalf("expected LLM summary in index result, got %#v", tree.Tree.Children)
	}
}

func TestRunIndexMapReturnsAssembledMetadataTree(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "guide.txt"), []byte("hello docs\n"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if _, err := index.Build(index.Options{
		Root:        root,
		Depth:       4,
		MaxFileSize: 1024,
	}); err != nil {
		t.Fatalf("build metadata: %v", err)
	}

	result, err := runIndexMap(cli.IndexMapCommand{Path: root})
	if err != nil {
		t.Fatalf("expected index map success, got %v", err)
	}

	mapped, ok := result.Data.(*index.PathIndexMap)
	if !ok {
		t.Fatalf("expected PathIndexMap, got %T", result.Data)
	}
	if mapped.Root != root {
		t.Fatalf("expected root %q, got %q", root, mapped.Root)
	}
	if mapped.Map.State != "ready" {
		t.Fatalf("expected ready root metadata node, got %#v", mapped.Map)
	}
	if len(mapped.Map.Files) != 1 {
		t.Fatalf("expected root map files to stay local-only, got %#v", mapped.Map.Files)
	}
	if len(mapped.Map.Children) != 1 {
		t.Fatalf("expected one child metadata node, got %#v", mapped.Map.Children)
	}
	if filepath.Base(mapped.Map.Children[0].Path) != "docs" {
		t.Fatalf("expected docs child, got %#v", mapped.Map.Children)
	}
}

func writeConfig(t *testing.T, storePath, body string) {
	t.Helper()
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(config.ConfigPath(storePath), []byte(body), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
}
