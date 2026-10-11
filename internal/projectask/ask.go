// Package projectask runs an isolated pi agent with bounded, catalog-only tools.
package projectask

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/austinjan/aascribe/internal/index"
	"github.com/austinjan/aascribe/internal/knowledge"
)

type Config struct {
	Node         string
	Worker       string
	Provider     string
	Model        string
	ModelsFile   string
	AuthFile     string
	CodexCommand string
	Timeout      time.Duration
	Concurrency  int
}
type Citation struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Hash      string `json:"hash"`
}
type Usage struct {
	Turns            int     `json:"turns"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}
type Answer struct {
	Status   string     `json:"status"`
	Answer   string     `json:"answer"`
	Sources  []Citation `json:"sources"`
	Provider string     `json:"provider"`
	Model    string     `json:"model"`
	Usage    Usage      `json:"usage"`
	Notice   string     `json:"notice"`
}
type Runner struct {
	state   sync.RWMutex
	catalog *knowledge.Catalog
	config  Config
	slots   chan struct{}
}

func New(catalog *knowledge.Catalog, config Config) (*Runner, error) {
	if catalog == nil {
		return nil, fmt.Errorf("knowledge catalog is required")
	}
	if config.Timeout < time.Second || config.Timeout > 5*time.Minute {
		return nil, fmt.Errorf("pi timeout must be between 1s and 5m")
	}
	if config.Concurrency < 1 || config.Concurrency > 8 {
		return nil, fmt.Errorf("pi concurrency must be between 1 and 8")
	}
	if config.Provider == "" || config.Model == "" {
		return nil, fmt.Errorf("pi provider and model are required")
	}
	node, err := exec.LookPath(config.Node)
	if err != nil {
		return nil, fmt.Errorf("Node.js is required: %w", err)
	}
	config.Node = node
	config.Worker, err = filepath.Abs(config.Worker)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(config.Worker)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("pi worker is missing: install runtime/pi dependencies and specify --pi-worker")
	}
	if config.ModelsFile != "" {
		config.ModelsFile, err = filepath.Abs(config.ModelsFile)
		if err != nil {
			return nil, err
		}
		if _, err = os.Stat(config.ModelsFile); err != nil {
			return nil, fmt.Errorf("pi models file is unavailable")
		}
	}
	if config.AuthFile != "" {
		if config.Provider != "openai-codex" {
			return nil, fmt.Errorf("pi auth file is supported only for openai-codex")
		}
		config.AuthFile, err = filepath.Abs(config.AuthFile)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(config.AuthFile)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("pi auth file is unavailable; complete the subscription login first")
		}
	}
	if config.CodexCommand != "" {
		if config.AuthFile == "" || config.Provider != "openai-codex" {
			return nil, fmt.Errorf("Codex CLI auth requires openai-codex and an explicit auth file")
		}
		config.CodexCommand, err = exec.LookPath(config.CodexCommand)
		if err != nil {
			return nil, fmt.Errorf("Codex CLI is unavailable")
		}
	}
	return &Runner{catalog: catalog, config: config, slots: make(chan struct{}, config.Concurrency)}, nil
}

type workerRequest struct {
	Question     string `json:"question"`
	BridgeURL    string `json:"bridge_url"`
	BridgeToken  string `json:"bridge_token"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	ModelsFile   string `json:"models_file,omitempty"`
	AuthFile     string `json:"auth_file,omitempty"`
	CodexCommand string `json:"codex_command,omitempty"`
}

