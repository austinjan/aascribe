package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

const Version = "0.1.0"

type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

type Parsed struct {
	Store     string
	Format    Format
	Quiet     bool
	Verbose   bool
	Help      bool
	Version   bool
	HelpTopic string
	Command   Command
}

type Command interface {
	Name() string
}

type InitCommand struct {
	Force bool
}

func (c InitCommand) Name() string { return "init" }

type LogsPathCommand struct{}

func (c LogsPathCommand) Name() string { return "logs" }

type LogsExportCommand struct {
	Output string
}

func (c LogsExportCommand) Name() string { return "logs" }

type LogsClearCommand struct {
	Force bool
}

func (c LogsClearCommand) Name() string { return "logs" }

type OutputListCommand struct{}

func (c OutputListCommand) Name() string { return "output" }

type OutputMetaCommand struct {
	ID string
}

func (c OutputMetaCommand) Name() string { return "output" }

type OutputShowCommand struct {
	ID string
}

func (c OutputShowCommand) Name() string { return "output" }

type OutputHeadCommand struct {
	ID    string
	Lines int
}

func (c OutputHeadCommand) Name() string { return "output" }

type OutputTailCommand struct {
	ID    string
	Lines int
}

func (c OutputTailCommand) Name() string { return "output" }

type OutputSliceCommand struct {
	ID     string
	Offset int
	Limit  int
}

func (c OutputSliceCommand) Name() string { return "output" }

type OutputGenerateCommand struct {
	Lines  int
	Width  int
	Prefix string
}

func (c OutputGenerateCommand) Name() string { return "output" }

type IndexCommand struct {
	Path        string
	Depth       int
	Include     []string
	Exclude     []string
	Concurrency int
	Async       bool
	Refresh     bool
	NoSummary   bool
	MaxFileSize int64
}

func (c IndexCommand) Name() string { return "index" }

type IndexCleanCommand struct {
	Path   string
	DryRun bool
	Force  bool
}

func (c IndexCleanCommand) Name() string { return "index" }

type IndexDirtyCommand struct {
	Path string
}

func (c IndexDirtyCommand) Name() string { return "index" }

type IndexEvalCommand struct {
	Path string
}

func (c IndexEvalCommand) Name() string { return "index" }

type IndexMapCommand struct {
	Path string
}

func (c IndexMapCommand) Name() string { return "index" }

type MapCommand struct {
	Path string
}

func (c MapCommand) Name() string { return "map" }

type OperationListCommand struct{}

func (c OperationListCommand) Name() string { return "operation" }

type OperationStatusCommand struct {
	ID string
}

func (c OperationStatusCommand) Name() string { return "operation" }

type OperationEventsCommand struct {
	ID string
}

func (c OperationEventsCommand) Name() string { return "operation" }

type OperationResultCommand struct {
	ID string
}

func (c OperationResultCommand) Name() string { return "operation" }

type OperationCancelCommand struct {
	ID string
}

func (c OperationCancelCommand) Name() string { return "operation" }

type OperationCleanCommand struct {
	DryRun bool
	Force  bool
}

func (c OperationCleanCommand) Name() string { return "operation" }

type OperationRunIndexCommand struct {
	OperationID string
	Index       IndexCommand
}

func (c OperationRunIndexCommand) Name() string { return "operation" }

type DescribeCommand struct {
	File    string
	Refresh bool
	Length  string
	Focus   string
}

func (c DescribeCommand) Name() string { return "describe" }

type RememberCommand struct {
	Content    string
	HasContent bool
	Tags       []string
	Source     string
	Session    string
	TTL        string
	Importance int
	Stdin      bool
}

func (c RememberCommand) Name() string { return "remember" }

type ConsolidateCommand struct {
	Since     string
	Until     string
	Session   string
	Tags      []string
	Topic     string
	DryRun    bool
	KeepShort bool
	MinItems  int
}

func (c ConsolidateCommand) Name() string { return "consolidate" }

type RecallCommand struct {
	Query         string
	Tier          string
	Limit         int
	Tags          []string
	Since         string
	Until         string
	MinScore      float64
	IncludeSource bool
}

func (c RecallCommand) Name() string { return "recall" }

type ChatCommand struct {
	Prompt string
}

func (c ChatCommand) Name() string { return "chat" }

type SummarizeCommand struct {
	File string
}

func (c SummarizeCommand) Name() string { return "summarize" }

type ListCommand struct {
	Tier    string
	Session string
	Tags    []string
	Since   string
	Until   string
	Limit   int
	Order   string
}

func (c ListCommand) Name() string { return "list" }

type ShowCommand struct {
	ID string
}

func (c ShowCommand) Name() string { return "show" }

type ForgetCommand struct {
	ID    string
	Force bool
}

func (c ForgetCommand) Name() string { return "forget" }

func Parse(args []string) (*Parsed, error) {
	parsed := &Parsed{
		Format: FormatText,
	}

	rest, err := parseGlobals(args, parsed)
	if err != nil {
		return parsed, err
	}

	if parsed.Help && len(rest) == 0 {
		return parsed, nil
	}
	if parsed.Version && len(rest) == 0 {
		return parsed, nil
	}
	if len(rest) == 0 {
		return parsed, newParseError(ParseErrorNoSubcommand, "root", "", "No subcommand provided.")
	}

	commandName := rest[0]
	commandArgs := rest[1:]
	command, err := parseSubcommand(commandName, commandArgs)
	if err != nil {
		return parsed, err
	}

	parsed.Command = command
	return parsed, nil
}

