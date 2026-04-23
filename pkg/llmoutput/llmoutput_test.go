package llmoutput

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliverKeepsSmallOutputInline(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InlineRuneLimit = 10

	out, err := Deliver(t.TempDir(), "chat", "hello", cfg)
	if err != nil {
		t.Fatalf("expected deliver success, got %v", err)
	}
	if out.Hint.Truncated {
		t.Fatalf("expected non-truncated output")
	}
	if out.StoredRef != nil {
		t.Fatalf("expected no stored ref, got %#v", out.StoredRef)
	}
	if out.InlineText != "hello" {
		t.Fatalf("expected inline hello, got %q", out.InlineText)
	}
}

func TestDeliverSpillsLargeOutputToFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InlineRuneLimit = 5

	out, err := Deliver(t.TempDir(), "chat", "hello world", cfg)
	if err != nil {
		t.Fatalf("expected deliver success, got %v", err)
	}
	if !out.Hint.Truncated {
		t.Fatalf("expected truncated output")
	}
	if out.StoredRef == nil {
		t.Fatalf("expected stored ref")
	}
	if out.Hint.OutputID == "" {
		t.Fatalf("expected output id")
	}
	if out.InlineText != "hello" {
		t.Fatalf("expected first chunk hello, got %q", out.InlineText)
	}
}

func TestHeadTailAndSliceReadStoredOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InlineRuneLimit = 4
	storePath := t.TempDir()

	delivered, err := Deliver(storePath, "chat", "one\ntwo\nthree\nfour", cfg)
	if err != nil {
		t.Fatalf("expected deliver success, got %v", err)
	}
	id := delivered.Hint.OutputID

	head, err := Head(storePath, id, 2, cfg)
	if err != nil {
		t.Fatalf("expected head success, got %v", err)
	}
	if !strings.Contains(head.Text, "one\ntwo") {
		t.Fatalf("expected head lines, got %q", head.Text)
	}

	tail, err := Tail(storePath, id, 2, cfg)
	if err != nil {
		t.Fatalf("expected tail success, got %v", err)
	}
	if !strings.Contains(tail.Text, "three\nfour") {
		t.Fatalf("expected tail lines, got %q", tail.Text)
	}

	slice, err := Slice(storePath, id, 4, 5, cfg)
	if err != nil {
		t.Fatalf("expected slice success, got %v", err)
	}
	if slice.RangeStart != 4 || slice.RangeEnd != 9 {
		t.Fatalf("expected range 4..9, got %d..%d", slice.RangeStart, slice.RangeEnd)
	}
}

func TestRetentionKeepsNewestFiftyOutputs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InlineRuneLimit = 1
	cfg.MaxRetained = 50
	storePath := t.TempDir()

	for i := 0; i < 55; i++ {
		if _, err := Deliver(storePath, "chat", "abcdef", cfg); err != nil {
			t.Fatalf("deliver %d failed: %v", i, err)
		}
	}

	items, err := List(storePath)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(items) != 50 {
		t.Fatalf("expected 50 retained outputs, got %d", len(items))
	}
	if _, err := Meta(storePath, "out_000001"); err == nil {
		t.Fatalf("expected oldest output to be evicted")
	}
	if _, err := Meta(storePath, "out_000055"); err != nil {
		t.Fatalf("expected newest output to remain, got %v", err)
	}
	for _, item := range items {
		if !strings.HasPrefix(filepath.Base(item.Path), "out_") {
			t.Fatalf("expected managed output file, got %q", item.Path)
		}
	}
}
