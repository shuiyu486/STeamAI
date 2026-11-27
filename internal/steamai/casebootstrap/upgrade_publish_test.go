package casebootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	upgradeFixtureSkill    = ".claude/skills/steamai/SKILL.md"
	upgradeFixtureLearning = ".steamai-vnext/contracts/learning-feedback.md"
	upgradeFixtureEvidence = ".steamai-vnext/contracts/templates/research/evidence.md"
	upgradeFixtureReviewer = ".steamai-vnext/contracts/templates/roles/reviewer.md"
	upgradeFixturePolicy   = ".steamai-vnext/pack-snapshot/common/policy.md"
	upgradeFixtureTool     = ".steamai-vnext/pack-snapshot/packs/new-pack/tooling/scripts/run.py"
)

// upgradePublishFixture 只搭出发布模块会碰到的框架文件，外加两个必须原样保留的项目文件。
func upgradePublishFixture(t *testing.T) string {
	t.Helper()
	root := newCaseRoot(t)
	for rel, text := range map[string]string{
		upgradeFixtureSkill:                        "# old skill\n",
		caseMarkerRel:                              "# old marker\n",
		upgradeFixtureLearning:                     "# old learning\n",
		upgradeFixtureEvidence:                     "# old evidence template\n",
		snapshotRel:                                "schema: old\n",
		upgradeFixturePolicy:                       "# old common\n",
		".steamai-vnext/members/analyst/CLAUDE.md": "# member\n",
		".steamai-vnext/evidence/E-1.md":           "# case evidence\n",
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte(text))
	}
	return root
}

func upgradeFixtureHash(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return hashBytes(data)
}

func upgradeFixtureReplace(t *testing.T, root, rel, text string) upgradeFile {
	t.Helper()
	return upgradeFile{Path: rel, Before: upgradeFixtureHash(t, root, rel), After: hashBytes([]byte(text)), Data: []byte(text)}
}

func upgradeFixtureDelete(t *testing.T, root, rel string) upgradeFile {
	t.Helper()
	return upgradeFile{Path: rel, Before: upgradeFixtureHash(t, root, rel)}
}

func upgradeFixtureCreate(rel, text string) upgradeFile {
	return upgradeFile{Path: rel, After: hashBytes([]byte(text)), Data: []byte(text)}
}

const (
	upgradeFixtureNewSkill    = "# new skill\n"
	upgradeFixtureNewLearning = "# new learning\n"
	upgradeFixtureNewReviewer = "# new reviewer template\n"
	upgradeFixtureNewTool     = "# synthetic tool that must not run\n"
)

// upgradeFixtureChanges 覆盖替换、删除、新建（含已有 pack 下多层全新目录）；项目说明故意排在最前。
func upgradeFixtureChanges(t *testing.T, root string) []upgradeFile {
	t.Helper()
	return []upgradeFile{
		upgradeFixtureReplace(t, root, caseMarkerRel, "# new marker\n"),
		upgradeFixtureReplace(t, root, upgradeFixtureSkill, upgradeFixtureNewSkill),
		upgradeFixtureReplace(t, root, upgradeFixtureLearning, upgradeFixtureNewLearning),
		upgradeFixtureDelete(t, root, upgradeFixtureEvidence),
		upgradeFixtureCreate(upgradeFixtureReviewer, upgradeFixtureNewReviewer),
		upgradeFixtureReplace(t, root, upgradeFixturePolicy, "# new common\n"),
		upgradeFixtureCreate(upgradeFixtureTool, upgradeFixtureNewTool),
		upgradeFixtureReplace(t, root, snapshotRel, "schema: new\n"),
	}
}

var upgradeFixtureOrder = []string{
	upgradeFixtureSkill, upgradeFixtureLearning, upgradeFixtureEvidence, upgradeFixtureReviewer,
	upgradeFixturePolicy, upgradeFixtureTool, snapshotRel, caseMarkerRel,
}

// upgradeFixtureExpected 是全部变更成功发布后的完整项目树（含新建文件所需的目录）。
func upgradeFixtureExpected(t *testing.T, root string, changes []upgradeFile) map[string]string {
	t.Helper()
	expected := upgradeTree(t, root)
	for _, change := range changes {
		if change.After == "" {
			delete(expected, change.Path)
			continue
		}
		expected[change.Path] = change.After
		if change.Before == "" {
			parts := strings.Split(change.Path, "/")
			for i := 1; i < len(parts); i++ {
				expected[strings.Join(parts[:i], "/")+"/"] = "dir"
			}
		}
	}
	return expected
}