func HelpText() string {
	return strings.TrimSpace(`
aascribe - Memory-first local CLI for project recall and agent-friendly workspace memory

Usage:
  aascribe [global flags] <command> [command flags]

What This CLI Does:
  aascribe stores and retrieves project memory in a local filesystem-backed store.
  It is designed for AI agents that need durable recall, indexing, and debugging context.
  When you are unsure where to start, initialize a store first, then inspect logs or use a command-specific help page.

Global flags:
  --store <path>       Path to the memory store root
  --format json|text   Output format (default: text)
  --quiet, -q          Suppress all command output
  --verbose, -v        Enable verbose mode
  --help, -h           Show help
  --version            Show version

Commands:
  init         Create or reinitialize the local memory store layout
  logs         Inspect, export, or clear aascribe logs
  output       Browse stored oversized outputs for LLM-safe continuation
  operation    Inspect persisted long-running operation state
  index        Index a folder for later retrieval and summarization
  describe     Summarize one file with optional length and focus controls
  remember     Write a short-term memory item
  consolidate  Turn short-term memories into longer-term memory entries
  recall       Search memories by query and filters
  chat         Send one prompt directly to the configured LLM for debugging
  summarize    Summarize one file through the LLM for debugging and quality checks
  list         List raw memory entries for inspection
  show         Show one memory entry in full
  forget       Delete one memory entry

Main Flows:
  1. Initialize a store for the current workspace:
     aascribe init
  2. Use an explicit project-local store when needed:
     aascribe --store ./project-mem init
  3. Check where logs are being written:
     aascribe logs path
  4. Export logs for debugging:
     aascribe logs export --output ./aascribe.log

Current Implementation Status:
  Working now:
    init
    logs path
    logs export
    logs clear
    output generate
    output list
    output meta
    output show
    output head
    output tail
    output slice
    operation list
    operation status
    operation events
    operation result
    operation cancel
    index
    index clean
    describe
    chat
    summarize
  CLI surface exists but command execution is still being implemented:
    remember, consolidate, recall, list, show, forget

How To Get More Information:
  aascribe <command> --help
  aascribe logs --help
  Read docs/USAGE.md for the longer reference
  Default output is compact text for LLM reading; prefer --format json when another agent or tool will parse the result
`)
}

