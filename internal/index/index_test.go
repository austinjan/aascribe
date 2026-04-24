package index

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const fixtureRoot = "../../tests/index-fixtures"

func TestBuildReturnsIndexedTree(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "README.md"), "# Demo\n\nhello world\n")
	mustWriteFile(t, filepath.Join(root, "notes.txt"), "plain text note\n")
	mustWriteFile(t, filepath.Join(root, "index.html"), "<html><body>Hello</body></html>\n")
	mustWriteFile(t, filepath.Join(root, "settings.conf"), "port=8080\nhost=localhost\n")
	mustWriteFile(t, filepath.Join(root, "LICENSE"), "sample license text\n")

	result, err := Build(Options{
		Root:        root,
		Depth:       2,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if result.Root != root {
		t.Fatalf("expected root %q, got %q", root, result.Root)
	}
	if result.Tree.Type != "dir" {
		t.Fatalf("expected dir root, got %q", result.Tree.Type)
	}
	if result.Tree.Path != filepath.Base(root) {
		t.Fatalf("expected root path %q, got %q", filepath.Base(root), result.Tree.Path)
	}
	if len(result.Tree.Children) != 5 {
		t.Fatalf("expected 5 children, got %d", len(result.Tree.Children))
	}
	for _, child := range result.Tree.Children {
		if child.Type != "file" {
			t.Fatalf("expected file node, got %q", child.Type)
		}
		if child.Hash == "" {
			t.Fatalf("expected file hash to be populated")
		}
		if child.Summary == "" {
			t.Fatalf("expected supported text file to have a summary, got %#v", child)
		}
		if child.SummarizedAt == "" {
			t.Fatalf("expected supported text file to have summarized_at, got %#v", child)
		}
	}

	meta := readMetadataFile(t, root)
	if len(meta.Files) != 5 {
		t.Fatalf("expected 5 metadata file records, got %#v", meta.Files)
	}
	for _, file := range meta.Files {
		if file.Status != "ok" {
			t.Fatalf("expected ok status, got %#v", file)
		}
		if file.Summary == "" || file.SummarizedAt == "" {
			t.Fatalf("expected persisted summary fields, got %#v", file)
		}
		if file.ContentHash == "" || file.ModTime == "" || file.FileType == "" {
			t.Fatalf("expected persisted metadata fields, got %#v", file)
		}
	}
}

func TestBuildHonorsIncludeExcludeAndDepth(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "main.go"), "package main\n")
	mustWriteFile(t, filepath.Join(root, "README.md"), "# Demo\n")
	mustWriteFile(t, filepath.Join(root, "vendor", "ignored.go"), "package vendor\n")
	mustWriteFile(t, filepath.Join(root, "nested", "child.go"), "package nested\n")

	result, err := Build(Options{
		Root:        root,
		Depth:       0,
		Include:     []string{"*.go"},
		Exclude:     []string{"vendor"},
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if len(result.Tree.Children) != 1 {
		t.Fatalf("expected only one matching child at depth 0, got %d", len(result.Tree.Children))
	}
	if filepath.Base(result.Tree.Children[0].Path) != "main.go" {
		t.Fatalf("expected main.go, got %q", result.Tree.Children[0].Path)
	}
}

func TestBuildNoSummaryOmitsSummaryFields(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "main.go"), "package main\n")

	result, err := Build(Options{
		Root:        root,
		Depth:       1,
		NoSummary:   true,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if result.Tree.Summary != "" || result.Tree.SummarizedAt != "" {
		t.Fatalf("expected root summary fields to be empty, got %#v", result.Tree)
	}
	fileNode := result.Tree.Children[0]
	if fileNode.Summary != "" || fileNode.SummarizedAt != "" {
		t.Fatalf("expected file summary fields to be empty, got %#v", fileNode)
	}
}

func TestBuildReadsGitignoreAndAaignoreAsBlacklist(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".gitignore"), "ignored.txt\nbuild/\n")
	mustWriteFile(t, filepath.Join(root, ".aaignore"), "*.log\n!important.log\nsecret/\n")
	mustWriteFile(t, filepath.Join(root, "main.go"), "package main\n")
	mustWriteFile(t, filepath.Join(root, "ignored.txt"), "skip me\n")
	mustWriteFile(t, filepath.Join(root, "debug.log"), "skip log\n")
	mustWriteFile(t, filepath.Join(root, "important.log"), "still skipped in v1\n")
	mustWriteFile(t, filepath.Join(root, "build", "artifact.txt"), "skip dir\n")
	mustWriteFile(t, filepath.Join(root, "secret", "token.txt"), "skip secret dir\n")

	result, err := Build(Options{
		Root:        root,
		Depth:       2,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if len(result.Tree.Children) != 3 {
		t.Fatalf("expected .aaignore, .gitignore, and main.go only, got %#v", result.Tree.Children)
	}
	for _, child := range result.Tree.Children {
		base := filepath.Base(child.Path)
		if base == "ignored.txt" || base == "debug.log" || base == "important.log" || base == "build" || base == "secret" {
			t.Fatalf("expected %q to be excluded by ignore files, got %#v", base, child)
		}
	}
}

func TestBuildSummarizesTextFilesRegardlessOfExtension(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "main.go"), "package main\n")
	mustWriteFile(t, filepath.Join(root, "README.md"), "# Demo\n")
	mustWriteFile(t, filepath.Join(root, "env.sample"), "PORT=8080\n")
	mustWriteFile(t, filepath.Join(root, "Dockerfile"), "FROM alpine:3.20\n")

	result, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if len(result.Tree.Children) != 4 {
		t.Fatalf("expected four files, got %d", len(result.Tree.Children))
	}

	found := map[string]IndexedPathNode{}
	for _, child := range result.Tree.Children {
		found[filepath.Base(child.Path)] = child
		if child.Summary == "" || child.SummarizedAt == "" {
			t.Fatalf("expected text file to be summarized regardless of extension, got %#v", child)
		}
	}

	for _, name := range []string{"main.go", "README.md", "env.sample", "Dockerfile"} {
		if found[name].Path == "" {
			t.Fatalf("expected %s in result, got %#v", name, result.Tree.Children)
		}
	}

	meta := readMetadataFile(t, root)
	if len(meta.NotIndexedFiles) != 0 {
		t.Fatalf("expected all text files to be indexed, got %#v", meta.NotIndexedFiles)
	}
}

