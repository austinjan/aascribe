package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
)

type CommandResult struct {
	Data any
	Text string
}

type Meta struct {
	Command    string `json:"command"`
	DurationMS int64  `json:"duration_ms"`
	Store      string `json:"store"`
}

type successEnvelope struct {
	OK   bool `json:"ok"`
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

type errorEnvelope struct {
	OK    bool      `json:"ok"`
	Error errorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewMeta(command string, started time.Time, store string) Meta {
	return Meta{
		Command:    command,
		DurationMS: time.Since(started).Milliseconds(),
		Store:      store,
	}
}

func WriteSuccess(w io.Writer, format cli.Format, quiet bool, result *CommandResult, meta Meta) error {
	if quiet {
		return nil
	}

	switch format {
	case cli.FormatText:
		_, err := fmt.Fprintln(w, result.Text)
		return err
	default:
		return writeJSON(w, successEnvelope{
			OK:   true,
			Data: result.Data,
			Meta: meta,
		})
	}
}

func WriteError(w io.Writer, format cli.Format, quiet bool, err error, command string, started time.Time, store string) error {
	if quiet {
		return nil
	}

	appErr := normalizeError(err)
	switch format {
	case cli.FormatText:
		_, writeErr := fmt.Fprintln(w, appErr.Message)
		return writeErr
	default:
		return writeJSON(w, errorEnvelope{
			OK: false,
			Error: errorBody{
				Code:    appErr.Code,
				Message: appErr.Message,
			},
			Meta: NewMeta(command, started, store),
		})
	}
}

func normalizeError(err error) *apperr.Error {
	if err == nil {
		return apperr.IOError("Unknown runtime error.")
	}
	if appErr, ok := err.(*apperr.Error); ok {
		return appErr
	}
	return apperr.IOError("%s", err.Error())
}

func ErrorCode(err error) string {
	return normalizeError(err).Code
}

func writeJSON(w io.Writer, payload any) error {
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return apperr.Serialization("Failed to serialize command output.")
	}
	if _, err := fmt.Fprintln(w, string(bytes)); err != nil {
		return apperr.IOError("Failed to write command output.")
	}
	return nil
}
