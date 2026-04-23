package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
	Examples []string `json:"examples,omitempty"`
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
	hint, examples := errorGuidance(appErr, command, store)
	switch format {
	case cli.FormatText:
		_, writeErr := fmt.Fprintln(w, renderTextError(appErr.Message, hint, examples))
		return writeErr
	default:
		return writeJSON(w, errorEnvelope{
			OK: false,
			Error: errorBody{
				Code:     appErr.Code,
				Message:  appErr.Message,
				Hint:     hint,
				Examples: examples,
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

func renderTextError(message, hint string, examples []string) string {
	body := message
	if hint != "" {
		body += "\nHint: " + hint
	}
	if len(examples) > 0 {
		body += "\nExamples:"
		for _, example := range examples {
			body += "\n  " + example
		}
	}
	return body
}

func errorGuidance(appErr *apperr.Error, command, store string) (string, []string) {
	message := appErr.Message
	switch appErr.Code {
	case "INVALID_ARGUMENTS":
		if strings.Contains(message, "No subcommand provided.") {
			return "Pick a subcommand or ask for help with `aascribe --help`.", []string{
				"aascribe init",
				"aascribe list",
				"aascribe logs path",
			}
		}
		if strings.Contains(message, "Missing value for --store.") {
			return "Pass a store path after `--store`, or omit the flag to use the default store.", []string{
				"aascribe --store ./project-mem init",
				"aascribe --store ./project-mem list",
				"aascribe init",
			}
		}
		if strings.Contains(message, "Missing value for --format.") || strings.Contains(message, "Invalid value for --format:") {
			return "Use one of the supported formats: `json` or `text`.", []string{
				"aascribe --format json logs path",
				"aascribe --format text init",
				"aascribe --help",
			}
		}
		if strings.Contains(message, "Unknown global flag") {
			return "Check the available global flags before retrying.", []string{
				"aascribe --help",
				"aascribe --format text init",
				"aascribe --store ./project-mem logs path",
			}
		}
		if strings.Contains(message, "Unknown subcommand") {
			return "Use one of the supported top-level commands.", []string{
				"aascribe init",
				"aascribe list",
				"aascribe logs path",
			}
		}
		if strings.Contains(message, "logs requires a subcommand") || strings.Contains(message, "Unknown logs subcommand") {
			return "Double-check the required flags for the logs subcommand you chose.", []string{
				"aascribe logs path",
				"aascribe logs export --output ./aascribe.log",
				"aascribe logs clear --force",
			}
		}
		if strings.Contains(message, "logs export requires --output") || strings.Contains(message, "logs export does not accept positional arguments") {
			return "Provide an output path with `--output` and avoid extra positional arguments.", []string{
				"aascribe logs export --output ./aascribe.log",
				"aascribe --store ./project-mem logs export --output ./debug/aascribe.log",
				"aascribe logs path",
			}
		}
		if strings.Contains(message, "logs clear requires --force") || strings.Contains(message, "logs clear does not accept positional arguments") {
			return "Clearing logs is destructive, so you need `--force` and no extra positional arguments.", []string{
				"aascribe logs clear --force",
				"aascribe --store ./project-mem logs clear --force",
				"aascribe logs path",
			}
		}
		if strings.Contains(message, "logs path does not accept extra arguments") {
			return "Run `logs path` by itself with no extra arguments.", []string{
				"aascribe logs path",
				"aascribe --store ./project-mem logs path",
				"aascribe logs export --output ./aascribe.log",
			}
		}
		if strings.Contains(message, "init does not accept positional arguments") {
			return "Run `init` without positional arguments; use flags like `--force` or `--store` instead.", []string{
				"aascribe init",
				"aascribe init --force",
				"aascribe --store ./project-mem init",
			}
		}
		if strings.Contains(message, "index requires exactly one path argument") {
			return "Pass exactly one path to index.", []string{
				"aascribe index .",
				"aascribe index ./internal --depth 2",
				"aascribe --store ./project-mem index .",
			}
		}
		if strings.Contains(message, "describe requires exactly one file argument") {
			return "Pass exactly one file path to describe.", []string{
				"aascribe describe ./README.md",
				"aascribe describe ./main.go --length short",
				"aascribe --store ./project-mem describe ./internal/app/app.go",
			}
		}
		if strings.Contains(message, "Invalid value for --length:") {
			return "Use one of the supported description lengths: `short`, `medium`, or `long`.", []string{
				"aascribe describe ./README.md --length short",
				"aascribe describe ./README.md --length medium",
				"aascribe describe ./README.md --length long",
			}
		}
		if strings.Contains(message, "remember accepts at most one positional content argument") {
			return "Pass the memory text as one argument, or use `--stdin` for longer content.", []string{
				`aascribe remember "Need to revisit parser errors"`,
				`echo "Need to revisit parser errors" | aascribe remember --stdin`,
				`aascribe remember "Add test coverage" --tag todo`,
			}
		}
		if strings.Contains(message, "consolidate does not accept positional arguments") {
			return "Use flags to filter consolidation; do not pass positional arguments.", []string{
				"aascribe consolidate",
				"aascribe consolidate --since 7d --topic parser",
				"aascribe consolidate --dry-run --tag bug",
			}
		}
		if strings.Contains(message, "recall requires exactly one query argument") {
			return "Pass exactly one recall query string.", []string{
				`aascribe recall "parser error"`,
				`aascribe recall "store path" --tier short`,
				`aascribe recall "logging" --limit 5`,
			}
		}
		if strings.Contains(message, "Invalid value for --tier:") {
			return "Use one of the supported tiers: `short`, `long`, or `all`.", []string{
				`aascribe recall "parser" --tier short`,
				"aascribe list --tier long",
				"aascribe list --tier all",
			}
		}
		if strings.Contains(message, "list does not accept positional arguments") {
			return "Run `list` without positional arguments and filter with flags instead.", []string{
				"aascribe list",
				"aascribe list --tier short",
				"aascribe list --tag todo --limit 10",
			}
		}
		if strings.Contains(message, "Invalid value for --order:") {
			return "Use one of the supported sort orders: `asc` or `desc`.", []string{
				"aascribe list --order asc",
				"aascribe list --order desc",
				"aascribe list --tier short --order desc",
			}
		}
		if strings.Contains(message, "show requires exactly one id argument") {
			return "Pass exactly one memory id to `show`.", []string{
				"aascribe show mem_123",
				"aascribe list",
				"aascribe recall \"parser\"",
			}
		}
		if strings.Contains(message, "forget requires exactly one id argument") {
			return "Pass exactly one memory id to `forget`.", []string{
				"aascribe forget mem_123 --force",
				"aascribe list",
				"aascribe show mem_123",
			}
		}
		return "Check the command syntax and required flags, or inspect the built-in help.", []string{
			"aascribe --help",
			"aascribe " + command + " --help",
		}
	case "STORE_NOT_FOUND":
		return "Initialize a store first, or point to the correct existing store with `--store`.", []string{
			"aascribe init",
			"aascribe --store ./project-mem init",
			"aascribe --store ./project-mem list",
		}
	case "NOT_IMPLEMENTED":
		switch command {
		case "index":
			return "Indexing is not wired up yet. For now, initialize a store or inspect logs while this command is still under development.", []string{
				"aascribe init",
				"aascribe logs path",
				"aascribe --store ./project-mem logs export --output ./aascribe.log",
			}
		case "describe":
			return "Describe exists in the CLI surface but is not implemented yet.", []string{
				"aascribe init",
				"aascribe logs path",
				"aascribe --help",
			}
		case "remember", "consolidate", "recall", "list", "show", "forget":
			return "That memory command is exposed in the interface but is not implemented yet.", []string{
				"aascribe init",
				"aascribe logs path",
				"aascribe --help",
			}
		default:
			return "That command exists in the interface but is not wired up yet. Use one of the currently working commands instead.", []string{
				"aascribe init",
				"aascribe logs path",
				"aascribe logs export --output ./aascribe.log",
			}
		}
	case "LOG_FILE_NOT_FOUND":
		targetStore := store
		if targetStore == "" || targetStore == "<unresolved>" {
			targetStore = "./data/memory"
		}
		return "No log file exists yet for that store. Run a command that writes logs, or confirm you are using the right store path.", []string{
			"aascribe --store " + targetStore + " init",
			"aascribe --store " + targetStore + " list",
			"aascribe --store " + targetStore + " logs path",
		}
	default:
		return "Review the error details, then retry with `--help` or a more specific subcommand.", []string{
			"aascribe --help",
			"aascribe logs path",
		}
	}
}
