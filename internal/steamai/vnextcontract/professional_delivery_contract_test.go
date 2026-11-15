package vnextcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestProfessionalDeliveryDualTrackContract 验证 DLV-01 研究/开发双交付合同：
// 同一套七字段与产物目录表达两类任务，代码路径分配有界，开发 accepted 不升级
// maturity，pack 工具按用途接纳且不自动安装。
func TestProfessionalDeliveryDualTrackContract(t *testing.T) {
	repo := repoRoot(t)

	caseTemplate := readPrototypeFile(t, repo, "vnext/templates/case/CLAUDE.md")
	for _, required := range []string{
		"明确授权的安全研究或自动化开发 case",
		"不为开发任务伪造安全 finding",
		"源码留在项目原位置",
		"复用同一套 `artifacts/`",
	} {
		assertContains(t, caseTemplate, required, "case dual-track contract")
	}

	memberTemplate := readPrototypeFile(t, repo, "vnext/templates/member/CLAUDE.md")
	for _, required := range []string{
		"case-relative 项目源码路径",
		"同一源码文件不得由多名成员并发写入",
		"整合他人改动前先核对当前基线与用户手工修改",
	} {
		assertContains(t, memberTemplate, required, "member code-path contract")
	}

	reviewerRole := readPrototypeFile(t, repo, "vnext/templates/roles/reviewer.md")
	for _, required := range []string{
		"明确分配给该审查的项目源码、exact diff 与测试回执",
		"开发交付审查绑定所审源码版本或 exact diff",
		"不自动取得 V0–V4 maturity 或 learning 资格",
		"独立运行测试由获准 verifier 承担",
	} {
		assertContains(t, reviewerRole, required, "reviewer dual-track contract")
	}

	analysisRole := readPrototypeFile(t, repo, "vnext/templates/roles/analysis-member.md")
	for _, required := range []string{
		"开发类任务在明确分配的项目源码路径内交付代码、补丁、测试观察与使用说明",
		"测试回执按 evidence 记录",
		"不为开发任务伪造安全 finding",
	} {
		assertContains(t, analysisRole, required, "analysis-member delivery contract")
	}

	skill := readPrototypeFile(t, repo, ".claude/skills/steamai/SKILL.md")
	for _, required := range []string{
		"安全研究或自动化开发 case",
		"开发类任务把要写的 case-relative 项目源码范围列入成员 `allowedWrites`",
		"绑定所审源码版本或 exact diff 与相关测试证据",
		"只表示此次代码交付经审查，不自动取得 V0–V4 maturity 或 learning 资格",
	} {
		assertContains(t, skill, required, "canonical dual-track behavior")
	}

	capabilities := readPrototypeFile(t, repo, "vnext/capabilities.md")
	for _, required := range []string{
		"非 MCP 形态的专业工具",
		"pack 工具按具体用途接纳",
		"先证明上游/native 能力不足",
		"继续禁止通用运行器、adapter host",
		"工具脚本不进入 learningTargets",
	} {
		assertContains(t, capabilities, required, "capabilities tool-admission contract")
	}

	rootClaude := readPrototypeFile(t, repo, "CLAUDE.md")
	for _, required := range []string{
		"面向已授权安全研究与自动化开发",
		"pack 工具按具体用途接纳",
		"继续禁止通用运行器与 adapter host",
	} {
		assertContains(t, rootClaude, required, "root CLAUDE.md positioning")
	}

	readme := readPrototypeFile(t, repo, "README.md")
	assertContains(t, readme, "已授权安全研究与自动化开发", "README.md positioning")

	acceptance := readPrototypeFile(t, repo, "vnext/acceptance.md")
	for _, required := range []string{
		"steamai-professional-delivery-v1",
		"研究/开发双交付合同",
		"不自动取得 V0–V4 或 learning 资格",
		"不安装、不注册、不调用 Playwright/Airtest/Strix/REA",
		"fake adapter",
	} {
		assertContains(t, acceptance, required, "acceptance professional-delivery gates")
	}
}

