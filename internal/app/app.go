package app

import (
	"fmt"
	"io"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/aascribe/internal/command"
	"github.com/austinjan/aascribe/internal/output"
	"github.com/austinjan/aascribe/internal/store"
)

func Run(args []string, stdout io.Writer) int {
	started := time.Now()
	parsed, err := cli.Parse(args)
	if err != nil {
		format := cli.FormatJSON
		storePath := "<unresolved>"
		commandName := "<parse>"
		if parsed != nil {
			if parsed.Format != "" {
				format = parsed.Format
			}
			if parsed.Store != "" {
				storePath = parsed.Store
			}
		}
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
		_ = output.WriteError(stdout, parsed.Format, parsed.Quiet, err, parsed.Command.Name(), started, unresolvedStore(parsed.Store))
		return normalizedStatus(err)
	}

	result, err := command.Execute(parsed.Command, storePath)
	if err != nil {
		_ = output.WriteError(stdout, parsed.Format, parsed.Quiet, err, parsed.Command.Name(), started, storePath)
		return normalizedStatus(err)
	}

	if err := output.WriteSuccess(stdout, parsed.Format, parsed.Quiet, result, output.NewMeta(parsed.Command.Name(), started, storePath)); err != nil {
		_ = output.WriteError(stdout, parsed.Format, parsed.Quiet, err, parsed.Command.Name(), started, storePath)
		return normalizedStatus(err)
	}

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
