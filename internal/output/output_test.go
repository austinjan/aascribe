package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
)

func TestWriteErrorJSONIncludesHintAndExamples(t *testing.T) {
	var out bytes.Buffer

	err := WriteError(&out, cli.FormatJSON, false, apperr.StoreNotFound("./data/memory"), "list", time.Now(), "./data/memory")
	if err != nil {
		t.Fatalf("expected write success, got %v", err)
	}

	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Code     string   `json:"code"`
			Message  string   `json:"message"`
			Hint     string   `json:"hint"`
			Examples []string `json:"examples"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	if payload.Error.Code != "STORE_NOT_FOUND" {
		t.Fatalf("expected STORE_NOT_FOUND, got %q", payload.Error.Code)
	}
	if payload.Error.Hint == "" {
		t.Fatalf("expected non-empty hint, got empty")
	}
	if len(payload.Error.Examples) < 2 {
		t.Fatalf("expected example commands, got %#v", payload.Error.Examples)
	}
}

func TestWriteErrorTextIncludesHintAndExamples(t *testing.T) {
	var out bytes.Buffer

	err := WriteError(&out, cli.FormatText, false, apperr.InvalidArguments("Missing value for --store."), "<parse>", time.Now(), "<unresolved>")
	if err != nil {
		t.Fatalf("expected write success, got %v", err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, "Hint:") {
		t.Fatalf("expected hint in text output, got %q", rendered)
	}
	if !strings.Contains(rendered, "Examples:") {
		t.Fatalf("expected examples in text output, got %q", rendered)
	}
	if !strings.Contains(rendered, "aascribe init") {
		t.Fatalf("expected example command in text output, got %q", rendered)
	}
}

func TestWriteErrorJSONTailorsLogsClearGuidance(t *testing.T) {
	var out bytes.Buffer

	err := WriteError(&out, cli.FormatJSON, false, apperr.InvalidArguments("logs clear requires --force."), "logs", time.Now(), "./data/memory")
	if err != nil {
		t.Fatalf("expected write success, got %v", err)
	}

	var payload struct {
		Error struct {
			Hint     string   `json:"hint"`
			Examples []string `json:"examples"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	if !strings.Contains(payload.Error.Hint, "--force") {
		t.Fatalf("expected force-specific hint, got %q", payload.Error.Hint)
	}
	if len(payload.Error.Examples) == 0 || payload.Error.Examples[0] != "aascribe logs clear --force" {
		t.Fatalf("expected logs clear example first, got %#v", payload.Error.Examples)
	}
}

func TestWriteErrorJSONTailorsLogsExportGuidance(t *testing.T) {
	var out bytes.Buffer

	err := WriteError(&out, cli.FormatJSON, false, apperr.InvalidArguments("logs export requires --output <path>."), "logs", time.Now(), "./data/memory")
	if err != nil {
		t.Fatalf("expected write success, got %v", err)
	}

	var payload struct {
		Error struct {
			Hint     string   `json:"hint"`
			Examples []string `json:"examples"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	if !strings.Contains(payload.Error.Hint, "--output") {
		t.Fatalf("expected output-specific hint, got %q", payload.Error.Hint)
	}
	if len(payload.Error.Examples) == 0 || !strings.Contains(payload.Error.Examples[0], "logs export --output") {
		t.Fatalf("expected logs export example first, got %#v", payload.Error.Examples)
	}
}

func TestWriteErrorJSONTailorsUnknownSubcommandGuidance(t *testing.T) {
	var out bytes.Buffer

	err := WriteError(&out, cli.FormatJSON, false, apperr.InvalidArguments("Unknown subcommand frobnicate."), "<parse>", time.Now(), "<unresolved>")
	if err != nil {
		t.Fatalf("expected write success, got %v", err)
	}

	var payload struct {
		Error struct {
			Hint     string   `json:"hint"`
			Examples []string `json:"examples"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	if !strings.Contains(payload.Error.Hint, "supported top-level commands") {
		t.Fatalf("expected subcommand-specific hint, got %q", payload.Error.Hint)
	}
	if len(payload.Error.Examples) < 3 {
		t.Fatalf("expected example commands, got %#v", payload.Error.Examples)
	}
}