func TestGameAutomationDMAndAirtestKeepConditionalRouting(t *testing.T) {
	repo := repoRoot(t)
	const pack = "packs/game-automation/"
	manifest := readPrototypeFile(t, repo, pack+"manifest.yml")
	tooling := manifestListValues(manifest, "tooling")
	for _, integration := range []string{"tooling/integrations/dm.md", "tooling/integrations/airtest.md"} {
		if !slices.Contains(tooling, integration) {
			t.Errorf("game-automation 未声明工具入口 %s", integration)
		}
		assertRegularNonSymlink(t, filepath.Join(repo, pack, integration), "游戏工具接入")
		for _, target := range manifestListValues(manifest, "learningTargets") {
			matched, err := filepath.Match(filepath.FromSlash(target), filepath.FromSlash(integration))
			if err != nil || matched {
				t.Errorf("learning target %q 不得覆盖接入材料 %s: %v", target, integration, err)
			}
		}
	}
	for _, rel := range []string{
		".claude/skills/steamai/SKILL.md", pack + "CLAUDE.local.snippet.md",
		pack + "references/game-automation/README.md", pack + "references/game-automation/toolchain-router.md",
		pack + "references/game-automation/workflow-template.md", pack + "tooling/README.md", pack + "tooling/catalog.yml",
	} {
		text := readPrototypeFile(t, repo, rel)
		assertContains(t, text, "integrations/dm.md", rel)
		assertContains(t, text, "airtest.md", rel)
	}
	for rel, required := range map[string][]string{
		".claude/skills/steamai/SKILL.md":                        {"大漠与 Airtest 并存", "用户明确选择或已有大漠资产", "不默认叠加两套输入通道"},
		pack + "references/game-automation/toolchain-router.md":  {"大漠与 Airtest 并存", "未指定工具且无既有资产", "不因新增大漠放弃 Airtest"},
		pack + "tooling/integrations/dm.md":                      {"7.2520", "不因存在 7.2642", "PE 位数", "对应版本的本地 CHM", "8954106a69c0bd5cf17cfd72409982226e721186", "functions_index.json", "空模板", "不要求安装 `dm-api`", "不自动启用或覆盖用户插件", "注册码", "不进入 canonical pack", "32 位 `dm.dll` 不能直接加载到 64 位 MFC 进程", "同一窗口串行输入", "未经本次明确批准不做", "防护/驱动参数", "不把原有 Airtest 验收当成大漠验收", "真实目标需另行确认", "不提供通用 DLL 调用器"},
		pack + "references/game-automation/README.md":            {"后台模式只有", "不自动回退前台输入"},
		pack + "references/game-automation/workflow-template.md": {"选定工具", "不默认预检两套工具", "C++/MFC", "不要求 Python"},
		pack + "tooling/recipes/windows-stateful-automation.md":  {"字库", "不因窗口自绘就强制改用 LLM", "大漠 C++/MFC", "绑定失效即停止且不回退前台输入"},
		pack + "tooling/integrations/airtest.md":                 {"airtest==1.4.3", "不因新增大漠而停用 Airtest"},
		pack + "tooling/catalog.yml":                             {"id: dm-window-automation", "id: airtest-desktop-automation", "id: windows-flow-reference", "status: candidate"},
	} {
		text := readPrototypeFile(t, repo, rel)
		for _, phrase := range required {
			assertContains(t, text, phrase, rel)
		}
	}
	for _, rel := range []string{
		".claude/skills/steamai/SKILL.md", pack + "references/game-automation/toolchain-router.md",
		pack + "references/game-automation/workflow-template.md", pack + "tooling/integrations/airtest.md",
	} {
		text := readPrototypeFile(t, repo, rel)
		for _, forced := range []string{
			"Windows 游戏自动化用 game 主包 `tooling/integrations/airtest.md`",
			"Windows 桌面游戏自动化脚本开发从任务开始使用固定版本的 Airtest",
			"**真实窗口冒烟**：Airtest 绑定目标",
		} {
			if strings.Contains(text, forced) {
				t.Errorf("%s 仍强制所有游戏任务使用 Airtest: %s", rel, forced)
			}
		}
	}
	assertContains(t, readPrototypeFile(t, repo, pack+"tooling/examples/windows-flow/requirements.txt"), "airtest==1.4.3", "Airtest 示例版本")
}

