package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

const (
	MethodBase       = "base"
	MethodSummary    = "summary"
	MethodClassifyV1 = "builtin.classify.v1"
	MethodRoutingV1  = "builtin.routing.v1"
)

type MethodStage string

const (
	MethodStageFile   MethodStage = "file"
	MethodStageFolder MethodStage = "folder"
	MethodStageStart  MethodStage = "start"
	MethodStageEnd    MethodStage = "end"
)

type MethodStatus string

const (
	MethodStatusOK      MethodStatus = "ok"
	MethodStatusFailed  MethodStatus = "failed"
	MethodStatusSkipped MethodStatus = "skipped"
)

type MethodFields map[string]any

type MethodOutput struct {
	Stage       MethodStage  `json:"stage"`
	GeneratedAt string       `json:"generated_at"`
	InputHash   string       `json:"input_hash"`
	Status      MethodStatus `json:"status"`
	Fields      MethodFields `json:"fields,omitempty"`
	Error       string       `json:"error,omitempty"`
}

type MethodDescriptor struct {
	ID                string         `json:"id"`
	Stage             MethodStage    `json:"stage"`
	Scope             string         `json:"scope"`
	Required          bool           `json:"required"`
	DefaultEnabled    bool           `json:"default_enabled"`
	AlwaysEnabled     bool           `json:"always_enabled"`
	RequiresSummary   bool           `json:"requires_summary"`
	SupportsNoSummary bool           `json:"supports_no_summary"`
	Description       string         `json:"description"`
	Purpose           string         `json:"purpose"`
	ConfigSchema      map[string]any `json:"config_schema,omitempty"`
}

type Method interface {
	ID() string
	Fingerprint() string
	Describe() MethodDescriptor
}

type FileMethod interface {
	Method
	BuildFile(ctx context.Context, input FileMethodInput) (MethodFields, error)
}

type FolderMethod interface {
	Method
	BuildFolder(ctx context.Context, input FolderMethodInput) (MethodFields, error)
}

type FileMethodInput struct {
	Path        string `json:"path"`
	FileName    string `json:"file_name"`
	FileType    string `json:"file_type"`
	ContentHash string `json:"content_hash"`
	Summary     string `json:"summary,omitempty"`
	NoSummary   bool   `json:"no_summary"`
	Status      string `json:"status"`
}

type FolderMethodInput struct {
	Path     string         `json:"path"`
	Files    []MetadataFile `json:"files,omitempty"`
	Children []Metadata     `json:"children,omitempty"`
}

type methodRegistry struct {
	fileMethods   map[string]FileMethod
	folderMethods map[string]FolderMethod
	descriptors   map[string]MethodDescriptor
}

func defaultMethodRegistry() methodRegistry {
	reg := methodRegistry{
		fileMethods:   map[string]FileMethod{},
		folderMethods: map[string]FolderMethod{},
		descriptors: map[string]MethodDescriptor{
			MethodBase: {
				ID:                MethodBase,
				Stage:             MethodStageFile,
				Scope:             "file",
				Required:          true,
				DefaultEnabled:    true,
				AlwaysEnabled:     true,
				RequiresSummary:   false,
				SupportsNoSummary: true,
				Description:       "Records required file metadata such as path, size, mod time, content hash, file type, status, skip reason, and error state.",
				Purpose:           "base_metadata",
			},
			MethodSummary: {
				ID:                MethodSummary,
				Stage:             MethodStageFile,
				Scope:             "file",
				Required:          false,
				DefaultEnabled:    true,
				AlwaysEnabled:     false,
				RequiresSummary:   false,
				SupportsNoSummary: false,
				Description:       "Produces file summaries through the configured summarizer and writes the existing top-level summary fields.",
				Purpose:           "file_summary",
			},
		},
	}
	reg.addFile(classifyMethodV1{})
	reg.addFolder(routingMethodV1{})
	return reg
}

func (r methodRegistry) addFile(method FileMethod) {
	r.fileMethods[method.ID()] = method
	r.descriptors[method.ID()] = method.Describe()
}

func (r methodRegistry) addFolder(method FolderMethod) {
	r.folderMethods[method.ID()] = method
	r.descriptors[method.ID()] = method.Describe()
}