func HelpTextForTopic(topic string) string {
	switch strings.TrimSpace(topic) {
	case "", "root":
		return HelpText()
	case "init":
		return strings.TrimSpace(`
aascribe init - create or reinitialize the memory store

Purpose:
  Set up the filesystem layout used by aascribe for short-term memory, long-term memory, index data, cache data, and logs.

Usage:
  aascribe init [--force]
  aascribe --store ./project-mem init [--force]

Key Flags:
  --force     Reinitialize an existing store and reset managed aascribe contents

Examples:
  aascribe init
  aascribe --store ./project-mem init
  aascribe --store ./project-mem init --force

What To Do Next:
  Run aascribe logs path to confirm the active log file
  Use aascribe --help to see the full command surface
  Read docs/USAGE.md for the longer command reference
`)
	case "logs":
		return strings.TrimSpace(`
aascribe logs - inspect and manage aascribe log files

Purpose:
  Help an agent discover the current log path, export logs for debugging, or clear logs explicitly.

Subcommands:
  path      Print the active log file path for the current store
  export    Copy the current log file to another path
  clear     Truncate the current log file, requires --force

Examples:
  aascribe logs path
  aascribe logs export --output ./aascribe.log
  aascribe logs clear --force
  aascribe --store ./project-mem logs path

Get More Information:
  aascribe logs path --help
  aascribe logs export --help
  aascribe logs clear --help
  Read docs/logging.md for deeper log examples
`)
	case "logs path":
		return strings.TrimSpace(`
aascribe logs path - print the active aascribe log file path

Purpose:
  Tell an agent where aascribe is currently writing logs for the active store.

Usage:
  aascribe logs path
  aascribe --store ./project-mem logs path

Examples:
  aascribe logs path
  aascribe --store ./project-mem logs path

Next Steps:
  Use aascribe logs export --output ./aascribe.log to copy the file
  Use aascribe logs clear --force to truncate the file
`)
	case "logs export":
		return strings.TrimSpace(`
aascribe logs export - copy the current log file to another path

Purpose:
  Export the active aascribe log file so another tool, agent, or person can inspect it.

Usage:
  aascribe logs export --output <path>
  aascribe --store ./project-mem logs export --output <path>

Required Flags:
  --output <path>     Destination file path for the exported log

Examples:
  aascribe logs export --output ./aascribe.log
  aascribe --store ./project-mem logs export --output ./debug/aascribe.log

Next Steps:
  Use aascribe logs path to confirm the source log file
  Read docs/logging.md for JSON examples and debugging guidance
`)
	case "logs clear":
		return strings.TrimSpace(`
aascribe logs clear - truncate the current log file

Purpose:
  Clear the active log file for the current store.

Usage:
  aascribe logs clear --force
  aascribe --store ./project-mem logs clear --force

Required Flags:
  --force     Required because this operation is destructive

Examples:
  aascribe logs clear --force
  aascribe --store ./project-mem logs clear --force

Next Steps:
  Use aascribe logs path to verify which file will be cleared
  Use aascribe logs export --output ./aascribe.log before clearing if you need a backup
`)
	case "output":
		return strings.TrimSpace(`
aascribe output - inspect stored oversized command outputs

Purpose:
  Help an agent continue reading large outputs that were spilled to managed files instead of being returned inline.

Subcommands:
  generate  Generate large output for testing the LLM output transport
  list      List recent stored outputs
  meta      Show metadata for one stored output
  show      Show metadata plus the default first chunk
  head      Show the first N lines of a stored output
  tail      Show the last N lines of a stored output
  slice     Show a deterministic rune-range slice of a stored output

Examples:
  aascribe output generate
  aascribe output generate --lines 300 --width 120
  aascribe output list
  aascribe output meta out_000001
  aascribe output show out_000001
  aascribe output head out_000001 --lines 100
  aascribe output tail out_000001 --lines 100
  aascribe output slice out_000001 --offset 4000 --limit 4000
`)
	case "output list":
		return strings.TrimSpace(`
aascribe output list - list recent stored outputs

Usage:
  aascribe output list

Examples:
  aascribe output list
`)
	case "output generate":
		return strings.TrimSpace(`
aascribe output generate - generate large output for transport testing

Usage:
  aascribe output generate [--lines <n>] [--width <n>] [--prefix <text>]

Examples:
  aascribe output generate
  aascribe output generate --lines 300 --width 120
  aascribe output generate --lines 20 --width 40 --prefix test
`)
	case "output meta":
		return strings.TrimSpace(`
aascribe output meta - show metadata for one stored output

Usage:
  aascribe output meta <output-id>

Examples:
  aascribe output meta out_000001
`)
	case "output show":
		return strings.TrimSpace(`
aascribe output show - show metadata plus the default first chunk

Usage:
  aascribe output show <output-id>

Examples:
  aascribe output show out_000001
`)
	case "output head":
		return strings.TrimSpace(`
aascribe output head - show the first N lines of a stored output

Usage:
  aascribe output head <output-id> [--lines <n>]

Examples:
  aascribe output head out_000001 --lines 100
`)
	case "output tail":
		return strings.TrimSpace(`
aascribe output tail - show the last N lines of a stored output

Usage:
  aascribe output tail <output-id> [--lines <n>]

Examples:
  aascribe output tail out_000001 --lines 100
`)
	case "output slice":
		return strings.TrimSpace(`
aascribe output slice - show a deterministic rune-range slice of a stored output

Usage:
  aascribe output slice <output-id> --offset <n> --limit <n>

Examples:
  aascribe output slice out_000001 --offset 4000 --limit 4000
`)
	case "operation":
		return strings.TrimSpace(`
aascribe operation - inspect persisted long-running operation state

Purpose:
  Help an agent inspect status, event history, and final results for persisted long-running operations.

Subcommands:
  list      List known operations for the active store
  status    Show the latest lifecycle snapshot for one operation
  events    Show the persisted event history for one operation
  result    Show the final result record for one completed operation
  cancel    Mark a pending or running operation as canceled
  clean     Remove terminal operation records from the active store

Examples:
  aascribe operation list
  aascribe operation status op_20260424T120000Z_ab12cd34
  aascribe operation events op_20260424T120000Z_ab12cd34
  aascribe operation result op_20260424T120000Z_ab12cd34
  aascribe operation cancel op_20260424T120000Z_ab12cd34
  aascribe operation clean --dry-run
`)
	case "operation list":
		return strings.TrimSpace(`
aascribe operation list - list persisted operations for the active store

Usage:
  aascribe operation list

Examples:
  aascribe operation list
`)
	case "operation status":
		return strings.TrimSpace(`
aascribe operation status - show the latest lifecycle snapshot for one operation

Usage:
  aascribe operation status <operation-id>

Examples:
  aascribe operation status op_20260424T120000Z_ab12cd34
`)
	case "operation events":
		return strings.TrimSpace(`
aascribe operation events - show persisted event history for one operation

Usage:
  aascribe operation events <operation-id>

Examples:
  aascribe operation events op_20260424T120000Z_ab12cd34
`)
	case "operation result":
		return strings.TrimSpace(`
aascribe operation result - show the final result record for one completed operation

Usage:
  aascribe operation result <operation-id>

Examples:
  aascribe operation result op_20260424T120000Z_ab12cd34
`)
	case "operation cancel":
		return strings.TrimSpace(`
aascribe operation cancel - mark a pending or running operation as canceled

Usage:
  aascribe operation cancel <operation-id>

Examples:
  aascribe operation cancel op_20260424T120000Z_ab12cd34
`)
	case "operation clean":
		return strings.TrimSpace(`
aascribe operation clean - remove terminal operation records

Purpose:
  Clean completed, failed, and canceled operation lifecycle records while preserving pending and running operations.

Usage:
  aascribe operation clean [--dry-run] [--force]

Behavior:
  Defaults to dry-run unless --force is provided.
  Does not remove managed output payloads referenced by operation results.

Examples:
  aascribe operation clean --dry-run
  aascribe operation clean --force
`)
	case "index":
		return strings.TrimSpace(`
aascribe index - index one path for later recall and summarization

Purpose:
  Prepare repository content for faster later retrieval by walking one directory tree, summarizing direct files per folder, and recording local-only index metadata.

Usage:
  aascribe index <path> [--depth <n>] [--include <glob>] [--exclude <glob>] [--refresh] [--no-summary] [--max-file-size <bytes>]
  aascribe index dirty <path>
  aascribe index eval <path>
  aascribe index clean <path> [--dry-run] --force

Behavior:
  Treat .gitignore and .aaignore in each visited folder as blacklist exclude files during traversal.
  Detect text files from file content instead of filename extension.
  Write .aascribe_index_meta.json in each indexed folder with local-only metadata for that folder.
  Use --concurrency to bound direct-file processing across the whole index run.
  Use --async to start index as an operation and inspect it later with operation status/events/result.
  Use index dirty to mark existing metadata stale so the next index run rebuilds it.
  Use index eval to preview which folders and direct files need indexing and which ones are unchanged.
  Use index clean to remove generated .aascribe_index_meta.json files recursively.

Examples:
  aascribe index .
  aascribe index --depth 2 ./internal
  aascribe index --concurrency 4 ./internal
  aascribe index --async ./internal
  aascribe --store ./project-mem index --exclude vendor .
  aascribe index dirty ./internal
  aascribe index eval ./internal
  aascribe index clean ./tests --dry-run --force

Further Info:
  aascribe --help
  Read docs/USAGE.md and docs/spec/index-spec.md for the stable index contract
`)
	case "index clean":
		return strings.TrimSpace(`
aascribe index clean - remove generated index metadata artifacts

Purpose:
  Recursively remove .aascribe_index_meta.json files created by aascribe index under one root path.

Usage:
  aascribe index clean <path> [--dry-run] --force

Required Flags:
  --force     Required because this command removes generated files

Optional Flags:
  --dry-run   Show which files would be removed without deleting them

Examples:
  aascribe index clean ./tests --dry-run --force
  aascribe index clean ./docs --force

Next Steps:
  Run aascribe index <path> again to regenerate metadata
  Use aascribe index --help for indexing behavior details
`)
	case "index dirty":
		return strings.TrimSpace(`
aascribe index dirty - mark existing index metadata as stale

Purpose:
  Mark .aascribe_index_meta.json files under one folder tree as dirty so the next index run rebuilds them instead of reusing unchanged file metadata.

Usage:
  aascribe index dirty <path>

Examples:
  aascribe index dirty ./internal
  aascribe index dirty ./tests

Notes:
  This command only updates existing metadata files.
  It does not create missing metadata files and does not summarize source files.
`)
	case "index eval":
		return strings.TrimSpace(`
aascribe index eval - preview what index would rebuild

Purpose:
  Evaluate one folder tree against current local metadata and report which folders and direct files need indexing, plus which ones are unchanged.

Usage:
  aascribe index eval <path>

Examples:
  aascribe index eval ./internal
  aascribe index eval ./tests

Notes:
  This command does not write metadata.
  Folder state is local-only: child folder changes do not make the parent folder changed unless the parent's own direct files or metadata changed.
`)
	case "describe":
		return strings.TrimSpace(`
aascribe describe - summarize one file

Purpose:
  Produce a short, medium, or long description of a single file, optionally focused on a specific area. This command surface exists, but execution is not fully implemented yet.

Usage:
  aascribe describe <file> [--length short|medium|long] [--focus <topic>] [--refresh]

Examples:
  aascribe describe ./README.md
  aascribe describe --length short ./internal/app/app.go
  aascribe describe --focus help ./internal/cli/cli.go

Further Info:
  aascribe --help
  Read docs/USAGE.md for the broader command reference
`)
	case "remember":
		return strings.TrimSpace(`
aascribe remember - write one short-term memory item

Purpose:
  Save a memory entry that an agent may want to recall later. This command surface exists, but execution is not fully implemented yet.

Usage:
  aascribe remember <content> [--tag <tag>] [--source <value>] [--session <id>] [--ttl <duration>] [--importance <1-5>]
  echo "..." | aascribe remember --stdin

Examples:
  aascribe remember "Need to revisit parser errors"
  aascribe remember "Confirm store path behavior" --tag todo --importance 4
  echo "Longer memory text" | aascribe remember --stdin

Further Info:
  aascribe --help
  Read docs/USAGE.md for the broader command reference
`)
	case "consolidate":
		return strings.TrimSpace(`
aascribe consolidate - merge short-term memories into longer-term memory

Purpose:
  Analyze short-term memory entries and produce longer-term memory records. This command surface exists, but execution is not fully implemented yet.

Usage:
  aascribe consolidate [--since <time>] [--until <time>] [--session <id>] [--tag <tag>] [--topic <topic>] [--dry-run] [--keep-short] [--min-items <n>]

Examples:
  aascribe consolidate
  aascribe consolidate --since 7d --topic parser
  aascribe consolidate --dry-run --tag bug

Further Info:
  aascribe --help
  Read docs/USAGE.md for the broader command reference
`)
	case "recall":
		return strings.TrimSpace(`
aascribe recall - search stored memories

Purpose:
  Retrieve relevant memories by query and filters. This command surface exists, but execution is not fully implemented yet.

Usage:
  aascribe recall <query> [--tier short|long|all] [--limit <n>] [--tag <tag>] [--since <time>] [--until <time>] [--min-score <0-1>] [--include-source]

Examples:
  aascribe recall "parser error"
  aascribe recall "store path" --tier short --limit 5
  aascribe recall "logging" --include-source

Further Info:
  aascribe --help
  Read docs/USAGE.md for the broader command reference
`)
	case "chat":
		return strings.TrimSpace(`
aascribe chat - send one prompt directly to the configured LLM

Purpose:
  Debug the LLM integration end to end using the current store config and secret env var.

Usage:
  aascribe chat <prompt>
  aascribe --store ./project-mem chat <prompt>

Examples:
  aascribe chat "Say hello in one short sentence."
  aascribe chat "Summarize the purpose of this repository in two bullets."
  aascribe --store ./project-mem chat "What model are you using?"

Next Steps:
  Check <store>/config.toml if config loading fails
  Confirm the API key env var configured in <store>/config.toml is set
  Use aascribe describe --help for the next intended consumer of the LLM layer
`)
	case "summarize":
		return strings.TrimSpace(`
aascribe summarize - summarize one file through the configured LLM

Purpose:
  Debug the summary quality for one file using the same LLM path that future describe/index work will rely on.

Usage:
  aascribe summarize <file>
  aascribe --store ./project-mem summarize <file>

Examples:
  aascribe summarize ./README.md
  aascribe summarize ./internal/cli/cli.go
  aascribe --store ./project-mem summarize ./main.go

Next Steps:
  Use aascribe chat to debug direct prompts separately
  Check <store>/config.toml if config loading fails
  Tune the prompt here before wiring describe/index onto the same summary core
`)
	case "list":
		return strings.TrimSpace(`
aascribe list - list memory entries

Purpose:
  Show raw memory entries for inspection or debugging. This command surface exists, but execution is not fully implemented yet.

Usage:
  aascribe list [--tier short|long|all] [--session <id>] [--tag <tag>] [--since <time>] [--until <time>] [--limit <n>] [--order asc|desc]

Examples:
  aascribe list
  aascribe list --tier short --limit 20
  aascribe list --tag todo --order asc

Further Info:
  aascribe --help
  Read docs/USAGE.md for the broader command reference
`)
	case "show":
		return strings.TrimSpace(`
aascribe show - show one memory entry in full

Purpose:
  Retrieve the full detail for one memory id. This command surface exists, but execution is not fully implemented yet.

Usage:
  aascribe show <id>

Examples:
  aascribe show mem_123
  aascribe show abcdef

Further Info:
  aascribe --help
  Read docs/USAGE.md for the broader command reference
`)
	case "forget":
		return strings.TrimSpace(`
aascribe forget - delete one memory entry

Purpose:
  Remove a stored memory entry by id. This command surface exists, but execution is not fully implemented yet.

Usage:
  aascribe forget <id> [--force]

Examples:
  aascribe forget mem_123
  aascribe forget mem_123 --force

Further Info:
  aascribe --help
  Read docs/USAGE.md for the broader command reference
`)
	default:
		return HelpText()
	}
}