func TestGameAutomationDMOfflineReferencesArePinnedAndUsable(t *testing.T) {
	repo := repoRoot(t)
	const base = "packs/game-automation/upstream/dm-7.2520/"
	assertContains(t, readPrototypeFile(t, repo, base+"SOURCE.md"), "8c041a1a03e300427c335e7b5dfc0458a44cb4283aaf95ec9c21ce73b2d27427", "原有大漠 CHM 来源不漂移")
	versions, err := filepath.Glob(filepath.Join(repo, "packs/game-automation/upstream/dm-*"))
	if err != nil || len(versions) == 0 {
		t.Fatalf("缺少大漠版本资料: %v", err)
	}
	for _, version := range versions {
		t.Run(filepath.Base(version), func(t *testing.T) {
			testDMReferenceVersion(t, repo, "packs/game-automation/upstream/"+filepath.Base(version)+"/")
		})
	}
	for page, required := range map[string][]string{
		"README.md":          {"无需安装", "42 个常用接口", "FindStrFast", "cpp-mfc.md", "CmpColor"},
		"threading.md":       {"STA和MTA", "建议使用MTA", "每个线程开头", "主线程绑定结束"},
		"EnableShareDict.md": {"主对象不可以被释放", "任何对象都不可以修改字库"},
		"FindStr.md":         {"返回字符串的索引 没找到返回-1"},
		"FindStrFast.md":     {"和\"这个字符没有列入查找计划"},
		"FindPic.md":         {"从0开始索引.如果没找到返回-1"},
		"CmpColor.md":        {"0: 颜色匹配", "1: 颜色不匹配"},
		"GetLastError.md":    {"必须紧跟上一句函数调用"},
		"BindWindow.md":      {"窗口的客户区坐标", "不是完整模式表", "OnScriptExit", "[UnBindWindow](UnBindWindow.md)"},
		"BindWindowEx.md":    {"OnScriptExit", "[UnBindWindow](UnBindWindow.md)"},
		"cpp-mfc.md":         {"COINIT_MULTITHREADED", "同窗口输入仍应串行", "COM 调用 HRESULT", "不从 VBS 原型臆造", "tooling/examples/dm-cpp-mfc/"},
	} {
		text := readPrototypeFile(t, repo, base+page)
		for _, phrase := range required {
			assertContains(t, text, phrase, base+page)
		}
	}
}

