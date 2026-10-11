package projectask

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/austinjan/aascribe/internal/index"
	"github.com/austinjan/aascribe/internal/knowledge"
)

func TestCitationMustMatchAReadChunk(t *testing.T) {
	b := &bridge{reads: []knowledge.Chunk{{Path: "auth.md", StartLine: 2, EndLine: 4, Hash: "hash"}}}
	for _, citation := range []Citation{{Path: "invented.md", StartLine: 2, EndLine: 4}, {Path: "auth.md", StartLine: 1, EndLine: 4}, {Path: "auth.md", StartLine: 2, EndLine: 5}} {
		answer := &Answer{Status: "answered", Answer: "Claim", Sources: []Citation{citation}}
		if err := b.validate(answer); err == nil {
			t.Fatalf("allowed unsupported citation: %+v", citation)
		}
	}
	if err := b.validate(&Answer{Status: "answered", Answer: "Claim"}); err == nil {
		t.Fatal("allowed uncited answer")
	}
	if err := b.validate(&Answer{Status: "insufficient_evidence", Answer: "No supporting sources found."}); err != nil {
		t.Fatal(err)
	}
}

func testRunner(t *testing.T) *Runner {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js required for process isolation tests")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.md"), []byte("Authentication\nToken expiry: 30 minutes.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := index.Build(index.Options{Root: root, Depth: -1, MaxFileSize: 1024 * 1024}); err != nil {
		t.Fatal(err)
	}
	catalog, err := knowledge.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close() })
	worker := filepath.Join(t.TempDir(), "worker.mjs")
	script := `let data=''; for await (const chunk of process.stdin) data+=chunk;
 const request=JSON.parse(data);
 if(request.question==='hang'){setInterval(()=>{},1000);} else {
 if(request.question==='fail'){process.exit(1);}
 if(request.question==='huge'){process.stdout.write('x'.repeat(70000));process.exit(0);}
 if(request.question==='read'){
  const res=await fetch(new URL('knowledge_read',request.bridge_url),{method:'POST',headers:{Authorization:'Bearer '+request.bridge_token},body:JSON.stringify({path:'auth.md'})});
  if(!res.ok) process.exit(1);
 }
 process.stdout.write(JSON.stringify({status:'answered',answer:'Tokens expire in 30 minutes.',sources:[{path:request.question==='forged'?'invented.md':'auth.md',start_line:2,end_line:2}]}));
 }`
	if err := os.WriteFile(worker, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	runner, err := New(catalog, Config{Node: "node", Worker: worker, Provider: "fixture", Model: "fixture", Timeout: time.Second, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func TestRunnerRejectsInventedUnseenAndOversizedAnswers(t *testing.T) {
	runner := testRunner(t)
	for _, question := range []string{"forged", "unseen", "huge", "fail"} {
		if _, err := runner.Ask(context.Background(), question); err == nil {
			t.Fatalf("allowed %s", question)
		}
	}
	answer, err := runner.Ask(context.Background(), "read")
	if err != nil || len(answer.Sources) != 1 || !strings.HasPrefix(answer.Sources[0].Hash, "sha256:") {
		t.Fatalf("answer=%+v err=%v", answer, err)
	}
}

func TestRunnerTimeoutCancellationAndCapacity(t *testing.T) {
	runner := testRunner(t)
	started := time.Now()
	if _, err := runner.Ask(context.Background(), "hang"); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout err=%v", err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("worker was not promptly terminated")
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := runner.Ask(ctx, "hang"); result <- err }()
	deadline := time.Now().Add(time.Second)
	for len(runner.slots) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if _, err := runner.Index(context.Background(), IndexInput{}); err == nil {
		t.Fatal("indexing was allowed during a query")
	}
	if _, err := runner.Ask(context.Background(), "read"); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("capacity err=%v", err)
	}
	cancel()
	if err := <-result; err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("cancellation err=%v", err)
	}
	// A canceled query releases its capacity slot.
	if _, err := runner.Ask(context.Background(), "read"); err != nil {
		t.Fatalf("capacity not released: %v", err)
	}
}

