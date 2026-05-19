package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/config"
	"github.com/austinjan/aascribe/internal/index"
	"github.com/austinjan/aascribe/internal/llm"
	"github.com/austinjan/aascribe/internal/logging"
	"github.com/austinjan/aascribe/internal/operation"
	"github.com/austinjan/aascribe/internal/output"
	"github.com/austinjan/aascribe/internal/search"
	"github.com/austinjan/aascribe/internal/store"
	"github.com/austinjan/agentcli-core/largeoutput"
)

var promptRunner = runPrompt

type tokenUsageTracker struct {
	mu              sync.Mutex
	promptTokens    int
	candidateTokens int
	totalTokens     int
	provider        string
	model           string
	available       bool
}

func (t *tokenUsageTracker) note(provider, model string, usage llm.Usage) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.available = true
	t.provider = provider
	t.model = model
	t.promptTokens += usage.PromptTokenCount
	t.candidateTokens += usage.CandidatesTokenCount
	t.totalTokens += usage.TotalTokenCount
}

func (t *tokenUsageTracker) snapshot() tokenUsageSnapshot {
	if t == nil {
		return tokenUsageSnapshot{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return tokenUsageSnapshot{
		PromptTokens:    t.promptTokens,
		CandidateTokens: t.candidateTokens,
		TotalTokens:     t.totalTokens,
		Provider:        t.provider,
		Model:           t.model,
		Available:       t.available,
	}
}

type tokenUsageSnapshot struct {
	PromptTokens    int
	CandidateTokens int
	TotalTokens     int
	Provider        string
	Model           string
	Available       bool
}

func Execute(command cli.Command, storePath string) (*output.CommandResult, error) {
	switch cmd := command.(type) {
	case cli.InitCommand:
		return runInit(storePath, cmd.Force)
	case cli.ConfigSetCommand:
		return runConfigSet(storePath, cmd)
	case cli.LogsPathCommand:
		return runLogsPath(storePath), nil
	case cli.LogsExportCommand:
		return runLogsExport(storePath, cmd.Output)
	case cli.LogsClearCommand:
		return runLogsClear(storePath, cmd.Force)
	case cli.OutputListCommand:
		return runOutputList(storePath)
	case cli.OutputGenerateCommand:
		return runOutputGenerate(cmd)
	case cli.OutputMetaCommand:
		return runOutputMeta(storePath, cmd.ID)
	case cli.OutputShowCommand:
		return runOutputShow(storePath, cmd.ID)
	case cli.OutputHeadCommand:
		return runOutputHead(storePath, cmd.ID, cmd.Lines)
	case cli.OutputTailCommand:
		return runOutputTail(storePath, cmd.ID, cmd.Lines)
	case cli.OutputSliceCommand:
		return runOutputSlice(storePath, cmd.ID, cmd.Offset, cmd.Limit)
	case cli.OperationListCommand:
		return runOperationList(storePath)
	case cli.OperationStatusCommand:
		return runOperationStatus(storePath, cmd.ID)
	case cli.OperationEventsCommand:
		return runOperationEvents(storePath, cmd.ID)
	case cli.OperationResultCommand:
		return runOperationResult(storePath, cmd.ID)
	case cli.OperationCancelCommand:
		return runOperationCancel(storePath, cmd.ID)
	case cli.OperationCleanCommand:
		return runOperationClean(storePath, cmd)
	case cli.OperationRunIndexCommand:
		return runOperationRunIndex(storePath, cmd.OperationID, cmd.Index)
	case cli.IndexCommand:
		return runIndex(storePath, cmd)
	case cli.IndexCleanCommand:
		return runIndexClean(cmd)
	case cli.IndexDirtyCommand:
		return runIndexDirty(cmd)
	case cli.IndexEvalCommand:
		return runIndexEval(cmd)
	case cli.IndexMethodsCommand:
		return runIndexMethods(cmd)
	case cli.IndexPlanValidateCommand:
		return runIndexPlanValidate(cmd)
	case cli.IndexReviewCommand:
		return runIndexReview(storePath, cmd)
	case cli.IndexReviewProposeCommand:
		return runIndexReviewPropose(storePath, cmd)
	case cli.IndexReviewPatchCommand:
		return runIndexReviewPatch(storePath, cmd)
	case cli.IndexReviewStatusCommand:
		return runIndexReviewStatus(storePath, cmd.CheckpointID)
	case cli.IndexReviewAcceptCommand:
		return runIndexReviewAccept(storePath, cmd.CheckpointID)
	case cli.IndexReviewRestoreCommand:
		return runIndexReviewRestore(storePath, cmd.CheckpointID)
	case cli.IndexMapCommand:
		return runIndexMap(cmd)
	case cli.MapCommand:
		return runMap(cmd)
	case cli.SearchCommand:
		return runSearch(cmd)
	case cli.DescribeCommand:
		return runDescribe(storePath, cmd)
	case cli.RememberCommand:
		return nil, apperr.NotImplemented(cmd.Name())
	case cli.ConsolidateCommand:
		return nil, apperr.NotImplemented(cmd.Name())
	case cli.RecallCommand:
		return nil, apperr.NotImplemented(cmd.Name())
	case cli.ChatCommand:
		return runChat(storePath, cmd.Prompt)
	case cli.SummarizeCommand:
		return runSummarize(storePath, cmd.File)
	case cli.ListCommand:
		return nil, apperr.NotImplemented(cmd.Name())
	case cli.ShowCommand:
		return nil, apperr.NotImplemented(cmd.Name())
	case cli.ForgetCommand:
		return nil, apperr.NotImplemented(cmd.Name())
	default:
		return nil, apperr.NotImplemented("unknown")
	}
}

var startAsyncIndexProcess = startIndexWorkerProcess

func resolveIndexPlan(cmd cli.IndexCommand) (cli.IndexCommand, error) {
	if strings.TrimSpace(cmd.IndexPlan) == "" {
		return cmd, nil
	}
	validation, err := index.ValidateIndexPlanFile(cmd.IndexPlan)
	if err != nil {
		return cmd, err
	}
	cmd.IndexPlan = validation.Path
	cmd.IndexPlanDigest = validation.Digest
	cmd.MetadataMethods = validation.Plan.Methods.Enabled
	cmd.NoMetadataMethods = false
	return cmd, nil
}

func runIndex(storePath string, cmd cli.IndexCommand) (*output.CommandResult, error) {
	if cmd.Async {
		return runIndexAsync(storePath, cmd)
	}
	var err error
	cmd, err = resolveIndexPlan(cmd)
	if err != nil {
		return nil, err
	}
	usage := &tokenUsageTracker{}
	noSummary := cmd.EffectiveNoSummary()
	var summarizer index.SummarizerFunc
	if !noSummary {
		var err error
		summarizer, err = buildRequiredLLMSummarizer(storePath, usage)
		if err != nil {
			return nil, err
		}
	}
	methodPrompts := loadIndexMethodPrompts(storePath, cmd.EffectiveMetadataMethods())
	result, err := index.Build(index.Options{
		Root:                cmd.Path,
		Depth:               cmd.Depth,
		Include:             cmd.Include,
		Exclude:             cmd.Exclude,
		MaxConcurrency:      cmd.Concurrency,
		Refresh:             cmd.Refresh,
		NoSummary:           noSummary,
		MaxFileSize:         cmd.MaxFileSize,
		MetadataMethods:     cmd.EffectiveMetadataMethods(),
		MethodPrompts:       methodPrompts,
		Summarizer:          summarizer,
		FailureNoticeWriter: os.Stderr,
	})
	if err != nil {
		return nil, err
	}
	reportPath, err := writeIndexCompletionReport(storePath, cmd, result, "", "sync", usage.snapshot())
	if err != nil {
		return nil, err
	}
	result.ReportPath = reportPath

	return &output.CommandResult{
		Data: result,
		Text: renderIndexText(result) + "\nreport: " + reportPath,
	}, nil
}

func runIndexMethods(cmd cli.IndexMethodsCommand) (*output.CommandResult, error) {
	descriptors, err := index.ListMethodDescriptors(cmd.Stage)
	if err != nil {
		return nil, err
	}
	lines := []string{"available index methods:"}
	if len(descriptors) == 0 {
		lines = append(lines, "- none")
	}
	for _, descriptor := range descriptors {
		lines = append(lines,
			"- "+descriptor.ID,
			"  stage: "+string(descriptor.Stage),
			fmt.Sprintf("  required: %t", descriptor.Required),
			fmt.Sprintf("  default_enabled: %t", descriptor.DefaultEnabled),
			fmt.Sprintf("  always_enabled: %t", descriptor.AlwaysEnabled),
			fmt.Sprintf("  requires_summary: %t", descriptor.RequiresSummary),
			fmt.Sprintf("  supports_no_summary: %t", descriptor.SupportsNoSummary),
			"  purpose: "+descriptor.Purpose,
			"  description: "+descriptor.Description,
		)
	}
	return &output.CommandResult{
		Data: map[string]any{
			"methods": descriptors,
		},
		Text: strings.Join(lines, "\n"),
	}, nil
}

func runIndexPlanValidate(cmd cli.IndexPlanValidateCommand) (*output.CommandResult, error) {
	validation, err := index.ValidateIndexPlanFile(cmd.Path)
	if err != nil {
		return nil, err
	}
	text := strings.Join([]string{
		"index plan valid",
		"path: " + validation.Path,
		"digest: " + validation.Digest,
		"enabled_methods: " + stringListText(validation.Plan.Methods.Enabled),
	}, "\n")
	return &output.CommandResult{
		Data: validation,
		Text: text,
	}, nil
}

type indexReviewState struct {
	CheckpointID     string   `json:"checkpoint_id"`
	State            string   `json:"state"`
	Method           string   `json:"method,omitempty"`
	ReviewedPath     string   `json:"reviewed_path,omitempty"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
	ReviewJSON       string   `json:"review_json,omitempty"`
	ReviewMarkdown   string   `json:"review_markdown,omitempty"`
	ProposalJSON     string   `json:"proposal_json,omitempty"`
	ProposalMarkdown string   `json:"proposal_markdown,omitempty"`
	PromptPath       string   `json:"prompt_path,omitempty"`
	MetadataBackups  []string `json:"metadata_backups,omitempty"`
	PromptBackups    []string `json:"prompt_backups,omitempty"`
}

type indexReviewFinding struct {
	Path       string `json:"path"`
	Severity   string `json:"severity"`
	Kind       string `json:"kind"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type indexReviewReport struct {
	Status         string               `json:"status"`
	CheckpointID   string               `json:"checkpoint_id"`
	Method         string               `json:"method"`
	ReviewedPath   string               `json:"reviewed_path"`
	CreatedAt      string               `json:"created_at"`
	ReviewJSON     string               `json:"review_json"`
	ReviewMarkdown string               `json:"review_markdown"`
	Findings       []indexReviewFinding `json:"findings"`
	Writes         map[string]bool      `json:"writes"`
	Artifacts      []map[string]string  `json:"artifacts"`
	Next           []map[string]string  `json:"next"`
}

type indexReviewProposal struct {
	Status          string               `json:"status"`
	CheckpointID    string               `json:"checkpoint_id"`
	ReviewJSON      string               `json:"review_json"`
	PromptPath      string               `json:"prompt_path"`
	ProposedContent string               `json:"proposed_content"`
	Findings        []indexReviewFinding `json:"findings"`
	Writes          map[string]bool      `json:"writes"`
	Artifacts       []map[string]string  `json:"artifacts"`
	Next            []map[string]string  `json:"next"`
}

func runIndexReview(storePath string, cmd cli.IndexReviewCommand) (*output.CommandResult, error) {
	root, err := filepath.Abs(cmd.Path)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve review path: %s.", cmd.Path)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, apperr.IOError("Failed to inspect review path: %s.", root)
	}
	if !info.IsDir() {
		return nil, apperr.InvalidArguments("index review path must be a directory: %s", root)
	}
	now := time.Now().UTC()
	checkpointID := fmt.Sprintf("index-review-%s", now.Format("20060102T150405Z"))
	checkpointDir := indexReviewCheckpointDir(storePath, checkpointID)
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return nil, apperr.IOError("Failed to create review checkpoint: %s.", checkpointDir)
	}
	metadataBackups, err := snapshotIndexReviewMetadata(root, checkpointDir)
	if err != nil {
		return nil, err
	}
	promptBackups, _ := snapshotIndexReviewPrompts(checkpointDir)
	findings, err := inspectRetrievalMetadata(root, cmd.Method)
	if err != nil {
		return nil, err
	}
	reportDir := filepath.Join(storePath, "report")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return nil, apperr.IOError("Failed to create report directory: %s.", reportDir)
	}
	base := now.Format("2006-01-02-150405") + "-retrieval-review"
	reviewJSON := filepath.Join(reportDir, base+".json")
	reviewMarkdown := filepath.Join(reportDir, base+".md")
	state := indexReviewState{
		CheckpointID:    checkpointID,
		State:           "review_created",
		Method:          cmd.Method,
		ReviewedPath:    root,
		CreatedAt:       now.Format(time.RFC3339),
		UpdatedAt:       now.Format(time.RFC3339),
		ReviewJSON:      reviewJSON,
		ReviewMarkdown:  reviewMarkdown,
		MetadataBackups: metadataBackups,
		PromptBackups:   promptBackups,
	}
	report := indexReviewReport{
		Status:         "review_created",
		CheckpointID:   checkpointID,
		Method:         cmd.Method,
		ReviewedPath:   root,
		CreatedAt:      now.Format(time.RFC3339),
		ReviewJSON:     reviewJSON,
		ReviewMarkdown: reviewMarkdown,
		Findings:       findings,
		Writes: map[string]bool{
			"checkpoint": true,
			"report":     true,
			"metadata":   false,
			"prompt":     false,
		},
		Artifacts: []map[string]string{
			{"type": "checkpoint", "id": checkpointID, "path": checkpointDir},
			{"type": "review_report_json", "path": reviewJSON},
			{"type": "review_report_markdown", "path": reviewMarkdown},
		},
		Next: []map[string]string{
			{"command": "aascribe index review propose " + reviewJSON, "when": "Use if findings indicate prompt-level problems."},
			{"command": "aascribe index review accept " + checkpointID, "when": "Use if no prompt changes are needed."},
			{"command": "aascribe index review restore " + checkpointID, "when": "Use if you need to return to the checkpointed metadata and prompt files."},
		},
	}
	if err := writeJSONFile(reviewJSON, report); err != nil {
		return nil, err
	}
	if err := os.WriteFile(reviewMarkdown, []byte(renderIndexReviewMarkdown(report)), 0o644); err != nil {
		return nil, apperr.IOError("Failed to write review report: %s.", reviewMarkdown)
	}
	if err := saveIndexReviewState(storePath, state); err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: report,
		Text: renderIndexReviewResult(report.Status, report.CheckpointID, report.Artifacts, report.Next),
	}, nil
}

