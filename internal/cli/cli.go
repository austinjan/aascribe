package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/austinjan/aascribe/internal/apperr"
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

type IndexCommand struct {
	Path        string
	Depth       int
	Include     []string
	Exclude     []string
	Refresh     bool
	NoSummary   bool
	MaxFileSize int64
}

func (c IndexCommand) Name() string { return "index" }

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
		Format: FormatJSON,
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
		return parsed, apperr.InvalidArguments("No subcommand provided.")
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
  --format json|text   Output format (default: json)
  --quiet, -q          Suppress all command output
  --verbose, -v        Enable verbose mode
  --help, -h           Show help
  --version            Show version

Commands:
  init         Create or reinitialize the local memory store layout
  logs         Inspect, export, or clear aascribe logs
  index        Index a folder for later retrieval and summarization
  describe     Summarize one file with optional length and focus controls
  remember     Write a short-term memory item
  consolidate  Turn short-term memories into longer-term memory entries
  recall       Search memories by query and filters
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
  CLI surface exists but command execution is still being implemented:
    index, describe, remember, consolidate, recall, list, show, forget

How To Get More Information:
  aascribe <command> --help
  aascribe logs --help
  Read docs/USAGE.md for the longer reference
  Prefer --format json when another agent will parse the result
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
	case "index":
		return strings.TrimSpace(`
aascribe index - index one path for later recall and summarization

Purpose:
  Prepare repository content for faster later retrieval. This command is part of the intended workflow, but execution is not fully implemented yet.

Usage:
  aascribe index <path> [--depth <n>] [--include <glob>] [--exclude <glob>] [--refresh] [--no-summary] [--max-file-size <bytes>]

Examples:
  aascribe index .
  aascribe index ./internal --depth 2
  aascribe --store ./project-mem index . --exclude vendor

Further Info:
  aascribe --help
  Read docs/USAGE.md and docs/tasks/index-tasks.md for design context
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
  aascribe describe ./internal/app/app.go --length short
  aascribe describe ./internal/cli/cli.go --focus help

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
	return tokens[0]
}

func parseGlobals(args []string, parsed *Parsed) ([]string, error) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			return args[i:], nil
		}

		switch {
		case arg == "--store":
			if i+1 >= len(args) {
				return nil, apperr.InvalidArguments("Missing value for --store.")
			}
			parsed.Store = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "--store="):
			parsed.Store = strings.TrimPrefix(arg, "--store=")
			i++
		case arg == "--format":
			if i+1 >= len(args) {
				return nil, apperr.InvalidArguments("Missing value for --format.")
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
			return nil, apperr.InvalidArguments("Unknown global flag %s.", arg)
		}
	}

	return nil, nil
}

func parseFormat(value string) (Format, error) {
	switch value {
	case "json":
		return FormatJSON, nil
	case "text":
		return FormatText, nil
	default:
		return "", apperr.InvalidArguments("Invalid value for --format: %s.", value)
	}
}

func parseSubcommand(name string, args []string) (Command, error) {
	switch name {
	case "init":
		return parseInit(args)
	case "logs":
		return parseLogs(args)
	case "index":
		return parseIndex(args)
	case "describe":
		return parseDescribe(args)
	case "remember":
		return parseRemember(args)
	case "consolidate":
		return parseConsolidate(args)
	case "recall":
		return parseRecall(args)
	case "list":
		return parseList(args)
	case "show":
		return parseShow(args)
	case "forget":
		return parseForget(args)
	default:
		return nil, apperr.InvalidArguments("Unknown subcommand %s.", name)
	}
}

func parseLogs(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, apperr.InvalidArguments("logs requires a subcommand: path, export, or clear.")
	}

	switch args[0] {
	case "path":
		if len(args[1:]) != 0 {
			return nil, apperr.InvalidArguments("logs path does not accept extra arguments.")
		}
		return LogsPathCommand{}, nil
	case "export":
		fs := newFlagSet("logs export")
		var output string
		fs.StringVar(&output, "output", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return nil, err
		}
		if output == "" {
			return nil, apperr.InvalidArguments("logs export requires --output <path>.")
		}
		if len(fs.Args()) != 0 {
			return nil, apperr.InvalidArguments("logs export does not accept positional arguments.")
		}
		return LogsExportCommand{Output: output}, nil
	case "clear":
		fs := newFlagSet("logs clear")
		var force bool
		fs.BoolVar(&force, "force", false, "")
		if err := fs.Parse(args[1:]); err != nil {
			return nil, err
		}
		if len(fs.Args()) != 0 {
			return nil, apperr.InvalidArguments("logs clear does not accept positional arguments.")
		}
		if !force {
			return nil, apperr.InvalidArguments("logs clear requires --force.")
		}
		return LogsClearCommand{Force: true}, nil
	default:
		return nil, apperr.InvalidArguments("Unknown logs subcommand %s.", args[0])
	}
}

func parseInit(args []string) (Command, error) {
	fs := newFlagSet("init")
	var force bool
	fs.BoolVar(&force, "force", false, "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 0 {
		return nil, apperr.InvalidArguments("init does not accept positional arguments.")
	}
	return InitCommand{Force: force}, nil
}

func parseIndex(args []string) (Command, error) {
	fs := newFlagSet("index")
	var cmd IndexCommand
	cmd.Depth = 3
	cmd.MaxFileSize = 1_048_576
	var include stringSlice
	var exclude stringSlice
	fs.IntVar(&cmd.Depth, "depth", 3, "")
	fs.Var(&include, "include", "")
	fs.Var(&exclude, "exclude", "")
	fs.BoolVar(&cmd.Refresh, "refresh", false, "")
	fs.BoolVar(&cmd.NoSummary, "no-summary", false, "")
	fs.Int64Var(&cmd.MaxFileSize, "max-file-size", 1_048_576, "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, apperr.InvalidArguments("index requires exactly one path argument.")
	}
	cmd.Path = fs.Args()[0]
	cmd.Include = include
	cmd.Exclude = exclude
	return cmd, nil
}

func parseDescribe(args []string) (Command, error) {
	fs := newFlagSet("describe")
	var cmd DescribeCommand
	cmd.Length = "medium"
	fs.BoolVar(&cmd.Refresh, "refresh", false, "")
	fs.StringVar(&cmd.Length, "length", "medium", "")
	fs.StringVar(&cmd.Focus, "focus", "", "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, apperr.InvalidArguments("describe requires exactly one file argument.")
	}
	if !oneOf(cmd.Length, "short", "medium", "long") {
		return nil, apperr.InvalidArguments("Invalid value for --length: %s.", cmd.Length)
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
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) > 1 {
		return nil, apperr.InvalidArguments("remember accepts at most one positional content argument.")
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
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 0 {
		return nil, apperr.InvalidArguments("consolidate does not accept positional arguments.")
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
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, apperr.InvalidArguments("recall requires exactly one query argument.")
	}
	if !oneOf(cmd.Tier, "short", "long", "all") {
		return nil, apperr.InvalidArguments("Invalid value for --tier: %s.", cmd.Tier)
	}
	cmd.Query = fs.Args()[0]
	cmd.Tags = tags
	return cmd, nil
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
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 0 {
		return nil, apperr.InvalidArguments("list does not accept positional arguments.")
	}
	if !oneOf(cmd.Tier, "short", "long", "all") {
		return nil, apperr.InvalidArguments("Invalid value for --tier: %s.", cmd.Tier)
	}
	if !oneOf(cmd.Order, "asc", "desc") {
		return nil, apperr.InvalidArguments("Invalid value for --order: %s.", cmd.Order)
	}
	cmd.Tags = tags
	return cmd, nil
}

func parseShow(args []string) (Command, error) {
	fs := newFlagSet("show")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, apperr.InvalidArguments("show requires exactly one id argument.")
	}
	return ShowCommand{ID: fs.Args()[0]}, nil
}

func parseForget(args []string) (Command, error) {
	fs := newFlagSet("forget")
	var cmd ForgetCommand
	fs.BoolVar(&cmd.Force, "force", false, "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(fs.Args()) != 1 {
		return nil, apperr.InvalidArguments("forget requires exactly one id argument.")
	}
	cmd.ID = fs.Args()[0]
	return cmd, nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
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
