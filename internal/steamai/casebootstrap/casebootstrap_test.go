package casebootstrap

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewApplyAndCurrentValidation(t *testing.T) {
	git, source := canonicalFixture(t)
	caseRoot := newCaseRoot(t)
	facts := fixtureFacts()

	before := treeDigest(t, caseRoot)
	preview, err := BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Identity == "" || preview.SourceDigest == "" || preview.SnapshotDigest == "" {
		t.Fatalf("preview identity 不完整: %+v", preview)
	}
	if !strings.Contains(preview.HumanPreview, ConfirmationPrefix+preview.Identity) {
		t.Fatal("human preview 缺少 exact confirmation")
	}
	for _, required := range []string{"source-kind:", "source-path:", "git-mode:", "head-blob:", "content-blob:", "pre-state:absent", "output-sha256:", "output-bytes:"} {
		if !strings.Contains(preview.HumanPreview, required) {
			t.Fatalf("human preview 缺少 identity 字段 %q", required)
		}
	}
	if after := treeDigest(t, caseRoot); !sameTree(before, after) {
		t.Fatal("preview 写入了目标")
	}
	if _, err := Apply(git, source, caseRoot, facts, "确认"); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("非 exact confirmation 返回 %v", err)
	}
	if after := treeDigest(t, caseRoot); !sameTree(before, after) {
		t.Fatal("未确认 apply 写入了目标")
	}

	applied, err := Apply(git, source, caseRoot, facts, ConfirmationPrefix+preview.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Identity != preview.Identity {
		t.Fatal("apply 没有绑定 preview identity")
	}
	if err := ValidateCurrent(caseRoot); err != nil {
		t.Fatalf("current validation: %v", err)
	}
	for _, write := range preview.Writes {
		got, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(write.TargetPath)))
		if err != nil {
			t.Fatalf("读取 %s: %v", write.TargetPath, err)
		}
		if string(got) != string(write.Data) {
			t.Fatalf("写入内容不匹配: %s", write.TargetPath)
		}
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrent(caseRoot); err != nil {
		t.Fatalf("删除 source 后 current case 不应失效: %v", err)
	}
}

func TestFactsRejectMarkerBreakingLineBreaks(t *testing.T) {
	for _, field := range []string{"name", "goal", "authorization", "prohibited", "stop"} {
		t.Run(field, func(t *testing.T) {
			facts := fixtureFacts()
			switch field {
			case "name":
				facts.Name = "line one\nline two"
			case "goal":
				facts.Goal = "line one\nline two"
			case "authorization":
				facts.Authorization = "line one\r\nline two"
			case "prohibited":
				facts.Prohibited = "line one\nline two"
			case "stop":
				facts.Stop = "line one\rline two"
			}
			if err := facts.Validate(); err == nil {
				t.Fatalf("field %s accepted a line break", field)
			}
		})
	}
}

