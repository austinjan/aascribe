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
	Store   string
	Format  Format
	Quiet   bool
	Verbose bool
	Help    bool
	Version bool
	Command Command
}

type Command interface {
	Name() string
}

type InitCommand struct {
	Force bool
}

func (c InitCommand) Name() string { return "init" }

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
aascribe - Memory-first local CLI for project recall

Usage:
  aascribe [global flags] <command> [command flags]

Global flags:
  --store <path>       Path to the memory store root
  --format json|text   Output format (default: json)
  --quiet, -q          Suppress all command output
  --verbose, -v        Enable verbose mode
  --help, -h           Show help
  --version            Show version

Commands:
  init
  index
  describe
  remember
  consolidate
  recall
  list
  show
  forget
`)
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
