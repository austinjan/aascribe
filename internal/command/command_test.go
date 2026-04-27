package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/config"
	"github.com/austinjan/aascribe/internal/index"
	"github.com/austinjan/aascribe/internal/llm"
	"github.com/austinjan/aascribe/internal/operation"
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

func TestRunIndexAsyncCreatesOperationAndStartsWorker(t *testing.T) {
	storePath := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var startedStore string
	var startedOperation string
	var startedCommand cli.IndexCommand
	original := startAsyncIndexProcess
	startAsyncIndexProcess = func(storePath, operationID string, cmd cli.IndexCommand) error {
		startedStore = storePath
		startedOperation = operationID
		startedCommand = cmd
		return nil
	}
	defer func() {
		startAsyncIndexProcess = original
	}()

	result, err := runIndex(storePath, cli.IndexCommand{
		Path:        root,
		Depth:       2,
		Concurrency: 3,
		Async:       true,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected async index success, got %v", err)
	}
	accepted, ok := result.Data.(*operation.Accepted)
	if !ok {
		t.Fatalf("expected OperationAccepted, got %T", result.Data)
	}
	if accepted.OperationID == "" || accepted.State != operation.StatePending {
		t.Fatalf("unexpected accepted payload: %#v", accepted)
	}
	if startedStore != storePath || startedOperation != accepted.OperationID || startedCommand.Path != root {
		t.Fatalf("expected worker start details, got store=%q op=%q cmd=%#v", startedStore, startedOperation, startedCommand)
	}

	status, err := operation.LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load operation status: %v", err)
	}
	if status.State != operation.StatePending || status.Stage != "queued" {
		t.Fatalf("unexpected initial async status: %#v", status)
	}
}

