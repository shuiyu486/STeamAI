package casebootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// upgradeHandoffSection 是新模板末尾的交接段正文（不含其前的空行）；旧模板没有这一段。
const upgradeHandoffSection = "## Commander 交接\n\n（由 Commander 随重要可见变化整体覆盖更新；接班以实际文件为准。）\n"

// upgradeTemplate 生成与真实 case 模板同构的合成模板：Case 边界与 roster 带占位符，
// 其余 H2 段是纯框架正文。
func upgradeTemplate(title, goalField, commander, factSources, charter, outputs, handoff string) string {
	return "# " + title + "\n\n## Case 边界\n\n- Case 名称：`{{CASE_NAME}}`\n- " + goalField + "：`{{GOAL}}`\n" +
		"- 授权范围：`{{AUTHORIZED_SCOPE}}`\n- 禁止事项：`{{PROHIBITED_ACTIONS}}`\n- 全局停止条件：`{{STOP_CONDITIONS}}`\n" +
		"- Selected pack：`{{PACK_NAME}}`\n- Source revision：`{{PACK_REVISION}}`\n- Pack tree：`{{PACK_SNAPSHOT_TREE}}`\n" +
		"{{AUX_PACK_IDENTITY}}- Common tree：`{{COMMON_SNAPSHOT_TREE}}`\n- Snapshot digest：`{{SNAPSHOT_DIGEST}}`\n\n" +
		"快照目录按 `{{PACK_NAME}}` 固定。\n\n## Commander\n\n" + commander + "\n\n## 当前团队\n\n" +
		"| Member | Kind | Durable state | Member source |\n|---|---|---|---|\n{{TEAM_ROSTER_ROWS}}\n\n" +
		"## 事实来源\n\n" + factSources + "\n\n## 协作章程\n\n" + charter + "\n\n## 共享研究产物\n\n" + outputs + "\n" + handoff
}

// upgradeTemplates 返回历史模板（旧标题、"研究目标"、无交接段、旧规则正文）与现行形态的新模板。
func upgradeTemplates() (string, string) {
	old := upgradeTemplate("STeamAI 安全研究 Case", "研究目标", "Commander 负责理解用户目标。",
		"- `durable`：旧事实来源。", "- 旧章程一。\n- 旧章程二。", "- `evidence/`：旧产物说明。", "")
	new := upgradeTemplate("STeamAI Case", "Case 目标", "Commander 负责目标、投入边界、分工与集成交付。",
		"- `durable`：新事实来源。", "- 新章程一。\n- 新章程二。\n- 新章程三。", "- `evidence/`：新产物说明。", "\n"+upgradeHandoffSection)
	return old, new
}

// upgradeSnapshot 用互不相同的十六进制身份构造快照元数据；index 区分新旧。
func upgradeSnapshot(index int, aux bool) snapshotMetadata {
	digit := func(offset, length int) string {
		at := (index + offset) % 16
		return strings.Repeat("0123456789abcdef"[at:at+1], length)
	}
	metadata := snapshotMetadata{
		Pack: "fixture-pack", Revision: digit(0, 40), PackTree: digit(1, 40), CommonTree: digit(2, 40),
		PayloadDigest: "sha256:" + digit(3, 64),
	}
	if aux {
		metadata.AuxPack, metadata.AuxPackTree = "aux-pack", digit(4, 40)
	}
	return metadata
}

func upgradeFacts(aux bool) Facts {
	facts := Facts{
		Name: "synthetic-case", Goal: "verify marker upgrade", Authorization: "temporary fixture files only",
		Prohibited: "network or real artifacts", Stop: "scope drift", Pack: "fixture-pack",
		Members: []MemberFacts{{Name: "analyst", Kind: "execution"}, {Name: "reviewer", Kind: "reviewer"}},
	}
	if aux {
		facts.AuxPack = "aux-pack"
	}
	return facts
}

func upgradeSource(metadata snapshotMetadata) frozenSource {
	return frozenSource{Revision: metadata.Revision, PackTree: metadata.PackTree, AuxPackTree: metadata.AuxPackTree, CommonTree: metadata.CommonTree}
}