func runIndexReviewPropose(storePath string, cmd cli.IndexReviewProposeCommand) (*output.CommandResult, error) {
	var report indexReviewReport
	if err := readJSONFile(cmd.ReviewJSON, &report); err != nil {
		return nil, err
	}
	state, err := loadIndexReviewState(storePath, report.CheckpointID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	reportDir := filepath.Dir(cmd.ReviewJSON)
	base := now.Format("2006-01-02-150405") + "-retrieval-prompt-proposal"
	proposalJSON := filepath.Join(reportDir, base+".json")
	proposalMarkdown := filepath.Join(reportDir, base+".md")
	promptPath := filepath.Join("prompts", "retrieval", "generate.v1.md")
	proposal := indexReviewProposal{
		Status:          "proposal_created",
		CheckpointID:    report.CheckpointID,
		ReviewJSON:      cmd.ReviewJSON,
		PromptPath:      promptPath,
		ProposedContent: defaultRetrievalPromptProposal(report.Findings),
		Findings:        report.Findings,
		Writes: map[string]bool{
			"checkpoint": true,
			"proposal":   true,
			"metadata":   false,
			"prompt":     false,
		},
		Artifacts: []map[string]string{
			{"type": "prompt_proposal_json", "path": proposalJSON},
			{"type": "prompt_proposal_markdown", "path": proposalMarkdown},
			{"type": "checkpoint", "id": report.CheckpointID},
		},
		Next: []map[string]string{
			{"command": "aascribe index review patch " + proposalJSON, "when": "Use if the prompt proposal is acceptable."},
			{"command": "aascribe index review restore " + report.CheckpointID, "when": "Use if you want to abandon this prompt experiment."},
		},
	}
	if err := writeJSONFile(proposalJSON, proposal); err != nil {
		return nil, err
	}
	if err := os.WriteFile(proposalMarkdown, []byte(renderIndexReviewProposalMarkdown(proposal)), 0o644); err != nil {
		return nil, apperr.IOError("Failed to write prompt proposal: %s.", proposalMarkdown)
	}
	state.State = "proposal_created"
	state.UpdatedAt = now.Format(time.RFC3339)
	state.ProposalJSON = proposalJSON
	state.ProposalMarkdown = proposalMarkdown
	state.PromptPath = promptPath
	if err := saveIndexReviewState(storePath, state); err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: proposal,
		Text: renderIndexReviewResult(proposal.Status, proposal.CheckpointID, proposal.Artifacts, proposal.Next),
	}, nil
}

func runIndexReviewPatch(storePath string, cmd cli.IndexReviewPatchCommand) (*output.CommandResult, error) {
	var proposal indexReviewProposal
	if err := readJSONFile(cmd.ProposalJSON, &proposal); err != nil {
		return nil, err
	}
	state, err := loadIndexReviewState(storePath, proposal.CheckpointID)
	if err != nil {
		return nil, err
	}
	promptPath := filepath.Clean(proposal.PromptPath)
	if filepath.IsAbs(promptPath) || strings.HasPrefix(promptPath, ".."+string(os.PathSeparator)) || promptPath == ".." {
		return nil, apperr.InvalidArguments("proposal prompt path must be a relative path inside the workspace: %s", proposal.PromptPath)
	}
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		return nil, apperr.IOError("Failed to create prompt directory: %s.", filepath.Dir(promptPath))
	}
	if err := os.WriteFile(promptPath, []byte(proposal.ProposedContent), 0o644); err != nil {
		return nil, apperr.IOError("Failed to write prompt file: %s.", promptPath)
	}
	state.State = "prompt_patched"
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	state.PromptPath = promptPath
	if err := saveIndexReviewState(storePath, state); err != nil {
		return nil, err
	}
	data := map[string]any{
		"status":        "prompt_patched",
		"checkpoint_id": proposal.CheckpointID,
		"writes": map[string]bool{
			"prompt":     true,
			"metadata":   false,
			"checkpoint": true,
		},
		"artifacts": []map[string]string{{"type": "prompt_file", "path": promptPath}, {"type": "checkpoint", "id": proposal.CheckpointID}},
		"next": []map[string]string{
			{"command": "aascribe index <path> --refresh --index-plan <plan>", "when": "Re-run affected index so the new prompt can produce fresh metadata."},
			{"command": "aascribe index review <path> --method " + state.Method, "when": "Review the patched output."},
			{"command": "aascribe index review accept " + proposal.CheckpointID, "when": "Use if the patched output improved."},
			{"command": "aascribe index review restore " + proposal.CheckpointID, "when": "Use if the patched output is worse."},
		},
	}
	return &output.CommandResult{Data: data, Text: renderIndexReviewResult("prompt_patched", proposal.CheckpointID, data["artifacts"].([]map[string]string), data["next"].([]map[string]string))}, nil
}

