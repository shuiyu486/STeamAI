package casebootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func BuildPreview(git, source, caseRoot string, facts Facts) (Preview, error) {
	if err := facts.Validate(); err != nil {
		return Preview{}, err
	}
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return Preview{}, err
	}
	if err := requirePlainDirectory(caseRoot); err != nil {
		return Preview{}, fmt.Errorf("case 根目录无效: %w", err)
	}
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	stateExists, err := exists(stateRoot)
	if err != nil {
		return Preview{}, err
	}
	if stateExists {
		return Preview{}, ErrPartialCase
	}
	frozen, err := freezeSource(git, source, facts.Pack)
	if err != nil {
		return Preview{}, err
	}
	writes, generated, snapshotDigest, err := buildWrites(frozen, caseRoot, facts)
	if err != nil {
		return Preview{}, err
	}
	preview := Preview{
		SchemaVersion: 1, Revision: frozen.Revision, PackTree: frozen.PackTree,
		CommonTree: frozen.CommonTree, SourceDigest: frozen.Digest,
		SnapshotDigest: snapshotDigest, Facts: facts,
		SourceRecords: frozen.Records, Writes: writes, SourceDiff: frozen.Diff,
		GeneratedFiles: generated,
	}
	preview.Identity = previewIdentity(preview)
	preview.HumanPreview = renderHumanPreview(preview)
	return preview, nil
}

func buildWrites(source frozenSource, caseRoot string, facts Facts) ([]PlannedWrite, map[string]string, string, error) {
	var writes []PlannedWrite
	appendSource := func(sourcePath, targetPath string) error {
		record, ok := source.ByPath[sourcePath]
		if !ok {
			return fmt.Errorf("Fresh source 缺少 %s", sourcePath)
		}
		writes = append(writes, sourceWrite(record, targetPath))
		return nil
	}
	if err := appendSource(".claude/skills/steamai/SKILL.md", ".claude/skills/steamai/SKILL.md"); err != nil {
		return nil, nil, "", err
	}
	if err := appendSource("vnext/learning-feedback.md", ".steamai-vnext/contracts/learning-feedback.md"); err != nil {
		return nil, nil, "", err
	}
	if err := appendSource("vnext/verified-learning.md", ".steamai-vnext/contracts/verified-learning.md"); err != nil {
		return nil, nil, "", err
	}
	for _, record := range source.Records {
		switch {
		case strings.HasPrefix(record.Path, "vnext/templates/"):
			templatePath, _ := strings.CutPrefix(record.Path, "vnext/")
			target := ".steamai-vnext/contracts/" + templatePath
			writes = append(writes, sourceWrite(record, target))
		case strings.HasPrefix(record.Path, "packs/"+facts.Pack+"/") || strings.HasPrefix(record.Path, "common/"):
			writes = append(writes, sourceWrite(record, ".steamai-vnext/pack-snapshot/"+record.Path))
		}
	}

	snapshotDigest := snapshotPayloadDigest(writes)
	caseTemplateRecord, ok := source.ByPath["vnext/templates/case/CLAUDE.md"]
	if !ok {
		return nil, nil, "", errors.New("Fresh source 缺少 case template")
	}
	caseFile := renderCaseTemplate(string(caseTemplateRecord.Data), facts, source, snapshotDigest)
	if strings.Contains(caseFile, "{{") {
		return nil, nil, "", errors.New("case template 仍有未解析 placeholder")
	}
	writes = append(writes, generatedWrite("case", ".steamai-vnext/CLAUDE.md", []byte(caseFile)))
	writes = append(writes, generatedWrite("artifact-index", ".steamai-vnext/artifacts/index.md", []byte("# Artifact Index\n\nNo artifacts indexed.\n")))
	for _, member := range facts.Members {
		memberFile, err := renderMember(source, facts.Pack, member)
		if err != nil {
			return nil, nil, "", err
		}
		target := ".steamai-vnext/members/" + member.Name + "/CLAUDE.md"
		writes = append(writes, generatedWrite("member:"+member.Name, target, []byte(memberFile)))
	}

	immutable := immutableRecords(writes)
	snapshot := renderSnapshot(source, facts.Pack, snapshotDigest, immutable)
	writes = append(writes, generatedWrite("snapshot-metadata", ".steamai-vnext/pack-snapshot/snapshot.yml", []byte(snapshot)))
	sort.Slice(writes, func(i, j int) bool { return writes[i].TargetPath < writes[j].TargetPath })
	for i := range writes {
		if err := annotateTarget(caseRoot, &writes[i]); err != nil {
			return nil, nil, "", err
		}
	}
	generated := map[string]string{
		"case":           ".steamai-vnext/CLAUDE.md",
		"artifact-index": ".steamai-vnext/artifacts/index.md",
		"snapshot":       ".steamai-vnext/pack-snapshot/snapshot.yml",
	}
	for _, member := range facts.Members {
		generated["member:"+member.Name] = ".steamai-vnext/members/" + member.Name + "/CLAUDE.md"
	}
	return writes, generated, snapshotDigest, nil
}

