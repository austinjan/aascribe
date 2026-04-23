package llmoutput

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/austinjan/aascribe/internal/apperr"
)

const (
	defaultInlineRuneLimit = 4000
	defaultMaxRetained     = 50
	defaultHeadLines       = 100
	defaultTailLines       = 100
	defaultShowRunes       = 4000
	defaultSliceRunes      = 4000
)

type Config struct {
	InlineRuneLimit   int
	MaxRetained       int
	DefaultHeadLines  int
	DefaultTailLines  int
	DefaultShowRunes  int
	DefaultSliceRunes int
}

type StoredOutputRef struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Command    string    `json:"command"`
	CreatedAt  time.Time `json:"created_at"`
	TotalBytes int64     `json:"total_bytes"`
	TotalRunes int       `json:"total_runes"`
}

type ContinuationHint struct {
	Stored            bool     `json:"stored"`
	Truncated         bool     `json:"truncated"`
	OutputID          string   `json:"output_id,omitempty"`
	InlineRangeStart  int      `json:"inline_range_start"`
	InlineRangeEnd    int      `json:"inline_range_end"`
	TotalBytes        int64    `json:"total_bytes"`
	TotalRunes        int      `json:"total_runes"`
	NextSuggestion    string   `json:"next_suggestion,omitempty"`
	AvailableCommands []string `json:"available_commands,omitempty"`
}

type DeliveredOutput struct {
	InlineText string           `json:"inline_text"`
	StoredRef  *StoredOutputRef `json:"stored_ref,omitempty"`
	Hint       ContinuationHint `json:"hint"`
}

type Chunk struct {
	OutputID   string   `json:"output_id"`
	Text       string   `json:"text"`
	RangeStart int      `json:"range_start"`
	RangeEnd   int      `json:"range_end"`
	TotalBytes int64    `json:"total_bytes"`
	TotalRunes int      `json:"total_runes"`
	Commands   []string `json:"available_commands"`
}

type manifest struct {
	NextID int            `json:"next_id"`
	Items  []manifestItem `json:"items"`
}

type manifestItem struct {
	ID         string    `json:"id"`
	FileName   string    `json:"file_name"`
	Command    string    `json:"command"`
	CreatedAt  time.Time `json:"created_at"`
	TotalBytes int64     `json:"total_bytes"`
	TotalRunes int       `json:"total_runes"`
}

func DefaultConfig() Config {
	return Config{
		InlineRuneLimit:   defaultInlineRuneLimit,
		MaxRetained:       defaultMaxRetained,
		DefaultHeadLines:  defaultHeadLines,
		DefaultTailLines:  defaultTailLines,
		DefaultShowRunes:  defaultShowRunes,
		DefaultSliceRunes: defaultSliceRunes,
	}
}

func Deliver(storePath, command, text string, cfg Config) (*DeliveredOutput, error) {
	cfg = normalizeConfig(cfg)
	totalBytes := int64(len(text))
	totalRunes := len([]rune(text))
	if totalRunes <= cfg.InlineRuneLimit {
		return &DeliveredOutput{
			InlineText: text,
			Hint: ContinuationHint{
				Stored:           false,
				Truncated:        false,
				InlineRangeStart: 0,
				InlineRangeEnd:   totalRunes,
				TotalBytes:       totalBytes,
				TotalRunes:       totalRunes,
			},
		}, nil
	}

	entry, err := persist(storePath, command, text, cfg)
	if err != nil {
		return nil, err
	}
	inlineText, start, end := runeSlice(text, 0, cfg.InlineRuneLimit)
	ref := toStoredOutputRef(storePath, entry)
	return &DeliveredOutput{
		InlineText: inlineText,
		StoredRef:  &ref,
		Hint: ContinuationHint{
			Stored:            true,
			Truncated:         true,
			OutputID:          entry.ID,
			InlineRangeStart:  start,
			InlineRangeEnd:    end,
			TotalBytes:        totalBytes,
			TotalRunes:        totalRunes,
			NextSuggestion:    fmt.Sprintf("aascribe output slice %s --offset %d --limit %d", entry.ID, end, cfg.DefaultSliceRunes),
			AvailableCommands: availableCommands(entry.ID, cfg),
		},
	}, nil
}

func List(storePath string) ([]StoredOutputRef, error) {
	m, err := loadManifest(storePath)
	if err != nil {
		return nil, err
	}
	items := make([]StoredOutputRef, 0, len(m.Items))
	for i := len(m.Items) - 1; i >= 0; i-- {
		items = append(items, toStoredOutputRef(storePath, m.Items[i]))
	}
	return items, nil
}