func upgradeFixtureHook(t *testing.T, hook func(index int, rel string)) {
	t.Helper()
	upgradeStepHook = hook
	t.Cleanup(func() { upgradeStepHook = nil })
}

func upgradeFixturePending(root string) string {
	return filepath.Join(root, ".steamai-vnext", upgradePendingDir)
}

func assertUpgradeTree(t *testing.T, root string, want map[string]string) {
	t.Helper()
	got := upgradeTree(t, root)
	if len(got) == len(want) {
		for rel, digest := range want {
			if got[rel] != digest {
				t.Fatalf("%s = %s，期望 %s", rel, got[rel], digest)
			}
		}
		return
	}
	for rel, digest := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("多出 %s = %s", rel, digest)
		}
	}
	for rel, digest := range want {
		if _, ok := got[rel]; !ok {
			t.Errorf("缺少 %s = %s", rel, digest)
		}
	}
	t.Fatal("项目树与期望不一致")
}

func TestValidUpgradePath(t *testing.T) {
	for _, valid := range []string{
		".claude/skills/steamai/SKILL.md", caseMarkerRel, snapshotRel,
		".steamai-vnext/contracts/learning-feedback.md", ".steamai-vnext/contracts/templates/case/CLAUDE.md",
		".steamai-vnext/pack-snapshot/common/policies/agent-team.md",
		".steamai-vnext/pack-snapshot/packs/binary-re/tooling/scripts/export_function_evidence.py",
		".steamai-vnext/pack-snapshot/packs/game-automation/upstream/v1.2/dm.dll",
		".steamai-vnext/contracts/verified-learning v2.md", ".steamai-vnext/contracts/templates/.gitkeep",
	} {
		if !validUpgradePath(valid) {
			t.Errorf("合法路径被拒绝: %s", valid)
		}
	}
	for _, invalid := range []string{
		"", ".", "..", "/", ".steamai-vnext", ".steamai-vnext/", ".steamai-vnext/contracts/",
		".steamai-vnext/contracts", ".steamai-vnext/CLAUDE.md.bak", ".steamai-vnext/members/analyst/CLAUDE.md",
		".steamai-vnext/pack-snapshot/../CLAUDE.md", ".steamai-vnext/contracts/./x.md", ".steamai-vnext/contracts//x.md",
		".steamai-vnext/contracts/x.md/", ".steamai-vnext\\contracts\\x.md", ".steamai\\skills\\steamai\\SKILL.md",
		"C:/x/.steamai-vnext/contracts/x.md", "/.steamai-vnext/contracts/x.md", "//server/share/x.md",
		"CLAUDE.md", ".claude/settings.local.json", ".claude/skills/steamai/SKILL.md.orig",
		".steamai-vnext/contracts/x.md:stream", ".steamai-vnext/contracts/x.md.", ".steamai-vnext/contracts/x.md ",
		".steamai-vnext/contracts/ x.md", ".steamai-vnext/contracts/nul.md", ".steamai-vnext/contracts/NUL.txt",
		".steamai-vnext/contracts/con.md", ".steamai-vnext/contracts/aux.tar.gz", ".steamai-vnext/contracts/com1.md",
		".steamai-vnext/contracts/lpt9.md", ".steamai-vnext/contracts/COM¹.md", ".steamai-vnext/contracts/CONIN$.md",
		".steamai-vnext/contracts/x*.md", ".steamai-vnext/contracts/x?.md", ".steamai-vnext/contracts/x|y.md",
		".steamai-vnext/contracts/x<y.md", ".steamai-vnext/contracts/x\"y.md", ".steamai-vnext/contracts/x\x01y.md",
	} {
		if validUpgradePath(invalid) {
			t.Errorf("非法路径被接受: %q", invalid)
		}
	}
}