func sourceWrite(record SourceRecord, target string) PlannedWrite {
	return PlannedWrite{
		SourceKind: "working-tree", SourcePath: record.Path, GitMode: record.GitMode,
		HeadBlob: record.HeadBlob, ContentBlob: record.ContentBlob,
		TargetPath: target, SHA256: record.SHA256, Bytes: record.Bytes,
		Data: append([]byte(nil), record.Data...), TargetPreBytes: -1,
	}
}

func generatedWrite(name, target string, data []byte) PlannedWrite {
	return PlannedWrite{
		SourceKind: "generated", SourcePath: "generated:" + name, GitMode: "generated",
		HeadBlob: hashBytes(data), ContentBlob: hashBytes(data), TargetPath: target,
		SHA256: hashBytes(data), Bytes: len(data), Data: append([]byte(nil), data...), TargetPreBytes: -1,
	}
}

func annotateTarget(caseRoot string, write *PlannedWrite) error {
	if err := validateTargetPath(caseRoot, write.TargetPath); err != nil {
		return err
	}
	target := filepath.Join(caseRoot, filepath.FromSlash(write.TargetPath))
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		write.TargetAction = "create"
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrCollision, write.TargetPath)
	}
	if err := rejectReparse(target); err != nil {
		return fmt.Errorf("%w: %s", ErrCollision, write.TargetPath)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if write.TargetPath != ".claude/skills/steamai/SKILL.md" || hashBytes(data) != write.SHA256 {
		return fmt.Errorf("%w: %s", ErrCollision, write.TargetPath)
	}
	write.TargetAction = "unchanged"
	write.TargetPreSHA256 = hashBytes(data)
	write.TargetPreBytes = len(data)
	return nil
}

