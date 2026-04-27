package operation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/austinjan/aascribe/pkg/llmoutput"
)

func TestCreateWritesInitialStatusSnapshot(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{
		Command: "index",
		Stage:   "starting",
		Message: "Preparing index operation.",
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}
	if accepted.OperationID == "" {
		t.Fatalf("expected operation id, got %#v", accepted)
	}
	if accepted.Command != "index" || accepted.State != StateRunning {
		t.Fatalf("unexpected accepted payload: %#v", accepted)
	}
	assertExists(t, filepath.Join(storePath, "operations", accepted.OperationID, "operation.json"))

	status, err := LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if status.Command != "index" || status.Stage != "starting" {
		t.Fatalf("unexpected loaded status: %#v", status)
	}
	if status.ResultReady {
		t.Fatalf("expected result_ready=false, got %#v", status)
	}
}

func TestSaveStatusAppendEventAndLoadEvents(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{Command: "index"})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	status, err := LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	status.Stage = "indexing"
	status.Message = "Indexing direct files."
	status.Progress = &Progress{Current: 2, Total: 5, Unit: "files", Percent: 40}
	if err := SaveStatus(storePath, status); err != nil {
		t.Fatalf("save status: %v", err)
	}
	if err := AppendEvent(storePath, accepted.OperationID, Event{
		Level:   "info",
		Stage:   "indexing",
		Message: "Indexed README.md",
		Data: map[string]any{
			"path": "README.md",
		},
	}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	reloaded, err := LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("reload status: %v", err)
	}
	if reloaded.Progress == nil || reloaded.Progress.Current != 2 {
		t.Fatalf("expected persisted progress, got %#v", reloaded)
	}

	events, err := LoadEvents(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events.Events) != 1 || events.Events[0].Message != "Indexed README.md" {
		t.Fatalf("unexpected event list: %#v", events)
	}
}

func TestReporterUpdatesStatusAndAppendsEvent(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{
		Command: "index",
		Stage:   "starting",
		Message: "Starting.",
	})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	reporter := NewReporter(storePath, accepted.OperationID)
	if err := reporter.Report(Report{
		Stage:   "processing_files",
		Message: "Processing direct files.",
		Progress: &Progress{
			Current: 3,
			Total:   10,
			Unit:    "files",
			Percent: 30,
		},
		Data: map[string]any{
			"folder": "internal",
		},
	}); err != nil {
		t.Fatalf("report progress: %v", err)
	}

	status, err := LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if status.Stage != "processing_files" || status.Message != "Processing direct files." {
		t.Fatalf("expected reporter status update, got %#v", status)
	}
	if status.Progress == nil || status.Progress.Current != 3 {
		t.Fatalf("expected reporter progress update, got %#v", status)
	}

	events, err := LoadEvents(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events.Events) != 1 || events.Events[0].Stage != "processing_files" {
		t.Fatalf("expected reporter event, got %#v", events)
	}
	if events.Events[0].Data["folder"] != "internal" {
		t.Fatalf("expected structured event data, got %#v", events.Events[0])
	}
}

