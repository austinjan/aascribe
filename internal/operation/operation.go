package operation

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/pkg/llmoutput"
)

type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateCanceled  State = "canceled"
)

type Progress struct {
	Current int     `json:"current"`
	Total   int     `json:"total"`
	Unit    string  `json:"unit"`
	Percent float64 `json:"percent"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Accepted struct {
	OperationID string `json:"operation_id"`
	Command     string `json:"command"`
	State       State  `json:"state"`
	StartedAt   string `json:"started_at"`
	StatusHint  string `json:"status_hint"`
	ResultHint  string `json:"result_hint"`
	CancelHint  string `json:"cancel_hint"`
}

type Status struct {
	OperationID string       `json:"operation_id"`
	Command     string       `json:"command"`
	State       State        `json:"state"`
	Stage       string       `json:"stage"`
	Message     string       `json:"message"`
	StartedAt   string       `json:"started_at"`
	UpdatedAt   string       `json:"updated_at"`
	CompletedAt string       `json:"completed_at,omitempty"`
	Progress    *Progress    `json:"progress,omitempty"`
	ResultReady bool         `json:"result_ready"`
	Error       *ErrorDetail `json:"error,omitempty"`
}

type Event struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Stage   string         `json:"stage"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

type EventList struct {
	OperationID string  `json:"operation_id"`
	Events      []Event `json:"events"`
}

type Result struct {
	OperationID string       `json:"operation_id"`
	State       State        `json:"state"`
	CompletedAt string       `json:"completed_at"`
	Data        any          `json:"data,omitempty"`
	OutputID    string       `json:"output_id,omitempty"`
	Truncated   bool         `json:"truncated"`
	Error       *ErrorDetail `json:"error,omitempty"`
}

type Summary struct {
	OperationID string `json:"operation_id"`
	Command     string `json:"command"`
	State       State  `json:"state"`
	Stage       string `json:"stage"`
	StartedAt   string `json:"started_at"`
	UpdatedAt   string `json:"updated_at"`
}

type List struct {
	Count int       `json:"count"`
	Items []Summary `json:"items"`
}

type CancelResult struct {
	OperationID string `json:"operation_id"`
	State       State  `json:"state"`
	CanceledAt  string `json:"canceled_at"`
	Message     string `json:"message"`
}

type CleanOptions struct {
	DryRun bool
	Force  bool
}

type CleanItem struct {
	OperationID string `json:"operation_id"`
	State       State  `json:"state"`
	Path        string `json:"path"`
	Reason      string `json:"reason,omitempty"`
}

type CleanResult struct {
	DryRun       bool        `json:"dry_run"`
	RemovedCount int         `json:"removed_count"`
	SkippedCount int         `json:"skipped_count"`
	Removed      []CleanItem `json:"removed"`
	Skipped      []CleanItem `json:"skipped"`
}

type CreateInput struct {
	Command string
	Stage   string
	Message string
	State   State
}

type Report struct {
	Stage    string
	Message  string
	Progress *Progress
	Level    string
	Data     map[string]any
}

type Reporter struct {
	storePath   string
	operationID string
}

var nowUTC = func() time.Time {
	return time.Now().UTC()
}

const DefaultCancelPollInterval = 200 * time.Millisecond

func Create(storePath string, input CreateInput) (*Accepted, error) {
	if strings.TrimSpace(input.Command) == "" {
		return nil, apperr.InvalidArguments("operation create requires a non-empty command name.")
	}
	if strings.TrimSpace(input.Stage) == "" {
		input.Stage = "starting"
	}
	if strings.TrimSpace(input.Message) == "" {
		input.Message = "Operation started."
	}
	if input.State == "" {
		input.State = StateRunning
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	timestamp := nowUTC().Format(time.RFC3339)
	status := &Status{
		OperationID: id,
		Command:     input.Command,
		State:       input.State,
		Stage:       input.Stage,
		Message:     input.Message,
		StartedAt:   timestamp,
		UpdatedAt:   timestamp,
		ResultReady: false,
	}
	if err := writeStatus(storePath, status); err != nil {
		return nil, err
	}

	return &Accepted{
		OperationID: id,
		Command:     input.Command,
		State:       input.State,
		StartedAt:   timestamp,
		StatusHint:  fmt.Sprintf("aascribe operation status %s", id),
		ResultHint:  fmt.Sprintf("aascribe operation result %s", id),
		CancelHint:  fmt.Sprintf("aascribe operation cancel %s", id),
	}, nil
}

func NewReporter(storePath, operationID string) *Reporter {
	return &Reporter{
		storePath:   storePath,
		operationID: operationID,
	}
}

func ContextWithCancelWatch(parent context.Context, storePath, operationID string) (context.Context, func()) {
	return ContextWithCancelWatchInterval(parent, storePath, operationID, DefaultCancelPollInterval)
}

func ContextWithCancelWatchInterval(parent context.Context, storePath, operationID string, interval time.Duration) (context.Context, func()) {
	if parent == nil {
		parent = context.Background()
	}
	if interval <= 0 {
		interval = DefaultCancelPollInterval
	}

	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			close(done)
			cancel()
		})
	}

	go watchCancel(ctx, storePath, operationID, interval, cancel, done)
	return ctx, stop
}