func TestBuildMetadataTracksNoSummaryAndSizeLimits(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "README.md"), "# Demo\n")
	mustWriteFile(t, filepath.Join(root, "large.md"), strings.Repeat("a", 32))

	_, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 8,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	meta := readMetadataFile(t, root)
	if len(meta.NotIndexedFiles) != 1 {
		t.Fatalf("expected one max-size not-indexed file, got %#v", meta.NotIndexedFiles)
	}
	if filepath.Base(meta.NotIndexedFiles[0].Path) != "large.md" || meta.NotIndexedFiles[0].Reason != "max_file_size_exceeded" {
		t.Fatalf("unexpected not-indexed entry: %#v", meta.NotIndexedFiles[0])
	}

	_, err = Build(Options{
		Root:        root,
		Depth:       1,
		NoSummary:   true,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success with no-summary, got %v", err)
	}

	meta = readMetadataFile(t, root)
	if len(meta.NotIndexedFiles) != 2 {
		t.Fatalf("expected both files to be marked not indexed in no-summary mode, got %#v", meta.NotIndexedFiles)
	}
	for _, entry := range meta.NotIndexedFiles {
		if entry.Reason != "no_summary_mode" {
			t.Fatalf("expected no_summary_mode reason, got %#v", entry)
		}
	}
}

func TestBuildMarksBinaryFilesAsNotIndexed(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "notes.txt"), "hello\n")
	if err := os.WriteFile(filepath.Join(root, "image.bin"), []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	result, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if len(result.Tree.Children) != 2 {
		t.Fatalf("expected two files, got %d", len(result.Tree.Children))
	}

	meta := readMetadataFile(t, root)
	if len(meta.NotIndexedFiles) != 1 {
		t.Fatalf("expected one binary not-indexed file, got %#v", meta.NotIndexedFiles)
	}
	if filepath.Base(meta.NotIndexedFiles[0].Path) != "image.bin" || meta.NotIndexedFiles[0].Reason != "binary_file" {
		t.Fatalf("unexpected not-indexed entry: %#v", meta.NotIndexedFiles[0])
	}
}

func TestBuildUsesProvidedSummarizer(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "notes.txt"), "hello world\n")

	result, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			if length != "medium" || focus != "" {
				t.Fatalf("unexpected summarizer args length=%q focus=%q", length, focus)
			}
			return "custom summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if len(result.Tree.Children) != 1 {
		t.Fatalf("expected one file, got %#v", result.Tree.Children)
	}
	if result.Tree.Children[0].Summary != "custom summary" {
		t.Fatalf("expected custom summary, got %#v", result.Tree.Children[0])
	}
}

