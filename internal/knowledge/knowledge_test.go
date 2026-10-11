package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/austinjan/aascribe/internal/index"
)

func testCatalog(t *testing.T) (*Catalog, string) {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{"auth.md": "登入驗證\nToken expires in 30 minutes.\n", ".env": "SECRET=hidden", "unpublished.md": "not indexed"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := index.Build(index.Options{Root: root, Depth: -1, MaxFileSize: 1024 * 1024, Exclude: []string{"unpublished.md"}, Summarizer: func(context.Context, string, string, string, string) (string, error) {
		return "Login 登入 token authentication", nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	c, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, root
}

func TestSearchReadAndFreshness(t *testing.T) {
	c, root := testCatalog(t)
	sources, err := c.Search(context.Background(), "登入 TOKEN", 10)
	if err != nil || len(sources) != 1 || sources[0].Path != "auth.md" || sources[0].Score != 2 {
		t.Fatalf("search = %+v, %v", sources, err)
	}
	chunk, err := c.Read("auth.md", 0, 5)
	if err != nil || chunk.Text != "登入驗證\n" || chunk.StartLine != 1 || chunk.EndLine != 1 || chunk.NextOffset != 5 || !chunk.Truncated {
		t.Fatalf("chunk = %+v, %v", chunk, err)
	}
	next, err := c.Read("auth.md", chunk.NextOffset, 8000)
	if err != nil || next.StartLine != 2 || next.EndLine != 2 || next.Truncated {
		t.Fatalf("next = %+v, %v", next, err)
	}
	if err := os.WriteFile(filepath.Join(root, "auth.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Read("auth.md", 0, 10); err == nil {
		t.Fatal("changed source was allowed")
	}
}

func TestUnpublishedAndInvalidReads(t *testing.T) {
	c, _ := testCatalog(t)
	for _, path := range []string{"../auth.md", ".env", "unpublished.md", "C:/outside.txt", "auth.md/../auth.md"} {
		if _, err := c.Read(path, 0, 10); err == nil {
			t.Fatalf("allowed %q", path)
		}
	}
	for _, offset := range []int{-1, 1000} {
		if _, err := c.Read("auth.md", offset, 10); err == nil {
			t.Fatalf("allowed offset %d", offset)
		}
	}
	if _, err := c.Read("auth.md", 0, 16001); err == nil {
		t.Fatal("allowed oversized read")
	}
	if _, err := c.Search(context.Background(), " ", 1); err == nil {
		t.Fatal("allowed empty query")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Search(ctx, "token", 1); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestReadRejectsSymlinkOutsideRoot(t *testing.T) {
	c, root := testCatalog(t)
	outside := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "auth.md")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Skipf("symlink privileges unavailable: %v", err)
	}
	if _, err := c.Read("auth.md", 0, 10); err == nil {
		t.Fatal("read escaped published root")
	}
}
