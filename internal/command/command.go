package command

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/config"
	"github.com/austinjan/aascribe/internal/llm"
	"github.com/austinjan/aascribe/internal/logging"
	"github.com/austinjan/aascribe/internal/output"
	"github.com/austinjan/aascribe/internal/store"
	"github.com/austinjan/aascribe/pkg/llmoutput"
)

func Execute(command cli.Command, storePath string) (*output.CommandResult, error) {
	switch cmd := command.(type) {
	case cli.InitCommand:
		return runInit(storePath, cmd.Force)
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
	case cli.IndexCommand:
		return nil, apperr.NotImplemented(cmd.Name())
	case cli.DescribeCommand:
		return nil, apperr.NotImplemented(cmd.Name())
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

func runLogsPath(storePath string) *output.CommandResult {
	logPath := logging.ActiveLogPath(storePath)
	return &output.CommandResult{
		Data: map[string]any{
			"path": logPath,
		},
		Text: logPath,
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
		Text: fmt.Sprintf("Exported log file to %s", outputPath),
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
		Text: fmt.Sprintf("Cleared log file at %s", logPath),
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

	return &output.CommandResult{
		Data: map[string]any{
			"store":          storePath,
			"created":        outcome.Created,
			"reinitialized":  outcome.Reinitialized,
			"layout_version": store.LayoutVersion(),
		},
		Text: text,
	}, nil
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
	items, err := llmoutput.List(storePath)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s %s %dB", item.ID, item.Command, item.TotalBytes))
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

func runOutputMeta(storePath, id string) (*output.CommandResult, error) {
	item, err := llmoutput.Meta(storePath, id)
	if err != nil {
		return nil, err
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
		Text: fmt.Sprintf("%s %s %dB", item.ID, item.Command, item.TotalBytes),
	}, nil
}

func runOutputShow(storePath, id string) (*output.CommandResult, error) {
	chunk, err := llmoutput.Show(storePath, id, llmoutput.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return outputChunkResult(chunk), nil
}

func runOutputHead(storePath, id string, lines int) (*output.CommandResult, error) {
	chunk, err := llmoutput.Head(storePath, id, lines, llmoutput.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return outputChunkResult(chunk), nil
}

func runOutputTail(storePath, id string, lines int) (*output.CommandResult, error) {
	chunk, err := llmoutput.Tail(storePath, id, lines, llmoutput.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return outputChunkResult(chunk), nil
}

func runOutputSlice(storePath, id string, offset, limit int) (*output.CommandResult, error) {
	chunk, err := llmoutput.Slice(storePath, id, offset, limit, llmoutput.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return outputChunkResult(chunk), nil
}

func outputChunkResult(chunk *llmoutput.Chunk) *output.CommandResult {
	return &output.CommandResult{
		Data: map[string]any{
			"output_id":          chunk.OutputID,
			"text":               chunk.Text,
			"range_start":        chunk.RangeStart,
			"range_end":          chunk.RangeEnd,
			"total_bytes":        chunk.TotalBytes,
			"total_runes":        chunk.TotalRunes,
			"available_commands": chunk.Commands,
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
