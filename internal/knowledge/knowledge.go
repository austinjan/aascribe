// Package knowledge exposes a bounded, read-only snapshot of existing index metadata.
package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/austinjan/aascribe/internal/index"
)

const metadataName = ".aascribe_index_meta.json"
const maxFileBytes = 1024 * 1024

type Source struct {
	Path      string `json:"path"`
	Summary   string `json:"summary"`
	Hash      string `json:"hash"`
	IndexedAt string `json:"indexed_at"`
	Score     int    `json:"score,omitempty"`
}
type Catalog struct {
	root    *os.Root
	path    string
	sources map[string]Source
}

// Open only admits regular, successfully summarized files beneath root.
// The directory owner controls publication by indexing a dedicated export root.
func Open(path string) (*Catalog, error) {
	return open(path, false, nil)
}

// OpenForIndexing permits an empty catalog so a remote client can create its first index.
func OpenForIndexing(path string) (*Catalog, error) { return open(path, true, nil) }

// OpenSelection publishes only the files included in the latest successful index traversal.
func OpenSelection(path string, paths []string) (*Catalog, error) {
	selected := make(map[string]bool, len(paths))
	for _, path := range paths {
		selected[filepath.ToSlash(path)] = true
	}
	return open(path, true, selected)
}

func open(path string, allowEmpty bool, selected map[string]bool) (*Catalog, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	c := &Catalog{root: root, path: path, sources: make(map[string]Source)}
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			switch strings.ToLower(entry.Name()) {
			case ".git", ".aws", ".ssh", ".codex", ".pi", "node_modules", ".venv", "dist", "target":
				if current == path {
					return nil
				}
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != metadataName {
			return nil
		}
		rel, err := filepath.Rel(path, current)
		if err != nil {
			return err
		}
		data, err := c.readBounded(rel, 4*maxFileBytes)
		if err != nil {
			return fmt.Errorf("read index %s: %w", rel, err)
		}
		var meta index.Metadata
		if err := json.Unmarshal(data, &meta); err != nil {
			return fmt.Errorf("invalid index %s: %w", rel, err)
		}
		if meta.Dirty {
			return nil
		}
		for _, file := range meta.Files {
			if file.Status != "ok" || file.Summary == "" || file.ContentHash == "" {
				continue
			}
			// Metadata records a display path prefixed by the indexed root name.
			// Each metadata file describes direct files only; never trust its full path.
			name := filepath.Base(filepath.FromSlash(file.Path))
			if name == "." || name == metadataName || strings.HasPrefix(strings.ToLower(name), ".env") {
				continue
			}
			sourcePath := filepath.Join(filepath.Dir(rel), name)
			info, err := c.root.Lstat(sourcePath)
			if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
				continue
			}
			key := filepath.ToSlash(sourcePath)
			if selected != nil && !selected[key] {
				continue
			}
			summary := []rune(file.Summary)
			if len(summary) > 2000 {
				file.Summary = string(summary[:2000]) + "…"
			}
			c.sources[key] = Source{Path: key, Summary: file.Summary, Hash: file.ContentHash, IndexedAt: file.SummarizedAt}
			if len(c.sources) > 10000 {
				return fmt.Errorf("export exceeds 10000 indexed files")
			}
		}
		return nil
	})
	if err != nil {
		root.Close()
		return nil, err
	}
	if len(c.sources) == 0 && !allowEmpty {
		root.Close()
		return nil, fmt.Errorf("no clean summarized files: run aascribe index on the export directory first")
	}
	return c, nil
}

func (c *Catalog) Close() error     { return c.root.Close() }
func (c *Catalog) RootPath() string { return c.path }
func (c *Catalog) Count() int       { return len(c.sources) }

func (c *Catalog) readBounded(path string, max int64) ([]byte, error) {
	f, err := c.root.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("source must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("source exceeds size limit")
	}
	return data, nil
}

func (c *Catalog) Search(ctx context.Context, query string, limit int) ([]Source, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 || len(query) > 2048 {
		return nil, fmt.Errorf("query must contain 1 to 2048 bytes")
	}
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > 50 {
		return nil, fmt.Errorf("limit must be between 1 and 50")
	}
	result := []Source{}
	for _, source := range c.sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		haystack := strings.ToLower(source.Path + " " + source.Summary)
		for _, term := range terms {
			if strings.Contains(haystack, term) {
				source.Score++
			}
		}
		if source.Score > 0 {
			result = append(result, source)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].Path < result[j].Path
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

type Chunk struct {
	Path            string `json:"path"`
	Text            string `json:"text"`
	Hash            string `json:"hash"`
	Offset          int    `json:"offset"`
	NextOffset      int    `json:"next_offset"`
	TotalCharacters int    `json:"total_characters"`
	Truncated       bool   `json:"truncated"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
}

func (c *Catalog) Read(path string, offset, limit int) (*Chunk, error) {
	source, ok := c.sources[path]
	if !ok {
		return nil, fmt.Errorf("source is not in the published index")
	}
	if limit == 0 {
		limit = 8000
	}
	if offset < 0 || limit < 1 || limit > 16000 {
		return nil, fmt.Errorf("invalid offset or limit (maximum 16000 characters)")
	}
	data, err := c.readBounded(filepath.FromSlash(path), maxFileBytes)
	if err != nil {
		return nil, fmt.Errorf("source is unavailable")
	}
	hash := sha256.Sum256(data)
	if "sha256:"+hex.EncodeToString(hash[:]) != source.Hash {
		return nil, fmt.Errorf("source changed since indexing: re-index and restart the MCP server")
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("source is not UTF-8 text")
	}
	chars := []rune(string(data))
	if offset > len(chars) {
		return nil, fmt.Errorf("offset is beyond the source")
	}
	end := min(offset+limit, len(chars))
	startLine := 1 + strings.Count(string(chars[:offset]), "\n")
	endLine := startLine + strings.Count(string(chars[offset:end]), "\n")
	if end > offset && chars[end-1] == '\n' {
		endLine--
	}
	return &Chunk{Path: path, Text: string(chars[offset:end]), Hash: source.Hash, Offset: offset, NextOffset: end, TotalCharacters: len(chars), Truncated: end < len(chars), StartLine: startLine, EndLine: endLine}, nil
}

func (c *Catalog) List(offset, limit int) ([]Source, error) {
	if offset < 0 || limit < 1 || limit > 50 {
		return nil, fmt.Errorf("invalid list offset or limit")
	}
	sources := make([]Source, 0, len(c.sources))
	for _, source := range c.sources {
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Path < sources[j].Path })
	if offset > len(sources) {
		return []Source{}, nil
	}
	return sources[offset:min(offset+limit, len(sources))], nil
}
