package steamai

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCanonicalGitSourceAcceptsDirtyTrackedSkill(t *testing.T) {
	source := makeCanonicalSource(t)
	git, err := exec.LookPath("git.exe")
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(source, ".claude", "skills", "steamai", "SKILL.md")
	if err := os.WriteFile(skill, []byte("# Dirty but tracked canonical skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateCanonicalSource(source); err != nil {
		t.Fatal(err)
	}
	if err := validateCanonicalGitSource(git, source); err != nil {
		t.Fatalf("dirty tracked skill rejected: %v", err)
	}
}

func TestCanonicalGitSourceRejectsNestedCheckoutAndUntrackedSkill(t *testing.T) {
	source := makeCanonicalSource(t)
	git, err := exec.LookPath("git.exe")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCanonicalGitSource(git, filepath.Join(source, ".claude")); err == nil {
		t.Fatal("nested directory accepted as canonical root")
	}
	if output, err := exec.Command(git, "-C", source, "rm", "--cached", "--quiet", "--", ".claude/skills/steamai/SKILL.md").CombinedOutput(); err != nil {
		t.Fatalf("untrack canonical skill: %v: %s", err, output)
	}
	if err := validateCanonicalGitSource(git, source); err == nil {
		t.Fatal("untracked canonical skill accepted")
	}
}