func Meta(storePath, id string) (*StoredOutputRef, error) {
	entry, err := find(storePath, id)
	if err != nil {
		return nil, err
	}
	ref := toStoredOutputRef(storePath, entry)
	return &ref, nil
}

func Show(storePath, id string, cfg Config) (*Chunk, error) {
	cfg = normalizeConfig(cfg)
	return Slice(storePath, id, 0, cfg.DefaultShowRunes, cfg)
}

func Head(storePath, id string, lines int, cfg Config) (*Chunk, error) {
	cfg = normalizeConfig(cfg)
	if lines <= 0 {
		lines = cfg.DefaultHeadLines
	}
	text, entry, err := loadStoredText(storePath, id)
	if err != nil {
		return nil, err
	}
	chunk := firstLines(text, lines)
	return &Chunk{
		OutputID:   id,
		Text:       chunk,
		RangeStart: 0,
		RangeEnd:   len([]rune(chunk)),
		TotalBytes: entry.TotalBytes,
		TotalRunes: entry.TotalRunes,
		Commands:   availableCommands(id, cfg),
	}, nil
}

func Tail(storePath, id string, lines int, cfg Config) (*Chunk, error) {
	cfg = normalizeConfig(cfg)
	if lines <= 0 {
		lines = cfg.DefaultTailLines
	}
	text, entry, err := loadStoredText(storePath, id)
	if err != nil {
		return nil, err
	}
	chunk, start := lastLines(text, lines)
	return &Chunk{
		OutputID:   id,
		Text:       chunk,
		RangeStart: start,
		RangeEnd:   start + len([]rune(chunk)),
		TotalBytes: entry.TotalBytes,
		TotalRunes: entry.TotalRunes,
		Commands:   availableCommands(id, cfg),
	}, nil
}

func Slice(storePath, id string, offset, limit int, cfg Config) (*Chunk, error) {
	cfg = normalizeConfig(cfg)
	if offset < 0 {
		return nil, apperr.InvalidArguments("output slice requires --offset >= 0.")
	}
	if limit <= 0 {
		limit = cfg.DefaultSliceRunes
	}
	text, entry, err := loadStoredText(storePath, id)
	if err != nil {
		return nil, err
	}
	if offset > entry.TotalRunes {
		return nil, apperr.InvalidArguments("output slice offset %d is outside the stored output range.", offset)
	}
	chunk, start, end := runeSlice(text, offset, limit)
	return &Chunk{
		OutputID:   id,
		Text:       chunk,
		RangeStart: start,
		RangeEnd:   end,
		TotalBytes: entry.TotalBytes,
		TotalRunes: entry.TotalRunes,
		Commands:   availableCommands(id, cfg),
	}, nil
}

func StoreDir(storePath string) string {
	return filepath.Join(storePath, "outputs")
}

func ManifestPath(storePath string) string {
	return filepath.Join(StoreDir(storePath), "manifest.json")
}

func normalizeConfig(cfg Config) Config {
	if cfg.InlineRuneLimit <= 0 {
		cfg.InlineRuneLimit = defaultInlineRuneLimit
	}
	if cfg.MaxRetained <= 0 {
		cfg.MaxRetained = defaultMaxRetained
	}
	if cfg.DefaultHeadLines <= 0 {
		cfg.DefaultHeadLines = defaultHeadLines
	}
	if cfg.DefaultTailLines <= 0 {
		cfg.DefaultTailLines = defaultTailLines
	}
	if cfg.DefaultShowRunes <= 0 {
		cfg.DefaultShowRunes = defaultShowRunes
	}
	if cfg.DefaultSliceRunes <= 0 {
		cfg.DefaultSliceRunes = defaultSliceRunes
	}
	return cfg
}

