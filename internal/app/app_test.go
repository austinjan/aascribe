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

func TestParseDefaultFormatJSON(t *testing.T) {
	parsed, err := cli.Parse([]string{"list"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	if parsed.Format != cli.FormatJSON {
		t.Fatalf("expected default json format, got %q", parsed.Format)
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
	assertExists(t, filepath.Join(storePath, "outputs"))
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
	status := Run([]string{"--store", storePath, "output", "head", "out_000001", "--lines", "2"}, &stdout, &stderr)
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
	status := Run([]string{"--store", storePath, "output", "generate", "--lines", "300", "--width", "120"}, &stdout, &stderr)
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
