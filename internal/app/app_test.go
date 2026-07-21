package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/config"
	"github.com/austinjan/aascribe/internal/operation"
	"github.com/austinjan/aascribe/internal/store"
	"github.com/austinjan/agentcli-core/largeoutput"
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

func TestParseGlobalStoreAfterCommand(t *testing.T) {
	parsed, err := cli.Parse([]string{"init", "--force", "--store", "/tmp/demo"})
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

func TestParseConfigGeminiAPIKey(t *testing.T) {
	parsed, err := cli.Parse([]string{"config", "gemini_api_key=secret"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.ConfigSetCommand)
	if !ok {
		t.Fatalf("expected config command, got %#v", parsed.Command)
	}
	if cmd.Key != "gemini_api_key" || cmd.Value != "secret" {
		t.Fatalf("unexpected config command: %#v", cmd)
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

func TestParseIndexFlagsAfterPath(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "./tests", "--depth", "2", "--include", "*.go", "--include", "*.md", "--refresh"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexCommand)
	if !ok {
		t.Fatalf("expected index command, got %#v", parsed.Command)
	}
	if cmd.Path != "./tests" || cmd.Depth != 2 || !cmd.Refresh {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
	if len(cmd.Include) != 2 || cmd.Include[0] != "*.go" || cmd.Include[1] != "*.md" {
		t.Fatalf("unexpected include flags: %#v", cmd.Include)
	}
}

func TestParseIndexMetadataMethods(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "./tests", "--metadata-method", "summary", "--metadata-method", "base", "--metadata-method", "builtin.classify.v1"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexCommand)
	if !ok {
		t.Fatalf("expected index command, got %#v", parsed.Command)
	}
	if len(cmd.MetadataMethods) != 3 || cmd.MetadataMethods[0] != "summary" || cmd.MetadataMethods[1] != "base" || cmd.MetadataMethods[2] != "builtin.classify.v1" {
		t.Fatalf("unexpected metadata methods: %#v", cmd.MetadataMethods)
	}
	effective := cmd.EffectiveMetadataMethods()
	if len(effective) != 3 || effective[0] != "base" || effective[1] != "summary" || effective[2] != "builtin.classify.v1" {
		t.Fatalf("unexpected effective metadata methods: %#v", effective)
	}
}

func TestParseIndexMethods(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "methods", "--stage", "file"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexMethodsCommand)
	if !ok {
		t.Fatalf("expected index methods command, got %#v", parsed.Command)
	}
	if cmd.Stage != "file" {
		t.Fatalf("unexpected stage: %#v", cmd)
	}
}

func TestParseIndexPlanValidate(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "plan", "validate", "./aascribe.index.toml"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexPlanValidateCommand)
	if !ok {
		t.Fatalf("expected index plan validate command, got %#v", parsed.Command)
	}
	if cmd.Path != "./aascribe.index.toml" {
		t.Fatalf("unexpected plan path: %#v", cmd)
	}
}

func TestParseIndexReviewWorkflow(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "review", "./coding", "--method", "builtin.retrieval.v1"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	review, ok := parsed.Command.(cli.IndexReviewCommand)
	if !ok || review.Path != "./coding" || review.Method != "builtin.retrieval.v1" {
		t.Fatalf("unexpected review command: %#v", parsed.Command)
	}

	parsed, err = cli.Parse([]string{"index", "review", "propose", "./data/report/review.json"})
	if err != nil {
		t.Fatalf("expected propose parse success, got %v", err)
	}
	if _, ok := parsed.Command.(cli.IndexReviewProposeCommand); !ok {
		t.Fatalf("expected propose command, got %#v", parsed.Command)
	}

	parsed, err = cli.Parse([]string{"index", "review", "restore", "index-review-abc"})
	if err != nil {
		t.Fatalf("expected restore parse success, got %v", err)
	}
	if _, ok := parsed.Command.(cli.IndexReviewRestoreCommand); !ok {
		t.Fatalf("expected restore command, got %#v", parsed.Command)
	}
}

func TestParseIndexPlanRejectsManualMethodSelection(t *testing.T) {
	_, err := cli.Parse([]string{"index", ".", "--index-plan", "./plan.toml", "--metadata-method", "builtin.classify.v1"})
	if err == nil {
		t.Fatalf("expected --index-plan and --metadata-method conflict")
	}
	_, err = cli.Parse([]string{"index", ".", "--index-plan", "./plan.toml", "--no-metadata-methods"})
	if err == nil {
		t.Fatalf("expected --index-plan and --no-metadata-methods conflict")
	}
}

func TestParseIndexNoMetadataMethodsKeepsBaseOnly(t *testing.T) {
	parsed, err := cli.Parse([]string{"index", "./tests", "--no-metadata-methods"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.IndexCommand)
	if !ok {
		t.Fatalf("expected index command, got %#v", parsed.Command)
	}
	effective := cmd.EffectiveMetadataMethods()
	if len(effective) != 1 || effective[0] != "base" {
		t.Fatalf("unexpected effective metadata methods: %#v", effective)
	}
	if !cmd.EffectiveNoSummary() {
		t.Fatalf("expected no summary when optional metadata methods are disabled")
	}
}

func TestParseIndexRejectsUnsupportedMetadataMethod(t *testing.T) {
	_, err := cli.Parse([]string{"index", "./tests", "--metadata-method", "classify"})
	if err == nil {
		t.Fatalf("expected unsupported metadata method error")
	}
	if !strings.Contains(err.Error(), "Unsupported metadata method") {
		t.Fatalf("expected unsupported metadata method message, got %v", err)
	}
}

func TestParseSearch(t *testing.T) {
	parsed, err := cli.Parse([]string{"search", "zprofile", "./docs", "--engine", "builtin", "--ignore-case", "--fixed-strings", "--glob", "*.md", "--max-count", "5"})
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	cmd, ok := parsed.Command.(cli.SearchCommand)
	if !ok {
		t.Fatalf("expected search command, got %#v", parsed.Command)
	}
	if cmd.Query != "zprofile" || cmd.Path != "./docs" || cmd.Engine != "builtin" {
		t.Fatalf("unexpected parsed command: %#v", cmd)
	}
	if !cmd.IgnoreCase || !cmd.FixedStrings || cmd.MaxCount != 5 {
		t.Fatalf("unexpected search flags: %#v", cmd)
	}
	if len(cmd.Glob) != 1 || cmd.Glob[0] != "*.md" {
		t.Fatalf("unexpected glob flags: %#v", cmd.Glob)
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

func TestParseMapTagModes(t *testing.T) {
	parsed, err := cli.Parse([]string{"map", "./tests", "-tag"})
	if err != nil {
		t.Fatalf("expected bare --tag parse success, got %v", err)
	}
	catalog := parsed.Command.(cli.MapCommand)
	if !catalog.ListTags || len(catalog.Tags) != 0 {
		t.Fatalf("expected tag catalog mode, got %#v", catalog)
	}

	parsed, err = cli.Parse([]string{"map", "./tests", "--tag", "zed", "--tag=docs"})
	if err != nil {
		t.Fatalf("expected tag query parse success, got %v", err)
	}
	query := parsed.Command.(cli.MapCommand)
	if query.ListTags || len(query.Tags) != 2 || query.Tags[0] != "zed" || query.Tags[1] != "docs" {
		t.Fatalf("expected repeated tag query, got %#v", query)
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
	if !strings.Contains(rendered, "aascribe map --help") || !strings.Contains(rendered, "aascribe index --help") {
		t.Fatalf("expected root help to route agents to command-specific help, got %q", rendered)
	}
	if strings.Contains(rendered, "Current Implementation Status:") || strings.Contains(rendered, "index review propose") {
		t.Fatalf("expected root help to stay progressively disclosed, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunMapHelpRoutesSearchWorkflow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"map", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "Use search after map") || !strings.Contains(rendered, "aascribe index map <path>") || !strings.Contains(rendered, "--max-files-per-dir") || !strings.Contains(rendered, "--tag <tag>") || !strings.Contains(rendered, "--recent-modified") {
		t.Fatalf("expected map help to route map/search workflow, got %q", rendered)
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
	if !strings.Contains(rendered, "aascribe index review --help") {
		t.Fatalf("expected index help to route agents to review help, got %q", rendered)
	}
	if strings.Contains(rendered, "aascribe index review propose") || strings.Contains(rendered, "aascribe index review patch") {
		t.Fatalf("expected index help to avoid detailed review workflow, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunIndexPlanHelpGuidesAgents(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"index", "plan", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	for _, want := range []string{
		"Agent Workflow",
		"aascribe --format json index methods",
		"Plan Verification Workflow For AI Agents",
		"Do not run aascribe index <path> --index-plan <file> until validation succeeds.",
		"version = \"index-plan-v1\"",
		"Do not write boolean keys",
		"builtin.classify.v1",
		"builtin.routing.v1",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected index plan help to contain %q, got %q", want, rendered)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunIndexPlanValidateHelpGuidesPlanRevision(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"index", "plan", "validate", "--help"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d", status)
	}
	rendered := stdout.String()
	for _, want := range []string{
		"Validation is mandatory before running an index plan.",
		"If validation fails, revise the plan and validate again.",
		"aascribe --format json index methods",
		"Do not run:",
		"until validation succeeds.",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected index plan validate help to contain %q, got %q", want, rendered)
		}
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

func TestRunSearchReturnsLineMatches(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("alpha\nzprofile setup\nZPROFILE again\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("zprofile outside glob\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "zprofile", root, "--engine", "builtin", "--ignore-case", "--fixed-strings", "--glob", "*.md"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Query      string `json:"query"`
			Engine     string `json:"engine"`
			MatchCount int    `json:"match_count"`
			Matches    []struct {
				Path   string `json:"path"`
				Line   int    `json:"line"`
				Column int    `json:"column"`
				Text   string `json:"text"`
			} `json:"matches"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data.Query != "zprofile" || payload.Data.Engine != "builtin" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload.Data.MatchCount != 2 || len(payload.Data.Matches) != 2 {
		t.Fatalf("expected two matches, got %#v", payload.Data)
	}
	if payload.Data.Matches[0].Path != "notes.md" || payload.Data.Matches[0].Line != 2 || payload.Data.Matches[0].Column != 1 {
		t.Fatalf("unexpected first match: %#v", payload.Data.Matches[0])
	}
}

func TestRunSearchRejectsEmptyQuery(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "", root, "--engine", "builtin"}, &stdout, &stderr)
	if status != 2 {
		t.Fatalf("expected invalid-arguments status 2, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if payload.OK || payload.Error.Code != "INVALID_ARGUMENTS" {
		t.Fatalf("expected INVALID_ARGUMENTS response, got %s", stdout.String())
	}
}

func TestRunSearchHonorsExplicitZeroMaxCount(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("needle one\nneedle two\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "needle", root, "--engine", "builtin", "--fixed-strings", "--max-count", "0"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			MatchCount int   `json:"match_count"`
			Truncated  bool  `json:"truncated"`
			Matches    []any `json:"matches"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if payload.Data.MatchCount != 0 || len(payload.Data.Matches) != 0 || !payload.Data.Truncated {
		t.Fatalf("explicit zero must return zero matching lines and report omitted matches: %s", stdout.String())
	}
}

func TestRunSearchReportsExactLimitAndTruncation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("needle one\nneedle two\nneedle three\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var jsonOut bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "needle", root, "--engine", "builtin", "--fixed-strings", "--max-count", "2"}, &jsonOut, &stderr)
	if status != 0 {
		t.Fatalf("expected JSON success, got %d stderr=%q stdout=%q", status, stderr.String(), jsonOut.String())
	}
	var payload struct {
		Data struct {
			MatchCount         int   `json:"match_count"`
			ReturnedMatchLines int   `json:"returned_match_lines"`
			Truncated          bool  `json:"truncated"`
			Matches            []any `json:"matches"`
		} `json:"data"`
	}
	if err := json.Unmarshal(jsonOut.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, jsonOut.String())
	}
	if payload.Data.MatchCount != 2 || payload.Data.ReturnedMatchLines != 2 || len(payload.Data.Matches) != 2 || !payload.Data.Truncated {
		t.Fatalf("expected exactly two returned matching lines with truncation, got %s", jsonOut.String())
	}

	var textOut bytes.Buffer
	stderr.Reset()
	status = Run([]string{"search", "needle", root, "--engine", "builtin", "--fixed-strings", "--max-count", "2"}, &textOut, &stderr)
	if status != 0 {
		t.Fatalf("expected text success, got %d stderr=%q stdout=%q", status, stderr.String(), textOut.String())
	}
	if !strings.Contains(textOut.String(), "returned_match_lines: 2") || !strings.Contains(textOut.String(), "truncated: true") {
		t.Fatalf("text output must expose the same limit decision, got %q", textOut.String())
	}
}

func TestRunSearchDeclaresMatchingLineUnitForRepeatedOccurrences(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("needle then needle again\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "needle", root, "--engine", "builtin", "--fixed-strings"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			MatchUnit          string `json:"match_unit"`
			ReturnedMatchLines int    `json:"returned_match_lines"`
			Matches            []any  `json:"matches"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if payload.Data.MatchUnit != "line" || payload.Data.ReturnedMatchLines != 1 || len(payload.Data.Matches) != 1 {
		t.Fatalf("repeated occurrences on one line must be one declared line result: %s", stdout.String())
	}
}

func TestRunSearchFileScopeDoesNotSearchSiblingFiles(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "selected.txt")
	if err := os.WriteFile(selected, []byte("nothing here\n"), 0o644); err != nil {
		t.Fatalf("write selected file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "sibling.txt"), []byte("needle only in sibling\n"), 0o644); err != nil {
		t.Fatalf("write sibling file: %v", err)
	}
	if gitPath, err := exec.LookPath("git"); err == nil {
		for _, args := range [][]string{{"init", "-q", root}, {"-C", root, "add", "selected.txt", "sibling.txt"}} {
			if output, err := exec.Command(gitPath, args...).CombinedOutput(); err != nil {
				t.Fatalf("prepare git fixture: %v: %s", err, output)
			}
		}
	}
	engines := []string{"builtin"}
	for _, candidate := range []string{"rg", "grep", "git"} {
		if _, err := exec.LookPath(candidate); err == nil {
			engine := candidate
			if candidate == "git" {
				engine = "git-grep"
			}
			engines = append(engines, engine)
		}
	}
	for _, engine := range engines {
		t.Run(engine, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := Run([]string{"--format", "json", "search", "needle", selected, "--engine", engine, "--fixed-strings"}, &stdout, &stderr)
			if status != 0 {
				t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
			}
			var payload struct {
				Data struct {
					Root               string `json:"root"`
					ReturnedMatchLines int    `json:"returned_match_lines"`
				} `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
			}
			if payload.Data.Root != filepath.ToSlash(selected) || payload.Data.ReturnedMatchLines != 0 {
				t.Fatalf("file scope must remain exact and exclude siblings: %s", stdout.String())
			}
		})
	}
}

func TestRunSearchFixedStringMetacharactersAreLiteralAcrossEngines(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("literal a.b[0] here\nregex decoy axb0 here\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if gitPath, err := exec.LookPath("git"); err == nil {
		for _, args := range [][]string{{"init", "-q", root}, {"-C", root, "add", "notes.txt"}} {
			if output, err := exec.Command(gitPath, args...).CombinedOutput(); err != nil {
				t.Fatalf("prepare git fixture: %v: %s", err, output)
			}
		}
	}

	engines := []string{"builtin"}
	for _, candidate := range []string{"rg", "grep", "git"} {
		if _, err := exec.LookPath(candidate); err == nil {
			engine := candidate
			if candidate == "git" {
				engine = "git-grep"
			}
			engines = append(engines, engine)
		}
	}

	for _, engine := range engines {
		t.Run(engine, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := Run([]string{"--format", "json", "search", "a.b[0]", root, "--engine", engine, "--fixed-strings"}, &stdout, &stderr)
			if status != 0 {
				t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
			}
			var payload struct {
				Data struct {
					QueryMode          string `json:"query_mode"`
					ReturnedMatchLines int    `json:"returned_match_lines"`
					Matches            []struct {
						Line int `json:"line"`
					} `json:"matches"`
				} `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
			}
			if payload.Data.QueryMode != "fixed" || payload.Data.ReturnedMatchLines != 1 || len(payload.Data.Matches) != 1 || payload.Data.Matches[0].Line != 1 {
				t.Fatalf("fixed-string semantics differ for %s: %s", engine, stdout.String())
			}
		})
	}
}

func TestRunSearchFixedStringCoverageIncludesTrackedAndUntrackedAcrossEngines(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required for the git-grep coverage fixture")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("shared needle tracked\n"), 0o644); err != nil {
		t.Fatalf("write tracked fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("shared needle untracked\n"), 0o644); err != nil {
		t.Fatalf("write untracked fixture: %v", err)
	}
	for _, args := range [][]string{{"init", "-q", root}, {"-C", root, "add", "tracked.txt"}} {
		if output, err := exec.Command(gitPath, args...).CombinedOutput(); err != nil {
			t.Fatalf("prepare git fixture: %v: %s", err, output)
		}
	}

	engines := []string{"builtin", "git-grep"}
	for _, candidate := range []string{"rg", "grep"} {
		if _, err := exec.LookPath(candidate); err == nil {
			engines = append(engines, candidate)
		}
	}
	for _, engine := range engines {
		t.Run(engine, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := Run([]string{"--format", "json", "search", "shared needle", root, "--engine", engine, "--fixed-strings"}, &stdout, &stderr)
			if status != 0 {
				t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
			}
			var payload struct {
				Data struct {
					ReturnedMatchLines int `json:"returned_match_lines"`
				} `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
			}
			if payload.Data.ReturnedMatchLines != 2 {
				t.Fatalf("%s must include both tracked and untracked files: %s", engine, stdout.String())
			}
		})
	}
}

func TestRunSearchEnforcesAndDescribesOneDefaultScopeAcrossEngines(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required for the scope fixture")
	}
	root := t.TempDir()
	files := map[string]string{
		"tracked.txt":   "scope needle tracked\n",
		"untracked.txt": "scope needle untracked\n",
		".hidden.txt":   "scope needle hidden\n",
		"ignored.txt":   "scope needle gitignored\n",
		"debug.log":     "scope needle aaignored\n",
		".gitignore":    "ignored.txt\n",
		".aaignore":     "*.log\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	for _, args := range [][]string{{"init", "-q", root}, {"-C", root, "add", "tracked.txt", ".gitignore", ".aaignore"}} {
		if output, err := exec.Command(gitPath, args...).CombinedOutput(); err != nil {
			t.Fatalf("prepare git fixture: %v: %s", err, output)
		}
	}

	engines := []string{"builtin", "git-grep"}
	for _, candidate := range []string{"rg", "grep"} {
		if _, err := exec.LookPath(candidate); err == nil {
			engines = append(engines, candidate)
		}
	}
	for _, engine := range engines {
		t.Run(engine, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := Run([]string{"--format", "json", "search", "scope needle", root, "--engine", engine, "--fixed-strings"}, &stdout, &stderr)
			if status != 0 {
				t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
			}
			var payload struct {
				Data struct {
					ReturnedMatchLines int `json:"returned_match_lines"`
					Scope              struct {
						Root          string   `json:"root"`
						TrackedOnly   bool     `json:"tracked_only"`
						Hidden        bool     `json:"hidden"`
						Ignored       bool     `json:"ignored"`
						IgnoreSources []string `json:"ignore_sources"`
					} `json:"scope"`
				} `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
			}
			if payload.Data.ReturnedMatchLines != 2 {
				t.Fatalf("%s must return only visible, non-ignored tracked and untracked files: %s", engine, stdout.String())
			}
			if payload.Data.Scope.Root != filepath.ToSlash(root) || payload.Data.Scope.TrackedOnly || payload.Data.Scope.Hidden || payload.Data.Scope.Ignored || !reflect.DeepEqual(payload.Data.Scope.IgnoreSources, []string{".gitignore", ".aaignore"}) {
				t.Fatalf("%s returned incorrect scope metadata: %s", engine, stdout.String())
			}
		})
	}
}

func TestRunSearchUnreadableFileMakesResultIncomplete(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "readable.txt"), []byte("needle readable\n"), 0o644); err != nil {
		t.Fatalf("write readable fixture: %v", err)
	}
	unreadable := filepath.Join(root, "unreadable.txt")
	if err := os.WriteFile(unreadable, []byte("needle unreadable\n"), 0o600); err != nil {
		t.Fatalf("write unreadable fixture: %v", err)
	}
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatalf("chmod unreadable fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "needle", root, "--engine", "builtin", "--fixed-strings"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("partial search should preserve useful results, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Complete           bool `json:"complete"`
			ReturnedMatchLines int  `json:"returned_match_lines"`
			PartialReasons     []struct {
				Kind string `json:"kind"`
				Path string `json:"path"`
			} `json:"partial_reasons"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if payload.Data.Complete || payload.Data.ReturnedMatchLines != 1 || len(payload.Data.PartialReasons) != 1 || payload.Data.PartialReasons[0].Kind != "unreadable" || payload.Data.PartialReasons[0].Path != "unreadable.txt" {
		t.Fatalf("expected one healthy match and one unreadable partial reason: %s", stdout.String())
	}
}

func TestRunSearchFindsMatchOnLongSupportedLine(t *testing.T) {
	root := t.TempDir()
	line := strings.Repeat("x", 128*1024) + " needle\n"
	if err := os.WriteFile(filepath.Join(root, "long.txt"), []byte(line), 0o644); err != nil {
		t.Fatalf("write long-line fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "needle", root, "--engine", "builtin", "--fixed-strings"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Complete           bool `json:"complete"`
			ReturnedMatchLines int  `json:"returned_match_lines"`
			Matches            []struct {
				Line int `json:"line"`
			} `json:"matches"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.Data.Complete || payload.Data.ReturnedMatchLines != 1 || len(payload.Data.Matches) != 1 || payload.Data.Matches[0].Line != 1 {
		t.Fatalf("supported long line must not be silently missed: %s", stdout.String())
	}
}

func TestRunSearchReportsBinaryExclusionWithoutIOFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "readable.txt"), []byte("needle text\n"), 0o644); err != nil {
		t.Fatalf("write text fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "binary.dat"), []byte("needle\x00binary\n"), 0o644); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "needle", root, "--engine", "builtin", "--fixed-strings"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Complete           bool  `json:"complete"`
			ReturnedMatchLines int   `json:"returned_match_lines"`
			PartialReasons     []any `json:"partial_reasons"`
			Exclusions         []struct {
				Kind string `json:"kind"`
				Path string `json:"path"`
			} `json:"exclusions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.Data.Complete || payload.Data.ReturnedMatchLines != 1 || len(payload.Data.PartialReasons) != 0 || len(payload.Data.Exclusions) != 1 || payload.Data.Exclusions[0].Kind != "binary" || payload.Data.Exclusions[0].Path != "binary.dat" {
		t.Fatalf("binary exclusion must be explicit and distinct from I/O failure: %s", stdout.String())
	}
}

func TestRunSearchPreservesColonInFilenameAcrossEngines(t *testing.T) {
	root := t.TempDir()
	filename := "colon:name.txt"
	if err := os.WriteFile(filepath.Join(root, filename), []byte("needle here\n"), 0o644); err != nil {
		t.Fatalf("write colon filename fixture: %v", err)
	}
	gitPath, gitErr := exec.LookPath("git")
	if gitErr == nil {
		for _, args := range [][]string{{"init", "-q", root}, {"-C", root, "add", filename}} {
			if output, err := exec.Command(gitPath, args...).CombinedOutput(); err != nil {
				t.Fatalf("prepare git fixture: %v: %s", err, output)
			}
		}
	}

	engines := []string{"builtin"}
	for _, candidate := range []string{"rg", "grep", "git"} {
		if _, err := exec.LookPath(candidate); err == nil {
			engine := candidate
			if candidate == "git" {
				engine = "git-grep"
			}
			engines = append(engines, engine)
		}
	}
	for _, engine := range engines {
		t.Run(engine, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := Run([]string{"--format", "json", "search", "needle", root, "--engine", engine, "--fixed-strings"}, &stdout, &stderr)
			if status != 0 {
				t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
			}
			var payload struct {
				Data struct {
					Matches []struct {
						Path   string `json:"path"`
						Line   int    `json:"line"`
						Column int    `json:"column"`
					} `json:"matches"`
				} `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
			}
			if len(payload.Data.Matches) != 1 || payload.Data.Matches[0].Path != filename || payload.Data.Matches[0].Line != 1 || payload.Data.Matches[0].Column != 1 {
				t.Fatalf("%s corrupted colon filename location: %s", engine, stdout.String())
			}
		})
	}
}

func TestRunSearchDeclaresByteColumnsAcrossEngines(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "unicode.txt"), []byte("台灣 needle\n"), 0o644); err != nil {
		t.Fatalf("write unicode fixture: %v", err)
	}
	gitPath, gitErr := exec.LookPath("git")
	if gitErr == nil {
		for _, args := range [][]string{{"init", "-q", root}, {"-C", root, "add", "unicode.txt"}} {
			if output, err := exec.Command(gitPath, args...).CombinedOutput(); err != nil {
				t.Fatalf("prepare git fixture: %v: %s", err, output)
			}
		}
	}

	engines := []string{"builtin"}
	for _, candidate := range []string{"rg", "grep", "git"} {
		if _, err := exec.LookPath(candidate); err == nil {
			engine := candidate
			if candidate == "git" {
				engine = "git-grep"
			}
			engines = append(engines, engine)
		}
	}
	for _, engine := range engines {
		t.Run(engine, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := Run([]string{"--format", "json", "search", "needle", root, "--engine", engine, "--fixed-strings"}, &stdout, &stderr)
			if status != 0 {
				t.Fatalf("expected success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
			}
			var payload struct {
				Data struct {
					ColumnUnit string `json:"column_unit"`
					Matches    []struct {
						Column int `json:"column"`
					} `json:"matches"`
				} `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
			}
			if payload.Data.ColumnUnit != "byte" || len(payload.Data.Matches) != 1 || payload.Data.Matches[0].Column != 8 {
				t.Fatalf("%s must report one-based byte columns: %s", engine, stdout.String())
			}
		})
	}
}

func TestRunSearchRegexFallbackNeverChangesAlternationMeaning(t *testing.T) {
	grepPath, err := exec.LookPath("grep")
	if err != nil {
		t.Skip("grep is required to reproduce the incompatible fallback")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "values.txt"), []byte("foo\nbar\n"), 0o644); err != nil {
		t.Fatalf("write regex fixture: %v", err)
	}
	binDir := t.TempDir()
	if err := os.Symlink(grepPath, filepath.Join(binDir, "grep")); err != nil {
		t.Fatalf("link grep boundary double: %v", err)
	}
	t.Setenv("PATH", binDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "foo|bar", root, "--engine", "auto"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected compatible fallback success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Engine             string `json:"engine"`
			QueryMode          string `json:"query_mode"`
			ReturnedMatchLines int    `json:"returned_match_lines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if payload.Data.Engine != "builtin" || payload.Data.QueryMode != "regex" || payload.Data.ReturnedMatchLines != 2 {
		t.Fatalf("auto fallback changed regex meaning: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	status = Run([]string{"--format", "json", "search", "foo|bar", root, "--engine", "grep"}, &stdout, &stderr)
	if status != 2 || !strings.Contains(stdout.String(), "UNSUPPORTED_QUERY") {
		t.Fatalf("explicit incompatible regex engine must be rejected, got status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
}

func TestRunSearchStopsExternalOutputAfterLimitPlusOne(t *testing.T) {
	root := t.TempDir()
	fixture := filepath.Join(root, "values.txt")
	if err := os.WriteFile(fixture, []byte("needle\n"), 0o644); err != nil {
		t.Fatalf("write search fixture: %v", err)
	}
	marker := filepath.Join(root, "producer-finished")
	binDir := t.TempDir()
	fakeRG := filepath.Join(binDir, "rg")
	script := fmt.Sprintf(`#!/bin/sh
last=""
for arg do last="$arg"; done
i=1
while [ "$i" -le 5000 ]; do
  printf '{"type":"match","data":{"path":{"text":"%%s"},"lines":{"text":"needle\\n"},"line_number":%%s,"submatches":[{"start":0}]}}\n' "$last" "$i"
  i=$((i + 1))
done
/usr/bin/touch %q
`, marker)
	if err := os.WriteFile(fakeRG, []byte(script), 0o755); err != nil {
		t.Fatalf("write rg boundary double: %v", err)
	}
	t.Setenv("PATH", binDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "needle", root, "--engine", "rg", "--fixed-strings", "--max-count", "1"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected bounded success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			ReturnedMatchLines int  `json:"returned_match_lines"`
			Truncated          bool `json:"truncated"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if payload.Data.ReturnedMatchLines != 1 || !payload.Data.Truncated {
		t.Fatalf("expected one returned line and limit-plus-one truncation: %s", stdout.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("external producer ran to completion instead of being stopped after limit-plus-one; marker err=%v", err)
	}
}

func TestRunSearchClassifiesExternalExecutionFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "values.txt"), []byte("needle\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	binDir := t.TempDir()
	fakeRG := filepath.Join(binDir, "rg")
	if err := os.WriteFile(fakeRG, []byte("#!/bin/sh\necho simulated rg failure >&2\nexit 2\n"), 0o755); err != nil {
		t.Fatalf("write rg boundary double: %v", err)
	}
	t.Setenv("PATH", binDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", "needle", root, "--engine", "auto", "--fixed-strings"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("auto should safely fall back, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Engine             string `json:"engine"`
			ReturnedMatchLines int    `json:"returned_match_lines"`
			EngineFailures     []struct {
				Engine string `json:"engine"`
				Kind   string `json:"kind"`
			} `json:"engine_failures"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if payload.Data.Engine != "builtin" || payload.Data.ReturnedMatchLines != 1 || len(payload.Data.EngineFailures) != 1 || payload.Data.EngineFailures[0].Engine != "rg" || payload.Data.EngineFailures[0].Kind != "execution_failed" {
		t.Fatalf("auto fallback must classify the failed engine: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	status = Run([]string{"--format", "json", "search", "needle", root, "--engine", "rg", "--fixed-strings"}, &stdout, &stderr)
	if status != 1 || !strings.Contains(stdout.String(), "rg execution failed") || !strings.Contains(stdout.String(), "simulated rg failure") {
		t.Fatalf("explicit engine failure needs clear diagnosis, got status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
}

func TestRunSearchSupportsRegexPattern(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("package demo\nfunc runSearch() {}\nfunc runIndex() {}\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "search", `func run(Search|Index)`, root, "--engine", "builtin", "--glob", "*.go"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Engine     string `json:"engine"`
			MatchCount int    `json:"match_count"`
			Matches    []struct {
				Path string `json:"path"`
				Line int    `json:"line"`
				Text string `json:"text"`
			} `json:"matches"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data.Engine != "builtin" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload.Data.MatchCount != 2 || len(payload.Data.Matches) != 2 {
		t.Fatalf("expected two regex matches, got %#v", payload.Data)
	}
	if payload.Data.Matches[0].Line != 2 || !strings.Contains(payload.Data.Matches[0].Text, "runSearch") {
		t.Fatalf("unexpected first regex match: %#v", payload.Data.Matches[0])
	}
	if payload.Data.Matches[1].Line != 3 || !strings.Contains(payload.Data.Matches[1].Text, "runIndex") {
		t.Fatalf("unexpected second regex match: %#v", payload.Data.Matches[1])
	}
}

func TestRunIndexReturnsTree(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &stdout, &stderr)
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
	status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexStdout, &indexStderr)
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
	status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexStdout, &indexStderr)
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
	status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexStdout, &indexStderr)
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
	status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexStdout, &indexStderr)
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
	status := Run([]string{"--format", "json", "index", "--depth", "2", root, "--no-metadata-methods"}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "json", "map", root, "--full"}, &stdout, &stderr)
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
	status := Run([]string{"--format", "json", "index", "--depth", "2", root, "--no-metadata-methods"}, &indexStdout, &indexStderr)
	if status != 0 {
		t.Fatalf("expected index success status, got %d stderr=%q stdout=%q", status, indexStderr.String(), indexStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status = Run([]string{"--format", "text", "map", root, "--full"}, &stdout, &stderr)
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

func TestMapMarksReadyMetadataStaleAfterNewFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write indexed file: %v", err)
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package newfile\n"), 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Map struct {
				State            string `json:"state"`
				Freshness        string `json:"freshness"`
				FreshnessChecked bool   `json:"freshness_checked"`
				Issues           []struct {
					Kind string `json:"kind"`
					Path string `json:"path"`
				} `json:"issues"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	if payload.Data.Map.State != "ready" || payload.Data.Map.Freshness != "stale" || !payload.Data.Map.FreshnessChecked || len(payload.Data.Map.Issues) != 1 || payload.Data.Map.Issues[0].Kind != "new_file" || filepath.Base(payload.Data.Map.Issues[0].Path) != "new.go" {
		t.Fatalf("ready metadata must be visibly stale after a new file: %s", stdout.String())
	}
}

func TestMapMarksReadyMetadataStaleAfterSameStatContentChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("write indexed file: %v", err)
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat indexed file: %v", err)
	}
	if err := os.WriteFile(path, []byte("bravo\n"), 0o644); err != nil {
		t.Fatalf("replace content: %v", err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("restore modtime: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Map struct {
				Freshness string `json:"freshness"`
				Issues    []struct {
					Kind string `json:"kind"`
				} `json:"issues"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	if payload.Data.Map.Freshness != "stale" || len(payload.Data.Map.Issues) != 1 || payload.Data.Map.Issues[0].Kind != "modified" {
		t.Fatalf("content fingerprint drift must invalidate old metadata: %s", stdout.String())
	}
}

func TestMapReportsRenameAndRemovesDeletedFileProjection(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old.txt")
	if err := os.WriteFile(oldPath, []byte("content\n"), 0o644); err != nil {
		t.Fatalf("write indexed file: %v", err)
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}
	if err := os.Rename(oldPath, filepath.Join(root, "new.txt")); err != nil {
		t.Fatalf("rename indexed file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Map struct {
				Freshness string `json:"freshness"`
				Files     []struct {
					Path string `json:"path"`
				} `json:"files"`
				Issues []struct {
					Kind string `json:"kind"`
					Path string `json:"path"`
				} `json:"issues"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	kinds := map[string]bool{}
	for _, issue := range payload.Data.Map.Issues {
		kinds[issue.Kind] = true
	}
	for _, file := range payload.Data.Map.Files {
		if filepath.Base(file.Path) == "old.txt" {
			t.Fatalf("deleted projection must not be presented as current: %s", stdout.String())
		}
	}
	if payload.Data.Map.Freshness != "stale" || !kinds["new_file"] || !kinds["deleted_file"] {
		t.Fatalf("rename must expose added and deleted drift: %s", stdout.String())
	}
}

func TestMapReportsIgnorePolicyDriftAndRecomputesVisibleFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".aaignore"), []byte("# initially empty\n"), 0o644); err != nil {
		t.Fatalf("write initial ignore policy: %v", err)
	}
	for _, name := range []string{"keep.txt", "drop.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}
	if err := os.WriteFile(filepath.Join(root, ".aaignore"), []byte("drop.txt\n"), 0o644); err != nil {
		t.Fatalf("change ignore policy: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Map struct {
				Freshness string `json:"freshness"`
				Files     []struct {
					Path string `json:"path"`
				} `json:"files"`
				Issues []struct {
					Kind string `json:"kind"`
				} `json:"issues"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	policyIssue := false
	for _, issue := range payload.Data.Map.Issues {
		policyIssue = policyIssue || issue.Kind == "ignore_policy_changed"
	}
	for _, file := range payload.Data.Map.Files {
		if filepath.Base(file.Path) == "drop.txt" {
			t.Fatalf("new ignore policy must remove drop.txt from visible projection: %s", stdout.String())
		}
	}
	if payload.Data.Map.Freshness != "stale" || !policyIssue {
		t.Fatalf("ignore policy drift must be explicit: %s", stdout.String())
	}
}

func TestMapExposesIndexedDescendantBelowUnindexedParent(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "parent", "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir descendant: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deep, "note.txt"), []byte("indexed descendant\n"), 0o644); err != nil {
		t.Fatalf("write descendant: %v", err)
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", deep, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index descendant failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	type node struct {
		Path    string `json:"path"`
		State   string `json:"state"`
		Partial bool   `json:"partial"`
		Next    string `json:"next"`
		Issues  []struct {
			Kind string `json:"kind"`
		} `json:"issues"`
		Children []node `json:"children"`
	}
	var payload struct {
		Data struct {
			Map node `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	rootNode := payload.Data.Map
	if rootNode.State != "unindexed" || !rootNode.Partial || len(rootNode.Issues) != 1 || rootNode.Issues[0].Kind != "uninspected_subtree" || rootNode.Next == "" || len(rootNode.Children) != 1 || len(rootNode.Children[0].Children) != 1 || rootNode.Children[0].Children[0].State != "ready" {
		t.Fatalf("unindexed parent must expose deterministic descendant routing: %s", stdout.String())
	}
}

func TestMapKeepsHealthySiblingWhenChildMetadataIsCorrupt(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"healthy", "broken"} {
		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(path, "note.txt"), []byte(dir+"\n"), 0o644); err != nil {
			t.Fatalf("write %s fixture: %v", dir, err)
		}
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}
	if err := os.WriteFile(filepath.Join(root, "broken", ".aascribe_index_meta.json"), []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("corrupt metadata: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map should preserve partial routing, got status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Map struct {
				Partial  bool `json:"partial"`
				Children []struct {
					Path   string `json:"path"`
					State  string `json:"state"`
					Issues []struct {
						Kind string `json:"kind"`
					} `json:"issues"`
				} `json:"children"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	states := map[string]string{}
	issueKinds := map[string]string{}
	for _, child := range payload.Data.Map.Children {
		base := filepath.Base(child.Path)
		states[base] = child.State
		if len(child.Issues) > 0 {
			issueKinds[base] = child.Issues[0].Kind
		}
	}
	if !payload.Data.Map.Partial || states["healthy"] != "ready" || states["broken"] != "corrupt" || issueKinds["broken"] != "corrupt_metadata" {
		t.Fatalf("corrupt child must not erase healthy sibling routing: %s", stdout.String())
	}
}

func TestMapKeepsHealthySiblingWhenChildMetadataIsUnreadable(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"healthy", "denied"} {
		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(path, "note.txt"), []byte(dir+"\n"), 0o644); err != nil {
			t.Fatalf("write %s fixture: %v", dir, err)
		}
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}
	metadataPath := filepath.Join(root, "denied", ".aascribe_index_meta.json")
	if err := os.Chmod(metadataPath, 0); err != nil {
		t.Fatalf("chmod metadata: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(metadataPath, 0o644) })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map should preserve partial routing, got status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Map struct {
				Partial  bool `json:"partial"`
				Children []struct {
					Path   string `json:"path"`
					State  string `json:"state"`
					Issues []struct {
						Kind string `json:"kind"`
					} `json:"issues"`
				} `json:"children"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	states := map[string]string{}
	issueKinds := map[string]string{}
	for _, child := range payload.Data.Map.Children {
		base := filepath.Base(child.Path)
		states[base] = child.State
		if len(child.Issues) > 0 {
			issueKinds[base] = child.Issues[0].Kind
		}
	}
	if !payload.Data.Map.Partial || states["healthy"] != "ready" || states["denied"] != "unreadable" || issueKinds["denied"] != "unreadable_metadata" {
		t.Fatalf("unreadable child must remain explicit beside healthy sibling: %s", stdout.String())
	}
}

func TestIndexEvalAndMapFollowInRootDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "memory")
	target := filepath.Join(root, ".agents", "skills", "adapt")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("# Adapt\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	linkParent := filepath.Join(root, ".claude", "skills")
	if err := os.MkdirAll(linkParent, 0o755); err != nil {
		t.Fatalf("mkdir link parent: %v", err)
	}
	alias := filepath.Join(linkParent, "adapt")
	if err := os.Symlink(filepath.Join("..", "..", ".agents", "skills", "adapt"), alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--store", storePath, "--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}
	var evalOut bytes.Buffer
	var evalErr bytes.Buffer
	if status := Run([]string{"--store", storePath, "--format", "json", "index", "eval", root}, &evalOut, &evalErr); status != 0 {
		t.Fatalf("eval must follow the same in-root directory symlink policy: status=%d stderr=%q stdout=%q", status, evalErr.String(), evalOut.String())
	}
	if !strings.Contains(evalOut.String(), filepath.ToSlash(alias)) {
		t.Fatalf("eval omitted symlink directory: %s", evalOut.String())
	}

	var mapOut bytes.Buffer
	var mapErr bytes.Buffer
	if status := Run([]string{"--store", storePath, "--format", "json", "map", root}, &mapOut, &mapErr); status != 0 {
		t.Fatalf("map must follow the same in-root directory symlink policy: status=%d stderr=%q stdout=%q", status, mapErr.String(), mapOut.String())
	}
	expanded := expandManagedJSONEnvelope(t, mapOut.Bytes(), storePath)
	if !strings.Contains(string(expanded), filepath.ToSlash(alias)) || !strings.Contains(string(expanded), `"state":"ready"`) {
		t.Fatalf("map omitted indexed symlink directory: %s", mapOut.String())
	}
}

func TestMapStopsAtOutOfRootSymlinkBoundary(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "secret.txt"), []byte("outside scope\n"), 0o644); err != nil {
		t.Fatalf("write external fixture: %v", err)
	}
	link := filepath.Join(root, "external-link")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map boundary decision should be structured success: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Map struct {
				Partial  bool `json:"partial"`
				Children []struct {
					Path    string `json:"path"`
					Partial bool   `json:"partial"`
					Issues  []struct {
						Kind string `json:"kind"`
					} `json:"issues"`
					Children []any `json:"children"`
				} `json:"children"`
			} `json:"map"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	if !payload.Data.Map.Partial || len(payload.Data.Map.Children) != 1 || !payload.Data.Map.Children[0].Partial || len(payload.Data.Map.Children[0].Issues) != 1 || payload.Data.Map.Children[0].Issues[0].Kind != "symlink_out_of_root" || len(payload.Data.Map.Children[0].Children) != 0 || strings.Contains(stdout.String(), "secret.txt") {
		t.Fatalf("map escaped requested root or hid boundary decision: %s", stdout.String())
	}
}

func TestMapStopsAtDirectorySymlinkCycle(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir cycle fixture: %v", err)
	}
	if err := os.Symlink(".", filepath.Join(dir, "loop")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index must stop safely at cycle: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}
	var evalOut bytes.Buffer
	var evalErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", "eval", root}, &evalOut, &evalErr); status != 0 || !strings.Contains(evalOut.String(), `"reason": "symlink_cycle"`) {
		t.Fatalf("eval must report cycle boundary: status=%d stderr=%q stdout=%q", status, evalErr.String(), evalOut.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("cycle decision should be structured success: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"kind": "symlink_cycle"`) || stdout.Len() > 20_000 {
		t.Fatalf("map did not terminate at symlink cycle: bytes=%d output=%s", stdout.Len(), stdout.String())
	}
}

func TestMapDeclaresBlacklistOnlyIgnorePolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".aaignore"), []byte("*.log\n!important.log\n"), 0o644); err != nil {
		t.Fatalf("write ignore policy: %v", err)
	}
	for _, name := range []string{"debug.log", "important.log", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			IgnorePolicy struct {
				Mode              string   `json:"mode"`
				NegationSupported bool     `json:"negation_supported"`
				Sources           []string `json:"sources"`
			} `json:"ignore_policy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	if payload.Data.IgnorePolicy.Mode != "blacklist" || payload.Data.IgnorePolicy.NegationSupported || !reflect.DeepEqual(payload.Data.IgnorePolicy.Sources, []string{".gitignore", ".aaignore"}) {
		t.Fatalf("map must disclose unsupported re-inclusion semantics: %s", stdout.String())
	}
}

func TestMapDefaultsBoundWideAndDeepTreesWithContinuations(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "memory")
	for i := 0; i < 25; i++ {
		name := filepath.Join(root, fmt.Sprintf("file-%02d.txt", i))
		if err := os.WriteFile(name, []byte("content\n"), 0o644); err != nil {
			t.Fatalf("write wide fixture: %v", err)
		}
	}
	deep := filepath.Join(root, "d1", "d2", "d3", "d4")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deep, "leaf.txt"), []byte("leaf\n"), 0o644); err != nil {
		t.Fatalf("write deep fixture: %v", err)
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--store", storePath, "--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--store", storePath, "--format", "json", "map", root}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	type omission struct {
		Reason string `json:"reason"`
		Path   string `json:"path"`
		Next   string `json:"next"`
	}
	type node struct {
		Partial  bool       `json:"partial"`
		Files    []any      `json:"files"`
		Omitted  []omission `json:"omitted"`
		Children []node     `json:"children"`
	}
	var payload struct {
		Data struct {
			Map node `json:"map"`
		} `json:"data"`
	}
	expanded := expandManagedJSONEnvelope(t, stdout.Bytes(), storePath)
	if err := json.Unmarshal(expanded, &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	reasons := map[string]bool{}
	var inspect func(node)
	inspect = func(current node) {
		for _, omitted := range current.Omitted {
			if omitted.Path == "" || omitted.Next == "" {
				t.Fatalf("every omission needs path and continuation: %s", stdout.String())
			}
			reasons[omitted.Reason] = true
		}
		for _, child := range current.Children {
			inspect(child)
		}
	}
	inspect(payload.Data.Map)
	if !payload.Data.Map.Partial || len(payload.Data.Map.Files) > 20 || !reasons["file_limit"] || !reasons["depth_limit"] {
		t.Fatalf("default map must bound width and depth semantically: %s", stdout.String())
	}

	var textOut, textErr bytes.Buffer
	if status := Run([]string{"--store", storePath, "--format", "text", "map", root}, &textOut, &textErr); status != 0 {
		t.Fatalf("text map failed: status=%d stderr=%q", status, textErr.String())
	}
	if !strings.Contains(textOut.String(), "--tag") || !strings.Contains(textOut.String(), "--recent-modified") {
		t.Fatalf("default map must advertise focused discovery flags: %s", textOut.String())
	}
}

func TestMapTagCatalogAndQueryReturnOnlyTagInformation(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"guide.md", "server.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	var indexOut, indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q", status, indexErr.String())
	}
	metadataPath := filepath.Join(root, ".aascribe_index_meta.json")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	files := metadata["files"].([]any)
	for _, raw := range files {
		file := raw.(map[string]any)
		tags := []any{"project"}
		if strings.HasSuffix(file["path"].(string), "guide.md") {
			tags = append(tags, "docs")
		}
		file["methods"] = map[string]any{
			"builtin.classify.v1": map[string]any{"stage": "file", "status": "ok", "fields": map[string]any{"tags": tags}},
		}
	}
	updated, _ := json.Marshal(metadata)
	if err := os.WriteFile(metadataPath, updated, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	var catalogOut, catalogErr bytes.Buffer
	if status := Run([]string{"--format", "text", "map", root, "--tag"}, &catalogOut, &catalogErr); status != 0 {
		t.Fatalf("tag catalog failed: status=%d stderr=%q", status, catalogErr.String())
	}
	if !strings.Contains(catalogOut.String(), "docs (1 files)") || !strings.Contains(catalogOut.String(), "project (2 files)") || strings.Contains(catalogOut.String(), "summary:") {
		t.Fatalf("unexpected compact tag catalog: %s", catalogOut.String())
	}

	var queryOut, queryErr bytes.Buffer
	if status := Run([]string{"--format", "text", "map", root, "--tag", "PROJECT", "--tag", "docs"}, &queryOut, &queryErr); status != 0 {
		t.Fatalf("tag query failed: status=%d stderr=%q", status, queryErr.String())
	}
	if !strings.Contains(queryOut.String(), "guide.md") || strings.Contains(queryOut.String(), "server.go") || strings.Contains(queryOut.String(), "summary:") {
		t.Fatalf("tag query must use case-insensitive AND matching without the normal map: %s", queryOut.String())
	}
}

func TestMapRecentModifiedReturnsCompactNewestFirstList(t *testing.T) {
	root := t.TempDir()
	older := filepath.Join(root, "older.txt")
	newer := filepath.Join(root, "newer.txt")
	for _, path := range []string{older, newer} {
		if err := os.WriteFile(path, []byte(path+"\n"), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(24 * time.Hour)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatalf("set older time: %v", err)
	}
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatalf("set newer time: %v", err)
	}
	var indexOut, indexErr bytes.Buffer
	if status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q", status, indexErr.String())
	}

	added := filepath.Join(root, "added.txt")
	if err := os.WriteFile(added, []byte("new after indexing\n"), 0o644); err != nil {
		t.Fatalf("write newly discovered fixture: %v", err)
	}
	addedTime := newTime.Add(24 * time.Hour)
	if err := os.Chtimes(added, addedTime, addedTime); err != nil {
		t.Fatalf("set added time: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"--format", "text", "map", root, "-recent-modified"}, &stdout, &stderr); status != 0 {
		t.Fatalf("recent-modified failed: status=%d stderr=%q", status, stderr.String())
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "recent_modified: 3 files") || strings.Index(rendered, "added.txt") > strings.Index(rendered, "newer.txt") || strings.Index(rendered, "newer.txt") > strings.Index(rendered, "older.txt") || strings.Contains(rendered, "state guide:") || strings.Contains(rendered, "summary:") {
		t.Fatalf("expected compact newest-first output, got: %s", rendered)
	}
}

func TestMapFullRecoversCompleteHierarchy(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "memory")
	for i := 0; i < 25; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%02d.txt", i)), []byte("content\n"), 0o644); err != nil {
			t.Fatalf("write wide fixture: %v", err)
		}
	}
	deep := filepath.Join(root, "d1", "d2", "d3", "d4")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deep, "leaf.txt"), []byte("leaf\n"), 0o644); err != nil {
		t.Fatalf("write leaf: %v", err)
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--store", storePath, "--format", "json", "index", root, "--depth", "8", "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--store", storePath, "--format", "json", "map", root, "--full"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("full map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	var payload struct {
		Data struct {
			Projection struct {
				Mode string `json:"mode"`
			} `json:"projection"`
			Map struct {
				Files []any `json:"files"`
			} `json:"map"`
		} `json:"data"`
	}
	expanded := expandManagedJSONEnvelope(t, stdout.Bytes(), storePath)
	if err := json.Unmarshal(expanded, &payload); err != nil {
		t.Fatalf("unmarshal map: %v\n%s", err, stdout.String())
	}
	if payload.Data.Projection.Mode != "full" || len(payload.Data.Map.Files) != 25 || strings.Contains(string(expanded), `"omitted"`) || !strings.Contains(string(expanded), "leaf.txt") {
		t.Fatalf("--full must explicitly recover the complete hierarchy: %s", stdout.String())
	}
}

func TestMapOversizedTextSpillsToManagedOutput(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "memory")
	for i := 0; i < 220; i++ {
		name := filepath.Join(root, fmt.Sprintf("verbose-file-%03d.txt", i))
		if err := os.WriteFile(name, []byte("content\n"), 0o644); err != nil {
			t.Fatalf("write large map fixture: %v", err)
		}
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--store", storePath, "--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--store", storePath, "--format", "text", "map", root, "--full"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	rendered := stdout.String()
	if len([]rune(rendered)) > 6_000 || !strings.Contains(rendered, "[partial output]") || !strings.Contains(rendered, "output_id: out_") || !strings.Contains(rendered, "next: aascribe output slice out_") {
		t.Fatalf("oversized map text must spill with deterministic continuation: runes=%d output=%s", len([]rune(rendered)), rendered)
	}
}

func TestMapOversizedJSONStoresCompletePayloadWithoutInlineDuplication(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "memory")
	for i := 0; i < 220; i++ {
		name := filepath.Join(root, fmt.Sprintf("verbose-file-%03d.txt", i))
		if err := os.WriteFile(name, []byte("content\n"), 0o644); err != nil {
			t.Fatalf("write large map fixture: %v", err)
		}
	}
	var indexOut bytes.Buffer
	var indexErr bytes.Buffer
	if status := Run([]string{"--store", storePath, "--format", "json", "index", root, "--no-metadata-methods"}, &indexOut, &indexErr); status != 0 {
		t.Fatalf("index failed: status=%d stderr=%q stdout=%q", status, indexErr.String(), indexOut.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--store", storePath, "--format", "json", "map", root, "--full"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("map failed: status=%d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	if len([]rune(stdout.String())) > 8_000 {
		t.Fatalf("oversized JSON map duplicated the full payload inline: runes=%d", len([]rune(stdout.String())))
	}
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal map envelope: %v\n%s", err, stdout.String())
	}
	if _, duplicated := payload.Data["value"]; duplicated {
		t.Fatalf("oversized JSON map must not duplicate its full value beside transport: %#v", payload.Data)
	}
	transport, ok := payload.Data["transport"].(map[string]any)
	if !ok || transport["truncated"] != true {
		t.Fatalf("expected truncated transport metadata: %#v", payload.Data)
	}
	outputID, _ := transport["output_id"].(string)
	if outputID == "" {
		t.Fatalf("expected managed output id: %#v", transport)
	}
	ref, err := largeoutput.Meta(storePath, outputID)
	if err != nil {
		t.Fatalf("load managed output metadata: %v", err)
	}
	chunk, err := largeoutput.Slice(storePath, outputID, 0, ref.TotalRunes, largeoutput.DefaultConfig())
	if err != nil {
		t.Fatalf("load complete managed output: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal([]byte(chunk.Text), &stored); err != nil {
		t.Fatalf("managed output must contain complete JSON map payload: %v", err)
	}
	if !strings.Contains(chunk.Text, "verbose-file-219.txt") || stored["projection"] == nil {
		t.Fatalf("managed JSON payload is incomplete: %s", chunk.Text)
	}
}

func TestRunIndexDirtyMarksMetadataDirty(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var indexStdout bytes.Buffer
	var indexStderr bytes.Buffer
	status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexStdout, &indexStderr)
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
	status := Run([]string{"--format", "json", "index", root, "--no-metadata-methods"}, &indexStdout, &indexStderr)
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
	configPath := config.ConfigPath(storePath)
	assertExists(t, configPath)
	if data["config_path"] != configPath {
		t.Fatalf("expected config_path %q, got %#v", configPath, data["config_path"])
	}
	if data["config_created"] != true {
		t.Fatalf("expected config_created=true, got %#v", data["config_created"])
	}
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected config file to be readable, got %v", err)
	}
	if !strings.Contains(string(configBytes), `api_key_env = "GEMINI_API_KEY"`) {
		t.Fatalf("expected default Gemini config template, got %s", string(configBytes))
	}
}

func TestInitFailsWithoutForceWhenStoreExists(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	assertNoError(t, os.MkdirAll(config.ConfigDir(storePath), 0o755))
	assertNoError(t, os.WriteFile(config.ConfigPath(storePath), []byte(`[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
`), 0o644))
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

func TestInitRepairsMissingConfigWhenStoreExists(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	assertNoError(t, os.MkdirAll(filepath.Join(storePath, "short_term"), 0o755))
	assertNoError(t, os.WriteFile(filepath.Join(storePath, "layout.json"), []byte(`{"layout_version":"bootstrap-v1"}`), 0o644))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "--store", storePath, "init"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected repair init success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	data := payload["data"].(map[string]any)
	if data["created"] != false {
		t.Fatalf("expected existing store created=false, got %#v", data["created"])
	}
	if data["reinitialized"] != false {
		t.Fatalf("expected repair without reinit, got %#v", data["reinitialized"])
	}
	if data["config_created"] != true {
		t.Fatalf("expected missing config to be created, got %#v", data["config_created"])
	}
	assertExists(t, config.ConfigPath(storePath))
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
	if !strings.Contains(rendered, "config: "+config.ConfigPath(storePath)) {
		t.Fatalf("expected config path hint, got %q", rendered)
	}
	if !strings.Contains(rendered, "next: set GEMINI_API_KEY") {
		t.Fatalf("expected LLM secret next-step hint, got %q", rendered)
	}
}

func TestRunConfigGeminiAPIKeyCreatesStoreAndEnvFile(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--format", "json", "--store", storePath, "config", "gemini_api_key=test-secret"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected config success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "test-secret") {
		t.Fatalf("expected config output not to echo secret, got %s", stdout.String())
	}
	assertExists(t, config.ConfigPath(storePath))
	envPath := filepath.Join(config.ConfigDir(storePath), ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("expected .env to be readable, got %v", err)
	}
	if string(content) != "GEMINI_API_KEY=test-secret\n" {
		t.Fatalf("unexpected .env content: %q", string(content))
	}
}

func TestRunCommandAutoCreatesMissingStore(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--format", "json", "--store", storePath, "logs", "path"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected logs path success, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	assertExists(t, filepath.Join(storePath, "layout.json"))
	assertExists(t, config.ConfigPath(storePath))
}

func TestRunCommandAutoRepairsMissingConfig(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	assertNoError(t, os.MkdirAll(filepath.Join(storePath, "short_term"), 0o755))
	assertNoError(t, os.WriteFile(filepath.Join(storePath, "layout.json"), []byte(`{"layout_version":"bootstrap-v1"}`), 0o644))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	t.Setenv("GEMINI_API_KEY", "")
	status := Run([]string{"--format", "json", "--store", storePath, "index", t.TempDir()}, &stdout, &stderr)
	if status != 1 {
		t.Fatalf("expected missing secret status after config repair, got %d stderr=%q stdout=%q", status, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "CONFIG_NOT_FOUND") {
		t.Fatalf("expected missing config to be repaired before command execution, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "MISSING_SECRET") {
		t.Fatalf("expected missing secret after config repair, got %s", stdout.String())
	}
	assertExists(t, config.ConfigPath(storePath))
}

func TestInitPreservesExistingConfig(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	assertNoError(t, os.MkdirAll(storePath, 0o755))
	configPath := config.ConfigPath(storePath)
	customConfig := []byte("[llm]\nprovider = \"gemini\"\nmodel = \"custom\"\napi_key_env = \"CUSTOM_KEY\"\ntimeout_seconds = 30\n")
	assertNoError(t, os.MkdirAll(config.ConfigDir(storePath), 0o755))
	assertNoError(t, os.WriteFile(configPath, customConfig, 0o644))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Run([]string{"--format", "json", "--store", storePath, "init", "--force"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected success status, got %d with output %s", status, stdout.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	data := payload["data"].(map[string]any)
	if data["config_created"] != false {
		t.Fatalf("expected config_created=false, got %#v", data["config_created"])
	}
	bytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected config file to remain readable, got %v", err)
	}
	if string(bytes) != string(customConfig) {
		t.Fatalf("expected existing config to be preserved, got %s", string(bytes))
	}
}

func TestLLMCommandAfterInitReportsMissingSecretNotMissingConfig(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "aascribe-store")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := Run([]string{"--store", storePath, "init"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("expected init success, got %d with output %s", status, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	t.Setenv("GEMINI_API_KEY", "")
	status = Run([]string{"--format", "json", "--store", storePath, "chat", "hello"}, &stdout, &stderr)
	if status != 1 {
		t.Fatalf("expected missing secret runtime failure, got %d with output %s", status, stdout.String())
	}
	if strings.Contains(stdout.String(), "CONFIG_NOT_FOUND") {
		t.Fatalf("expected config to exist after init, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "MISSING_SECRET") {
		t.Fatalf("expected missing secret guidance, got %s", stdout.String())
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
	_, err := largeoutput.Deliver(storePath, "chat", "one\ntwo\nthree\nfour", largeoutput.DeliverOptions{Config: largeoutput.Config{InlineRuneLimit: 4}})
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

func expandManagedJSONEnvelope(t *testing.T, envelope []byte, storePath string) []byte {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(envelope, &raw); err != nil {
		t.Fatalf("unmarshal JSON envelope: %v\n%s", err, envelope)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw["data"], &data); err != nil {
		t.Fatalf("unmarshal JSON data: %v\n%s", err, envelope)
	}
	transportRaw, spilled := data["transport"]
	if !spilled {
		return envelope
	}
	var transport struct {
		OutputID string `json:"output_id"`
	}
	if err := json.Unmarshal(transportRaw, &transport); err != nil || transport.OutputID == "" {
		t.Fatalf("invalid managed transport: %v\n%s", err, envelope)
	}
	ref, err := largeoutput.Meta(storePath, transport.OutputID)
	if err != nil {
		t.Fatalf("load managed JSON metadata: %v", err)
	}
	chunk, err := largeoutput.Slice(storePath, transport.OutputID, 0, ref.TotalRunes, largeoutput.DefaultConfig())
	if err != nil {
		t.Fatalf("load managed JSON payload: %v", err)
	}
	if !json.Valid([]byte(chunk.Text)) {
		t.Fatalf("managed payload is not JSON: %s", chunk.Text)
	}
	raw["data"] = json.RawMessage(chunk.Text)
	expanded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("rebuild JSON envelope: %v", err)
	}
	return expanded
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