func HelpTopicFromArgs(args []string) (string, bool) {
	if len(args) == 0 {
		return "root", true
	}

	tokens := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			return classifyHelpTopic(tokens), true
		case arg == "--store" || arg == "--format":
			i++
		case strings.HasPrefix(arg, "--store="), strings.HasPrefix(arg, "--format="):
		case strings.HasPrefix(arg, "-"):
		default:
			if len(tokens) < 2 {
				tokens = append(tokens, arg)
			}
		}
	}

	return "", false
}

func classifyHelpTopic(tokens []string) string {
	if len(tokens) == 0 {
		return "root"
	}
	if tokens[0] == "logs" {
		if len(tokens) > 1 {
			return "logs " + tokens[1]
		}
		return "logs"
	}
	if tokens[0] == "output" {
		if len(tokens) > 1 {
			return "output " + tokens[1]
		}
		return "output"
	}
	if tokens[0] == "operation" {
		if len(tokens) > 1 {
			return "operation " + tokens[1]
		}
		return "operation"
	}
	return tokens[0]
}

func parseGlobals(args []string, parsed *Parsed) ([]string, error) {
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			rest = append(rest, arg)
			i++
			continue
		}

		switch {
		case arg == "--store":
			if i+1 >= len(args) {
				return nil, newParseError(ParseErrorMissingFlagValue, "global", "--store", "Missing value for --store.")
			}
			parsed.Store = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "--store="):
			parsed.Store = strings.TrimPrefix(arg, "--store=")
			i++
		case arg == "--format":
			if i+1 >= len(args) {
				return nil, newParseError(ParseErrorMissingFlagValue, "global", "--format", "Missing value for --format.")
			}
			format, err := parseFormat(args[i+1])
			if err != nil {
				return nil, err
			}
			parsed.Format = format
			i += 2
		case strings.HasPrefix(arg, "--format="):
			format, err := parseFormat(strings.TrimPrefix(arg, "--format="))
			if err != nil {
				return nil, err
			}
			parsed.Format = format
			i++
		case arg == "--quiet" || arg == "-q":
			parsed.Quiet = true
			i++
		case arg == "--verbose" || arg == "-v":
			parsed.Verbose = true
			i++
		case arg == "--help" || arg == "-h":
			parsed.Help = true
			i++
		case arg == "--version":
			parsed.Version = true
			i++
		default:
			if len(rest) == 0 {
				return nil, newUnknownGlobalFlagError(arg)
			}
			rest = append(rest, arg)
			i++
		}
	}

	return rest, nil
}

