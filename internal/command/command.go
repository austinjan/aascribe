package command

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/logging"
	"github.com/austinjan/aascribe/internal/output"
	"github.com/austinjan/aascribe/internal/store"
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