func runIndexReviewStatus(storePath, checkpointID string) (*output.CommandResult, error) {
	state, err := loadIndexReviewState(storePath, checkpointID)
	if err != nil {
		return nil, err
	}
	data := map[string]any{
		"status":        "checkpoint_status",
		"checkpoint_id": checkpointID,
		"state":         state.State,
		"can":           indexReviewAllowedActions(state.State),
		"writes": map[string]bool{
			"prompt":     false,
			"metadata":   false,
			"checkpoint": false,
		},
		"artifacts": []map[string]string{{"type": "checkpoint", "id": checkpointID, "path": indexReviewCheckpointDir(storePath, checkpointID)}},
	}
	return &output.CommandResult{Data: data, Text: "status: checkpoint_status\ncheckpoint_id: " + checkpointID + "\nstate: " + state.State + "\ncan: " + stringListText(indexReviewAllowedActions(state.State))}, nil
}

func runIndexReviewAccept(storePath, checkpointID string) (*output.CommandResult, error) {
	state, err := loadIndexReviewState(storePath, checkpointID)
	if err != nil {
		return nil, err
	}
	state.State = "accepted"
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveIndexReviewState(storePath, state); err != nil {
		return nil, err
	}
	data := map[string]any{
		"status":        "accepted",
		"checkpoint_id": checkpointID,
		"writes":        map[string]bool{"checkpoint": true, "metadata": false, "prompt": false},
		"next":          []map[string]string{{"command": "aascribe index review status " + checkpointID, "when": "Use to inspect the closed checkpoint."}},
	}
	return &output.CommandResult{Data: data, Text: "status: accepted\ncheckpoint_id: " + checkpointID}, nil
}

func runIndexReviewRestore(storePath, checkpointID string) (*output.CommandResult, error) {
	state, err := loadIndexReviewState(storePath, checkpointID)
	if err != nil {
		return nil, err
	}
	if err := restoreIndexReviewCheckpoint(state); err != nil {
		return nil, err
	}
	state.State = "restored"
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveIndexReviewState(storePath, state); err != nil {
		return nil, err
	}
	data := map[string]any{
		"status":        "restored",
		"checkpoint_id": checkpointID,
		"writes":        map[string]bool{"checkpoint": true, "metadata": true, "prompt": true},
		"next":          []map[string]string{{"command": "aascribe index review status " + checkpointID, "when": "Use to confirm restore state."}},
	}
	return &output.CommandResult{Data: data, Text: "status: restored\ncheckpoint_id: " + checkpointID}, nil
}

func indexReviewCheckpointDir(storePath, checkpointID string) string {
	return filepath.Join(storePath, "index-review", "checkpoints", checkpointID)
}

func indexReviewStatePath(storePath, checkpointID string) string {
	return filepath.Join(indexReviewCheckpointDir(storePath, checkpointID), "state.json")
}

func saveIndexReviewState(storePath string, state indexReviewState) error {
	path := indexReviewStatePath(storePath, state.CheckpointID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperr.IOError("Failed to create review checkpoint directory: %s.", filepath.Dir(path))
	}
	return writeJSONFile(path, state)
}

func loadIndexReviewState(storePath, checkpointID string) (indexReviewState, error) {
	var state indexReviewState
	path := indexReviewStatePath(storePath, checkpointID)
	if err := readJSONFile(path, &state); err != nil {
		return state, err
	}
	return state, nil
}

func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return apperr.Serialization("Failed to serialize JSON artifact.")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperr.IOError("Failed to create artifact directory: %s.", filepath.Dir(path))
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return apperr.IOError("Failed to write JSON artifact: %s.", path)
	}
	return nil
}

func readJSONFile(path string, target any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return apperr.IOError("Failed to read JSON artifact: %s.", path)
	}
	if err := json.Unmarshal(content, target); err != nil {
		return apperr.Serialization("Failed to parse JSON artifact: %s.", path)
	}
	return nil
}

func snapshotIndexReviewMetadata(root, checkpointDir string) ([]string, error) {
	backups := []string{}
	metadataRoot := filepath.Join(checkpointDir, "metadata")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Base(path) != ".aascribe_index_meta.json" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(metadataRoot, rel)
		if err := copyFileForReview(path, target); err != nil {
			return err
		}
		backups = append(backups, target+"=>"+path)
		return nil
	})
	if err != nil {
		return nil, apperr.IOError("Failed to snapshot index metadata under %s.", root)
	}
	return backups, nil
}

func snapshotIndexReviewPrompts(checkpointDir string) ([]string, error) {
	backups := []string{}
	for _, path := range indexReviewPromptPaths() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		target := filepath.Join(checkpointDir, "prompts", path)
		if err := copyFileForReview(path, target); err != nil {
			return backups, err
		}
		backups = append(backups, target+"=>"+path)
	}
	return backups, nil
}

func restoreIndexReviewCheckpoint(state indexReviewState) error {
	promptTargets := map[string]bool{}
	for _, promptBackup := range state.PromptBackups {
		_, promptTarget, promptOK := strings.Cut(promptBackup, "=>")
		if promptOK {
			promptTargets[promptTarget] = true
		}
	}
	for _, mapping := range append(append([]string{}, state.MetadataBackups...), state.PromptBackups...) {
		source, target, ok := strings.Cut(mapping, "=>")
		if !ok {
			continue
		}
		if err := copyFileForReview(source, target); err != nil {
			return err
		}
	}
	for _, promptPath := range indexReviewPromptPaths() {
		if promptTargets[promptPath] {
			continue
		}
		if err := os.Remove(promptPath); err != nil && !os.IsNotExist(err) {
			return apperr.IOError("Failed to remove prompt created after checkpoint: %s.", promptPath)
		}
	}
	return nil
}

func indexReviewPromptPaths() []string {
	return []string{
		filepath.Join("prompts", "retrieval", "generate.v1.md"),
		filepath.Join("prompts", "retrieval", "review.v1.md"),
		filepath.Join("prompts", "retrieval", "patch.v1.md"),
	}
}

func copyFileForReview(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return apperr.IOError("Failed to read checkpoint source: %s.", source)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return apperr.IOError("Failed to create checkpoint target directory: %s.", filepath.Dir(target))
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return apperr.IOError("Failed to write checkpoint target: %s.", target)
	}
	return nil
}