func parseFormat(value string) (Format, error) {
	switch value {
	case "json":
		return FormatJSON, nil
	case "text":
		return FormatText, nil
	default:
		return "", newParseError(ParseErrorInvalidFlagValue, "global", "--format", "Invalid value for --format: %s.", value)
	}
}

func parseSubcommand(name string, args []string) (Command, error) {
	switch name {
	case "init":
		return parseInit(args)
	case "logs":
		return parseLogs(args)
	case "output":
		return parseOutput(args)
	case "operation":
		return parseOperation(args)
	case "index":
		return parseIndex(args)
	case "map":
		return parseMap(args)
	case "describe":
		return parseDescribe(args)
	case "remember":
		return parseRemember(args)
	case "consolidate":
		return parseConsolidate(args)
	case "recall":
		return parseRecall(args)
	case "chat":
		return parseChat(args)
	case "summarize":
		return parseSummarize(args)
	case "list":
		return parseList(args)
	case "show":
		return parseShow(args)
	case "forget":
		return parseForget(args)
	default:
		return nil, newUnknownCommandError(name)
	}
}

func parseLogs(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, newParseError(ParseErrorMissingNestedCommand, "logs", "", "logs requires a subcommand: path, export, or clear.")
	}

	switch args[0] {
	case "path":
		if len(args[1:]) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "logs path", "", "logs path does not accept extra arguments.")
		}
		return LogsPathCommand{}, nil
	case "export":
		fs := newFlagSet("logs export")
		var output string
		fs.StringVar(&output, "output", "", "")
		if err := parseFlags(fs, args[1:]); err != nil {
			return nil, err
		}
		if output == "" {
			return nil, newParseError(ParseErrorMissingRequiredFlag, "logs export", "--output", "logs export requires --output <path>.")
		}
		if len(fs.Args()) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "logs export", "", "logs export does not accept positional arguments.")
		}
		return LogsExportCommand{Output: output}, nil
	case "clear":
		fs := newFlagSet("logs clear")
		var force bool
		fs.BoolVar(&force, "force", false, "")
		if err := parseFlags(fs, args[1:]); err != nil {
			return nil, err
		}
		if len(fs.Args()) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "logs clear", "", "logs clear does not accept positional arguments.")
		}
		if !force {
			return nil, newParseError(ParseErrorMissingRequiredFlag, "logs clear", "--force", "logs clear requires --force.")
		}
		return LogsClearCommand{Force: true}, nil
	default:
		return nil, newUnknownNestedCommandError("logs", args[0], []string{"path", "export", "clear"})
	}
}

