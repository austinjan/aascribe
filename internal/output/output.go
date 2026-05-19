package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
	"github.com/austinjan/agentcli-core/largeoutput"
)

type CommandResult struct {
	Data           any
	Text           string
	PrimaryTextKey string
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

	payloadData := result.Data
	textOut := result.Text
	if result.PrimaryTextKey != "" && meta.Store != "" {
		cfg := largeoutput.DefaultConfig()
		delivered, err := largeoutput.Deliver(meta.Store, meta.Command, result.Text, largeoutput.DeliverOptions{Config: cfg})
		if err != nil {
			return err
		}
		textOut = renderDeliveredText(delivered, cfg)
		payloadData = decorateData(result.Data, result.PrimaryTextKey, delivered, cfg)
	}

	switch format {
	case cli.FormatText:
		_, err := fmt.Fprintln(w, textOut)
		return err
	default:
		return writeJSON(w, successEnvelope{
			OK:   true,
			Data: payloadData,
			Meta: meta,
		})
	}
}

func WriteError(w io.Writer, format cli.Format, quiet bool, err error, command string, started time.Time, store string) error {
	if quiet {
		return nil
	}

	appErr := normalizeError(err)
	hint, examples := errorGuidance(err, appErr, command, store)
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
	type appErrorCarrier interface {
		AppError() *apperr.Error
	}
	if carrier, ok := err.(appErrorCarrier); ok && carrier.AppError() != nil {
		return carrier.AppError()
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

func errorGuidance(err error, appErr *apperr.Error, command, store string) (string, []string) {
	message := appErr.Message
	if parseErr, ok := err.(*cli.ParseError); ok {
		return parseErrorGuidance(parseErr)
	}
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
		if strings.Contains(message, "operation requires a subcommand") || strings.Contains(message, "Unknown operation subcommand") {
			return "Choose one of the supported operation subcommands.", []string{
				"aascribe operation list",
				"aascribe operation status op_20260424T120000Z_ab12cd34",
				"aascribe operation events op_20260424T120000Z_ab12cd34",
				"aascribe operation result op_20260424T120000Z_ab12cd34",
			}
		}
		if strings.Contains(message, "operation list does not accept extra arguments") {
			return "Run `operation list` by itself with no extra arguments.", []string{
				"aascribe operation list",
				"aascribe operation status op_20260424T120000Z_ab12cd34",
			}
		}
		if strings.Contains(message, "operation status requires exactly one operation id argument") {
			return "Pass exactly one operation id to `operation status`.", []string{
				"aascribe operation list",
				"aascribe operation status op_20260424T120000Z_ab12cd34",
			}
		}
		if strings.Contains(message, "operation events requires exactly one operation id argument") {
			return "Pass exactly one operation id to `operation events`.", []string{
				"aascribe operation list",
				"aascribe operation events op_20260424T120000Z_ab12cd34",
			}
		}
		if strings.Contains(message, "operation result requires exactly one operation id argument") {
			return "Pass exactly one operation id to `operation result`.", []string{
				"aascribe operation list",
				"aascribe operation result op_20260424T120000Z_ab12cd34",
			}
		}
		if strings.Contains(message, "operation cancel requires exactly one operation id argument") {
			return "Pass exactly one operation id to `operation cancel`.", []string{
				"aascribe operation list",
				"aascribe operation cancel op_20260424T120000Z_ab12cd34",
			}
		}
		if strings.Contains(message, "operation clean does not accept positional arguments") {
			return "Run `operation clean` without positional arguments; use `--dry-run` to preview or `--force` to remove terminal operation records.", []string{
				"aascribe operation clean --dry-run",
				"aascribe operation clean --force",
				"aascribe operation list",
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
		if strings.Contains(message, "chat requires exactly one prompt argument") {
			return "Pass exactly one quoted prompt string to the debug chat command.", []string{
				`aascribe chat "Say hello in one short sentence."`,
				`aascribe --store ./project-mem chat "Summarize this repository in one paragraph."`,
				"aascribe chat --help",
			}
		}
		if strings.Contains(message, "summarize requires exactly one file argument") {
			return "Pass exactly one file path to the summarize debug command.", []string{
				"aascribe summarize ./README.md",
				"aascribe summarize ./internal/cli/cli.go",
				"aascribe summarize --help",
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
	case "CONFIG_NOT_FOUND":
		return "Create the config skeleton, then set the required Gemini API key. The key has no default.", []string{
			"aascribe config gemini_api_key=your-real-key",
			"cat ./data/config/config.toml",
			"aascribe index <path>",
		}
	case "INVALID_CONFIG":
		return "Fix data/config/config.toml. LLM-backed commands require [llm] provider = \"gemini\", model, api_key_env, and a positive timeout_seconds.", []string{
			"cat ./data/config/config.toml",
			"set missing fields such as: model = \"gemini-2.5-flash\"",
			"aascribe config gemini_api_key=your-real-key",
		}
	case "MISSING_SECRET":
		return "Set the required Gemini API key before running summary-backed index or other LLM-backed commands. The key has no default.", []string{
			"aascribe config gemini_api_key=your-real-key",
			"export GEMINI_API_KEY=your-real-key",
			"echo 'GEMINI_API_KEY=your-real-key' >> ./data/config/.env",
			"aascribe index <path>",
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
	case "OUTPUT_NOT_FOUND":
		return "That stored output id does not exist anymore. Check the retained outputs list and retry with a current id.", []string{
			"aascribe output list",
			"aascribe output meta out_000001",
			"aascribe output show out_000001",
		}
	case "OPERATION_NOT_FOUND":
		return "That operation id does not exist in the active store. List current operations first and retry with a valid id.", []string{
			"aascribe operation list",
			"aascribe operation status op_20260424T120000Z_ab12cd34",
			"aascribe operation result op_20260424T120000Z_ab12cd34",
		}
	case "OPERATION_RESULT_NOT_READY":
		return "That operation has not completed yet. Inspect its current state or event history, then retry later.", []string{
			"aascribe operation status op_20260424T120000Z_ab12cd34",
			"aascribe operation events op_20260424T120000Z_ab12cd34",
			"aascribe operation result op_20260424T120000Z_ab12cd34",
		}
	case "OPERATION_ALREADY_TERMINAL":
		return "That operation already finished, failed, or was canceled. Inspect status or result instead.", []string{
			"aascribe operation status op_20260424T120000Z_ab12cd34",
			"aascribe operation result op_20260424T120000Z_ab12cd34",
			"aascribe operation list",
		}
	default:
		return "Review the error details, then retry with `--help` or a more specific subcommand.", []string{
			"aascribe --help",
			"aascribe logs path",
		}
	}
}

func parseErrorGuidance(err *cli.ParseError) (string, []string) {
	switch err.Kind {
	case cli.ParseErrorNoSubcommand:
		return "Pick a subcommand or ask for help with `aascribe --help`.", []string{
			"aascribe init",
			"aascribe logs path",
			"aascribe summarize ./README.md",
		}
	case cli.ParseErrorMissingFlagValue:
		if err.Token == "--store" {
			return "Pass a store path after `--store`, or omit the flag to use the default store.", []string{
				"aascribe --store ./project-mem init",
				"aascribe --store ./project-mem logs path",
				"aascribe init",
			}
		}
		if err.Token == "--format" {
			return "Provide a value after `--format`. Supported values are `json` and `text`.", []string{
				"aascribe --format json logs path",
				"aascribe --format text chat \"Say hello\"",
				"aascribe --help",
			}
		}
	case cli.ParseErrorInvalidFlagValue:
		switch err.Token {
		case "--format":
			return "Use one of the supported formats: `json` or `text`.", []string{
				"aascribe --format json logs path",
				"aascribe --format text init",
				"aascribe --help",
			}
		case "--length":
			return "Use one of the supported description lengths: `short`, `medium`, or `long`.", []string{
				"aascribe describe ./README.md --length short",
				"aascribe describe ./README.md --length medium",
				"aascribe describe ./README.md --length long",
			}
		case "--tier":
			return "Use one of the supported tiers: `short`, `long`, or `all`.", []string{
				`aascribe recall "parser" --tier short`,
				"aascribe list --tier long",
				"aascribe list --tier all",
			}
		case "--order":
			return "Use one of the supported sort orders: `asc` or `desc`.", []string{
				"aascribe list --order asc",
				"aascribe list --order desc",
				"aascribe list --tier short --order desc",
			}
		}
	case cli.ParseErrorUnknownGlobalFlag:
		if len(err.Suggestions) > 0 {
			return "That global flag is not recognized. Did you mean one of these?", prefixExamples(err.Suggestions, "aascribe ")
		}
		return "Check the available global flags before retrying.", []string{
			"aascribe --help",
			"aascribe --format text init",
			"aascribe --store ./project-mem logs path",
		}
	case cli.ParseErrorUnknownCommand:
		if len(err.Suggestions) > 0 {
			return "That top-level command is not recognized. Did you mean one of these?", prefixExamples(err.Suggestions, "aascribe ")
		}
		return "Use one of the supported top-level commands.", []string{
			"aascribe init",
			"aascribe logs path",
			"aascribe summarize ./README.md",
		}
	case cli.ParseErrorMissingNestedCommand:
		if err.Scope == "logs" {
			return "Choose one of the supported logs subcommands.", []string{
				"aascribe logs path",
				"aascribe logs export --output ./aascribe.log",
				"aascribe logs clear --force",
			}
		}
		if err.Scope == "output" {
			return "Choose one of the supported output subcommands.", []string{
				"aascribe output generate",
				"aascribe output list",
				"aascribe output meta out_000001",
				"aascribe output show out_000001",
			}
		}
		if err.Scope == "operation" {
			return "Choose one of the supported operation subcommands.", []string{
				"aascribe operation list",
				"aascribe operation status op_20260424T120000Z_ab12cd34",
				"aascribe operation events op_20260424T120000Z_ab12cd34",
				"aascribe operation result op_20260424T120000Z_ab12cd34",
				"aascribe operation cancel op_20260424T120000Z_ab12cd34",
			}
		}
	case cli.ParseErrorUnknownNestedCommand:
		if err.Scope == "logs" {
			if len(err.Suggestions) > 0 {
				return "That logs subcommand is not recognized. Did you mean one of these?", prefixExamples(err.Suggestions, "aascribe logs ")
			}
			return "Double-check the logs subcommand you chose.", []string{
				"aascribe logs path",
				"aascribe logs export --output ./aascribe.log",
				"aascribe logs clear --force",
			}
		}
		if err.Scope == "output" {
			if len(err.Suggestions) > 0 {
				return "That output subcommand is not recognized. Did you mean one of these?", prefixExamples(err.Suggestions, "aascribe output ")
			}
			return "Double-check the output subcommand you chose.", []string{
				"aascribe output generate",
				"aascribe output list",
				"aascribe output meta out_000001",
				"aascribe output show out_000001",
			}
		}
		if err.Scope == "operation" {
			if len(err.Suggestions) > 0 {
				return "That operation subcommand is not recognized. Did you mean one of these?", prefixExamples(err.Suggestions, "aascribe operation ")
			}
			return "Double-check the operation subcommand you chose.", []string{
				"aascribe operation list",
				"aascribe operation status op_20260424T120000Z_ab12cd34",
				"aascribe operation events op_20260424T120000Z_ab12cd34",
				"aascribe operation result op_20260424T120000Z_ab12cd34",
				"aascribe operation cancel op_20260424T120000Z_ab12cd34",
			}
		}
	case cli.ParseErrorMissingRequiredFlag:
		switch err.Scope {
		case "logs export":
			return "Provide an output path with `--output`.", []string{
				"aascribe logs export --output ./aascribe.log",
				"aascribe --store ./project-mem logs export --output ./debug/aascribe.log",
				"aascribe logs path",
			}
		case "logs clear":
			return "Clearing logs is destructive, so you need `--force`.", []string{
				"aascribe logs clear --force",
				"aascribe --store ./project-mem logs clear --force",
				"aascribe logs path",
			}
		case "output slice":
			if err.Token == "--offset" {
				return "Provide the starting rune offset to read from the stored output.", []string{
					"aascribe output slice out_000001 --offset 0 --limit 4000",
					"aascribe output show out_000001",
					"aascribe output meta out_000001",
				}
			}
			return "Provide a positive `--limit` when requesting a stored output slice.", []string{
				"aascribe output slice out_000001 --offset 0 --limit 4000",
				"aascribe output head out_000001 --lines 100",
				"aascribe output tail out_000001 --lines 100",
			}
		}
	case cli.ParseErrorMissingRequiredArg:
		switch err.Scope {
		case "index":
			return "Pass exactly one path to index.", []string{
				"aascribe index .",
				"aascribe index ./internal --depth 2",
				"aascribe --store ./project-mem index .",
			}
		case "describe":
			return "Pass exactly one file path to describe.", []string{
				"aascribe describe ./README.md",
				"aascribe describe ./main.go --length short",
				"aascribe --store ./project-mem describe ./internal/app/app.go",
			}
		case "recall":
			return "Pass exactly one recall query string.", []string{
				`aascribe recall "parser error"`,
				`aascribe recall "store path" --tier short`,
				`aascribe recall "logging" --limit 5`,
			}
		case "chat":
			return "Pass exactly one quoted prompt string to the debug chat command.", []string{
				`aascribe chat "Say hello in one short sentence."`,
				`aascribe --store ./project-mem chat "Summarize this repository in one paragraph."`,
				"aascribe chat --help",
			}
		case "summarize":
			return "Pass exactly one file path to the summarize debug command.", []string{
				"aascribe summarize ./README.md",
				"aascribe summarize ./internal/cli/cli.go",
				"aascribe summarize --help",
			}
		case "show":
			return "Pass exactly one memory id to `show`.", []string{
				"aascribe show mem_123",
				"aascribe list",
				"aascribe recall \"parser\"",
			}
		case "forget":
			return "Pass exactly one memory id to `forget`.", []string{
				"aascribe forget mem_123 --force",
				"aascribe list",
				"aascribe show mem_123",
			}
		case "output meta":
			return "Pass exactly one stored output id to `output meta`.", []string{
				"aascribe output generate",
				"aascribe output list",
				"aascribe output meta out_000001",
				"aascribe output show out_000001",
			}
		case "output show":
			return "Pass exactly one stored output id to `output show`.", []string{
				"aascribe output generate",
				"aascribe output show out_000001",
				"aascribe output list",
				"aascribe output meta out_000001",
			}
		case "output head":
			return "Pass exactly one stored output id to `output head`.", []string{
				"aascribe output generate",
				"aascribe output head out_000001 --lines 100",
				"aascribe output list",
				"aascribe output meta out_000001",
			}
		case "output tail":
			return "Pass exactly one stored output id to `output tail`.", []string{
				"aascribe output generate",
				"aascribe output tail out_000001 --lines 100",
				"aascribe output list",
				"aascribe output meta out_000001",
			}
		case "output slice":
			return "Pass exactly one stored output id to `output slice`.", []string{
				"aascribe output generate",
				"aascribe output slice out_000001 --offset 0 --limit 4000",
				"aascribe output show out_000001",
				"aascribe output meta out_000001",
			}
		case "operation status":
			return "Pass exactly one operation id to `operation status`.", []string{
				"aascribe operation list",
				"aascribe operation status op_20260424T120000Z_ab12cd34",
			}
		case "operation events":
			return "Pass exactly one operation id to `operation events`.", []string{
				"aascribe operation list",
				"aascribe operation events op_20260424T120000Z_ab12cd34",
			}
		case "operation result":
			return "Pass exactly one operation id to `operation result`.", []string{
				"aascribe operation list",
				"aascribe operation result op_20260424T120000Z_ab12cd34",
			}
		case "operation cancel":
			return "Pass exactly one operation id to `operation cancel`.", []string{
				"aascribe operation list",
				"aascribe operation cancel op_20260424T120000Z_ab12cd34",
			}
		}
	case cli.ParseErrorDoesNotAcceptArgs:
		switch err.Scope {
		case "logs path":
			return "Run `logs path` by itself with no extra arguments.", []string{
				"aascribe logs path",
				"aascribe --store ./project-mem logs path",
				"aascribe logs export --output ./aascribe.log",
			}
		case "logs export":
			return "Provide `--output` and avoid extra positional arguments.", []string{
				"aascribe logs export --output ./aascribe.log",
				"aascribe --store ./project-mem logs export --output ./debug/aascribe.log",
				"aascribe logs path",
			}
		case "logs clear":
			return "Use `--force` and do not pass extra positional arguments.", []string{
				"aascribe logs clear --force",
				"aascribe --store ./project-mem logs clear --force",
				"aascribe logs path",
			}
		case "init":
			return "Run `init` without positional arguments; use flags like `--force` or `--store` instead.", []string{
				"aascribe init",
				"aascribe init --force",
				"aascribe --store ./project-mem init",
			}
		case "consolidate":
			return "Use flags to filter consolidation; do not pass positional arguments.", []string{
				"aascribe consolidate",
				"aascribe consolidate --since 7d --topic parser",
				"aascribe consolidate --dry-run --tag bug",
			}
		case "list":
			return "Run `list` without positional arguments and filter with flags instead.", []string{
				"aascribe list",
				"aascribe list --tier short",
				"aascribe list --tag todo --limit 10",
			}
		case "output list":
			return "Run `output list` by itself with no extra arguments.", []string{
				"aascribe output generate",
				"aascribe output list",
				"aascribe output meta out_000001",
				"aascribe output show out_000001",
			}
		case "operation list":
			return "Run `operation list` by itself with no extra arguments.", []string{
				"aascribe operation list",
				"aascribe operation status op_20260424T120000Z_ab12cd34",
			}
		case "output generate":
			return "Run `output generate` without positional arguments; use flags like `--lines`, `--width`, and `--prefix` instead.", []string{
				"aascribe output generate",
				"aascribe output generate --lines 300 --width 120",
				"aascribe output list",
			}
		}
	case cli.ParseErrorTooManyArgs:
		if err.Scope == "remember" {
			return "Pass the memory text as one argument, or use `--stdin` for longer content.", []string{
				`aascribe remember "Need to revisit parser errors"`,
				`echo "Need to revisit parser errors" | aascribe remember --stdin`,
				`aascribe remember "Add test coverage" --tag todo`,
			}
		}
	}

	return "Check the command syntax and required flags, or inspect the built-in help.", []string{
		"aascribe --help",
		"aascribe " + err.Scope + " --help",
	}
}

func decorateData(data any, primaryTextKey string, delivered *largeoutput.DeliveredOutput, cfg largeoutput.Config) any {
	if !delivered.Continuation.Truncated {
		return data
	}
	transport := buildTransportEnvelope(delivered.Continuation, cfg)
	payload, ok := data.(map[string]any)
	if !ok {
		return map[string]any{
			"value":     data,
			"transport": transport,
		}
	}
	cloned := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		cloned[key] = value
	}
	cloned[primaryTextKey] = delivered.InlineText
	cloned["transport"] = transport
	return cloned
}

func renderDeliveredText(delivered *largeoutput.DeliveredOutput, cfg largeoutput.Config) string {
	if !delivered.Continuation.Truncated {
		return delivered.InlineText
	}
	c := delivered.Continuation
	return strings.TrimSpace(fmt.Sprintf(`%s

[partial output]
output_id: %s
range: %d..%d of %d runes
next: %s`, delivered.InlineText, c.OutputID, c.InlineRangeStart, c.InlineRangeEnd, c.TotalRunes, NextOutputSliceCommand(c.OutputID, c.NextOffset, cfg)))
}

// buildTransportEnvelope produces the aascribe-specific JSON shape consumers
// expect under data.transport, composing the next_suggestion and
// available_commands strings from the structured Continuation fields.
func buildTransportEnvelope(c largeoutput.Continuation, cfg largeoutput.Config) map[string]any {
	out := map[string]any{
		"stored":             c.Stored,
		"truncated":          c.Truncated,
		"inline_range_start": c.InlineRangeStart,
		"inline_range_end":   c.InlineRangeEnd,
		"total_bytes":        c.TotalBytes,
		"total_runes":        c.TotalRunes,
	}
	if c.OutputID != "" {
		out["output_id"] = c.OutputID
		out["next_suggestion"] = NextOutputSliceCommand(c.OutputID, c.NextOffset, cfg)
		out["available_commands"] = AvailableOutputCommands(c.OutputID, cfg)
	}
	if c.TruncationMsg != "" {
		out["truncation_msg"] = c.TruncationMsg
	}
	if len(c.Hints) > 0 {
		out["hints"] = c.Hints
	}
	return out
}

// NextOutputSliceCommand returns the aascribe-shaped command string the agent
// should run to fetch the next chunk of a stored output.
func NextOutputSliceCommand(id string, nextOffset int, cfg largeoutput.Config) string {
	return fmt.Sprintf("aascribe output slice %s --offset %d --limit %d", id, nextOffset, cfg.DefaultSliceRunes)
}

// MapLargeOutputErr translates a largeoutput sentinel error into the matching
// aascribe apperr.Error so the JSON envelope and exit code stay backward
// compatible. Pass the id the caller looked up (or "" if not applicable).
func MapLargeOutputErr(err error, id string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, largeoutput.ErrNotFound) {
		return apperr.NotFoundOutput(id)
	}
	if errors.Is(err, largeoutput.ErrInvalidOffset) {
		return apperr.InvalidArguments("%s", err.Error())
	}
	return err
}

// AvailableOutputCommands lists the aascribe commands an agent can use to
// inspect a stored output.
func AvailableOutputCommands(id string, cfg largeoutput.Config) []string {
	return []string{
		fmt.Sprintf("aascribe output show %s", id),
		fmt.Sprintf("aascribe output head %s --lines %d", id, cfg.DefaultHeadLines),
		fmt.Sprintf("aascribe output tail %s --lines %d", id, cfg.DefaultTailLines),
		fmt.Sprintf("aascribe output slice %s --offset %d --limit %d", id, cfg.DefaultSliceRunes, cfg.DefaultSliceRunes),
		fmt.Sprintf("aascribe output meta %s", id),
	}
}

func prefixExamples(values []string, prefix string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, prefix+value)
	}
	return out
}