func testDMReferenceVersion(t *testing.T, repo, base string) {
	t.Helper()
	source := readPrototypeFile(t, repo, base+"SOURCE.md")
	rows := regexp.MustCompile("(?m)^\\| `([^`]+\\.md)` \\| `([^`]+\\.htm)` \\| `([0-9a-f]{64})` \\| ([0-9]+) \\| `([0-9a-f]{64})` \\|$").FindAllStringSubmatch(source, -1)
	if len(rows) == 0 {
		t.Fatal("大漠版本缺少逐页来源/输出校验表")
	}
	rowsByOutput := map[string][]string{}
	for _, row := range rows {
		rowsByOutput[row[1]] = row
	}
	if data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(base+"selection.json"))); err == nil {
		checkDMSelectionJSON(t, data, base, source, rowsByOutput)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	// 固定清单 tooling/scripts/dm_docs.selection.json 固定当前版本的选择
	// （见 tooling/dm-docs-update.md）：标签与本版本一致时同样逐项核对。
	fixed, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash("packs/game-automation/tooling/scripts/dm_docs.selection.json")))
	if err != nil {
		t.Fatal(err)
	}
	var fixedLabel struct {
		SdkLabel string `json:"sdk_label"`
	}
	if json.Unmarshal(fixed, &fixedLabel) == nil &&
		"dm-"+fixedLabel.SdkLabel == filepath.Base(strings.TrimSuffix(filepath.ToSlash(base), "/")) {
		checkDMSelectionJSON(t, fixed, base, source, rowsByOutput)
	}
	covered := map[string]bool{"SOURCE.md": true}
	for _, name := range []string{"README.md", "cpp-mfc.md", "color-format.md", "selection.json"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(base+name))); err == nil {
			covered[name] = true
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	for _, row := range rows {
		if filepath.Base(row[1]) != row[1] || covered[row[1]] {
			t.Fatalf("重复或越界资料路径: %s", row[1])
		}
		covered[row[1]] = true
		data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(base+row[1])))
		if err != nil {
			t.Fatal(err)
		}
		wantBytes, err := strconv.Atoi(row[4])
		sum := sha256.Sum256(data)
		if err != nil || len(data) != wantBytes || hex.EncodeToString(sum[:]) != row[5] {
			t.Errorf("固定节选 %s 与 SOURCE.md bytes/SHA-256 不符", row[1])
		}
		assertContains(t, string(data), row[2], row[1]+" 原页来源")
		if row[1][0] >= 'A' && row[1][0] <= 'Z' {
			assertContains(t, string(data), "## 原型", row[1])
			assertContains(t, string(data), "## 返回值", row[1])
		}
	}
	manifest := readPrototypeFile(t, repo, "packs/game-automation/manifest.yml")
	for _, entry := range []string{"README.md", "SOURCE.md", "cpp-mfc.md", "color-format.md"} {
		if covered[entry] && !slices.Contains(manifestListValues(manifest, "tooling"), strings.TrimPrefix(base, "packs/game-automation/")+entry) {
			t.Errorf("manifest 未声明大漠离线入口 %s%s", base, entry)
		}
	}
	files, err := os.ReadDir(filepath.Join(repo, filepath.FromSlash(base)))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(covered) {
		t.Errorf("固定资料目录含未登记或缺少的文件: got %d want %d", len(files), len(covered))
	}
	linkRE := regexp.MustCompile(`\]\(([^)#]+\.md)(?:#[^)]*)?\)`)
	for _, entry := range files {
		rel := base + entry.Name()
		assertRegularNonSymlink(t, filepath.Join(repo, filepath.FromSlash(rel)), "离线只读资料")
		if entry.IsDir() || !covered[entry.Name()] {
			t.Errorf("大漠固定资料含未登记内容: %s", rel)
			continue
		}
		text := readPrototypeFile(t, repo, rel)
		if (filepath.Ext(entry.Name()) == ".md" && len(text) > 16*1024) || !utf8.ValidString(text) || strings.ContainsRune(text, '�') {
			t.Errorf("大漠资料超预算或编码损坏: %s", rel)
		}
		for _, target := range manifestListValues(manifest, "learningTargets") {
			match, err := filepath.Match(filepath.FromSlash(target), filepath.FromSlash(strings.TrimPrefix(base, "packs/game-automation/")+entry.Name()))
			if err != nil || match {
				t.Errorf("learning target %s 覆盖固定资料 %s", target, rel)
			}
		}
		for _, link := range linkRE.FindAllStringSubmatch(text, -1) {
			if !covered[link[1]] {
				t.Errorf("离线路由 %s 指向缺失或越界页面 %s", rel, link[1])
			}
		}
	}
}

