package app

import (
	"fmt"
	"io"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/command"
	"github.com/austinjan/aascribe/internal/logging"
	"github.com/austinjan/aascribe/internal/output"
	"github.com/austinjan/aascribe/internal/store"
)

func Run(args []string, stdout, stderr io.Writer) int {
	started := time.Now()
	if helpTopic, ok := cli.HelpTopicFromArgs(args); ok {
		_, _ = fmt.Fprintln(stdout, cli.HelpTextForTopic(helpTopic))
		return apperr.ExitSuccess.Code()
	}

	parsed, err := cli.Parse(args)
	if err != nil {
		format := cli.FormatJSON
		storePath := "<unresolved>"
		commandName := "<parse>"
		verbose := false
		if parsed != nil {
			if parsed.Format != "" {
				format = parsed.Format
			}
			if parsed.Store != "" {
				storePath = parsed.Store
			}
			verbose = parsed.Verbose
		}
		logger := logging.New(stderr, "", verbose)
		defer logger.Close()
		logger.Debug("command failed during parse", "command", commandName, "store", storePath, "error_code", output.ErrorCode(err), "error_message", err.Error())
		_ = output.WriteError(stdout, format, false, err, commandName, started, storePath)
		return normalizedStatus(err)
	}

	if parsed.Version && parsed.Command == nil {
		_, _ = fmt.Fprintln(stdout, cli.Version)
		return apperr.ExitSuccess.Code()
	}
	if parsed.Help && parsed.Command == nil {
		_, _ = fmt.Fprintln(stdout, cli.HelpText())
		return apperr.ExitSuccess.Code()
	}

	storePath, err := store.ResolveStorePath(parsed.Store)
	if err != nil {
		logger := logging.New(stderr, "", parsed.Verbose)
		defer logger.Close()
		logger.Error("failed to resolve store path", "command", parsed.Command.Name(), "store", unresolvedStore(parsed.Store), "error_code", output.ErrorCode(err), "error_message", err.Error())
		_ = output.WriteError(stdout, parsed.Format, parsed.Quiet, err, parsed.Command.Name(), started, unresolvedStore(parsed.Store))
		return normalizedStatus(err)
	}

	logStorePath := storePath
	if parsed.Command.Name() == "logs" || parsed.Command.Name() == "init" {
		logStorePath = ""
	}
	logger := logging.New(stderr, logStorePath, parsed.Verbose)
	defer logger.Close()
	logger.Info("command started", "command", parsed.Command.Name(), "store", storePath, "format", parsed.Format, "verbose", parsed.Verbose)

	result, err := command.Execute(parsed.Command, storePath)
	if err != nil {
		logger.Error("command failed", "command", parsed.Command.Name(), "store", storePath, "error_code", output.ErrorCode(err), "error_message", err.Error(), "duration_ms", time.Since(started).Milliseconds())
		_ = output.WriteError(stdout, parsed.Format, parsed.Quiet, err, parsed.Command.Name(), started, storePath)
		return normalizedStatus(err)
	}

	if err := output.WriteSuccess(stdout, parsed.Format, parsed.Quiet, result, output.NewMeta(parsed.Command.Name(), started, storePath)); err != nil {
		logger.Error("failed to write command output", "command", parsed.Command.Name(), "store", storePath, "error_code", output.ErrorCode(err), "error_message", err.Error())
		_ = output.WriteError(stdout, parsed.Format, parsed.Quiet, err, parsed.Command.Name(), started, storePath)
		return normalizedStatus(err)
	}

	logger.Info("command finished", "command", parsed.Command.Name(), "store", storePath, "status", "ok", "duration_ms", time.Since(started).Milliseconds())
	return apperr.ExitSuccess.Code()
}

func normalizedStatus(err error) int {
	if appErr, ok := err.(*apperr.Error); ok {
		return appErr.Status.Code()
	}
	return apperr.ExitGeneralRuntimeError.Code()
}

func unresolvedStore(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return "<unresolved>"
}