func parseOutput(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, newParseError(ParseErrorMissingNestedCommand, "output", "", "output requires a subcommand: list, meta, show, head, tail, or slice.")
	}

	switch args[0] {
	case "generate":
		fs := newFlagSet("output generate")
		var lines int
		var width int
		var prefix string
		fs.IntVar(&lines, "lines", 300, "")
		fs.IntVar(&width, "width", 120, "")
		fs.StringVar(&prefix, "prefix", "line", "")
		if err := parseFlags(fs, args[1:]); err != nil {
			return nil, err
		}
		if len(fs.Args()) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "output generate", "", "output generate does not accept positional arguments.")
		}
		if lines <= 0 {
			return nil, newParseError(ParseErrorInvalidFlagValue, "output generate", "--lines", "Invalid value for --lines: %d.", lines)
		}
		if width <= 0 {
			return nil, newParseError(ParseErrorInvalidFlagValue, "output generate", "--width", "Invalid value for --width: %d.", width)
		}
		return OutputGenerateCommand{Lines: lines, Width: width, Prefix: prefix}, nil
	case "list":
		if len(args[1:]) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "output list", "", "output list does not accept extra arguments.")
		}
		return OutputListCommand{}, nil
	case "meta":
		if len(args[1:]) != 1 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "output meta", "output-id", "output meta requires exactly one output id argument.")
		}
		return OutputMetaCommand{ID: args[1]}, nil
	case "show":
		if len(args[1:]) != 1 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "output show", "output-id", "output show requires exactly one output id argument.")
		}
		return OutputShowCommand{ID: args[1]}, nil
	case "head":
		if len(args) < 2 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "output head", "output-id", "output head requires exactly one output id argument.")
		}
		fs := newFlagSet("output head")
		var lines int
		fs.IntVar(&lines, "lines", 100, "")
		if err := parseFlags(fs, args[2:]); err != nil {
			return nil, err
		}
		if len(fs.Args()) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "output head", "", "output head does not accept extra positional arguments.")
		}
		return OutputHeadCommand{ID: args[1], Lines: lines}, nil
	case "tail":
		if len(args) < 2 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "output tail", "output-id", "output tail requires exactly one output id argument.")
		}
		fs := newFlagSet("output tail")
		var lines int
		fs.IntVar(&lines, "lines", 100, "")
		if err := parseFlags(fs, args[2:]); err != nil {
			return nil, err
		}
		if len(fs.Args()) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "output tail", "", "output tail does not accept extra positional arguments.")
		}
		return OutputTailCommand{ID: args[1], Lines: lines}, nil
	case "slice":
		if len(args) < 2 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "output slice", "output-id", "output slice requires exactly one output id argument.")
		}
		fs := newFlagSet("output slice")
		var offset int
		var limit int
		fs.IntVar(&offset, "offset", -1, "")
		fs.IntVar(&limit, "limit", 0, "")
		if err := parseFlags(fs, args[2:]); err != nil {
			return nil, err
		}
		if len(fs.Args()) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "output slice", "", "output slice does not accept extra positional arguments.")
		}
		if offset < 0 {
			return nil, newParseError(ParseErrorMissingRequiredFlag, "output slice", "--offset", "output slice requires --offset <n>.")
		}
		if limit <= 0 {
			return nil, newParseError(ParseErrorMissingRequiredFlag, "output slice", "--limit", "output slice requires --limit <n>.")
		}
		return OutputSliceCommand{ID: args[1], Offset: offset, Limit: limit}, nil
	default:
		return nil, newUnknownNestedCommandError("output", args[0], []string{"list", "meta", "show", "head", "tail", "slice"})
	}
}

func parseInit(args []string) (Command, error) {
	fs := newFlagSet("init")
	var force bool
	fs.BoolVar(&force, "force", false, "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 0 {
		return nil, newParseError(ParseErrorDoesNotAcceptArgs, "init", "", "init does not accept positional arguments.")
	}
	return InitCommand{Force: force}, nil
}

func parseOperation(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, newParseError(ParseErrorMissingNestedCommand, "operation", "", "operation requires a subcommand: list, status, events, result, cancel, or clean.")
	}

	switch args[0] {
	case "list":
		if len(args[1:]) != 0 {
			return nil, newParseError(ParseErrorDoesNotAcceptArgs, "operation list", "", "operation list does not accept extra arguments.")
		}
		return OperationListCommand{}, nil
	case "status":
		if len(args[1:]) != 1 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "operation status", "operation-id", "operation status requires exactly one operation id argument.")
		}
		return OperationStatusCommand{ID: args[1]}, nil
	case "events":
		if len(args[1:]) != 1 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "operation events", "operation-id", "operation events requires exactly one operation id argument.")
		}
		return OperationEventsCommand{ID: args[1]}, nil
	case "result":
		if len(args[1:]) != 1 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "operation result", "operation-id", "operation result requires exactly one operation id argument.")
		}
		return OperationResultCommand{ID: args[1]}, nil
	case "cancel":
		if len(args[1:]) != 1 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "operation cancel", "operation-id", "operation cancel requires exactly one operation id argument.")
		}
		return OperationCancelCommand{ID: args[1]}, nil
	case "clean":
		return parseOperationClean(args[1:])
	case "run-index":
		return parseOperationRunIndex(args[1:])
	default:
		return nil, newUnknownNestedCommandError("operation", args[0], []string{"list", "status", "events", "result", "cancel", "clean"})
	}
}

