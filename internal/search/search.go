package search

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Options struct {
	Query        string
	Root         string
	Engine       string
	IgnoreCase   bool
	FixedStrings bool
	Glob         []string
	MaxCount     int
}

type Result struct {
	Query     string  `json:"query"`
	Root      string  `json:"root"`
	Engine    string  `json:"engine"`
	MatchCnt  int     `json:"match_count"`
	Truncated bool    `json:"truncated"`
	Matches   []Match `json:"matches"`
}

type Match struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column,omitempty"`
	Text   string `json:"text"`
}

func Run(opts Options) (*Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Engine == "" {
		opts.Engine = "auto"
	}
	if opts.MaxCount == 0 {
		opts.MaxCount = 100
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(root); err != nil {
		return nil, err
	} else if !info.IsDir() {
		root = filepath.Dir(root)
	}
	opts.Root = root

	engines := []string{opts.Engine}
	if opts.Engine == "auto" {
		engines = []string{"rg", "git-grep", "grep", "builtin"}
	}
	var lastErr error
	for _, engine := range engines {
		result, err := runEngine(engine, opts)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, errEngineUnavailable) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

var errEngineUnavailable = errors.New("search engine unavailable")

func runEngine(engine string, opts Options) (*Result, error) {
	switch engine {
	case "rg":
		if _, err := exec.LookPath("rg"); err != nil {
			return nil, errEngineUnavailable
		}
		return runRG(opts)
	case "git-grep":
		if _, err := exec.LookPath("git"); err != nil {
			return nil, errEngineUnavailable
		}
		if err := exec.Command("git", "-C", opts.Root, "rev-parse", "--is-inside-work-tree").Run(); err != nil {
			return nil, errEngineUnavailable
		}
		return runGitGrep(opts)
	case "grep":
		if _, err := exec.LookPath("grep"); err != nil {
			return nil, errEngineUnavailable
		}
		return runGrep(opts)
	case "builtin":
		return runBuiltin(opts)
	default:
		return nil, errEngineUnavailable
	}
}

func runRG(opts Options) (*Result, error) {
	args := []string{"--json", "--line-number", "--column"}
	if opts.IgnoreCase {
		args = append(args, "--ignore-case")
	}
	if opts.FixedStrings {
		args = append(args, "--fixed-strings")
	}
	for _, glob := range opts.Glob {
		args = append(args, "--glob", glob)
	}
	args = append(args, "--", opts.Query, opts.Root)
	cmd := exec.Command("rg", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return emptyResult(opts, "rg"), nil
		}
		return nil, err
	}
	result := emptyResult(opts, "rg")
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		var event struct {
			Type string `json:"type"`
			Data struct {
				Path struct {
					Text string `json:"text"`
				} `json:"path"`
				Lines struct {
					Text string `json:"text"`
				} `json:"lines"`
				LineNumber int `json:"line_number"`
				Submatches []struct {
					Start int `json:"start"`
				} `json:"submatches"`
			} `json:"data"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || event.Type != "match" {
			continue
		}
		column := 0
		if len(event.Data.Submatches) > 0 {
			column = event.Data.Submatches[0].Start + 1
		}
		addMatch(result, opts, Match{
			Path:   displayPath(opts.Root, event.Data.Path.Text),
			Line:   event.Data.LineNumber,
			Column: column,
			Text:   strings.TrimRight(event.Data.Lines.Text, "\r\n"),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func runGitGrep(opts Options) (*Result, error) {
	args := []string{"-C", opts.Root, "grep", "-n", "--column"}
	if opts.IgnoreCase {
		args = append(args, "-i")
	}
	if opts.FixedStrings {
		args = append(args, "-F")
	}
	args = append(args, "--", opts.Query, "--")
	if len(opts.Glob) > 0 {
		for _, glob := range opts.Glob {
			args = append(args, glob)
		}
	} else {
		args = append(args, ".")
	}
	cmd := exec.Command("git", args...)
	return parseColonLines(opts, "git-grep", cmd)
}

func runGrep(opts Options) (*Result, error) {
	args := []string{"-R", "-I", "-n"}
	if opts.IgnoreCase {
		args = append(args, "-i")
	}
	if opts.FixedStrings {
		args = append(args, "-F")
	}
	for _, glob := range opts.Glob {
		args = append(args, "--include", glob)
	}
	args = append(args, "--", opts.Query, opts.Root)
	cmd := exec.Command("grep", args...)
	return parseColonLines(opts, "grep", cmd)
}

func parseColonLines(opts Options, engine string, cmd *exec.Cmd) (*Result, error) {
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return emptyResult(opts, engine), nil
		}
		return nil, err
	}
	result := emptyResult(opts, engine)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 4)
		if len(parts) < 3 {
			continue
		}
		line := atoi(parts[1])
		column := 0
		text := ""
		if len(parts) == 4 {
			column = atoi(parts[2])
			text = parts[3]
		} else {
			text = parts[2]
			column = firstColumn(text, opts)
		}
		addMatch(result, opts, Match{
			Path:   displayPath(opts.Root, parts[0]),
			Line:   line,
			Column: column,
			Text:   text,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func runBuiltin(opts Options) (*Result, error) {
	result := emptyResult(opts, "builtin")
	matcher, err := buildMatcher(opts)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(opts.Root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel := displayPath(opts.Root, path)
		if len(opts.Glob) > 0 && !matchesAnyGlob(rel, opts.Glob) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || bytes.Contains(data, []byte{0}) {
			return nil
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			column := matcher(line)
			if column <= 0 {
				continue
			}
			addMatch(result, opts, Match{Path: rel, Line: lineNo, Column: column, Text: line})
			if result.Truncated {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func buildMatcher(opts Options) (func(string) int, error) {
	query := opts.Query
	if opts.FixedStrings {
		if opts.IgnoreCase {
			query = strings.ToLower(query)
			return func(line string) int {
				index := strings.Index(strings.ToLower(line), query)
				return index + 1
			}, nil
		}
		return func(line string) int {
			index := strings.Index(line, query)
			return index + 1
		}, nil
	}
	if opts.IgnoreCase {
		query = "(?i)" + query
	}
	re, err := regexp.Compile(query)
	if err != nil {
		return nil, err
	}
	return func(line string) int {
		index := re.FindStringIndex(line)
		if index == nil {
			return 0
		}
		return index[0] + 1
	}, nil
}

func addMatch(result *Result, opts Options, match Match) {
	if opts.MaxCount >= 0 && len(result.Matches) >= opts.MaxCount {
		result.Truncated = true
		return
	}
	result.Matches = append(result.Matches, match)
	result.MatchCnt = len(result.Matches)
}

func emptyResult(opts Options, engine string) *Result {
	return &Result{
		Query:   opts.Query,
		Root:    filepath.ToSlash(opts.Root),
		Engine:  engine,
		Matches: []Match{},
	}
}

func displayPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func matchesAnyGlob(path string, globs []string) bool {
	base := filepath.Base(path)
	for _, glob := range globs {
		if ok, _ := filepath.Match(glob, path); ok {
			return true
		}
		if ok, _ := filepath.Match(glob, base); ok {
			return true
		}
	}
	return false
}

func firstColumn(text string, opts Options) int {
	matcher, err := buildMatcher(opts)
	if err != nil {
		return 0
	}
	return matcher(text)
}

func atoi(value string) int {
	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return n
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