func TestBuildRecordsFailedFilesAndContinues(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "good-a.txt"), "hello\n")
	mustWriteFile(t, filepath.Join(root, "bad.txt"), "broken\n")
	mustWriteFile(t, filepath.Join(root, "good-b.txt"), "world\n")

	result, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			if strings.Contains(path, "bad.txt") {
				return "", os.ErrPermission
			}
			return "ok summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if len(result.Tree.Children) != 2 {
		t.Fatalf("expected only successful files in tree, got %#v", result.Tree.Children)
	}
	meta := readMetadataFile(t, root)
	if len(meta.FailedFiles) != 1 {
		t.Fatalf("expected one failed file record, got %#v", meta.FailedFiles)
	}
	if filepath.Base(meta.FailedFiles[0].Path) != "bad.txt" {
		t.Fatalf("expected bad.txt failure, got %#v", meta.FailedFiles[0])
	}
	if !strings.Contains(meta.FailedFiles[0].Error, "permission") {
		t.Fatalf("expected permission error, got %#v", meta.FailedFiles[0])
	}
	if len(meta.Files) != 3 {
		t.Fatalf("expected metadata records for all files, got %#v", meta.Files)
	}
	var sawFailed bool
	for _, file := range meta.Files {
		if filepath.Base(file.Path) == "bad.txt" {
			sawFailed = true
			if file.Status != "failed" || !strings.Contains(file.Error, "permission") {
				t.Fatalf("unexpected failed metadata file record: %#v", file)
			}
		}
	}
	if !sawFailed {
		t.Fatalf("expected failed file record in metadata files, got %#v", meta.Files)
	}
}

func TestBuildWarnsAfterConsecutiveFailures(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "bad-a.txt"), "a\n")
	mustWriteFile(t, filepath.Join(root, "bad-b.txt"), "b\n")
	mustWriteFile(t, filepath.Join(root, "good.txt"), "ok\n")
	var stderr bytes.Buffer

	result, err := Build(Options{
		Root:                root,
		Depth:               1,
		MaxFileSize:         1024,
		FailureThreshold:    2,
		FailureNoticeWriter: &stderr,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			if strings.Contains(path, "bad-") {
				return "", os.ErrInvalid
			}
			return "ok summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	if len(result.Tree.Children) != 1 || filepath.Base(result.Tree.Children[0].Path) != "good.txt" {
		t.Fatalf("expected good.txt to still be indexed, got %#v", result.Tree.Children)
	}
	meta := readMetadataFile(t, root)
	if len(meta.FailedFiles) != 2 {
		t.Fatalf("expected two failed files, got %#v", meta.FailedFiles)
	}
	if len(meta.Warnings) != 1 {
		t.Fatalf("expected one warning, got %#v", meta.Warnings)
	}
	if !strings.Contains(meta.Warnings[0], "consecutive failures") {
		t.Fatalf("unexpected warning, got %#v", meta.Warnings)
	}
	if !strings.Contains(stderr.String(), "Please check") {
		t.Fatalf("expected stderr warning, got %q", stderr.String())
	}
}

func TestBuildReusesUnchangedFileMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	mustWriteFile(t, path, "hello world\n")

	calls := 0
	_, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			calls++
			return "first summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected first build success, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one summarizer call on first build, got %d", calls)
	}

	calls = 0
	_, err = Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			calls++
			return "second summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected second build success, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected unchanged file to reuse metadata without summarizer call, got %d", calls)
	}

	meta := readMetadataFile(t, root)
	if len(meta.Files) != 1 || meta.Files[0].Summary != "first summary" {
		t.Fatalf("expected reused metadata summary, got %#v", meta.Files)
	}
}

func TestBuildResummarizesModifiedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	mustWriteFile(t, path, "hello world\n")

	_, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			return "first summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected first build success, got %v", err)
	}

	mustWriteFile(t, path, "hello changed world\n")
	calls := 0
	_, err = Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			calls++
			return "updated summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected second build success, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected modified file to resummarize once, got %d", calls)
	}

	meta := readMetadataFile(t, root)
	if len(meta.Files) != 1 || meta.Files[0].Summary != "updated summary" {
		t.Fatalf("expected updated metadata summary, got %#v", meta.Files)
	}
}

func TestBuildRetriesPreviouslyFailedFileEvenWhenUnchanged(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	mustWriteFile(t, path, "hello world\n")

	_, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			return "", os.ErrPermission
		},
	})
	if err != nil {
		t.Fatalf("expected failed-file build to still succeed overall, got %v", err)
	}

	calls := 0
	_, err = Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			calls++
			return "recovered summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected retry build success, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected previously failed file to retry summarization once, got %d", calls)
	}

	meta := readMetadataFile(t, root)
	if len(meta.Files) != 1 || meta.Files[0].Summary != "recovered summary" || meta.Files[0].Status != "ok" {
		t.Fatalf("expected recovered metadata file record, got %#v", meta.Files)
	}
}