func parseOperationClean(args []string) (Command, error) {
	fs := newFlagSet("operation clean")
	var cmd OperationCleanCommand
	fs.BoolVar(&cmd.DryRun, "dry-run", false, "")
	fs.BoolVar(&cmd.Force, "force", false, "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 0 {
		return nil, newParseError(ParseErrorDoesNotAcceptArgs, "operation clean", "", "operation clean does not accept positional arguments.")
	}
	if !cmd.Force {
		cmd.DryRun = true
	}
	return cmd, nil
}

func parseIndex(args []string) (Command, error) {
	if len(args) > 0 && args[0] == "clean" {
		return parseIndexClean(args[1:])
	}
	if len(args) > 0 && args[0] == "dirty" {
		return parseIndexDirty(args[1:])
	}
	if len(args) > 0 && args[0] == "eval" {
		return parseIndexEval(args[1:])
	}
	if len(args) > 0 && args[0] == "map" {
		return parseMap(args[1:])
	}
	fs := newFlagSet("index")
	var cmd IndexCommand
	cmd.Depth = 3
	cmd.Concurrency = 4
	cmd.MaxFileSize = 1_048_576
	var include stringSlice
	var exclude stringSlice
	fs.IntVar(&cmd.Depth, "depth", 3, "")
	fs.Var(&include, "include", "")
	fs.Var(&exclude, "exclude", "")
	fs.IntVar(&cmd.Concurrency, "concurrency", 4, "")
	fs.BoolVar(&cmd.Async, "async", false, "")
	fs.BoolVar(&cmd.Refresh, "refresh", false, "")
	fs.BoolVar(&cmd.NoSummary, "no-summary", false, "")
	fs.Int64Var(&cmd.MaxFileSize, "max-file-size", 1_048_576, "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "index", "path", "index requires exactly one path argument.")
	}
	if cmd.Concurrency <= 0 {
		return nil, newParseError(ParseErrorInvalidFlagValue, "index", "--concurrency", "Invalid value for --concurrency: %d.", cmd.Concurrency)
	}
	cmd.Path = fs.Args()[0]
	cmd.Include = include
	cmd.Exclude = exclude
	return cmd, nil
}

func parseOperationRunIndex(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "operation run-index", "operation-id", "operation run-index requires an operation id argument.")
	}
	cmd, err := parseIndex(args[1:])
	if err != nil {
		return nil, err
	}
	indexCmd := cmd.(IndexCommand)
	indexCmd.Async = false
	return OperationRunIndexCommand{
		OperationID: args[0],
		Index:       indexCmd,
	}, nil
}

func parseIndexMap(args []string) (Command, error) {
	return parseMap(args)
}

func parseIndexDirty(args []string) (Command, error) {
	fs := newFlagSet("index dirty")
	var cmd IndexDirtyCommand
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "index dirty", "path", "index dirty requires exactly one path argument.")
	}
	cmd.Path = fs.Args()[0]
	return cmd, nil
}

func parseIndexEval(args []string) (Command, error) {
	fs := newFlagSet("index eval")
	var cmd IndexEvalCommand
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "index eval", "path", "index eval requires exactly one path argument.")
	}
	cmd.Path = fs.Args()[0]
	return cmd, nil
}

func parseMap(args []string) (Command, error) {
	fs := newFlagSet("index map")
	var cmd MapCommand
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "map", "path", "map requires exactly one path argument.")
	}
	cmd.Path = fs.Args()[0]
	return cmd, nil
}

func parseIndexClean(args []string) (Command, error) {
	fs := newFlagSet("index clean")
	var cmd IndexCleanCommand
	fs.BoolVar(&cmd.DryRun, "dry-run", false, "")
	fs.BoolVar(&cmd.Force, "force", false, "")
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd.Path = args[0]
		if err := parseFlags(fs, args[1:]); err != nil {
			return nil, err
		}
		if len(fs.Args()) != 0 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "index clean", "path", "index clean requires exactly one path argument.")
		}
	} else {
		if err := parseFlags(fs, args); err != nil {
			return nil, err
		}
		if len(fs.Args()) != 1 {
			return nil, newParseError(ParseErrorMissingRequiredArg, "index clean", "path", "index clean requires exactly one path argument.")
		}
		cmd.Path = fs.Args()[0]
	}
	if !cmd.Force {
		return nil, newParseError(ParseErrorMissingRequiredFlag, "index clean", "--force", "index clean requires --force.")
	}
	return cmd, nil
}

func parseDescribe(args []string) (Command, error) {
	fs := newFlagSet("describe")
	var cmd DescribeCommand
	cmd.Length = "medium"
	fs.BoolVar(&cmd.Refresh, "refresh", false, "")
	fs.StringVar(&cmd.Length, "length", "medium", "")
	fs.StringVar(&cmd.Focus, "focus", "", "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "describe", "file", "describe requires exactly one file argument.")
	}
	if !oneOf(cmd.Length, "short", "medium", "long") {
		return nil, newParseError(ParseErrorInvalidFlagValue, "describe", "--length", "Invalid value for --length: %s.", cmd.Length)
	}
	cmd.File = fs.Args()[0]
	return cmd, nil
}

