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
		"vnext/templates/case/CLAUDE.md",
		"vnext/templates/member/CLAUDE.md",
		"vnext/templates/roles/analysis-member.md",
		"vnext/templates/roles/reviewer.md",
		"vnext/templates/research/artifact-index.md",
		"vnext/templates/research/evidence.md",
		"vnext/templates/research/finding.md",
		"vnext/templates/research/review.md",
		"vnext/templates/research/review-round.md",
		"vnext/templates/research/learning-candidate.md",
		"vnext/templates/research/learning-review.md",
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
		"用户确认只授权该 exact tuple",
		"任何 synthetic acceptance 不得自动写回 canonical pack",
		"必须读取并合并 `.steamai-vnext/contracts/templates/roles/reviewer.md`",
		"`ALLOWED_WRITES` 只允许对应 `../../reviews/` 路径",
		"`needs-evidence` 返回原 owner",
		"只从有 current `accepted` review round 的 finding",
		"`.steamai-vnext/contracts/learning-feedback.md`",
		"用户确认前 canonical source pack 零写",
		"可 `git apply --check` 的 exact patch",
		"应用前按合同重验 snapshot",
		"只有后续 case 明确选择新 revision 和新 snapshot digest 才消费",
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
		"`active`：已分配当前正式任务，计入 3 名 execution + 1 名 Reviewer 上限",
		"每个问题默认一名 owner、最多一名 verifier",
		"只有 Commander 可以创建 durable member",
		"预期替换的当前任务和新任务",
		"用户直接纠偏优先",
		"`SendMessage` 和其它跨会话输入",
		"不能冒充用户纠偏",
		"Reviewer 只读 artifact/evidence/finding，只写 `reviews/`",
		"`needs-evidence` 返回原 owner",
		"Source revision",
		"Pack tree",
		"Common tree",
		"Snapshot digest",
		"case-pinned 目录",
		"不读取 mutable source pack",
	} {
		assertContains(t, caseTemplate, required, "case template")
	}
	for _, required := range []string{
		"你的身份属于本成员目录，不属于某个 session",
		"本节是当前正式任务",
		"expected current task 的全部七项与 new current task 的全部七项",
		"返回 `HOLD_STALE_TASK`，零覆盖",
		"角色特有例外",
		"由父目录 `.steamai-vnext/CLAUDE.md` 唯一拥有",
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
	for _, required := range []string{"{{DECISION}}", "{{REVIEW_ROUND}}", "{{FINDING_SHA256}}", "{{REVIEWED_EVIDENCE_REFS_WITH_SHA256}}", "只在文件末尾追加完整 round", "Reviewer 不直接修改原 finding"} {
		assertContains(t, review, required, "review template")
	}
	round := readPrototypeFile(t, repo, "vnext/templates/research/review-round.md")
	for _, required := range []string{"{{REVIEW_ROUND}}", "{{PREVIOUS_REVIEW_ROUND_OR_NONE}}", "{{FINDING_SHA256}}", "不能作为 current `accepted`"} {
		assertContains(t, round, required, "review round template")
	}
	assertContains(t, learning, "{{KIND}}", "learning template")
	for _, required := range []string{"只读 artifact、evidence、finding", "唯一允许写入 `reviews/`", "不执行 heavy action"} {
		assertContains(t, role, required, "Reviewer role")
	}
	for _, required := range []string{"Source finding SHA-256", "Source accepted review", "Pack tree", "Common tree", "Snapshot digest", "Eligibility 检查", "`learningTargets`", "`denyPatterns`", "candidate 创建后保持 immutable"} {
		assertContains(t, learning, required, "learning template")
	}
	learningReview := readPrototypeFile(t, repo, "vnext/templates/research/learning-review.md")
	for _, required := range []string{"Checkpoint A — Eligibility", "Checkpoint B — Exact proposal patch", "Manifest base blob", "Target base blob", "Patch SHA-256", "Patch target count：`1`", "Patch decision"} {
		assertContains(t, learningReview, required, "learning review template")
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
