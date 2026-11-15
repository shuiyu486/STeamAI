package casebootstrap

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// repoFreshFixture 把真实仓库 stage-0 的 skill/contracts/pack/common 闭包
// 复制进纯合成临时 source 并提交；exact confirmation 由本测试 harness
// 持有，不冒充用户。pack 选择参数化，供仓库级 Fresh 合同测试复用。
type repoFreshFixture struct {
	git     string
	source  string
	facts   Facts
	entries []gitEntry
}

func buildRealRepoFreshFixture(t *testing.T, pack, memberReads string) *repoFreshFixture {
	t.Helper()
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
		"vnext/templates", "packs/" + pack, "common",
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
	facts.Pack = pack
	facts.Members[0].Inputs = memberReads
	facts.Members[0].AllowedReads = memberReads
	return &repoFreshFixture{git: git, source: source, facts: facts, entries: entries}
}

// applyFresh 用生产 BuildPreview+Apply 发布一个 Fresh case：preview 零写入、
// 每个 planned write 落盘 bytes/source bytes/SHA 一致、闭包完整复制且
// InspectCurrent 与发布的 preview 匹配。
func (f *repoFreshFixture) applyFresh(t *testing.T, root string) (Preview, CurrentIdentity) {
	t.Helper()
	preview, err := BuildPreview(f.git, f.source, root, f.facts)
	if err != nil {
		t.Fatal(err)
	}
	if len(treeDigest(t, root)) != 0 {
		t.Fatal("preview wrote into the fresh case")
	}
	if applied, err := Apply(f.git, f.source, root, f.facts, ConfirmationPrefix+preview.Identity); err != nil || applied.Identity != preview.Identity {
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
			sourceData, err := os.ReadFile(filepath.Join(f.source, filepath.FromSlash(write.SourcePath)))
			if err != nil || !bytes.Equal(data, sourceData) {
				t.Fatalf("source bytes were not preserved: %s: %v", write.SourcePath, err)
			}
			copied[write.SourcePath] = true
		}
	}
	if len(copied) != len(f.entries) || !sameTree(expected, treeDigest(t, root)) {
		t.Fatal("Fresh did not materialize exactly the planned tree and source closure")
	}
	current, err := InspectCurrent(root)
	if err != nil || current.Pack != f.facts.Pack || current.SourceDigest != preview.SourceDigest || current.PayloadDigest != preview.SnapshotDigest {
		t.Fatalf("InspectCurrent does not match the published preview: %v", err)
	}
	return preview, current
}