func TestBuildAddsNewFileWithoutResummarizingUnchangedSibling(t *testing.T) {
	root := t.TempDir()
	existingPath := filepath.Join(root, "existing.txt")
	newPath := filepath.Join(root, "new.txt")
	mustWriteFile(t, existingPath, "hello world\n")

	_, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			if filepath.Base(path) == "existing.txt" {
				return "existing summary", nil
			}
			return "unexpected", nil
		},
	})
	if err != nil {
		t.Fatalf("expected first build success, got %v", err)
	}

	mustWriteFile(t, newPath, "new file here\n")
	calls := map[string]int{}
	_, err = Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			calls[filepath.Base(path)]++
			switch filepath.Base(path) {
			case "new.txt":
				return "new summary", nil
			case "existing.txt":
				return "existing changed unexpectedly", nil
			default:
				return "unexpected", nil
			}
		},
	})
	if err != nil {
		t.Fatalf("expected second build success, got %v", err)
	}
	if calls["new.txt"] != 1 {
		t.Fatalf("expected new file to summarize once, got %#v", calls)
	}
	if calls["existing.txt"] != 0 {
		t.Fatalf("expected unchanged sibling to reuse metadata, got %#v", calls)
	}

	meta := readMetadataFile(t, root)
	if len(meta.Files) != 2 {
		t.Fatalf("expected two file records, got %#v", meta.Files)
	}
	found := map[string]string{}
	for _, file := range meta.Files {
		found[filepath.Base(file.Path)] = file.Summary
	}
	if found["existing.txt"] != "existing summary" || found["new.txt"] != "new summary" {
		t.Fatalf("unexpected metadata summaries: %#v", found)
	}
}

func TestBuildRemovesDeletedFileWhileReusingUnchangedSibling(t *testing.T) {
	root := t.TempDir()
	keepPath := filepath.Join(root, "keep.txt")
	deletePath := filepath.Join(root, "delete.txt")
	mustWriteFile(t, keepPath, "keep me\n")
	mustWriteFile(t, deletePath, "delete me\n")

	_, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			switch filepath.Base(path) {
			case "keep.txt":
				return "keep summary", nil
			case "delete.txt":
				return "delete summary", nil
			default:
				return "unexpected", nil
			}
		},
	})
	if err != nil {
		t.Fatalf("expected first build success, got %v", err)
	}

	if err := os.Remove(deletePath); err != nil {
		t.Fatalf("remove deleted file: %v", err)
	}
	calls := 0
	_, err = Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			calls++
			return "unexpected rerun", nil
		},
	})
	if err != nil {
		t.Fatalf("expected second build success, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected unchanged remaining file to reuse metadata without summarizer call, got %d", calls)
	}

	meta := readMetadataFile(t, root)
	if len(meta.Files) != 1 {
		t.Fatalf("expected deleted file to be removed from metadata, got %#v", meta.Files)
	}
	if filepath.Base(meta.Files[0].Path) != "keep.txt" || meta.Files[0].Summary != "keep summary" {
		t.Fatalf("unexpected remaining metadata record: %#v", meta.Files)
	}
}

func TestMarkDirtyMarksFolderTreeMetadataDirty(t *testing.T) {
	root := copyFixtureTree(t)
	if _, err := Build(Options{
		Root:        root,
		Depth:       8,
		MaxFileSize: 1024,
	}); err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	result, err := MarkDirty(root)
	if err != nil {
		t.Fatalf("expected mark dirty success, got %v", err)
	}
	if result.MarkedCount == 0 {
		t.Fatalf("expected metadata files to be marked dirty, got %#v", result)
	}

	for _, dir := range []string{
		root,
		filepath.Join(root, "assets"),
		filepath.Join(root, "docs"),
		filepath.Join(root, "docs", "level1"),
	} {
		meta := readMetadataFile(t, dir)
		if !meta.Dirty {
			t.Fatalf("expected dirty metadata in %s, got %#v", dir, meta)
		}
	}
}