func (r *Runner) Ask(parent context.Context, question string) (*Answer, error) {
	if strings.TrimSpace(question) == "" || len(question) > 8000 {
		return nil, fmt.Errorf("question must contain 1 to 8000 bytes")
	}
	if !r.state.TryRLock() {
		return nil, fmt.Errorf("project index is updating; retry later")
	}
	defer r.state.RUnlock()
	if r.catalog.Count() == 0 {
		return nil, fmt.Errorf("project has no published sources; call index_project first")
	}
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	default:
		return nil, fmt.Errorf("project query capacity reached; retry later")
	}
	ctx, cancel := context.WithTimeout(parent, r.config.Timeout)
	defer cancel()
	started := time.Now()
	slog.Info("project query started", "provider", r.config.Provider, "model", r.config.Model)
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(random)
	bridge := &bridge{catalog: r.catalog, token: token}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("cannot start private knowledge bridge")
	}
	server := &http.Server{Handler: bridge, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, MaxHeaderBytes: 8192}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	input, err := json.Marshal(workerRequest{Question: question, BridgeURL: "http://" + listener.Addr().String() + "/", BridgeToken: token, Provider: r.config.Provider, Model: r.config.Model, ModelsFile: r.config.ModelsFile, AuthFile: r.config.AuthFile, CodexCommand: r.config.CodexCommand})
	if err != nil {
		return nil, err
	}
	// No project-local config, session files or extension discovery runs here.
	workDir, err := os.MkdirTemp("", "aascribe-pi-")
	if err != nil {
		return nil, fmt.Errorf("cannot create pi working directory")
	}
	defer os.RemoveAll(workDir)
	cmd := exec.CommandContext(ctx, r.config.Node, r.config.Worker)
	cmd.Dir = workDir
	cmd.Env = workerEnvironment()
	cmd.Stdin = bytes.NewReader(input)
	var stdout cappedBuffer
	stdout.limit = 64 * 1024
	cmd.Stdout = &stdout
	// The worker prints only a sanitized diagnostic. Never return raw provider logs.
	var stderr cappedBuffer
	stderr.limit = 4096
	cmd.Stderr = &stderr
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		slog.Warn("pi query failed", "duration_ms", time.Since(started).Milliseconds(), "timed_out_or_canceled", ctx.Err() != nil)
		if ctx.Err() != nil {
			return nil, fmt.Errorf("project query canceled or timed out: %w", ctx.Err())
		}
		switch diagnostic := strings.TrimSpace(string(stderr.Bytes())); diagnostic {
		case "pi token budget exceeded", "pi model turn budget exceeded", "pi model request failed", "pi returned invalid answer JSON", "pi returned an invalid answer shape", "pi returned an invalid citation", "pi answered without source citations":
			return nil, fmt.Errorf("%s", diagnostic)
		}
		return nil, fmt.Errorf("pi worker failed; check installed dependencies, model credentials and query budgets")
	}
	if stdout.exceeded {
		return nil, fmt.Errorf("pi answer exceeds output budget")
	}
	var answer Answer
	if err := json.Unmarshal(stdout.Bytes(), &answer); err != nil {
		return nil, fmt.Errorf("pi returned invalid answer JSON")
	}
	if err := bridge.validate(&answer); err != nil {
		return nil, err
	}
	answer.Provider, answer.Model = r.config.Provider, r.config.Model
	answer.Notice = "Citations were verified against source chunks read by pi. This does not prove every statement. Call index_project to refresh changed sources."
	if r.config.AuthFile != "" {
		answer.Notice += " Using Codex subscription OAuth; estimated_cost_usd is a catalog estimate, not a subscription charge."
	}
	slog.Info("project query finished", "duration_ms", time.Since(started).Milliseconds(), "status", answer.Status, "sources", len(answer.Sources), "turns", answer.Usage.Turns, "tokens", answer.Usage.TotalTokens)
	return &answer, nil
}

func (r *Runner) Close() error {
	r.state.Lock()
	defer r.state.Unlock()
	return r.catalog.Close()
}

type IndexInput struct {
	Refresh bool `json:"refresh,omitempty" jsonschema:"Rebuild all summaries even for unchanged files; default false reuses unchanged index entries"`
	Depth   *int `json:"depth,omitempty" jsonschema:"Maximum directory depth from 0 to 64; default 20"`
}
type IndexResult struct {
	Status         string `json:"status"`
	PublishedFiles int    `json:"published_files"`
	Refresh        bool   `json:"refresh"`
	Depth          int    `json:"depth"`
	DurationMS     int64  `json:"duration_ms"`
	SummaryMode    string `json:"summary_mode"`
	Notice         string `json:"notice"`
}