// upgradeIdentityPairs 返回新旧快照各自独有、不会互相包含的身份值。
func upgradeIdentityPairs(from, to snapshotMetadata) [][2]string {
	pairs := [][2]string{{from.Revision, to.Revision}, {from.PackTree, to.PackTree}, {from.CommonTree, to.CommonTree}, {from.PayloadDigest, to.PayloadDigest}}
	if from.AuxPack != "" {
		pairs = append(pairs, [2]string{from.AuxPackTree, to.AuxPackTree})
	}
	return pairs
}

func TestUpgradeMarkerRewritesIdentityAndPristineRuleSections(t *testing.T) {
	oldTemplate, newTemplate := upgradeTemplates()
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	facts := upgradeFacts(true)
	live := []byte(renderCaseTemplate(oldTemplate, facts, upgradeSource(from), from.PayloadDigest))
	upgraded, err := upgradeMarker(live, []byte(oldTemplate), []byte(newTemplate), from, to)
	if err != nil {
		t.Fatal(err)
	}
	got := string(upgraded)
	// 未改动过的历史 case：规则段等于新模板 Fresh 的正文（含新模板中与交接段相隔的空行）；
	// 标题、旧目标字段名和"没有交接段"保持原样。
	want := renderCaseTemplate(newTemplate, facts, upgradeSource(to), to.PayloadDigest)
	want = strings.Replace(want, "# STeamAI Case\n", "# STeamAI 安全研究 Case\n", 1)
	want = strings.Replace(want, "- Case 目标：", "- 研究目标：", 1)
	want = strings.TrimSuffix(want, upgradeHandoffSection)
	if got != want {
		t.Fatalf("升级结果与预期不符:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if hasHandoffHeading(got) {
		t.Fatal("升级不应伪造交接段")
	}
}

func TestUpgradeMarkerKeepsCustomizedUnknownAndHandoffSections(t *testing.T) {
	oldTemplate, newTemplate := upgradeTemplates()
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	facts := upgradeFacts(true)
	live := renderCaseTemplate(oldTemplate, facts, upgradeSource(from), from.PayloadDigest)
	const commanderNote = "\n\n项目约定：先核对 evidence 再派工。"
	const notes = "## 项目备注\n\n- 内部约定：周五汇总。\n"
	// 交接段里既有同值的精确身份行，也有提到旧身份值的散文；两者都不在顶部元数据区，必须原样保留。
	handoff := "\n## Commander 交接\n\n- 进行中：analyst 正在核对。\n- Source revision：`" + from.Revision +
		"`\n- 已归档 Snapshot digest：`" + from.PayloadDigest + "`，Pack tree " + from.PackTree + "。\n"
	live = strings.Replace(live, "Commander 负责理解用户目标。", "Commander 负责理解用户目标。"+commanderNote, 1) + notes + handoff
	upgraded, err := upgradeMarker([]byte(live), []byte(oldTemplate), []byte(newTemplate), from, to)
	if err != nil {
		t.Fatal(err)
	}
	got := string(upgraded)
	// 用户改过的 Commander 段、未知段和交接段（含其中的旧身份值）逐字保留。
	for _, text := range []string{"Commander 负责理解用户目标。" + commanderNote, notes, handoff} {
		if !strings.Contains(got, text) {
			t.Fatalf("升级改写了应保留的段落: %s", text)
		}
	}
	// 仍是旧模板原文的段已更新，旧正文不再出现；用户改过的 Commander 段保留旧文，不换成新模板正文。
	for _, text := range []string{"- 新章程三。", "- `durable`：新事实来源。", "- `evidence/`：新产物说明。"} {
		if !strings.Contains(got, text) {
			t.Fatalf("未改动的规则段没有更新: %s", text)
		}
	}
	for _, text := range []string{"Commander 负责目标、投入边界、分工与集成交付。", "- 旧章程一。", "- `durable`：旧事实来源。", "- `evidence/`：旧产物说明。"} {
		if strings.Contains(got, text) {
			t.Fatalf("不应出现的正文残留: %s", text)
		}
	}
	for _, pair := range upgradeIdentityPairs(from, to) {
		if strings.Count(got, pair[1]) != 1 {
			t.Fatalf("身份行 %s 未唯一换成新值", pair[1][:20])
		}
	}
	if strings.Count(got, from.Revision) != 1 || strings.Count(got, from.PayloadDigest) != 1 || strings.Count(got, from.PackTree) != 1 {
		t.Fatal("同值文本应只保留在交接段原文里")
	}
}

// 交接段第一个标题之后不再选择任何规则段，哪怕其内子标题与白名单段同名且正文仍是旧模板原文。
func TestUpgradeMarkerDoesNotSelectSectionsAfterHandoffHeading(t *testing.T) {
	oldTemplate, newTemplate := upgradeTemplates()
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	facts := upgradeFacts(true)
	const outputs = "## 共享研究产物\n\n- `evidence/`：旧产物说明。\n"
	live := renderCaseTemplate(oldTemplate, facts, upgradeSource(from), from.PayloadDigest)
	// 主 共享研究产物 段被删掉，唯一一份原文现在位于交接段内的同名子标题下。
	handoff := "## Commander 交接\n\n- 进行中：analyst 正在核对。\n\n" + outputs
	live = strings.Replace(live, outputs, "", 1) + handoff
	upgraded, err := upgradeMarker([]byte(live), []byte(oldTemplate), []byte(newTemplate), from, to)
	if err != nil {
		t.Fatal(err)
	}
	got := string(upgraded)
	if !strings.Contains(got, handoff) || strings.Contains(got, "新产物说明") {
		t.Fatal("交接段内的子标题被当成可更新的规则段")
	}
	if !strings.Contains(got, "- 新章程三。") {
		t.Fatal("交接之前的白名单段应照常更新")
	}
}

// 新旧模板正文相同的段保留 live 原字节，避免混合换行在同一版本上被改写。
func TestUpgradeMarkerKeepsOriginalBytesWhenSectionUnchanged(t *testing.T) {
	oldTemplate, newTemplate := upgradeTemplates()
	newTemplate = strings.Replace(newTemplate, "- 新章程一。\n- 新章程二。\n- 新章程三。", "- 旧章程一。\n- 旧章程二。", 1)
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	facts := upgradeFacts(true)
	live := strings.Replace(renderCaseTemplate(oldTemplate, facts, upgradeSource(from), from.PayloadDigest), "- 旧章程一。\n", "- 旧章程一。\r\n", 1)
	upgraded, err := upgradeMarker([]byte(live), []byte(oldTemplate), []byte(newTemplate), from, to)
	if err != nil {
		t.Fatal(err)
	}
	got := string(upgraded)
	if !strings.Contains(got, "## 协作章程\n\n- 旧章程一。\r\n- 旧章程二。\n") {
		t.Fatal("正文未变的段应保留原字节")
	}
	if !strings.Contains(got, "Commander 负责目标、投入边界、分工与集成交付。") {
		t.Fatal("其它段应照常更新")
	}
}

func TestUpgradeMarkerPreservesLiveLineEndings(t *testing.T) {
	oldTemplate, newTemplate := upgradeTemplates()
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	facts := upgradeFacts(true)
	live := renderCaseTemplate(oldTemplate, facts, upgradeSource(from), from.PayloadDigest)
	crlf := func(text string) string { return strings.ReplaceAll(text, "\n", "\r\n") }
	identityLine := "- Snapshot digest：`" + to.PayloadDigest + "`"
	var baseline string
	for _, variant := range []struct {
		name     string
		oldCRLF  bool
		liveCRLF bool
		newCRLF  bool
	}{
		{name: "LF"},
		{name: "CRLF", oldCRLF: true, liveCRLF: true, newCRLF: true},
		{name: "旧模板 CRLF 项目 LF", oldCRLF: true},
		{name: "项目 CRLF 模板 LF", liveCRLF: true},
		{name: "新模板 CRLF", newCRLF: true},
	} {
		t.Run(variant.name, func(t *testing.T) {
			oldText, liveText, newText := oldTemplate, live, newTemplate
			if variant.oldCRLF {
				oldText = crlf(oldText)
			}
			if variant.liveCRLF {
				liveText = crlf(liveText)
			}
			if variant.newCRLF {
				newText = crlf(newText)
			}
			upgraded, err := upgradeMarker([]byte(liveText), []byte(oldText), []byte(newText), from, to)
			if err != nil {
				t.Fatal(err)
			}
			got := string(upgraded)
			if normalized := markerLF(got); baseline == "" {
				baseline = normalized
			} else if normalized != baseline {
				t.Fatalf("换行风格改变了升级内容:\n%s", normalized)
			}
			bare := strings.ReplaceAll(got, "\r\n", "")
			if variant.liveCRLF {
				if strings.ContainsAny(bare, "\r\n") || !strings.Contains(got, identityLine+"\r\n") {
					t.Fatal("CRLF 项目升级后出现非 CRLF 换行")
				}
			} else if strings.Contains(got, "\r") || !strings.Contains(got, identityLine+"\n") {
				t.Fatal("LF 项目升级后出现 CR")
			}
		})
	}
}

func TestUpgradeMarkerWithoutOldTemplateOnlyRewritesIdentity(t *testing.T) {
	oldTemplate, newTemplate := upgradeTemplates()
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	facts := upgradeFacts(true)
	live := renderCaseTemplate(oldTemplate, facts, upgradeSource(from), from.PayloadDigest)
	upgraded, err := upgradeMarker([]byte(live), nil, []byte(newTemplate), from, to)
	if err != nil {
		t.Fatal(err)
	}
	if string(upgraded) != replaceUpgradeIdentity(live, from, to) {
		t.Fatal("没有旧模板时应只改身份行，不猜测规则段")
	}
}

// 最早的 case marker 没有 H2 段：身份行只认 roster 表头之前的文首部分。
func TestUpgradeMarkerHistoricalMarkerWithoutBoundarySection(t *testing.T) {
	_, newTemplate := upgradeTemplates()
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	head := "# Old synthetic case\n- Case 名称：`synthetic-case`\n- 研究目标：`old fixture`\n- 授权范围：`local fixture`\n- 禁止事项：`network`\n" +
		"- 全局停止条件：`scope drift`\n- Selected pack：`fixture-pack`\n- Source revision：`" + from.Revision + "`\n- Pack tree：`" + from.PackTree + "`\n" +
		"- Auxiliary pack：`aux-pack`\n- Auxiliary pack tree：`" + from.AuxPackTree + "`\n- Common tree：`" + from.CommonTree + "`\n- Snapshot digest：`" + from.PayloadDigest + "`\n\n"
	roster := "| Member | Kind | Durable state | Member source |\n|---|---|---|---|\n| none | execution | inactive | none |\n"
	handoff := "\n## Commander 交接\n\n- Snapshot digest：`" + from.PayloadDigest + "`\n"
	for _, tc := range []struct {
		name, tail string
	}{
		{"无段首部", ""},
		{"交接里的同值行不动", handoff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upgraded, err := upgradeMarker([]byte(head+roster+tc.tail), nil, []byte(newTemplate), from, to)
			if err != nil {
				t.Fatal(err)
			}
			if want := replaceUpgradeIdentity(head+roster, from, to) + tc.tail; string(upgraded) != want {
				t.Fatalf("历史 marker 升级结果与预期不符:\n%s", upgraded)
			}
		})
	}
}