func parseRemember(args []string) (Command, error) {
	fs := newFlagSet("remember")
	var cmd RememberCommand
	var tags stringSlice
	cmd.Importance = 3
	fs.Var(&tags, "tag", "")
	fs.StringVar(&cmd.Source, "source", "", "")
	fs.StringVar(&cmd.Session, "session", "", "")
	fs.StringVar(&cmd.TTL, "ttl", "", "")
	fs.IntVar(&cmd.Importance, "importance", 3, "")
	fs.BoolVar(&cmd.Stdin, "stdin", false, "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) > 1 {
		return nil, newParseError(ParseErrorTooManyArgs, "remember", "content", "remember accepts at most one positional content argument.")
	}
	if len(fs.Args()) == 1 {
		cmd.Content = fs.Args()[0]
		cmd.HasContent = true
	}
	cmd.Tags = tags
	return cmd, nil
}

func parseConsolidate(args []string) (Command, error) {
	fs := newFlagSet("consolidate")
	var cmd ConsolidateCommand
	var tags stringSlice
	cmd.Since = "7d"
	cmd.Until = "now"
	cmd.MinItems = 3
	fs.StringVar(&cmd.Since, "since", "7d", "")
	fs.StringVar(&cmd.Until, "until", "now", "")
	fs.StringVar(&cmd.Session, "session", "", "")
	fs.Var(&tags, "tag", "")
	fs.StringVar(&cmd.Topic, "topic", "", "")
	fs.BoolVar(&cmd.DryRun, "dry-run", false, "")
	fs.BoolVar(&cmd.KeepShort, "keep-short", false, "")
	fs.IntVar(&cmd.MinItems, "min-items", 3, "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 0 {
		return nil, newParseError(ParseErrorDoesNotAcceptArgs, "consolidate", "", "consolidate does not accept positional arguments.")
	}
	cmd.Tags = tags
	return cmd, nil
}

func parseRecall(args []string) (Command, error) {
	fs := newFlagSet("recall")
	var cmd RecallCommand
	var tags stringSlice
	cmd.Tier = "all"
	cmd.Limit = 10
	cmd.MinScore = 0.3
	fs.StringVar(&cmd.Tier, "tier", "all", "")
	fs.IntVar(&cmd.Limit, "limit", 10, "")
	fs.Var(&tags, "tag", "")
	fs.StringVar(&cmd.Since, "since", "", "")
	fs.StringVar(&cmd.Until, "until", "", "")
	fs.Float64Var(&cmd.MinScore, "min-score", 0.3, "")
	fs.BoolVar(&cmd.IncludeSource, "include-source", false, "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "recall", "query", "recall requires exactly one query argument.")
	}
	if !oneOf(cmd.Tier, "short", "long", "all") {
		return nil, newParseError(ParseErrorInvalidFlagValue, "recall", "--tier", "Invalid value for --tier: %s.", cmd.Tier)
	}
	cmd.Query = fs.Args()[0]
	cmd.Tags = tags
	return cmd, nil
}

func parseChat(args []string) (Command, error) {
	fs := newFlagSet("chat")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "chat", "prompt", "chat requires exactly one prompt argument.")
	}
	return ChatCommand{Prompt: fs.Args()[0]}, nil
}

func parseSummarize(args []string) (Command, error) {
	fs := newFlagSet("summarize")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "summarize", "file", "summarize requires exactly one file argument.")
	}
	return SummarizeCommand{File: fs.Args()[0]}, nil
}

func parseList(args []string) (Command, error) {
	fs := newFlagSet("list")
	var cmd ListCommand
	var tags stringSlice
	cmd.Tier = "all"
	cmd.Limit = 50
	cmd.Order = "desc"
	fs.StringVar(&cmd.Tier, "tier", "all", "")
	fs.StringVar(&cmd.Session, "session", "", "")
	fs.Var(&tags, "tag", "")
	fs.StringVar(&cmd.Since, "since", "", "")
	fs.StringVar(&cmd.Until, "until", "", "")
	fs.IntVar(&cmd.Limit, "limit", 50, "")
	fs.StringVar(&cmd.Order, "order", "desc", "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 0 {
		return nil, newParseError(ParseErrorDoesNotAcceptArgs, "list", "", "list does not accept positional arguments.")
	}
	if !oneOf(cmd.Tier, "short", "long", "all") {
		return nil, newParseError(ParseErrorInvalidFlagValue, "list", "--tier", "Invalid value for --tier: %s.", cmd.Tier)
	}
	if !oneOf(cmd.Order, "asc", "desc") {
		return nil, newParseError(ParseErrorInvalidFlagValue, "list", "--order", "Invalid value for --order: %s.", cmd.Order)
	}
	cmd.Tags = tags
	return cmd, nil
}

func parseShow(args []string) (Command, error) {
	fs := newFlagSet("show")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "show", "id", "show requires exactly one id argument.")
	}
	return ShowCommand{ID: fs.Args()[0]}, nil
}

func parseForget(args []string) (Command, error) {
	fs := newFlagSet("forget")
	var cmd ForgetCommand
	fs.BoolVar(&cmd.Force, "force", false, "")
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, newParseError(ParseErrorMissingRequiredArg, "forget", "id", "forget requires exactly one id argument.")
	}
	cmd.ID = fs.Args()[0]
	return cmd, nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}

		name, hasInlineValue := flagName(arg)
		flagDef := fs.Lookup(name)
		flags = append(flags, arg)
		if flagDef == nil || hasInlineValue || isBoolFlag(flagDef) {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}

	ordered := append(flags, positionals...)
	return fs.Parse(ordered)
}

func flagName(arg string) (string, bool) {
	name := strings.TrimLeft(arg, "-")
	valueIndex := strings.Index(name, "=")
	if valueIndex < 0 {
		return name, false
	}
	return name[:valueIndex], true
}

func isBoolFlag(flagDef *flag.Flag) bool {
	_, ok := flagDef.Value.(interface{ IsBoolFlag() bool })
	return ok
}

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func (p *Parsed) String() string {
	return fmt.Sprintf("format=%s command=%v", p.Format, p.Command)
}