func TestPublishUpgradePublishesInOrderIntoNewDirectories(t *testing.T) {
	root := upgradePublishFixture(t)
	changes := upgradeFixtureChanges(t, root)
	expected := upgradeFixtureExpected(t, root, changes)
	var order []string
	upgradeFixtureHook(t, func(_ int, rel string) { order = append(order, rel) })
	verifyCalls := 0
	err := publishUpgrade(root, changes, func() error {
		verifyCalls++
		if _, err := os.Stat(upgradeFixturePending(root)); err != nil {
			return fmt.Errorf("深验时保留目录应仍存在: %w", err)
		}
		for _, change := range changes {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(change.Path)))
			if change.After == "" {
				if !os.IsNotExist(err) {
					return fmt.Errorf("%s 应已删除: %v", change.Path, err)
				}
				continue
			}
			if err != nil || string(data) != string(change.Data) {
				return fmt.Errorf("%s 在深验时不是新内容: %v", change.Path, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if verifyCalls != 1 {
		t.Fatalf("verify 调用次数 = %d", verifyCalls)
	}
	if strings.Join(order, "\n") != strings.Join(upgradeFixtureOrder, "\n") {
		t.Fatalf("发布顺序 = %v", order)
	}
	// 逐项比较整棵树：没有多余目录（含项目根），也没有保留目录残留。
	assertUpgradeTree(t, root, expected)
}

func TestPublishUpgradeWithoutMarkerChange(t *testing.T) {
	// 同一 HEAD 下只改 skill：项目说明不变，列表里没有它。
	root := upgradePublishFixture(t)
	changes := []upgradeFile{
		upgradeFixtureReplace(t, root, snapshotRel, "schema: new source digest\n"),
		upgradeFixtureReplace(t, root, upgradeFixtureSkill, upgradeFixtureNewSkill),
	}
	expected := upgradeFixtureExpected(t, root, changes)
	var order []string
	upgradeFixtureHook(t, func(_ int, rel string) { order = append(order, rel) })
	if err := publishUpgrade(root, changes, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, "\n") != strings.Join([]string{upgradeFixtureSkill, snapshotRel}, "\n") {
		t.Fatalf("发布顺序 = %v", order)
	}
	assertUpgradeTree(t, root, expected)
}

func TestPublishUpgradeRollsBackWhenVerifyFails(t *testing.T) {
	for _, withNewDirectories := range []bool{false, true} {
		t.Run(fmt.Sprint(withNewDirectories), func(t *testing.T) {
			root := upgradePublishFixture(t)
			changes := upgradeFixtureChanges(t, root)
			if !withNewDirectories {
				// 去掉多层全新目录下新建文件的那一项。
				changes = append(changes[:6], changes[7])
			}
			before := upgradeTree(t, root)
			deep := errors.New("deep check failed")
			err := publishUpgrade(root, changes, func() error { return deep })
			if !errors.Is(err, deep) || !strings.Contains(err.Error(), "已全部撤回") || errors.Is(err, ErrUpgradePreserved) {
				t.Fatalf("深验失败的返回 = %v", err)
			}
			assertUpgradeTree(t, root, before)
		})
	}
}

func TestPublishUpgradeRejectsInvalidPlansWithoutWriting(t *testing.T) {
	valid := func(t *testing.T, root string) []upgradeFile { return upgradeFixtureChanges(t, root) }
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, root string, changes []upgradeFile) []upgradeFile
	}{
		{"empty", func(*testing.T, string, []upgradeFile) []upgradeFile { return nil }},
		{"missing snapshot", func(t *testing.T, root string, _ []upgradeFile) []upgradeFile {
			return []upgradeFile{upgradeFixtureReplace(t, root, upgradeFixtureSkill, upgradeFixtureNewSkill)}
		}},
		{"member path", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			changes[0].Path = ".steamai-vnext/members/analyst/CLAUDE.md"
			return changes
		}},
		{"case evidence path", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			changes[0].Path = ".steamai-vnext/evidence/E-1.md"
			return changes
		}},
		{"escape", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			changes[0].Path = ".steamai-vnext/contracts/../../../CLAUDE.md"
			return changes
		}},
		{"unsafe windows name", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			changes[0].Path = ".steamai-vnext/contracts/nul.md"
			return changes
		}},
		{"duplicate", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			return append(changes, changes[1])
		}},
		{"no-op", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			changes[1].After, changes[1].Data = changes[1].Before, nil
			return changes
		}},
		{"data mismatch", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			changes[1].Data = []byte("# something else\n")
			return changes
		}},
		{"uppercase digest", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			changes[1].Before = strings.ToUpper(changes[1].Before)
			return changes
		}},
		{"delete with data", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			changes[3].Data = []byte("x")
			return changes
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := upgradePublishFixture(t)
			before := upgradeTree(t, root)
			err := publishUpgrade(root, test.mutate(t, root, valid(t, root)), func() error { return nil })
			if !errors.Is(err, ErrUpgradeInvalid) {
				t.Fatalf("无效变更列表返回 %v", err)
			}
			assertUpgradeTree(t, root, before)
		})
	}
	t.Run("nil verify", func(t *testing.T) {
		root := upgradePublishFixture(t)
		before := upgradeTree(t, root)
		if err := publishUpgrade(root, valid(t, root), nil); !errors.Is(err, ErrUpgradeInvalid) {
			t.Fatalf("nil verify 返回 %v", err)
		}
		assertUpgradeTree(t, root, before)
	})
}

