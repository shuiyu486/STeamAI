package caseshim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectRepoCaseShimReady(t *testing.T) {
	readiness := Inspect(repoRoot(t))
	counts := ReadinessCountsFor(readiness)
	if !readiness.Ready || readiness.Summary != "case shim readiness ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected case shim readiness: %+v", readiness)
	}
	if counts.RequiredPhrases == 0 || counts.CanonicalSkillPhrases == 0 || counts.ForbiddenStrings == 0 || counts.Boundaries == 0 {
		t.Fatalf("case shim readiness omitted required sections: %+v", readiness)
	}
	assertPhrasePresent(t, readiness.RequiredPhrases, "Go-native backend")
	assertPhrasePresent(t, readiness.RequiredPhrases, "不展示底层脚本或 CLI 命令")
	assertPhrasePresent(t, readiness.RequiredPhrases, "新会话 first screen 先使用 `/rekit`")
	assertPhrasePresent(t, readiness.RequiredPhrases, "durable artifacts 接手")
	assertPhrasePresent(t, readiness.RequiredPhrases, "next-batch action queues")
	assertPhrasePresent(t, readiness.CanonicalSkillPhrases, "底层 Go CLI 是 canonical runtime")
	assertBoundaryPresent(t, readiness.Boundaries, "first screen")
	assertForbiddenAbsent(t, readiness.ForbiddenStrings, "rekit.ps1")
	assertForbiddenAbsent(t, readiness.ForbiddenStrings, "go run")
}

func TestInspectInstalledDetectsTemplateMatchAndDrift(t *testing.T) {
	repo := t.TempDir()
	caseRoot := t.TempDir()
	writeRepoText(t, repo, TemplateRelPath, "thin shim\n")
	writeCaseText(t, caseRoot, ".claude/skills/rekit/SKILL.md", "thin shim\n")

	ready := InspectInstalled(repo, caseRoot)
	if !ready.Ready || !ready.MatchesTemplate || ready.Summary != "installed case shim readiness ok" || len(ready.Warnings) != 0 {
		t.Fatalf("unexpected installed shim readiness: %+v", ready)
	}

	writeCaseText(t, caseRoot, ".claude/skills/rekit/SKILL.md", "drift\n")
	drift := InspectInstalled(repo, caseRoot)
	if drift.Ready || drift.MatchesTemplate || drift.Summary != "installed case shim readiness has warnings" || len(drift.Warnings) == 0 || !strings.Contains(drift.Warnings[0], "shim differs") {
		t.Fatalf("unexpected installed shim drift readiness: %+v", drift)
	}
}

func TestAssertReadyDetectsPowerShellOrGoCommandLeakage(t *testing.T) {
	repo := t.TempDir()
	writeRepoText(t, repo, TemplateRelPath, strings.Join([]string{
		"case-local 薄 shim",
		"不包含业务逻辑",
		"canonical `/rekit`",
		".rekit/instance.yml",
		".re-template.yml",
		"<templateRoot>/.claude/skills/rekit/SKILL.md",
		"canonical runtime",
		"sync` / `promote` 默认必须 review-first",
		"新会话 first screen 先使用 `/rekit`",
		"status case shim ready=true",
		"installedShimMatchesTemplate=true",
		"durable artifacts 接手",
		"next-batch action queues",
		"不要在本 shim 里维护模板规则",
		"不要读取或修改用户级 `~/.claude/skills`",
		"不要在 shim 中复制逻辑",
		"不展示底层脚本或 CLI 命令",
		"Go-native backend",
		"rekit.ps1",
	}, "\n"))
	writeRepoText(t, repo, CanonicalSkillRelPath, strings.Join(requiredCanonicalSkillPhrases, "\n"))

	err := AssertReady(repo)
	if err == nil || !strings.Contains(err.Error(), "rekit.ps1") {
		t.Fatalf("AssertReady error = %v, want forbidden rekit.ps1", err)
	}
}

func TestAssertReadyDetectsMissingThinShimBoundary(t *testing.T) {
	repo := t.TempDir()
	writeRepoText(t, repo, TemplateRelPath, strings.Join([]string{
		"case-local 薄 shim",
		"不包含业务逻辑",
		"canonical `/rekit`",
	}, "\n"))
	writeRepoText(t, repo, CanonicalSkillRelPath, strings.Join(requiredCanonicalSkillPhrases, "\n"))

	err := AssertReady(repo)
	if err == nil || !strings.Contains(err.Error(), "case shim missing required phrase") {
		t.Fatalf("AssertReady error = %v, want missing phrase diagnostic", err)
	}
}

func assertPhrasePresent(t *testing.T, phrases []PhraseCheck, want string) {
	t.Helper()
	for _, phrase := range phrases {
		if phrase.Phrase == want {
			if !phrase.Present {
				t.Fatalf("phrase %q present=false", want)
			}
			return
		}
	}
	t.Fatalf("missing phrase check %q: %+v", want, phrases)
}

func assertBoundaryPresent(t *testing.T, boundaries []string, want string) {
	t.Helper()
	for _, boundary := range boundaries {
		if strings.Contains(boundary, want) {
			return
		}
	}
	t.Fatalf("missing boundary containing %q: %+v", want, boundaries)
}

func assertForbiddenAbsent(t *testing.T, checks []StringCheck, want string) {
	t.Helper()
	for _, check := range checks {
		if check.Pattern == want {
			if check.Present {
				t.Fatalf("forbidden pattern %q present", want)
			}
			return
		}
	}
	t.Fatalf("missing forbidden check %q: %+v", want, checks)
}

func writeRepoText(t *testing.T, repo, rel, text string) {
	t.Helper()
	writeCaseText(t, repo, rel, text)
}

func writeCaseText(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found while locating repo root")
		}
		wd = parent
	}
}
