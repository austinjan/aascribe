package index

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/austinjan/aascribe/internal/apperr"
)

const IndexPlanVersion = "index-plan-v1"

type IndexPlan struct {
	Version string           `toml:"version" json:"version"`
	Methods IndexPlanMethods `toml:"methods" json:"methods"`
}

type IndexPlanMethods struct {
	Enabled []string             `toml:"enabled" json:"enabled"`
	Custom  []CustomMethodConfig `toml:"custom" json:"custom,omitempty"`
}

type CustomMethodConfig struct {
	ID              string   `toml:"id" json:"id"`
	Stage           string   `toml:"stage" json:"stage"`
	Scope           string   `toml:"scope" json:"scope"`
	RequiresSummary bool     `toml:"requires_summary" json:"requires_summary"`
	Command         []string `toml:"command" json:"command"`
	Timeout         string   `toml:"timeout" json:"timeout,omitempty"`
}

type IndexPlanValidation struct {
	Path   string    `json:"path"`
	Digest string    `json:"digest"`
	Plan   IndexPlan `json:"plan"`
}

func ValidateIndexPlanFile(path string) (*IndexPlanValidation, error) {
	if strings.TrimSpace(path) == "" {
		return nil, apperr.InvalidArguments("index plan validate requires exactly one plan path.")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve index plan path: %s.", path)
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.PathNotFound(path)
		}
		return nil, apperr.IOError("Failed to read index plan: %s.", absPath)
	}
	var plan IndexPlan
	if err := toml.Unmarshal(content, &plan); err != nil {
		return nil, apperr.Serialization("Failed to parse index plan: %s.", err.Error())
	}
	if err := ValidateIndexPlan(plan, filepath.Dir(absPath)); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(content)
	return &IndexPlanValidation{
		Path:   absPath,
		Digest: "sha256:" + hex.EncodeToString(sum[:]),
		Plan:   plan,
	}, nil
}

func ValidateIndexPlan(plan IndexPlan, planDir string) error {
	if plan.Version != IndexPlanVersion {
		return apperr.InvalidArguments("Unsupported index plan version: %s.", plan.Version)
	}
	customIDs := map[string]bool{}
	for _, custom := range plan.Methods.Custom {
		if strings.TrimSpace(custom.ID) == "" {
			return apperr.InvalidArguments("Custom index method requires id.")
		}
		if IsKnownMethodID(custom.ID) {
			return apperr.InvalidArguments("Custom index method id conflicts with built-in method: %s.", custom.ID)
		}
		if customIDs[custom.ID] {
			return apperr.InvalidArguments("Duplicate custom index method id: %s.", custom.ID)
		}
		customIDs[custom.ID] = true
		if !oneOfString(custom.Stage, string(MethodStageFile), string(MethodStageFolder), string(MethodStageStart), string(MethodStageEnd)) {
			return apperr.InvalidArguments("Invalid custom index method stage for %s: %s.", custom.ID, custom.Stage)
		}
		if strings.TrimSpace(custom.Scope) == "" {
			return apperr.InvalidArguments("Custom index method %s requires scope.", custom.ID)
		}
		if len(custom.Command) == 0 || strings.TrimSpace(custom.Command[0]) == "" {
			return apperr.InvalidArguments("Custom index method %s requires command.", custom.ID)
		}
		if err := validatePlanCommandPath(planDir, custom.Command[0]); err != nil {
			return err
		}
	}

	enabled := map[string]bool{}
	for _, method := range plan.Methods.Enabled {
		if strings.TrimSpace(method) == "" {
			return apperr.InvalidArguments("Index plan enabled method id cannot be empty.")
		}
		if enabled[method] {
			return apperr.InvalidArguments("Duplicate enabled index method: %s.", method)
		}
		enabled[method] = true
		if IsKnownMethodID(method) {
			continue
		}
		if customIDs[method] {
			return apperr.InvalidArguments("Custom index method execution is not available in v2: %s.", method)
		}
		return apperr.InvalidArguments("Unknown index method in plan: %s.", method)
	}
	return nil
}

func validatePlanCommandPath(planDir, command string) error {
	if filepath.IsAbs(command) {
		return nil
	}
	cleaned := filepath.Clean(command)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return apperr.InvalidArguments("Invalid custom index method command path: %s.", command)
	}
	if planDir == "" {
		return nil
	}
	resolved := filepath.Clean(filepath.Join(planDir, cleaned))
	if resolved == planDir {
		return apperr.InvalidArguments("Invalid custom index method command path: %s.", command)
	}
	return nil
}

func oneOfString(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
