package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/austinjan/aascribe/internal/cli"
)

func TestSkillDefaultsRepositoryQuestionsToReadOnlyDiscovery(t *testing.T) {
	skill := readIndexingSkill(t)
	readOnly := markdownSection(t, skill, "## Read-Only Repository Questions (Default)")

	for _, mutatingCommand := range []string{`"$AASCRIBE" init`, `"$AASCRIBE" index `} {
		if strings.Contains(readOnly, mutatingCommand) {
			t.Fatalf("default read-only workflow must not contain mutating command %q", mutatingCommand)
		}
	}
	if !strings.Contains(readOnly, "Do not run") {
		t.Fatal("default read-only workflow must explicitly prohibit repository mutation without authorization")
	}
}

func TestSkillRoutesKnownEvidenceWithoutRequiringMapFirst(t *testing.T) {
	skill := readIndexingSkill(t)
	routing := markdownSection(t, skill, "## Intent-Based Routing")

	for _, required := range []string{
		"Known file or location",
		"read it directly",
		"Known literal, symbol, path, or error",
		"search first",
		"Unknown terminology or broad concept",
		"map",
	} {
		if !strings.Contains(routing, required) {
			t.Errorf("intent-based routing is missing %q", required)
		}
	}
	if strings.Index(routing, "read it directly") > strings.Index(routing, "map") {
		t.Error("known-file branch must route to direct inspection without requiring map first")
	}
}

func TestSkillRequiresCompleteUntruncatedScopeForAbsenceClaims(t *testing.T) {
	skill := readIndexingSkill(t)
	claims := markdownSection(t, skill, "## Absence Claims")

	for _, required := range []string{
		"adequate declared scope",
		"`complete: true`",
		"`truncated: false`",
		"Do not claim absence",
	} {
		if !strings.Contains(claims, required) {
			t.Errorf("absence-claim contract is missing %q", required)
		}
	}
}

func TestSkillSeparatesExplicitIndexRequestsFromReadOnlyQuestions(t *testing.T) {
	skill := readIndexingSkill(t)
	readOnly := markdownSection(t, skill, "## Read-Only Repository Questions (Default)")
	mutating := markdownSection(t, skill, "## Explicitly Mutating Index Workflow")
	if !strings.Contains(readOnly, "Missing metadata is not permission") || !strings.Contains(readOnly, "freshness") && !strings.Contains(readOnly, "cannot be trusted") {
		t.Fatal("read-only branch must preserve discovery when metadata is missing or stale")
	}
	for _, required := range []string{"only when the user explicitly asks", `"$AASCRIBE" index <folder>`, "--refresh"} {
		if !strings.Contains(mutating, required) {
			t.Errorf("explicit mutation branch is missing %q", required)
		}
	}
}

func TestBundledSkillBinaryMatchesRetrievalContract(t *testing.T) {
	repoRoot := repositoryRoot(t)
	binary := filepath.Join(repoRoot, "skills", "indexing-folder", "bin", "aascribe")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	for _, topic := range []string{"map", "search"} {
		command := exec.Command(binary, topic, "--help")
		got, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("run bundled %s help: %v\n%s", topic, err, got)
		}
		want := cli.HelpTextForTopic(topic) + "\n"
		if string(got) != want {
			t.Fatalf("bundled %s help does not match source contract\n--- bundled ---\n%s\n--- source ---\n%s", topic, got, want)
		}
	}
	versionOutput, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("run bundled version: %v\n%s", err, versionOutput)
	}
	if got, want := strings.TrimSpace(string(versionOutput)), cli.Version; got != want {
		t.Fatalf("bundled version %q does not match source version %q", got, want)
	}

	schema, err := os.ReadFile(filepath.Join(repoRoot, "docs", "shapes", "PathIndexMap.schema.json"))
	if err != nil {
		t.Fatalf("read PathIndexMap schema: %v", err)
	}
	for _, field := range []string{`"projection"`, `"omitted"`, `"freshness_checked"`} {
		if !strings.Contains(string(schema), field) {
			t.Errorf("PathIndexMap schema is missing %s", field)
		}
	}
}

func readIndexingSkill(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), "skills", "indexing-folder", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read indexing skill: %v", err)
	}
	return string(content)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate contract test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
}

func markdownSection(t *testing.T, document, heading string) string {
	t.Helper()
	start := strings.Index(document, heading)
	if start < 0 {
		t.Fatalf("missing required section %q", heading)
	}
	rest := document[start+len(heading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}
