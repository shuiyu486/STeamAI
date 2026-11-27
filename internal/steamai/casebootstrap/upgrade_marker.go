package casebootstrap

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

const (
	// commanderHandoffHeading 是 Commander 单写的交接段；显式更新绝不改写其中任何内容，
	// 也不会给没有交接段的旧 case 补一段。
	commanderHandoffHeading = "Commander 交接"
	// caseBoundaryHeading 是身份行所在的顶部元数据段。
	caseBoundaryHeading = "Case 边界"
	// markerRosterHeader 与 validateCaseMarkerText 识别 roster 的表头一致。
	markerRosterHeader = "| Member | Kind | Durable state | Member source |"
)

// upgradableMarkerSections 是允许整段换新模板正文的已知静态规则段；其它段一律保留原字节。
var upgradableMarkerSections = []string{"Commander", "协作章程", "事实来源", "共享研究产物"}

type markerLine struct{ text, nl string }

type markerSection struct {
	heading string // 去掉 "## " 前缀的标题
	start   int    // 标题行下标
	end     int    // 下一段标题行下标（不含）；末段为 len(lines)
}

// upgradeMarker 在内存中把旧 case 的 .steamai-vnext/CLAUDE.md 推进到新快照身份，不做任何 I/O。
//
// 只有两类改动，其余字节（含每行原有换行）原样保留：
//   - 顶部元数据区里唯一精确等于旧快照值的身份行换成新值：Source revision、Pack tree、Common tree、
//     Snapshot digest、可选 Auxiliary pack tree。其它位置的同名/同值文本不动；不换包，不做全局替换。
//   - 调用方传入 immutable 旧模板时，白名单内仍是未改动纯框架的 H2 段整段换成新模板正文：live、旧模板、
//     新模板各恰好一段同名段，新段没有占位符且与旧段不同，live 该段与旧模板逐字一致（只归一化 CRLF）。
//     交接段标题起（含其内子标题）之后不再选择任何段；用户改过的段、未知段、带项目事实的
//     Case 边界与 roster 都不触碰，也不新增 live 没有的段。没有旧模板时只做第一类。
//
// 结果必须通过 validateCaseMarkerText(to) 且 roster 与升级前一致；错误信息不含文件内容。
func upgradeMarker(live, oldTemplate, newTemplate []byte, from, to snapshotMetadata) ([]byte, error) {
	if len(newTemplate) == 0 {
		return nil, errors.New("case marker 升级缺少新模板")
	}
	if from.Pack != to.Pack || from.AuxPack != to.AuxPack {
		return nil, errors.New("case marker 升级不支持更换 pack")
	}
	roster, err := validateCaseMarkerText(string(live), from)
	if err != nil {
		return nil, errors.New("旧 case marker 与旧快照不符")
	}
	lines := splitMarkerLines(string(live))
	sections := markerSections(lines)
	region, ok := markerIdentityRegion(lines, sections)
	if !ok {
		return nil, errors.New("旧 case marker 顶部元数据区无法唯一判定")
	}
	identity := [][3]string{
		{"Source revision", from.Revision, to.Revision},
		{"Pack tree", from.PackTree, to.PackTree},
		{"Common tree", from.CommonTree, to.CommonTree},
		{"Snapshot digest", from.PayloadDigest, to.PayloadDigest},
	}
	if from.AuxPack != "" {
		identity = append(identity, [3]string{"Auxiliary pack tree", from.AuxPackTree, to.AuxPackTree})
	}
	for _, field := range identity {
		index, count := -1, 0
		for i := range region {
			if strings.TrimSpace(region[i].text) == "- "+field[0]+"：`"+field[1]+"`" {
				index, count = i, count+1
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("旧 case marker %s 行缺失或不唯一", field[0])
		}
		text := region[index].text
		lead := len(text) - len(strings.TrimLeftFunc(text, unicode.IsSpace))
		trail := len(strings.TrimRightFunc(text, unicode.IsSpace))
		region[index].text = text[:lead] + "- " + field[0] + "：`" + field[2] + "`" + text[trail:]
	}
	updates := upgradeableMarkerSections(lines, sections, oldTemplate, newTemplate)
	var out strings.Builder
	next := 0
	for _, section := range sections {
		writeMarkerLines(&out, lines[next:section.start+1])
		if body, ok := updates[section.heading]; ok {
			out.WriteString(body)
		} else {
			writeMarkerLines(&out, lines[section.start+1:section.end])
		}
		next = section.end
	}
	writeMarkerLines(&out, lines[next:])
	upgraded := out.String()
	after, err := validateCaseMarkerText(upgraded, to)
	if err != nil {
		return nil, errors.New("升级后的 case marker 无效")
	}
	if !slices.Equal(roster, after) {
		return nil, errors.New("case marker 升级改变了 roster")
	}
	return []byte(upgraded), nil
}

// markerIdentityRegion 返回身份行所在的顶部元数据区（lines 的子切片）：唯一的 "## Case 边界" 段；
// 历史文件没有该段时，是首个 H2 标题和 roster 表头之前的文首部分。无法唯一判定时 ok=false。
func markerIdentityRegion(lines []markerLine, sections []markerSection) ([]markerLine, bool) {
	rosterHeader := slices.IndexFunc(lines, func(line markerLine) bool {
		return strings.TrimSpace(line.text) == markerRosterHeader
	})
	if rosterHeader < 0 {
		return nil, false
	}
	var boundary markerSection
	count := 0
	for _, section := range sections {
		if section.heading == caseBoundaryHeading {
			boundary, count = section, count+1
		}
	}
	switch {
	case count > 1, count == 1 && boundary.start >= rosterHeader:
		return nil, false
	case count == 1:
		return lines[boundary.start:boundary.end], true
	}
	end := rosterHeader
	if len(sections) > 0 && sections[0].start < end {
		end = sections[0].start
	}
	return lines[:end], true
}

// upgradeableMarkerSections 找出 live 中仍是未改动纯框架的白名单 H2 段，返回其新模板正文（按 live
// 该段标题行的换行风格）。交接段标题起之后、含占位符的段、用户改过的段和无法唯一对照的段都不在结果里。
func upgradeableMarkerSections(live []markerLine, sections []markerSection, oldTemplate, newTemplate []byte) map[string]string {
	updates := map[string]string{}
	if len(oldTemplate) == 0 {
		return updates
	}
	oldLines, newLines := splitMarkerLines(string(oldTemplate)), splitMarkerLines(string(newTemplate))
	oldSections, newSections := markerSections(oldLines), markerSections(newLines)
	for _, section := range sections {
		if section.heading == commanderHandoffHeading {
			break
		}
		if !slices.Contains(upgradableMarkerSections, section.heading) {
			continue
		}
		old, oldOK := uniqueMarkerSection(oldSections, section.heading)
		fresh, newOK := uniqueMarkerSection(newSections, section.heading)
		if _, unique := uniqueMarkerSection(sections, section.heading); !oldOK || !newOK || !unique {
			continue
		}
		// live 段与旧模板逐字一致，即证明它没有项目事实；新段必须仍是无需渲染的框架文本。
		// 新旧正文相同则保留 live 原字节，避免混合换行被改写。
		oldBody := markerLF(markerSectionText(oldLines, old))
		newBody := markerLF(markerSectionText(newLines, fresh))
		if newBody == oldBody || strings.Contains(newBody, "{{") || markerLF(markerSectionText(live, section)) != oldBody {
			continue
		}
		updates[section.heading] = markerWithLineEnding(newBody, live[section.start].nl)
	}
	return updates
}

// uniqueMarkerSection 返回 sections 中唯一同名的那个段。
func uniqueMarkerSection(sections []markerSection, heading string) (markerSection, bool) {
	var found markerSection
	count := 0
	for _, section := range sections {
		if section.heading == heading {
			found, count = section, count+1
		}
	}
	return found, count == 1
}

// markerSectionText 返回段标题行之后、下一段标题行之前的全部行（含各自换行）。
func markerSectionText(lines []markerLine, section markerSection) string {
	var text strings.Builder
	writeMarkerLines(&text, lines[section.start+1:section.end])
	return text.String()
}

func markerLF(text string) string { return strings.ReplaceAll(text, "\r\n", "\n") }

// markerWithLineEnding 把已归一化为 LF 的段正文换成 live 使用的换行。
func markerWithLineEnding(text, nl string) string {
	if nl == "\r\n" {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

// splitMarkerLines 按行拆分并保留每行自己的换行；重新拼接即得原文。
func splitMarkerLines(text string) []markerLine {
	var lines []markerLine
	for len(text) > 0 {
		index := strings.IndexByte(text, '\n')
		if index < 0 {
			return append(lines, markerLine{text, ""})
		}
		line := markerLine{text[:index], "\n"}
		if strings.HasSuffix(line.text, "\r") {
			line.text, line.nl = strings.TrimSuffix(line.text, "\r"), "\r\n"
		}
		lines = append(lines, line)
		text = text[index+1:]
	}
	return lines
}

func writeMarkerLines(out *strings.Builder, lines []markerLine) {
	for _, line := range lines {
		out.WriteString(line.text)
		out.WriteString(line.nl)
	}
}

// markerSections 只按 H2 标题切段；段内任何改动（含更小标题）都算作该段正文的一部分。
func markerSections(lines []markerLine) []markerSection {
	var sections []markerSection
	for i, line := range lines {
		trimmed := strings.TrimSpace(line.text)
		if !strings.HasPrefix(trimmed, "## ") {
			continue
		}
		if len(sections) > 0 {
			sections[len(sections)-1].end = i
		}
		sections = append(sections, markerSection{heading: strings.TrimSpace(trimmed[3:]), start: i, end: len(lines)})
	}
	return sections
}
