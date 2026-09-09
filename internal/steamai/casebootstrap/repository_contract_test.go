package casebootstrap

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepositoryRoleContractsMaterializeThroughFresh(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := gitIndexEntries(git, repository, []string{
		".claude/skills/steamai/SKILL.md", "vnext/learning-feedback.md", "vnext/verified-learning.md",
		"vnext/templates", "packs/binary-re", "common",
	})
	if err != nil || len(entries) == 0 {
		t.Fatalf("repository source closure: %v", err)
	}
	// 读取当前工作树而非 HEAD bytes；只允许临时 source 写入 Git index。
	source := t.TempDir()
	for _, entry := range entries {
		path := filepath.Join(repository, filepath.FromSlash(entry.Path))
		if err := requirePlainPath(repository, path, false); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(source, filepath.FromSlash(entry.Path))
		writeFile(t, target, data)
		if entry.Mode == "100755" {
			if err := os.Chmod(target, 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	runGit(t, git, source, "init", "--quiet")
	runGit(t, git, source, "config", "user.name", "STeamAI fixture")
	runGit(t, git, source, "config", "user.email", "fixture@example.invalid")
	runGit(t, git, source, "config", "core.autocrlf", "false")
	runGit(t, git, source, "add", "--", ".")
	for _, entry := range entries {
		if entry.Mode == "100755" {
			// Windows 文件权限不能单独保留 Git index 的可执行位。
			runGit(t, git, source, "update-index", "--chmod=+x", "--", entry.Path)
		}
	}
	runGit(t, git, source, "commit", "--quiet", "-m", "repository contract fixture")

	facts := fixtureFacts()
	facts.Pack = "binary-re"
	facts.Members[0].Inputs = "../../pack-snapshot/packs/binary-re/manifest.yml ../../pack-snapshot/packs/binary-re/references/binary-re/README.md ../../pack-snapshot/packs/binary-re/references/binary-re/general-analysis.md"
	facts.Members[0].AllowedReads = facts.Members[0].Inputs
	facts.Members = append(facts.Members, MemberFacts{
		Name: "reviewer", Kind: "reviewer", Role: "independent reviewer", Responsibility: "review fixture claims",
		TaskGoal: "check fixture evidence against the supplied goal", Inputs: "../../evidence/E-fixture.md ../../findings/F-fixture.md",
		AllowedReads: "../../evidence/E-fixture.md ../../findings/F-fixture.md", AllowedWrites: "../../reviews/R-fixture.md",
		Deliverables: "independent fixture review", StopOrEscalate: "missing fixture evidence", ExitConditions: "review delivered",
	})
	applyFresh := func(root string) (Preview, CurrentIdentity) {
		t.Helper()
		preview, err := BuildPreview(git, source, root, facts)
		if err != nil {
			t.Fatal(err)
		}
		if len(treeDigest(t, root)) != 0 {
			t.Fatal("preview wrote into the fresh case")
		}
		if applied, err := Apply(git, source, root, facts, ConfirmationPrefix+preview.Identity); err != nil || applied.Identity != preview.Identity {
			t.Fatalf("Apply changed or rejected the preview: %v", err)
		}
		expected, copied := map[string]string{}, map[string]bool{}
		for _, write := range preview.Writes {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(write.TargetPath)))
			if err != nil || !bytes.Equal(data, write.Data) || len(data) != write.Bytes || hashBytes(data) != write.SHA256 {
				t.Fatalf("planned write differs from actual bytes: %s: %v", write.TargetPath, err)
			}
			expected[write.TargetPath] = write.SHA256
			if write.SourceKind == "working-tree" {
				sourceData, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(write.SourcePath)))
				if err != nil || !bytes.Equal(data, sourceData) {
					t.Fatalf("source bytes were not preserved: %s: %v", write.SourcePath, err)
				}
				copied[write.SourcePath] = true
			}
		}
		if len(copied) != len(entries) || !sameTree(expected, treeDigest(t, root)) {
			t.Fatal("Fresh did not materialize exactly the planned tree and source closure")
		}
		current, err := InspectCurrent(root)
		if err != nil || current.Pack != facts.Pack || current.SourceDigest != preview.SourceDigest || current.PayloadDigest != preview.SnapshotDigest {
			t.Fatalf("InspectCurrent does not match the published preview: %v", err)
		}
		return preview, current
	}

	oldRoot := newCaseRoot(t)
	preview, identity := applyFresh(oldRoot)
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
		path := filepath.Join(source, filepath.FromSlash(update.source))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, append(data, []byte("\n"+update.marker+"\n")...))
	}
	// 三处更新保持未暂存：当前 bytes 只应进入后续 Fresh。
	updated, _ := applyFresh(newCaseRoot(t))
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