func inspectRetrievalMetadata(root, method string) ([]indexReviewFinding, error) {
	findings := []indexReviewFinding{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Base(path) != ".aascribe_index_meta.json" {
			return nil
		}
		var meta struct {
			Files []struct {
				Path    string `json:"path"`
				Methods map[string]struct {
					Fields map[string]any `json:"fields"`
				} `json:"methods"`
			} `json:"files"`
		}
		if err := readJSONFile(path, &meta); err != nil {
			return err
		}
		for _, file := range meta.Files {
			output, ok := file.Methods[method]
			if !ok {
				continue
			}
			fieldsText := fmt.Sprint(output.Fields)
			for _, label := range []string{"Example", "Note", "Warning", "Tip"} {
				if strings.Contains(fieldsText, "from:"+label) || strings.Contains(fieldsText, "name:"+label) || strings.Contains(fieldsText, "\""+label+"\"") {
					findings = append(findings, indexReviewFinding{
						Path:       file.Path,
						Severity:   "high",
						Kind:       "label_as_entity",
						Message:    label + " appears to be treated as a retrieval entity or alias source.",
						Suggestion: "Prompt should treat Markdown labels such as Example, Note, Warning, and Tip as context labels, not entities.",
					})
				}
			}
			if strings.Contains(fieldsText, "](") {
				findings = append(findings, indexReviewFinding{
					Path:       file.Path,
					Severity:   "medium",
					Kind:       "markdown_leakage",
					Message:    "Raw Markdown link syntax appears in retrieval metadata.",
					Suggestion: "Prompt should emit clean link text or omit navigation links.",
				})
			}
			if looksLikeReviewCodeLeak(fieldsText) {
				findings = append(findings, indexReviewFinding{
					Path:       file.Path,
					Severity:   "medium",
					Kind:       "code_snippet_leakage",
					Message:    "Command-like code snippets appear in retrieval metadata.",
					Suggestion: "Prompt should avoid emitting examples, shell commands, and code snippets as retrieval terms.",
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(findings) == 0 {
		findings = append(findings, indexReviewFinding{
			Path:       root,
			Severity:   "info",
			Kind:       "no_obvious_issues",
			Message:    "No obvious deterministic retrieval metadata issues were detected.",
			Suggestion: "If semantic misses remain, run an LLM review prompt or add eval cases.",
		})
	}
	return findings, nil
}

func looksLikeReviewCodeLeak(value string) bool {
	lower := strings.ToLower(value)
	for _, needle := range []string{" fd ", " rg ", " grep ", " curl ", " xh ", " bat file", "*.txt", "```"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func renderIndexReviewMarkdown(report indexReviewReport) string {
	lines := []string{
		"# Retrieval Review Report",
		"",
		"- status: " + report.Status,
		"- checkpoint_id: " + report.CheckpointID,
		"- method: " + report.Method,
		"- reviewed_path: " + report.ReviewedPath,
		"",
		"## Findings",
	}
	for _, finding := range report.Findings {
		lines = append(lines, "", "- "+finding.Severity+" "+finding.Kind+": "+finding.Path, "  - "+finding.Message)
		if finding.Suggestion != "" {
			lines = append(lines, "  - suggestion: "+finding.Suggestion)
		}
	}
	lines = append(lines, "", "## Next", "")
	for _, next := range report.Next {
		lines = append(lines, "- `"+next["command"]+"` — "+next["when"])
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderIndexReviewProposalMarkdown(proposal indexReviewProposal) string {
	return strings.Join([]string{
		"# Retrieval Prompt Proposal",
		"",
		"- status: " + proposal.Status,
		"- checkpoint_id: " + proposal.CheckpointID,
		"- prompt_path: " + proposal.PromptPath,
		"",
		"## Proposed Prompt",
		"",
		"```markdown",
		proposal.ProposedContent,
		"```",
	}, "\n") + "\n"
}

func defaultRetrievalPromptProposal(findings []indexReviewFinding) string {
	return strings.TrimSpace(`# Retrieval Metadata Generation Prompt v1

You generate retrieval metadata for an AI agent. Return strict JSON only.

Rules:
- Preserve important source terms, names, and roles.
- Add useful aliases and multilingual terms when they help future LLM retrieval.
- Treat Markdown labels such as Example, Note, Warning, Tip, and Usage as context labels, not entities.
- Do not emit shell commands, code examples, Markdown link syntax, or long snippets as retrieval terms.
- Prefer a small number of natural likely_queries over many formulaic templates.
- aliases[].from must refer to a real entity name unless the alias explicitly expands an acronym.

Required JSON fields:
- entities: array of {name,type,role,source_quote}
- aliases: array of {from,to,reason}
- retrieval_terms: array of strings
- likely_queries: array of natural-language user intents
- warnings: array of strings
`) + "\n"
}

func renderIndexReviewResult(status, checkpointID string, artifacts []map[string]string, next []map[string]string) string {
	lines := []string{"status: " + status, "checkpoint_id: " + checkpointID, "artifacts:"}
	for _, artifact := range artifacts {
		label := artifact["type"]
		value := artifact["path"]
		if value == "" {
			value = artifact["id"]
		}
		lines = append(lines, "- "+label+": "+value)
	}
	lines = append(lines, "next:")
	for _, item := range next {
		lines = append(lines, "- "+item["command"])
	}
	return strings.Join(lines, "\n")
}

func indexReviewAllowedActions(state string) []string {
	switch state {
	case "review_created":
		return []string{"propose", "accept", "restore"}
	case "proposal_created":
		return []string{"patch", "restore"}
	case "prompt_patched":
		return []string{"accept", "restore"}
	case "accepted", "restored":
		return []string{"status"}
	default:
		return []string{"status", "restore"}
	}
}

func runIndexAsync(storePath string, cmd cli.IndexCommand) (*output.CommandResult, error) {
	var err error
	cmd, err = resolveIndexPlan(cmd)
	if err != nil {
		return nil, err
	}
	accepted, err := operation.Create(storePath, operation.CreateInput{
		Command: "index",
		Stage:   "queued",
		Message: "Index operation accepted.",
		State:   operation.StatePending,
	})
	if err != nil {
		return nil, err
	}
	if cmd.IndexPlan != "" {
		snapshotPath, err := operation.SnapshotFile(storePath, accepted.OperationID, cmd.IndexPlan, "index-plan.toml")
		if err != nil {
			return nil, err
		}
		cmd.IndexPlan = snapshotPath
	}
	if err := startAsyncIndexProcess(storePath, accepted.OperationID, cmd); err != nil {
		status, loadErr := operation.LoadStatus(storePath, accepted.OperationID)
		if loadErr == nil {
			now := time.Now().UTC().Format(time.RFC3339)
			status.State = operation.StateFailed
			status.Stage = "failed"
			status.Message = "Failed to start async index worker."
			status.UpdatedAt = now
			status.CompletedAt = now
			status.ResultReady = true
			status.Error = &operation.ErrorDetail{Code: output.ErrorCode(err), Message: err.Error()}
			_ = operation.SaveStatus(storePath, status)
			_ = operation.SaveResult(storePath, &operation.Result{
				OperationID: accepted.OperationID,
				State:       operation.StateFailed,
				CompletedAt: now,
				Truncated:   false,
				Error:       status.Error,
			})
		}
		return nil, err
	}
	return &output.CommandResult{
		Data: accepted,
		Text: renderOperationAcceptedText(accepted),
	}, nil
}

func startIndexWorkerProcess(storePath, operationID string, cmd cli.IndexCommand) error {
	exe, err := os.Executable()
	if err != nil {
		return apperr.IOError("Failed to resolve current executable for async index.")
	}
	args := []string{"--store", storePath, "operation", "run-index", operationID}
	args = append(args, indexArgs(cmd)...)
	worker := exec.Command(exe, args...)
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return apperr.IOError("Failed to open null device for async index worker.")
	}
	worker.Stdin = devNull
	worker.Stdout = devNull
	worker.Stderr = devNull
	if err := worker.Start(); err != nil {
		_ = devNull.Close()
		return apperr.IOError("Failed to start async index worker: %s.", err.Error())
	}
	_ = devNull.Close()
	return nil
}

func indexArgs(cmd cli.IndexCommand) []string {
	args := []string{
		"--depth", fmt.Sprintf("%d", cmd.Depth),
		"--concurrency", fmt.Sprintf("%d", cmd.Concurrency),
		"--max-file-size", fmt.Sprintf("%d", cmd.MaxFileSize),
	}
	for _, pattern := range cmd.Include {
		args = append(args, "--include", pattern)
	}
	for _, pattern := range cmd.Exclude {
		args = append(args, "--exclude", pattern)
	}
	if cmd.Refresh {
		args = append(args, "--refresh")
	}
	if cmd.NoSummary {
		args = append(args, "--no-summary")
	}
	if cmd.IndexPlan != "" {
		args = append(args, "--index-plan", cmd.IndexPlan)
	} else {
		for _, method := range cmd.MetadataMethods {
			args = append(args, "--metadata-method", method)
		}
		if cmd.NoMetadataMethods {
			args = append(args, "--no-metadata-methods")
		}
	}
	args = append(args, cmd.Path)
	return args
}

func runOperationRunIndex(storePath, operationID string, cmd cli.IndexCommand) (*output.CommandResult, error) {
	var err error
	cmd, err = resolveIndexPlan(cmd)
	if err != nil {
		return nil, err
	}
	reporter := operation.NewReporter(storePath, operationID)
	status, err := operation.LoadStatus(storePath, operationID)
	if err != nil {
		return nil, err
	}
	if status.State == operation.StateCanceled {
		return &output.CommandResult{Data: status, Text: renderOperationStatusText(status)}, nil
	}
	status.State = operation.StateRunning
	status.Stage = "indexing"
	status.Message = "Indexing path."
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := operation.SaveStatus(storePath, status); err != nil {
		return nil, err
	}
	_ = reporter.Report(operation.Report{
		Stage:   "indexing",
		Message: "Indexing path.",
		Data: map[string]any{
			"path": cmd.Path,
		},
	})

	ctx, stopCancelWatch := operation.ContextWithCancelWatch(context.Background(), storePath, operationID)
	defer stopCancelWatch()

	usage := &tokenUsageTracker{}
	noSummary := cmd.EffectiveNoSummary()
	var summarizer index.SummarizerFunc
	var summarizerErr error
	if !noSummary {
		summarizer, summarizerErr = buildRequiredLLMSummarizer(storePath, usage)
	}
	var result *index.PathIndexTree
	var buildErr error
	if summarizerErr != nil {
		buildErr = summarizerErr
	} else {
		result, buildErr = index.Build(index.Options{
			Context:             ctx,
			Root:                cmd.Path,
			Depth:               cmd.Depth,
			Include:             cmd.Include,
			Exclude:             cmd.Exclude,
			MaxConcurrency:      cmd.Concurrency,
			Refresh:             cmd.Refresh,
			NoSummary:           noSummary,
			MaxFileSize:         cmd.MaxFileSize,
			MetadataMethods:     cmd.EffectiveMetadataMethods(),
			Summarizer:          summarizer,
			FailureNoticeWriter: io.Discard,
		})
	}

	current, loadErr := operation.LoadStatus(storePath, operationID)
	if loadErr != nil {
		return nil, loadErr
	}
	if current.State == operation.StateCanceled {
		return &output.CommandResult{Data: current, Text: renderOperationStatusText(current)}, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if buildErr != nil {
		current.State = operation.StateFailed
		current.Stage = "failed"
		current.Message = buildErr.Error()
		current.UpdatedAt = now
		current.CompletedAt = now
		current.ResultReady = true
		current.Error = &operation.ErrorDetail{Code: output.ErrorCode(buildErr), Message: buildErr.Error()}
		if saveErr := operation.SaveStatus(storePath, current); saveErr != nil {
			return nil, saveErr
		}
		if saveErr := operation.SaveResult(storePath, &operation.Result{
			OperationID: operationID,
			State:       operation.StateFailed,
			CompletedAt: now,
			Truncated:   false,
			Error:       current.Error,
		}); saveErr != nil {
			return nil, saveErr
		}
		_ = reporter.Report(operation.Report{Stage: "failed", Message: buildErr.Error(), Level: "error"})
		return nil, buildErr
	}

	reportPath, err := writeIndexCompletionReport(storePath, cmd, result, operationID, "async", usage.snapshot())
	if err != nil {
		current.State = operation.StateFailed
		current.Stage = "failed"
		current.Message = err.Error()
		current.UpdatedAt = now
		current.CompletedAt = now
		current.ResultReady = true
		current.Error = &operation.ErrorDetail{Code: output.ErrorCode(err), Message: err.Error()}
		if saveErr := operation.SaveStatus(storePath, current); saveErr != nil {
			return nil, saveErr
		}
		if saveErr := operation.SaveResult(storePath, &operation.Result{
			OperationID: operationID,
			State:       operation.StateFailed,
			CompletedAt: now,
			Truncated:   false,
			Error:       current.Error,
		}); saveErr != nil {
			return nil, saveErr
		}
		_ = reporter.Report(operation.Report{Stage: "failed", Message: err.Error(), Level: "error"})
		return nil, err
	}
	result.ReportPath = reportPath

	current.State = operation.StateSucceeded
	current.Stage = "complete"
	current.Message = "Index operation completed."
	current.UpdatedAt = now
	current.CompletedAt = now
	current.ResultReady = true
	current.Error = nil
	if err := operation.SaveStatus(storePath, current); err != nil {
		return nil, err
	}
	if err := operation.SaveResult(storePath, &operation.Result{
		OperationID: operationID,
		State:       operation.StateSucceeded,
		CompletedAt: now,
		Data:        result,
		Truncated:   false,
	}); err != nil {
		return nil, err
	}
	_ = reporter.Report(operation.Report{Stage: "complete", Message: "Index operation completed."})
	return &output.CommandResult{Data: result, Text: renderIndexText(result)}, nil
}

func runIndexClean(cmd cli.IndexCleanCommand) (*output.CommandResult, error) {
	result, err := index.CleanArtifacts(cmd.Path, cmd.DryRun)
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf("Removed %d index artifact(s) under %s", result.RemovedCount, result.Root)
	if result.DryRun {
		text = fmt.Sprintf("Would remove %d index artifact(s) under %s", result.RemovedCount, result.Root)
	}
	text += "\nnext: aascribe index " + result.Root

	return &output.CommandResult{
		Data: result,
		Text: text,
	}, nil
}

func runIndexDirty(cmd cli.IndexDirtyCommand) (*output.CommandResult, error) {
	result, err := index.MarkDirty(cmd.Path)
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf("No index metadata files found under %s", result.Root)
	if result.MarkedCount > 0 {
		text = fmt.Sprintf("Marked %d index metadata file(s) dirty under %s", result.MarkedCount, result.Root)
	}
	text += "\nnext: aascribe index " + result.Root
	return &output.CommandResult{
		Data: result,
		Text: text,
	}, nil
}

func runIndexEval(cmd cli.IndexEvalCommand) (*output.CommandResult, error) {
	result, err := index.Eval(cmd.Path)
	if err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: result,
		Text: renderEvalText(result),
	}, nil
}

func runIndexMap(cmd cli.IndexMapCommand) (*output.CommandResult, error) {
	result, err := index.BuildMap(cmd.Path)
	if err != nil {
		return nil, err
	}

	return &output.CommandResult{
		Data: result,
		Text: renderMapText(result),
	}, nil
}

func runMap(cmd cli.MapCommand) (*output.CommandResult, error) {
	result, err := index.BuildMap(cmd.Path)
	if err != nil {
		return nil, err
	}

	return &output.CommandResult{
		Data: result,
		Text: renderMapText(result),
	}, nil
}

func runSearch(cmd cli.SearchCommand) (*output.CommandResult, error) {
	result, err := search.Run(search.Options{
		Query:        cmd.Query,
		Root:         cmd.Path,
		Engine:       cmd.Engine,
		IgnoreCase:   cmd.IgnoreCase,
		FixedStrings: cmd.FixedStrings,
		Glob:         cmd.Glob,
		MaxCount:     cmd.MaxCount,
	})
	if err != nil {
		return nil, apperr.IOError("Search failed: %v.", err)
	}
	return &output.CommandResult{
		Data: result,
		Text: renderSearchText(result),
	}, nil
}

func runDescribe(storePath string, cmd cli.DescribeCommand) (*output.CommandResult, error) {
	result, err := describeWithFallback(storePath, cmd)
	if err != nil {
		return nil, err
	}

	return &output.CommandResult{
		Data: result,
		Text: result.Summary,
	}, nil
}

func describeWithFallback(storePath string, cmd cli.DescribeCommand) (*index.FileDescription, error) {
	return index.DescribeFileWithSummarizer(cmd.File, cmd.Length, cmd.Focus, buildLLMSummarizer(storePath, nil))
}

func runLogsPath(storePath string) *output.CommandResult {
	logPath := logging.ActiveLogPath(storePath)
	return &output.CommandResult{
		Data: map[string]any{
			"path": logPath,
		},
		Text: strings.Join([]string{
			logPath,
			"next: aascribe logs export --output ./aascribe.log",
		}, "\n"),
	}
}

func runLogsExport(storePath, outputPath string) (*output.CommandResult, error) {
	sourcePath := logging.ActiveLogPath(storePath)
	if _, err := os.Stat(sourcePath); err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.LogFileNotFound(sourcePath)
		}
		return nil, apperr.IOError("Failed to inspect log file: %s.", sourcePath)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, apperr.IOError("Failed to create export directory: %s.", filepath.Dir(outputPath))
	}
	if err := copyFile(sourcePath, outputPath); err != nil {
		return nil, err
	}

	return &output.CommandResult{
		Data: map[string]any{
			"source_path": sourcePath,
			"output_path": outputPath,
		},
		Text: fmt.Sprintf("Exported log file to %s\nnext: inspect %s", outputPath, outputPath),
	}, nil
}

func runLogsClear(storePath string, force bool) (*output.CommandResult, error) {
	if !force {
		return nil, apperr.InvalidArguments("logs clear requires --force.")
	}
	logPath := logging.ActiveLogPath(storePath)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, apperr.IOError("Failed to create log directory: %s.", filepath.Dir(logPath))
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, apperr.IOError("Failed to clear log file: %s.", logPath)
	}
	if err := file.Close(); err != nil {
		return nil, apperr.IOError("Failed to close cleared log file: %s.", logPath)
	}

	return &output.CommandResult{
		Data: map[string]any{
			"path":    logPath,
			"cleared": true,
		},
		Text: fmt.Sprintf("Cleared log file at %s\nnext: aascribe logs path", logPath),
	}, nil
}

func runInit(storePath string, force bool) (*output.CommandResult, error) {
	outcome, err := store.InitializeStore(storePath, force)
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf("Initialized aascribe store at %s", storePath)
	if outcome.Reinitialized {
		text = fmt.Sprintf("Reinitialized aascribe store at %s", storePath)
	}
	text += "\nconfig: " + outcome.ConfigPath
	if outcome.ConfigCreated {
		text += "\nnext: set GEMINI_API_KEY before LLM-backed commands, or edit " + outcome.ConfigPath
	} else {
		text += "\nnext: aascribe logs path"
	}

	return &output.CommandResult{
		Data: map[string]any{
			"store":          storePath,
			"created":        outcome.Created,
			"reinitialized":  outcome.Reinitialized,
			"layout_version": store.LayoutVersion(),
			"config_path":    outcome.ConfigPath,
			"config_created": outcome.ConfigCreated,
		},
		Text: text,
	}, nil
}

func runConfigSet(storePath string, cmd cli.ConfigSetCommand) (*output.CommandResult, error) {
	if _, err := store.InitializeStore(storePath, false); err != nil {
		if appErr, ok := err.(*apperr.Error); !ok || appErr.Code != "STORE_ALREADY_EXISTS" {
			return nil, err
		}
	}
	envPath := filepath.Join(config.ConfigDir(storePath), ".env")
	envName := ""
	switch cmd.Key {
	case "gemini_api_key":
		envName = "GEMINI_API_KEY"
	default:
		return nil, apperr.InvalidArguments("Unsupported config key: %s.", cmd.Key)
	}
	if err := upsertDotEnvValue(envPath, envName, cmd.Value); err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: map[string]any{
			"config_path": config.ConfigPath(storePath),
			"env_path":    envPath,
			"key":         cmd.Key,
			"env_name":    envName,
			"updated":     true,
		},
		Text: fmt.Sprintf("Updated %s in %s\nnext: aascribe index <path>", cmd.Key, envPath),
	}, nil
}

func upsertDotEnvValue(path, key, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperr.IOError("Failed to create config directory: %s.", filepath.Dir(path))
	}
	lines := []string{}
	if content, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	} else if !os.IsNotExist(err) {
		return apperr.IOError("Failed to read config env file: %s.", path)
	}
	replacement := key + "=" + value
	updated := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+"=") {
			lines[i] = replacement
			updated = true
		}
	}
	if !updated {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines[len(lines)-1] = replacement
		} else {
			lines = append(lines, replacement)
		}
	}
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return apperr.IOError("Failed to write config env file: %s.", path)
	}
	return nil
}

