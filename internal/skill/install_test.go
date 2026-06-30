package skill

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallAndUninstallFolderSkill(t *testing.T) {
	root := t.TempDir()

	installed, err := Install(Target{Folder: root})
	if err != nil {
		t.Fatalf("install skill: %v", err)
	}
	if installed.Scope != "folder" {
		t.Fatalf("expected folder scope, got %#v", installed)
	}
	if installed.InstallDir != filepath.Join(root, "skills", IndexingFolderName) {
		t.Fatalf("unexpected install dir: %s", installed.InstallDir)
	}
	assertRegularFile(t, installed.SkillPath)
	assertRegularFile(t, installed.BinaryPath)

	info, err := os.Stat(installed.BinaryPath)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected installed binary to be executable, mode=%v", info.Mode())
	}

	removed, err := Uninstall(Target{Folder: root})
	if err != nil {
		t.Fatalf("uninstall skill: %v", err)
	}
	if !removed.Removed || removed.AlreadyAbsent {
		t.Fatalf("expected removed result, got %#v", removed)
	}
	if _, err := os.Stat(installed.InstallDir); !os.IsNotExist(err) {
		t.Fatalf("expected install dir removed, stat err=%v", err)
	}
	if len(removed.RemovedPaths) == 0 {
		t.Fatalf("expected removed paths to be reported")
	}
}

func TestUninstallFolderSkillAlreadyAbsent(t *testing.T) {
	root := t.TempDir()

	removed, err := Uninstall(Target{Folder: root})
	if err != nil {
		t.Fatalf("uninstall absent skill: %v", err)
	}
	if !removed.AlreadyAbsent || removed.Removed {
		t.Fatalf("expected already absent result, got %#v", removed)
	}
}

func TestInstallUserSkillUsesHomeCodexRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	installed, err := Install(Target{User: true})
	if err != nil {
		t.Fatalf("install user skill: %v", err)
	}
	expectedRoot := filepath.Join(home, codexUserRootDir)
	if installed.TargetRoot != expectedRoot {
		t.Fatalf("expected target root %q, got %q", expectedRoot, installed.TargetRoot)
	}
	assertRegularFile(t, installed.SkillPath)
	assertRegularFile(t, installed.BinaryPath)
}

func assertRegularFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("expected regular file at %s, got mode %v", path, info.Mode())
	}
}