// TestRepositoryFourStateAvailabilityMaterializeThroughFresh 验证真实仓库的
// canonical skill 与 common 四态 policy 经生产 BuildPreview+Apply 落进 Fresh
// case（真实 bytes，非合成 oracle），且源码工作树更新只进入后续 Fresh，
// 不改写已有 case。
func TestRepositoryFourStateAvailabilityMaterializeThroughFresh(t *testing.T) {
	fixture := buildRealRepoFreshFixture(t, "game-automation",
		"../../pack-snapshot/packs/game-automation/manifest.yml ../../pack-snapshot/packs/game-automation/references/game-automation/README.md")
	oldRoot := newCaseRoot(t)
	preview, identity := fixture.applyFresh(t, oldRoot)

	skill := writeForTarget(t, preview, ".claude/skills/steamai/SKILL.md").Data
	for _, required := range []string{
		"`未安装`、`已安装未暴露`、`会话可见未实测`、`本任务实测可用`",
		"common/policies/tool-adapters.md",
	} {
		if !bytes.Contains(skill, []byte(required)) {
			t.Fatalf("Fresh skill 缺少四态报告短指针合同: %s", required)
		}
	}
	policy := writeForTarget(t, preview, ".steamai-vnext/pack-snapshot/common/policies/tool-adapters.md").Data
	for _, required := range []string{
		"## 可用性四态报告", "`未安装`", "`已安装未暴露`", "`会话可见未实测`", "`本任务实测可用`",
		"安装事实已核实", "不造安装状态", "不臆断已安装", "权限阻塞", "不改判为未安装", "按需报告",
	} {
		if !bytes.Contains(policy, []byte(required)) {
			t.Fatalf("Fresh common policy 缺少四态判定细则: %s", required)
		}
	}

	oldTree := treeDigest(t, oldRoot)
	marker := []byte("\n<!-- four-state-policy-update -->\n")
	path := filepath.Join(fixture.source, filepath.FromSlash("common/policies/tool-adapters.md"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, append(append([]byte{}, data...), marker...))
	// 更新保持未暂存：当前 bytes 只应进入后续 Fresh。common 属 snapshot
	// payload，其工作树 bytes 变化应同时改变 source/payload 摘要与 preview
	// identity，而 Revision（HEAD）不变。
	updated, _ := fixture.applyFresh(t, newCaseRoot(t))
	if updated.SourceDigest == preview.SourceDigest || updated.Identity == preview.Identity || updated.Revision != preview.Revision || updated.SnapshotDigest == preview.SnapshotDigest {
		t.Fatal("unstaged policy update did not change the source/payload identity at unchanged revision")
	}
	if !bytes.Contains(writeForTarget(t, updated, ".steamai-vnext/pack-snapshot/common/policies/tool-adapters.md").Data, marker) {
		t.Fatal("new Fresh missed the working-tree policy update")
	}
	after, err := InspectCurrent(oldRoot)
	if err != nil || !reflect.DeepEqual(identity, after) || !sameTree(oldTree, treeDigest(t, oldRoot)) {
		t.Fatalf("source update or later Fresh changed the old current case: %v", err)
	}
}

// TestRepositoryGameAutomationAssetsMaterializeThroughFresh 验证
// game-automation 的示例和两种工具入口按真实仓库 bytes 经生产 Fresh
// 进入 snapshot。stage-0 index 缺失即漏交付，不得静默跳过 CI。
func TestRepositoryGameAutomationAssetsMaterializeThroughFresh(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	const packRoot = "packs/game-automation"
	assets := []string{
		packRoot + "/tooling/examples/windows-flow/localize.py",
		packRoot + "/tooling/examples/windows-flow/synth_target.py",
		packRoot + "/tooling/examples/windows-flow/synth_device.py",
		packRoot + "/tooling/integrations/dm.md",
		packRoot + "/tooling/integrations/airtest.md",
		packRoot + "/references/game-automation/toolchain-router.md",
	}
	// 大漠资料按版本目录自动发现：所有 dm-* 版本目录的全部普通文件（含
	// 未来版本目录内的 selection.json），不固定版本号、页数或扩展名。
	versions, err := filepath.Glob(filepath.Join(repository, filepath.FromSlash(packRoot+"/upstream/dm-*")))
	if err != nil || len(versions) == 0 {
		t.Fatalf("缺少大漠版本资料目录: %v", err)
	}
	for _, version := range versions {
		pages, err := os.ReadDir(version)
		if err != nil || len(pages) == 0 {
			t.Fatalf("大漠版本资料目录为空: %s: %v", version, err)
		}
		for _, page := range pages {
			if page.IsDir() {
				t.Fatalf("大漠版本资料目录不应包含子目录: %s/%s", version, page.Name())
			}
			assets = append(assets, packRoot+"/upstream/"+filepath.Base(version)+"/"+page.Name())
		}
	}
	// C++/MFC 基础工程与 dm_docs 资料工具按目录闭包枚举；source 交付闭包
	// 不得携带构建产物或 pycache。新增文件自动纳入，不需要登记清单。
	for _, root := range []string{packRoot + "/tooling/examples/dm-cpp-mfc", packRoot + "/tooling/scripts"} {
		err := filepath.WalkDir(filepath.Join(repository, filepath.FromSlash(root)), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(repository, path)
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if slices.Contains([]string{"__pycache__", "build", "CMakeFiles", "dist"}, entry.Name()) {
					t.Fatalf("source 交付闭包含构建产物目录: %s", filepath.ToSlash(rel))
				}
				return nil
			}
			if slices.Contains([]string{"CMakeCache.txt", "cmake_install.cmake"}, entry.Name()) || strings.HasSuffix(entry.Name(), ".pyc") {
				t.Fatalf("source 交付闭包含构建产物: %s", filepath.ToSlash(rel))
			}
			assets = append(assets, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	assets = append(assets, packRoot+"/tooling/dm-docs-update.md")
	probe, err := gitIndexEntries(git, repository, []string{packRoot})
	if err != nil {
		t.Fatal(err)
	}
	tracked := map[string]bool{}
	for _, entry := range probe {
		tracked[entry.Path] = true
	}
	var missing []string
	for _, asset := range assets {
		if !tracked[asset] {
			missing = append(missing, asset)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("game-automation 交付文件未进入 stage-0 index（漏 git add 即漏交付，不得静默通过 CI）: %v", missing)
	}

	fixture := buildRealRepoFreshFixture(t, "game-automation",
		"../../pack-snapshot/packs/game-automation/manifest.yml ../../pack-snapshot/packs/game-automation/references/game-automation/README.md")
	root := newCaseRoot(t)
	preview, _ := fixture.applyFresh(t, root)
	for _, asset := range assets {
		repoBytes, err := os.ReadFile(filepath.Join(repository, filepath.FromSlash(asset)))
		if err != nil {
			t.Fatal(err)
		}
		write := writeForTarget(t, preview, ".steamai-vnext/pack-snapshot/"+asset)
		caseBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(write.TargetPath)))
		if err != nil || !bytes.Equal(write.Data, repoBytes) || !bytes.Equal(caseBytes, repoBytes) || hashBytes(caseBytes) != write.SHA256 {
			t.Fatalf("交付文件未按实际 bytes 进入 pack snapshot: %s: %v", asset, err)
		}
	}
}