func TestBuildReindexesUnchangedFileWhenMetadataDirty(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	mustWriteFile(t, path, "hello world\n")

	_, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			return "first summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected first build success, got %v", err)
	}

	if _, err := MarkDirty(root); err != nil {
		t.Fatalf("expected mark dirty success, got %v", err)
	}

	calls := 0
	_, err = Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			calls++
			return "dirty rebuild summary", nil
		},
	})
	if err != nil {
		t.Fatalf("expected rebuild success, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected dirty metadata to force reindex, got %d calls", calls)
	}

	meta := readMetadataFile(t, root)
	if meta.Dirty {
		t.Fatalf("expected fresh metadata to clear dirty flag, got %#v", meta)
	}
	if len(meta.Files) != 1 || meta.Files[0].Summary != "dirty rebuild summary" {
		t.Fatalf("expected rebuilt metadata, got %#v", meta.Files)
	}
}

func TestEvalReportsNeedsIndexAndUnchangedEntries(t *testing.T) {
	root := t.TempDir()
	keepPath := filepath.Join(root, "keep.txt")
	changePath := filepath.Join(root, "change.txt")
	childPath := filepath.Join(root, "docs", "guide.txt")
	mustWriteFile(t, keepPath, "keep\n")
	mustWriteFile(t, changePath, "change\n")
	mustWriteFile(t, childPath, "guide\n")

	if _, err := Build(Options{
		Root:        root,
		Depth:       4,
		MaxFileSize: 1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			return "summary", nil
		},
	}); err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	mustWriteFile(t, changePath, "changed content\n")
	result, err := Eval(root)
	if err != nil {
		t.Fatalf("expected eval success, got %v", err)
	}

	if evalFolderState(result.Folders, root) != "needs_index" {
		t.Fatalf("expected root folder to need index, got %#v", result.Folders)
	}
	if evalFolderState(result.Folders, filepath.Join(root, "docs")) != "unchanged" {
		t.Fatalf("expected child folder to stay unchanged, got %#v", result.Folders)
	}
	if evalFileState(result.Files, filepath.ToSlash(filepath.Join(filepath.Base(root), "change.txt"))) != "needs_index" {
		t.Fatalf("expected changed file to need index, got %#v", result.Files)
	}
	if evalFileState(result.Files, filepath.ToSlash(filepath.Join(filepath.Base(root), "keep.txt"))) != "unchanged" {
		t.Fatalf("expected unchanged file to stay unchanged, got %#v", result.Files)
	}
	if evalFileState(result.Files, filepath.ToSlash(filepath.Join(filepath.Base(root), "docs", "guide.txt"))) != "unchanged" {
		t.Fatalf("expected unchanged child file to stay unchanged, got %#v", result.Files)
	}
}

func TestBuildRespectsMaxConcurrency(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 6; i++ {
		mustWriteFile(t, filepath.Join(root, "file-"+itoa(i)+".txt"), "hello\n")
	}

	release := make(chan struct{})
	var current int32
	var maxSeen int32
	done := make(chan struct{})
	var buildErr error

	go func() {
		defer close(done)
		_, buildErr = Build(Options{
			Root:           root,
			Depth:          1,
			MaxConcurrency: 2,
			MaxFileSize:    1024,
			Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
				now := atomic.AddInt32(&current, 1)
				for {
					prior := atomic.LoadInt32(&maxSeen)
					if now <= prior || atomic.CompareAndSwapInt32(&maxSeen, prior, now) {
						break
					}
				}
				<-release
				atomic.AddInt32(&current, -1)
				return "summary", nil
			},
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&maxSeen) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	<-done

	if buildErr != nil {
		t.Fatalf("expected build success, got %v", buildErr)
	}
	if atomic.LoadInt32(&maxSeen) != 2 {
		t.Fatalf("expected max concurrency 2, got %d", atomic.LoadInt32(&maxSeen))
	}
}

func TestBuildConcurrentFileProcessingRemainsDeterministic(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"c.txt", "a.txt", "b.txt"} {
		mustWriteFile(t, filepath.Join(root, name), "hello\n")
	}

	_, err := Build(Options{
		Root:           root,
		Depth:          1,
		MaxConcurrency: 3,
		MaxFileSize:    1024,
		Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
			switch filepath.Base(path) {
			case "c.txt":
				time.Sleep(40 * time.Millisecond)
			case "a.txt":
				time.Sleep(5 * time.Millisecond)
			case "b.txt":
				time.Sleep(20 * time.Millisecond)
			}
			return "summary for " + filepath.Base(path), nil
		},
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	meta := readMetadataFile(t, root)
	got := []string{}
	for _, file := range meta.Files {
		got = append(got, filepath.Base(file.Path))
	}
	want := []string{"a.txt", "b.txt", "c.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected deterministic metadata order %v, got %v", want, got)
	}
}

