package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"

	"github.com/austinjan/aascribe/internal/apperr"
	"github.com/austinjan/aascribe/internal/cli"
)

const (
	ProviderGemini = "gemini"

	EnvFormat            = "AASCRIBE_FORMAT"
	EnvLLMModel          = "AASCRIBE_LLM_MODEL"
	EnvLLMTimeoutSeconds = "AASCRIBE_LLM_TIMEOUT_SECONDS"
)

type File struct {
	LLM      LLMSection      `toml:"llm"`
	Defaults DefaultsSection `toml:"defaults"`
	Index    IndexSection    `toml:"index"`
}

type LLMSection struct {
	Provider       string `toml:"provider"`
	Model          string `toml:"model"`
	APIKeyEnv      string `toml:"api_key_env"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
}

type DefaultsSection struct {
	Format string `toml:"format"`
}

type IndexSection struct {
	MaxFileSize  int64 `toml:"max_file_size"`
	DefaultDepth int   `toml:"default_depth"`
}

type ResolveOptions struct {
	Format         *string
	LLMModel       *string
	TimeoutSeconds *int
}

type Resolved struct {
	ConfigPath string
	LLM        ResolvedLLM
	Defaults   ResolvedDefaults
	Index      ResolvedIndex
}

type ResolvedLLM struct {
	Provider       string
	Model          string
	APIKeyEnv      string
	APIKey         string
	TimeoutSeconds int
}

type ResolvedDefaults struct {
	Format cli.Format
}

type ResolvedIndex struct {
	MaxFileSize  int64
	DefaultDepth int
}

type LookupEnvFunc func(string) (string, bool)

func ConfigPath(storePath string) string {
	return filepath.Join(storePath, "config.toml")
}

func Load(storePath string) (*File, error) {
	configPath := ConfigPath(storePath)
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.ConfigNotFound(configPath)
		}
		return nil, apperr.IOError("Failed to inspect config file: %s.", configPath)
	}

	var cfg File
	meta, err := toml.DecodeFile(configPath, &cfg)
	if err != nil {
		return nil, apperr.InvalidConfig(configPath, "failed to parse TOML: %v", err)
	}
	if err := validate(configPath, meta, &cfg); err != nil {
		return nil, err
	}

	applyConfigDefaults(&cfg)
	return &cfg, nil
}

func Resolve(storePath string, opts ResolveOptions, lookup LookupEnvFunc) (*Resolved, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	lookup = withDotEnvFallback(storePath, lookup)

	cfg, err := Load(storePath)
	if err != nil {
		return nil, err
	}

	resolved := &Resolved{
		ConfigPath: ConfigPath(storePath),
		LLM: ResolvedLLM{
			Provider:       cfg.LLM.Provider,
			Model:          cfg.LLM.Model,
			APIKeyEnv:      cfg.LLM.APIKeyEnv,
			TimeoutSeconds: cfg.LLM.TimeoutSeconds,
		},
		Defaults: ResolvedDefaults{
			Format: cli.Format(cfg.Defaults.Format),
		},
		Index: ResolvedIndex{
			MaxFileSize:  cfg.Index.MaxFileSize,
			DefaultDepth: cfg.Index.DefaultDepth,
		},
	}

	if value, ok := lookup(EnvFormat); ok && value != "" {
		format, err := parseFormat(value)
		if err != nil {
			return nil, apperr.InvalidConfig(resolved.ConfigPath, "%s", err.Error())
		}
		resolved.Defaults.Format = format
	}
	if value, ok := lookup(EnvLLMModel); ok && value != "" {
		resolved.LLM.Model = value
	}
	if value, ok := lookup(EnvLLMTimeoutSeconds); ok && value != "" {
		timeout, err := strconv.Atoi(value)
		if err != nil || timeout <= 0 {
			return nil, apperr.InvalidConfig(resolved.ConfigPath, "%s must be a positive integer.", EnvLLMTimeoutSeconds)
		}
		resolved.LLM.TimeoutSeconds = timeout
	}

	if opts.Format != nil {
		format, err := parseFormat(*opts.Format)
		if err != nil {
			return nil, apperr.InvalidConfig(resolved.ConfigPath, "%s", err.Error())
		}
		resolved.Defaults.Format = format
	}
	if opts.LLMModel != nil && *opts.LLMModel != "" {
		resolved.LLM.Model = *opts.LLMModel
	}
	if opts.TimeoutSeconds != nil {
		if *opts.TimeoutSeconds <= 0 {
			return nil, apperr.InvalidConfig(resolved.ConfigPath, "timeout_seconds must be a positive integer.")
		}
		resolved.LLM.TimeoutSeconds = *opts.TimeoutSeconds
	}

	apiKey, ok := lookup(resolved.LLM.APIKeyEnv)
	if !ok || apiKey == "" {
		return nil, apperr.MissingSecret(resolved.LLM.APIKeyEnv)
	}
	resolved.LLM.APIKey = apiKey

	return resolved, nil
}

func applyConfigDefaults(cfg *File) {
	if cfg.Defaults.Format == "" {
		cfg.Defaults.Format = string(cli.FormatJSON)
	}
	if cfg.Index.MaxFileSize == 0 {
		cfg.Index.MaxFileSize = 1_048_576
	}
	if cfg.Index.DefaultDepth == 0 {
		cfg.Index.DefaultDepth = 3
	}
}

func validate(configPath string, meta toml.MetaData, cfg *File) error {
	if !meta.IsDefined("llm") {
		return apperr.InvalidConfig(configPath, "missing [llm] section")
	}
	if cfg.LLM.Provider == "" {
		return apperr.InvalidConfig(configPath, "missing llm.provider")
	}
	if cfg.LLM.Provider != ProviderGemini {
		return apperr.InvalidConfig(configPath, "unsupported llm.provider %q", cfg.LLM.Provider)
	}
	if cfg.LLM.Model == "" {
		return apperr.InvalidConfig(configPath, "missing llm.model")
	}
	if cfg.LLM.APIKeyEnv == "" {
		return apperr.InvalidConfig(configPath, "missing llm.api_key_env")
	}
	if cfg.LLM.TimeoutSeconds <= 0 {
		return apperr.InvalidConfig(configPath, "llm.timeout_seconds must be a positive integer")
	}
	if cfg.Defaults.Format != "" {
		if _, err := parseFormat(cfg.Defaults.Format); err != nil {
			return apperr.InvalidConfig(configPath, "%s", err.Error())
		}
	}
	if cfg.Index.MaxFileSize < 0 {
		return apperr.InvalidConfig(configPath, "index.max_file_size must be zero or positive")
	}
	if cfg.Index.DefaultDepth < 0 {
		return apperr.InvalidConfig(configPath, "index.default_depth must be zero or positive")
	}
	return nil
}

func parseFormat(value string) (cli.Format, error) {
	switch value {
	case string(cli.FormatJSON):
		return cli.FormatJSON, nil
	case string(cli.FormatText):
		return cli.FormatText, nil
	default:
		return "", apperr.InvalidArguments("invalid format %q; expected json or text", value)
	}
}

func withDotEnvFallback(storePath string, primary LookupEnvFunc) LookupEnvFunc {
	dotEnvValues := loadDotEnvValues(storePath)
	return func(key string) (string, bool) {
		if value, ok := primary(key); ok && value != "" {
			return value, true
		}
		value, ok := dotEnvValues[key]
		return value, ok
	}
}

func loadDotEnvValues(storePath string) map[string]string {
	values := map[string]string{}

	paths := []string{}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		paths = append(paths, filepath.Join(cwd, ".env"))
	}
	if storePath != "" {
		paths = append(paths, filepath.Join(storePath, ".env"))
	}

	for _, path := range paths {
		parsed, err := godotenv.Read(path)
		if err != nil {
			continue
		}
		for key, value := range parsed {
			values[key] = value
		}
	}

	return values
}