func renderCaseTemplate(template string, facts Facts, source frozenSource, snapshotDigest string) string {
	rows := []string{}
	for _, member := range facts.Members {
		rows = append(rows, fmt.Sprintf("| %s | %s | active | `.steamai-vnext/members/%s/CLAUDE.md` |", member.Name, member.Kind, member.Name))
	}
	if len(rows) == 0 {
		rows = append(rows, "| none | execution | inactive | none |")
	}
	replacements := map[string]string{
		"{{CASE_NAME}}": facts.Name, "{{GOAL}}": facts.Goal,
		"{{AUTHORIZED_SCOPE}}": facts.Authorization, "{{PROHIBITED_ACTIONS}}": facts.Prohibited,
		"{{STOP_CONDITIONS}}": facts.Stop, "{{PACK_NAME}}": facts.Pack,
		"{{PACK_REVISION}}": source.Revision, "{{PACK_SNAPSHOT_TREE}}": source.PackTree,
		"{{COMMON_SNAPSHOT_TREE}}": source.CommonTree, "{{SNAPSHOT_DIGEST}}": snapshotDigest,
		"{{TEAM_ROSTER_ROWS}}": strings.Join(rows, "\n"),
	}
	for placeholder, value := range replacements {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	return template
}

func renderMember(source frozenSource, pack string, member MemberFacts) (string, error) {
	templateRecord, ok := source.ByPath["vnext/templates/member/CLAUDE.md"]
	if !ok {
		return "", errors.New("Fresh source 缺少 member template")
	}
	roleRules := ""
	if member.Kind == "reviewer" {
		role, ok := source.ByPath["vnext/templates/roles/reviewer.md"]
		if !ok {
			return "", errors.New("Fresh source 缺少 reviewer role template")
		}
		roleRules = string(role.Data)
	} else if role, ok := source.ByPath["vnext/templates/roles/analysis-member.md"]; ok {
		roleRules = string(role.Data)
	}
	template := string(templateRecord.Data)
	values := map[string]string{
		"{{MEMBER_NAME}}": member.Name, "{{ROLE}}": member.Role,
		"{{RESPONSIBILITY}}": member.Responsibility, "{{TASK_GOAL}}": member.TaskGoal,
		"{{INPUTS}}": member.Inputs, "{{ALLOWED_READS}}": member.AllowedReads,
		"{{ALLOWED_WRITES}}": member.AllowedWrites, "{{DELIVERABLES}}": member.Deliverables,
		"{{STOP_OR_ESCALATE}}": member.StopOrEscalate, "{{EXIT_CONDITIONS}}": member.ExitConditions,
		"{{ROLE_SPECIFIC_RULES}}": roleRules,
	}
	for placeholder, value := range values {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	if strings.Contains(template, "{{") {
		return "", fmt.Errorf("成员 %s 模板仍有未解析 placeholder", member.Name)
	}
	_ = pack
	return template, nil
}

func immutableRecords(writes []PlannedWrite) []PlannedWrite {
	var records []PlannedWrite
	for _, write := range writes {
		if write.SourceKind == "working-tree" && (strings.HasPrefix(write.TargetPath, ".steamai-vnext/contracts/") || write.TargetPath == ".claude/skills/steamai/SKILL.md") {
			records = append(records, write)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].TargetPath < records[j].TargetPath })
	return records
}

func renderSnapshot(source frozenSource, pack, snapshotDigest string, immutable []PlannedWrite) string {
	var payloadLines []string
	for _, record := range snapshotPayloadWritesFromSource(source.Records, pack) {
		payloadLines = append(payloadLines, fmt.Sprintf(
			"  - path: %s\n    git-mode: %s\n    head-blob: %s\n    content-blob: %s\n    sha256: %s\n    bytes: %d",
			record.Path, record.GitMode, record.HeadBlob, record.ContentBlob, record.SHA256, record.Bytes))
	}
	var immutableLines []string
	for _, write := range immutable {
		immutableLines = append(immutableLines, fmt.Sprintf(
			"  - path: %s\n    sha256: %s\n    bytes: %d", write.TargetPath, write.SHA256, write.Bytes))
	}
	return fmt.Sprintf(
		"schema: steamai-case-snapshot-v2\npack: %s\nrevision: %s\npack-tree: %s\ncommon-tree: %s\nsource-digest: %s\npayload-digest: %s\nfiles:\n%s\nimmutable-files:\n%s\n",
		pack, source.Revision, source.PackTree, source.CommonTree, source.Digest, snapshotDigest,
		strings.Join(payloadLines, "\n"), strings.Join(immutableLines, "\n"))
}

func snapshotPayloadWritesFromSource(records []SourceRecord, pack string) []SourceRecord {
	var payload []SourceRecord
	for _, record := range records {
		if strings.HasPrefix(record.Path, "packs/"+pack+"/") || strings.HasPrefix(record.Path, "common/") {
			payload = append(payload, record)
		}
	}
	sort.Slice(payload, func(i, j int) bool { return payload[i].Path < payload[j].Path })
	return payload
}

func snapshotPayloadDigest(writes []PlannedWrite) string {
	var records []string
	for _, write := range writes {
		if strings.HasPrefix(write.TargetPath, ".steamai-vnext/pack-snapshot/packs/") || strings.HasPrefix(write.TargetPath, ".steamai-vnext/pack-snapshot/common/") {
			records = append(records, strings.Join([]string{
				write.SourcePath, write.GitMode, write.ContentBlob, strconv.Itoa(write.Bytes), write.SHA256,
			}, "\x00"))
		}
	}
	sort.Strings(records)
	return "sha256:" + hashBytes([]byte(strings.Join(records, "\n")+"\n"))
}

func previewIdentity(preview Preview) string {
	facts, _ := json.Marshal(preview.Facts)
	parts := []string{
		"schema", strconv.Itoa(preview.SchemaVersion), "revision", preview.Revision,
		"pack-tree", preview.PackTree, "common-tree", preview.CommonTree,
		"source-digest", preview.SourceDigest, "snapshot-digest", preview.SnapshotDigest,
		"facts", string(facts),
	}
	for _, write := range preview.Writes {
		parts = append(parts, "write", write.SourceKind, write.SourcePath, write.GitMode,
			write.HeadBlob, write.ContentBlob, write.TargetPath, write.TargetAction,
			write.TargetPreSHA256, strconv.Itoa(write.TargetPreBytes), write.SHA256, strconv.Itoa(write.Bytes))
	}
	return hashBytes([]byte(strings.Join(parts, "\x00") + "\n"))
}

func renderHumanPreview(preview Preview) string {
	var changed []string
	for _, record := range preview.SourceRecords {
		if record.Changed {
			changed = append(changed, record.Path)
		}
	}
	var targets []string
	for _, write := range preview.Writes {
		preState := "absent"
		if write.TargetAction == "unchanged" {
			preState = fmt.Sprintf("sha256:%s bytes:%d", write.TargetPreSHA256, write.TargetPreBytes)
		}
		targets = append(targets, fmt.Sprintf(
			"- target:%s action:%s pre-state:%s output-sha256:%s output-bytes:%d source-kind:%s source-path:%s git-mode:%s head-blob:%s content-blob:%s",
			write.TargetPath, write.TargetAction, preState, write.SHA256, write.Bytes, write.SourceKind,
			write.SourcePath, write.GitMode, write.HeadBlob, write.ContentBlob,
		))
	}
	changedText := "无"
	if len(changed) > 0 {
		changedText = strings.Join(changed, "\n- ")
		changedText = "- " + changedText
	}
	var generated []string
	for name, target := range preview.GeneratedFiles {
		write := writeByTarget(preview.Writes, target)
		generated = append(generated, fmt.Sprintf("--- %s: %s ---\n%s", name, target, string(write.Data)))
	}
	sort.Strings(generated)
	return fmt.Sprintf(
		"STeamAI Fresh 零写入预览\nCase: %s\nGoal: %s\nAuthorization: %s\nProhibited: %s\nStop: %s\nSelected pack: %s\nBase revision: %s\nSource digest: %s\nSnapshot digest: %s\n本机未提交且会进入 case 的 source 文件:\n%s\n计划写入:\n%s\n生成文件全文:\n%s\nCanonical working-tree diff:\n%s\nBlockers: 无\n当前仍为零写入。\n确认命令: %s%s\n",
		preview.Facts.Name, preview.Facts.Goal, preview.Facts.Authorization, preview.Facts.Prohibited,
		preview.Facts.Stop, preview.Facts.Pack, preview.Revision, preview.SourceDigest,
		preview.SnapshotDigest, changedText, strings.Join(targets, "\n"), strings.Join(generated, "\n"),
		preview.SourceDiff, ConfirmationPrefix, preview.Identity)
}

func validateTargetPath(caseRoot, targetRel string) error {
	if targetRel == "" || filepath.IsAbs(filepath.FromSlash(targetRel)) || strings.Contains(targetRel, "\\") {
		return ErrCollision
	}
	target := filepath.Clean(filepath.Join(caseRoot, filepath.FromSlash(targetRel)))
	if !pathWithin(target, caseRoot) {
		return ErrCollision
	}
	rel, err := filepath.Rel(caseRoot, target)
	if err != nil {
		return err
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	current := caseRoot
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || rejectReparse(current) != nil {
			return ErrCollision
		}
	}
	return nil
}

func exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