func TestPublishUpgradeRejectsDriftBeforeWriting(t *testing.T) {
	for _, test := range []struct {
		name     string
		mutate   func(t *testing.T, root string, changes []upgradeFile) []upgradeFile
		contains string
	}{
		{"stale before", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			// 确认后文件被改过：清单里的 Before 不再等于当前内容。
			writeFile(t, filepath.Join(root, filepath.FromSlash(upgradeFixturePolicy)), []byte("# edited after preview\n"))
			return changes
		}, upgradeFixturePolicy},
		{"create target exists", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			writeFile(t, filepath.Join(root, filepath.FromSlash(upgradeFixtureReviewer)), []byte("# appeared after preview\n"))
			return changes
		}, upgradeFixtureReviewer},
		{"delete target missing", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(upgradeFixtureEvidence))); err != nil {
				t.Fatal(err)
			}
			return changes
		}, upgradeFixtureEvidence},
		{"target replaced by directory", func(t *testing.T, root string, changes []upgradeFile) []upgradeFile {
			path := filepath.Join(root, filepath.FromSlash(upgradeFixtureLearning))
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			return changes
		}, upgradeFixtureLearning},
		{"case alias", func(t *testing.T, root string, _ []upgradeFile) []upgradeFile {
			writeFile(t, filepath.Join(root, filepath.FromSlash(".steamai-vnext/contracts/Guide.md")), []byte("# guide\n"))
			aliased := upgradeFile{Path: ".steamai-vnext/contracts/guide.md", Before: hashBytes([]byte("# guide\n")),
				After: hashBytes([]byte("# new guide\n")), Data: []byte("# new guide\n")}
			return []upgradeFile{aliased, upgradeFixtureReplace(t, root, snapshotRel, "schema: new\n")}
		}, "别名"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := upgradePublishFixture(t)
			changes := test.mutate(t, root, upgradeFixtureChanges(t, root))
			before := upgradeTree(t, root)
			err := publishUpgrade(root, changes, func() error { return nil })
			if !errors.Is(err, ErrTargetDrift) || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("前态漂移返回 %v", err)
			}
			assertUpgradeTree(t, root, before)
		})
	}
}