func TestBuildRespectsFolderConcurrency(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 4; i++ {
		mustWriteFile(t, filepath.Join(root, "dir-"+itoa(i), "file.txt"), "hello\n")
	}

	release := make(chan struct{})
	var current int32
	var maxSeen int32
	done := make(chan struct{})
	var buildErr error

	go func() {
		defer close(done)
		_, buildErr = Build(Options{
			Root:           root,
			Depth:          4,
			MaxConcurrency: 2,
			MaxFileSize:    1024,
			Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
				now := atomic.AddInt32(&current, 1)
				for {
					prior := atomic.LoadInt32(&maxSeen)
					if now <= prior || atomic.CompareAndSwapInt32(&maxSeen, prior, now) {
						break
					}
				}
				<-release
				atomic.AddInt32(&current, -1)
				return "summary", nil
			},
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&maxSeen) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	<-done

	if buildErr != nil {
		t.Fatalf("expected build success, got %v", buildErr)
	}
	if atomic.LoadInt32(&maxSeen) != 2 {
		t.Fatalf("expected folder-local rebuild concurrency 2, got %d", atomic.LoadInt32(&maxSeen))
	}
}

func TestBuildWithCanceledContextStopsIndexing(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 3; i++ {
		mustWriteFile(t, filepath.Join(root, "file-"+itoa(i)+".txt"), "hello\n")
	}

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	done := make(chan struct{})
	var buildErr error
	go func() {
		defer close(done)
		_, buildErr = Build(Options{
			Context:        ctx,
			Root:           root,
			Depth:          1,
			MaxConcurrency: 2,
			MaxFileSize:    1024,
			Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
				select {
				case started <- struct{}{}:
				default:
				}
				<-ctx.Done()
				return "", ctx.Err()
			},
		})
	}()

	<-started
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected build to stop after context cancellation")
	}

	if buildErr == nil || !strings.Contains(buildErr.Error(), "canceled") {
		t.Fatalf("expected cancellation error, got %v", buildErr)
	}
}

func TestBuildDeepTreeDoesNotDeadlockAtLowConcurrency(t *testing.T) {
	root := t.TempDir()
	current := root
	for i := 0; i < 5; i++ {
		current = filepath.Join(current, "level-"+itoa(i))
		mustWriteFile(t, filepath.Join(current, "file.txt"), "hello\n")
	}

	done := make(chan struct{})
	var buildErr error
	go func() {
		defer close(done)
		_, buildErr = Build(Options{
			Root:           root,
			Depth:          8,
			MaxConcurrency: 1,
			MaxFileSize:    1024,
			Summarizer: func(ctx context.Context, path, content, length, focus string) (string, error) {
				return "summary", nil
			},
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected deep tree build to complete without deadlock")
	}
	if buildErr != nil {
		t.Fatalf("expected build success, got %v", buildErr)
	}
}

func TestDescribeFileReturnsSchemaFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.conf")
	mustWriteFile(t, path, "host=localhost\nport=8080\n")

	result, err := DescribeFile(path, "short", "configuration")
	if err != nil {
		t.Fatalf("expected describe success, got %v", err)
	}

	if result.Path != filepath.ToSlash(path) {
		t.Fatalf("expected path %q, got %q", filepath.ToSlash(path), result.Path)
	}
	if result.Length != "short" {
		t.Fatalf("expected short length, got %q", result.Length)
	}
	if result.Focus != "configuration" {
		t.Fatalf("expected focus configuration, got %q", result.Focus)
	}
	if result.Summary == "" || !strings.Contains(result.Summary, "Focus: configuration.") {
		t.Fatalf("expected focused summary, got %#v", result)
	}
	if result.GeneratedAt == "" {
		t.Fatalf("expected generated_at timestamp, got %#v", result)
	}
}

func TestDescribeFileRejectsBinaryFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "image.bin")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	_, err := DescribeFile(path, "medium", "")
	if err == nil {
		t.Fatalf("expected binary describe failure")
	}
	if !strings.Contains(err.Error(), "only supports text files") {
		t.Fatalf("expected text-file error, got %v", err)
	}
}

