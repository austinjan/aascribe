package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		return &llm.TextResponse{
			Text: "LLM index summary.",
			Usage: llm.Usage{
				PromptTokenCount:     11,
				CandidatesTokenCount: 7,
				TotalTokenCount:      18,
			},
		}, nil
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
	if tree.ReportPath == "" {
		t.Fatalf("expected report path in index result")
	}
	report, err := os.ReadFile(tree.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	reportText := string(report)
	for _, want := range []string{
		"# Indexing Report",
		"- mode: sync",
		"- total_discovered_files: 1",
		"- summarized_files: 1",
		"- prompt_tokens: 11",
		"- output_tokens: 7",
		"- total_tokens: 18",
	} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("expected report to contain %q, got:\n%s", want, reportText)
		}
	}
}

func TestRunIndexRequiresLLMWhenSummaryEnabled(t *testing.T) {
	storePath := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := runIndex(storePath, cli.IndexCommand{
		Path:        root,
		Depth:       1,
		MaxFileSize: 1024,
	})
	if err == nil {
		t.Fatalf("expected missing LLM config error")
	}
	if !strings.Contains(err.Error(), "No config file") && !strings.Contains(err.Error(), "Invalid config") && !strings.Contains(err.Error(), "Environment variable") {
		t.Fatalf("expected LLM config or secret error, got %v", err)
	}
}

func TestRunIndexBaseOnlyDoesNotRequireLLM(t *testing.T) {
	storePath := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := runIndex(storePath, cli.IndexCommand{
		Path:              root,
		Depth:             1,
		MaxFileSize:       1024,
		NoMetadataMethods: true,
	})
	if err != nil {
		t.Fatalf("expected base-only index success, got %v", err)
	}
	tree, ok := result.Data.(*index.PathIndexTree)
	if !ok {
		t.Fatalf("expected PathIndexTree, got %T", result.Data)
	}
	if len(tree.Tree.Children) != 1 || tree.Tree.Children[0].Summary != "" {
		t.Fatalf("expected base-only index without summary, got %#v", tree.Tree.Children)
	}
}

func TestRunIndexClassifyAndRoutingDoesNotRequireLLM(t *testing.T) {
	storePath := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := runIndex(storePath, cli.IndexCommand{
		Path:            root,
		Depth:           1,
		MaxFileSize:     1024,
		MetadataMethods: []string{"builtin.classify.v1", "builtin.routing.v1"},
	})
	if err != nil {
		t.Fatalf("expected classify/routing index success, got %v", err)
	}
	tree, ok := result.Data.(*index.PathIndexTree)
	if !ok {
		t.Fatalf("expected PathIndexTree, got %T", result.Data)
	}
	if tree.Run == nil || tree.Run.MethodStatusCounts["builtin.classify.v1:ok"] != 1 || tree.Run.MethodStatusCounts["builtin.routing.v1:ok"] != 1 {
		t.Fatalf("expected method run counts, got %#v", tree.Run)
	}
	report, err := os.ReadFile(tree.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	reportText := string(report)
	for _, want := range []string{
		"## Metadata Methods",
		"- enabled_methods: [base, builtin.classify.v1, builtin.routing.v1]",
		"- builtin.classify.v1:ok: 1",
		"- source: 1",
	} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("expected report to contain %q, got:\n%s", want, reportText)
		}
	}
}

