package casebootstrap

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRepositoryGeneralDeliveryPackMaterializeThroughFresh 验证普通项目主包
// general-delivery 按真实仓库 stage-0 bytes 经生产 BuildPreview+Apply 完整进入
// Fresh case，并与“没选这个包”和“已有 case”区分开：
//
//   - pack 目录下每个文件都在 stage-0 index：漏 git add 即漏交付，不得静默跳过 CI；
//   - snapshot 里该 pack 的每个分发文件与仓库 working-tree bytes 逐字节一致，
//     packs/ 下只有这一个包，不夹带或补装别的 pack；
//   - 分发内容带着本包的核心合同，router 指向的 common 入口和成员任务里的路径
//     在发布后的 case 中都解析到真实文件；
//   - 选了别的包的 case 不分发本包文件；
//   - source 后续更新只进入之后的新 Fresh，已有 case 的身份与 bytes 不变。
func TestRepositoryGeneralDeliveryPackMaterializeThroughFresh(t *testing.T) {
	const (
		pack     = "general-delivery"
		packRoot = "packs/" + pack
		manifest = packRoot + "/manifest.yml"
		router   = packRoot + "/references/" + pack + "/README.md"
		examples = packRoot + "/references/" + pack + "/examples.md"
		snapshot = ".steamai-vnext/pack-snapshot/"
	)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	// 交付闭包按目录枚举：pack 内新增文件自动纳入，漏 git add 即漏交付。
	tracked := map[string]bool{}
	entries, err := gitIndexEntries(git, repository, []string{packRoot})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		tracked[entry.Path] = true
	}
	var files, unstaged []string
	err = filepath.WalkDir(filepath.Join(repository, filepath.FromSlash(packRoot)), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repository, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, rel)
		if !tracked[rel] || !entry.Type().IsRegular() {
			unstaged = append(unstaged, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unstaged) > 0 {
		t.Fatalf("general-delivery 交付文件未进入 stage-0 index（漏 git add 即漏交付，不得静默通过 CI）: %v", unstaged)
	}

	// 核心合同：分发的必须是当前这版做法，旧版或空壳 pack 不能通过。
	contract := map[string][]string{
		manifest: {"name: " + pack, "id: run-provided-program", "id: install-dependency-or-runtime", "id: external-delivery"},
		router: {
			"接收者怎么用", "新写法不必照抄旧实现", "资料没写的不补造",
		},
		examples: {"范例 A", "范例 B", "范例 C", "先记下要保持的行为", "新需求有两种读法", "脱离源码检验", "未知，资料没写", "换一份资料再核"},
	}
	// router 写的是相对本包根目录的 common 入口；成员任务写的是 member-relative 路径。
	routes := []string{"../../common/policies/verification.md", "../../common/tooling/windows-desktop-mcp.md"}
	reads := []string{
		"../../pack-snapshot/" + manifest, "../../pack-snapshot/" + router, "../../pack-snapshot/" + examples,
		"../../pack-snapshot/common/policies/verification.md",
	}

	fixture := buildRealRepoFreshFixture(t, pack, strings.Join(reads, " "))
	oldRoot := newCaseRoot(t)
	preview, identity := fixture.applyFresh(t, oldRoot)
	oldTree := treeDigest(t, oldRoot)

	// 读取实际分发内容：pack 命名空间下恰好是目录里的全部文件，bytes 与仓库一致。
	expected := make(map[string]bool, len(files))
	for _, rel := range files {
		expected[snapshot+rel] = true
		repoBytes, err := os.ReadFile(filepath.Join(repository, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		caseBytes, err := os.ReadFile(filepath.Join(oldRoot, filepath.FromSlash(snapshot+rel)))
		if err != nil || !bytes.Equal(caseBytes, repoBytes) {
			t.Fatalf("交付文件未按实际 bytes 进入 pack snapshot: %s: %v", rel, err)
		}
	}
	for rel := range oldTree {
		if strings.HasPrefix(rel, snapshot+packRoot+"/") && !expected[rel] {
			t.Fatalf("snapshot 的 pack 命名空间多出文件: %s", rel)
		}
	}
	names, err := os.ReadDir(filepath.Join(oldRoot, filepath.FromSlash(snapshot+"packs")))
	if err != nil || len(names) != 1 || names[0].Name() != pack {
		t.Fatalf("snapshot 夹带或补装了其它 pack: %v (%v)", names, err)
	}
	for rel, markers := range contract {
		data, err := os.ReadFile(filepath.Join(oldRoot, filepath.FromSlash(snapshot+rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, marker := range markers {
			if !bytes.Contains(data, []byte(marker)) {
				t.Fatalf("分发内容缺少本包核心合同 %s: %q", rel, marker)
			}
		}
	}

	// 从 Commander 实际收到的项目说明，经 manifest 到 router，不依赖先知道包名。
	entry := snapshot + manifest
	guide, err := os.ReadFile(filepath.Join(oldRoot, ".steamai-vnext", "CLAUDE.md"))
	if err != nil || !bytes.Contains(guide, []byte("`"+entry+"`")) {
		t.Fatalf("项目说明未指向本项目实际分发的主包入口 %s: %v", entry, err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(oldRoot, filepath.FromSlash(entry)))
	if err != nil || !bytes.Contains(manifestBytes, []byte("router: "+strings.TrimPrefix(router, packRoot+"/"))) {
		t.Fatalf("项目入口未指向本包分发的 router: %v", err)
	}

	// router 的入口与成员任务路径在发布后的 case 里解析到真实文件。
	routerBytes := writeForTarget(t, preview, snapshot+router).Data
	for _, route := range routes {
		if !bytes.Contains(routerBytes, []byte(route)) {
			t.Fatalf("router 缺少入口 %s", route)
		}
		if info, err := os.Stat(filepath.Join(oldRoot, filepath.FromSlash(snapshot+packRoot+"/"+route))); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("router 入口在 case 中不存在: %s: %v", route, err)
		}
	}
	memberDir := filepath.Join(oldRoot, ".steamai-vnext", "members", fixture.facts.Members[0].Name)
	for _, read := range reads {
		if info, err := os.Stat(filepath.Join(memberDir, filepath.FromSlash(read))); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("成员任务路径在 case 中不存在: %s: %v", read, err)
		}
	}

	// 对照：没选本包的 case 不分发本包文件。
	control := buildRealRepoFreshFixture(t, "web-security",
		"../../pack-snapshot/packs/web-security/manifest.yml ../../pack-snapshot/packs/web-security/references/web-security/README.md")
	controlRoot := newCaseRoot(t)
	control.applyFresh(t, controlRoot)
	controlTree := treeDigest(t, controlRoot)
	for rel := range controlTree {
		if strings.HasPrefix(rel, snapshot+packRoot+"/") {
			t.Fatalf("没选本包的 case 分发了本包文件: %s", rel)
		}
	}

	// 已有 case：source 后续更新只进入之后的新 Fresh，旧 case 的身份与 bytes 不变。
	update := []byte("\n<!-- general-delivery-source-update -->\n")
	path := filepath.Join(fixture.source, filepath.FromSlash(router))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, append(append([]byte{}, data...), update...))
	// pack 文件属于 snapshot payload：更新应改变 source/payload 摘要与 preview identity，Revision 不变。
	updated, _ := fixture.applyFresh(t, newCaseRoot(t))
	if updated.SourceDigest == preview.SourceDigest || updated.Identity == preview.Identity || updated.Revision != preview.Revision || updated.SnapshotDigest == preview.SnapshotDigest {
		t.Fatal("unstaged pack update did not change the source/payload identity at unchanged revision")
	}
	if !bytes.Contains(writeForTarget(t, updated, snapshot+router).Data, update) {
		t.Fatal("new Fresh missed the working-tree pack update")
	}
	after, err := InspectCurrent(oldRoot)
	if err != nil || !reflect.DeepEqual(identity, after) || !sameTree(oldTree, treeDigest(t, oldRoot)) || !sameTree(controlTree, treeDigest(t, controlRoot)) {
		t.Fatalf("source update or later Fresh changed an existing case: %v", err)
	}
}