// checkDMSelectionJSON 对 selection.json 做与 tooling/scripts/dm_docs.py
// 生成结构一致的基本校验：顶层 schema/标签/CHM 指纹字段齐全，逐页
// output/source/hash 与 SOURCE.md 表行一致且全部已审。页数不设固定值，
// 未来版本增删页面不需要改这里。
func checkDMSelectionJSON(t *testing.T, data []byte, base, source string, rows map[string][]string) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("selection.json 不是合法 JSON: %v", err)
	}
	if len(top) != 5 {
		t.Fatalf("selection.json 顶层字段数不符 dm_docs 生成结构: %d", len(top))
	}
	for _, field := range []string{"schema", "sdk_label", "chm_sha256", "chm_bytes", "pages"} {
		if _, ok := top[field]; !ok {
			t.Fatalf("selection.json 缺少顶层字段 %s", field)
		}
	}
	var schema int
	if err := json.Unmarshal(top["schema"], &schema); err != nil || schema != 1 {
		t.Fatalf("selection.json schema 不受支持: %s", top["schema"])
	}
	var label string
	if err := json.Unmarshal(top["sdk_label"], &label); err != nil || label == "" ||
		"dm-"+label != filepath.Base(strings.TrimSuffix(filepath.ToSlash(base), "/")) {
		t.Fatalf("selection.json sdk_label 与版本目录不符: %s", top["sdk_label"])
	}
	var chm struct {
		SHA256 string `json:"chm_sha256"`
		Bytes  int    `json:"chm_bytes"`
	}
	if err := json.Unmarshal(data, &chm); err != nil || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(chm.SHA256) || chm.Bytes <= 0 {
		t.Fatalf("selection.json CHM 指纹字段无效: %s", top["chm_sha256"])
	}
	assertContains(t, source, chm.SHA256, base+"SOURCE.md")
	assertContains(t, source, strconv.Itoa(chm.Bytes), base+"SOURCE.md")
	var pages []json.RawMessage
	if err := json.Unmarshal(top["pages"], &pages); err != nil || len(pages) == 0 || len(pages) != len(rows) {
		t.Fatalf("selection.json 页数 %d 与 SOURCE.md 表行 %d 不一致", len(pages), len(rows))
	}
	seen := map[string]bool{}
	for _, raw := range pages {
		var page map[string]json.RawMessage
		if err := json.Unmarshal(raw, &page); err != nil || len(page) != 5 {
			t.Fatalf("selection.json 页字段数不符 dm_docs 生成结构: %s", raw)
		}
		for _, field := range []string{"output", "source", "source_sha256", "output_sha256", "review"} {
			if _, ok := page[field]; !ok {
				t.Fatalf("selection.json 页缺少字段 %s: %s", field, raw)
			}
		}
		var entry struct {
			Output    string
			Source    string
			SourceSHA string `json:"source_sha256"`
			OutputSHA string `json:"output_sha256"`
			Review    string
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			t.Fatal(err)
		}
		row, ok := rows[entry.Output]
		if !ok {
			t.Errorf("selection.json 页 %s 未出现在 SOURCE.md 表中", entry.Output)
			continue
		}
		seen[entry.Output] = true
		hashRE := regexp.MustCompile(`^[0-9a-f]{64}$`)
		if entry.Source != row[2] || entry.SourceSHA != row[3] || entry.OutputSHA != row[5] ||
			!hashRE.MatchString(entry.SourceSHA) || !hashRE.MatchString(entry.OutputSHA) {
			t.Errorf("selection.json 页 %s 与 SOURCE.md 行不一致", entry.Output)
		}
		if entry.Review != "reviewed" {
			t.Errorf("selection.json 页 %s 不是已审状态: %s", entry.Output, entry.Review)
		}
	}
	if len(seen) != len(rows) {
		t.Fatalf("selection.json 页集合与 SOURCE.md 表行不一致: got %d want %d", len(seen), len(rows))
	}
}