func watchCancel(ctx context.Context, storePath, operationID string, interval time.Duration, cancel context.CancelFunc, done <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			status, err := LoadStatus(storePath, operationID)
			if err == nil && status.State == StateCanceled {
				cancel()
				return
			}
		}
	}
}

func (r *Reporter) Report(update Report) error {
	if r == nil {
		return nil
	}
	status, err := LoadStatus(r.storePath, r.operationID)
	if err != nil {
		return err
	}
	timestamp := nowUTC().Format(time.RFC3339)
	if strings.TrimSpace(update.Stage) != "" {
		status.Stage = update.Stage
	}
	if strings.TrimSpace(update.Message) != "" {
		status.Message = update.Message
	}
	if update.Progress != nil {
		status.Progress = update.Progress
	}
	status.UpdatedAt = timestamp
	if err := SaveStatus(r.storePath, status); err != nil {
		return err
	}

	message := update.Message
	if message == "" {
		message = status.Message
	}
	level := update.Level
	if level == "" {
		level = "info"
	}
	data := update.Data
	if data == nil {
		data = map[string]any{}
	}
	return AppendEvent(r.storePath, r.operationID, Event{
		Time:    timestamp,
		Level:   level,
		Stage:   status.Stage,
		Message: message,
		Data:    data,
	})
}

func SaveStatus(storePath string, status *Status) error {
	if status == nil {
		return apperr.InvalidArguments("operation save status requires a status payload.")
	}
	if strings.TrimSpace(status.OperationID) == "" {
		return apperr.InvalidArguments("operation save status requires operation_id.")
	}
	if strings.TrimSpace(status.Command) == "" {
		return apperr.InvalidArguments("operation save status requires command.")
	}
	if strings.TrimSpace(status.Stage) == "" {
		return apperr.InvalidArguments("operation save status requires stage.")
	}
	if strings.TrimSpace(status.Message) == "" {
		return apperr.InvalidArguments("operation save status requires message.")
	}
	if strings.TrimSpace(status.StartedAt) == "" {
		status.StartedAt = nowUTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(status.UpdatedAt) == "" {
		status.UpdatedAt = nowUTC().Format(time.RFC3339)
	}
	return writeStatus(storePath, status)
}

func LoadStatus(storePath, id string) (*Status, error) {
	status := &Status{}
	if err := readJSON(statusPath(storePath, id), status); err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.OperationNotFound(id)
		}
		return nil, translateReadError(err, "operation snapshot", statusPath(storePath, id))
	}
	return status, nil
}

func Cancel(storePath, id string) (*CancelResult, error) {
	status, err := LoadStatus(storePath, id)
	if err != nil {
		return nil, err
	}
	if status.State == StateSucceeded || status.State == StateFailed {
		return nil, apperr.OperationAlreadyTerminal(id)
	}

	timestamp := nowUTC().Format(time.RFC3339)
	if status.State == StateCanceled {
		return &CancelResult{
			OperationID: id,
			State:       StateCanceled,
			CanceledAt:  status.CompletedAt,
			Message:     "Operation was already canceled.",
		}, nil
	}

	status.State = StateCanceled
	status.Stage = "canceled"
	status.Message = "Operation canceled."
	status.UpdatedAt = timestamp
	status.CompletedAt = timestamp
	status.ResultReady = true
	status.Error = &ErrorDetail{
		Code:    "OPERATION_CANCELED",
		Message: "Operation canceled.",
	}
	if err := SaveStatus(storePath, status); err != nil {
		return nil, err
	}
	if err := SaveResult(storePath, &Result{
		OperationID: id,
		State:       StateCanceled,
		CompletedAt: timestamp,
		Truncated:   false,
		Error:       status.Error,
	}); err != nil {
		return nil, err
	}
	if err := AppendEvent(storePath, id, Event{
		Time:    timestamp,
		Level:   "info",
		Stage:   "canceled",
		Message: "Operation canceled.",
		Data:    map[string]any{},
	}); err != nil {
		return nil, err
	}

	return &CancelResult{
		OperationID: id,
		State:       StateCanceled,
		CanceledAt:  timestamp,
		Message:     "Operation canceled.",
	}, nil
}

