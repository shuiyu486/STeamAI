package casebootstrap

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 同时记录目录，避免把预览留下的空 staging 误判为零写入。
func upgradeTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := treeDigest(t, root)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			tree[filepath.ToSlash(rel)+"/"] = "dir"
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func freshForUpgrade(t *testing.T, git, source, root string, facts Facts) {
	t.Helper()
	p, err := BuildPreview(git, source, root, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(git, source, root, facts, ConfirmationPrefix+p.Identity); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeOnlyConfirmedFrameworkAndKeepsProjectWork(t *testing.T) {
	git, source := canonicalFixture(t)
	root, other := newCaseRoot(t), newCaseRoot(t)
	facts := fixtureFacts()
	facts.AuxPack = "aux-pack"
	freshForUpgrade(t, git, source, root, facts)
	freshForUpgrade(t, git, source, other, facts)
	control := upgradeTree(t, other)
	for _, path := range []string{"CLAUDE.md", "src/user.cpp", ".claude/settings.local.json", ".steamai-vnext/artifacts/index.md", ".steamai-vnext/evidence/E-old.md", ".steamai-vnext/reviews/R-old.md", ".steamai-vnext/learnings/candidates/L-old.md", ".steamai-vnext/evaluations/runs/prior/result.json"} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte("case-only\r\nprivate-fixture\r\n"))
	}
	memberPath := filepath.Join(root, ".steamai-vnext/members/analyst/CLAUDE.md")
	member, err := os.ReadFile(memberPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, memberPath, append(member, []byte("\r\n当前任务：保留用户的纠偏。\r\n")...))
	markerPath := filepath.Join(root, filepath.FromSlash(caseMarkerRel))
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	handoff := []byte("\n## Commander 交接\r\n原负责人尚未交付，已发操作不重做。\r\n")
	writeFile(t, markerPath, append(marker, handoff...))
	before := upgradeTree(t, root)
	writeFile(t, filepath.Join(source, "common/policy.md"), []byte("common v2\n"))
	writeFile(t, filepath.Join(source, "packs/aux-pack/router.md"), []byte("aux v2\n"))
	writeFile(t, filepath.Join(source, "packs/fixture-pack/new.txt"), []byte("new material\n"))
	runGit(t, git, source, "add", "--", "packs/fixture-pack/new.txt")
	runGit(t, git, source, "rm", "--quiet", "--", "packs/fixture-pack/method.md")
	writeFile(t, filepath.Join(source, ".claude/skills/steamai/SKILL.md"), []byte("new skill\n"))
	writeFile(t, filepath.Join(source, "vnext/templates/member/CLAUDE.md"), []byte("updated template for future members\n"))
	const physicalID = "fixture-case-id"
	preview := func() string {
		t.Helper()
		s, err := PreviewUpgrade(git, source, root, physicalID)
		if err != nil {
			t.Fatal(err)
		}
		return s.CommandText()
	}
	token := func(s string) string {
		t.Helper()
		_, v, ok := strings.Cut(s, "steamai upgrade --apply ")
		v = strings.TrimSpace(v)
		if !ok || !shaPattern.MatchString(v) {
			t.Fatalf("缺确认: %s", s)
		}
		return v
	}
	s := preview()
	for _, want := range []string{"新增  .steamai-vnext/pack-snapshot/packs/fixture-pack/new.txt", "移除  .steamai-vnext/pack-snapshot/packs/fixture-pack/method.md", "替换  .steamai-vnext/pack-snapshot/packs/aux-pack/router.md", "替换  .claude/skills/steamai/SKILL.md"} {
		if !strings.Contains(s, want) {
			t.Fatalf("预览缺少 %s", want)
		}
	}
	if strings.Contains(s, "private-fixture") || strings.Contains(s, string(handoff)) {
		t.Fatal("预览泄露项目记录")
	}
	id := token(s)
	if err := ApplyUpgrade(git, source, root, "another-physical-directory", id); err == nil {
		t.Fatal("旧确认可用于不同物理目录")
	}
	if err := ApplyUpgrade(git, source, other, "other-id", id); err == nil {
		t.Fatal("跨项目复用确认")
	}
	writeFile(t, filepath.Join(source, "common/policy.md"), []byte("common v3\n"))
	if err := ApplyUpgrade(git, source, root, physicalID, id); err == nil {
		t.Fatal("同一 HEAD 下来源字节改变仍应用")
	}
	if !sameTree(before, upgradeTree(t, root)) {
		t.Fatal("预览或拒绝确认时改变了项目")
	}
	id = token(preview())
	handoff = append(handoff, []byte("另一个已确认的断点。\r\n")...)
	writeFile(t, markerPath, append(marker, handoff...))
	before = upgradeTree(t, root)
	if err := ApplyUpgrade(git, source, root, physicalID, id); err == nil || !sameTree(before, upgradeTree(t, root)) {
		t.Fatal("交接变化后旧确认仍有效或发生了写入")
	}
	if err := ApplyUpgrade(git, source, root, physicalID, token(preview())); err != nil {
		t.Fatal(err)
	}
	after := upgradeTree(t, root)
	for path, value := range before {
		if strings.HasPrefix(path, ".steamai-vnext/pack-snapshot/") || strings.HasPrefix(path, ".steamai-vnext/contracts/") || path == caseMarkerRel || path == ".claude/skills/steamai/SKILL.md" {
			continue
		}
		if after[path] != value {
			t.Fatalf("更新改变非框架文件或目录: %s", path)
		}
	}
	m, err := os.ReadFile(markerPath)
	if err != nil || !bytes.HasSuffix(m, handoff) {
		t.Fatal("丢失项目交接")
	}
	if !sameTree(control, upgradeTree(t, other)) {
		t.Fatal("旁边的项目被更新")
	}
	// 独立从相同来源 Fresh，逐个对比 skill、合同和快照，而不是仅检查来源提交。
	ref := newCaseRoot(t)
	freshForUpgrade(t, git, source, ref, facts)
	wanted := upgradeTree(t, ref)
	for path, value := range wanted {
		if path == ".claude/skills/steamai/SKILL.md" || strings.HasPrefix(path, ".steamai-vnext/pack-snapshot/") || strings.HasPrefix(path, ".steamai-vnext/contracts/") {
			if after[path] != value {
				t.Fatalf("与新 Fresh 内容不同: %s", path)
			}
		}
	}
	if _, ok := after[".steamai-vnext/pack-snapshot/packs/fixture-pack/method.md"]; ok {
		t.Fatal("没有移除预览中列出的旧框架文件")
	}
	if _, err := InspectCurrent(root); err != nil {
		t.Fatal(err)
	}
	if got := preview(); !strings.Contains(got, "无需更新") || strings.Contains(got, "--apply") || !sameTree(after, upgradeTree(t, root)) {
		t.Fatalf("再次预览: %s", got)
	}
	// 再迭代一次：同一 HEAD 下仅 skill 有变，case 说明连版本行都不应被重写。
	writeFile(t, filepath.Join(source, ".claude/skills/steamai/SKILL.md"), []byte("skill v3 only\n"))
	s = preview()
	if strings.Contains(s, "替换  "+caseMarkerRel+"\n") {
		t.Fatal("仅改 skill 却计划重写项目说明")
	}
	// 本次不发布项目说明；若写入期间出现新交接，仍须撤回自己的文件，保留对方的交接并重新预览。
	changedMarker := append(bytes.Clone(m), []byte("刚收到的交接补充。\r\n")...)
	if !t.Run("写入期间交接变化", func(t *testing.T) {
		upgradeStepHook = func(_ int, rel string) {
			if rel == ".claude/skills/steamai/SKILL.md" {
				writeFile(t, markerPath, changedMarker)
			}
		}
		t.Cleanup(func() { upgradeStepHook = nil })
		if err := ApplyUpgrade(git, source, root, physicalID, token(s)); !errors.Is(err, ErrTargetDrift) {
			t.Fatalf("没有拒绝并发改过的交接：%v", err)
		}
		kept, err := os.ReadFile(markerPath)
		tree := upgradeTree(t, root)
		tree[caseMarkerRel] = after[caseMarkerRel] // 唯一应与上一版本不同的文件由另一写者产生。
		if err != nil || !bytes.Equal(kept, changedMarker) || !sameTree(after, tree) {
			t.Fatal("没有保留新交接，或没有完整撤回本次框架写入")
		}
	}) {
		return
	}
	if err := ApplyUpgrade(git, source, root, physicalID, token(preview())); err != nil {
		t.Fatal(err)
	}
	updatedMarker, err := os.ReadFile(markerPath)
	if err != nil || !bytes.Equal(changedMarker, updatedMarker) || !strings.Contains(preview(), "无需更新") {
		t.Fatal("第二次迭代未正确完成")
	}
}

