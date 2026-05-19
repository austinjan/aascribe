package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	MethodBase        = "base"
	MethodSummary     = "summary"
	MethodClassifyV1  = "builtin.classify.v1"
	MethodRetrievalV1 = "builtin.retrieval.v1"
	MethodRoutingV1   = "builtin.routing.v1"
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
	Text        string `json:"text,omitempty"`
	Prompt      Prompt `json:"prompt,omitempty"`
	NoSummary   bool   `json:"no_summary"`
	Status      string `json:"status"`
}

type Prompt struct {
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	Text   string `json:"text,omitempty"`
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
	reg.addFile(retrievalMethodV1{})
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
	hashInput := input
	if fileInput, ok := input.(FileMethodInput); ok {
		fileInput.Text = ""
		fileInput.Prompt.Text = ""
		hashInput = fileInput
	}
	payload := map[string]any{
		"input":       hashInput,
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

type retrievalMethodV1 struct{}

func (retrievalMethodV1) ID() string { return MethodRetrievalV1 }

func (retrievalMethodV1) Fingerprint() string { return "builtin.retrieval.v1:deterministic:2" }

func (retrievalMethodV1) Describe() MethodDescriptor {
	return MethodDescriptor{
		ID:                MethodRetrievalV1,
		Stage:             MethodStageFile,
		Scope:             "file",
		Required:          false,
		DefaultEnabled:    false,
		AlwaysEnabled:     false,
		RequiresSummary:   false,
		SupportsNoSummary: true,
		Description:       "Emits deterministic LLM-facing retrieval hints from path, summary, and bounded file text.",
		Purpose:           "llm_retrieval_hints",
		ConfigSchema: map[string]any{
			"max_terms":   "integer",
			"max_queries": "integer",
		},
	}
}

func (m retrievalMethodV1) BuildFile(ctx context.Context, input FileMethodInput) (MethodFields, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hints := buildRetrievalHints(input)
	fields := MethodFields{
		"retrieval_terms": hints.terms,
		"aliases":         hints.aliases,
		"entities":        hints.entities,
		"likely_queries":  hints.queries,
	}
	if input.Prompt.Path != "" || input.Prompt.SHA256 != "" {
		fields["prompt"] = map[string]string{
			"path":   input.Prompt.Path,
			"sha256": input.Prompt.SHA256,
		}
	}
	if promptWantsWarnings(input.Prompt.Text) {
		fields["warnings"] = []string{}
	}
	return fields, nil
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

type retrievalHints struct {
	terms    []string
	aliases  []map[string]string
	entities []map[string]string
	queries  []string
}

func buildRetrievalHints(input FileMethodInput) retrievalHints {
	source := strings.Join([]string{input.Path, input.Summary, input.Text}, "\n")
	lowerSource := strings.ToLower(source)
	includeSourceQuote := promptWantsSourceQuote(input.Prompt.Text)
	terms := []string{}
	queries := []string{}
	aliases := []map[string]string{}
	entities := []map[string]string{}

	addTerm := func(term string) {
		term = cleanRetrievalPhrase(term)
		if term == "" || isRetrievalNoisePhrase(term) {
			return
		}
		addUniqueString(&terms, term)
	}
	addQuery := func(query string) {
		query = cleanRetrievalPhrase(query)
		if query == "" || isRetrievalNoisePhrase(query) {
			return
		}
		addUniqueString(&queries, query)
	}
	addAlias := func(from, to string) {
		from = cleanRetrievalPhrase(from)
		to = cleanRetrievalPhrase(to)
		if from == "" || to == "" {
			return
		}
		for _, alias := range aliases {
			if alias["from"] == from && alias["to"] == to {
				return
			}
		}
		aliases = append(aliases, map[string]string{"from": from, "to": to})
		addTerm(from)
		addTerm(to)
	}
	addEntity := func(name, role, sourceQuote string) {
		name = cleanRetrievalName(name)
		role = cleanRetrievalPhrase(role)
		if name == "" {
			return
		}
		role = truncatePhrase(role, 10)
		for _, entity := range entities {
			if entity["name"] == name && entity["role"] == role {
				return
			}
		}
		entity := map[string]string{
			"name": name,
			"type": "tool_or_topic",
			"role": role,
		}
		if includeSourceQuote {
			entity["source_quote"] = cleanRetrievalPhrase(sourceQuote)
		}
		entities = append(entities, entity)
		addTerm(name)
		addTerm(role)
		if role != "" {
			addTerm(name + " " + role)
			addQuery("how to set up " + role)
			addQuery("which tool handles " + role)
			addQuery("find " + role)
			addQuery("configure " + role)
		}
		if strings.Contains(lowerSource, "install") {
			addQuery("install " + name)
		}
	}

	for _, part := range strings.FieldsFunc(filepath.ToSlash(input.Path), func(r rune) bool {
		return r == '/' || r == '.' || r == '-' || r == '_'
	}) {
		addTerm(part)
	}

	for _, line := range strings.Split(source, "\n") {
		name, role, sourceQuote, ok := parseRetrievalEntityLine(line)
		if ok {
			addEntity(name, role, sourceQuote)
		}
	}

	if strings.Contains(source, "WSL") || strings.Contains(lowerSource, "windows subsystem for linux") {
		addAlias("WSL", "Windows Subsystem for Linux")
		addTerm("linux development environment")
		addQuery("set up editor on linux")
		addQuery("set up development tools on Windows Subsystem for Linux")
	}
	if strings.Contains(lowerSource, "zed") && strings.Contains(lowerSource, "wsl") {
		addQuery("install zed in wsl")
		addQuery("how to install zed on Windows Subsystem for Linux")
		addQuery("set up zed editor in wsl")
	}
	for _, entity := range entities {
		name := strings.ToLower(entity["name"])
		role := strings.ToLower(entity["role"])
		switch {
		case name == "xh" || strings.Contains(role, "http"):
			addAlias(entity["name"], "HTTP client")
			addAlias(entity["name"], "API client")
			addTerm("API testing")
		case name == "starship" || strings.Contains(role, "prompt"):
			addAlias(entity["name"], "cross-shell prompt")
			addTerm("shell prompt")
		case name == "zoxide" || strings.Contains(role, "cd command"):
			addAlias(entity["name"], "directory navigation")
			addAlias(entity["name"], "fuzzy directory jump")
			addTerm("smarter cd command")
		}
	}

	sort.Strings(terms)
	sort.Slice(aliases, func(i, j int) bool {
		if aliases[i]["from"] == aliases[j]["from"] {
			return aliases[i]["to"] < aliases[j]["to"]
		}
		return aliases[i]["from"] < aliases[j]["from"]
	})
	sort.Slice(entities, func(i, j int) bool {
		if entities[i]["name"] == entities[j]["name"] {
			return entities[i]["role"] < entities[j]["role"]
		}
		return entities[i]["name"] < entities[j]["name"]
	})
	sort.Strings(queries)

	return retrievalHints{
		terms:    limitStrings(terms, 32),
		aliases:  limitStringMaps(aliases, 12),
		entities: limitStringMaps(entities, 16),
		queries:  limitStrings(queries, 24),
	}
}

func parseRetrievalEntityLine(line string) (string, string, string, bool) {
	line = strings.TrimSpace(line)
	if shouldSkipRetrievalLine(line) {
		return "", "", "", false
	}
	trimmed, ok := trimRetrievalListMarker(line)
	if !ok {
		return "", "", "", false
	}
	line = trimmed
	if line == "" {
		return "", "", "", false
	}
	for _, sep := range []string{" - ", " – ", " — ", ": "} {
		if before, after, ok := strings.Cut(line, sep); ok {
			name := cleanRetrievalName(before)
			role := cleanRetrievalPhrase(after)
			if name != "" && role != "" && len(strings.Fields(name)) <= 4 {
				return name, role, line, true
			}
		}
	}
	return "", "", "", false
}

func promptWantsSourceQuote(prompt string) bool {
	return strings.Contains(strings.ToLower(prompt), "source_quote")
}

func promptWantsWarnings(prompt string) bool {
	return strings.Contains(strings.ToLower(prompt), "warnings")
}

func cleanRetrievalName(value string) string {
	value = stripMarkdownLinks(value)
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`*_[](){}<>#")
	value = strings.TrimSpace(strings.ReplaceAll(value, "**", ""))
	return value
}

func cleanRetrievalPhrase(value string) string {
	value = stripMarkdownLinks(value)
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`*_[](){}<>#.,;:")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) < 2 {
		return ""
	}
	return value
}

var markdownLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)

func stripMarkdownLinks(value string) string {
	return markdownLinkPattern.ReplaceAllString(value, "$1")
}

func shouldSkipRetrievalLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "`") {
		return true
	}
	if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
		return true
	}
	if strings.Contains(trimmed, "](") {
		return true
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "example ") ||
		strings.HasPrefix(lower, "example:") ||
		strings.HasPrefix(lower, "examples ") ||
		strings.HasPrefix(lower, "examples:")
}

