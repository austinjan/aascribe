package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/operation"
	"github.com/austinjan/aascribe/internal/store"
	"github.com/austinjan/aascribe/pkg/llmoutput"
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

func TestParseDefaultFormatText(t *testing.T) {
	parsed, err := cli.Parse([]string{"list"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	if parsed.Format != cli.FormatText {
		t.Fatalf("expected default text format, got %q", parsed.Format)
	}
}

func TestParseChatPrompt(t *testing.T) {
	parsed, err := cli.Parse([]string{"chat", "hello"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.ChatCommand)
	if !ok {
		t.Fatalf("expected chat command, got %#v", parsed.Command)
	}
	if cmd.Prompt != "hello" {
		t.Fatalf("expected prompt hello, got %q", cmd.Prompt)
	}
}

func TestParseSummarizeFile(t *testing.T) {
	parsed, err := cli.Parse([]string{"summarize", "README.md"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.SummarizeCommand)
	if !ok {
		t.Fatalf("expected summarize command, got %#v", parsed.Command)
	}
	if cmd.File != "README.md" {
		t.Fatalf("expected file README.md, got %q", cmd.File)
	}
}

func TestParseOutputSlice(t *testing.T) {
	parsed, err := cli.Parse([]string{"output", "slice", "out_000001", "--offset", "10", "--limit", "20"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.OutputSliceCommand)
	if !ok {
		t.Fatalf("expected output slice command, got %#v", parsed.Command)
	}
	if cmd.ID != "out_000001" || cmd.Offset != 10 || cmd.Limit != 20 {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseOutputGenerate(t *testing.T) {
	parsed, err := cli.Parse([]string{"output", "generate", "--lines", "20", "--width", "40", "--prefix", "demo"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.OutputGenerateCommand)
	if !ok {
		t.Fatalf("expected output generate command, got %#v", parsed.Command)
	}
	if cmd.Lines != 20 || cmd.Width != 40 || cmd.Prefix != "demo" {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseOperationStatus(t *testing.T) {
	parsed, err := cli.Parse([]string{"operation", "status", "op_demo"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.OperationStatusCommand)
	if !ok {
		t.Fatalf("expected operation status command, got %#v", parsed.Command)
	}
	if cmd.ID != "op_demo" {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseOperationCancel(t *testing.T) {
	parsed, err := cli.Parse([]string{"operation", "cancel", "op_demo"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.OperationCancelCommand)
	if !ok {
		t.Fatalf("expected operation cancel command, got %#v", parsed.Command)
	}
	if cmd.ID != "op_demo" {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseOperationCleanDefaultsToDryRun(t *testing.T) {
	parsed, err := cli.Parse([]string{"operation", "clean"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.OperationCleanCommand)
	if !ok {
		t.Fatalf("expected operation clean command, got %#v", parsed.Command)
	}
	if !cmd.DryRun || cmd.Force {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseOperationCleanForce(t *testing.T) {
	parsed, err := cli.Parse([]string{"operation", "clean", "--force"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.OperationCleanCommand)
	if !ok {
		t.Fatalf("expected operation clean command, got %#v", parsed.Command)
	}
	if cmd.DryRun || !cmd.Force {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseIndexClean(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "clean", "./tests", "--dry-run", "--force"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexCleanCommand)
	if !ok {
		t.Fatalf("expected index clean command, got %#v", parsed.Command)
	}
	if cmd.Path != "./tests" || !cmd.DryRun || !cmd.Force {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseIndexConcurrency(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "--concurrency", "6", "./tests"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexCommand)
	if !ok {
		t.Fatalf("expected index command, got %#v", parsed.Command)
	}
	if cmd.Path != "./tests" || cmd.Concurrency != 6 {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseIndexAsync(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "--async", "./tests"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexCommand)
	if !ok {
		t.Fatalf("expected index command, got %#v", parsed.Command)
	}
	if cmd.Path != "./tests" || !cmd.Async {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseIndexMap(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "map", "./tests"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.MapCommand)
	if !ok {
		t.Fatalf("expected map command, got %#v", parsed.Command)
	}
	if cmd.Path != "./tests" {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseIndexDirty(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "dirty", "./tests"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexDirtyCommand)
	if !ok {
		t.Fatalf("expected index dirty command, got %#v", parsed.Command)
	}
	if cmd.Path != "./tests" {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseIndexEval(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "eval", "./tests"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexEvalCommand)
	if !ok {
		t.Fatalf("expected index eval command, got %#v", parsed.Command)
	}
	if cmd.Path != "./tests" {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestParseMap(t *testing.T) {
	parsed, err := cli.Parse([]string{"map", "./tests"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.MapCommand)
	if !ok {
		t.Fatalf("expected map command, got %#v", parsed.Command)
	}
	if cmd.Path != "./tests" {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
}

func TestRunWithoutArgsPrintsHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run(nil, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected help text, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunTopLevelHelpIncludesAgentGuidance(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "What This CLI Does:") {
		t.Fatalf("expected agent-oriented help section, got %q", rendered)
	}
	if !strings.Contains(rendered, "How To Get More Information:") {
		t.Fatalf("expected next-step guidance, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe <command> --help") {
		t.Fatalf("expected follow-up help guidance, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunLogsHelpIncludesSubcommandsAndExamples(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"logs", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "Subcommands:") {
		t.Fatalf("expected logs subcommands section, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe logs export --output ./aascribe.log") {
		t.Fatalf("expected logs example, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunOutputHelpIncludesSubcommandsAndExamples(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"output", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "Subcommands:") {
		t.Fatalf("expected output subcommands section, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe output generate") {
		t.Fatalf("expected output generate example, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe output slice out_000001 --offset 4000 --limit 4000") {
		t.Fatalf("expected output slice example, got %q", rendered)
	}
}

func TestRunOperationHelpIncludesSubcommandsAndExamples(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"operation", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "Subcommands:") {
		t.Fatalf("expected operation subcommands section, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe operation status") {
		t.Fatalf("expected operation status example, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunOperationStatusReturnsJSONPayload(t *testing.T) {
	storePath := t.TempDir()
	accepted, err := operation.Create(storePath, operation.CreateInput{
		Command: "index",
		Stage:   "starting",
		Message: "Preparing index operation.",
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "--store", storePath, "operation", "status", accepted.OperationID}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d with stdout=%s stderr=%s", status, stdout.String(), stderr.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			OperationID string `json:"operation_id"`
			Command     string `json:"command"`
			State       string `json:"state"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	if !payload.OK || payload.Data.OperationID != accepted.OperationID || payload.Data.Command != "index" {
		t.Fatalf("unexpected payload: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "command started") || !strings.Contains(stderr.String(), "command finished") {
		t.Fatalf("expected command lifecycle logs on stderr, got %q", stderr.String())
	}
}

func TestRunOperationCancelReturnsJSONPayload(t *testing.T) {
	storePath := t.TempDir()
	accepted, err := operation.Create(storePath, operation.CreateInput{
		Command: "index",
		Stage:   "starting",
		Message: "Preparing index operation.",
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "--store", storePath, "operation", "cancel", accepted.OperationID}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d with stdout=%s stderr=%s", status, stdout.String(), stderr.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			OperationID string `json:"operation_id"`
			State       string `json:"state"`
			Message     string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	if !payload.OK || payload.Data.OperationID != accepted.OperationID || payload.Data.State != "canceled" {
		t.Fatalf("unexpected payload: %s", stdout.String())
	}
}

func TestRunLogsExportHelpIncludesRequiredFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"logs", "export", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "Required Flags:") {
		t.Fatalf("expected required flags section, got %q", rendered)
	}
	if !strings.Contains(rendered, "--output <path>") {
		t.Fatalf("expected output flag guidance, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunChatHelpIncludesPurpose(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"chat", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "send one prompt directly to the configured LLM") {
		t.Fatalf("expected chat help text, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe chat \"Say hello in one short sentence.\"") {
		t.Fatalf("expected chat example, got %q", rendered)
	}
}

func TestRunSummarizeHelpIncludesPurpose(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"summarize", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "summarize one file through the configured LLM") {
		t.Fatalf("expected summarize help text, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe summarize ./README.md") {
		t.Fatalf("expected summarize example, got %q", rendered)
	}
}

func TestRunIndexHelpMentionsIgnoreFiles(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"index", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, ".gitignore") || !strings.Contains(rendered, ".aaignore") {
		t.Fatalf("expected index help to mention ignore files, got %q", rendered)
	}
	if !strings.Contains(rendered, "--concurrency") {
		t.Fatalf("expected index help to mention concurrency, got %q", rendered)
	}
	if !strings.Contains(rendered, ".aascribe_index_meta.json") {
		t.Fatalf("expected index help to mention metadata file, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe index --depth 2 ./internal") {
		t.Fatalf("expected index help examples to reflect current flag ordering, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunIndexCleanHelpMentionsForce(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"index", "clean", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "--force") || !strings.Contains(rendered, ".aascribe_index_meta.json") {
		t.Fatalf("expected index clean help text, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunIndexDirtyHelpMentionsDirtyMetadata(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"index", "dirty", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, ".aascribe_index_meta.json") || !strings.Contains(rendered, "stale") {
		t.Fatalf("expected index dirty help text, got %q", rendered)
	}
}

func TestRunIndexEvalHelpMentionsPreview(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"index", "eval", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "preview") || !strings.Contains(rendered, "unchanged") {
		t.Fatalf("expected index eval help text, got %q", rendered)
	}
}

func TestRunDescribeReturnsFileDescription(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.conf")
	if err := os.WriteFile(path, []byte("host=localhost\nport=8080\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--format", "json", "describe", "--length", "short", "--focus", "configuration", path}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Path        string `json:"path"`
			Summary     string `json:"summary"`
			Length      string `json:"length"`
			Focus       string `json:"focus"`
			GeneratedAt string `json:"generated_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok response, got %s", stdout.String())
	}
	if payload.Data.Path != filepath.ToSlash(path) {
		t.Fatalf("expected path %q, got %q", filepath.ToSlash(path), payload.Data.Path)
	}
	if payload.Data.Length != "short" || payload.Data.Focus != "configuration" {
		t.Fatalf("unexpected data payload: %#v", payload.Data)
	}
	if payload.Data.Summary == "" || payload.Data.GeneratedAt == "" {
		t.Fatalf("expected summary and generated_at, got %#v", payload.Data)
	}
}

func TestRunIndexReturnsTree(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--format", "json", "index", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Root string `json:"root"`
			Tree struct {
				Path     string `json:"path"`
				Type     string `json:"type"`
				Children []struct {
					Path    string `json:"path"`
					Type    string `json:"type"`
					Summary string `json:"summary"`
				} `json:"children"`
			} `json:"tree"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok response, got %s", stdout.String())
	}
	if payload.Data.Root != root {
		t.Fatalf("expected root %q, got %q", root, payload.Data.Root)
	}
	if payload.Data.Tree.Type != "dir" {
		t.Fatalf("expected root dir, got %q", payload.Data.Tree.Type)
	}
	if len(payload.Data.Tree.Children) != 1 {
		t.Fatalf("expected one child, got %d", len(payload.Data.Tree.Children))
	}
	if payload.Data.Tree.Children[0].Type != "file" {
		t.Fatalf("expected file child, got %q", payload.Data.Tree.Children[0].Type)
	}
}

func TestRunIndexMapReturnsAssembledMap(t *testing.T) {
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

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", root}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "json", "index", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Root string `json:"root"`
			Map  struct {
				Path     string `json:"path"`
				State    string `json:"state"`
				Children []struct {
					Path  string `json:"path"`
					State string `json:"state"`
				} `json:"children"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok response, got %s", stdout.String())
	}
	if payload.Data.Root != root {
		t.Fatalf("expected root %q, got %q", root, payload.Data.Root)
	}
	if payload.Data.Map.State != "ready" {
		t.Fatalf("expected ready map state, got %#v", payload.Data.Map)
	}
	if len(payload.Data.Map.Children) != 1 {
		t.Fatalf("expected one child, got %#v", payload.Data.Map.Children)
	}
	if filepath.Base(payload.Data.Map.Children[0].Path) != "docs" {
		t.Fatalf("expected docs child, got %#v", payload.Data.Map.Children)
	}
	if strings.Contains(stdout.String(), "\"last_updated\"") || strings.Contains(stdout.String(), "\"content_hash\"") {
		t.Fatalf("expected map output to omit low-signal metadata fields, got %s", stdout.String())
	}
}

func TestRunMapReturnsAssembledMap(t *testing.T) {
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

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", root}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Root string `json:"root"`
			Map  struct {
				Path     string `json:"path"`
				State    string `json:"state"`
				Children []struct {
					Path  string `json:"path"`
					State string `json:"state"`
				} `json:"children"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok response, got %s", stdout.String())
	}
	if payload.Data.Root != root {
		t.Fatalf("expected root %q, got %q", root, payload.Data.Root)
	}
	if payload.Data.Map.State != "ready" {
		t.Fatalf("expected ready map state, got %#v", payload.Data.Map)
	}
	if len(payload.Data.Map.Children) != 1 {
		t.Fatalf("expected one child, got %#v", payload.Data.Map.Children)
	}
	if filepath.Base(payload.Data.Map.Children[0].Path) != "docs" {
		t.Fatalf("expected docs child, got %#v", payload.Data.Map.Children)
	}
	if strings.Contains(stdout.String(), "\"last_updated\"") || strings.Contains(stdout.String(), "\"content_hash\"") {
		t.Fatalf("expected map output to omit low-signal metadata fields, got %s", stdout.String())
	}
}

func TestRunMapOmitsIgnoredDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "guide.txt"), []byte("hello docs\n"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ignored"), 0o755); err != nil {
		t.Fatalf("mkdir ignored: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored", "secret.txt"), []byte("secret\n"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", root}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Map struct {
				Children []struct {
					Path string `json:"path"`
				} `json:"children"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok response, got %s", stdout.String())
	}
	for _, child := range payload.Data.Map.Children {
		if filepath.Base(child.Path) == "ignored" {
			t.Fatalf("expected ignored directory to be omitted from map, got %#v", payload.Data.Map.Children)
		}
	}
}

func TestRunMapTextReturnsCompactTree(t *testing.T) {
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

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", root}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "text", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	rendered := stdout.String()
	if !strings.Contains(rendered, "map is a routing overview") {
		t.Fatalf("expected routing precision guidance in text output, got %q", rendered)
	}
	if !strings.Contains(rendered, filepath.Clean(root)) {
		t.Fatalf("expected root path in text output, got %q", rendered)
	}
	if !strings.Contains(rendered, "summary:") {
		t.Fatalf("expected summary line in text output, got %q", rendered)
	}
	if !strings.Contains(rendered, "main.go") || !strings.Contains(rendered, "docs") {
		t.Fatalf("expected compact tree entries in text output, got %q", rendered)
	}
	if strings.Contains(rendered, "content_hash") || strings.Contains(rendered, "last_updated") || strings.Contains(rendered, "{") {
		t.Fatalf("expected compact text tree instead of JSON-ish output, got %q", rendered)
	}
}

func TestRunMapUnindexedNodeUsesSimpleStateAndGuide(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "level1", "level2"), 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "level1", "guide.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", "--depth", "2", root}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected map success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			StateGuide map[string]string `json:"state_guide"`
			Map        struct {
				Children []struct {
					Path     string `json:"path"`
					Children []struct {
						Path     string `json:"path"`
						Children []struct {
							Path  string `json:"path"`
							State string `json:"state"`
						} `json:"children"`
					} `json:"children"`
				} `json:"children"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok response, got %s", stdout.String())
	}
	level2 := payload.Data.Map.Children[0].Children[0].Children[0]
	if filepath.Base(level2.Path) != "level2" {
		t.Fatalf("expected level2 node, got %#v", level2)
	}
	if level2.State != "unindexed" {
		t.Fatalf("expected unindexed state, got %#v", level2)
	}
	if payload.Data.StateGuide["unindexed"] == "" || payload.Data.StateGuide["ready"] == "" || payload.Data.StateGuide["precision"] == "" {
		t.Fatalf("expected state guide entries, got %#v", payload.Data.StateGuide)
	}
}

func TestRunMapTextShowsStateGuideAndUnindexedState(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "level1", "level2"), 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "level1", "guide.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", "--depth", "2", root}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "text", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected map success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	rendered := stdout.String()
	if !strings.Contains(rendered, "state guide:") {
		t.Fatalf("expected state guide in text output, got %q", rendered)
	}
	if !strings.Contains(rendered, "unindexed:") {
		t.Fatalf("expected unindexed guide entry in text output, got %q", rendered)
	}
	if !strings.Contains(rendered, "level2 [unindexed]") {
		t.Fatalf("expected unindexed node in text output, got %q", rendered)
	}
}

func TestRunIndexDirtyMarksMetadataDirty(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", root}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "json", "index", "dirty", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected dirty success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	content, err := os.ReadFile(filepath.Join(root, ".aascribe_index_meta.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(content), "\"dirty\": true") {
		t.Fatalf("expected dirty metadata file, got %s", string(content))
	}
}

func TestRunIndexEvalReturnsChangedAndUnchangedStates(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "change.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write change: %v", err)
	}

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", root}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}
	if err := os.WriteFile(filepath.Join(root, "change.txt"), []byte("after\n"), 0o644); err != nil {
		t.Fatalf("rewrite change: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "json", "index", "eval", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected eval success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Files []struct {
				Path  string `json:"path"`
				State string `json:"state"`
			} `json:"files"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("expected ok response, got %s", stdout.String())
	}
	states := map[string]string{}
	for _, file := range payload.Data.Files {
		states[filepath.Base(file.Path)] = file.State
	}
	if states["change.txt"] != "needs_index" || states["keep.txt"] != "unchanged" {
		t.Fatalf("unexpected eval file states: %#v", states)
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

	status := Run([]string{"--format", "json", "--store", storePath, "init"}, &stdout, &stderr)
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
	assertExists(t, filepath.Join(storePath, "outputs"))
	assertExists(t, filepath.Join(storePath, "operations"))
	assertExists(t, filepath.Join(storePath, "layout.json"))
}

func TestInitFailsWithoutForceWhenStoreExists(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--format", "json", "--store", storePath, "init"}, &stdout, &stderr)
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
	rendered := strings.TrimSpace(stdout.String())
	if !strings.Contains(rendered, expected) {
		t.Fatalf("expected %q in output, got %q", expected, rendered)
	}
	if !strings.Contains(rendered, "next: aascribe logs path") {
		t.Fatalf("expected next-step hint, got %q", rendered)
	}
}

func TestStubbedCommandReturnsNotImplemented(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	assertNoError(t, os.MkdirAll(storePath, 0o755))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--format", "json", "--store", storePath, "list"}, &stdout, &stderr)
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

	status := Run([]string{"--format", "json", "--verbose", "--store", storePath, "init"}, &stdout, &stderr)
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

	status := Run([]string{"--format", "json", "--store", storePath, "logs", "path"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	data := payload["data"].(map[string]any)
	expected := filepath.Join(filepath.Dir(storePath), "logs", "aascribe.log")
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
	logPath := filepath.Join(filepath.Dir(storePath), "logs", "aascribe.log")
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

func TestOutputHeadReadsStoredOutput(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	assertNoError(t, os.MkdirAll(storePath, 0o755))
	_, err := llmoutput.Deliver(storePath, "chat", "one\ntwo\nthree\nfour", llmoutput.Config{InlineRuneLimit: 4})
	if err != nil {
		t.Fatalf("expected stored output, got %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "--store", storePath, "output", "head", "out_000001", "--lines", "2"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected output head success, got %d with stdout=%s stderr=%s", status, stdout.String(), stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	data := payload["data"].(map[string]any)
	if !strings.Contains(data["text"].(string), "one\ntwo") {
		t.Fatalf("expected head output, got %#v", data["text"])
	}
}

func TestOutputGenerateSpillsAndReturnsTransportHint(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	assertNoError(t, os.MkdirAll(storePath, 0o755))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "--store", storePath, "output", "generate", "--lines", "300", "--width", "120"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected output generate success, got %d with stdout=%s stderr=%s", status, stdout.String(), stderr.String())
	}

	var payload struct {
		Data struct {
			Text      string `json:"text"`
			Transport struct {
				Truncated bool   `json:"truncated"`
				OutputID  string `json:"output_id"`
			} `json:"transport"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	if !payload.Data.Transport.Truncated {
		t.Fatalf("expected truncated transport metadata")
	}
	if payload.Data.Transport.OutputID == "" {
		t.Fatalf("expected output id")
	}
	if payload.Data.Text == "" {
		t.Fatalf("expected inline text")
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
