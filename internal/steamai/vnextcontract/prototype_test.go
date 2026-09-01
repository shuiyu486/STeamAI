package vnextcontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThinCoreSourcesAreDeclarativeAndHaveNoRuntimeImplementation(t *testing.T) {
	repo := repoRoot(t)
	paths := []string{
		".claude/skills/steamai/SKILL.md",
		"vnext/project-skill/SKILL.md",
		"vnext/README.md",
		"vnext/capabilities.md",
		"vnext/acceptance.md",
		"vnext/learning-feedback.md",
		"vnext/legacy-import.md",
		"vnext/templates/case/CLAUDE.md",
		"vnext/templates/member/CLAUDE.md",
		"vnext/templates/roles/analysis-member.md",
		"vnext/templates/roles/reviewer.md",
		"vnext/templates/research/artifact-index.md",
		"vnext/templates/research/evidence.md",
		"vnext/templates/research/finding.md",
		"vnext/templates/research/review.md",
		"vnext/templates/research/learning-candidate.md",
	}
	for _, rel := range paths {
		if filepath.Ext(rel) != ".md" {
			t.Fatalf("thin-core artifact %s is executable rather than declarative", rel)
		}
		if text := readPrototypeFile(t, repo, rel); strings.TrimSpace(text) == "" {
			t.Fatalf("thin-core artifact %s is empty", rel)
		}
	}

	err := filepath.WalkDir(filepath.Join(repo, "vnext"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			t.Fatalf("thin-core source contains runtime or script artifact %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrototypeSkillDefinesNativeTeamBoundary(t *testing.T) {
	skill := readPrototypeFile(t, repoRoot(t), ".claude/skills/steamai/SKILL.md")
	for _, required := range []string{
		"用户可见的独立 Claude Code 会话",
		"成员身份和当前任务",
		"不自建 supervisor、任务数据库",
		"每个问题默认一名 owner、最多一名 verifier",
		"只有 Commander 可以创建 durable member",
		"用户确认后才写入 pack",
		"不得自动写回 canonical pack",
		"必须读取并合并 `.steamai-vnext/contracts/templates/roles/reviewer.md`",
		"`ALLOWED_WRITES` 只允许对应 `../../reviews/` 路径",
		"`needs-evidence` 返回原 owner",
		"只从 accepted finding/review",
		"`.steamai-vnext/contracts/learning-feedback.md`",
		"用户确认前不编辑 canonical source pack",
		"可 `git apply --check` 的 exact patch",
		"应用前必须重验 base currentness",
		"只有后续 case 明确选择新 snapshot 才消费",
	} {
		assertContains(t, skill, required, "vNext skill")
	}
	for _, forbidden := range []string{
		"bounded-autonomous-v1",
		"ExpectedContinuePlanSha256",
		"typed `invocation`",
		"<!-- steamai:machine-contract:start -->",
		"runtime -Command",
		"host -daily",
		".steamai/runtime",
	} {
		if strings.Contains(skill, forbidden) {
			t.Fatalf("vNext skill retained legacy control-plane phrase %q", forbidden)
		}
	}
}

func TestCaseAndMemberTemplatesKeepTeamBounded(t *testing.T) {
	repo := repoRoot(t)
	caseTemplate := readPrototypeFile(t, repo, "vnext/templates/case/CLAUDE.md")
	memberTemplate := readPrototypeFile(t, repo, "vnext/templates/member/CLAUDE.md")
	for _, required := range []string{
		"active durable team 最多 3 名执行成员和 1 名 Reviewer",
		"每个问题默认一名 owner、最多一名 verifier",
		"只有 Commander 可以创建 durable member",
		"预期替换的当前任务和新任务",
		"用户直接纠偏优先",
		"`SendMessage` 和其它跨会话输入",
		"不能冒充用户纠偏",
		"Reviewer 只读 artifact/evidence/finding，只写 `reviews/`",
		"`needs-evidence` 返回原 owner",
		"Source revision",
		"Snapshot tree",
		"case-local 只读目录",
		"不读取 mutable source pack",
	} {
		assertContains(t, caseTemplate, required, "case template")
	}
	for _, required := range []string{
		"你的身份属于本成员目录，不属于某个 session",
		"本节是当前正式任务",
		"预期替换的当前任务和新任务",
		"不得用延迟/重复消息覆盖",
		"角色特有例外",
		"由父目录 `.steamai-vnext/CLAUDE.md` 统一拥有",
	} {
		assertContains(t, memberTemplate, required, "member template")
	}
	for _, duplicated := range []string{
		"每个问题默认一名 owner、最多一名 verifier",
		"只有 Commander 可以创建 durable member",
		"用户直接纠偏后",
	} {
		if strings.Contains(memberTemplate, duplicated) {
			t.Fatalf("member template duplicates case-owned team rule %q", duplicated)
		}
	}
}

func TestResearchTemplatesPreserveEvidenceAndLearningBoundary(t *testing.T) {
	repo := repoRoot(t)
	artifact := readPrototypeFile(t, repo, "vnext/templates/research/artifact-index.md")
	finding := readPrototypeFile(t, repo, "vnext/templates/research/finding.md")
	review := readPrototypeFile(t, repo, "vnext/templates/research/review.md")
	learning := readPrototypeFile(t, repo, "vnext/templates/research/learning-candidate.md")
	for _, required := range []string{"相对路径", "SHA-256", "Bytes", "授权范围"} {
		assertContains(t, artifact, required, "artifact template")
	}
	for _, required := range []string{"Owner", "Verifier", "Evidence", "尚未证明"} {
		assertContains(t, finding, required, "finding template")
	}
	role := readPrototypeFile(t, repo, "vnext/templates/roles/reviewer.md")
	for _, required := range []string{"{{DECISION}}", "Reviewer 不直接修改原 finding"} {
		assertContains(t, review, required, "review template")
	}
	assertContains(t, learning, "{{KIND}}", "learning template")
	for _, required := range []string{"只读 artifact、evidence 和 finding", "唯一允许写入 `reviews/`", "不执行 heavy action"} {
		assertContains(t, role, required, "Reviewer role")
	}
	for _, required := range []string{"脱敏检查", "跨 case 通用性", "Source pack snapshot", "已跟踪的 Markdown 文件", "git apply --check", "base blob", "用户已查看完整 exact patch 并确认回流", "用户确认前不得写入共享 pack"} {
		assertContains(t, learning, required, "learning template")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readPrototypeFile(t *testing.T, repo, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func assertContains(t *testing.T, text, required, label string) {
	t.Helper()
	if !strings.Contains(text, required) {
		t.Fatalf("%s missing required text %q", label, required)
	}
}