func TestPublishUpgradeRefusesPreExistingPending(t *testing.T) {
	for _, kind := range []string{"directory", "file"} {
		t.Run(kind, func(t *testing.T) {
			root := upgradePublishFixture(t)
			pending := upgradeFixturePending(root)
			if kind == "directory" {
				writeFile(t, filepath.Join(pending, "keep.txt"), []byte("previous originals\n"))
			} else if err := os.WriteFile(pending, []byte("not ours\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			changes := upgradeFixtureChanges(t, root)
			before := upgradeTree(t, root)
			err := publishUpgrade(root, changes, func() error { return nil })
			if !errors.Is(err, ErrUpgradePending) || !strings.Contains(err.Error(), pending) {
				t.Fatalf("预先存在的保留目录返回 %v", err)
			}
			assertUpgradeTree(t, root, before)
		})
	}
}

func TestPublishUpgradeRollsBackWhenCreateTargetAppears(t *testing.T) {
	root := upgradePublishFixture(t)
	changes := upgradeFixtureChanges(t, root)
	before := upgradeTree(t, root)
	upgradeFixtureHook(t, func(_ int, rel string) {
		if rel == upgradeFixtureReviewer {
			// 复核之后、发布之前，别的写者建出了同名文件。
			writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte("# foreign\n"))
		}
	})
	err := publishUpgrade(root, changes, func() error { return errors.New("不应到达深验") })
	if !errors.Is(err, ErrTargetDrift) || !strings.Contains(err.Error(), upgradeFixtureReviewer) || !strings.Contains(err.Error(), "已全部撤回") {
		t.Fatalf("新建目标冲突返回 %v", err)
	}
	// 之前已发布的全部撤回；意外出现的文件原样保留，也不再多出任何目录。
	expected := before
	expected[upgradeFixtureReviewer] = hashBytes([]byte("# foreign\n"))
	expected[".steamai-vnext/contracts/templates/roles/"] = "dir"
	assertUpgradeTree(t, root, expected)
}

func TestPublishUpgradeRollsBackWhenNewContentCannotBePut(t *testing.T) {
	root := upgradePublishFixture(t)
	changes := upgradeFixtureChanges(t, root)
	before := upgradeTree(t, root)
	upgradeFixtureHook(t, func(_ int, rel string) {
		// 模拟原件移出后新内容放不进去：把保留目录里对应的新内容拿走。
		if rel == upgradeFixturePolicy {
			if err := os.Remove(filepath.Join(upgradeFixturePending(root), "new", filepath.FromSlash(rel))); err != nil {
				t.Fatal(err)
			}
		}
	})
	err := publishUpgrade(root, changes, func() error { return errors.New("不应到达深验") })
	if err == nil || errors.Is(err, ErrUpgradePreserved) || !strings.Contains(err.Error(), upgradeFixturePolicy) || !strings.Contains(err.Error(), "已全部撤回") {
		t.Fatalf("新内容放不进去的返回 = %v", err)
	}
	assertUpgradeTree(t, root, before)
}

func TestPublishUpgradePreservesWhenRollbackIsUnsafe(t *testing.T) {
	root := upgradePublishFixture(t)
	changes := upgradeFixtureChanges(t, root)
	before := upgradeTree(t, root)
	const foreign = "# someone edited the published skill\n"
	upgradeFixtureHook(t, func(_ int, rel string) {
		if rel == upgradeFixtureReviewer {
			writeFile(t, filepath.Join(root, filepath.FromSlash(upgradeFixtureSkill)), []byte(foreign))
			writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte("# foreign\n"))
		}
	})
	err := publishUpgrade(root, changes, func() error { return errors.New("不应到达深验") })
	if !errors.Is(err, ErrUpgradePreserved) || !strings.Contains(err.Error(), upgradeFixtureSkill) || strings.Contains(err.Error(), foreign) {
		t.Fatalf("不安全撤回返回 %v", err)
	}
	pending := upgradeFixturePending(root)
	// 被第三方改过的 skill 没有被收回，它的原件仍在 backup；能安全撤回的条目原件已放回、新内容收回到 new。
	for rel, want := range map[string]string{
		"README.txt": upgradePendingREADME,
		filepath.Join("backup", filepath.FromSlash(upgradeFixtureSkill)): "# old skill\n",
		filepath.Join("new", filepath.FromSlash(upgradeFixtureLearning)): upgradeFixtureNewLearning,
		filepath.Join("new", filepath.FromSlash(upgradeFixturePolicy)):   "# new common\n",
	} {
		data, err := os.ReadFile(filepath.Join(pending, rel))
		if err != nil || string(data) != want {
			t.Fatalf("保留目录 %s = %q, %v", rel, data, err)
		}
	}
	for _, rel := range []string{filepath.Join("backup", filepath.FromSlash(upgradeFixtureLearning)), filepath.Join("backup", filepath.FromSlash(upgradeFixtureEvidence))} {
		if _, err := os.Stat(filepath.Join(pending, rel)); !os.IsNotExist(err) {
			t.Fatalf("已放回的原件不应还留在 %s: %v", rel, err)
		}
	}
	failure, err := os.ReadFile(filepath.Join(pending, "failure.txt"))
	if err != nil || !strings.Contains(string(failure), upgradeFixtureSkill) || strings.Contains(string(failure), foreign) {
		t.Fatalf("failure.txt = %q, %v", failure, err)
	}
	var manifest upgradeManifest
	manifestData, err := os.ReadFile(filepath.Join(pending, "manifest.json"))
	if err != nil || json.Unmarshal(manifestData, &manifest) != nil || manifest.Schema != upgradeManifestSchema || len(manifest.Changes) != len(changes) {
		t.Fatalf("manifest.json 无效: %s, %v", manifestData, err)
	}
	if got := upgradeFixtureHash(t, root, upgradeFixtureSkill); got != hashBytes([]byte(foreign)) {
		t.Fatal("被第三方改动的已发布文件被覆盖")
	}
	if got := upgradeFixtureHash(t, root, upgradeFixtureLearning); got != before[upgradeFixtureLearning] {
		t.Fatal("可安全撤回的条目没有撤回")
	}
	if got := upgradeFixtureHash(t, root, upgradeFixtureEvidence); got != before[upgradeFixtureEvidence] {
		t.Fatal("被删除的原件没有恢复")
	}
	if _, err := os.Lstat(filepath.Join(root, ".steamai-vnext", "pack-snapshot", "packs")); !os.IsNotExist(err) {
		t.Fatalf("未发布到的新目录被创建: %v", err)
	}
}