func TestCleanArtifactsDryRunAndRemove(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a", metadataFilename), "{}")
	mustWriteFile(t, filepath.Join(root, "b", "c", metadataFilename), "{}")

	dryRun, err := CleanArtifacts(root, true)
	if err != nil {
		t.Fatalf("expected dry-run success, got %v", err)
	}
	if dryRun.RemovedCount != 2 || len(dryRun.RemovedPaths) != 2 || !dryRun.DryRun {
		t.Fatalf("unexpected dry-run result: %#v", dryRun)
	}
	for _, path := range dryRun.RemovedPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected dry-run to keep file %s: %v", path, err)
		}
	}

	result, err := CleanArtifacts(root, false)
	if err != nil {
		t.Fatalf("expected cleanup success, got %v", err)
	}
	if result.RemovedCount != 2 || result.DryRun {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	for _, path := range result.RemovedPaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected removed file %s to be gone, got err=%v", path, err)
		}
	}
}

func TestBuildFixtureHonorsIgnoreFilesAndTextDetection(t *testing.T) {
	root := copyFixtureTree(t)

	result, err := Build(Options{
		Root:        root,
		Depth:       8,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	childByBase := map[string]IndexedPathNode{}
	for _, child := range result.Tree.Children {
		childByBase[filepath.Base(child.Path)] = child
	}

	for _, ignored := range []string{"ignored-dir", "skip-root.txt"} {
		if _, ok := childByBase[ignored]; ok {
			t.Fatalf("expected %s to be excluded by fixture ignore files, got %#v", ignored, result.Tree.Children)
		}
	}

	for _, present := range []string{"README.md", "notes.conf", "assets", "docs", ".gitignore", ".aaignore"} {
		if _, ok := childByBase[present]; !ok {
			t.Fatalf("expected %s in root children, got %#v", present, result.Tree.Children)
		}
	}

	if childByBase["README.md"].Summary == "" || childByBase["notes.conf"].Summary == "" {
		t.Fatalf("expected root text files to be summarized, got %#v", result.Tree.Children)
	}

	assets := requireChildByBase(t, result.Tree, "assets")
	svg := requireChildByBase(t, assets, "diagram.svg")
	if svg.Summary == "" || svg.SummarizedAt == "" {
		t.Fatalf("expected text-based image fixture to be summarized, got %#v", svg)
	}

	meta := readMetadataFile(t, root)
	if len(meta.NotIndexedFiles) != 0 {
		t.Fatalf("expected checked-in fixture files to all be indexable text, got %#v", meta.NotIndexedFiles)
	}
}

func TestBuildFixtureIteratesThreeLevelsAndRespectsDepth(t *testing.T) {
	root := copyFixtureTree(t)

	full, err := Build(Options{
		Root:        root,
		Depth:       8,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	docs := requireChildByBase(t, full.Tree, "docs")
	level1 := requireChildByBase(t, docs, "level1")
	level2 := requireChildByBase(t, level1, "level2")
	deep := requireChildByBase(t, level2, "deep")

	for _, node := range []IndexedPathNode{docs, level1, level2, deep} {
		if node.Type != "dir" {
			t.Fatalf("expected directory node, got %#v", node)
		}
		if node.Summary == "" {
			t.Fatalf("expected directory summary, got %#v", node)
		}
	}

	for _, name := range []string{"guide.txt", "page.html", "Dockerfile", "LICENSE"} {
		var owner IndexedPathNode
		switch name {
		case "guide.txt":
			owner = level1
		case "page.html":
			owner = level2
		default:
			owner = deep
		}
		child := requireChildByBase(t, owner, name)
		if child.Summary == "" || child.SummarizedAt == "" {
			t.Fatalf("expected nested text file %s to be summarized, got %#v", name, child)
		}
	}

	limited, err := Build(Options{
		Root:        root,
		Depth:       1,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected limited-depth build success, got %v", err)
	}
	limitedDocs := requireChildByBase(t, limited.Tree, "docs")
	if hasChildBase(limitedDocs, "level1") {
		t.Fatalf("expected level1 to be omitted at depth 1, got %#v", limitedDocs.Children)
	}
}

func TestBuildWritesMetadataPerFolder(t *testing.T) {
	root := copyFixtureTree(t)

	_, err := Build(Options{
		Root:        root,
		Depth:       8,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	for _, dir := range []string{
		root,
		filepath.Join(root, "assets"),
		filepath.Join(root, "docs"),
		filepath.Join(root, "docs", "level1"),
		filepath.Join(root, "docs", "level1", "level2"),
		filepath.Join(root, "docs", "level1", "level2", "deep"),
	} {
		if _, err := os.Stat(filepath.Join(dir, metadataFilename)); err != nil {
			t.Fatalf("expected metadata in %s: %v", dir, err)
		}
	}
}

func TestBuildRootMetadataStaysShallowPerFolder(t *testing.T) {
	root := copyFixtureTree(t)

	_, err := Build(Options{
		Root:        root,
		Depth:       8,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	meta := readMetadataFile(t, root)
	for _, file := range meta.Files {
		if strings.HasPrefix(file.Path, filepath.ToSlash(filepath.Join(filepath.Base(root), "docs"))+"/") {
			t.Fatalf("expected root metadata to exclude descendant file record %q", file.Path)
		}
		if strings.HasPrefix(file.Path, filepath.ToSlash(filepath.Join(filepath.Base(root), "assets"))+"/") {
			t.Fatalf("expected root metadata to exclude descendant file record %q", file.Path)
		}
	}
}

func TestBuildAppliesNestedIgnoreFilesWithinChildScope(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "keep.skip"), "keep root\n")
	mustWriteFile(t, filepath.Join(root, "nested", ".aaignore"), "*.skip\n")
	mustWriteFile(t, filepath.Join(root, "nested", "drop.skip"), "drop nested\n")
	mustWriteFile(t, filepath.Join(root, "nested", "keep.txt"), "keep nested\n")
	mustWriteFile(t, filepath.Join(root, "sibling", "keep.skip"), "keep sibling\n")

	result, err := Build(Options{
		Root:        root,
		Depth:       4,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	rootSkip := requireChildByBase(t, result.Tree, "keep.skip")
	if rootSkip.Path == "" {
		t.Fatalf("expected root keep.skip to remain visible")
	}

	nested := requireChildByBase(t, result.Tree, "nested")
	if hasChildBase(nested, "drop.skip") {
		t.Fatalf("expected nested drop.skip to be excluded by nested .aaignore, got %#v", nested.Children)
	}
	if !hasChildBase(nested, "keep.txt") {
		t.Fatalf("expected nested keep.txt to remain visible, got %#v", nested.Children)
	}

	sibling := requireChildByBase(t, result.Tree, "sibling")
	if !hasChildBase(sibling, "keep.skip") {
		t.Fatalf("expected sibling keep.skip to remain visible, got %#v", sibling.Children)
	}
}

func TestBuildMetadataIncludesLocalFolderFields(t *testing.T) {
	root := copyFixtureTree(t)

	_, err := Build(Options{
		Root:        root,
		Depth:       8,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	meta := readMetadataFile(t, root)
	if meta.FolderDescription == "" {
		t.Fatalf("expected folder description, got %#v", meta)
	}
	if meta.BriefSummary == "" {
		t.Fatalf("expected brief summary, got %#v", meta)
	}
	if meta.Stats.DirectFileCount == 0 {
		t.Fatalf("expected direct file count in stats, got %#v", meta.Stats)
	}
	if meta.Stats.DirectDirCount == 0 {
		t.Fatalf("expected direct dir count in stats, got %#v", meta.Stats)
	}
}

func TestBuildMetadataDoesNotPersistChildren(t *testing.T) {
	root := copyFixtureTree(t)

	_, err := Build(Options{
		Root:        root,
		Depth:       8,
		MaxFileSize: 1024,
	})
	if err != nil {
		t.Fatalf("expected build success, got %v", err)
	}

	meta := readMetadataFile(t, root)
	if strings.Contains(readMetadataFileRaw(t, root), "\"children\"") {
		t.Fatalf("expected local metadata JSON to omit children, got %#v", meta)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readMetadataFile(t *testing.T, root string) Metadata {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, metadataFilename))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta Metadata
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v\n%s", err, string(content))
	}
	return meta
}

func readMetadataFileRaw(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, metadataFilename))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	return string(content)
}

func copyFixtureTree(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs(fixtureRoot)
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "fixture")
	if err := copyDir(source, dest); err != nil {
		t.Fatalf("copy fixture tree: %v", err)
	}
	return dest
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func requireChildByBase(t *testing.T, node IndexedPathNode, base string) IndexedPathNode {
	t.Helper()
	for _, child := range node.Children {
		if filepath.Base(child.Path) == base {
			return child
		}
	}
	t.Fatalf("expected child %s in %#v", base, node.Children)
	return IndexedPathNode{}
}

func hasChildBase(node IndexedPathNode, base string) bool {
	for _, child := range node.Children {
		if filepath.Base(child.Path) == base {
			return true
		}
	}
	return false
}

func evalFolderState(items []EvalFolder, path string) string {
	for _, item := range items {
		if item.Path == filepath.ToSlash(path) {
			return item.State
		}
	}
	return ""
}

func evalFileState(items []EvalFile, path string) string {
	for _, item := range items {
		if item.Path == filepath.ToSlash(path) {
			return item.State
		}
	}
	return ""
}