func TestRunOperationRunIndexCompletesOperationResult(t *testing.T) {
	storePath := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	accepted, err := operation.Create(storePath, operation.CreateInput{
		Command: "index",
		Stage:   "queued",
		Message: "Index operation accepted.",
		State:   operation.StatePending,
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	_, err = runOperationRunIndex(storePath, accepted.OperationID, cli.IndexCommand{
		Path:        root,
		Depth:       1,
		Concurrency: 1,
		NoSummary:   true,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("run operation index: %v", err)
	}

	status, err := operation.LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if status.State != operation.StateSucceeded || !status.ResultReady {
		t.Fatalf("expected succeeded status, got %#v", status)
	}
	result, err := operation.LoadResult(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load result: %v", err)
	}
	if result.State != operation.StateSucceeded || result.Data == nil {
		t.Fatalf("expected succeeded result data, got %#v", result)
	}

	encoded, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatalf("marshal result data: %v", err)
	}
	if !strings.Contains(string(encoded), "notes.txt") {
		t.Fatalf("expected indexed file in result data, got %s", string(encoded))
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

func TestRunOperationCommandsReturnStoredLifecycleData(t *testing.T) {
	storePath := t.TempDir()
	accepted, err := operation.Create(storePath, operation.CreateInput{
		Command: "index",
		Stage:   "starting",
		Message: "Preparing index operation.",
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	status, err := operation.LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	status.Stage = "indexing"
	status.Message = "Indexing files."
	status.ResultReady = true
	if err := operation.SaveStatus(storePath, status); err != nil {
		t.Fatalf("save status: %v", err)
	}
	if err := operation.AppendEvent(storePath, accepted.OperationID, operation.Event{
		Level:   "info",
		Stage:   "indexing",
		Message: "Indexed README.md",
	}); err != nil {
		t.Fatalf("append event: %v", err)
	}
	if err := operation.SaveResult(storePath, &operation.Result{
		OperationID: accepted.OperationID,
		State:       operation.StateSucceeded,
		CompletedAt: status.UpdatedAt,
		Truncated:   false,
		Data: map[string]any{
			"root": "/tmp/demo",
		},
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}

	listResult, err := runOperationList(storePath)
	if err != nil {
		t.Fatalf("run operation list: %v", err)
	}
	if !strings.Contains(listResult.Text, accepted.OperationID) {
		t.Fatalf("expected operation id in list text, got %q", listResult.Text)
	}

	statusResult, err := runOperationStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("run operation status: %v", err)
	}
	loadedStatus, ok := statusResult.Data.(*operation.Status)
	if !ok || loadedStatus.OperationID != accepted.OperationID {
		t.Fatalf("unexpected status payload: %#v", statusResult.Data)
	}

	eventsResult, err := runOperationEvents(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("run operation events: %v", err)
	}
	loadedEvents, ok := eventsResult.Data.(*operation.EventList)
	if !ok || len(loadedEvents.Events) != 1 {
		t.Fatalf("unexpected events payload: %#v", eventsResult.Data)
	}

	resultResult, err := runOperationResult(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("run operation result: %v", err)
	}
	loadedResult, ok := resultResult.Data.(*operation.Result)
	if !ok || loadedResult.OperationID != accepted.OperationID {
		t.Fatalf("unexpected result payload: %#v", resultResult.Data)
	}
}

func TestRunOperationCancelPersistsCanceledState(t *testing.T) {
	storePath := t.TempDir()
	accepted, err := operation.Create(storePath, operation.CreateInput{
		Command: "index",
		Stage:   "starting",
		Message: "Preparing index operation.",
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	result, err := runOperationCancel(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("run operation cancel: %v", err)
	}
	cancelResult, ok := result.Data.(*operation.CancelResult)
	if !ok || cancelResult.State != operation.StateCanceled {
		t.Fatalf("unexpected cancel payload: %#v", result.Data)
	}

	status, err := operation.LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if status.State != operation.StateCanceled {
		t.Fatalf("expected canceled status, got %#v", status)
	}
}

func TestOperationTextOutputsIncludeUsefulNextHints(t *testing.T) {
	storePath := t.TempDir()
	accepted, err := operation.Create(storePath, operation.CreateInput{
		Command: "index",
		Stage:   "queued",
		Message: "Index operation accepted.",
		State:   operation.StatePending,
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	listResult, err := runOperationList(storePath)
	if err != nil {
		t.Fatalf("operation list: %v", err)
	}
	if !strings.Contains(listResult.Text, "next: aascribe operation status <operation-id>") {
		t.Fatalf("expected operation list next hint, got %q", listResult.Text)
	}

	statusResult, err := runOperationStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("operation status: %v", err)
	}
	if !strings.Contains(statusResult.Text, "next: aascribe operation events "+accepted.OperationID) {
		t.Fatalf("expected operation status next hint, got %q", statusResult.Text)
	}

	if _, err := operation.Cancel(storePath, accepted.OperationID); err != nil {
		t.Fatalf("cancel operation: %v", err)
	}
	resultResult, err := runOperationResult(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("operation result: %v", err)
	}
	if !strings.Contains(resultResult.Text, "error: OPERATION_CANCELED") {
		t.Fatalf("expected canceled result detail, got %q", resultResult.Text)
	}
}

func TestOperationResultTextPointsOversizedDataToOutputTransport(t *testing.T) {
	storePath := t.TempDir()
	accepted, err := operation.Create(storePath, operation.CreateInput{
		Command: "index",
		Stage:   "complete",
		Message: "Index operation completed.",
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}
	if err := operation.SaveResult(storePath, &operation.Result{
		OperationID: accepted.OperationID,
		State:       operation.StateSucceeded,
		Data: map[string]any{
			"summary": strings.Repeat("large result line\n", 400),
		},
		Truncated: false,
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}

	result, err := runOperationResult(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("operation result: %v", err)
	}
	loaded, ok := result.Data.(*operation.Result)
	if !ok || loaded.OutputID == "" || !loaded.Truncated {
		t.Fatalf("expected output transport result, got %#v", result.Data)
	}
	if !strings.Contains(result.Text, "data: stored_output") {
		t.Fatalf("expected stored output marker, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "aascribe output show "+loaded.OutputID) {
		t.Fatalf("expected output show hint, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "aascribe output slice "+loaded.OutputID+" --offset 0 --limit 4000") {
		t.Fatalf("expected output slice hint, got %q", result.Text)
	}
}

func TestIndexManagementTextOutputsIncludeUsefulNextHints(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := index.Build(index.Options{
		Root:        root,
		Depth:       1,
		NoSummary:   true,
		MaxFileSize: 1024,
	}); err != nil {
		t.Fatalf("build index: %v", err)
	}

	evalResult, err := runIndexEval(cli.IndexEvalCommand{Path: root})
	if err != nil {
		t.Fatalf("index eval: %v", err)
	}
	if !strings.Contains(evalResult.Text, "next: aascribe map ") {
		t.Fatalf("expected eval map hint, got %q", evalResult.Text)
	}

	dirtyResult, err := runIndexDirty(cli.IndexDirtyCommand{Path: root})
	if err != nil {
		t.Fatalf("index dirty: %v", err)
	}
	if !strings.Contains(dirtyResult.Text, "next: aascribe index ") {
		t.Fatalf("expected dirty index hint, got %q", dirtyResult.Text)
	}

	cleanResult, err := runIndexClean(cli.IndexCleanCommand{Path: root, DryRun: true, Force: true})
	if err != nil {
		t.Fatalf("index clean: %v", err)
	}
	if !strings.Contains(cleanResult.Text, "next: aascribe index ") {
		t.Fatalf("expected clean index hint, got %q", cleanResult.Text)
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
