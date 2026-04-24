package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/austinjan/aascribe/internal/apperr"
)

const layoutVersion = "bootstrap-v1"

var managedDirectories = []string{"short_term", "long_term", "index", "cache", "outputs", "operations"}
var managedFiles = []string{"layout.json"}

type InitOutcome struct {
	Created       bool
	Reinitialized bool
}

func ResolveStorePath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if envPath := os.Getenv("AASCRIBE_STORE"); envPath != "" {
		return envPath, nil
	}
	workingDir, err := os.Getwd()
	if err != nil || workingDir == "" {
		return "", apperr.WorkingDirectoryUnavailable()
	}
	return filepath.Join(workingDir, "data", "memory"), nil
}

func InitializeStore(path string, force bool) (*InitOutcome, error) {
	info, err := os.Stat(path)
	existedBefore := err == nil
	if err == nil && !force {
		return nil, apperr.StoreAlreadyExists(path)
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, apperr.IOError("Failed to inspect store root: %s.", path)
	}

	if existedBefore && !info.IsDir() && force {
		if err := os.Remove(path); err != nil {
			return nil, apperr.IOError("Failed to reset managed file: %s.", path)
		}
		existedBefore = false
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, apperr.IOError("Failed to create store root: %s.", path)
	}
	if force {
		if err := resetManagedContents(path); err != nil {
			return nil, err
		}
	}
	for _, dir := range managedDirectories {
		if err := os.MkdirAll(filepath.Join(path, dir), 0o755); err != nil {
			return nil, apperr.IOError("Failed to create store directory: %s.", filepath.Join(path, dir))
		}
	}
	layoutDoc := map[string]any{
		"layout_version": layoutVersion,
		"storage": map[string]string{
			"short_term": "short_term",
			"long_term":  "long_term",
			"index":      "index",
			"cache":      "cache",
			"outputs":    "outputs",
			"operations": "operations",
		},
	}
	layoutBytes, err := json.MarshalIndent(layoutDoc, "", "  ")
	if err != nil {
		return nil, apperr.Serialization("Failed to serialize layout metadata.")
	}
	layoutPath := filepath.Join(path, "layout.json")
	if err := os.WriteFile(layoutPath, layoutBytes, 0o644); err != nil {
		return nil, apperr.IOError("Failed to write layout metadata: %s.", layoutPath)
	}

	return &InitOutcome{
		Created:       !existedBefore,
		Reinitialized: existedBefore && force,
	}, nil
}

func LayoutVersion() string {
	return layoutVersion
}

func resetManagedContents(root string) error {
	for _, dir := range managedDirectories {
		if err := removeIfExists(filepath.Join(root, dir)); err != nil {
			return err
		}
	}
	for _, file := range managedFiles {
		if err := removeIfExists(filepath.Join(root, file)); err != nil {
			return err
		}
	}
	return nil
}

func removeIfExists(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return apperr.IOError("Failed to inspect managed path: %s.", path)
	}
	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return apperr.IOError("Failed to reset managed directory: %s.", path)
		}
		return nil
	}
	if err := os.Remove(path); err != nil {
		return apperr.IOError("Failed to reset managed file: %s.", path)
	}
	return nil
}