func AppendEvent(storePath, id string, event Event) error {
	if _, err := LoadStatus(storePath, id); err != nil {
		return err
	}
	if strings.TrimSpace(event.Time) == "" {
		event.Time = nowUTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(event.Level) == "" {
		event.Level = "info"
	}
	if strings.TrimSpace(event.Stage) == "" {
		event.Stage = "running"
	}
	if event.Data == nil {
		event.Data = map[string]any{}
	}
	if err := os.MkdirAll(operationDir(storePath, id), 0o755); err != nil {
		return apperr.IOError("Failed to create operation directory: %s.", operationDir(storePath, id))
	}
	file, err := os.OpenFile(eventsPath(storePath, id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return apperr.IOError("Failed to append operation event log: %s.", eventsPath(storePath, id))
	}
	defer file.Close()
	encoded, err := json.Marshal(event)
	if err != nil {
		return apperr.Serialization("Failed to serialize operation event.")
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return apperr.IOError("Failed to write operation event log: %s.", eventsPath(storePath, id))
	}
	return nil
}

func LoadEvents(storePath, id string) (*EventList, error) {
	if _, err := LoadStatus(storePath, id); err != nil {
		return nil, err
	}
	path := eventsPath(storePath, id)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &EventList{OperationID: id, Events: []Event{}}, nil
		}
		return nil, apperr.IOError("Failed to read operation event log: %s.", path)
	}
	defer file.Close()

	events := []Event{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, apperr.Serialization("Failed to parse operation event log: %s.", path)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, apperr.IOError("Failed to scan operation event log: %s.", path)
	}
	return &EventList{OperationID: id, Events: events}, nil
}

func SaveResult(storePath string, result *Result) error {
	if result == nil {
		return apperr.InvalidArguments("operation save result requires a result payload.")
	}
	if strings.TrimSpace(result.OperationID) == "" {
		return apperr.InvalidArguments("operation save result requires operation_id.")
	}
	if strings.TrimSpace(result.CompletedAt) == "" {
		result.CompletedAt = nowUTC().Format(time.RFC3339)
	}
	if err := ensureOperationExists(storePath, result.OperationID); err != nil {
		return err
	}
	prepared, err := prepareResultForStorage(storePath, result)
	if err != nil {
		return err
	}
	return writeJSONAtomic(resultPath(storePath, prepared.OperationID), prepared)
}

func LoadResult(storePath, id string) (*Result, error) {
	if _, err := LoadStatus(storePath, id); err != nil {
		return nil, err
	}
	result := &Result{}
	if err := readJSON(resultPath(storePath, id), result); err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.OperationResultNotReady(id)
		}
		return nil, translateReadError(err, "operation result", resultPath(storePath, id))
	}
	return result, nil
}