func TestRunIndexMethodsListsDescriptors(t *testing.T) {
	result, err := runIndexMethods(cli.IndexMethodsCommand{Stage: "file"})
	if err != nil {
		t.Fatalf("expected index methods success, got %v", err)
	}
	if !strings.Contains(result.Text, "builtin.classify.v1") || !strings.Contains(result.Text, "builtin.retrieval.v1") || !strings.Contains(result.Text, "description:") {
		t.Fatalf("expected descriptor text, got %q", result.Text)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	methods, ok := data["methods"].([]index.MethodDescriptor)
	if !ok || len(methods) == 0 {
		t.Fatalf("expected method descriptors, got %#v", result.Data)
	}
	for _, method := range methods {
		if method.Stage != index.MethodStageFile {
			t.Fatalf("expected file-only descriptor, got %#v", method)
		}
	}
}

func TestRunIndexPlanValidate(t *testing.T) {
	root := t.TempDir()
	planPath := filepath.Join(root, "aascribe.index.toml")
	if err := os.WriteFile(planPath, []byte(`version = "index-plan-v1"

[methods]
enabled = ["builtin.classify.v1"]
`), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	result, err := runIndexPlanValidate(cli.IndexPlanValidateCommand{Path: planPath})
	if err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
	if !strings.Contains(result.Text, "index plan valid") || !strings.Contains(result.Text, "builtin.classify.v1") {
		t.Fatalf("unexpected validation text: %q", result.Text)
	}
}

func TestRunIndexReviewWorkflowCreatesAgentArtifacts(t *testing.T) {
	workspace := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWD)

	storePath := filepath.Join(workspace, "data", "memory")
	root := filepath.Join(workspace, "coding")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	metadata := `{
  "version": "index-meta-v1",
  "folder_path": "` + root + `",
  "files": [
    {
      "path": "coding/CLAUDE.md",
      "methods": {
        "builtin.retrieval.v1": {
          "fields": {
            "aliases": [{"from": "Example", "to": "HTTP client"}],
            "retrieval_terms": ["` + "bat file.py" + `"]
          }
        }
      }
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(root, ".aascribe_index_meta.json"), []byte(metadata), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	reviewResult, err := runIndexReview(storePath, cli.IndexReviewCommand{Path: root, Method: "builtin.retrieval.v1"})
	if err != nil {
		t.Fatalf("expected review success, got %v", err)
	}
	review, ok := reviewResult.Data.(indexReviewReport)
	if !ok {
		t.Fatalf("expected review report, got %T", reviewResult.Data)
	}
	if review.CheckpointID == "" || len(review.Findings) == 0 || !review.Writes["checkpoint"] || review.Writes["prompt"] {
		t.Fatalf("unexpected review report: %#v", review)
	}
	if _, err := os.Stat(review.ReviewJSON); err != nil {
		t.Fatalf("expected review json artifact: %v", err)
	}

	proposalResult, err := runIndexReviewPropose(storePath, cli.IndexReviewProposeCommand{ReviewJSON: review.ReviewJSON})
	if err != nil {
		t.Fatalf("expected proposal success, got %v", err)
	}
	proposal, ok := proposalResult.Data.(indexReviewProposal)
	if !ok {
		t.Fatalf("expected proposal, got %T", proposalResult.Data)
	}
	if proposal.PromptPath != filepath.Join("prompts", "retrieval", "generate.v1.md") || !strings.Contains(proposal.ProposedContent, "Return strict JSON only") {
		t.Fatalf("unexpected proposal: %#v", proposal)
	}

	patchResult, err := runIndexReviewPatch(storePath, cli.IndexReviewPatchCommand{ProposalJSON: proposal.Artifacts[0]["path"]})
	if err != nil {
		t.Fatalf("expected patch success, got %v", err)
	}
	if !strings.Contains(patchResult.Text, "prompt_patched") {
		t.Fatalf("unexpected patch text: %q", patchResult.Text)
	}
	if _, err := os.Stat(filepath.Join(workspace, "prompts", "retrieval", "generate.v1.md")); err != nil {
		t.Fatalf("expected prompt file: %v", err)
	}

	statusResult, err := runIndexReviewStatus(storePath, review.CheckpointID)
	if err != nil {
		t.Fatalf("expected status success, got %v", err)
	}
	if !strings.Contains(statusResult.Text, "prompt_patched") {
		t.Fatalf("unexpected status text: %q", statusResult.Text)
	}

	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("- **xh** - HTTP client\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	indexResult, err := runIndex(storePath, cli.IndexCommand{
		Path:            root,
		Depth:           1,
		NoSummary:       true,
		MaxFileSize:     1024,
		MetadataMethods: []string{index.MethodBase, index.MethodRetrievalV1},
	})
	if err != nil {
		t.Fatalf("expected index success with patched prompt, got %v", err)
	}
	tree, ok := indexResult.Data.(*index.PathIndexTree)
	if !ok || tree.ReportPath == "" {
		t.Fatalf("expected index tree with report path, got %#v", indexResult.Data)
	}
	updatedMeta, err := os.ReadFile(filepath.Join(root, ".aascribe_index_meta.json"))
	if err != nil {
		t.Fatalf("read updated metadata: %v", err)
	}
	if !strings.Contains(string(updatedMeta), "source_quote") || !strings.Contains(string(updatedMeta), "prompts/retrieval/generate.v1.md") {
		t.Fatalf("expected patched prompt to affect retrieval metadata, got %s", string(updatedMeta))
	}

	if _, err := runIndexReviewRestore(storePath, review.CheckpointID); err != nil {
		t.Fatalf("expected restore success, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "prompts", "retrieval", "generate.v1.md")); !os.IsNotExist(err) {
		t.Fatalf("expected restore to remove prompt created after checkpoint, stat err=%v", err)
	}
}

func TestRunIndexUsesIndexPlan(t *testing.T) {
	storePath := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	planPath := filepath.Join(root, "aascribe.index.toml")
	if err := os.WriteFile(planPath, []byte(`version = "index-plan-v1"

[methods]
enabled = ["builtin.classify.v1", "builtin.routing.v1"]
`), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	result, err := runIndex(storePath, cli.IndexCommand{
		Path:        root,
		Depth:       1,
		MaxFileSize: 1024,
		IndexPlan:   planPath,
	})
	if err != nil {
		t.Fatalf("expected index plan run success, got %v", err)
	}
	tree, ok := result.Data.(*index.PathIndexTree)
	if !ok {
		t.Fatalf("expected PathIndexTree, got %T", result.Data)
	}
	if tree.Run == nil || tree.Run.MethodStatusCounts["builtin.classify.v1:ok"] == 0 || tree.Run.MethodStatusCounts["builtin.routing.v1:ok"] == 0 {
		t.Fatalf("expected plan-selected method counts, got %#v", tree.Run)
	}
	report, err := os.ReadFile(tree.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	reportText := string(report)
	for _, want := range []string{
		"- index_plan: " + planPath,
		"- index_plan_digest: sha256:",
		"- metadata_methods: [base, builtin.classify.v1, builtin.routing.v1]",
	} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("expected report to contain %q, got:\n%s", want, reportText)
		}
	}
}

func TestRunIndexAsyncSnapshotsIndexPlan(t *testing.T) {
	storePath := t.TempDir()
	root := t.TempDir()
	planPath := filepath.Join(root, "aascribe.index.toml")
	if err := os.WriteFile(planPath, []byte(`version = "index-plan-v1"

[methods]
enabled = ["builtin.classify.v1"]
`), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	var startedOperation string
	var startedCommand cli.IndexCommand
	original := startAsyncIndexProcess
	startAsyncIndexProcess = func(storePath, operationID string, cmd cli.IndexCommand) error {
		startedOperation = operationID
		startedCommand = cmd
		return nil
	}
	defer func() {
		startAsyncIndexProcess = original
	}()

	_, err := runIndex(storePath, cli.IndexCommand{
		Path:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Async:       true,
		IndexPlan:   planPath,
	})
	if err != nil {
		t.Fatalf("expected async index accepted, got %v", err)
	}
	if startedOperation == "" {
		t.Fatalf("expected worker start")
	}
	if filepath.Base(startedCommand.IndexPlan) != "index-plan.toml" || startedCommand.IndexPlan == planPath {
		t.Fatalf("expected snapshotted plan path, got %#v", startedCommand)
	}
	if _, err := os.Stat(startedCommand.IndexPlan); err != nil {
		t.Fatalf("expected snapshotted plan file: %v", err)
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

func TestIndexArgsPreservesMetadataMethodFlags(t *testing.T) {
	args := indexArgs(cli.IndexCommand{
		Path:            "./tests",
		Depth:           2,
		Concurrency:     4,
		MaxFileSize:     1024,
		MetadataMethods: []string{"summary"},
	})
	rendered := strings.Join(args, " ")
	if !strings.Contains(rendered, "--metadata-method summary") {
		t.Fatalf("expected metadata method flag in args, got %#v", args)
	}
	if args[len(args)-1] != "./tests" {
		t.Fatalf("expected path to remain last, got %#v", args)
	}
}

func TestIndexArgsPreservesNoMetadataMethodsFlag(t *testing.T) {
	args := indexArgs(cli.IndexCommand{
		Path:              "./tests",
		Depth:             2,
		Concurrency:       4,
		MaxFileSize:       1024,
		NoMetadataMethods: true,
	})
	rendered := strings.Join(args, " ")
	if !strings.Contains(rendered, "--no-metadata-methods") {
		t.Fatalf("expected no metadata methods flag in args, got %#v", args)
	}
	if args[len(args)-1] != "./tests" {
		t.Fatalf("expected path to remain last, got %#v", args)
	}
}

func TestIndexArgsPreservesIndexPlanWithoutManualMethods(t *testing.T) {
	args := indexArgs(cli.IndexCommand{
		Path:            "./tests",
		Depth:           2,
		Concurrency:     4,
		MaxFileSize:     1024,
		IndexPlan:       "./aascribe.index.toml",
		MetadataMethods: []string{"builtin.classify.v1"},
	})
	rendered := strings.Join(args, " ")
	if !strings.Contains(rendered, "--index-plan ./aascribe.index.toml") {
		t.Fatalf("expected index plan flag in args, got %#v", args)
	}
	if strings.Contains(rendered, "--metadata-method") {
		t.Fatalf("did not expect metadata method flags when index plan is present, got %#v", args)
	}
	if args[len(args)-1] != "./tests" {
		t.Fatalf("expected path to remain last, got %#v", args)
	}
}

func TestCreateIndexReportFileAvoidsSameSecondCollision(t *testing.T) {
	reportDir := t.TempDir()
	finishedAt := time.Date(2026, 4, 29, 3, 3, 31, 0, time.UTC)

	first, firstFile, err := createIndexReportFile(reportDir, finishedAt)
	if err != nil {
		t.Fatalf("create first report: %v", err)
	}
	defer firstFile.Close()
	if filepath.Base(first) != "2026-04-29-030331-indexing-report.md" {
		t.Fatalf("unexpected first report path: %s", first)
	}

	second, secondFile, err := createIndexReportFile(reportDir, finishedAt)
	if err != nil {
		t.Fatalf("create second report: %v", err)
	}
	defer secondFile.Close()
	if filepath.Base(second) != "2026-04-29-030331-2-indexing-report.md" {
		t.Fatalf("unexpected second report path: %s", second)
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
	if !strings.Contains(string(encoded), "report_path") {
		t.Fatalf("expected report_path in result data, got %s", string(encoded))
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

func TestRunOperationCleanReturnsCleanupSummary(t *testing.T) {
	storePath := t.TempDir()
	done, err := operation.Create(storePath, operation.CreateInput{Command: "index"})
	if err != nil {
		t.Fatalf("create done operation: %v", err)
	}
	status, err := operation.LoadStatus(storePath, done.OperationID)
	if err != nil {
		t.Fatalf("load done operation: %v", err)
	}
	status.State = operation.StateSucceeded
	status.Stage = "complete"
	status.Message = "Operation completed."
	status.ResultReady = true
	status.CompletedAt = status.UpdatedAt
	if err := operation.SaveStatus(storePath, status); err != nil {
		t.Fatalf("save done operation: %v", err)
	}
	running, err := operation.Create(storePath, operation.CreateInput{Command: "index", State: operation.StateRunning})
	if err != nil {
		t.Fatalf("create running operation: %v", err)
	}

	result, err := runOperationClean(storePath, cli.OperationCleanCommand{DryRun: true})
	if err != nil {
		t.Fatalf("operation clean: %v", err)
	}
	clean, ok := result.Data.(*operation.CleanResult)
	if !ok || !clean.DryRun || clean.RemovedCount != 1 || clean.SkippedCount != 1 {
		t.Fatalf("unexpected clean payload: %#v", result.Data)
	}
	if !strings.Contains(result.Text, "Would remove 1 terminal operation(s)") {
		t.Fatalf("expected dry-run text, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "skipped 1 active operation(s)") {
		t.Fatalf("expected skipped text, got %q", result.Text)
	}
	if _, err := operation.LoadStatus(storePath, done.OperationID); err != nil {
		t.Fatalf("dry-run should preserve done operation: %v", err)
	}
	if _, err := operation.LoadStatus(storePath, running.OperationID); err != nil {
		t.Fatalf("dry-run should preserve running operation: %v", err)
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
	if err := os.MkdirAll(config.ConfigDir(storePath), 0o755); err != nil {
		t.Fatalf("mkdir config dir failed: %v", err)
	}
	if err := os.WriteFile(config.ConfigPath(storePath), []byte(body), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
}
