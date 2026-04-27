package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/config"
)

const layoutVersion = "bootstrap-v1"

var managedDirectories = []string{"short_term", "long_term", "index", "cache", "outputs", "operations"}
var managedFiles = []string{"layout.json"}

type InitOutcome struct {
	Created       bool
	Reinitialized bool
	ConfigPath    string
	ConfigCreated bool
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

	configPath := config.ConfigPath(path)
	configCreated, err := ensureDefaultConfig(configPath)
	if err != nil {
		return nil, err
	}

	return &InitOutcome{
		Created:       !existedBefore,
		Reinitialized: existedBefore && force,
		ConfigPath:    configPath,
		ConfigCreated: configCreated,
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

func ensureDefaultConfig(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, apperr.IOError("Failed to inspect config file: %s.", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, apperr.IOError("Failed to create config directory: %s.", filepath.Dir(path))
	}

	config := `[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30

[defaults]
format = "json"

[index]
max_file_size = 1048576
default_depth = 3
`
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		return false, apperr.IOError("Failed to write default config file: %s.", path)
	}
	return true, nil
}
