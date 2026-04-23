package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
)

func TestLoadValidConfig(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
`)

	cfg, err := Load(storePath)
	if err != nil {
		t.Fatalf("expected load success, got %v", err)
	}
	if cfg.LLM.Provider != ProviderGemini {
		t.Fatalf("expected provider %q, got %q", ProviderGemini, cfg.LLM.Provider)
	}
	if cfg.Defaults.Format != "json" {
		t.Fatalf("expected default format json, got %q", cfg.Defaults.Format)
	}
	if cfg.Index.DefaultDepth != 3 {
		t.Fatalf("expected default depth 3, got %d", cfg.Index.DefaultDepth)
	}
}

func TestLoadMissingConfig(t *testing.T) {
	storePath := t.TempDir()
	_, err := Load(storePath)
	assertAppErrorCode(t, err, "CONFIG_NOT_FOUND")
}

func TestLoadMissingLLMSection(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[defaults]
format = "json"
`)

	_, err := Load(storePath)
	assertAppErrorCode(t, err, "INVALID_CONFIG")
}

func TestLoadMissingAPIKeyEnv(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
timeout_seconds = 30
`)

	_, err := Load(storePath)
	assertAppErrorCode(t, err, "INVALID_CONFIG")
}

func TestLoadUnsupportedProvider(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "openai"
model = "gpt"
api_key_env = "OPENAI_API_KEY"
timeout_seconds = 30
`)

	_, err := Load(storePath)
	assertAppErrorCode(t, err, "INVALID_CONFIG")
}

func TestResolveLoadsSecretFromConfiguredEnvVar(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
`)

	resolved, err := Resolve(storePath, ResolveOptions{}, lookupEnv(map[string]string{
		"GEMINI_API_KEY": "secret-token",
	}))
	if err != nil {
		t.Fatalf("expected resolve success, got %v", err)
	}
	if resolved.LLM.APIKey != "secret-token" {
		t.Fatalf("expected secret token, got %q", resolved.LLM.APIKey)
	}
}

func TestResolveMissingSecretFails(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
`)

	_, err := Resolve(storePath, ResolveOptions{}, lookupEnv(map[string]string{}))
	assertAppErrorCode(t, err, "MISSING_SECRET")
}

func TestResolveLoadsSecretFromDotEnvInWorkingDirectory(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
`)

	projectRoot := t.TempDir()
	writeEnvFile(t, filepath.Join(projectRoot, ".env"), "GEMINI_API_KEY=dotenv-secret\n")

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory, got %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("failed to switch working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}()

	resolved, err := Resolve(storePath, ResolveOptions{}, lookupEnv(map[string]string{}))
	if err != nil {
		t.Fatalf("expected resolve success, got %v", err)
	}
	if resolved.LLM.APIKey != "dotenv-secret" {
		t.Fatalf("expected dotenv secret, got %q", resolved.LLM.APIKey)
	}
}

func TestResolveEnvOverridesDotEnv(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
`)

	projectRoot := t.TempDir()
	writeEnvFile(t, filepath.Join(projectRoot, ".env"), "GEMINI_API_KEY=dotenv-secret\n")

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory, got %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("failed to switch working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}()

	resolved, err := Resolve(storePath, ResolveOptions{}, lookupEnv(map[string]string{
		"GEMINI_API_KEY": "shell-secret",
	}))
	if err != nil {
		t.Fatalf("expected resolve success, got %v", err)
	}
	if resolved.LLM.APIKey != "shell-secret" {
		t.Fatalf("expected shell secret override, got %q", resolved.LLM.APIKey)
	}
}

func TestResolveEnvOverridesConfig(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30

[defaults]
format = "json"
`)

	resolved, err := Resolve(storePath, ResolveOptions{}, lookupEnv(map[string]string{
		"GEMINI_API_KEY":     "secret-token",
		EnvFormat:            "text",
		EnvLLMModel:          "gemini-2.5-pro",
		EnvLLMTimeoutSeconds: "45",
	}))
	if err != nil {
		t.Fatalf("expected resolve success, got %v", err)
	}
	if resolved.Defaults.Format != cli.FormatText {
		t.Fatalf("expected text format, got %q", resolved.Defaults.Format)
	}
	if resolved.LLM.Model != "gemini-2.5-pro" {
		t.Fatalf("expected model override, got %q", resolved.LLM.Model)
	}
	if resolved.LLM.TimeoutSeconds != 45 {
		t.Fatalf("expected timeout override, got %d", resolved.LLM.TimeoutSeconds)
	}
}

func TestResolveFlagOverridesEnvAndConfig(t *testing.T) {
	storePath := t.TempDir()
	writeConfig(t, storePath, `
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30

[defaults]
format = "json"
`)

	format := "json"
	model := "gemini-2.5-flash-lite"
	timeout := 60
	resolved, err := Resolve(storePath, ResolveOptions{
		Format:         &format,
		LLMModel:       &model,
		TimeoutSeconds: &timeout,
	}, lookupEnv(map[string]string{
		"GEMINI_API_KEY":     "secret-token",
		EnvFormat:            "text",
		EnvLLMModel:          "gemini-2.5-pro",
		EnvLLMTimeoutSeconds: "45",
	}))
	if err != nil {
		t.Fatalf("expected resolve success, got %v", err)
	}
	if resolved.Defaults.Format != cli.FormatJSON {
		t.Fatalf("expected flag format override, got %q", resolved.Defaults.Format)
	}
	if resolved.LLM.Model != model {
		t.Fatalf("expected flag model override, got %q", resolved.LLM.Model)
	}
	if resolved.LLM.TimeoutSeconds != timeout {
		t.Fatalf("expected flag timeout override, got %d", resolved.LLM.TimeoutSeconds)
	}
}

func TestConfigPath(t *testing.T) {
	storePath := "/tmp/aascribe"
	expected := filepath.Join(storePath, "config.toml")
	if got := ConfigPath(storePath); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func writeConfig(t *testing.T, storePath, body string) {
	t.Helper()
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(ConfigPath(storePath), []byte(body), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
}

func writeEnvFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir env dir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write env failed: %v", err)
	}
}

func lookupEnv(values map[string]string) LookupEnvFunc {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func assertAppErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", want)
	}
	appErr, ok := err.(*apperr.Error)
	if !ok {
		t.Fatalf("expected apperr.Error, got %T", err)
	}
	if appErr.Code != want {
		t.Fatalf("expected error code %s, got %s", want, appErr.Code)
	}
}
