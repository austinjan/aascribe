package search

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const maxScannerTokenSize = 1024 * 1024

type Options struct {
	Query        string
	Root         string
	Engine       string
	IgnoreCase   bool
	FixedStrings bool
	Glob         []string
	MaxCount     int
	files        []string
	partial      []PartialReason
	exclusions   []Exclusion
}

type Scope struct {
	Root          string   `json:"root"`
	TrackedOnly   bool     `json:"tracked_only"`
	Hidden        bool     `json:"hidden"`
	Ignored       bool     `json:"ignored"`
	IgnoreSources []string `json:"ignore_sources"`
}

type PartialReason struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type Exclusion struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type EngineFailure struct {
	Engine  string `json:"engine"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type Result struct {
	Query              string          `json:"query"`
	Root               string          `json:"root"`
	Engine             string          `json:"engine"`
	QueryMode          string          `json:"query_mode"`
	MatchUnit          string          `json:"match_unit"`
	ColumnUnit         string          `json:"column_unit"`
	Scope              Scope           `json:"scope"`
	MatchCnt           int             `json:"match_count"`
	ReturnedMatchLines int             `json:"returned_match_lines"`
	Truncated          bool            `json:"truncated"`
	Complete           bool            `json:"complete"`
	PartialReasons     []PartialReason `json:"partial_reasons"`
	Exclusions         []Exclusion     `json:"exclusions"`
	EngineFailures     []EngineFailure `json:"engine_failures"`
	Matches            []Match         `json:"matches"`
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
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	opts.Root = root
	opts.files, opts.partial, opts.exclusions, err = collectScopeFiles(root, opts.Glob)
	if err != nil {
		return nil, err
	}

	engines := []string{opts.Engine}
	if opts.Engine == "auto" {
		if opts.FixedStrings {
			engines = []string{"rg", "git-grep", "grep", "builtin"}
		} else {
			engines = []string{"rg", "builtin"}
		}
	}
	var lastErr error
	engineFailures := []EngineFailure{}
	for _, engine := range engines {
		result, err := runEngine(engine, opts)
		if err == nil {
			result.EngineFailures = engineFailures
			return result, nil
		}
		if opts.Engine != "auto" {
			return nil, err
		}
		if !errors.Is(err, errEngineUnavailable) {
			if engine == "builtin" {
				return nil, err
			}
			engineFailures = append(engineFailures, EngineFailure{Engine: engine, Kind: "execution_failed", Message: err.Error()})
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
		if err := exec.Command("git", "-C", searchCommandRoot(opts.Root), "rev-parse", "--is-inside-work-tree").Run(); err != nil {
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
	if len(opts.files) == 0 {
		return emptyResult(opts, "rg"), nil
	}
	args := []string{"--json", "--line-number", "--column"}
	if opts.IgnoreCase {
		args = append(args, "--ignore-case")
	}
	if opts.FixedStrings {
		args = append(args, "--fixed-strings")
	}
	args = append(args, "--", opts.Query)
	args = append(args, opts.files...)
	cmd := exec.Command("rg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	result := emptyResult(opts, "rg")
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxScannerTokenSize)
	stoppedAtLimit := false
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
		if result.Truncated {
			stoppedAtLimit = true
			_ = cmd.Process.Kill()
			break
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if stoppedAtLimit {
		return result, nil
	}
	if scanErr != nil {
		return nil, scanErr
	}
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return emptyResult(opts, "rg"), nil
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return nil, fmt.Errorf("rg execution failed: %s", message)
	}
	return result, nil
}

func runGitGrep(opts Options) (*Result, error) {
	if len(opts.files) == 0 {
		return emptyResult(opts, "git-grep"), nil
	}
	commandRoot := searchCommandRoot(opts.Root)
	args := []string{"-C", commandRoot, "grep", "--no-index", "-z", "-n", "--column"}
	if opts.IgnoreCase {
		args = append(args, "-i")
	}
	if opts.FixedStrings {
		args = append(args, "-F")
	}
	args = append(args, "--", opts.Query, "--")
	for _, path := range opts.files {
		rel, err := filepath.Rel(commandRoot, path)
		if err != nil {
			return nil, err
		}
		args = append(args, filepath.ToSlash(rel))
	}
	cmd := exec.Command("git", args...)
	return parseNullFilenameLines(opts, "git-grep", cmd, true)
}

func runGrep(opts Options) (*Result, error) {
	if len(opts.files) == 0 {
		return emptyResult(opts, "grep"), nil
	}
	args := []string{"-H", "-I", "-n", "--null"}
	if opts.IgnoreCase {
		args = append(args, "-i")
	}
	if opts.FixedStrings {
		args = append(args, "-F")
	}
	args = append(args, "--", opts.Query)
	args = append(args, opts.files...)
	cmd := exec.Command("grep", args...)
	return parseNullFilenameLines(opts, "grep", cmd, false)
}

func parseNullFilenameLines(opts Options, engine string, cmd *exec.Cmd, hasColumn bool) (*Result, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	result := emptyResult(opts, engine)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxScannerTokenSize)
	stoppedAtLimit := false
	for scanner.Scan() {
		fields := bytes.Split(scanner.Bytes(), []byte{0})
		if len(fields) < 2 {
			continue
		}
		path := string(fields[0])
		var line, column int
		var text string
		if hasColumn && len(fields) >= 4 {
			line = atoi(string(fields[1]))
			column = atoi(string(fields[2]))
			text = string(bytes.Join(fields[3:], []byte{0}))
			text = strings.TrimSuffix(text, "\n")
		} else {
			rest := string(bytes.Join(fields[1:], []byte{0}))
			partCount := 2
			if hasColumn {
				partCount = 3
			}
			parts := strings.SplitN(rest, ":", partCount)
			if len(parts) < partCount {
				continue
			}
			line = atoi(parts[0])
			if hasColumn {
				column = atoi(parts[1])
				text = parts[2]
			} else {
				text = parts[1]
				column = firstColumn(text, opts)
			}
		}
		addMatch(result, opts, Match{
			Path:   displayPath(opts.Root, path),
			Line:   line,
			Column: column,
			Text:   text,
		})
		if result.Truncated {
			stoppedAtLimit = true
			_ = cmd.Process.Kill()
			break
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if stoppedAtLimit {
		return result, nil
	}
	if scanErr != nil {
		return nil, scanErr
	}
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return emptyResult(opts, engine), nil
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return nil, fmt.Errorf("%s execution failed: %s", engine, message)
	}
	return result, nil
}

func runBuiltin(opts Options) (*Result, error) {
	result := emptyResult(opts, "builtin")
	matcher, err := buildMatcher(opts)
	if err != nil {
		return nil, err
	}
	for _, path := range opts.files {
		rel := displayPath(opts.Root, path)
		data, err := os.ReadFile(path)
		if err != nil || bytes.Contains(data, []byte{0}) {
			continue
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 64*1024), maxScannerTokenSize)
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
				return result, nil
			}
		}
		if err := scanner.Err(); err != nil {
			result.Complete = false
			result.PartialReasons = append(result.PartialReasons, PartialReason{Kind: "scanner_limit", Path: rel, Message: err.Error()})
		}
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
	result.ReturnedMatchLines = len(result.Matches)
}

func emptyResult(opts Options, engine string) *Result {
	queryMode := "regex"
	if opts.FixedStrings {
		queryMode = "fixed"
	}
	partial := append([]PartialReason{}, opts.partial...)
	exclusions := append([]Exclusion{}, opts.exclusions...)
	return &Result{
		Query:      opts.Query,
		Root:       filepath.ToSlash(opts.Root),
		Engine:     engine,
		QueryMode:  queryMode,
		MatchUnit:  "line",
		ColumnUnit: "byte",
		Scope: Scope{
			Root:          filepath.ToSlash(opts.Root),
			TrackedOnly:   false,
			Hidden:        false,
			Ignored:       false,
			IgnoreSources: []string{".gitignore", ".aaignore"},
		},
		Complete:       len(partial) == 0,
		PartialReasons: partial,
		Exclusions:     exclusions,
		EngineFailures: []EngineFailure{},
		Matches:        []Match{},
	}
}

type scopeIgnoreRule struct {
	base    string
	pattern string
}

func collectScopeFiles(root string, globs []string) ([]string, []PartialReason, []Exclusion, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, nil, err
	}
	if !info.IsDir() {
		if len(globs) > 0 && !matchesAnyGlob(filepath.Base(root), globs) {
			return []string{}, []PartialReason{}, []Exclusion{}, nil
		}
		binary, err := inspectCandidate(root)
		if err != nil {
			return []string{}, []PartialReason{{Kind: "unreadable", Path: filepath.ToSlash(filepath.Base(root)), Message: err.Error()}}, []Exclusion{}, nil
		}
		if binary {
			return []string{}, []PartialReason{}, []Exclusion{{Kind: "binary", Path: filepath.ToSlash(filepath.Base(root))}}, nil
		}
		return []string{root}, []PartialReason{}, []Exclusion{}, nil
	}

	rulesByDir := map[string][]scopeIgnoreRule{}
	files := []string{}
	partial := []PartialReason{}
	exclusions := []Exclusion{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			partial = append(partial, PartialReason{Kind: "unreadable", Path: displayPath(root, path), Message: walkErr.Error()})
			return nil
		}
		if path == root {
			rules, err := loadScopeIgnoreRules(path, nil)
			if err != nil {
				return err
			}
			rulesByDir[path] = rules
			return nil
		}

		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		parentRules := rulesByDir[filepath.Dir(path)]
		if scopePathIgnored(path, entry.Name(), parentRules) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			rules, err := loadScopeIgnoreRules(path, parentRules)
			if err != nil {
				return err
			}
			rulesByDir[path] = rules
			return nil
		}
		rel := displayPath(root, path)
		if len(globs) == 0 || matchesAnyGlob(rel, globs) {
			binary, err := inspectCandidate(path)
			if err != nil {
				partial = append(partial, PartialReason{Kind: "unreadable", Path: rel, Message: err.Error()})
				return nil
			}
			if binary {
				exclusions = append(exclusions, Exclusion{Kind: "binary", Path: rel})
				return nil
			}
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return files, partial, exclusions, nil
}

func inspectCandidate(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	buffer := make([]byte, 8192)
	n, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return bytes.Contains(buffer[:n], []byte{0}), nil
}

func loadScopeIgnoreRules(dir string, inherited []scopeIgnoreRule) ([]scopeIgnoreRule, error) {
	rules := append([]scopeIgnoreRule(nil), inherited...)
	for _, name := range []string{".gitignore", ".aaignore"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
				continue
			}
			line = filepath.ToSlash(strings.TrimPrefix(line, "/"))
			if line != "" {
				rules = append(rules, scopeIgnoreRule{base: dir, pattern: line})
			}
		}
	}
	return rules, nil
}

func scopePathIgnored(path, name string, rules []scopeIgnoreRule) bool {
	for _, rule := range rules {
		rel, err := filepath.Rel(rule.base, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		rel = filepath.ToSlash(rel)
		pattern := rule.pattern
		dirPattern := strings.TrimSuffix(pattern, "/")
		if dirPattern != "" && (rel == dirPattern || strings.HasPrefix(rel, dirPattern+"/")) {
			return true
		}
		if ok, _ := filepath.Match(pattern, name); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, rel); ok {
			return true
		}
		if !strings.Contains(pattern, "/") {
			for _, part := range strings.Split(rel, "/") {
				if ok, _ := filepath.Match(pattern, part); ok {
					return true
				}
			}
		}
	}
	return false
}

func searchCommandRoot(root string) string {
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		return filepath.Dir(root)
	}
	return root
}

func displayPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return filepath.ToSlash(filepath.Base(path))
		}
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