func TestUpgradeDoesNotReplaceProjectHistoryWithoutContentChanges(t *testing.T) {
	git, source := canonicalFixture(t)
	root := newCaseRoot(t)
	policy := filepath.Join(source, "common/policy.md")
	writeFile(t, policy, []byte("already received\n"))
	freshForUpgrade(t, git, source, root, fixtureFacts())
	before := upgradeTree(t, root)
	// 先只提交项目以外的文件，再提交本项目已经收到的原样字节。
	writeFile(t, filepath.Join(source, "README.md"), []byte("fixture notes\n"))
	for _, rel := range []string{"README.md", "common/policy.md"} {
		runGit(t, git, source, "add", "--", rel)
		runGit(t, git, source, "commit", "-qm", "record "+rel)
		p, err := PreviewUpgrade(git, source, root, "fixture")
		if err != nil || p.Confirmation != "" || !strings.Contains(p.Summary, "无需更新") || !sameTree(before, upgradeTree(t, root)) {
			t.Errorf("只变化来源记录 %s 却要求更新项目：%q (%v)", rel, p.Summary, err)
		}
	}
	writeFile(t, policy, []byte("new content\n"))
	p, err := PreviewUpgrade(git, source, root, "fixture")
	if err != nil || p.Confirmation == "" || !strings.Contains(p.Details, "pack-snapshot/common/policy.md") {
		t.Fatalf("实际内容变化应仍可采用：%q (%v)", p.Summary, err)
	}
}