func trimRetrievalListMarker(line string) (string, bool) {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	dot := strings.Index(line, ". ")
	if dot > 0 && dot <= 3 {
		number := line[:dot]
		for _, ch := range number {
			if ch < '0' || ch > '9' {
				return "", false
			}
		}
		return strings.TrimSpace(line[dot+2:]), true
	}
	return "", false
}

func isRetrievalNoisePhrase(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	if lower == "example" || strings.HasPrefix(lower, "example ") || strings.Contains(lower, "```") {
		return true
	}
	allDigits := true
	for _, ch := range trimmed {
		if ch < '0' || ch > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return true
	}
	if strings.Contains(trimmed, "](") || strings.Contains(trimmed, "[") || strings.Contains(trimmed, "]") {
		return true
	}
	fields := strings.Fields(trimmed)
	if len(fields) > 8 {
		return true
	}
	if len(fields) > 1 && looksLikeShellCommand(fields) {
		return true
	}
	return false
}

func looksLikeShellCommand(fields []string) bool {
	commandNames := map[string]bool{
		"cat": true, "cd": true, "curl": true, "fd": true, "find": true, "grep": true,
		"ls": true, "mkdir": true, "rg": true, "rm": true, "sed": true, "wget": true,
		"xh": true,
	}
	first := strings.Trim(fields[0], "`")
	if commandNames[first] {
		return true
	}
	for _, field := range fields[1:] {
		if strings.HasPrefix(field, "-") || strings.Contains(field, "*") || strings.Contains(field, "|") {
			return true
		}
	}
	return false
}

func truncatePhrase(value string, maxWords int) string {
	words := strings.Fields(value)
	if len(words) <= maxWords {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:maxWords], " ")
}

func addUniqueString(values *[]string, value string) {
	for _, existing := range *values {
		if strings.EqualFold(existing, value) {
			return
		}
	}
	*values = append(*values, value)
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitStringMaps(values []map[string]string, limit int) []map[string]string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
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