func runOperationList(storePath string) (*output.CommandResult, error) {
	result, err := operation.ListOperations(storePath)
	if err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: result,
		Text: renderOperationListText(result),
	}, nil
}

func runOperationStatus(storePath, id string) (*output.CommandResult, error) {
	result, err := operation.LoadStatus(storePath, id)
	if err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: result,
		Text: renderOperationStatusText(result),
	}, nil
}

func runOperationEvents(storePath, id string) (*output.CommandResult, error) {
	result, err := operation.LoadEvents(storePath, id)
	if err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: result,
		Text: renderOperationEventsText(result),
	}, nil
}

func runOperationResult(storePath, id string) (*output.CommandResult, error) {
	result, err := operation.LoadResult(storePath, id)
	if err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: result,
		Text: renderOperationResultText(result),
	}, nil
}

func runOperationCancel(storePath, id string) (*output.CommandResult, error) {
	result, err := operation.Cancel(storePath, id)
	if err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: result,
		Text: fmt.Sprintf("%s: %s", result.OperationID, result.Message),
	}, nil
}

func runOperationClean(storePath string, cmd cli.OperationCleanCommand) (*output.CommandResult, error) {
	result, err := operation.Clean(storePath, operation.CleanOptions{
		DryRun: cmd.DryRun,
		Force:  cmd.Force,
	})
	if err != nil {
		return nil, err
	}
	return &output.CommandResult{
		Data: result,
		Text: renderOperationCleanText(result),
	}, nil
}