// 原件备份在撤回前丢失时，不能先把项目里的新内容收回（那会让项目缺文件）：保留新内容并报告。
func TestPublishUpgradeKeepsPublishedContentWhenOriginalBackupIsMissing(t *testing.T) {
	root := upgradePublishFixture(t)
	changes := upgradeFixtureChanges(t, root)
	before := upgradeTree(t, root)
	upgradeFixtureHook(t, func(_ int, rel string) {
		if rel == upgradeFixtureReviewer {
			if err := os.Remove(filepath.Join(upgradeFixturePending(root), "backup", filepath.FromSlash(upgradeFixtureSkill))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte("# foreign\n"))
		}
	})
	err := publishUpgrade(root, changes, func() error { return errors.New("不应到达深验") })
	if !errors.Is(err, ErrUpgradePreserved) || !strings.Contains(err.Error(), upgradeFixtureSkill) {
		t.Fatalf("原件备份缺失时返回 %v", err)
	}
	if got := upgradeFixtureHash(t, root, upgradeFixtureSkill); got != hashBytes([]byte(upgradeFixtureNewSkill)) {
		t.Fatal("原件备份缺失时收回了项目里的新内容")
	}
	if got := upgradeFixtureHash(t, root, upgradeFixtureLearning); got != before[upgradeFixtureLearning] {
		t.Fatal("其它可安全撤回的条目没有撤回")
	}
	if _, err := os.Stat(filepath.Join(upgradeFixturePending(root), "failure.txt")); err != nil {
		t.Fatalf("未写 failure.txt: %v", err)
	}
}

func TestUpgradeTakeOriginalVerifiesMovedBytes(t *testing.T) {
	root := upgradePublishFixture(t)
	target := filepath.Join(root, filepath.FromSlash(upgradeFixtureLearning))
	backup := filepath.Join(upgradeFixturePending(root), "backup", filepath.FromSlash(upgradeFixtureLearning))
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		t.Fatal(err)
	}
	confirmed := upgradeFixtureHash(t, root, upgradeFixtureLearning)

	// 复核之后文件被改过：移入槽位、发现不是原件、原样放回。
	writeFile(t, target, []byte("# edited after confirmation\n"))
	edited := upgradeFixtureHash(t, root, upgradeFixtureLearning)
	moved, err := upgradeTakeOriginal(target, backup, confirmed, upgradeFixtureLearning)
	if moved || !errors.Is(err, ErrTargetDrift) {
		t.Fatalf("被改过的原件 moved=%v err=%v", moved, err)
	}
	if got := upgradeFixtureHash(t, root, upgradeFixtureLearning); got != edited {
		t.Fatal("被改过的文件没有原样放回")
	}
	if _, err := os.Lstat(backup); !os.IsNotExist(err) {
		t.Fatalf("放回后槽位应已空: %v", err)
	}

	// 仍是确认时的原件：移入槽位并报告 moved。
	moved, err = upgradeTakeOriginal(target, backup, edited, upgradeFixtureLearning)
	if !moved || err != nil {
		t.Fatalf("原件 moved=%v err=%v", moved, err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("原件移出后目标应不存在: %v", err)
	}
	if got := upgradeFixtureHash(t, root, filepath.Join(".steamai-vnext", upgradePendingDir, "backup", filepath.FromSlash(upgradeFixtureLearning))); got != edited {
		t.Fatal("槽位里的不是移出的原件")
	}
}

