package casebootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepositoryRoleContractsMaterializeThroughFresh(t *testing.T) {
	fixture := buildRealRepoFreshFixture(t, "binary-re",
		"../../pack-snapshot/packs/binary-re/manifest.yml ../../pack-snapshot/packs/binary-re/references/binary-re/README.md ../../pack-snapshot/packs/binary-re/references/binary-re/general-analysis.md")
	facts := &fixture.facts
	facts.Members = append(facts.Members, MemberFacts{
		Name: "reviewer", Kind: "reviewer", Role: "independent reviewer", Responsibility: "review fixture claims",
		TaskGoal: "check fixture evidence against the supplied goal", Inputs: "../../evidence/E-fixture.md ../../findings/F-fixture.md",
		AllowedReads: "../../evidence/E-fixture.md ../../findings/F-fixture.md", AllowedWrites: "../../reviews/R-fixture.md",
		Deliverables: "independent fixture review", StopOrEscalate: "missing fixture evidence", ExitConditions: "review delivered",
	})
	applyFresh := fixture.applyFresh

	oldRoot := newCaseRoot(t)
	preview, identity := applyFresh(t, oldRoot)
	for target, text := range map[string]string{
		".claude/skills/steamai/SKILL.md":           "当前任务内的方法调整不是正式改派",
		".steamai-vnext/CLAUDE.md":                  "任务说明预期结果而非固定研究步骤",
		".steamai-vnext/members/analyst/CLAUDE.md":  "不只交付操作建议",
		".steamai-vnext/members/reviewer/CLAUDE.md": "自主选择只读复核路径",
	} {
		if !bytes.Contains(writeForTarget(t, preview, target).Data, []byte(text)) {
			t.Fatalf("real repository contract is missing from %s: %s", target, text)
		}
	}
	roles := []string{"analysis-member.md", "reviewer.md"}
	for i, member := range facts.Members {
		text := string(writeForTarget(t, preview, ".steamai-vnext/members/"+member.Name+"/CLAUDE.md").Data)
		role := writeForTarget(t, preview, ".steamai-vnext/contracts/templates/roles/"+roles[i]).Data
		otherRole := writeForTarget(t, preview, ".steamai-vnext/contracts/templates/roles/"+roles[1-i]).Data
		if !strings.Contains(text, string(role)) || strings.Contains(text, strings.SplitN(string(otherRole), "\n", 2)[0]) {
			t.Fatalf("member %s lost or mixed the real role contracts", member.Name)
		}
		if strings.Contains(text, "{{") || !strings.Contains(text, "HOLD_STALE_TASK") {
			t.Fatalf("member %s did not retain the real member template", member.Name)
		}
		for label, value := range map[string]string{
			"目标": member.TaskGoal, "输入": member.Inputs, "允许读取": member.AllowedReads,
			"允许写入": member.AllowedWrites, "交付": member.Deliverables,
			"停止或升级条件": member.StopOrEscalate, "完成/退出条件": member.ExitConditions,
		} {
			if strings.Count(text, "- "+label+"：`"+value+"`") != 1 {
				t.Fatalf("member %s lost or duplicated task field %s", member.Name, label)
			}
		}
	}
	oldTree := treeDigest(t, oldRoot)
	updates := []struct{ source, target, marker string }{
		{".claude/skills/steamai/SKILL.md", ".claude/skills/steamai/SKILL.md", "<!-- fresh-skill-update -->"},
		{"vnext/templates/roles/analysis-member.md", ".steamai-vnext/members/analyst/CLAUDE.md", "<!-- fresh-analysis-update -->"},
		{"vnext/templates/roles/reviewer.md", ".steamai-vnext/members/reviewer/CLAUDE.md", "<!-- fresh-reviewer-update -->"},
	}
	for _, update := range updates {
		path := filepath.Join(fixture.source, filepath.FromSlash(update.source))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, append(data, []byte("\n"+update.marker+"\n")...))
	}
	// 三处更新保持未暂存：当前 bytes 只应进入后续 Fresh。
	updated, _ := applyFresh(t, newCaseRoot(t))
	if updated.SourceDigest == preview.SourceDigest || updated.Identity == preview.Identity || updated.Revision != preview.Revision || updated.SnapshotDigest != preview.SnapshotDigest {
		t.Fatal("unstaged contract updates did not change only the source/preview identity")
	}
	for _, update := range updates {
		if !bytes.Contains(writeForTarget(t, updated, update.target).Data, []byte(update.marker)) {
			t.Fatalf("new Fresh missed working-tree update: %s", update.source)
		}
	}
	for i, role := range roles {
		text := writeForTarget(t, updated, ".steamai-vnext/contracts/templates/roles/"+role).Data
		otherMember := writeForTarget(t, updated, updates[2-i].target).Data
		if !bytes.Contains(text, []byte(updates[i+1].marker)) || bytes.Contains(otherMember, []byte(updates[i+1].marker)) {
			t.Fatalf("updated role %s was lost or mixed into the other member", role)
		}
	}
	after, err := InspectCurrent(oldRoot)
	if err != nil || !reflect.DeepEqual(identity, after) || !sameTree(oldTree, treeDigest(t, oldRoot)) {
		t.Fatalf("source update or later Fresh changed the old current case: %v", err)
	}
}