func TestIndexCreatesUpdatesAndRemovesPublishedSources(t *testing.T) {
	runner := testRunner(t)
	root := runner.catalog.RootPath()
	if err := os.Remove(filepath.Join(root, ".aascribe_index_meta.json")); err != nil {
		t.Fatal(err)
	}
	empty, err := knowledge.OpenForIndexing(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = runner.catalog.Close()
	runner.catalog = empty
	t.Cleanup(func() { _ = runner.Close() })
	if _, err := runner.Ask(context.Background(), "read"); err == nil || !strings.Contains(err.Error(), "index_project") {
		t.Fatalf("unindexed query = %v", err)
	}
	result, err := runner.Index(context.Background(), IndexInput{})
	if err != nil || result.PublishedFiles != 1 || result.Status != "completed" {
		t.Fatalf("create = %+v %v", result, err)
	}
	if _, err := runner.Ask(context.Background(), "read"); err != nil {
		t.Fatalf("first index not available to query: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "auth.md"), []byte("Authentication\nToken expiry: 45 minutes.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = runner.Index(context.Background(), IndexInput{Refresh: true})
	if err != nil || !result.Refresh {
		t.Fatalf("update = %+v %v", result, err)
	}
	chunk, err := runner.catalog.Read("auth.md", 0, 8000)
	if err != nil || !strings.Contains(chunk.Text, "45 minutes") {
		t.Fatalf("new source = %+v %v", chunk, err)
	}
	if err := os.Remove(filepath.Join(root, "auth.md")); err != nil {
		t.Fatal(err)
	}
	result, err = runner.Index(context.Background(), IndexInput{})
	if err != nil || result.PublishedFiles != 0 {
		t.Fatalf("deleted source = %+v %v", result, err)
	}
}

func TestIndexExcludesOldMetadataAndPreservesCatalogOnCancellation(t *testing.T) {
	runner := testRunner(t)
	root := runner.catalog.RootPath()
	t.Cleanup(func() { _ = runner.Close() })
	child := filepath.Join(root, "private")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "old.md"), []byte("Old private source."), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Index(context.Background(), IndexInput{}); err != nil {
		t.Fatal(err)
	}
	if runner.catalog.Count() != 2 {
		t.Fatal("child source was not indexed")
	}
	if err := os.WriteFile(filepath.Join(root, ".aaignore"), []byte("private\n.aaignore\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := runner.Index(context.Background(), IndexInput{})
	if err != nil || result.PublishedFiles != 1 {
		t.Fatalf("ignored old metadata = %+v %v", result, err)
	}
	if _, err := runner.catalog.Read("private/old.md", 0, 10); err == nil {
		t.Fatal("ignored source remained published")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := runner.catalog
	if _, err := runner.Index(ctx, IndexInput{Refresh: true}); err == nil {
		t.Fatal("index ignored cancellation")
	}
	if runner.catalog != before {
		t.Fatal("cancellation replaced the catalog")
	}
	depth := -1
	if _, err := runner.Index(context.Background(), IndexInput{Depth: &depth}); err == nil {
		t.Fatal("accepted invalid depth")
	}
	runner.state.Lock()
	if _, err := runner.Ask(context.Background(), "read"); err == nil {
		t.Fatal("query started during catalog update")
	}
	runner.state.Unlock()
}

func TestBridgeAuthenticationAndBudget(t *testing.T) {
	b := &bridge{token: strings.Repeat("x", 64), calls: 24}
	req := httptest.NewRequest(http.MethodPost, "/knowledge_list", strings.NewReader("{}"))
	response := httptest.NewRecorder()
	b.ServeHTTP(response, req)
	if response.Code != 401 {
		t.Fatalf("unauthorized = %d", response.Code)
	}
	req.Header.Set("Authorization", "Bearer "+b.token)
	response = httptest.NewRecorder()
	b.ServeHTTP(response, req)
	if response.Code != 429 {
		t.Fatalf("budget = %d", response.Code)
	}
}

func TestCappedOutput(t *testing.T) {
	buffer := cappedBuffer{limit: 4}
	n, err := buffer.Write([]byte("secret excessive output"))
	if err != nil || n != 23 || buffer.String() != "secr" || !buffer.exceeded {
		t.Fatalf("buffer=%+v n=%d err=%v", buffer, n, err)
	}
}
