package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/austinjan/aascribe/internal/apperr"
)

var defaultExcludes = []string{".git", "node_modules", "target", "dist", ".venv", metadataFilename}

const metadataFilename = ".aascribe_index_meta.json"

type Options struct {
	Context             context.Context
	Root                string
	Depth               int
	Include             []string
	Exclude             []string
	MaxConcurrency      int
	Refresh             bool
	NoSummary           bool
	MaxFileSize         int64
	Summarizer          SummarizerFunc
	FailureThreshold    int
	FailureNoticeWriter io.Writer
}

type SummarizerFunc func(ctx context.Context, path, content, length, focus string) (string, error)

type PathIndexTree struct {
	Root string          `json:"root"`
	Tree IndexedPathNode `json:"tree"`
}

type CleanResult struct {
	Root         string   `json:"root"`
	RemovedPaths []string `json:"removed_paths"`
	RemovedCount int      `json:"removed_count"`
	DryRun       bool     `json:"dry_run"`
}

type DirtyResult struct {
	Root        string   `json:"root"`
	MarkedPaths []string `json:"marked_paths"`
	MarkedCount int      `json:"marked_count"`
}

type EvalResult struct {
	Root    string       `json:"root"`
	Folders []EvalFolder `json:"folders"`
	Files   []EvalFile   `json:"files"`
}

type EvalFolder struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type EvalFile struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type PathIndexMap struct {
	Root       string            `json:"root"`
	StateGuide map[string]string `json:"state_guide,omitempty"`
	Map        IndexMapNode      `json:"map"`
}

type IndexMapNode struct {
	Path              string         `json:"path"`
	State             string         `json:"state"`
	FolderDescription string         `json:"folder_description,omitempty"`
	BriefSummary      string         `json:"brief_summary,omitempty"`
	Stats             MetadataStats  `json:"stats,omitempty"`
	Files             []IndexMapFile `json:"files,omitempty"`
	Children          []IndexMapNode `json:"children,omitempty"`
}