func renderMapText(result *index.PathIndexMap) string {
	if result == nil {
		return ""
	}
	var lines []string
	lines = append(lines, "usage: map is a routing overview; for precise answers inspect the target file directly or re-run index without --no-summary.")
	if len(result.StateGuide) > 0 {
		lines = append(lines, "state guide:")
		keys := make([]string, 0, len(result.StateGuide))
		for key := range result.StateGuide {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, "  "+key+": "+result.StateGuide[key])
		}
	}
	var walk func(node index.IndexMapNode, depth int)
	walk = func(node index.IndexMapNode, depth int) {
		indent := strings.Repeat("  ", depth)
		label := filepath.Base(node.Path)
		if depth == 0 {
			label = filepath.Clean(node.Path)
		}
		line := indent + label
		if node.State != "" && node.State != "ready" {
			line += " [" + node.State + "]"
		}
		lines = append(lines, line)
		if node.BriefSummary != "" {
			lines = append(lines, indent+"  "+"summary: "+node.BriefSummary)
		}
		if node.State == "unindexed" {
			return
		}
		for _, file := range node.Files {
			fileLine := indent + "  " + filepath.Base(file.Path)
			if file.Status != "" && file.Status != "ok" {
				fileLine += " [" + file.Status + "]"
			}
			detail := file.Summary
			if detail == "" {
				detail = file.FileType
			}
			if detail != "" {
				fileLine += " - " + detail
			}
			lines = append(lines, fileLine)
		}
		for _, child := range node.Children {
			walk(child, depth+1)
		}
	}
	walk(result.Map, 0)
	return strings.Join(lines, "\n")
}

func renderSearchText(result *search.Result) string {
	lines := []string{
		fmt.Sprintf("search: %q", result.Query),
		"engine: " + result.Engine,
		fmt.Sprintf("matches: %d", result.MatchCnt),
	}
	for _, match := range result.Matches {
		location := fmt.Sprintf("%s:%d", match.Path, match.Line)
		if match.Column > 0 {
			location = fmt.Sprintf("%s:%d:%d", match.Path, match.Line, match.Column)
		}
		lines = append(lines, location+": "+match.Text)
	}
	if result.Truncated {
		lines = append(lines, "truncated: true")
		lines = append(lines, "next: narrow the path, add --glob, or raise --max-count")
	} else if result.MatchCnt == 0 {
		lines = append(lines, "next: try --ignore-case, broaden the path, or use aascribe map for semantic routing")
	} else {
		lines = append(lines, "next: inspect the matched files before making final claims")
	}
	return strings.Join(lines, "\n")
}

func renderOperationListText(result *operation.List) string {
	if result == nil || len(result.Items) == 0 {
		return "No stored operations.\nnext: aascribe index --async <path>"
	}
	lines := []string{fmt.Sprintf("operations: %d", result.Count)}
	for _, item := range result.Items {
		lines = append(lines, fmt.Sprintf("- %s  %s  %s  %s", item.OperationID, item.Command, item.State, item.Stage))
	}
	lines = append(lines, "next: aascribe operation status <operation-id>")
	return strings.Join(lines, "\n")
}

func renderOperationAcceptedText(result *operation.Accepted) string {
	if result == nil {
		return ""
	}
	return strings.Join([]string{
		fmt.Sprintf("operation: %s", result.OperationID),
		fmt.Sprintf("command: %s", result.Command),
		fmt.Sprintf("state: %s", result.State),
		"next:",
		"  " + result.StatusHint,
		"  " + result.ResultHint,
		"  " + result.CancelHint,
	}, "\n")
}

func renderOperationStatusText(result *operation.Status) string {
	if result == nil {
		return ""
	}
	lines := []string{
		fmt.Sprintf("operation: %s", result.OperationID),
		fmt.Sprintf("command: %s", result.Command),
		fmt.Sprintf("state: %s", result.State),
		fmt.Sprintf("stage: %s", result.Stage),
		fmt.Sprintf("message: %s", result.Message),
		fmt.Sprintf("result_ready: %t", result.ResultReady),
	}
	if result.Progress != nil {
		lines = append(lines, fmt.Sprintf("progress: %d/%d %s (%.1f%%)", result.Progress.Current, result.Progress.Total, result.Progress.Unit, result.Progress.Percent))
	}
	if result.CompletedAt != "" {
		lines = append(lines, fmt.Sprintf("completed_at: %s", result.CompletedAt))
	}
	if result.Error != nil {
		lines = append(lines, fmt.Sprintf("error: %s - %s", result.Error.Code, result.Error.Message))
	}
	switch {
	case result.ResultReady:
		lines = append(lines, "next: aascribe operation result "+result.OperationID)
	case result.State == operation.StateRunning || result.State == operation.StatePending:
		lines = append(lines, "next: aascribe operation events "+result.OperationID)
		lines = append(lines, "cancel: aascribe operation cancel "+result.OperationID)
	}
	return strings.Join(lines, "\n")
}

func renderOperationEventsText(result *operation.EventList) string {
	if result == nil {
		return ""
	}
	if len(result.Events) == 0 {
		return fmt.Sprintf("operation: %s\nNo recorded events.\nnext: aascribe operation status %s", result.OperationID, result.OperationID)
	}
	lines := []string{fmt.Sprintf("operation: %s", result.OperationID)}
	for _, event := range result.Events {
		lines = append(lines, fmt.Sprintf("- %s [%s] %s: %s", event.Time, event.Level, event.Stage, event.Message))
	}
	lines = append(lines, "next: aascribe operation status "+result.OperationID)
	return strings.Join(lines, "\n")
}

func renderOperationResultText(result *operation.Result) string {
	if result == nil {
		return ""
	}
	lines := []string{
		fmt.Sprintf("operation: %s", result.OperationID),
		fmt.Sprintf("state: %s", result.State),
		fmt.Sprintf("completed_at: %s", result.CompletedAt),
		fmt.Sprintf("truncated: %t", result.Truncated),
	}
	if result.OutputID != "" {
		lines = append(lines, fmt.Sprintf("output_id: %s", result.OutputID))
		lines = append(lines, "data: stored_output")
		lines = append(lines, "next: aascribe output show "+result.OutputID)
		lines = append(lines, "slice: aascribe output slice "+result.OutputID+" --offset 0 --limit 4000")
	}
	if result.Error != nil {
		lines = append(lines, fmt.Sprintf("error: %s - %s", result.Error.Code, result.Error.Message))
	}
	if result.Data != nil {
		lines = append(lines, "data: available")
		lines = append(lines, "next: aascribe --format json operation result "+result.OperationID)
	}
	return strings.Join(lines, "\n")
}

func renderOperationCleanText(result *operation.CleanResult) string {
	if result == nil {
		return ""
	}
	action := "Removed"
	next := "next: aascribe operation list"
	if result.DryRun {
		action = "Would remove"
		next = "next: aascribe operation clean --force"
	}
	lines := []string{
		fmt.Sprintf("%s %d terminal operation(s)", action, result.RemovedCount),
		fmt.Sprintf("skipped %d active operation(s)", result.SkippedCount),
	}
	for _, item := range result.Removed {
		line := fmt.Sprintf("- %s [%s]", item.OperationID, item.State)
		if result.DryRun {
			line += " would_remove"
		} else {
			line += " removed"
		}
		lines = append(lines, line)
	}
	for _, item := range result.Skipped {
		reason := item.Reason
		if reason == "" {
			reason = "skipped"
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] skipped: %s", item.OperationID, item.State, reason))
	}
	lines = append(lines, next)
	return strings.Join(lines, "\n")
}

func renderEvalText(result *index.EvalResult) string {
	if result == nil {
		return ""
	}

	lines := []string{"index eval: " + result.Root}
	needsFolders, unchangedFolders := partitionEvalFolders(result.Folders)
	needsFiles, unchangedFiles := partitionEvalFiles(result.Files)

	lines = append(lines, fmt.Sprintf("folders needing index: %d", len(needsFolders)))
	for _, item := range needsFolders {
		lines = append(lines, "  "+item.Path+" ["+item.Reason+"]")
	}
	lines = append(lines, fmt.Sprintf("unchanged folders: %d", len(unchangedFolders)))
	for _, item := range unchangedFolders {
		lines = append(lines, "  "+item.Path)
	}

	lines = append(lines, fmt.Sprintf("files needing index: %d", len(needsFiles)))
	for _, item := range needsFiles {
		lines = append(lines, "  "+item.Path+" ["+item.Reason+"]")
	}
	lines = append(lines, fmt.Sprintf("unchanged files: %d", len(unchangedFiles)))
	for _, item := range unchangedFiles {
		lines = append(lines, "  "+item.Path)
	}
	if len(needsFolders) > 0 || len(needsFiles) > 0 {
		lines = append(lines, "next: aascribe index "+result.Root)
	} else {
		lines = append(lines, "next: aascribe map "+result.Root)
	}

	return strings.Join(lines, "\n")
}

