package defaultdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectRepoPublicDefaultDocsReady(t *testing.T) {
	readiness := Inspect(repoRoot(t))
	if !readiness.Ready || readiness.Summary != "public default docs readiness ok" || len(readiness.Warnings) != 0 {
		t.Fatalf("unexpected public default docs readiness: %+v", readiness)
	}
	assertDocument(t, readiness, "README.md")
	assertDocument(t, readiness, ".claude/skills/rekit/SKILL.md")
	assertDocument(t, readiness, "CLAUDE.md")
	assertDocument(t, readiness, "docs/autonomous-goal.md")
	assertPhrase(t, readiness, "README.md", "用户主要指挥主 Agent / Mission Commander")
	assertPhrase(t, readiness, ".claude/skills/rekit/SKILL.md", "底层 Go CLI 是 canonical runtime")
	assertPhrase(t, readiness, "CLAUDE.md", "PowerShell-free / Go-native / 跨平台收敛")
	assertPhrase(t, readiness, "docs/autonomous-goal.md", "默认继续自主推进")
	if len(readiness.ForbiddenCommands) != 0 {
		t.Fatalf("unexpected forbidden public default commands: %+v", readiness.ForbiddenCommands)
	}
}

func TestInspectDetectsPowerShellFacadeCommandSnippet(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	writeFile(t, filepath.Join(repo, "README.md"), readyREADME+"\n```powershell\n./rekit/rekit.ps1 -Command doctor\n```\n")

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("public default docs unexpectedly ready despite PowerShell command snippet: %+v", readiness)
	}
	if len(readiness.ForbiddenCommands) != 1 || !strings.Contains(readiness.ForbiddenCommands[0].Snippet, "rekit.ps1") {
		t.Fatalf("unexpected forbidden command diagnostics: %+v", readiness.ForbiddenCommands)
	}
	assertWarningContains(t, readiness.Warnings, "exposes PowerShell façade command")
}

func TestInspectDetectsMissingMissionControlDefaultPhrase(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	writeFile(t, filepath.Join(repo, "README.md"), "# README\n\nmissing public defaults\n")

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("public default docs unexpectedly ready despite missing phrases: %+v", readiness)
	}
	assertWarningContains(t, readiness.Warnings, "README.md missing required phrase")
}

func assertDocument(t *testing.T, readiness Readiness, path string) {
	t.Helper()
	for _, doc := range readiness.Documents {
		if doc.Path == path {
			if !doc.Present || strings.TrimSpace(doc.Purpose) == "" {
				t.Fatalf("document %s = %+v, want present with purpose", path, doc)
			}
			return
		}
	}
	t.Fatalf("missing document %s: %+v", path, readiness.Documents)
}

func assertPhrase(t *testing.T, readiness Readiness, path, phrase string) {
	t.Helper()
	for _, check := range readiness.RequiredPhrases {
		if check.Path == path && check.Phrase == phrase {
			if !check.Present {
				t.Fatalf("phrase %s/%q present=false", path, phrase)
			}
			return
		}
	}
	t.Fatalf("missing phrase check %s/%q: %+v", path, phrase, readiness.RequiredPhrases)
}

func assertWarningContains(t *testing.T, warnings []string, want string) {
	t.Helper()
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return
		}
	}
	t.Fatalf("warnings missing %q: %v", want, warnings)
}

func writeReadyDocs(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, "README.md"), readyREADME)
	writeFile(t, filepath.Join(repo, ".claude", "skills", "rekit", "SKILL.md"), readySkill)
	writeFile(t, filepath.Join(repo, "CLAUDE.md"), readyClaude)
	writeFile(t, filepath.Join(repo, "docs", "autonomous-goal.md"), readyAutonomousGoal)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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

const readyREADME = `# README

用户主要指挥主 Agent / Mission Commander。
Go CLI/backend 是背后的 canonical deterministic runtime/API。
` + "`rekit.ps1` 仅作为迁移期 legacy façade。" + `
默认路径继续向 PowerShell-free / Go-native / 跨平台收敛。
这里不需要你手动执行底层脚本。
用户不需要把 ` + "`/rekit`" + ` 子命令当成主要交互界面。
`

const readySkill = `# skill

产品方向是 Mission Control。
底层 Go CLI 是 canonical runtime。
` + "`rekit.ps1` 只是迁移期 legacy façade。" + `
底层 runtime 只作为 ` + "`/rekit`" + ` 的内部实现。
`

const readyClaude = `# CLAUDE

PowerShell-free / Go-native / 跨平台收敛。
PowerShell replacement/removal 不再因“删除 PowerShell”本身停下询问。
默认远程 CI 入口是 ` + "`.github/workflows/release-gate.yml`" + `，在 Linux、Windows、macOS 上运行 Go-native release checks。
`

const readyAutonomousGoal = `# goal

PowerShell-free / Go-native / 跨平台。
每轮自主推进按这个循环做。
默认继续自主推进。
`
