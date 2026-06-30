package skill

import (
	"embed"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/austinjan/aascribe/internal/apperr"
)

const (
	IndexingFolderName = "indexing-folder"
	codexUserRootDir   = ".codex"
)

//go:embed assets/indexing-folder/SKILL.md
var skillAssets embed.FS

type Target struct {
	User   bool
	Folder string
}

type InstallResult struct {
	Skill        string   `json:"skill"`
	Scope        string   `json:"scope"`
	TargetRoot   string   `json:"target_root"`
	InstallDir   string   `json:"install_dir"`
	SkillPath    string   `json:"skill_path"`
	BinaryPath   string   `json:"binary_path"`
	WrittenFiles []string `json:"written_files"`
}

type UninstallResult struct {
	Skill         string   `json:"skill"`
	Scope         string   `json:"scope"`
	TargetRoot    string   `json:"target_root"`
	InstallDir    string   `json:"install_dir"`
	Removed       bool     `json:"removed"`
	RemovedPaths  []string `json:"removed_paths"`
	AlreadyAbsent bool     `json:"already_absent"`
}

func Install(target Target) (*InstallResult, error) {
	resolved, err := resolveTarget(target)
	if err != nil {
		return nil, err
	}
	installDir := filepath.Join(resolved.root, "skills", IndexingFolderName)
	binDir := filepath.Join(installDir, "bin")
	skillPath := filepath.Join(installDir, "SKILL.md")
	binaryPath := filepath.Join(binDir, binaryName())

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return nil, apperr.IOError("Failed to create skill install directory: %s.", binDir)
	}

	content, err := skillAssets.ReadFile("assets/indexing-folder/SKILL.md")
	if err != nil {
		return nil, apperr.IOError("Failed to read bundled indexing skill asset.")
	}
	if err := os.WriteFile(skillPath, content, 0o644); err != nil {
		return nil, apperr.IOError("Failed to write skill file: %s.", skillPath)
	}

	executable, err := os.Executable()
	if err != nil {
		return nil, apperr.IOError("Failed to resolve current aascribe executable.")
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, apperr.IOError("Failed to resolve current aascribe executable path.")
	}
	if samePath(executable, binaryPath) {
		if err := os.Chmod(binaryPath, 0o755); err != nil {
			return nil, apperr.IOError("Failed to mark skill binary executable: %s.", binaryPath)
		}
	} else if err := copyExecutable(executable, binaryPath); err != nil {
		return nil, err
	}

	return &InstallResult{
		Skill:      IndexingFolderName,
		Scope:      resolved.scope,
		TargetRoot: resolved.root,
		InstallDir: installDir,
		SkillPath:  skillPath,
		BinaryPath: binaryPath,
		WrittenFiles: []string{
			skillPath,
			binaryPath,
		},
	}, nil
}

func Uninstall(target Target) (*UninstallResult, error) {
	resolved, err := resolveTarget(target)
	if err != nil {
		return nil, err
	}
	installDir := filepath.Join(resolved.root, "skills", IndexingFolderName)
	result := &UninstallResult{
		Skill:      IndexingFolderName,
		Scope:      resolved.scope,
		TargetRoot: resolved.root,
		InstallDir: installDir,
	}

	if _, err := os.Stat(installDir); err != nil {
		if os.IsNotExist(err) {
			result.AlreadyAbsent = true
			return result, nil
		}
		return nil, apperr.IOError("Failed to inspect skill install directory: %s.", installDir)
	}

	paths, err := listPaths(installDir)
	if err != nil {
		return nil, err
	}
	if err := os.RemoveAll(installDir); err != nil {
		return nil, apperr.IOError("Failed to remove skill install directory: %s.", installDir)
	}
	result.Removed = true
	result.RemovedPaths = paths
	return result, nil
}

type resolvedTarget struct {
	scope string
	root  string
}

func resolveTarget(target Target) (*resolvedTarget, error) {
	if target.User == (strings.TrimSpace(target.Folder) != "") {
		return nil, apperr.InvalidArguments("Skill install target requires exactly one target: --user or -f/--folder <folder>.")
	}
	if target.User {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, apperr.IOError("Failed to resolve user home directory.")
		}
		return &resolvedTarget{
			scope: "user",
			root:  filepath.Join(home, codexUserRootDir),
		}, nil
	}
	root, err := expandAndAbs(target.Folder)
	if err != nil {
		return nil, err
	}
	return &resolvedTarget{
		scope: "folder",
		root:  root,
	}, nil
}

func expandAndAbs(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", apperr.InvalidArguments("Folder target must not be empty.")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", apperr.IOError("Failed to resolve user home directory.")
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", apperr.InvalidArguments("Failed to resolve folder target: %s.", path)
	}
	return abs, nil
}

func copyExecutable(source, dest string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return apperr.IOError("Failed to open current aascribe executable: %s.", source)
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return apperr.IOError("Failed to create skill binary directory: %s.", filepath.Dir(dest))
	}
	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return apperr.IOError("Failed to write skill binary: %s.", dest)
	}
	if _, err := destFile.ReadFrom(sourceFile); err != nil {
		_ = destFile.Close()
		return apperr.IOError("Failed to copy skill binary: %s.", dest)
	}
	if err := destFile.Close(); err != nil {
		return apperr.IOError("Failed to close skill binary: %s.", dest)
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return apperr.IOError("Failed to mark skill binary executable: %s.", dest)
	}
	return nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "aascribe.exe"
	}
	return "aascribe"
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	return filepath.Clean(absA) == filepath.Clean(absB)
}

func listPaths(root string) ([]string, error) {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, apperr.IOError("Failed to list skill install directory before removal: %s.", root)
	}
	return paths, nil
}