func persist(storePath, command, text string, cfg Config) (manifestItem, error) {
	if err := os.MkdirAll(StoreDir(storePath), 0o755); err != nil {
		return manifestItem{}, apperr.IOError("Failed to create output directory: %s.", StoreDir(storePath))
	}
	m, err := loadManifest(storePath)
	if err != nil {
		return manifestItem{}, err
	}
	id := fmt.Sprintf("out_%06d", m.NextID)
	fileName := id + ".txt"
	path := filepath.Join(StoreDir(storePath), fileName)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return manifestItem{}, apperr.IOError("Failed to write stored output file: %s.", path)
	}
	entry := manifestItem{
		ID:         id,
		FileName:   fileName,
		Command:    command,
		CreatedAt:  time.Now().UTC(),
		TotalBytes: int64(len(text)),
		TotalRunes: len([]rune(text)),
	}
	m.NextID++
	m.Items = append(m.Items, entry)
	if len(m.Items) > cfg.MaxRetained {
		overflow := len(m.Items) - cfg.MaxRetained
		evicted := m.Items[:overflow]
		for _, item := range evicted {
			_ = os.Remove(filepath.Join(StoreDir(storePath), item.FileName))
		}
		m.Items = m.Items[overflow:]
	}
	if err := saveManifest(storePath, m); err != nil {
		return manifestItem{}, err
	}
	return entry, nil
}

func loadManifest(storePath string) (*manifest, error) {
	path := ManifestPath(storePath)
	bytes, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &manifest{NextID: 1, Items: nil}, nil
	}
	if err != nil {
		return nil, apperr.IOError("Failed to read output manifest: %s.", path)
	}
	var m manifest
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, apperr.InvalidArguments("Output manifest is invalid at %s.", path)
	}
	if m.NextID <= 0 {
		m.NextID = 1
	}
	sort.SliceStable(m.Items, func(i, j int) bool {
		return m.Items[i].CreatedAt.Before(m.Items[j].CreatedAt)
	})
	return &m, nil
}

func saveManifest(storePath string, m *manifest) error {
	bytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return apperr.Serialization("Failed to serialize output manifest.")
	}
	if err := os.WriteFile(ManifestPath(storePath), bytes, 0o644); err != nil {
		return apperr.IOError("Failed to write output manifest: %s.", ManifestPath(storePath))
	}
	return nil
}

func find(storePath, id string) (manifestItem, error) {
	m, err := loadManifest(storePath)
	if err != nil {
		return manifestItem{}, err
	}
	for _, item := range m.Items {
		if item.ID == id {
			return item, nil
		}
	}
	return manifestItem{}, apperr.NotFoundOutput(id)
}

func loadStoredText(storePath, id string) (string, manifestItem, error) {
	entry, err := find(storePath, id)
	if err != nil {
		return "", manifestItem{}, err
	}
	path := filepath.Join(StoreDir(storePath), entry.FileName)
	bytes, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", manifestItem{}, apperr.NotFoundOutput(id)
	}
	if err != nil {
		return "", manifestItem{}, apperr.IOError("Failed to read stored output file: %s.", path)
	}
	return string(bytes), entry, nil
}

func toStoredOutputRef(storePath string, item manifestItem) StoredOutputRef {
	return StoredOutputRef{
		ID:         item.ID,
		Path:       filepath.Join(StoreDir(storePath), item.FileName),
		Command:    item.Command,
		CreatedAt:  item.CreatedAt,
		TotalBytes: item.TotalBytes,
		TotalRunes: item.TotalRunes,
	}
}

func availableCommands(id string, cfg Config) []string {
	return []string{
		fmt.Sprintf("aascribe output show %s", id),
		fmt.Sprintf("aascribe output head %s --lines %d", id, cfg.DefaultHeadLines),
		fmt.Sprintf("aascribe output tail %s --lines %d", id, cfg.DefaultTailLines),
		fmt.Sprintf("aascribe output slice %s --offset %d --limit %d", id, cfg.DefaultSliceRunes, cfg.DefaultSliceRunes),
		fmt.Sprintf("aascribe output meta %s", id),
	}
}

func runeSlice(text string, offset, limit int) (string, int, int) {
	runes := []rune(text)
	if offset < 0 {
		offset = 0
	}
	if offset > len(runes) {
		offset = len(runes)
	}
	end := offset + limit
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[offset:end]), offset, end
}

func firstLines(text string, lines int) string {
	parts := strings.Split(text, "\n")
	if lines >= len(parts) {
		return text
	}
	return strings.Join(parts[:lines], "\n")
}

func lastLines(text string, lines int) (string, int) {
	parts := strings.Split(text, "\n")
	if lines >= len(parts) {
		return text, 0
	}
	selected := strings.Join(parts[len(parts)-lines:], "\n")
	start := len([]rune(strings.Join(parts[:len(parts)-lines], "\n")))
	if start > 0 {
		start++
	}
	return selected, start
}