func ListOperations(storePath string) (*List, error) {
	dir := storeDir(storePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, apperr.IOError("Failed to create operations directory: %s.", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, apperr.IOError("Failed to read operations directory: %s.", dir)
	}
	items := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		status, err := LoadStatus(storePath, entry.Name())
		if err != nil {
			continue
		}
		items = append(items, Summary{
			OperationID: status.OperationID,
			Command:     status.Command,
			State:       status.State,
			Stage:       status.Stage,
			StartedAt:   status.StartedAt,
			UpdatedAt:   status.UpdatedAt,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt == items[j].UpdatedAt {
			return items[i].OperationID > items[j].OperationID
		}
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return &List{Count: len(items), Items: items}, nil
}

func Clean(storePath string, opts CleanOptions) (*CleanResult, error) {
	if !opts.Force {
		opts.DryRun = true
	}
	dir := storeDir(storePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, apperr.IOError("Failed to create operations directory: %s.", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, apperr.IOError("Failed to read operations directory: %s.", dir)
	}

	result := &CleanResult{
		DryRun:  opts.DryRun,
		Removed: []CleanItem{},
		Skipped: []CleanItem{},
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		path := operationDir(storePath, id)
		status, err := LoadStatus(storePath, id)
		if err != nil {
			result.Skipped = append(result.Skipped, CleanItem{
				OperationID: id,
				State:       "",
				Path:        path,
				Reason:      "unreadable_status",
			})
			continue
		}
		item := CleanItem{
			OperationID: id,
			State:       status.State,
			Path:        path,
		}
		if !isTerminal(status.State) {
			item.Reason = "active_operation"
			result.Skipped = append(result.Skipped, item)
			continue
		}
		result.Removed = append(result.Removed, item)
		if opts.DryRun {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return nil, apperr.IOError("Failed to remove operation directory: %s.", path)
		}
	}
	result.RemovedCount = len(result.Removed)
	result.SkippedCount = len(result.Skipped)
	return result, nil
}

func isTerminal(state State) bool {
	return state == StateSucceeded || state == StateFailed || state == StateCanceled
}

func storeDir(storePath string) string {
	return filepath.Join(storePath, "operations")
}

func operationDir(storePath, id string) string {
	return filepath.Join(storeDir(storePath), id)
}

func statusPath(storePath, id string) string {
	return filepath.Join(operationDir(storePath, id), "operation.json")
}

func eventsPath(storePath, id string) string {
	return filepath.Join(operationDir(storePath, id), "events.jsonl")
}

func resultPath(storePath, id string) string {
	return filepath.Join(operationDir(storePath, id), "result.json")
}

func ensureOperationExists(storePath, id string) error {
	if _, err := os.Stat(statusPath(storePath, id)); err != nil {
		if os.IsNotExist(err) {
			return apperr.OperationNotFound(id)
		}
		return apperr.IOError("Failed to inspect operation snapshot: %s.", statusPath(storePath, id))
	}
	return nil
}

func writeStatus(storePath string, status *Status) error {
	if err := os.MkdirAll(operationDir(storePath, status.OperationID), 0o755); err != nil {
		return apperr.IOError("Failed to create operation directory: %s.", operationDir(storePath, status.OperationID))
	}
	return writeJSONAtomic(statusPath(storePath, status.OperationID), status)
}

func writeJSONAtomic(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperr.IOError("Failed to create parent directory: %s.", filepath.Dir(path))
	}
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return apperr.Serialization("Failed to serialize operation payload.")
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, append(bytes, '\n'), 0o644); err != nil {
		return apperr.IOError("Failed to write temporary operation payload: %s.", tempPath)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return apperr.IOError("Failed to replace operation payload: %s.", path)
	}
	return nil
}

func readJSON(path string, target any) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bytes, target); err != nil {
		return apperr.Serialization("Failed to parse JSON payload: %s.", path)
	}
	return nil
}

func translateReadError(err error, kind, path string) error {
	if appErr, ok := err.(*apperr.Error); ok {
		return appErr
	}
	if os.IsNotExist(err) {
		return err
	}
	return apperr.IOError("Failed to read %s: %s.", kind, path)
}

func prepareResultForStorage(storePath string, result *Result) (*Result, error) {
	if result.Data == nil || result.OutputID != "" {
		return result, nil
	}

	dataJSON, err := json.MarshalIndent(result.Data, "", "  ")
	if err != nil {
		return nil, apperr.Serialization("Failed to serialize operation result data.")
	}
	delivered, err := llmoutput.Deliver(storePath, "operation result", string(dataJSON), llmoutput.DefaultConfig())
	if err != nil {
		return nil, err
	}
	if !delivered.Hint.Truncated {
		return result, nil
	}

	trimmed := *result
	trimmed.Data = nil
	trimmed.OutputID = delivered.Hint.OutputID
	trimmed.Truncated = true
	return &trimmed, nil
}

func newID() (string, error) {
	stamp := nowUTC().Format("20060102T150405Z")
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", apperr.IOError("Failed to generate operation id.")
	}
	return "op_" + stamp + "_" + hex.EncodeToString(buf), nil
}