func TestUpgradeContractAloneDoesNotEnableMissingEvaluationWorkspace(t *testing.T) {
	git, source := canonicalFixture(t)
	root := newCaseRoot(t)
	freshForUpgrade(t, git, source, root, fixtureFacts())
	if ok, err := SupportsVerifiedLearning(root); err != nil || !ok {
		t.Fatalf("新项目应具备完整评测布局：%v", err)
	}
	// 即使完整合同已固定，来源比评测工作区旧或使用者没有这个目录时，也不能把可用性判成真。
	if err := os.Remove(filepath.Join(root, ".steamai-vnext/evaluations/work")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrent(root); err != nil {
		t.Fatal(err)
	}
	if ok, err := SupportsVerifiedLearning(root); err != nil || ok {
		t.Fatalf("仅存在合同就报告可用：%v", err)
	}
}

func TestUpgradeRejectsDamageAndDowngradeWithoutWriting(t *testing.T) {
	git, source := canonicalFixture(t)
	runGit(t, git, source, "commit", "--allow-empty", "-qm", "second")
	root := newCaseRoot(t)
	freshForUpgrade(t, git, source, root, fixtureFacts())
	before := upgradeTree(t, root)
	runGit(t, git, source, "checkout", "--quiet", "--detach", "HEAD~1")
	if _, err := PreviewUpgrade(git, source, root, "fixture"); err == nil || !strings.Contains(err.Error(), "不降级") || !sameTree(before, upgradeTree(t, root)) {
		t.Fatalf("接受倒退或修改项目: %v", err)
	}
	writeFile(t, filepath.Join(root, ".steamai-vnext/pack-snapshot/snapshot.yml"), []byte("sensitive fixture metadata\n"))
	before = upgradeTree(t, root)
	_, err := PreviewUpgrade(git, source, root, "fixture")
	if err == nil || strings.Contains(err.Error(), "sensitive") || !strings.Contains(err.Error(), "snapshot.yml") || !sameTree(before, upgradeTree(t, root)) {
		t.Fatalf("坏清单未安全拒绝: %v", err)
	}
}
