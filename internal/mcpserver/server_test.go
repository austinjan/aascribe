package mcpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/austinjan/aascribe/internal/index"
	"github.com/austinjan/aascribe/internal/knowledge"
	"github.com/austinjan/aascribe/internal/mcpserver"
	"github.com/austinjan/aascribe/internal/projectask"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type authTransport struct{ base http.RoundTripper }

func (a authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+testToken)
	return a.base.RoundTrip(req)
}

type stubAsker struct{}

func (stubAsker) Ask(ctx context.Context, question string) (*projectask.Answer, error) {
	if question == "fail" {
		return nil, fmt.Errorf("no supporting evidence")
	}
	return &projectask.Answer{Status: "insufficient_evidence", Answer: "No evidence for " + question, Sources: []projectask.Citation{}}, nil
}
func connect(t *testing.T, handler http.Handler) *mcp.ClientSession {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	t.Cleanup(cancel)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: authTransport{base: http.DefaultTransport}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestMCPAuthenticationOriginsAndToolContract(t *testing.T) {
	handler, err := mcpserver.Handler(stubAsker{}, testToken, []string{"https://trusted.example"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, token, origin string
		want                int
	}{
		{"/mcp", "", "", 401}, {"/mcp", "bad", "", 401},
		{"/mcp", testToken, "https://evil.example", 403}, {"/unknown", testToken, "", 404},
	} {
		req := httptest.NewRequest("POST", tc.path, strings.NewReader("{}"))
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		req.Header.Set("Origin", tc.origin)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != tc.want {
			t.Fatalf("%+v: status %d", tc, recorder.Code)
		}
	}
	session := connect(t, handler)
	tools, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "ask_project" {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "ask_project", Arguments: map[string]any{"question": "How does auth work?"}})
	if err != nil || result.IsError || result.StructuredContent == nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "ask_project", Arguments: map[string]any{"question": "fail"}})
	if err != nil || !result.IsError {
		t.Fatalf("tool error=%+v err=%v", result, err)
	}
}