func partitionEvalFolders(items []index.EvalFolder) ([]index.EvalFolder, []index.EvalFolder) {
	var needs []index.EvalFolder
	var unchanged []index.EvalFolder
	for _, item := range items {
		if item.State == "needs_index" {
			needs = append(needs, item)
			continue
		}
		unchanged = append(unchanged, item)
	}
	return needs, unchanged
}

func partitionEvalFiles(items []index.EvalFile) ([]index.EvalFile, []index.EvalFile) {
	var needs []index.EvalFile
	var unchanged []index.EvalFile
	for _, item := range items {
		if item.State == "needs_index" {
			needs = append(needs, item)
			continue
		}
		unchanged = append(unchanged, item)
	}
	return needs, unchanged
}

func runChat(storePath, prompt string) (*output.CommandResult, error) {
	resolved, err := config.Resolve(storePath, config.ResolveOptions{}, nil)
	if err != nil {
		return nil, err
	}

	response, err := runPrompt(resolved, prompt)
	if err != nil {
		return nil, err
	}

	return &output.CommandResult{
		Data: map[string]any{
			"provider":      response.Provider,
			"model":         response.Model,
			"text":          response.Text,
			"finish_reason": response.FinishReason,
			"usage": map[string]any{
				"prompt_token_count":     response.Usage.PromptTokenCount,
				"candidates_token_count": response.Usage.CandidatesTokenCount,
				"total_token_count":      response.Usage.TotalTokenCount,
			},
		},
		Text:           response.Text,
		PrimaryTextKey: "text",
	}, nil
}

func runSummarize(storePath, sourcePath string) (*output.CommandResult, error) {
	resolved, err := config.Resolve(storePath, config.ResolveOptions{}, nil)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, apperr.IOError("Failed to read summary source file: %s.", sourcePath)
	}

	response, err := runPrompt(resolved, buildSummaryPrompt(sourcePath, string(content)))
	if err != nil {
		return nil, err
	}

	return &output.CommandResult{
		Data: map[string]any{
			"provider":      response.Provider,
			"model":         response.Model,
			"path":          sourcePath,
			"summary":       response.Text,
			"finish_reason": response.FinishReason,
			"usage": map[string]any{
				"prompt_token_count":     response.Usage.PromptTokenCount,
				"candidates_token_count": response.Usage.CandidatesTokenCount,
				"total_token_count":      response.Usage.TotalTokenCount,
			},
		},
		Text:           response.Text,
		PrimaryTextKey: "summary",
	}, nil
}

func runOutputList(storePath string) (*output.CommandResult, error) {
	items, err := largeoutput.List(storePath)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(items))
	if len(items) == 0 {
		lines = append(lines, "No stored outputs.")
		lines = append(lines, "next: run a command that may produce large output, then retry aascribe output list")
	}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s %s %dB", item.ID, item.Command, item.TotalBytes))
	}
	if len(items) > 0 {
		lines = append(lines, "next: aascribe output show <output-id>")
	}
	return &output.CommandResult{
		Data: map[string]any{
			"outputs": items,
			"count":   len(items),
		},
		Text: strings.Join(lines, "\n"),
	}, nil
}

func runOutputGenerate(cmd cli.OutputGenerateCommand) (*output.CommandResult, error) {
	lines := make([]string, 0, cmd.Lines)
	bodyWidth := cmd.Width
	if bodyWidth < 16 {
		bodyWidth = 16
	}
	body := strings.Repeat("x", bodyWidth)
	for i := 1; i <= cmd.Lines; i++ {
		lines = append(lines, fmt.Sprintf("%s %04d %s", cmd.Prefix, i, body))
	}
	text := strings.Join(lines, "\n")
	return &output.CommandResult{
		Data: map[string]any{
			"text":   text,
			"lines":  cmd.Lines,
			"width":  cmd.Width,
			"prefix": cmd.Prefix,
		},
		Text:           text,
		PrimaryTextKey: "text",
	}, nil
}

func writeIndexCompletionReport(storePath string, cmd cli.IndexCommand, result *index.PathIndexTree, operationID, mode string, usage tokenUsageSnapshot) (string, error) {
	if result == nil || result.Run == nil {
		return "", apperr.IOError("Failed to build index report: missing run statistics.")
	}
	reportDir := filepath.Join(filepath.Dir(storePath), "report")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return "", apperr.IOError("Failed to create index report directory: %s.", reportDir)
	}
	finishedAt, err := time.Parse(time.RFC3339, result.Run.FinishedAt)
	if err != nil {
		finishedAt = time.Now().UTC()
	}
	reportPath, reportFile, err := createIndexReportFile(reportDir, finishedAt.UTC())
	if err != nil {
		return "", err
	}
	defer reportFile.Close()
	content := renderIndexCompletionReport(cmd, result, operationID, mode, usage, reportPath)
	if _, err := reportFile.WriteString(content); err != nil {
		return "", apperr.IOError("Failed to write index report: %s.", reportPath)
	}
	return reportPath, nil
}

func createIndexReportFile(reportDir string, finishedAt time.Time) (string, *os.File, error) {
	stem := finishedAt.Format("2006-01-02-150405")
	for i := 1; ; i++ {
		filename := stem + "-indexing-report.md"
		if i > 1 {
			filename = fmt.Sprintf("%s-%d-indexing-report.md", stem, i)
		}
		reportPath := filepath.Join(reportDir, filename)
		reportFile, err := os.OpenFile(reportPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return reportPath, reportFile, nil
		}
		if !os.IsExist(err) {
			return "", nil, apperr.IOError("Failed to write index report: %s.", reportPath)
		}
	}
}

