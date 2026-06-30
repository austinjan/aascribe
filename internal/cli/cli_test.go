package cli

import "testing"

func TestParseInstallSkillsTargets(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		user   bool
		folder string
	}{
		{
			name: "user target",
			args: []string{"install", "skills", "-user"},
			user: true,
		},
		{
			name:   "short folder target",
			args:   []string{"install", "skills", "-f", "~/code"},
			folder: "~/code",
		},
		{
			name:   "long folder target",
			args:   []string{"install", "skills", "--folder", "/tmp/project"},
			folder: "/tmp/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(tt.args)
			if err != nil {
				t.Fatalf("parse install skills: %v", err)
			}
			cmd, ok := parsed.Command.(InstallSkillsCommand)
			if !ok {
				t.Fatalf("expected InstallSkillsCommand, got %T", parsed.Command)
			}
			if cmd.User != tt.user || cmd.Folder != tt.folder {
				t.Fatalf("unexpected target: %#v", cmd)
			}
		})
	}
}

func TestParseUninstallSkillsTarget(t *testing.T) {
	parsed, err := Parse([]string{"uninstall", "skill", "-f", "~/code"})
	if err != nil {
		t.Fatalf("parse uninstall skill: %v", err)
	}
	cmd, ok := parsed.Command.(UninstallSkillsCommand)
	if !ok {
		t.Fatalf("expected UninstallSkillsCommand, got %T", parsed.Command)
	}
	if cmd.Folder != "~/code" {
		t.Fatalf("expected folder target, got %#v", cmd)
	}
}

func TestParseInstallSkillsRequiresOneTarget(t *testing.T) {
	if _, err := Parse([]string{"install", "skills"}); err == nil {
		t.Fatalf("expected missing target error")
	}
	if _, err := Parse([]string{"install", "skills", "--user", "-f", "."}); err == nil {
		t.Fatalf("expected conflicting target error")
	}
}