// TestGameAutomationDMCppMFCProjectAndDocsToolStayAdmitted 验证大漠 C++/MFC
// 基础工程与 dm_docs 资料工具按真实文件接纳：manifest/catalog 声明的入口
// 是实际存在的普通文件，工程闭包携带真实构建/源码/测试文件且不含构建
// 产物，learning 边界不覆盖。此处只校验交付接线，不以字符串 oracle 冒充
// 实际 C++/MFC 测试（工程自身的构建与合成测试由 pack 侧验收）。
func TestGameAutomationDMCppMFCProjectAndDocsToolStayAdmitted(t *testing.T) {
	repo := repoRoot(t)
	const pack = "packs/game-automation/"
	manifest := readPrototypeFile(t, repo, pack+"manifest.yml")
	tooling := manifestListValues(manifest, "tooling")
	for _, entry := range []string{
		"tooling/dm-docs-update.md",
		"tooling/scripts/dm_docs.py",
		"tooling/scripts/dm_docs.selection.json",
		"tooling/examples/dm-cpp-mfc/README.md",
		"tooling/examples/dm-cpp-mfc/CMakeLists.txt",
	} {
		if !slices.Contains(tooling, entry) {
			t.Errorf("manifest 未声明大漠工程/资料工具入口 %s", entry)
		}
		assertRegularNonSymlink(t, filepath.Join(repo, pack, entry), "大漠工程/资料工具入口")
	}
	catalog := readPrototypeFile(t, repo, pack+"tooling/catalog.yml")
	for _, anchor := range []string{
		"id: dm-document-update", "id: dm-cpp-mfc-starter",
		"tooling/dm-docs-update.md", "tooling/examples/dm-cpp-mfc/",
	} {
		assertContains(t, catalog, anchor, pack+"tooling/catalog.yml")
	}

	files := map[string]bool{pack + "tooling/dm-docs-update.md": true}
	for _, root := range []string{pack + "tooling/examples/dm-cpp-mfc", pack + "tooling/scripts"} {
		err := filepath.WalkDir(filepath.Join(repo, filepath.FromSlash(root)), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(repo, path)
			if err != nil {
				return err
			}
			name := entry.Name()
			if entry.IsDir() {
				if slices.Contains([]string{"__pycache__", "build", "CMakeFiles", "dist"}, name) {
					t.Errorf("大漠工程/工具目录含构建产物目录: %s", filepath.ToSlash(rel))
				}
				return nil
			}
			if slices.Contains([]string{"CMakeCache.txt", "cmake_install.cmake"}, name) || strings.HasSuffix(name, ".pyc") {
				t.Errorf("大漠工程/工具目录含构建产物: %s", filepath.ToSlash(rel))
				return nil
			}
			assertRegularNonSymlink(t, path, "大漠工程/工具文件")
			files[filepath.ToSlash(rel)] = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, required := range []string{
		pack + "tooling/examples/dm-cpp-mfc/CMakeLists.txt",
		pack + "tooling/examples/dm-cpp-mfc/src/mfc_main.cpp",
		pack + "tooling/examples/dm-cpp-mfc/src/worker_main.cpp",
		pack + "tooling/examples/dm-cpp-mfc/tests/synthetic_dm.cpp",
		pack + "tooling/examples/dm-cpp-mfc/tests/test_worker.py",
		pack + "tooling/scripts/dm_docs.py",
		pack + "tooling/scripts/dm_docs.selection.json",
	} {
		if !files[required] {
			t.Errorf("大漠 C++/MFC 工程缺少实际构建/源码/测试文件: %s", required)
		}
	}
	for rel := range files {
		for _, target := range manifestListValues(manifest, "learningTargets") {
			matched, err := filepath.Match(filepath.FromSlash(target), filepath.FromSlash(rel))
			if err != nil || matched {
				t.Errorf("learning target %q 不得覆盖大漠工程/工具文件 %s: %v", target, rel, err)
			}
		}
	}
}

// TestMemberTemplateKeepsCanonicalSevenFieldSet 确认双交付合同没有为开发任务
// 新增必填字段：member 模板的 placeholder 集合仍是既有 11 项。
func TestMemberTemplateKeepsCanonicalSevenFieldSet(t *testing.T) {
	repo := repoRoot(t)
	memberTemplate := readPrototypeFile(t, repo, "vnext/templates/member/CLAUDE.md")
	re := regexp.MustCompile(`\{\{[A-Z_0-9]+\}\}`)
	got := map[string]bool{}
	for _, m := range re.FindAllString(memberTemplate, -1) {
		got[m] = true
	}
	want := []string{
		"{{MEMBER_NAME}}", "{{ROLE}}", "{{RESPONSIBILITY}}",
		"{{TASK_GOAL}}", "{{INPUTS}}", "{{ALLOWED_READS}}", "{{ALLOWED_WRITES}}",
		"{{DELIVERABLES}}", "{{STOP_OR_ESCALATE}}", "{{EXIT_CONDITIONS}}",
		"{{ROLE_SPECIFIC_RULES}}",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("member template lost canonical placeholder %s", w)
		}
		delete(got, w)
	}
	for extra := range got {
		t.Errorf("member template introduced non-canonical placeholder %s", extra)
	}
}
