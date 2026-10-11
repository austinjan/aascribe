package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetadataUsesUniqueTemporaryFile(t *testing.T) {
	root := t.TempDir()
	legacyTemp := filepath.Join(root, metadataFilename+".tmp")
	if err := os.WriteFile(legacyTemp, []byte("preserve existing file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(Options{Root: root, Depth: 0, MaxFileSize: 1024 * 1024, RejectSymlinks: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(legacyTemp)
	if err != nil || string(data) != "preserve existing file" {
		t.Fatalf("temporary-file collision modified existing content: %s %v", data, err)
	}
	metadata, err := readMetadata(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata.Files) != 0 {
		t.Fatal("index included its own temporary metadata")
	}
}

func TestRejectSymlinksSkipsExternalSource(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(private, []byte("private external source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(private, filepath.Join(root, "linked.txt")); err != nil {
		t.Skipf("symlink privileges unavailable: %v", err)
	}
	result, err := Build(Options{Root: root, Depth: 0, MaxFileSize: 1024 * 1024, RejectSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tree.Children) != 0 {
		t.Fatal("index followed an external symlink")
	}
}