func TestSaveResultAndListOperations(t *testing.T) {
	storePath := t.TempDir()

	first, err := Create(storePath, CreateInput{Command: "index", Message: "first"})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := Create(storePath, CreateInput{Command: "map", Message: "second"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	secondStatus, err := LoadStatus(storePath, second.OperationID)
	if err != nil {
		t.Fatalf("load second status: %v", err)
	}
	secondStatus.State = StateSucceeded
	secondStatus.Stage = "complete"
	secondStatus.Message = "Operation completed."
	secondStatus.ResultReady = true
	secondStatus.CompletedAt = secondStatus.UpdatedAt
	if err := SaveStatus(storePath, secondStatus); err != nil {
		t.Fatalf("save second status: %v", err)
	}
	if err := SaveResult(storePath, &Result{
		OperationID: second.OperationID,
		State:       StateSucceeded,
		Data: map[string]any{
			"root": "/tmp/demo",
		},
		Truncated: false,
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}

	result, err := LoadResult(storePath, second.OperationID)
	if err != nil {
		t.Fatalf("load result: %v", err)
	}
	if result.State != StateSucceeded {
		t.Fatalf("unexpected result: %#v", result)
	}

	listed, err := ListOperations(storePath)
	if err != nil {
		t.Fatalf("list operations: %v", err)
	}
	if listed.Count != 2 {
		t.Fatalf("expected 2 operations, got %#v", listed)
	}
	ids := map[string]bool{
		first.OperationID:  false,
		second.OperationID: false,
	}
	for _, item := range listed.Items {
		_, ok := ids[item.OperationID]
		if !ok {
			t.Fatalf("unexpected operation in list: %#v", item)
		}
		ids[item.OperationID] = true
	}
	for id, seen := range ids {
		if !seen {
			t.Fatalf("missing operation %s in list", id)
		}
	}
}

func TestSaveResultStoresOversizedDataThroughOutputTransport(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{Command: "index", Message: "large result"})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}
	largeSummary := strings.Repeat("large result line\n", 400)
	if err := SaveResult(storePath, &Result{
		OperationID: accepted.OperationID,
		State:       StateSucceeded,
		Data: map[string]any{
			"summary": largeSummary,
		},
		Truncated: false,
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}

	result, err := LoadResult(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load result: %v", err)
	}
	if !result.Truncated || result.OutputID == "" {
		t.Fatalf("expected output transport reference, got %#v", result)
	}
	if result.Data != nil {
		t.Fatalf("expected oversized data to be omitted from result.json, got %#v", result.Data)
	}
	meta, err := llmoutput.Meta(storePath, result.OutputID)
	if err != nil {
		t.Fatalf("expected stored output metadata: %v", err)
	}
	if meta.TotalRunes <= 4000 {
		t.Fatalf("expected large stored output, got %#v", meta)
	}
}

func TestCancelRunningOperationPersistsCanceledLifecycle(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{Command: "index"})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	cancelResult, err := Cancel(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("cancel operation: %v", err)
	}
	if cancelResult.State != StateCanceled {
		t.Fatalf("expected canceled state, got %#v", cancelResult)
	}

	status, err := LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if status.State != StateCanceled || !status.ResultReady || status.CompletedAt == "" {
		t.Fatalf("expected canceled terminal status, got %#v", status)
	}

	result, err := LoadResult(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load result: %v", err)
	}
	if result.State != StateCanceled || result.Error == nil || result.Error.Code != "OPERATION_CANCELED" {
		t.Fatalf("expected canceled result, got %#v", result)
	}

	events, err := LoadEvents(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events.Events) != 1 || events.Events[0].Stage != "canceled" {
		t.Fatalf("expected cancel event, got %#v", events)
	}
}

func TestCancelAlreadyCanceledOperationIsIdempotent(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{Command: "index"})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}
	if _, err := Cancel(storePath, accepted.OperationID); err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	second, err := Cancel(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("second cancel should be idempotent, got %v", err)
	}
	if second.State != StateCanceled || second.Message != "Operation was already canceled." {
		t.Fatalf("unexpected idempotent cancel result: %#v", second)
	}
}

func TestCancelSucceededOperationIsRejected(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{Command: "index"})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}
	status, err := LoadStatus(storePath, accepted.OperationID)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	status.State = StateSucceeded
	status.Stage = "complete"
	status.Message = "Operation completed."
	status.ResultReady = true
	status.CompletedAt = status.UpdatedAt
	if err := SaveStatus(storePath, status); err != nil {
		t.Fatalf("save status: %v", err)
	}

	if _, err := Cancel(storePath, accepted.OperationID); err == nil {
		t.Fatalf("expected terminal operation cancel to fail")
	}
}

func TestContextWithCancelWatchCancelsWhenOperationIsCanceled(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{Command: "index"})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	ctx, stop := ContextWithCancelWatchInterval(context.Background(), storePath, accepted.OperationID, time.Millisecond)
	defer stop()

	if _, err := Cancel(storePath, accepted.OperationID); err != nil {
		t.Fatalf("cancel operation: %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatalf("expected watched context to be canceled")
	}
}

func TestContextWithCancelWatchStopIsIdempotent(t *testing.T) {
	storePath := t.TempDir()

	accepted, err := Create(storePath, CreateInput{Command: "index"})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}

	ctx, stop := ContextWithCancelWatchInterval(context.Background(), storePath, accepted.OperationID, time.Millisecond)
	stop()
	stop()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatalf("expected stopped context to be canceled")
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}