// Index updates only the server-configured root and swaps catalogs while no queries are active.
func (r *Runner) Index(parent context.Context, input IndexInput) (*IndexResult, error) {
	depth := 20
	if input.Depth != nil {
		depth = *input.Depth
	}
	if depth < 0 || depth > 64 {
		return nil, fmt.Errorf("depth must be between 0 and 64")
	}
	if !r.state.TryLock() {
		return nil, fmt.Errorf("project is busy querying or indexing; retry later")
	}
	defer r.state.Unlock()
	ctx, cancel := context.WithTimeout(parent, r.config.Timeout)
	defer cancel()
	started := time.Now()
	slog.Info("project indexing started", "refresh", input.Refresh, "depth", depth)
	var warnings bytes.Buffer
	root := r.catalog.RootPath()
	tree, err := index.Build(index.Options{Context: ctx, Root: root, Depth: depth, Refresh: input.Refresh, MaxFileSize: 1024 * 1024, MaxConcurrency: 2, RejectSymlinks: true,
		Exclude: []string{".env*", ".aws", ".ssh", ".codex", ".pi"}, FailureThreshold: 1, FailureNoticeWriter: &warnings})
	if err != nil {
		return nil, fmt.Errorf("project indexing failed; retry index_project: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("project indexing canceled or timed out: %w", err)
	}
	paths := []string{}
	var collect func(index.IndexedPathNode)
	collect = func(node index.IndexedPathNode) {
		if node.Type == "file" && node.Summary != "" {
			_, relative, ok := strings.Cut(filepath.ToSlash(node.Path), "/")
			if ok {
				paths = append(paths, relative)
			}
		}
		for _, child := range node.Children {
			collect(child)
		}
	}
	collect(tree.Tree)
	catalog, err := knowledge.OpenSelection(root, paths)
	if err != nil {
		return nil, fmt.Errorf("index written but reload failed; retry index_project: %w", err)
	}
	old := r.catalog
	r.catalog = catalog
	_ = old.Close()
	status := "completed"
	notice := "Index updated and loaded; ask_project uses it immediately. New summaries are local extractive summaries; unchanged summaries are reused unless refresh=true."
	if warnings.Len() != 0 {
		status = "completed_with_warnings"
		notice += " Some files or directories failed indexing and were omitted."
	}
	slog.Info("project indexing finished", "status", status, "files", catalog.Count(), "duration_ms", time.Since(started).Milliseconds())
	return &IndexResult{Status: status, PublishedFiles: catalog.Count(), Refresh: input.Refresh, Depth: depth, DurationMS: time.Since(started).Milliseconds(), SummaryMode: "extractive", Notice: notice}, nil
}

func workerEnvironment() []string {
	names := []string{"PATH", "SystemRoot", "WINDIR", "TEMP", "TMP", "HOME", "USERPROFILE", "LOCALAPPDATA", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "AASCRIBE_PI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY"}
	env := []string{}
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

type cappedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	n := len(data)
	remaining := b.limit - b.Len()
	if n > remaining {
		b.exceeded = true
		data = data[:remaining]
	}
	_, _ = b.Buffer.Write(data)
	return n, nil
}

type bridge struct {
	catalog    *knowledge.Catalog
	token      string
	mu         sync.Mutex
	calls      int
	characters int
	reads      []knowledge.Chunk
}

func (b *bridge) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actual := sha256.Sum256([]byte(req.Header.Get("Authorization")))
	expected := sha256.Sum256([]byte("Bearer " + b.token))
	if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
		writeError(w, 401, "unauthorized")
		return
	}
	if req.Method != http.MethodPost {
		writeError(w, 405, "POST required")
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	if b.calls > 24 {
		writeError(w, 429, "knowledge tool budget exceeded")
		return
	}
	req.Body = http.MaxBytesReader(w, req.Body, 8192)
	var input struct {
		Query  string `json:"query"`
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid tool arguments")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeError(w, 400, "expected one JSON object")
		return
	}
	var result any
	var err error
	switch req.URL.Path {
	case "/knowledge_search":
		result, err = b.catalog.Search(req.Context(), input.Query, input.Limit)
	case "/knowledge_list":
		if input.Limit == 0 {
			input.Limit = 20
		}
		result, err = b.catalog.List(input.Offset, input.Limit)
	case "/knowledge_read":
		var chunk *knowledge.Chunk
		chunk, err = b.catalog.Read(input.Path, input.Offset, input.Limit)
		if err == nil {
			result = chunk
		}
	default:
		writeError(w, 404, "unknown knowledge tool")
		return
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if sources, ok := result.([]knowledge.Source); ok {
		for i := range sources {
			text := []rune(sources[i].Summary)
			if len(text) > 2000 {
				sources[i].Summary = string(text[:2000]) + "…"
			}
		}
	}
	data, err := json.Marshal(result)
	if err != nil {
		writeError(w, 500, "cannot serialize result")
		return
	}
	b.characters += len(data)
	if b.characters > 128*1024 {
		writeError(w, 429, "knowledge context budget exceeded")
		return
	}
	if chunk, ok := result.(*knowledge.Chunk); ok && chunk.Text != "" {
		b.reads = append(b.reads, *chunk)
	}
	_, _ = w.Write(data)
}
func writeError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func (b *bridge) validate(answer *Answer) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if (answer.Status != "answered" && answer.Status != "insufficient_evidence") || strings.TrimSpace(answer.Answer) == "" || len([]rune(answer.Answer)) > 6000 || len(answer.Sources) > 24 {
		return fmt.Errorf("pi returned invalid answer shape")
	}
	if answer.Status == "answered" && len(answer.Sources) == 0 {
		return fmt.Errorf("pi answered without verified sources")
	}
	for i := range answer.Sources {
		citation := &answer.Sources[i]
		if citation.StartLine < 1 || citation.EndLine < citation.StartLine {
			return fmt.Errorf("pi returned invalid citation range")
		}
		matched := false
		for _, read := range b.reads {
			if citation.Path == read.Path && citation.StartLine >= read.StartLine && citation.EndLine <= read.EndLine {
				// Re-check the current hash before returning the answer, so edits during a query fail closed.
				if _, err := b.catalog.Read(read.Path, 0, 1); err != nil {
					return fmt.Errorf("cited source changed during query; re-index and restart")
				}
				citation.Hash = read.Hash
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("pi cited a source range it did not read")
		}
	}
	if answer.Sources == nil {
		answer.Sources = []Citation{}
	}
	return nil
}