// replaceUpgradeIdentity 把各身份值第一次出现换成新值，用于构造"只改身份行"的预期结果。
func replaceUpgradeIdentity(text string, from, to snapshotMetadata) string {
	for _, pair := range upgradeIdentityPairs(from, to) {
		text = strings.Replace(text, pair[0], pair[1], 1)
	}
	return text
}

// hasHandoffHeading 只认交接段的标题行；正文里提到"Commander 交接"不算。
func hasHandoffHeading(text string) bool {
	return slices.ContainsFunc(markerSections(splitMarkerLines(text)), func(section markerSection) bool {
		return section.heading == commanderHandoffHeading
	})
}

func TestUpgradeMarkerRejectsAmbiguousOrMismatchedInput(t *testing.T) {
	oldTemplate, newTemplate := upgradeTemplates()
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	facts := upgradeFacts(true)
	live := renderCaseTemplate(oldTemplate, facts, upgradeSource(from), from.PayloadDigest)
	revisionLine := "- Source revision：`" + from.Revision + "`\n"
	singleFrom, auxAdded := upgradeSnapshot(0, false), upgradeSnapshot(5, true)
	otherPack := to
	otherPack.Pack = "other-pack"
	for _, tc := range []struct {
		name                      string
		live, oldTemplate, newTpl []byte
		from, to                  snapshotMetadata
	}{
		// 校验器读全文件会接受只在交接段里的同名行；升级不得改交接，必须拒绝。
		{"身份行只在交接段", []byte(strings.Replace(live, revisionLine, "", 1) + "\n## Commander 交接\n\n" + revisionLine), []byte(oldTemplate), []byte(newTemplate), from, to},
		{"Case 边界段重复", []byte(live + "\n## Case 边界\n\n（归档副本）\n"), []byte(oldTemplate), []byte(newTemplate), from, to},
		{"更换主 pack", []byte(live), []byte(oldTemplate), []byte(newTemplate), from, otherPack},
		{"新增辅助 pack", []byte(renderCaseTemplate(oldTemplate, upgradeFacts(false), upgradeSource(singleFrom), singleFrom.PayloadDigest)), []byte(oldTemplate), []byte(newTemplate), singleFrom, auxAdded},
		{"旧 marker 与旧快照不符", []byte(strings.Replace(live, from.Revision, strings.Repeat("e", 40), 1)), []byte(oldTemplate), []byte(newTemplate), from, to},
		{"缺少新模板", []byte(live), []byte(oldTemplate), nil, from, to},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upgraded, err := upgradeMarker(tc.live, tc.oldTemplate, tc.newTpl, tc.from, tc.to)
			if err == nil || upgraded != nil {
				t.Fatalf("应拒绝: %v", err)
			}
			for _, fact := range []string{facts.Name, facts.Goal, facts.Authorization, facts.Prohibited, from.Revision, from.PayloadDigest, to.Revision, "Old synthetic"} {
				if strings.Contains(err.Error(), fact) {
					t.Fatalf("错误信息带出项目事实: %v", err)
				}
			}
		})
	}
}

