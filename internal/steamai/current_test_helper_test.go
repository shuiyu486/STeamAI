package steamai

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
)

func materializeCurrentCaseFixture(t *testing.T, root string, memberNames ...string) {
	t.Helper()
	git, source := canonicalFreshFixture(t)
	facts := casebootstrap.Facts{
		Name: "synthetic-case", Goal: "test current entry", Authorization: "temporary fixture files only",
		Prohibited: "network or real artifacts", Stop: "scope drift", Pack: "fixture-pack",
	}
	for _, name := range memberNames {
		facts.Members = append(facts.Members, casebootstrap.MemberFacts{
			Name: name, Kind: "execution", Role: "fixture analyst", Responsibility: "fixture analysis",
			TaskGoal: "complete fixture task", Inputs: "../../pack-snapshot/packs/fixture-pack/manifest.yml",
			AllowedReads: "../../pack-snapshot/packs/fixture-pack/manifest.yml", AllowedWrites: "../../evidence/E-fixture.md",
			Deliverables: "fixture evidence", StopOrEscalate: "scope drift", ExitConditions: "fixture complete",
		})
	}
	preview, err := casebootstrap.BuildPreview(git, source, root, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := casebootstrap.Apply(git, source, root, facts, casebootstrap.ConfirmationPrefix+preview.Identity); err != nil {
		t.Fatal(err)
	}
}

func canonicalFreshFixture(t *testing.T) (string, string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	root := filepath.Join(t.TempDir(), "canonical")
	files := map[string]string{
		".claude/skills/steamai/SKILL.md":          "# Fixture skill\n",
		"vnext/learning-feedback.md":               "# Learning contract\n",
		"vnext/verified-learning.md":               "# Verified learning contract\n",
		"vnext/templates/case/CLAUDE.md":           "# STeamAI 安全研究 Case\n\n## Case 边界\n\n- Case 名称：`{{CASE_NAME}}`\n- 研究目标：`{{GOAL}}`\n- 授权范围：`{{AUTHORIZED_SCOPE}}`\n- 禁止事项：`{{PROHIBITED_ACTIONS}}`\n- 全局停止条件：`{{STOP_CONDITIONS}}`\n- Selected pack：`{{PACK_NAME}}`\n- Source revision：`{{PACK_REVISION}}`\n- Pack tree：`{{PACK_SNAPSHOT_TREE}}`\n- Common tree：`{{COMMON_SNAPSHOT_TREE}}`\n- Snapshot digest：`{{SNAPSHOT_DIGEST}}`\n\n## 当前团队\n\n| Member | Kind | Durable state | Member source |\n|---|---|---|---|\n{{TEAM_ROSTER_ROWS}}\n",
		"vnext/templates/member/CLAUDE.md":         "# {{MEMBER_NAME}}\n{{ROLE}}\n{{RESPONSIBILITY}}\n{{TASK_GOAL}}\n{{INPUTS}}\n{{ALLOWED_READS}}\n{{ALLOWED_WRITES}}\n{{DELIVERABLES}}\n{{STOP_OR_ESCALATE}}\n{{EXIT_CONDITIONS}}\n{{ROLE_SPECIFIC_RULES}}\n",
		"vnext/templates/roles/analysis-member.md": "# Analysis\n",
		"vnext/templates/roles/reviewer.md":        "# Reviewer\n",
		"vnext/templates/research/evidence.md":     "# Evidence\n",
		"packs/fixture-pack/manifest.yml":          "name: fixture-pack\nentrypoints:\n  router: router.md\n",
		"packs/fixture-pack/router.md":             "# Router\n",
		"common/policy.md":                         "# Policy\n",
	}
	for rel, text := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.name", "STeamAI fixture"},
		{"config", "user.email", "fixture@example.invalid"},
		{"add", "--", "."},
		{"commit", "--quiet", "-m", "fixture"},
	} {
		cmd := exec.Command(git, args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	return git, root
}
