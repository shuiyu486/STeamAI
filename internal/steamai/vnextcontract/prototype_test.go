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
		"vnext/README.md",
		"vnext/capabilities.md",
		"vnext/acceptance.md",
		"vnext/learning-feedback.md",
		"vnext/verified-learning.md",
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
		"vnext/templates/research/learning-batch-review.md",
		"vnext/templates/research/replay-spec.md",
		"vnext/templates/research/replay-result.md",
		"vnext/templates/research/evaluation-scenario.md",
		"vnext/templates/research/evaluation-rubric.md",
		"vnext/templates/research/evaluation-attestation.md",
		"vnext/templates/research/blind-decision.md",
		"vnext/templates/research/field-outcome.md",
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
		"`ALLOWED_WRITES` 只允许任务指定的 exact `../../reviews/<file>.md` 或 exact `../../evaluations/attestations/<id>.md`",
		"`needs-evidence` 返回原 owner",
		"只从有 current `accepted` review round 的 finding",
		"`.steamai-vnext/contracts/learning-feedback.md`",
		"用户确认前 canonical source pack 零写",
		"可 `git apply --check` 的 exact patch",
		"应用前按合同重验 snapshot",
		"只有后续 Fresh 明确绑定新的 current source records 与 snapshot digest 才消费",
		"HEAD revision 可以保持不变",
		"## Case-pinned pack 按需路由",
		"entrypoints.router",
		"../../pack-snapshot/packs/<selected-pack>/...",
		"不得默认扫描或串读整个 pack/common",
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
		"Reviewer 只读 artifact/evidence/finding/evaluation spec/run，只写 `reviews/` 与任务明确列出的 exact `evaluations/attestations/<id>.md`",
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
		"领域输入必须列出 Commander 按需选择的",
		"只读取任务列出的 case-pinned paths",
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
	evidence := readPrototypeFile(t, repo, "vnext/templates/research/evidence.md")
	for _, required := range []string{"Artifact alias", "Artifact path", "Artifact SHA-256", "Artifact bytes", "Authorized use", "artifact bytes 漂移都会使本 evidence stale"} {
		assertContains(t, evidence, required, "evidence template")
	}
	for _, required := range []string{"Owner", "Verifier", "Evidence", "尚未证明"} {
		assertContains(t, finding, required, "finding template")
	}
	role := readPrototypeFile(t, repo, "vnext/templates/roles/reviewer.md")
	for _, required := range []string{"{{DECISION}}", "{{REVIEW_ROUND}}", "{{FINDING_SHA256}}", "{{REVIEWED_EVIDENCE_REFS_WITH_SHA256}}", "只在文件末尾追加完整 round", "artifact tuple", "Reviewer 不直接修改原 finding"} {
		assertContains(t, review, required, "review template")
	}
	round := readPrototypeFile(t, repo, "vnext/templates/research/review-round.md")
	for _, required := range []string{"{{REVIEW_ROUND}}", "{{PREVIOUS_REVIEW_ROUND_OR_NONE}}", "{{FINDING_SHA256}}", "不能作为 current `accepted`"} {
		assertContains(t, round, required, "review round template")
	}
	for _, required := range []string{"{{KIND}}", "Claim kind", "Required maturity", "mechanical", "analysis-method", "behavioral"} {
		assertContains(t, learning, required, "learning template")
	}
	for _, required := range []string{"只读 artifact、evidence、finding", "exact `reviews/<file>.md` 或 exact `evaluations/attestations/<id>.md`", "不执行 heavy action", "artifact alias/path/SHA-256/bytes/authorized-use tuple"} {
		assertContains(t, role, required, "Reviewer role")
	}
	for _, required := range []string{"Source finding SHA-256", "Source accepted review", "Pack tree", "Common tree", "Snapshot digest", "Eligibility 检查", "`learningTargets`", "`denyPatterns`", "candidate 创建后保持 immutable"} {
		assertContains(t, learning, required, "learning template")
	}
	learningReview := readPrototypeFile(t, repo, "vnext/templates/research/learning-review.md")
	for _, required := range []string{"Checkpoint A — Eligibility", "Candidate SHA-256", "Evidence/generalization", "Dedup/conflict", "candidate review 不绑定或授权任何 patch"} {
		assertContains(t, learningReview, required, "learning review template")
	}
	batchReview := readPrototypeFile(t, repo, "vnext/templates/research/learning-batch-review.md")
	for _, required := range []string{"## Candidates", "Claim kind", "Required maturity", "Eligibility review SHA-256", "## Targets", "Preimage SHA-256", "Postimage SHA-256", "Patch SHA-256", "Calibration attestation", "Promotion attestation", "Run bundle identity", "Run bundle reveal SHA-256", "同目录的 `reveal.json`", "Evaluated patch SHA-256", "`git apply --check` result", "Decision"} {
		assertContains(t, batchReview, required, "learning batch review template")
	}
	verified := readPrototypeFile(t, repo, "vnext/verified-learning.md")
	for _, required := range []string{"V0 Reviewed", "V1 Mechanically verified", "V2 Replay-backed", "V3 Comparative", "V4 Field-observed", "no-go", "明确 opt-in", "--safe-mode", "suite manifest", "salted pack commitments", "sibling `reveal.json`", "suspended process", "PROCESS_SUSPEND_RESUME", "失败结果也是证据", "先发布失败 bundle，再返回 typed nonzero outcome", "no-go`/`inconclusive` 仍发布 immutable structural closure"} {
		assertContains(t, verified, required, "verified learning contract")
	}
	rubric := readPrototypeFile(t, repo, "vnext/templates/research/evaluation-rubric.md")
	for _, required := range []string{"Covered control classes", "全部五种 control class", "只写在对应 scenario 与 SuiteSpec"} {
		assertContains(t, rubric, required, "evaluation rubric template")
	}
	scenario := readPrototypeFile(t, repo, "vnext/templates/research/evaluation-scenario.md")
	for _, required := range []string{"Replay class", "Synthetic fixture", "Credentials", "Tool network", "Real targets", "Claude API call", "Calibration slot ID", "Expected control class", "Initial pairs", "Maximum pairs", "retry-to-success"} {
		assertContains(t, scenario, required, "evaluation scenario template")
	}
	attestation := readPrototypeFile(t, repo, "vnext/templates/research/evaluation-attestation.md")
	for _, required := range []string{"Blind decision", "Blind decision SHA-256", "Run bundle reveal", "Run bundle reveal SHA-256", "独立 `reveal.json` 的 path/SHA", "suite manifest", "所有预注册 expected slots", "均必须为 literal `none`"} {
		assertContains(t, attestation, required, "evaluation attestation template")
	}
	for _, rel := range []string{"replay-spec.md", "replay-result.md", "evaluation-scenario.md", "evaluation-rubric.md", "evaluation-attestation.md", "blind-decision.md", "field-outcome.md"} {
		if text := readPrototypeFile(t, repo, "vnext/templates/research/"+rel); strings.Contains(text, "`pass`") && !strings.Contains(text, "不得预填") {
			t.Fatalf("verified-learning template %s may prefill a positive decision", rel)
		}
	}
	for _, forbidden := range []string{"- Candidate：`learnings/candidates/L-*.md`", "- Target：`packs/<selected-pack>/**/*.md`"} {
		if strings.Contains(batchReview, forbidden) {
			t.Fatalf("learning batch review template 包含会被 parser 当成真实记录的示例 %q", forbidden)
		}
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