// 仓库真实模板的规则段必须能被识别并更新；带占位符的 Case 边界/roster 与交接段不受影响。
func TestUpgradeMarkerRecognizesRepositoryCaseTemplateSections(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "vnext", "templates", "case", "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	newLines := splitMarkerLines(markerLF(string(data)))
	newSections := markerSections(newLines)
	if len(newSections) == 0 {
		t.Fatal("仓库模板没有 H2 段")
	}
	// 旧模板：真实模板去掉交接段，并把白名单段的正文换成旧文本。
	var old strings.Builder
	var ruleSections []string
	writeMarkerLines(&old, newLines[:newSections[0].start])
	for _, section := range newSections {
		if section.heading == commanderHandoffHeading {
			continue
		}
		writeMarkerLines(&old, newLines[section.start:section.start+1])
		if !slices.Contains(upgradableMarkerSections, section.heading) || strings.Contains(markerSectionText(newLines, section), "{{") {
			writeMarkerLines(&old, newLines[section.start+1:section.end])
			continue
		}
		fmt.Fprintf(&old, "\n旧 %s 正文。\n\n", section.heading)
		ruleSections = append(ruleSections, section.heading)
	}
	if !slices.Contains(ruleSections, "Commander") || !slices.Contains(ruleSections, "协作章程") {
		t.Fatalf("仓库模板中 Commander/协作章程 不再是无占位符规则段: %v", ruleSections)
	}
	from, to := upgradeSnapshot(0, true), upgradeSnapshot(5, true)
	facts := upgradeFacts(true)
	oldTemplate := old.String()
	live := renderCaseTemplate(oldTemplate, facts, upgradeSource(from), from.PayloadDigest)
	upgraded, err := upgradeMarker([]byte(live), []byte(oldTemplate), data, from, to)
	if err != nil {
		t.Fatal(err)
	}
	got := string(upgraded)
	liveLines, gotLines := splitMarkerLines(live), splitMarkerLines(got)
	if hasHandoffHeading(got) {
		t.Fatal("升级不应给旧 case 补交接段")
	}
	for _, section := range newSections {
		if section.heading == commanderHandoffHeading {
			continue
		}
		want := markerSectionText(newLines, section)
		if !slices.Contains(ruleSections, section.heading) {
			// 其余段是项目事实、roster 或白名单之外的正文：除顶部元数据区的身份行外必须与升级前逐字一致。
			before, ok := uniqueMarkerSection(markerSections(liveLines), section.heading)
			if !ok {
				t.Fatalf("升级前的 marker 缺少唯一的 %s 段", section.heading)
			}
			want = replaceUpgradeIdentity(markerSectionText(liveLines, before), from, to)
		}
		after, ok := uniqueMarkerSection(markerSections(gotLines), section.heading)
		if !ok || markerLF(markerSectionText(gotLines, after)) != markerLF(want) {
			t.Fatalf("段 %s 升级结果不符", section.heading)
		}
	}
}