func TestLoopbackHostProtectionForTunnel(t *testing.T) {
	handler, err := mcpserver.Handler(stubAsker{}, testToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		host string
		want int
	}{{"knowledge.example.com", 403}, {"localhost:8080", 200}} {
		req := httptest.NewRequest("POST", "http://"+tc.host+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"tunnel-test","version":"1"}}}`))
		req.RemoteAddr = "127.0.0.1:12345"
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != tc.want {
			t.Fatalf("host %s: status %d body %s", tc.host, recorder.Code, recorder.Body.String())
		}
	}
}

func TestPowerShellSmokeClient(t *testing.T) {
	powershell, err := exec.LookPath("pwsh")
	if err != nil {
		powershell, err = exec.LookPath("powershell")
	}
	if err != nil {
		t.Skip("PowerShell unavailable")
	}
	handler, err := mcpserver.Handler(stubAsker{}, testToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	script, err := filepath.Abs("../../scripts/ask-project.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, powershell, "-NoProfile", "-File", script, "-Url", server.URL+"/mcp", "-Question", "測試專案")
	cmd.Env = append(os.Environ(), "AASCRIBE_MCP_TOKEN="+testToken)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("smoke client failed: %v %s", err, data)
	}
	var answer projectask.Answer
	if err := json.Unmarshal(data, &answer); err != nil {
		t.Fatalf("smoke output invalid: %v %s", err, data)
	}
	if answer.Status != "insufficient_evidence" || !strings.Contains(answer.Answer, "測試專案") {
		t.Fatalf("answer=%+v", answer)
	}
}

func TestIndexProjectMCP(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node required for runner configuration")
	}
	root := t.TempDir()
	catalog, err := knowledge.OpenForIndexing(root)
	if err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(t.TempDir(), "worker.mjs")
	if err := os.WriteFile(worker, []byte("process.exit(1)"), 0600); err != nil {
		t.Fatal(err)
	}
	runner, err := projectask.New(catalog, projectask.Config{Node: "node", Worker: worker, Provider: "fixture", Model: "fixture", Timeout: 10 * time.Second, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	handler, err := mcpserver.Handler(runner, testToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, handler)
	tools, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil || len(tools.Tools) != 2 {
		t.Fatalf("tools=%+v %v", tools, err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "index_project" && (tool.Annotations == nil || tool.Annotations.ReadOnlyHint) {
			t.Fatal("index tool incorrectly marked read-only")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "auth.md"), []byte("Token expires after 30 minutes."), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "index_project", Arguments: map[string]any{"refresh": true}})
	if err != nil || result.IsError {
		t.Fatalf("index=%+v %v", result, err)
	}
	data, _ := json.Marshal(result.StructuredContent)
	var indexed projectask.IndexResult
	if err := json.Unmarshal(data, &indexed); err != nil {
		t.Fatal(err)
	}
	if indexed.PublishedFiles != 1 || !indexed.Refresh || indexed.Status != "completed" {
		t.Fatalf("index=%+v", indexed)
	}
	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "index_project", Arguments: map[string]any{"depth": -1}})
	if err != nil || !result.IsError {
		t.Fatalf("invalid depth=%+v %v", result, err)
	}
}

func makeCatalog(t *testing.T) *knowledge.Catalog {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.md"), []byte("# Authentication\nAccess tokens expire in 30 minutes.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := index.Build(index.Options{Root: root, Depth: -1, MaxFileSize: 1024 * 1024, Summarizer: func(context.Context, string, string, string, string) (string, error) {
		return "Authentication token expiry", nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := knowledge.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close() })
	return catalog
}
func piWorker(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js unavailable; install Node and run npm ci in runtime/pi")
	}
	worker, err := filepath.Abs("../../runtime/pi/worker.mjs")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(worker), "node_modules", "@earendil-works", "pi-coding-agent", "package.json")); err != nil {
		t.Skip("pi dependencies unavailable; run npm ci in runtime/pi to enable real SDK integration tests")
	}
	return worker
}

// This exercises the REAL pi SDK/agent loop and REAL MCP SDK over HTTP.
// Only the paid model provider is replaced by a deterministic local SSE fixture.
func TestAskProjectRealPiAndMCP(t *testing.T) {
	worker := piWorker(t)
	t.Setenv("AASCRIBE_PI_API_KEY", "fixture-key")
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
			Messages []struct {
				Role    string
				Content any
			} `json:"messages"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
			http.Error(w, "invalid", 400)
			return
		}
		names := map[string]bool{}
		for _, tool := range body.Tools {
			names[tool.Function.Name] = true
		}
		if len(names) != 3 || !names["knowledge_list"] || !names["knowledge_search"] || !names["knowledge_read"] {
			t.Errorf("unexpected pi tools: %v", names)
		}
		step := calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		var delta map[string]any
		finish := "tool_calls"
		switch step {
		case 1:
			delta = toolDelta("search-1", "knowledge_search", `{"query":"authentication","limit":5}`)
		case 2:
			found := false
			for _, message := range body.Messages {
				if message.Role == "tool" && strings.Contains(fmt.Sprint(message.Content), "auth.md") {
					found = true
				}
			}
			if !found {
				t.Error("pi did not receive search result")
			}
			delta = toolDelta("read-1", "knowledge_read", `{"path":"auth.md"}`)
		default:
			found := false
			for _, message := range body.Messages {
				if message.Role == "tool" && strings.Contains(fmt.Sprint(message.Content), "30 minutes") {
					found = true
				}
			}
			if !found {
				t.Error("pi did not receive source content")
			}
			finish = "stop"
			delta = map[string]any{"role": "assistant", "content": `{"status":"answered","answer":"Access tokens expire in 30 minutes.","sources":[{"path":"auth.md","start_line":2,"end_line":2}]}`}
		}
		chunk := map[string]any{"id": "fixture", "object": "chat.completion.chunk", "created": 1, "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": nil}}}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		chunk["choices"] = []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}
		chunk["usage"] = map[string]int{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120}
		data, _ = json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
	}))
	t.Cleanup(provider.Close)
	modelsFile := filepath.Join(t.TempDir(), "models.json")
	models := map[string]any{"providers": map[string]any{"fixture": map[string]any{"baseUrl": provider.URL + "/v1", "api": "openai-completions", "apiKey": "fixture-key", "models": []any{map[string]any{"id": "fixture", "name": "fixture", "reasoning": false, "input": []string{"text"}, "contextWindow": 64000, "maxTokens": 4096, "cost": map[string]int{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0}}}}}}
	data, _ := json.Marshal(models)
	if err := os.WriteFile(modelsFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	runner, err := projectask.New(makeCatalog(t), projectask.Config{Node: "node", Worker: worker, Provider: "fixture", Model: "fixture", ModelsFile: modelsFile, Timeout: 30 * time.Second, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := mcpserver.Handler(runner, testToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, handler)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "ask_project", Arguments: map[string]any{"question": "When do access tokens expire?"}})
	if err != nil || result.IsError {
		t.Fatalf("ask result=%+v err=%v", result, err)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	var answer projectask.Answer
	if err := json.Unmarshal(encoded, &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Status != "answered" || len(answer.Sources) != 1 || !strings.HasPrefix(answer.Sources[0].Hash, "sha256:") || answer.Sources[0].StartLine != 2 || calls.Load() != 3 || answer.Usage.Turns != 3 {
		t.Fatalf("answer=%+v model calls=%d", answer, calls.Load())
	}
}
func toolDelta(id, name, arguments string) map[string]any {
	return map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": id, "type": "function", "function": map[string]any{"name": name, "arguments": arguments}}}}
}

func TestLivePiQuery(t *testing.T) {
	if os.Getenv("AASCRIBE_LIVE_PI_TEST") != "1" {
		t.Skip("set AASCRIBE_LIVE_PI_TEST=1 to test a paid real model against a synthetic project")
	}
	worker := piWorker(t)
	runner, err := projectask.New(makeCatalog(t), projectask.Config{Node: "node", Worker: worker, Provider: "google", Model: "gemini-2.5-flash", Timeout: 90 * time.Second, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := mcpserver.Handler(runner, testToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, handler)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "ask_project", Arguments: map[string]any{"question": "When do access tokens expire? Read the source and cite the relevant line."}})
	if err != nil || result.IsError {
		t.Fatalf("live ask result=%+v err=%v", result, err)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	var answer projectask.Answer
	if err := json.Unmarshal(encoded, &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Status != "answered" || len(answer.Sources) == 0 || !strings.Contains(answer.Answer, "30") {
		t.Fatalf("live answer=%+v", answer)
	}
	t.Logf("Live pi answered with %d verified citations, %d model turns, %d tokens", len(answer.Sources), answer.Usage.Turns, answer.Usage.TotalTokens)
}
