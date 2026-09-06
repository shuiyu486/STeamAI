package casebootstrap

import (
	"encoding/json"
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

func TestCurrentValidationKeepsPreVerifiedLearningCasesCurrent(t *testing.T) {
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

	contract := ".steamai-vnext/contracts/verified-learning.md"
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(contract))); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"evaluations/specs", "evaluations/runs", "evaluations/attestations", "evaluations/outcomes", "evaluations/work"} {
		if err := os.RemoveAll(filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	snapshotPath := filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", "snapshot.yml")
	snapshotData, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := string(snapshotData)
	lines := strings.Split(snapshot, "\n")
	filtered := make([]string, 0, len(lines))
	for index := 0; index < len(lines); index++ {
		if lines[index] == "  - path: "+contract {
			index += 2
			continue
		}
		filtered = append(filtered, lines[index])
	}
	if err := os.WriteFile(snapshotPath, []byte(strings.Join(filtered, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrent(caseRoot); err != nil {
		t.Fatalf("pre-verified-learning current case 被新 executable 拒绝: %v", err)
	}
	enabled, err := SupportsVerifiedLearning(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("pre-verified-learning case 被错误标记为支持新版 learning")
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

	for _, invalid := range []string{
		"../../reviews/", "../../reviews/nested/R-fixture.md", "../../reviews/R-fixture.txt",
		"../../evaluations/attestations/", "../../evaluations/attestations/nested/CAL.md",
		"../../evaluations/runs/RUN/manifest.json", "../../evaluations/attestations/CAL.md,../../findings/F-fixture.md",
		"../../reviews/R-*.md", "../../reviews/R-?.md", "../../reviews/R<bad>.md", "../../reviews/R:bad.md",
		"../../reviews/CON.md", "../../reviews/nul.md", "../../reviews/COM1.extra.md", "../../evaluations/attestations/LPT9.md",
		"../../reviews/R.md.", "../../reviews/R.md ",
	} {
		candidate := invalidReviewer
		candidate.AllowedWrites = invalid
		invalidFacts = facts
		invalidFacts.Members = append(invalidFacts.Members, candidate)
		if err := invalidFacts.Validate(); err == nil {
			t.Fatalf("Reviewer 非 exact 写入范围 %q 未被拒绝", invalid)
		}
	}

	facts.Members = append(facts.Members, MemberFacts{
		Name: "reviewer", Kind: "reviewer", Role: "independent reviewer", Responsibility: "review supplied findings",
		TaskGoal: "review current evidence", Inputs: "../../findings/ and ../../evidence/", AllowedReads: "../../findings/ ../../evidence/ ../../artifacts/",
		AllowedWrites: "../../reviews/R-fixture.md,../../evaluations/attestations/CAL-001.md", Deliverables: "one review round and one attestation", StopOrEscalate: "missing evidence", ExitConditions: "decision recorded",
	})
	preview, err := BuildPreview(git, source, newCaseRoot(t), facts)
	if err != nil {
		t.Fatal(err)
	}
	member := writeForTarget(t, preview, ".steamai-vnext/members/reviewer/CLAUDE.md")
	text := string(member.Data)
	for _, required := range []string{"Reviewer 是独立审查成员", "../../reviews/R-fixture.md", "../../evaluations/attestations/CAL-001.md", "不执行 heavy action"} {
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

func TestAuxPackPreviewApplyAndCurrent(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		t.Run(fmt.Sprint(reversed), func(t *testing.T) {
			git, source := canonicalFixture(t)
			facts := fixtureFacts()
			facts.AuxPack = "aux-pack"
			if reversed {
				facts.Pack, facts.AuxPack = facts.AuxPack, facts.Pack
			}
			caseRoot := newCaseRoot(t)
			preview, err := BuildPreview(git, source, caseRoot, facts)
			if err != nil {
				t.Fatal(err)
			}
			if !hexIdentityPattern.MatchString(preview.AuxPackTree) {
				t.Fatal("auxiliary tree 未冻结")
			}
			seenSources, seenTargets := map[string]bool{}, map[string]bool{}
			for _, record := range preview.SourceRecords {
				if seenSources[record.Path] {
					t.Fatalf("重复 source %s", record.Path)
				}
				seenSources[record.Path] = true
			}
			for _, write := range preview.Writes {
				if seenTargets[write.TargetPath] {
					t.Fatalf("重复 target %s", write.TargetPath)
				}
				seenTargets[write.TargetPath] = true
			}
			for _, path := range []string{"common/policy.md", "packs/fixture-pack/manifest.yml", "packs/aux-pack/manifest.yml", "packs/aux-pack/method.md"} {
				if !seenSources[path] || !seenTargets[".steamai-vnext/pack-snapshot/"+path] {
					t.Fatalf("source/payload 缺少 %s", path)
				}
			}
			changed := preview
			changed.AuxPackTree = strings.Repeat("f", 40)
			if previewIdentity(changed) == preview.Identity {
				t.Fatal("identity 未绑定 auxiliary tree")
			}
			if _, err := Apply(git, source, caseRoot, facts, ConfirmationPrefix+preview.Identity); err != nil {
				t.Fatal(err)
			}
			identity, err := InspectCurrent(caseRoot)
			if err != nil || identity.Pack != facts.Pack || identity.AuxPack != facts.AuxPack || identity.AuxPackTree != preview.AuxPackTree {
				t.Fatalf("主辅 identity 不匹配: %+v %v", identity, err)
			}
			if err := os.RemoveAll(source); err != nil {
				t.Fatal(err)
			}
			before := treeDigest(t, caseRoot)
			if err := ValidateCurrent(caseRoot); err != nil {
				t.Fatalf("current 依赖 canonical: %v", err)
			}
			if !sameTree(before, treeDigest(t, caseRoot)) {
				t.Fatal("current validator 写入了 case")
			}
		})
	}
}

func TestFactsPackSelectionsRejectAmbiguity(t *testing.T) {
	data, err := json.Marshal(fixtureFacts())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, selection string
		valid           bool
	}{
		{"absent auxiliary", `"pack":"fixture-pack"`, true},
		{"one auxiliary", `"pack":"fixture-pack","auxPack":"aux-pack"`, true},
		{"same main", `"pack":"fixture-pack","auxPack":"fixture-pack"`, false},
		{"empty main", `"pack":""`, false},
		{"empty auxiliary", `"pack":"fixture-pack","auxPack":""`, false},
		{"null auxiliary", `"pack":"fixture-pack","auxPack":null`, false},
		{"multiple auxiliary", `"pack":"fixture-pack","auxPack":["aux-pack"]`, false},
		{"path auxiliary", `"pack":"fixture-pack","auxPack":"../aux-pack"`, false},
		{"template auxiliary", `"pack":"fixture-pack","auxPack":"_template"`, false},
		{"duplicate main", `"pack":"fixture-pack","pack":"fixture-pack"`, false},
		{"duplicate auxiliary", `"pack":"fixture-pack","auxPack":"aux-pack","auxPack":"aux-pack"`, false},
		{"case alias", `"pack":"fixture-pack","Pack":"fixture-pack"`, false},
		{"auxiliary case alias", `"pack":"fixture-pack","auxPack":"aux-pack","AuxPack":"aux-pack"`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			text := strings.Replace(string(data), `"pack":"fixture-pack"`, test.selection, 1)
			_, err := DecodeFacts(strings.NewReader(text))
			if (err == nil) != test.valid {
				t.Fatalf("DecodeFacts valid=%v: %v", test.valid, err)
			}
		})
	}
}

func TestSinglePackOmitsAuxiliaryIdentity(t *testing.T) {
	git, source := canonicalFixture(t)
	preview, err := BuildPreview(git, source, newCaseRoot(t), fixtureFacts())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	for _, unexpected := range []string{`"auxPack"`, `"auxPackTree"`, "aux-pack:", "Auxiliary pack：", "{{AUX_PACK_IDENTITY}}", "packs/aux-pack/"} {
		if strings.Contains(string(data), unexpected) {
			t.Fatalf("单包 preview 包含 %s", unexpected)
		}
	}
	changed := preview
	changed.AuxPackTree = strings.Repeat("f", 40)
	if previewIdentity(changed) != preview.Identity {
		t.Fatal("无辅助时 identity 增加了空辅助 tree 字段")
	}
}

func TestAuxPackSourceValidationAndDrift(t *testing.T) {
	for _, test := range []struct {
		name         string
		mutate       func(*testing.T, string, string)
		previewValid bool
	}{
		{"missing manifest", func(t *testing.T, git, source string) {
			runGit(t, git, source, "rm", "--", "packs/aux-pack/manifest.yml")
		}, false},
		{"missing router", func(t *testing.T, git, source string) { runGit(t, git, source, "rm", "--", "packs/aux-pack/router.md") }, false},
		{"wrong manifest name", func(t *testing.T, git, source string) {
			writeFile(t, filepath.Join(source, "packs/aux-pack/manifest.yml"), []byte("name: other-pack\nentrypoints:\n  router: router.md\n"))
		}, false},
		{"escaping router", func(t *testing.T, git, source string) {
			writeFile(t, filepath.Join(source, "packs/aux-pack/manifest.yml"), []byte("name: aux-pack\nentrypoints:\n  router: ../fixture-pack/router.md\n"))
		}, false},
		{"working bytes", func(t *testing.T, git, source string) {
			writeFile(t, filepath.Join(source, "packs/aux-pack/method.md"), []byte("# Changed auxiliary method\n"))
		}, true},
		{"index tree", func(t *testing.T, git, source string) {
			path := filepath.Join(source, "packs/aux-pack/method.md")
			writeFile(t, path, []byte("# Staged auxiliary method\n"))
			runGit(t, git, source, "add", "--", "packs/aux-pack/method.md")
			writeFile(t, path, []byte("# Auxiliary method\n"))
		}, true},
		{"index mode", func(t *testing.T, git, source string) {
			runGit(t, git, source, "update-index", "--chmod=+x", "--", "packs/aux-pack/method.md")
		}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			git, source := canonicalFixture(t)
			facts := fixtureFacts()
			facts.AuxPack = "aux-pack"
			root := newCaseRoot(t)
			preview, err := BuildPreview(git, source, root, facts)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, git, source)
			updated, err := BuildPreview(git, source, root, facts)
			if (err == nil) != test.previewValid {
				t.Fatalf("preview valid=%v: %v", test.previewValid, err)
			}
			if err == nil && updated.Identity == preview.Identity {
				t.Fatal("辅助漂移未使 preview 失效")
			}
			if _, err := Apply(git, source, root, facts, ConfirmationPrefix+preview.Identity); err == nil {
				t.Fatal("旧确认接受辅助漂移")
			}
			if err := applyPreview(git, source, root, preview); err == nil {
				t.Fatal("staging 发布前未拒绝辅助漂移")
			}
			if len(treeDigest(t, root)) != 0 {
				t.Fatal("拒绝辅助漂移后留下文件")
			}
		})
	}
}

func TestAuxPackRequiresTemplateIdentityBeforePublishing(t *testing.T) {
	for _, test := range []struct{ name, auxPack, replacement string }{
		{"missing token", "aux-pack", ""},
		{"duplicate token", "aux-pack", "{{AUX_PACK_IDENTITY}}{{AUX_PACK_IDENTITY}}"},
		{"preexisting auxiliary identity", "aux-pack", "- Auxiliary pack：`aux-pack`\n{{AUX_PACK_IDENTITY}}"},
		{"preexisting auxiliary tree", "aux-pack", "- Auxiliary pack tree：`" + strings.Repeat("a", 40) + "`\n{{AUX_PACK_IDENTITY}}"},
		{"auxiliary marker without selection", "", "- Auxiliary pack：`aux-pack`\n- Auxiliary pack tree：`" + strings.Repeat("a", 40) + "`\n{{AUX_PACK_IDENTITY}}"},
		{"empty auxiliary marker without selection", "", "- Auxiliary pack：``\n- Auxiliary pack tree：``\n{{AUX_PACK_IDENTITY}}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			git, source := canonicalFixture(t)
			path := filepath.Join(source, "vnext/templates/case/CLAUDE.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, []byte(strings.ReplaceAll(string(data), "{{AUX_PACK_IDENTITY}}", test.replacement)))
			facts := fixtureFacts()
			facts.AuxPack = test.auxPack
			root := newCaseRoot(t)
			if _, err := BuildPreview(git, source, root, facts); err == nil || !strings.Contains(err.Error(), "template identity") {
				t.Fatalf("无效辅助 identity 模板被接受: %v", err)
			}
			if _, err := Apply(git, source, root, facts, ConfirmationPrefix+strings.Repeat("a", 64)); err == nil {
				t.Fatal("无效辅助 identity 模板被发布")
			}
			if len(treeDigest(t, root)) != 0 {
				t.Fatal("无效辅助模板留下 partial case")
			}
		})
	}
}

func TestCurrentRejectsAmbiguousPackBindings(t *testing.T) {
	git, source := canonicalFixture(t)
	facts := fixtureFacts()
	facts.AuxPack = "aux-pack"
	root := newCaseRoot(t)
	preview, err := BuildPreview(git, source, root, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(git, source, root, facts, ConfirmationPrefix+preview.Identity); err != nil {
		t.Fatal(err)
	}
	snapshot := string(writeForTarget(t, preview, ".steamai-vnext/pack-snapshot/snapshot.yml").Data)
	marker := string(writeForTarget(t, preview, ".steamai-vnext/CLAUDE.md").Data)
	for _, field := range []struct{ snapshot, marker, value string }{
		{"pack", "Selected pack", facts.Pack}, {"pack-tree", "Pack tree", preview.PackTree},
		{"aux-pack", "Auxiliary pack", facts.AuxPack}, {"aux-pack-tree", "Auxiliary pack tree", preview.AuxPackTree},
	} {
		for _, mutation := range []string{"duplicate", "empty", "missing"} {
			for _, location := range []string{"snapshot", "marker"} {
				t.Run(field.snapshot+"/"+mutation+"/"+location, func(t *testing.T) {
					s, m := snapshot, marker
					line := field.snapshot + ": " + field.value + "\n"
					replacement := ""
					if location == "marker" {
						line = "- " + field.marker + "：`" + field.value + "`\n"
					}
					if mutation == "duplicate" {
						replacement = line + line
					}
					if mutation == "empty" {
						replacement = field.snapshot + ": \n"
						if location == "marker" {
							replacement = "- " + field.marker + "：``\n"
						}
					}
					if location == "snapshot" {
						s = strings.Replace(s, line, replacement, 1)
					} else {
						m = strings.Replace(m, line, replacement, 1)
					}
					writeFile(t, filepath.Join(root, ".steamai-vnext/pack-snapshot/snapshot.yml"), []byte(s))
					writeFile(t, filepath.Join(root, ".steamai-vnext/CLAUDE.md"), []byte(m))
					if err := ValidateCurrent(root); err == nil {
						t.Fatal("歧义身份被接受")
					}
				})
			}
		}
	}
	for _, invalid := range []string{facts.Pack, "_template", "../aux-pack"} {
		t.Run("invalid auxiliary "+invalid, func(t *testing.T) {
			s := strings.Replace(snapshot, "aux-pack: aux-pack\n", "aux-pack: "+invalid+"\n", 1)
			m := strings.Replace(marker, "- Auxiliary pack：`aux-pack`", "- Auxiliary pack：`"+invalid+"`", 1)
			writeFile(t, filepath.Join(root, ".steamai-vnext/pack-snapshot/snapshot.yml"), []byte(s))
			writeFile(t, filepath.Join(root, ".steamai-vnext/CLAUDE.md"), []byte(m))
			if err := ValidateCurrent(root); err == nil {
				t.Fatal("非法或主辅相同的身份被接受")
			}
		})
	}
}

func TestCurrentRejectsSelfConsistentAuxiliaryPayload(t *testing.T) {
	for _, mutation := range []string{"missing manifest", "missing router", "wrong manifest", "escaping router", "ghost auxiliary", "undeclared second", "undeclared third", "aux bytes", "aux missing file"} {
		t.Run(mutation, func(t *testing.T) {
			git, source := canonicalFixture(t)
			root := newCaseRoot(t)
			facts := fixtureFacts()
			facts.AuxPack = "aux-pack"
			preview, err := BuildPreview(git, source, root, facts)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Apply(git, source, root, facts, ConfirmationPrefix+preview.Identity); err != nil {
				t.Fatal(err)
			}
			snapshotPath := filepath.Join(root, ".steamai-vnext/pack-snapshot/snapshot.yml")
			markerPath := filepath.Join(root, ".steamai-vnext/CLAUDE.md")
			snapshot := string(writeForTarget(t, preview, ".steamai-vnext/pack-snapshot/snapshot.yml").Data)
			marker := string(writeForTarget(t, preview, ".steamai-vnext/CLAUDE.md").Data)
			removeRecord := func(path string) {
				start := strings.Index(snapshot, "  - path: "+path+"\n")
				if start < 0 {
					t.Fatalf("缺少待删除 record %s", path)
				}
				end := start
				for range 6 {
					end += strings.IndexByte(snapshot[end:], '\n') + 1
				}
				snapshot = snapshot[:start] + snapshot[end:]
				if err := os.Remove(filepath.Join(root, ".steamai-vnext/pack-snapshot", filepath.FromSlash(path))); err != nil {
					t.Fatal(err)
				}
			}
			rebind := true
			switch mutation {
			case "missing manifest":
				removeRecord("packs/aux-pack/manifest.yml")
			case "missing router":
				removeRecord("packs/aux-pack/router.md")
			case "ghost auxiliary":
				for _, path := range []string{"manifest.yml", "method.md", "router.md"} {
					removeRecord("packs/aux-pack/" + path)
				}
			case "wrong manifest", "escaping router":
				record := sourceRecord(t, preview, "packs/aux-pack/manifest.yml")
				data := []byte("name: other-pack\nentrypoints:\n  router: router.md\n")
				if mutation == "escaping router" {
					data = []byte("name: aux-pack\nentrypoints:\n  router: ../fixture-pack/router.md\n")
				}
				writeFile(t, filepath.Join(root, ".steamai-vnext/pack-snapshot", record.Path), data)
				snapshot = strings.Replace(snapshot, "    sha256: "+record.SHA256+"\n    bytes: "+fmt.Sprint(record.Bytes), "    sha256: "+hashBytes(data)+"\n    bytes: "+fmt.Sprint(len(data)), 1)
			case "undeclared second":
				snapshot = strings.Replace(snapshot, "aux-pack: aux-pack\naux-pack-tree: "+preview.AuxPackTree+"\n", "", 1)
				marker = strings.Replace(marker, "- Auxiliary pack：`aux-pack`\n- Auxiliary pack tree：`"+preview.AuxPackTree+"`\n", "", 1)
			case "undeclared third":
				old := "packs/fixture-pack/router.md"
				newPath := "packs/third-pack/router.md"
				record := sourceRecord(t, preview, old)
				writeFile(t, filepath.Join(root, ".steamai-vnext/pack-snapshot", newPath), record.Data)
				if err := os.Remove(filepath.Join(root, ".steamai-vnext/pack-snapshot", old)); err != nil {
					t.Fatal(err)
				}
				snapshot = strings.Replace(snapshot, "  - path: "+old+"\n", "  - path: "+newPath+"\n", 1)
			case "aux bytes":
				writeFile(t, filepath.Join(root, ".steamai-vnext/pack-snapshot/packs/aux-pack/method.md"), []byte("changed\n"))
				rebind = false
			case "aux missing file":
				if err := os.Remove(filepath.Join(root, ".steamai-vnext/pack-snapshot/packs/aux-pack/method.md")); err != nil {
					t.Fatal(err)
				}
				rebind = false
			}
			writeFile(t, snapshotPath, []byte(snapshot))
			if rebind {
				digest, err := ActualSnapshotDigest(root)
				if err != nil {
					t.Fatalf("异常 fixture 的外层摘要无法计算: %v", err)
				}
				snapshot = strings.Replace(snapshot, preview.SnapshotDigest, digest, 1)
				marker = strings.Replace(marker, preview.SnapshotDigest, digest, 1)
				writeFile(t, snapshotPath, []byte(snapshot))
			}
			writeFile(t, markerPath, []byte(marker))
			if err := ValidateCurrent(root); err == nil {
				t.Fatal("异常辅助 snapshot 被接受")
			}
		})
	}
}

func TestFixedOldSinglePackCurrentRemainsUnchanged(t *testing.T) {
	root := newCaseRoot(t)
	for _, path := range []string{"members", "artifacts", "evidence", "findings", "reviews", "learnings/candidates", "learnings/patches"} {
		if err := os.MkdirAll(filepath.Join(root, ".steamai-vnext", path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".claude/skills/steamai/SKILL.md":                          "# Old skill\n",
		".steamai-vnext/contracts/learning-feedback.md":            "# Old learning\n",
		".steamai-vnext/pack-snapshot/common/policy.md":            "# Old common\n",
		".steamai-vnext/pack-snapshot/packs/old-pack/manifest.yml": "name: old-pack\nentrypoints:\n  router: router.md\n",
		".steamai-vnext/pack-snapshot/packs/old-pack/router.md":    "# Old router\n",
		".steamai-vnext/artifacts/index.md":                        "# Old artifacts\n",
		".steamai-vnext/CLAUDE.md":                                 "# Old synthetic case\n- Case 名称：`old-case`\n- 研究目标：`old fixture`\n- 授权范围：`local fixture`\n- 禁止事项：`network`\n- 全局停止条件：`scope drift`\n- Selected pack：`old-pack`\n- Source revision：`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`\n- Pack tree：`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`\n- Common tree：`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`\n- Snapshot digest：`sha256:90d4253c8dc9fccc4bd16e85f4ba909f295a12abcc77b06e200b6188fbf2e852`\n\n| Member | Kind | Durable state | Member source |\n|---|---|---|---|\n| none | execution | inactive | none |\n",
		".steamai-vnext/pack-snapshot/snapshot.yml": `schema: steamai-case-snapshot-v2
pack: old-pack
revision: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
pack-tree: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
common-tree: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
source-digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
payload-digest: sha256:90d4253c8dc9fccc4bd16e85f4ba909f295a12abcc77b06e200b6188fbf2e852
files:
  - path: common/policy.md
    git-mode: 100644
    head-blob: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    content-blob: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    sha256: 1b04b502f5415bcd6ebad232cc6446bc2cf7b1f431d1186dd71b4ecc7e68d7a9
    bytes: 13
  - path: packs/old-pack/manifest.yml
    git-mode: 100644
    head-blob: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    content-blob: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    sha256: 206e4bd8e33cdd8c8e85ca544e8428a0e48a661f7741e50b8d6316c2b410c4dc
    bytes: 48
  - path: packs/old-pack/router.md
    git-mode: 100644
    head-blob: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    content-blob: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    sha256: 387872bab8316e0fbd280fd8001cbfb4f43bb4917661474bf55104f1346ce179
    bytes: 13
immutable-files:
  - path: .claude/skills/steamai/SKILL.md
    sha256: 2ab93b68b2005e82b210f3537e1d38550ae1f28cbd208c466708e80eb7ae4383
    bytes: 12
  - path: .steamai-vnext/contracts/learning-feedback.md
    sha256: 31daf6d1e76229daab087b636bf6a8083c891af10de8ec8819cfd2bfff892c1c
    bytes: 15
`,
	}
	for path, data := range files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(data))
	}
	before := treeDigest(t, root)
	identity, err := InspectCurrent(root)
	if err != nil || identity.Pack != "old-pack" || identity.AuxPack != "" || identity.AuxPackTree != "" {
		t.Fatalf("旧 current 读取失败: %+v %v", identity, err)
	}
	if !sameTree(before, treeDigest(t, root)) {
		t.Fatal("旧 current 被改写或补装")
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
		"vnext/verified-learning.md":               "# Verified learning contract\n",
		"vnext/templates/case/CLAUDE.md":           "# STeamAI 安全研究 Case\n\n## Case 边界\n\n- Case 名称：`{{CASE_NAME}}`\n- 研究目标：`{{GOAL}}`\n- 授权范围：`{{AUTHORIZED_SCOPE}}`\n- 禁止事项：`{{PROHIBITED_ACTIONS}}`\n- 全局停止条件：`{{STOP_CONDITIONS}}`\n- Selected pack：`{{PACK_NAME}}`\n- Source revision：`{{PACK_REVISION}}`\n- Pack tree：`{{PACK_SNAPSHOT_TREE}}`\n{{AUX_PACK_IDENTITY}}- Common tree：`{{COMMON_SNAPSHOT_TREE}}`\n- Snapshot digest：`{{SNAPSHOT_DIGEST}}`\n\n## 当前团队\n\n| Member | Kind | Durable state | Member source |\n|---|---|---|---|\n{{TEAM_ROSTER_ROWS}}\n",
		"vnext/templates/member/CLAUDE.md":         "# {{MEMBER_NAME}}\n{{ROLE}}\n{{RESPONSIBILITY}}\n{{TASK_GOAL}}\n{{INPUTS}}\n{{ALLOWED_READS}}\n{{ALLOWED_WRITES}}\n{{DELIVERABLES}}\n{{STOP_OR_ESCALATE}}\n{{EXIT_CONDITIONS}}\n{{ROLE_SPECIFIC_RULES}}\n",
		"vnext/templates/roles/analysis-member.md": "# Analysis role\n",
		"vnext/templates/roles/reviewer.md":        "Reviewer 是独立审查成员；唯一允许写入 `reviews/`；不执行 heavy action。\n",
		"vnext/templates/research/evidence.md":     "# Evidence\n",
		"packs/fixture-pack/manifest.yml":          "name: fixture-pack\nentrypoints:\n  router: router.md\n",
		"packs/fixture-pack/router.md":             "# Router\n",
		"packs/fixture-pack/method.md":             "# Method\n",
		"packs/aux-pack/manifest.yml":              "name: aux-pack\nentrypoints:\n  router: router.md\n",
		"packs/aux-pack/router.md":                 "# Auxiliary router\n",
		"packs/aux-pack/method.md":                 "# Auxiliary method\n",
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