type IndexMapFile struct {
	Path     string `json:"path"`
	FileType string `json:"file_type,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Status   string `json:"status,omitempty"`
}

type IndexedPathNode struct {
	Path         string            `json:"path"`
	Type         string            `json:"type"`
	Size         int64             `json:"size,omitempty"`
	Hash         string            `json:"hash,omitempty"`
	Summary      string            `json:"summary,omitempty"`
	SummarizedAt string            `json:"summarized_at,omitempty"`
	Children     []IndexedPathNode `json:"children,omitempty"`
}

type FileDescription struct {
	Path        string `json:"path"`
	Summary     string `json:"summary"`
	Length      string `json:"length"`
	Focus       string `json:"focus,omitempty"`
	GeneratedAt string `json:"generated_at,omitempty"`
}

type Metadata struct {
	Version           string           `json:"version"`
	FolderPath        string           `json:"folder_path"`
	LastUpdated       string           `json:"last_updated"`
	Dirty             bool             `json:"dirty,omitempty"`
	FolderDescription string           `json:"folder_description,omitempty"`
	BriefSummary      string           `json:"brief_summary,omitempty"`
	Stats             MetadataStats    `json:"stats,omitempty"`
	Files             []MetadataFile   `json:"files,omitempty"`
	NotIndexedFiles   []NotIndexedFile `json:"not_indexed_files,omitempty"`
	FailedFiles       []FailedFile     `json:"failed_files,omitempty"`
	Warnings          []string         `json:"warnings,omitempty"`
}

type MetadataStats struct {
	DirectFileCount int `json:"direct_file_count,omitempty"`
	DirectDirCount  int `json:"direct_dir_count,omitempty"`
	IndexedFiles    int `json:"indexed_files,omitempty"`
	NotIndexedFiles int `json:"not_indexed_files,omitempty"`
	FailedFiles     int `json:"failed_files,omitempty"`
}

type MetadataFile struct {
	Path             string `json:"path"`
	Size             int64  `json:"size"`
	ModTime          string `json:"mod_time"`
	ContentHash      string `json:"content_hash"`
	FileType         string `json:"file_type"`
	Summary          string `json:"summary,omitempty"`
	SummarizedAt     string `json:"summarized_at,omitempty"`
	Status           string `json:"status"`
	NotIndexedReason string `json:"not_indexed_reason,omitempty"`
	Error            string `json:"error,omitempty"`
}

type NotIndexedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type FailedFile struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type fileAnalysis struct {
	path        string
	size        int64
	modTime     string
	hash        string
	fileType    string
	summary     string
	generatedAt string
	notIndexed  string
}

type failureTracker struct {
	threshold           int
	writer              io.Writer
	consecutiveFailures int
	warningEmitted      bool
}

type buildRuntime struct {
	ctx           context.Context
	fileWorkers   int
	folderLimiter chan struct{}
}

type fileBuildResult struct {
	node     IndexedPathNode
	metadata Metadata
	err      error
}

type childDirResult struct {
	path string
	node IndexedPathNode
	err  error
}

func Build(opts Options) (*PathIndexTree, error) {
	if opts.Root == "" {
		return nil, apperr.InvalidArguments("index requires exactly one path argument.")
	}
	if opts.MaxFileSize < 0 {
		return nil, apperr.InvalidArguments("index requires a non-negative --max-file-size.")
	}

	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve index path: %s.", opts.Root)
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.PathNotFound(opts.Root)
		}
		return nil, apperr.IOError("Failed to inspect index path: %s.", root)
	}
	if !info.IsDir() {
		return nil, apperr.InvalidArguments("index requires a directory path: %s.", opts.Root)
	}

	matcher := newMatcher(opts.Include, append([]string{}, append(defaultExcludes, opts.Exclude...)...))
	timestamp := time.Now().UTC().Format(time.RFC3339)
	tracker := newFailureTracker(opts.FailureThreshold, opts.FailureNoticeWriter)
	rt := newBuildRuntime(opts.MaxConcurrency)
	if opts.Context != nil {
		rt.ctx = opts.Context
	}
	rootName := filepath.Base(root)
	if rootName == "." || rootName == string(filepath.Separator) || rootName == "" {
		rootName = filepath.Clean(root)
	}

	tree, err := buildDir(root, rootName, 0, opts, matcher, timestamp, tracker, rt, true)
	if err != nil {
		return nil, err
	}

	return &PathIndexTree{
		Root: root,
		Tree: tree,
	}, nil
}

func CleanArtifacts(path string, dryRun bool) (*CleanResult, error) {
	if path == "" {
		return nil, apperr.InvalidArguments("index clean requires exactly one path argument.")
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve clean path: %s.", path)
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.PathNotFound(path)
		}
		return nil, apperr.IOError("Failed to inspect clean path: %s.", root)
	}
	if !info.IsDir() {
		return nil, apperr.InvalidArguments("index clean requires a directory path: %s.", path)
	}

	removed := []string{}
	err = filepath.Walk(root, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != metadataFilename {
			return nil
		}
		removed = append(removed, current)
		if dryRun {
			return nil
		}
		if err := os.Remove(current); err != nil {
			return apperr.IOError("Failed to remove index metadata: %s.", current)
		}
		return nil
	})
	if err != nil {
		if appErr, ok := err.(*apperr.Error); ok {
			return nil, appErr
		}
		return nil, apperr.IOError("Failed to walk clean path: %s.", root)
	}

	return &CleanResult{
		Root:         root,
		RemovedPaths: removed,
		RemovedCount: len(removed),
		DryRun:       dryRun,
	}, nil
}

func MarkDirty(path string) (*DirtyResult, error) {
	if path == "" {
		return nil, apperr.InvalidArguments("index dirty requires exactly one path argument.")
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve dirty path: %s.", path)
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.PathNotFound(path)
		}
		return nil, apperr.IOError("Failed to inspect dirty path: %s.", root)
	}
	if !info.IsDir() {
		return nil, apperr.InvalidArguments("index dirty requires a directory path: %s.", path)
	}

	result := &DirtyResult{Root: root}
	err = filepath.Walk(root, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			return nil
		}

		meta, err := readMetadata(current)
		if err != nil {
			return err
		}
		if meta == nil {
			return nil
		}
		meta.Dirty = true
		if err := writeMetadata(current, meta); err != nil {
			return err
		}
		result.MarkedPaths = append(result.MarkedPaths, current)
		return nil
	})
	if err != nil {
		if appErr, ok := err.(*apperr.Error); ok {
			return nil, appErr
		}
		return nil, apperr.IOError("Failed to walk dirty path: %s.", root)
	}

	sort.Strings(result.MarkedPaths)
	result.MarkedCount = len(result.MarkedPaths)
	return result, nil
}

func Eval(path string) (*EvalResult, error) {
	if path == "" {
		return nil, apperr.InvalidArguments("index eval requires exactly one path argument.")
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve eval path: %s.", path)
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.PathNotFound(path)
		}
		return nil, apperr.IOError("Failed to inspect eval path: %s.", root)
	}
	if !info.IsDir() {
		return nil, apperr.InvalidArguments("index eval requires a directory path: %s.", path)
	}

	matcher := newMatcher(nil, append([]string{}, defaultExcludes...))
	rootName := filepath.Base(root)
	if rootName == "." || rootName == string(filepath.Separator) || rootName == "" {
		rootName = filepath.Clean(root)
	}

	result := &EvalResult{Root: root}
	if err := evalDir(root, rootName, matcher, result); err != nil {
		return nil, err
	}

	sort.Slice(result.Folders, func(i, j int) bool {
		if result.Folders[i].State == result.Folders[j].State {
			return result.Folders[i].Path < result.Folders[j].Path
		}
		return result.Folders[i].State < result.Folders[j].State
	})
	sort.Slice(result.Files, func(i, j int) bool {
		if result.Files[i].State == result.Files[j].State {
			return result.Files[i].Path < result.Files[j].Path
		}
		return result.Files[i].State < result.Files[j].State
	})
	return result, nil
}

func BuildMap(path string) (*PathIndexMap, error) {
	if path == "" {
		return nil, apperr.InvalidArguments("index map requires exactly one path argument.")
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve index map path: %s.", path)
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.PathNotFound(path)
		}
		return nil, apperr.IOError("Failed to inspect index map path: %s.", root)
	}
	if !info.IsDir() {
		return nil, apperr.InvalidArguments("index map requires a directory path: %s.", path)
	}

	matcher := newMatcher(nil, append([]string{}, defaultExcludes...))
	rootName := filepath.Base(root)
	if rootName == "." || rootName == string(filepath.Separator) || rootName == "" {
		rootName = filepath.Clean(root)
	}
	node, err := buildMapNode(root, rootName, matcher)
	if err != nil {
		return nil, err
	}
	return &PathIndexMap{
		Root: root,
		StateGuide: map[string]string{
			"dirty":     "Metadata exists but is marked stale. Re-run index before trusting it.",
			"precision": "Map is a routing overview. For precise answers, inspect the target file directly or re-run index without --no-summary.",
			"ready":     "Metadata exists for this directory. Use the summary and files shown here first.",
			"unindexed": "No metadata file exists for this directory yet. If needed, inspect the directory directly or run index on it.",
		},
		Map: node,
	}, nil
}

func DescribeFile(path, length, focus string) (*FileDescription, error) {
	return DescribeFileWithSummarizer(path, length, focus, nil)
}

func DescribeFileWithSummarizer(path, length, focus string, summarizer SummarizerFunc) (*FileDescription, error) {
	if path == "" {
		return nil, apperr.InvalidArguments("describe requires exactly one file argument.")
	}
	if length == "" {
		length = "medium"
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve describe path: %s.", path)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.PathNotFound(path)
		}
		return nil, apperr.IOError("Failed to inspect describe path: %s.", absPath)
	}
	if info.IsDir() {
		return nil, apperr.InvalidArguments("describe requires a file path: %s.", path)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	analysis, err := analyzeFile(context.Background(), absPath, filepath.ToSlash(path), -1, timestamp, summarizer, length, focus)
	if err != nil {
		return nil, err
	}
	if analysis.notIndexed != "" {
		switch analysis.notIndexed {
		case "binary_file":
			return nil, apperr.InvalidArguments("describe only supports text files: %s.", path)
		case "max_file_size_exceeded":
			return nil, apperr.InvalidArguments("describe cannot summarize files over the size limit: %s.", path)
		default:
			return nil, apperr.InvalidArguments("describe could not summarize file: %s.", path)
		}
	}

	return &FileDescription{
		Path:        filepath.ToSlash(path),
		Summary:     analysis.summary,
		Length:      length,
		Focus:       focus,
		GeneratedAt: timestamp,
	}, nil
}

func loadIgnorePatterns(root string) ([]string, error) {
	var patterns []string
	for _, fileName := range []string{".gitignore", ".aaignore"} {
		filePatterns, err := loadIgnoreFilePatterns(root, fileName)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, filePatterns...)
	}
	return patterns, nil
}

func loadIgnoreFilePatterns(root, fileName string) ([]string, error) {
	path := filepath.Join(root, fileName)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, apperr.IOError("Failed to read %s: %s.", fileName, path)
	}

	lines := strings.Split(normalizeWhitespace(string(content)), "\n")
	patterns := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "!"):
			// First pass only supports blacklist-style ignore entries.
			continue
		}

		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		patterns = append(patterns, filepath.ToSlash(line))
	}
	return patterns, nil
}

func buildDir(absPath, displayPath string, depth int, opts Options, matcher matcher, timestamp string, tracker *failureTracker, rt *buildRuntime, allowParallelChildren bool) (IndexedPathNode, error) {
	if err := runtimeContextErr(rt); err != nil {
		return IndexedPathNode{}, err
	}
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return IndexedPathNode{}, apperr.IOError("Failed to read directory: %s.", absPath)
	}

	localPatterns, err := loadIgnorePatterns(absPath)
	if err != nil {
		return IndexedPathNode{}, err
	}
	localMatcher := matcher.withScopedExcludes(displayPath, localPatterns)
	metadata := &Metadata{
		Version:     "index-meta-v1",
		FolderPath:  absPath,
		LastUpdated: timestamp,
		Dirty:       false,
	}
	priorMetadata, _ := readMetadata(absPath)
	dirEntries := make([]os.DirEntry, 0, len(entries))
	fileEntries := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if err := runtimeContextErr(rt); err != nil {
			return IndexedPathNode{}, err
		}
		name := entry.Name()
		childDisplay := filepath.ToSlash(filepath.Join(displayPath, name))
		if localMatcher.skip(name, childDisplay, entry.IsDir()) {
			continue
		}

		if entry.IsDir() {
			if opts.Depth >= 0 && depth >= opts.Depth {
				continue
			}
			dirEntries = append(dirEntries, entry)
			continue
		}
		fileEntries = append(fileEntries, entry)
	}

	children := make([]IndexedPathNode, 0, len(entries))
	childResults, err := processChildDirs(absPath, displayPath, depth, dirEntries, opts, localMatcher, timestamp, tracker, rt, allowParallelChildren)
	if err != nil {
		return IndexedPathNode{}, err
	}
	for _, result := range childResults {
		if result.err != nil {
			recordFailure(metadata, result.path, result.err.Error())
			recordMetadataFailure(metadata, result.path, result.err.Error())
			tracker.noteFailure(metadata)
			continue
		}
		children = append(children, result.node)
		tracker.noteSuccess()
	}

	if err := runWithFolderLimiter(rt, func() error {
		fileResults, err := processFilesConcurrently(absPath, displayPath, fileEntries, opts, timestamp, priorMetadata, rt)
		if err != nil {
			return err
		}
		for _, result := range fileResults {
			if result.err != nil {
				recordFailure(metadata, result.node.Path, result.err.Error())
				recordMetadataFailure(metadata, result.node.Path, result.err.Error())
				tracker.noteFailure(metadata)
				continue
			}
			children = append(children, result.node)
			mergeMetadata(metadata, &result.metadata)
			tracker.noteSuccess()
		}
		return nil
	}); err != nil {
		return IndexedPathNode{}, err
	}

	sort.Slice(children, func(i, j int) bool {
		if children[i].Type == children[j].Type {
			return children[i].Path < children[j].Path
		}
		return children[i].Type == "dir"
	})

	node := IndexedPathNode{
		Path:     filepath.ToSlash(displayPath),
		Type:     "dir",
		Children: children,
	}
	if !opts.NoSummary {
		node.Summary = summarizeDir(children)
		node.SummarizedAt = timestamp
	}
	metadata.FolderDescription = summarizeDir(children)
	metadata.BriefSummary = summarizeDir(children)
	metadata.Stats = buildMetadataStats(children, metadata)
	if err := writeMetadata(absPath, metadata); err != nil {
		return IndexedPathNode{}, err
	}

	return node, nil
}

func buildFile(absPath, displayPath string, opts Options, timestamp string, metadata *Metadata, priorMetadata *Metadata) (IndexedPathNode, error) {
	if reused, ok, err := reuseFileFromMetadata(absPath, displayPath, opts, metadata, priorMetadata); err != nil {
		return IndexedPathNode{}, err
	} else if ok {
		return reused, nil
	}

	ctx := context.Background()
	if opts.Context != nil {
		ctx = opts.Context
	}
	analysis, err := analyzeFile(ctx, absPath, displayPath, opts.MaxFileSize, timestamp, opts.Summarizer, "medium", "")
	if err != nil {
		return IndexedPathNode{}, err
	}

	node := IndexedPathNode{
		Path: filepath.ToSlash(displayPath),
		Type: "file",
		Size: analysis.size,
		Hash: analysis.hash,
	}
	if opts.NoSummary {
		recordNotIndexed(metadata, displayPath, "no_summary_mode")
		recordMetadataFile(metadata, analysis, "not_indexed", "no_summary_mode", "")
		return node, nil
	}

	if analysis.notIndexed != "" {
		recordNotIndexed(metadata, displayPath, analysis.notIndexed)
		recordMetadataFile(metadata, analysis, "not_indexed", analysis.notIndexed, "")
		return node, nil
	}
	node.Summary = analysis.summary
	node.SummarizedAt = analysis.generatedAt
	recordMetadataFile(metadata, analysis, "ok", "", "")
	return node, nil
}

func processFilesConcurrently(absPath, displayPath string, entries []os.DirEntry, opts Options, timestamp string, priorMetadata *Metadata, rt *buildRuntime) ([]fileBuildResult, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	results := make([]fileBuildResult, len(entries))
	type fileJob struct {
		index int
		entry os.DirEntry
	}
	jobs := make(chan fileJob)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	workerCount := rt.fileWorkers
	if workerCount > len(entries) {
		workerCount = len(entries)
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := runtimeContextErr(rt); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
				name := job.entry.Name()
				childDisplay := filepath.ToSlash(filepath.Join(displayPath, name))
				childAbs := filepath.Join(absPath, name)
				localMeta := &Metadata{}
				node, err := buildFile(childAbs, childDisplay, opts, timestamp, localMeta, priorMetadata)
				results[job.index] = fileBuildResult{
					node:     node,
					metadata: *localMeta,
					err:      err,
				}
				if err != nil {
					results[job.index].node = IndexedPathNode{
						Path: filepath.ToSlash(childDisplay),
						Type: "file",
					}
				}
			}
		}()
	}

sendLoop:
	for i, entry := range entries {
		if err := runtimeContextErr(rt); err != nil {
			close(jobs)
			wg.Wait()
			return nil, err
		}
		select {
		case jobs <- fileJob{index: i, entry: entry}:
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return nil, err
		}
		if len(errCh) > 0 {
			break sendLoop
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].node.Path < results[j].node.Path
	})
	return results, nil
}

func summarizeDir(children []IndexedPathNode) string {
	files := 0
	dirs := 0
	for _, child := range children {
		if child.Type == "dir" {
			dirs++
		} else {
			files++
		}
	}

	switch {
	case files == 0 && dirs == 0:
		return "Empty directory."
	case files == 0:
		return pluralSummary(dirs, "subdirectory", "subdirectories") + "."
	case dirs == 0:
		return pluralSummary(files, "file", "files") + "."
	default:
		return pluralSummary(files, "file", "files") + " and " + pluralSummary(dirs, "subdirectory", "subdirectories") + "."
	}
}

func pluralSummary(count int, singular, plural string) string {
	if count == 1 {
		return "Contains 1 " + singular
	}
	return "Contains " + itoa(count) + " " + plural
}

func summarizeFile(displayPath string, content []byte, maxFileSize int64) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(displayPath)), ".")
	lines := strings.Split(normalizeWhitespace(string(content)), "\n")
	snippet := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		snippet = truncateWords(line, 14)
		break
	}

	switch {
	case ext == "go" && strings.HasPrefix(snippet, "package "):
		return "Go source file in " + strings.TrimPrefix(snippet, "package ") + "."
	case snippet != "":
		return fileTypeLabel(ext) + ": " + snippet
	default:
		return fileTypeLabel(ext) + "."
	}
}

func analyzeFile(ctx context.Context, absPath, displayPath string, maxFileSize int64, timestamp string, summarizer SummarizerFunc, length, focus string) (*fileAnalysis, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, apperr.IOError("Index canceled: %v.", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, apperr.IOError("Failed to inspect file: %s.", absPath)
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, apperr.IOError("Failed to read file: %s.", absPath)
	}

	result := &fileAnalysis{
		path:        filepath.ToSlash(displayPath),
		size:        info.Size(),
		modTime:     info.ModTime().UTC().Format(time.RFC3339),
		hash:        "sha256:" + sha256Hex(content),
		fileType:    detectFileType(displayPath, content),
		generatedAt: timestamp,
	}
	if reason := notIndexedReason(displayPath, content, maxFileSize); reason != "" {
		result.notIndexed = reason
		return result, nil
	}
	if summarizer != nil {
		if err := ctx.Err(); err != nil {
			return nil, apperr.IOError("Index canceled: %v.", err)
		}
		summary, err := summarizer(ctx, filepath.ToSlash(displayPath), string(content), length, focus)
		if err != nil {
			return nil, err
		}
		result.summary = strings.TrimSpace(summary)
	} else {
		result.summary = summarizeFile(displayPath, content, maxFileSize)
		result.summary = adjustSummaryLength(result.summary, length)
		if focus != "" {
			result.summary = result.summary + " Focus: " + focus + "."
		}
	}
	return result, nil
}

func adjustSummaryLength(summary, length string) string {
	words := strings.Fields(summary)
	switch length {
	case "short":
		return trimWords(words, 8)
	case "long":
		if summary == "" {
			return summary
		}
		return summary + " This summary was generated from the current file contents."
	default:
		return trimWords(words, 20)
	}
}

func trimWords(words []string, maxWords int) string {
	if len(words) == 0 {
		return ""
	}
	if len(words) <= maxWords {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:maxWords], " ") + "..."
}

func notIndexedReason(displayPath string, content []byte, maxFileSize int64) string {
	if isBinary(content) {
		return "binary_file"
	}
	if maxFileSize >= 0 && int64(len(content)) > maxFileSize {
		return "max_file_size_exceeded"
	}
	return ""
}

func detectFileType(displayPath string, content []byte) string {
	if isBinary(content) {
		return "binary"
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(displayPath)), ".")
	if ext == "" {
		return "text/plain"
	}
	return "text/" + ext
}

func fileTypeLabel(ext string) string {
	switch ext {
	case "go":
		return "Go source file"
	case "md":
		return "Markdown document"
	case "json":
		return "JSON document"
	case "toml":
		return "TOML config file"
	case "yaml", "yml":
		return "YAML document"
	case "txt":
		return "Text file"
	case "":
		return "File"
	default:
		return strings.ToUpper(ext) + " file"
	}
}

func normalizeWhitespace(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return content
}

func truncateWords(input string, maxWords int) string {
	words := strings.Fields(input)
	if len(words) <= maxWords {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:maxWords], " ") + "..."
}

func isBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	if !utf8.Valid(content) {
		return true
	}
	sample := content
	if len(sample) > 1024 {
		sample = sample[:1024]
	}
	for _, b := range sample {
		if b == 0 {
			return true
		}
	}
	return false
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

type matcher struct {
	include        []string
	exclude        []string
	scopedExcludes []scopedPattern
}

type scopedPattern struct {
	scope   string
	pattern string
}

func newMatcher(include, exclude []string) matcher {
	return matcher{include: include, exclude: exclude}
}

func (m matcher) withScopedExcludes(scope string, patterns []string) matcher {
	if len(patterns) == 0 {
		return m
	}
	next := matcher{
		include:        append([]string(nil), m.include...),
		exclude:        append([]string(nil), m.exclude...),
		scopedExcludes: append([]scopedPattern(nil), m.scopedExcludes...),
	}
	scope = filepath.ToSlash(strings.TrimPrefix(scope, "./"))
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		next.scopedExcludes = append(next.scopedExcludes, scopedPattern{
			scope:   scope,
			pattern: pattern,
		})
	}
	return next
}

func (m matcher) skip(name, displayPath string, isDir bool) bool {
	if matchesAny(m.exclude, name, displayPath) {
		return true
	}
	if matchesScopedAny(m.scopedExcludes, name, displayPath) {
		return true
	}
	if isDir || len(m.include) == 0 {
		return false
	}
	return !matchesAny(m.include, name, displayPath)
}

func matchesScopedAny(patterns []scopedPattern, name, displayPath string) bool {
	for _, scoped := range patterns {
		if scoped.pattern == "" || scoped.scope == "" {
			continue
		}
		if displayPath != scoped.scope && !strings.HasPrefix(displayPath, scoped.scope+"/") {
			continue
		}
		relative := strings.TrimPrefix(displayPath, scoped.scope)
		relative = strings.TrimPrefix(relative, "/")
		if relative == "" {
			relative = name
		}
		if matchesAny([]string{scoped.pattern}, name, relative) {
			return true
		}
	}
	return false
}

func matchesAny(patterns []string, name, displayPath string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		dirPattern := strings.TrimSuffix(pattern, "/")
		if dirPattern != "" && matchesPathScope(displayPath, dirPattern) {
			return true
		}
		if ok, _ := filepath.Match(pattern, name); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, displayPath); ok {
			return true
		}
		if strings.Contains(pattern, "/") {
			if ok, _ := filepath.Match(pattern, displayPath); ok {
				return true
			}
			continue
		}
		for _, part := range strings.Split(displayPath, "/") {
			if ok, _ := filepath.Match(pattern, part); ok {
				return true
			}
		}
	}
	return false
}

func matchesPathScope(displayPath, pattern string) bool {
	if displayPath == pattern || strings.HasPrefix(displayPath, pattern+"/") {
		return true
	}
	return strings.HasSuffix(displayPath, "/"+pattern) || strings.Contains(displayPath, "/"+pattern+"/")
}

func newFailureTracker(threshold int, writer io.Writer) *failureTracker {
	if threshold <= 0 {
		threshold = 3
	}
	return &failureTracker{
		threshold: threshold,
		writer:    writer,
	}
}

func (t *failureTracker) noteFailure(metadata *Metadata) {
	if t == nil {
		return
	}
	t.consecutiveFailures++
	if t.consecutiveFailures < t.threshold || t.warningEmitted {
		return
	}

	warning := fmt.Sprintf("Index encountered %d consecutive failures. Please check the files, permissions, ignore rules, or LLM/config setup.", t.consecutiveFailures)
	if metadata != nil {
		metadata.Warnings = append(metadata.Warnings, warning)
	}
	if t.writer != nil {
		_, _ = fmt.Fprintln(t.writer, warning)
	}
	t.warningEmitted = true
}

func (t *failureTracker) noteSuccess() {
	if t == nil {
		return
	}
	t.consecutiveFailures = 0
	t.warningEmitted = false
}

func recordNotIndexed(metadata *Metadata, displayPath, reason string) {
	if metadata == nil || reason == "" {
		return
	}
	metadata.NotIndexedFiles = append(metadata.NotIndexedFiles, NotIndexedFile{
		Path:   filepath.ToSlash(displayPath),
		Reason: reason,
	})
}

func recordFailure(metadata *Metadata, displayPath, errMsg string) {
	if metadata == nil || errMsg == "" {
		return
	}
	metadata.FailedFiles = append(metadata.FailedFiles, FailedFile{
		Path:  filepath.ToSlash(displayPath),
		Error: errMsg,
	})
}

func recordMetadataFile(metadata *Metadata, analysis *fileAnalysis, status, notIndexedReason, errMsg string) {
	if metadata == nil || analysis == nil {
		return
	}
	metadata.Files = append(metadata.Files, MetadataFile{
		Path:             analysis.path,
		Size:             analysis.size,
		ModTime:          analysis.modTime,
		ContentHash:      analysis.hash,
		FileType:         analysis.fileType,
		Summary:          analysis.summary,
		SummarizedAt:     analysis.generatedAtIfSummarized(),
		Status:           status,
		NotIndexedReason: notIndexedReason,
		Error:            errMsg,
	})
}

func recordMetadataFailure(metadata *Metadata, displayPath, errMsg string) {
	if metadata == nil {
		return
	}
	metadata.Files = append(metadata.Files, MetadataFile{
		Path:   filepath.ToSlash(displayPath),
		Status: "failed",
		Error:  errMsg,
	})
}

func buildMetadataStats(children []IndexedPathNode, metadata *Metadata) MetadataStats {
	stats := MetadataStats{}
	for _, child := range children {
		switch child.Type {
		case "file":
			stats.DirectFileCount++
		case "dir":
			stats.DirectDirCount++
		}
	}
	for _, file := range metadata.Files {
		switch file.Status {
		case "ok":
			stats.IndexedFiles++
		case "failed":
			stats.FailedFiles++
		case "not_indexed":
			stats.NotIndexedFiles++
		}
	}
	return stats
}

func readMetadata(root string) (*Metadata, error) {
	content, err := os.ReadFile(filepath.Join(root, metadataFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, apperr.IOError("Failed to read index metadata: %s.", filepath.Join(root, metadataFilename))
	}

	var meta Metadata
	if err := json.Unmarshal(content, &meta); err != nil {
		return nil, apperr.Serialization("Failed to parse index metadata.")
	}
	return &meta, nil
}

func reuseFileFromMetadata(absPath, displayPath string, opts Options, metadata *Metadata, priorMetadata *Metadata) (IndexedPathNode, bool, error) {
	if opts.Refresh || opts.NoSummary || priorMetadata == nil || priorMetadata.Dirty {
		return IndexedPathNode{}, false, nil
	}

	prior := findMetadataFile(priorMetadata, displayPath)
	if prior == nil {
		return IndexedPathNode{}, false, nil
	}
	if prior.Status == "failed" {
		return IndexedPathNode{}, false, nil
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return IndexedPathNode{}, false, apperr.IOError("Failed to inspect file: %s.", absPath)
	}
	currentModTime := info.ModTime().UTC().Format(time.RFC3339)
	if info.Size() != prior.Size || currentModTime != prior.ModTime {
		ctx := context.Background()
		if opts.Context != nil {
			ctx = opts.Context
		}
		analysis, err := analyzeFile(ctx, absPath, displayPath, opts.MaxFileSize, time.Now().UTC().Format(time.RFC3339), nil, "medium", "")
		if err != nil {
			return IndexedPathNode{}, false, err
		}
		if analysis.hash != prior.ContentHash {
			return IndexedPathNode{}, false, nil
		}
	}

	node := IndexedPathNode{
		Path: filepath.ToSlash(displayPath),
		Type: "file",
		Size: prior.Size,
		Hash: prior.ContentHash,
	}
	if prior.Status == "ok" {
		node.Summary = prior.Summary
		node.SummarizedAt = prior.SummarizedAt
	}
	metadata.Files = append(metadata.Files, *prior)
	if prior.Status == "not_indexed" {
		recordNotIndexed(metadata, displayPath, prior.NotIndexedReason)
	}
	return node, true, nil
}

func findMetadataFile(metadata *Metadata, displayPath string) *MetadataFile {
	if metadata == nil {
		return nil
	}
	normalized := filepath.ToSlash(displayPath)
	for i := range metadata.Files {
		if metadata.Files[i].Path == normalized {
			return &metadata.Files[i]
		}
	}
	return nil
}

func (f *fileAnalysis) generatedAtIfSummarized() string {
	if f == nil || f.summary == "" {
		return ""
	}
	return f.generatedAt
}

func writeMetadata(root string, metadata *Metadata) error {
	content, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return apperr.Serialization("Failed to serialize index metadata.")
	}

	target := filepath.Join(root, metadataFilename)
	temp := target + ".tmp"
	if err := os.WriteFile(temp, content, 0o644); err != nil {
		return apperr.IOError("Failed to write index metadata: %s.", temp)
	}
	if err := os.Rename(temp, target); err != nil {
		return apperr.IOError("Failed to finalize index metadata: %s.", target)
	}
	return nil
}

func buildMapNode(dir, displayPath string, matcher matcher) (IndexMapNode, error) {
	node := IndexMapNode{
		Path: filepath.ToSlash(dir),
	}

	metaPath := filepath.Join(dir, metadataFilename)
	content, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			node.State = "unindexed"
			return node, nil
		}
		return IndexMapNode{}, apperr.IOError("Failed to read index metadata: %s.", metaPath)
	}

	var meta Metadata
	if err := json.Unmarshal(content, &meta); err != nil {
		return IndexMapNode{}, apperr.Serialization("Failed to parse index metadata.")
	}
	if meta.Dirty {
		node.State = "dirty"
	} else {
		node.State = "ready"
	}
	node.FolderDescription = meta.FolderDescription
	node.BriefSummary = meta.BriefSummary
	node.Stats = meta.Stats
	node.Files = projectMapFiles(meta.Files)

	localPatterns, err := loadIgnorePatterns(dir)
	if err != nil {
		return IndexMapNode{}, err
	}
	localMatcher := matcher.withScopedExcludes(displayPath, localPatterns)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return IndexMapNode{}, apperr.IOError("Failed to read directory: %s.", dir)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		childDisplay := filepath.ToSlash(filepath.Join(displayPath, entry.Name()))
		if localMatcher.skip(entry.Name(), childDisplay, true) {
			continue
		}
		childDir := filepath.Join(dir, entry.Name())
		childNode, err := buildMapNode(childDir, childDisplay, localMatcher)
		if err != nil {
			return IndexMapNode{}, err
		}
		node.Children = append(node.Children, childNode)
	}

	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Path < node.Children[j].Path
	})
	return node, nil
}

func projectMapFiles(files []MetadataFile) []IndexMapFile {
	if len(files) == 0 {
		return nil
	}
	projected := make([]IndexMapFile, 0, len(files))
	for _, file := range files {
		projected = append(projected, IndexMapFile{
			Path:     file.Path,
			FileType: file.FileType,
			Summary:  file.Summary,
			Status:   file.Status,
		})
	}
	return projected
}

func newBuildRuntime(maxConcurrency int) *buildRuntime {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.GOMAXPROCS(0)
	}
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	return &buildRuntime{
		ctx:           context.Background(),
		fileWorkers:   maxConcurrency,
		folderLimiter: make(chan struct{}, maxConcurrency),
	}
}

func runWithFolderLimiter(rt *buildRuntime, fn func() error) error {
	if rt == nil || rt.folderLimiter == nil {
		return fn()
	}
	if err := runtimeContextErr(rt); err != nil {
		return err
	}
	select {
	case rt.folderLimiter <- struct{}{}:
	case <-rt.ctx.Done():
		return apperr.IOError("Index canceled: %v.", rt.ctx.Err())
	}
	defer func() { <-rt.folderLimiter }()
	return fn()
}

func mergeMetadata(target, source *Metadata) {
	if target == nil || source == nil {
		return
	}
	target.Files = append(target.Files, source.Files...)
	target.NotIndexedFiles = append(target.NotIndexedFiles, source.NotIndexedFiles...)
	target.FailedFiles = append(target.FailedFiles, source.FailedFiles...)
	target.Warnings = append(target.Warnings, source.Warnings...)
}

func runtimeContextErr(rt *buildRuntime) error {
	if rt == nil || rt.ctx == nil {
		return nil
	}
	if err := rt.ctx.Err(); err != nil {
		return apperr.IOError("Index canceled: %v.", err)
	}
	return nil
}

func processChildDirs(absPath, displayPath string, depth int, entries []os.DirEntry, opts Options, matcher matcher, timestamp string, tracker *failureTracker, rt *buildRuntime, allowParallelChildren bool) ([]childDirResult, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	results := make([]childDirResult, len(entries))
	if !allowParallelChildren {
		for i, entry := range entries {
			if err := runtimeContextErr(rt); err != nil {
				return nil, err
			}
			name := entry.Name()
			childDisplay := filepath.ToSlash(filepath.Join(displayPath, name))
			childAbs := filepath.Join(absPath, name)
			node, err := buildDir(childAbs, childDisplay, depth+1, opts, matcher, timestamp, tracker, rt, false)
			results[i] = childDirResult{
				path: childDisplay,
				node: node,
				err:  err,
			}
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].path < results[j].path
		})
		return results, nil
	}

	type dirJob struct {
		index int
		entry os.DirEntry
	}
	jobs := make(chan dirJob)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	workerCount := rt.fileWorkers
	if workerCount > len(entries) {
		workerCount = len(entries)
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := runtimeContextErr(rt); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
				name := job.entry.Name()
				childDisplay := filepath.ToSlash(filepath.Join(displayPath, name))
				childAbs := filepath.Join(absPath, name)
				node, err := buildDir(childAbs, childDisplay, depth+1, opts, matcher, timestamp, tracker, rt, false)
				results[job.index] = childDirResult{
					path: childDisplay,
					node: node,
					err:  err,
				}
			}
		}()
	}
	for i, entry := range entries {
		if err := runtimeContextErr(rt); err != nil {
			close(jobs)
			wg.Wait()
			return nil, err
		}
		select {
		case jobs <- dirJob{index: i, entry: entry}:
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return nil, err
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	if err := runtimeContextErr(rt); err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].path < results[j].path
	})
	return results, nil
}

func evalDir(absPath, displayPath string, matcher matcher, result *EvalResult) error {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return apperr.IOError("Failed to read directory: %s.", absPath)
	}

	localPatterns, err := loadIgnorePatterns(absPath)
	if err != nil {
		return err
	}
	localMatcher := matcher.withScopedExcludes(displayPath, localPatterns)
	meta, err := readMetadata(absPath)
	if err != nil {
		return err
	}

	currentFiles := map[string]struct{}{}
	folderState := "unchanged"
	folderReason := "unchanged"
	if meta == nil {
		folderState = "needs_index"
		folderReason = "metadata_missing"
	} else if meta.Dirty {
		folderState = "needs_index"
		folderReason = "dirty"
	}

	for _, entry := range entries {
		name := entry.Name()
		childDisplay := filepath.ToSlash(filepath.Join(displayPath, name))
		if localMatcher.skip(name, childDisplay, entry.IsDir()) {
			continue
		}

		childAbs := filepath.Join(absPath, name)
		if entry.IsDir() {
			if err := evalDir(childAbs, childDisplay, localMatcher, result); err != nil {
				return err
			}
			continue
		}

		currentFiles[childDisplay] = struct{}{}
		fileState, fileReason, err := evalFile(childAbs, childDisplay, meta)
		if err != nil {
			return err
		}
		if meta == nil {
			fileState = "needs_index"
			fileReason = "metadata_missing"
		} else if meta.Dirty {
			fileState = "needs_index"
			fileReason = "dirty"
		}
		result.Files = append(result.Files, EvalFile{
			Path:   filepath.ToSlash(childDisplay),
			State:  fileState,
			Reason: fileReason,
		})
		if folderState == "unchanged" && fileState == "needs_index" {
			folderState = "needs_index"
			folderReason = "direct_files_changed"
		}
	}

	if meta != nil {
		for _, prior := range meta.Files {
			if _, ok := currentFiles[prior.Path]; ok {
				continue
			}
			result.Files = append(result.Files, EvalFile{
				Path:   prior.Path,
				State:  "needs_index",
				Reason: "deleted",
			})
			if folderState == "unchanged" {
				folderState = "needs_index"
				folderReason = "direct_files_changed"
			}
		}
	}

	result.Folders = append(result.Folders, EvalFolder{
		Path:   filepath.ToSlash(absPath),
		State:  folderState,
		Reason: folderReason,
	})
	return nil
}

func evalFile(absPath, displayPath string, meta *Metadata) (string, string, error) {
	if meta == nil {
		return "needs_index", "metadata_missing", nil
	}
	prior := findMetadataFile(meta, displayPath)
	if prior == nil {
		return "needs_index", "new", nil
	}
	if prior.Status == "failed" {
		return "needs_index", "previous_failure", nil
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", "", apperr.IOError("Failed to inspect file: %s.", absPath)
	}
	currentModTime := info.ModTime().UTC().Format(time.RFC3339)
	if info.Size() != prior.Size || currentModTime != prior.ModTime {
		analysis, err := analyzeFile(context.Background(), absPath, displayPath, -1, time.Now().UTC().Format(time.RFC3339), nil, "medium", "")
		if err != nil {
			return "", "", err
		}
		if analysis.hash != prior.ContentHash {
			return "needs_index", "modified", nil
		}
	}
	return "unchanged", "unchanged", nil
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