func ListMethodDescriptors(stage string) ([]MethodDescriptor, error) {
	reg := defaultMethodRegistry()
	normalizedStage := strings.TrimSpace(stage)
	descriptors := make([]MethodDescriptor, 0, len(reg.descriptors))
	for _, descriptor := range reg.descriptors {
		if normalizedStage != "" && string(descriptor.Stage) != normalizedStage {
			continue
		}
		descriptors = append(descriptors, descriptor)
	}
	sort.Slice(descriptors, func(i, j int) bool {
		if descriptors[i].Stage == descriptors[j].Stage {
			return descriptors[i].ID < descriptors[j].ID
		}
		return descriptors[i].Stage < descriptors[j].Stage
	})
	return descriptors, nil
}

func IsKnownMethodID(id string) bool {
	_, ok := defaultMethodRegistry().descriptors[id]
	return ok
}

func selectedMethods(opts Options) []string {
	seen := map[string]bool{}
	methods := []string{}
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		methods = append(methods, id)
	}
	add(MethodBase)
	if len(opts.MetadataMethods) == 0 {
		if !opts.NoSummary {
			add(MethodSummary)
		}
		return methods
	}
	for _, method := range opts.MetadataMethods {
		if method == MethodSummary && opts.NoSummary {
			continue
		}
		add(method)
	}
	return methods
}

func selectedMethodSet(opts Options) map[string]bool {
	out := map[string]bool{}
	for _, method := range selectedMethods(opts) {
		out[method] = true
	}
	return out
}