func renderIndexCompletionReport(cmd cli.IndexCommand, result *index.PathIndexTree, operationID, mode string, usage tokenUsageSnapshot, reportPath string) string {
	run := result.Run
	lines := []string{
		"# Indexing Report",
		"",
		"## Summary",
		"",
		"- indexed_root: " + result.Root,
		"- mode: " + mode,
	}
	if operationID != "" {
		lines = append(lines, "- operation_id: "+operationID)
	}
	lines = append(lines,
		"- started_at: "+run.StartedAt,
		"- finished_at: "+run.FinishedAt,
		"- elapsed: "+elapsedText(run.StartedAt, run.FinishedAt),
		"- report_path: "+reportPath,
	)
	if cmd.IndexPlan != "" {
		lines = append(lines, "- index_plan: "+cmd.IndexPlan)
	}
	if cmd.IndexPlanDigest != "" {
		lines = append(lines, "- index_plan_digest: "+cmd.IndexPlanDigest)
	}
	lines = append(lines,
		"",
		"## File Counts",
		"",
		fmt.Sprintf("- total_discovered_files: %d", run.TotalDiscoveredFiles),
		fmt.Sprintf("- summarized_files: %d", run.SummarizedFiles),
		fmt.Sprintf("- unchanged_skipped_files: %d", run.UnchangedSkippedFiles),
		fmt.Sprintf("- not_indexed_files: %d", run.NotIndexedFiles),
		fmt.Sprintf("- failed_files: %d", run.FailedFiles),
		fmt.Sprintf("- metadata_files: %d", run.MetadataFiles),
		"",
		"## Token Usage",
		"",
	)
	if usage.Available {
		lines = append(lines,
			"- provider: "+usage.Provider,
			"- model: "+usage.Model,
			fmt.Sprintf("- prompt_tokens: %d", usage.PromptTokens),
			fmt.Sprintf("- output_tokens: %d", usage.CandidateTokens),
			fmt.Sprintf("- total_tokens: %d", usage.TotalTokens),
		)
	} else {
		lines = append(lines,
			"- provider: unavailable",
			"- model: unavailable",
			"- prompt_tokens: 0",
			"- output_tokens: 0",
			"- total_tokens: 0",
		)
	}
	lines = append(lines,
		"",
		"## Effective Options",
		"",
		fmt.Sprintf("- depth: %d", cmd.Depth),
		"- include: "+stringListText(cmd.Include),
		"- exclude: "+stringListText(cmd.Exclude),
		fmt.Sprintf("- refresh: %t", cmd.Refresh),
		fmt.Sprintf("- no_summary: %t", cmd.EffectiveNoSummary()),
		"- metadata_methods: "+stringListText(cmd.EffectiveMetadataMethods()),
		fmt.Sprintf("- max_file_size: %d", cmd.MaxFileSize),
		fmt.Sprintf("- concurrency: %d", cmd.Concurrency),
		"",
		"## Metadata Methods",
		"",
		"- enabled_methods: "+stringListText(run.EnabledMethods),
		fmt.Sprintf("- low_confidence_items: %d", run.LowConfidenceItems),
		fmt.Sprintf("- uncategorized_items: %d", run.UncategorizedItems),
		"",
		"### Method Status Counts",
		"",
	)
	lines = appendCountMap(lines, run.MethodStatusCounts)
	lines = append(lines, "", "### Method Category Counts", "")
	lines = appendCountMap(lines, run.MethodCategoryCounts)
	lines = append(lines, "", "### Method Tag Counts", "")
	lines = appendCountMap(lines, run.MethodTagCounts)
	lines = append(lines,
		"",
		"## Not Indexed Reasons",
		"",
	)
	lines = appendCountMap(lines, run.NotIndexedReasons)
	lines = append(lines, "", "## Failure Reasons", "")
	lines = appendCountMap(lines, run.FailureReasons)
	lines = append(lines, "", "## Warnings", "")
	if len(run.Warnings) == 0 {
		lines = append(lines, "- none")
	} else {
		for _, warning := range run.Warnings {
			lines = append(lines, "- "+warning)
		}
	}
	lines = append(lines, "", "## Covered Folders", "")
	if len(run.CoveredFolders) == 0 {
		lines = append(lines, "- none")
	} else {
		for _, folder := range run.CoveredFolders {
			lines = append(lines, "- "+folder)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func appendCountMap(lines []string, counts map[string]int) []string {
	if len(counts) == 0 {
		return append(lines, "- none")
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("- %s: %d", key, counts[key]))
	}
	return lines
}

func stringListText(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	return "[" + strings.Join(values, ", ") + "]"
}

func elapsedText(startedAt, finishedAt string) string {
	started, startErr := time.Parse(time.RFC3339, startedAt)
	finished, finishErr := time.Parse(time.RFC3339, finishedAt)
	if startErr != nil || finishErr != nil {
		return "unavailable"
	}
	return finished.Sub(started).String()
}

func renderIndexText(result *index.PathIndexTree) string {
	var lines []string
	var walk func(node index.IndexedPathNode, depth int)
	walk = func(node index.IndexedPathNode, depth int) {
		prefix := strings.Repeat("  ", depth)
		line := prefix + node.Path
		if node.Type == "dir" {
			line += "/"
		}
		if node.Summary != "" {
			line += " - " + node.Summary
		}
		lines = append(lines, line)
		for _, child := range node.Children {
			walk(child, depth+1)
		}
	}
	walk(result.Tree, 0)
	return strings.Join(lines, "\n")
}

func runOutputMeta(storePath, id string) (*output.CommandResult, error) {
	item, err := largeoutput.Meta(storePath, id)
	if err != nil {
		return nil, output.MapLargeOutputErr(err, id)
	}
	return &output.CommandResult{
		Data: map[string]any{
			"id":          item.ID,
			"path":        item.Path,
			"command":     item.Command,
			"created_at":  item.CreatedAt,
			"total_bytes": item.TotalBytes,
			"total_runes": item.TotalRunes,
		},
		Text: strings.Join([]string{
			fmt.Sprintf("%s %s %dB", item.ID, item.Command, item.TotalBytes),
			"next: aascribe output show " + item.ID,
			"slice: aascribe output slice " + item.ID + " --offset 0 --limit 4000",
		}, "\n"),
	}, nil
}

func runOutputShow(storePath, id string) (*output.CommandResult, error) {
	chunk, err := largeoutput.Show(storePath, id, largeoutput.DefaultConfig())
	if err != nil {
		return nil, output.MapLargeOutputErr(err, id)
	}
	return outputChunkResult(chunk), nil
}

func runOutputHead(storePath, id string, lines int) (*output.CommandResult, error) {
	chunk, err := largeoutput.Head(storePath, id, lines, largeoutput.DefaultConfig())
	if err != nil {
		return nil, output.MapLargeOutputErr(err, id)
	}
	return outputChunkResult(chunk), nil
}

func runOutputTail(storePath, id string, lines int) (*output.CommandResult, error) {
	chunk, err := largeoutput.Tail(storePath, id, lines, largeoutput.DefaultConfig())
	if err != nil {
		return nil, output.MapLargeOutputErr(err, id)
	}
	return outputChunkResult(chunk), nil
}

func runOutputSlice(storePath, id string, offset, limit int) (*output.CommandResult, error) {
	chunk, err := largeoutput.Slice(storePath, id, offset, limit, largeoutput.DefaultConfig())
	if err != nil {
		return nil, output.MapLargeOutputErr(err, id)
	}
	return outputChunkResult(chunk), nil
}

func outputChunkResult(chunk *largeoutput.Chunk) *output.CommandResult {
	cfg := largeoutput.DefaultConfig()
	return &output.CommandResult{
		Data: map[string]any{
			"output_id":          chunk.OutputID,
			"text":               chunk.Text,
			"range_start":        chunk.RangeStart,
			"range_end":          chunk.RangeEnd,
			"total_bytes":        chunk.TotalBytes,
			"total_runes":        chunk.TotalRunes,
			"available_commands": output.AvailableOutputCommands(chunk.OutputID, cfg),
		},
		Text: chunk.Text,
	}
}

func runPrompt(resolved *config.Resolved, prompt string) (*llm.TextResponse, error) {
	client := llm.NewGeminiClient(llm.GeminiConfig{
		Model:          resolved.LLM.Model,
		APIKey:         resolved.LLM.APIKey,
		TimeoutSeconds: resolved.LLM.TimeoutSeconds,
	})
	return client.GenerateText(prompt)
}

func buildSummaryPrompt(sourcePath, content string) string {
	return fmt.Sprintf(`You are given a file as input.

Task:
Generate a structured summary that helps a reader or LLM agent quickly understand the file.

Requirements:

1. Core Purpose
- What is the main purpose of this file?
- What role does it play in its context?

2. Key Concepts
- Extract important concepts, topics, or keywords
- Use precise terms (technical, domain-specific, or conceptual)
- Avoid vague descriptions

3. Main Content / Logic
- Describe the main ideas, flow, or logic in the file
- Keep it concise but accurate

4. Important Elements (if applicable)
- For code: exported/public functions, classes, APIs
- For documents: key sections, arguments, or structures
- For data: important fields or schema
- If none, state "No notable elements"

5. Dependencies / References (if applicable)
- Related systems, documents, or external references

Constraints:
- Be concise but precise
- Avoid vague phrases like "various things" or "general content"
- Prefer clarity and specificity over oversimplification
- Use bullet points
- Do not invent missing context; only mention dependencies or references that are explicitly visible in the file
- If a section has no meaningful content, say "None"

File path: %s

File content:
%s`, sourcePath, content)
}

func buildDescribePrompt(sourcePath, content, length, focus string) string {
	focusInstruction := "No special focus."
	if focus != "" {
		focusInstruction = "Prioritize this focus area: " + focus + "."
	}

	return fmt.Sprintf(`You are summarizing one file for aascribe describe.

Write exactly one concise paragraph.

Requirements:
- Summary length target: %s
- %s
- Explain the file's purpose and the most important visible content.
- Be specific and factual.
- Do not mention unseen context.
- Do not use bullets.

File path: %s

File content:
%s`, length, focusInstruction, sourcePath, content)
}

func buildLLMSummarizer(storePath string, usage *tokenUsageTracker) index.SummarizerFunc {
	resolved, err := config.Resolve(storePath, config.ResolveOptions{}, nil)
	if err != nil {
		return nil
	}
	return llmSummarizer(resolved, usage)
}

func buildRequiredLLMSummarizer(storePath string, usage *tokenUsageTracker) (index.SummarizerFunc, error) {
	resolved, err := config.Resolve(storePath, config.ResolveOptions{}, nil)
	if err != nil {
		return nil, err
	}
	return llmSummarizer(resolved, usage), nil
}

func loadIndexMethodPrompts(storePath string, methods []string) map[string]index.Prompt {
	out := map[string]index.Prompt{}
	for _, method := range methods {
		if method != index.MethodRetrievalV1 {
			continue
		}
		if prompt, ok := loadRetrievalPrompt(storePath); ok {
			out[method] = prompt
		}
	}
	return out
}

func loadRetrievalPrompt(storePath string) (index.Prompt, bool) {
	for _, path := range retrievalPromptCandidates(storePath) {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(content)
		return index.Prompt{
			Path:   filepath.ToSlash(path),
			SHA256: "sha256:" + hex.EncodeToString(sum[:]),
			Text:   string(content),
		}, true
	}
	content := []byte(defaultRetrievalPromptProposal(nil))
	sum := sha256.Sum256(content)
	return index.Prompt{
		Path:   "embedded:prompts/retrieval/generate.v1.md",
		SHA256: "sha256:" + hex.EncodeToString(sum[:]),
		Text:   string(content),
	}, true
}

func retrievalPromptCandidates(storePath string) []string {
	return []string{
		filepath.Join(storePath, "prompts", "retrieval", "generate.v1.md"),
		filepath.Join("prompts", "retrieval", "generate.v1.md"),
	}
}

func llmSummarizer(resolved *config.Resolved, usage *tokenUsageTracker) index.SummarizerFunc {
	return func(ctx context.Context, path, content, length, focus string) (string, error) {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		response, err := promptRunner(resolved, buildDescribePrompt(path, content, length, focus))
		if err != nil {
			return "", err
		}
		if usage != nil {
			usage.note(resolved.LLM.Provider, resolved.LLM.Model, response.Usage)
		}
		return response.Text, nil
	}
}

func copyFile(sourcePath, outputPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return apperr.IOError("Failed to open log file: %s.", sourcePath)
	}
	defer source.Close()

	dest, err := os.Create(outputPath)
	if err != nil {
		return apperr.IOError("Failed to create exported log file: %s.", outputPath)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return apperr.IOError("Failed to export log file to %s.", outputPath)
	}
	return nil
}