func TestApplyRejectsSourceAndTargetDrift(t *testing.T) {
	t.Run("source working tree", func(t *testing.T) {
		git, source := canonicalFixture(t)
		caseRoot := newCaseRoot(t)
		facts := fixtureFacts()
		preview, err := BuildPreview(git, source, caseRoot, facts)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(source, "vnext", "learning-feedback.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, []byte("\nlocal accepted experience\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Apply(git, source, caseRoot, facts, ConfirmationPrefix+preview.Identity); !errors.Is(err, ErrConfirmationRequired) {
			t.Fatalf("source drift 应使旧确认失效，返回 %v", err)
		}
		assertStateMissing(t, caseRoot)
	})

	t.Run("target pre-state", func(t *testing.T) {
		git, source := canonicalFixture(t)
		caseRoot := newCaseRoot(t)
		facts := fixtureFacts()
		preview, err := BuildPreview(git, source, caseRoot, facts)
		if err != nil {
			t.Fatal(err)
		}
		skill := writeForTarget(t, preview, ".claude/skills/steamai/SKILL.md")
		writeFile(t, filepath.Join(caseRoot, filepath.FromSlash(skill.TargetPath)), skill.Data)
		if _, err := Apply(git, source, caseRoot, facts, ConfirmationPrefix+preview.Identity); !errors.Is(err, ErrConfirmationRequired) {
			t.Fatalf("target drift 应使旧确认失效，返回 %v", err)
		}
		assertStateMissing(t, caseRoot)
	})
}

func TestWorkingTreeBytesAreFreshAuthority(t *testing.T) {
	git, source := canonicalFixture(t)
	facts := fixtureFacts()
	path := filepath.Join(source, "packs", facts.Pack, "method.md")
	content := []byte("# Locally confirmed method\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	preview, err := BuildPreview(git, source, newCaseRoot(t), facts)
	if err != nil {
		t.Fatal(err)
	}
	record := sourceRecord(t, preview, "packs/fixture-pack/method.md")
	if !record.Changed || string(record.Data) != string(content) || record.ContentBlob == record.HeadBlob {
		t.Fatal("unstaged tracked working-tree bytes 未进入 preview")
	}

	runGit(t, git, source, "add", "--", "packs/fixture-pack/method.md")
	preview, err = BuildPreview(git, source, newCaseRoot(t), facts)
	if err != nil {
		t.Fatal(err)
	}
	record = sourceRecord(t, preview, "packs/fixture-pack/method.md")
	if !record.Changed || string(record.Data) != string(content) {
		t.Fatal("staged tracked working-tree bytes 未进入 preview")
	}
}

func TestStageZeroIndexIsFreshTrackedAuthority(t *testing.T) {
	t.Run("staged add", func(t *testing.T) {
		git, source := canonicalFixture(t)
		writeFile(t, filepath.Join(source, "common", "added.md"), []byte("# Added\n"))
		runGit(t, git, source, "add", "--", "common/added.md")
		preview, err := BuildPreview(git, source, newCaseRoot(t), fixtureFacts())
		if err != nil {
			t.Fatal(err)
		}
		record := sourceRecord(t, preview, "common/added.md")
		if record.HeadBlob != "" || !record.Changed || string(record.Data) != "# Added\n" {
			t.Fatalf("staged add record = %+v", record)
		}
	})

	t.Run("staged delete", func(t *testing.T) {
		git, source := canonicalFixture(t)
		if err := os.Remove(filepath.Join(source, "common", "policy.md")); err != nil {
			t.Fatal(err)
		}
		runGit(t, git, source, "add", "--", "common/policy.md")
		writeFile(t, filepath.Join(source, "common", "replacement.md"), []byte("# Replacement\n"))
		runGit(t, git, source, "add", "--", "common/replacement.md")
		preview, err := BuildPreview(git, source, newCaseRoot(t), fixtureFacts())
		if err != nil {
			t.Fatal(err)
		}
		if hasSourceRecord(preview, "common/policy.md") || !hasSourceRecord(preview, "common/replacement.md") {
			t.Fatal("staged delete/add 未按 current index path set 生效")
		}
	})

	t.Run("staged rename", func(t *testing.T) {
		git, source := canonicalFixture(t)
		oldPath := filepath.Join(source, "packs", "fixture-pack", "method.md")
		newPath := filepath.Join(source, "packs", "fixture-pack", "renamed-method.md")
		if err := os.Rename(oldPath, newPath); err != nil {
			t.Fatal(err)
		}
		runGit(t, git, source, "add", "--", "packs/fixture-pack/method.md", "packs/fixture-pack/renamed-method.md")
		preview, err := BuildPreview(git, source, newCaseRoot(t), fixtureFacts())
		if err != nil {
			t.Fatal(err)
		}
		if hasSourceRecord(preview, "packs/fixture-pack/method.md") || !hasSourceRecord(preview, "packs/fixture-pack/renamed-method.md") {
			t.Fatal("staged rename 未按 current index path set 生效")
		}
	})
}

func TestSourceClosureRejectsUntrackedAndUnmerged(t *testing.T) {
	t.Run("untracked", func(t *testing.T) {
		git, source := canonicalFixture(t)
		writeFile(t, filepath.Join(source, "common", "untracked.md"), []byte("fixture\n"))
		if _, err := BuildPreview(git, source, newCaseRoot(t), fixtureFacts()); err == nil || !strings.Contains(err.Error(), "未跟踪") {
			t.Fatalf("untracked closure 返回 %v", err)
		}
	})

	t.Run("intent to add", func(t *testing.T) {
		git, source := canonicalFixture(t)
		writeFile(t, filepath.Join(source, "common", "intent.md"), []byte("fixture\n"))
		runGit(t, git, source, "add", "-N", "--", "common/intent.md")
		if _, err := BuildPreview(git, source, newCaseRoot(t), fixtureFacts()); err == nil || !strings.Contains(err.Error(), "intent-to-add") {
			t.Fatalf("intent-to-add 返回 %v", err)
		}
	})
}

func TestFactsLimitAndReviewerRendering(t *testing.T) {
	facts := fixtureFacts()
	for i := range 3 {
		member := facts.Members[0]
		member.Name = "worker-" + string(rune('a'+i))
		member.Kind = "execution"
		facts.Members = append(facts.Members, member)
	}
	if err := facts.Validate(); err == nil {
		t.Fatal("4 名 execution member 未被拒绝")
	}

	git, source := canonicalFixture(t)
	facts = fixtureFacts()
	invalidReviewer := MemberFacts{
		Name: "reviewer", Kind: "reviewer", Role: "independent reviewer", Responsibility: "review supplied findings",
		TaskGoal: "review current evidence", Inputs: "../../findings/ and ../../evidence/", AllowedReads: "../../findings/ ../../evidence/ ../../artifacts/",
		AllowedWrites: "../../reviews/R-fixture.md,../../findings/F-fixture.md", Deliverables: "one review round", StopOrEscalate: "missing evidence", ExitConditions: "decision recorded",
	}
	invalidFacts := facts
	invalidFacts.Members = append(invalidFacts.Members, invalidReviewer)
	if err := invalidFacts.Validate(); err == nil {
		t.Fatal("Reviewer 混合 reviews/findings 写入范围未被拒绝")
	}

	facts.Members = append(facts.Members, MemberFacts{
		Name: "reviewer", Kind: "reviewer", Role: "independent reviewer", Responsibility: "review supplied findings",
		TaskGoal: "review current evidence", Inputs: "../../findings/ and ../../evidence/", AllowedReads: "../../findings/ ../../evidence/ ../../artifacts/",
		AllowedWrites: "../../reviews/R-fixture.md", Deliverables: "one review round", StopOrEscalate: "missing evidence", ExitConditions: "decision recorded",
	})
	preview, err := BuildPreview(git, source, newCaseRoot(t), facts)
	if err != nil {
		t.Fatal(err)
	}
	member := writeForTarget(t, preview, ".steamai-vnext/members/reviewer/CLAUDE.md")
	text := string(member.Data)
	for _, required := range []string{"Reviewer 是独立审查成员", "唯一允许写入 `reviews/`", "不执行 heavy action"} {
		if !strings.Contains(text, required) {
			t.Fatalf("Reviewer render 缺少 %q", required)
		}
	}
}

func TestExactOrphanSkillRemainsRetryable(t *testing.T) {
	git, source := canonicalFixture(t)
	caseRoot := newCaseRoot(t)
	facts := fixtureFacts()
	first, err := BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		t.Fatal(err)
	}
	skill := writeForTarget(t, first, ".claude/skills/steamai/SKILL.md")
	writeFile(t, filepath.Join(caseRoot, filepath.FromSlash(skill.TargetPath)), skill.Data)
	retry, err := BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		t.Fatal(err)
	}
	retrySkill := writeForTarget(t, retry, skill.TargetPath)
	if retrySkill.TargetAction != "unchanged" {
		t.Fatalf("orphan exact skill action = %s", retrySkill.TargetAction)
	}
	if _, err := Apply(git, source, caseRoot, facts, ConfirmationPrefix+retry.Identity); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrent(caseRoot); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentRosterAllowsHistoricalMembersAndRejectsMarkerDrift(t *testing.T) {
	git, source := canonicalFixture(t)
	caseRoot := newCaseRoot(t)
	facts := fixtureFacts()
	preview, err := BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(git, source, caseRoot, facts, ConfirmationPrefix+preview.Identity); err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(caseRoot, ".steamai-vnext", "CLAUDE.md")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(marker)
	row := "| analyst | execution | active | `.steamai-vnext/members/analyst/CLAUDE.md` |"
	insertAt := strings.Index(text, row)
	if insertAt < 0 {
		t.Fatal("fixture marker 缺少 roster row")
	}
	insertAt += len(row)
	var historical strings.Builder
	for _, name := range []string{"old-a", "old-b", "old-c", "old-d"} {
		memberRoot := filepath.Join(caseRoot, ".steamai-vnext", "members", name)
		writeFile(t, filepath.Join(memberRoot, "CLAUDE.md"), []byte("# Historical member\n"))
		fmt.Fprintf(&historical, "\n| %s | execution | completed | `.steamai-vnext/members/%s/CLAUDE.md` |", name, name)
	}
	text = text[:insertAt] + historical.String() + text[insertAt:]
	writeFile(t, markerPath, []byte(text))
	identity, err := InspectCurrent(caseRoot)
	if err != nil {
		t.Fatalf("5 个历史 member 目录应为 current: %v", err)
	}
	if len(identity.Roster) != 5 {
		t.Fatalf("roster size = %d", len(identity.Roster))
	}

	writeFile(t, markerPath, []byte(strings.Replace(text, "- Source revision：`"+identity.Revision+"`", "- Source revision：`"+strings.Repeat("f", 40)+"`", 1)))
	if err := ValidateCurrent(caseRoot); err == nil {
		t.Fatal("篡改 case marker identity 未被拒绝")
	}
	writeFile(t, markerPath, nil)
	if err := ValidateCurrent(caseRoot); err == nil {
		t.Fatal("空 case marker 未被拒绝")
	}
}

func TestCurrentValidationRejectsPayloadAndPathSetDrift(t *testing.T) {
	git, source := canonicalFixture(t)
	caseRoot := newCaseRoot(t)
	facts := fixtureFacts()
	preview, err := BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(git, source, caseRoot, facts, ConfirmationPrefix+preview.Identity); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", "common", "policy.md")
	if err := os.WriteFile(path, []byte("drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrent(caseRoot); err == nil {
		t.Fatal("payload bytes drift 未被拒绝")
	}

	writeFile(t, path, sourceRecord(t, preview, "common/policy.md").Data)
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", "common", "undeclared.md"), []byte("extra\n"))
	if err := ValidateCurrent(caseRoot); err == nil {
		t.Fatal("payload path-set drift 未被拒绝")
	}
}

func fixtureFacts() Facts {
	return Facts{
		Name: "synthetic-case", Goal: "verify production Fresh", Authorization: "temporary fixture files only",
		Prohibited: "network or real artifacts", Stop: "scope drift", Pack: "fixture-pack",
		Members: []MemberFacts{{
			Name: "analyst", Kind: "execution", Role: "static analyst", Responsibility: "analyze fixture inputs",
			TaskGoal: "produce fixture evidence", Inputs: "../../pack-snapshot/packs/fixture-pack/manifest.yml and ../../pack-snapshot/packs/fixture-pack/router.md",
			AllowedReads:  "../../pack-snapshot/packs/fixture-pack/manifest.yml ../../pack-snapshot/packs/fixture-pack/router.md ../../pack-snapshot/packs/fixture-pack/method.md",
			AllowedWrites: "../../evidence/E-fixture.md ../../findings/F-fixture.md", Deliverables: "fixture evidence and finding",
			StopOrEscalate: "scope drift", ExitConditions: "deliverables complete",
		}},
	}
}

func canonicalFixture(t *testing.T) (string, string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	root := filepath.Join(t.TempDir(), "canonical")
	files := map[string]string{
		".claude/skills/steamai/SKILL.md":          "# Fixture skill\n",
		"vnext/learning-feedback.md":               "# Learning contract\n",
		"vnext/templates/case/CLAUDE.md":           "# STeamAI 安全研究 Case\n\n## Case 边界\n\n- Case 名称：`{{CASE_NAME}}`\n- 研究目标：`{{GOAL}}`\n- 授权范围：`{{AUTHORIZED_SCOPE}}`\n- 禁止事项：`{{PROHIBITED_ACTIONS}}`\n- 全局停止条件：`{{STOP_CONDITIONS}}`\n- Selected pack：`{{PACK_NAME}}`\n- Source revision：`{{PACK_REVISION}}`\n- Pack tree：`{{PACK_SNAPSHOT_TREE}}`\n- Common tree：`{{COMMON_SNAPSHOT_TREE}}`\n- Snapshot digest：`{{SNAPSHOT_DIGEST}}`\n\n## 当前团队\n\n| Member | Kind | Durable state | Member source |\n|---|---|---|---|\n{{TEAM_ROSTER_ROWS}}\n",
		"vnext/templates/member/CLAUDE.md":         "# {{MEMBER_NAME}}\n{{ROLE}}\n{{RESPONSIBILITY}}\n{{TASK_GOAL}}\n{{INPUTS}}\n{{ALLOWED_READS}}\n{{ALLOWED_WRITES}}\n{{DELIVERABLES}}\n{{STOP_OR_ESCALATE}}\n{{EXIT_CONDITIONS}}\n{{ROLE_SPECIFIC_RULES}}\n",
		"vnext/templates/roles/analysis-member.md": "# Analysis role\n",
		"vnext/templates/roles/reviewer.md":        "Reviewer 是独立审查成员；唯一允许写入 `reviews/`；不执行 heavy action。\n",
		"vnext/templates/research/evidence.md":     "# Evidence\n",
		"packs/fixture-pack/manifest.yml":          "name: fixture-pack\nentrypoints:\n  router: router.md\n",
		"packs/fixture-pack/router.md":             "# Router\n",
		"packs/fixture-pack/method.md":             "# Method\n",
		"common/policy.md":                         "# Policy\n",
	}
	for rel, text := range files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte(text))
	}
	runGit(t, git, root, "init", "--quiet")
	runGit(t, git, root, "config", "user.name", "STeamAI fixture")
	runGit(t, git, root, "config", "user.email", "fixture@example.invalid")
	runGit(t, git, root, "add", "--", ".")
	runGit(t, git, root, "commit", "--quiet", "-m", "fixture")
	return git, root
}

func newCaseRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "case")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func runGit(t *testing.T, git, root string, args ...string) {
	t.Helper()
	cmd := exec.Command(git, args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeForTarget(t *testing.T, preview Preview, target string) PlannedWrite {
	t.Helper()
	for _, write := range preview.Writes {
		if write.TargetPath == target {
			return write
		}
	}
	t.Fatalf("preview 缺少 %s", target)
	return PlannedWrite{}
}

func hasSourceRecord(preview Preview, path string) bool {
	for _, record := range preview.SourceRecords {
		if record.Path == path {
			return true
		}
	}
	return false
}

func sourceRecord(t *testing.T, preview Preview, path string) SourceRecord {
	t.Helper()
	for _, record := range preview.SourceRecords {
		if record.Path == path {
			return record
		}
	}
	t.Fatalf("preview 缺少 source %s", path)
	return SourceRecord{}
}

func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = hashBytes(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func sameTree(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, digest := range left {
		if right[path] != digest {
			return false
		}
	}
	return true
}

func assertStateMissing(t *testing.T, caseRoot string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai-vnext")); !os.IsNotExist(err) {
		t.Fatalf("存在 partial state: %v", err)
	}
}