func stableInputHash(input any, method Method) string {
	payload := map[string]any{
		"input":       input,
		"method_id":   method.ID(),
		"fingerprint": method.Fingerprint(),
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type classifyMethodV1 struct{}

func (classifyMethodV1) ID() string { return MethodClassifyV1 }

func (classifyMethodV1) Fingerprint() string { return "builtin.classify.v1:deterministic:1" }

func (classifyMethodV1) Describe() MethodDescriptor {
	return MethodDescriptor{
		ID:                MethodClassifyV1,
		Stage:             MethodStageFile,
		Scope:             "file",
		Required:          false,
		DefaultEnabled:    false,
		AlwaysEnabled:     false,
		RequiresSummary:   false,
		SupportsNoSummary: true,
		Description:       "Classifies each text file for agent routing using path, file type, and optional summary signals.",
		Purpose:           "file_classification",
		ConfigSchema: map[string]any{
			"low_confidence_threshold": "number",
		},
	}
}

func (m classifyMethodV1) BuildFile(ctx context.Context, input FileMethodInput) (MethodFields, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	category, confidence, reason := classifyFile(input)
	return MethodFields{
		"category":   category,
		"tags":       classifyTags(input, category),
		"confidence": confidence,
		"reason":     reason,
	}, nil
}

type routingMethodV1 struct{}

func (routingMethodV1) ID() string { return MethodRoutingV1 }

func (routingMethodV1) Fingerprint() string { return "builtin.routing.v1:deterministic:1" }

func (routingMethodV1) Describe() MethodDescriptor {
	return MethodDescriptor{
		ID:                MethodRoutingV1,
		Stage:             MethodStageFolder,
		Scope:             "folder",
		Required:          false,
		DefaultEnabled:    false,
		AlwaysEnabled:     false,
		RequiresSummary:   false,
		SupportsNoSummary: true,
		Description:       "Aggregates child file classifications into folder-level category and routing hints.",
		Purpose:           "folder_routing",
		ConfigSchema: map[string]any{
			"low_confidence_threshold": "number",
			"include_tags":             "boolean",
		},
	}
}

func (m routingMethodV1) BuildFolder(ctx context.Context, input FolderMethodInput) (MethodFields, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	categoryCounts := map[string]int{}
	tagCounts := map[string]int{}
	lowConfidence := 0
	uncategorized := 0

	for _, file := range input.Files {
		output, ok := file.Methods[MethodClassifyV1]
		if !ok || output.Status != MethodStatusOK {
			uncategorized++
			continue
		}
		category, _ := output.Fields["category"].(string)
		if category == "" || category == "unknown" {
			uncategorized++
		} else {
			categoryCounts[category]++
		}
		if confidence, ok := numericField(output.Fields["confidence"]); ok && confidence < 0.6 {
			lowConfidence++
		}
		for _, tag := range stringSliceField(output.Fields["tags"]) {
			tagCounts[tag]++
		}
	}
	for _, child := range input.Children {
		output, ok := child.Methods[MethodRoutingV1]
		if !ok || output.Status != MethodStatusOK {
			continue
		}
		for category, count := range intMapField(output.Fields["category_counts"]) {
			categoryCounts[category] += count
		}
		for tag, count := range intMapField(output.Fields["tag_counts"]) {
			tagCounts[tag] += count
		}
		if count, ok := intField(output.Fields["low_confidence_count"]); ok {
			lowConfidence += count
		}
		if count, ok := intField(output.Fields["uncategorized_count"]); ok {
			uncategorized += count
		}
	}

	return MethodFields{
		"category_counts":        categoryCounts,
		"tag_counts":             tagCounts,
		"dominant_category":      dominantKey(categoryCounts),
		"low_confidence_count":   lowConfidence,
		"uncategorized_count":    uncategorized,
		"direct_classified":      sumCounts(categoryCounts),
		"direct_uncategorized":   uncategorized,
		"routing_method_version": "builtin.routing.v1",
	}, nil
}

func classifyFile(input FileMethodInput) (string, float64, string) {
	path := strings.ToLower(filepath.ToSlash(input.Path))
	fileType := strings.ToLower(input.FileType)
	name := strings.ToLower(input.FileName)
	summary := strings.ToLower(input.Summary)

	switch {
	case strings.Contains(path, "_test.") || strings.HasSuffix(path, ".test") || strings.Contains(path, "/test/") || strings.Contains(path, "/tests/"):
		return "test", 0.9, "Path or filename indicates test coverage."
	case strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt") || strings.Contains(path, "/docs/"):
		return "docs", 0.86, "Path or file type indicates documentation."
	case strings.Contains(fileType, "go") || strings.Contains(fileType, "javascript") || strings.Contains(fileType, "typescript") || strings.Contains(fileType, "python") || strings.Contains(path, "/internal/") || strings.Contains(path, "/cmd/"):
		return "source", 0.84, "Path or file type indicates source code."
	case strings.Contains(name, "dockerfile") || strings.Contains(path, "makefile") || strings.Contains(path, ".toml") || strings.Contains(path, ".yaml") || strings.Contains(path, ".yml") || strings.Contains(path, ".json") || strings.Contains(path, ".conf"):
		return "config", 0.78, "Filename or extension indicates configuration."
	case strings.Contains(summary, "schema") || strings.Contains(path, "/schema") || strings.Contains(path, "/shapes/"):
		return "schema", 0.74, "Path or summary indicates schema data."
	default:
		return "unknown", 0.35, "No strong deterministic routing signal matched."
	}
}

func classifyTags(input FileMethodInput, category string) []string {
	tags := []string{}
	add := func(tag string) {
		tag = strings.Trim(strings.ToLower(tag), ".-_ ")
		if tag == "" {
			return
		}
		for _, existing := range tags {
			if existing == tag {
				return
			}
		}
		tags = append(tags, tag)
	}
	add(category)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(input.Path)), ".")
	add(ext)
	for _, part := range strings.Split(filepath.ToSlash(input.Path), "/") {
		switch part {
		case "", ".", "..":
			continue
		}
		if len(tags) >= 6 {
			break
		}
		add(part)
	}
	sort.Strings(tags)
	return tags
}

func dominantKey(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	best := ""
	bestCount := 0
	for _, key := range keys {
		if counts[key] > bestCount {
			best = key
			bestCount = counts[key]
		}
	}
	return best
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func numericField(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		out, err := v.Float64()
		return out, err == nil
	default:
		return 0, false
	}
}

func intField(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		out, err := v.Int64()
		return int(out), err == nil
	default:
		return 0, false
	}
}

func stringSliceField(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := []string{}
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func intMapField(value any) map[string]int {
	out := map[string]int{}
	switch v := value.(type) {
	case map[string]int:
		return v
	case map[string]any:
		for key, item := range v {
			if count, ok := intField(item); ok {
				out[key] = count
			}
		}
	}
	return out
}