func TestPublishUpgradeInterruptedRunIsRecognizableAndBlocksRetry(t *testing.T) {
	root := upgradePublishFixture(t)
	changes := upgradeFixtureChanges(t, root)
	before := upgradeTree(t, root)
	ordered, err := buildUpgradeOrder(changes)
	if err != nil {
		t.Fatal(err)
	}
	run, err := newUpgradeRun(root, ordered)
	if err != nil {
		t.Fatal(err)
	}
	if err := run.stage(); err != nil {
		t.Fatal(err)
	}
	if err := run.publishOne(0); err != nil {
		t.Fatal(err)
	}
	// 进程在此被硬中断：没有撤回，也没有清理。
	after := upgradeTree(t, root)
	if after[upgradeFixtureSkill] != hashBytes([]byte(upgradeFixtureNewSkill)) {
		t.Fatal("已发布的第一个文件不是新内容")
	}
	for rel, digest := range before {
		if rel != upgradeFixtureSkill && after[rel] != digest {
			t.Fatalf("中断后其它文件被改动: %s", rel)
		}
	}
	pending := upgradeFixturePending(root)
	for rel, want := range map[string]string{
		"README.txt": upgradePendingREADME,
		filepath.Join("backup", filepath.FromSlash(upgradeFixtureSkill)): "# old skill\n",
		filepath.Join("new", filepath.FromSlash(caseMarkerRel)):          "# new marker\n",
	} {
		data, err := os.ReadFile(filepath.Join(pending, rel))
		if err != nil || string(data) != want {
			t.Fatalf("中断后保留目录 %s = %q, %v", rel, data, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(pending, "new", filepath.FromSlash(upgradeFixtureSkill))); !os.IsNotExist(err) {
		t.Fatalf("已发布的新内容不应还留在 new: %v", err)
	}
	interrupted := upgradeTree(t, root)
	err = publishUpgrade(root, changes, func() error { return nil })
	if !errors.Is(err, ErrUpgradePending) {
		t.Fatalf("中断后重试返回 %v", err)
	}
	assertUpgradeTree(t, root, interrupted)
}

func TestPublishUpgradeLeavesReplacementPendingAlone(t *testing.T) {
	root := upgradePublishFixture(t)
	pending := upgradeFixturePending(root)
	displaced := filepath.Join(filepath.Dir(pending), "saved-originals")
	before := upgradeTree(t, root)
	err := publishUpgrade(root, upgradeFixtureChanges(t, root), func() error {
		if err := os.Rename(pending, displaced); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(pending, "keep.txt"), []byte("do not touch"))
		return errors.New("不能安全完成本次核验")
	})
	if !errors.Is(err, ErrUpgradePreserved) {
		t.Fatalf("保留目录换过仍尝试撤回：%v", err)
	}
	foreign := upgradeTree(t, pending)
	if len(foreign) != 2 || foreign["keep.txt"] != hashBytes([]byte("do not touch")) {
		t.Fatalf("动了不属于本次更新的目录：%v", foreign)
	}
	if got := upgradeFixtureHash(t, displaced, "backup/"+upgradeFixtureSkill); got != before[upgradeFixtureSkill] {
		t.Fatal("先前移出的原件未保留")
	}
}

func TestPublishUpgradeFinishRefusesForeignManifest(t *testing.T) {
	root := upgradePublishFixture(t)
	ordered, err := buildUpgradeOrder(upgradeFixtureChanges(t, root))
	if err != nil {
		t.Fatal(err)
	}
	run, err := newUpgradeRun(root, ordered)
	if err != nil {
		t.Fatal(err)
	}
	if err := run.stage(); err != nil {
		t.Fatal(err)
	}
	for index := range run.changes {
		if err := run.publishOne(index); err != nil {
			t.Fatal(err)
		}
	}
	manifest := filepath.Join(run.pending, "manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"schema":"`+upgradeManifestSchema+`","id":"00000000000000000000000000000000"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run.finish(); !errors.Is(err, ErrUpgradePreserved) {
		t.Fatalf("清单不符时返回 %v", err)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("清单不符的目录被删除: %v", err)
	}
}
