package casebootstrap

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type snapshotMetadata struct {
	Schema         string
	Pack           string
	AuxPack        string
	Revision       string
	PackTree       string
	AuxPackTree    string
	CommonTree     string
	SourceDigest   string
	PayloadDigest  string
	Files          []SourceRecord
	ImmutableFiles []PlannedWrite
}

type CurrentIdentity struct {
	Pack          string
	AuxPack       string
	Revision      string
	PackTree      string
	AuxPackTree   string
	CommonTree    string
	SourceDigest  string
	PayloadDigest string
	Roster        []RosterMember
}

// 升级使用刚经深验的同一份清单和项目说明，不重新读取一份可能不同的文件。
type currentSnapshot struct {
	metadata snapshotMetadata
	manifest []byte
	marker   []byte
}

type RosterMember struct {
	Name  string
	Kind  string
	State string
	Ref   string
}

func ValidateCurrent(caseRoot string) error {
	_, err := InspectCurrent(caseRoot)
	return err
}

type currentIssue struct {
	section string
	err     error
}

func (e *currentIssue) Error() string { return e.err.Error() }
func (e *currentIssue) Unwrap() error { return e.err }

// CurrentIssue 只返回固定的检查位置，不把损坏文件正文或业务内容显示给用户。
func CurrentIssue(err error) string {
	if issue, ok := errors.AsType[*currentIssue](err); ok {
		return issue.section
	}
	return "项目根目录或框架文件"
}

func InspectCurrent(caseRoot string) (CurrentIdentity, error) {
	return inspectCurrent(caseRoot, false, nil)
}

func inspectCurrent(caseRoot string, publishing bool, checked *currentSnapshot) (CurrentIdentity, error) {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return CurrentIdentity{}, err
	}
	if err := requirePlainDirectory(caseRoot); err != nil {
		return CurrentIdentity{}, err
	}
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	if err := requirePlainPath(caseRoot, stateRoot, true); err != nil {
		return CurrentIdentity{}, &currentIssue{".steamai-vnext", err}
	}
	if !publishing {
		if pending, err := exists(filepath.Join(stateRoot, upgradePendingDir)); err != nil || pending {
			return CurrentIdentity{}, &currentIssue{".steamai-vnext/.upgrade-pending（上次更新未收尾，请保留原件）", errors.New("项目更新尚未收尾")}
		}
	}
	for _, rel := range []string{
		"contracts", "pack-snapshot", "members", "artifacts", "evidence", "findings", "reviews",
		"learnings/candidates", "learnings/patches",
	} {
		if err := requirePlainPath(caseRoot, filepath.Join(stateRoot, filepath.FromSlash(rel)), true); err != nil {
			return CurrentIdentity{}, &currentIssue{".steamai-vnext/" + rel, fmt.Errorf("current case 缺少或包含无效目录 %s: %w", rel, err)}
		}
	}
	for _, rel := range []string{".steamai-vnext/CLAUDE.md", ".steamai-vnext/contracts/learning-feedback.md", ".steamai-vnext/pack-snapshot/snapshot.yml", ".steamai-vnext/artifacts/index.md", ".claude/skills/steamai/SKILL.md"} {
		if err := requirePlainPath(caseRoot, filepath.Join(caseRoot, filepath.FromSlash(rel)), false); err != nil {
			return CurrentIdentity{}, &currentIssue{rel, fmt.Errorf("current case 缺少或包含无效文件 %s: %w", rel, err)}
		}
	}
	manifest, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(snapshotRel)))
	if err != nil {
		return CurrentIdentity{}, &currentIssue{snapshotRel, err}
	}
	metadata, err := parseSnapshotBytes(manifest)
	if err != nil {
		return CurrentIdentity{}, &currentIssue{snapshotRel, err}
	}
	if metadata.Schema != "steamai-case-snapshot-v2" || !packNamePattern.MatchString(metadata.Pack) ||
		!hexIdentityPattern.MatchString(metadata.Revision) || !hexIdentityPattern.MatchString(metadata.PackTree) ||
		!hexIdentityPattern.MatchString(metadata.CommonTree) || !prefixedSHA256(metadata.SourceDigest) ||
		!prefixedSHA256(metadata.PayloadDigest) {
		return CurrentIdentity{}, &currentIssue{snapshotRel, errors.New("current snapshot identity 无效")}
	}
	if err := validateSnapshotPayload(caseRoot, metadata); err != nil {
		return CurrentIdentity{}, &currentIssue{".steamai-vnext/pack-snapshot（固定包内容）", err}
	}
	if err := validateImmutableFiles(caseRoot, metadata.ImmutableFiles); err != nil {
		return CurrentIdentity{}, &currentIssue{"项目 skill 或 .steamai-vnext/contracts", err}
	}
	marker, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(caseMarkerRel)))
	if err != nil {
		return CurrentIdentity{}, &currentIssue{caseMarkerRel, err}
	}
	roster, err := validateCaseMarkerText(string(marker), metadata)
	if err != nil {
		return CurrentIdentity{}, &currentIssue{caseMarkerRel, err}
	}
	if err := validateMembers(caseRoot, roster); err != nil {
		return CurrentIdentity{}, &currentIssue{".steamai-vnext/members 与项目成员清单", err}
	}
	if checked != nil {
		*checked = currentSnapshot{metadata, manifest, marker}
	}
	return CurrentIdentity{
		Pack: metadata.Pack, AuxPack: metadata.AuxPack, Revision: metadata.Revision,
		PackTree: metadata.PackTree, AuxPackTree: metadata.AuxPackTree,
		CommonTree: metadata.CommonTree, SourceDigest: metadata.SourceDigest, PayloadDigest: metadata.PayloadDigest,
		Roster: roster,
	}, nil
}

func prefixedSHA256(value string) bool {
	digest, ok := strings.CutPrefix(value, "sha256:")
	return ok && shaPattern.MatchString(digest)
}

func validateCaseMarkerText(text string, metadata snapshotMetadata) ([]RosterMember, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("current case marker 为空")
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), "：`")
		if ok && strings.HasSuffix(value, "`") {
			if _, exists := fields[key]; exists && (key == "Selected pack" || key == "Pack tree" || key == "Auxiliary pack" || key == "Auxiliary pack tree") {
				return nil, fmt.Errorf("current case marker %s 重复", key)
			}
			if fields[key] == "" {
				fields[key] = strings.TrimSuffix(value, "`")
			}
		}
	}
	for key, want := range map[string]string{
		"Selected pack": metadata.Pack, "Source revision": metadata.Revision, "Pack tree": metadata.PackTree,
		"Common tree": metadata.CommonTree, "Snapshot digest": metadata.PayloadDigest,
	} {
		if fields[key] != want {
			return nil, fmt.Errorf("current case marker %s 与 snapshot 不匹配", key)
		}
	}
	for key, want := range map[string]string{
		"Auxiliary pack": metadata.AuxPack, "Auxiliary pack tree": metadata.AuxPackTree,
	} {
		value, present := fields[key]
		if (metadata.AuxPack != "") != present || value != want {
			return nil, fmt.Errorf("current case marker %s 与 snapshot 不匹配", key)
		}
	}
	for _, key := range []string{"Case 名称", "授权范围", "禁止事项", "全局停止条件"} {
		if strings.TrimSpace(fields[key]) == "" {
			return nil, fmt.Errorf("current case marker 缺少 %s", key)
		}
	}
	// 目标字段：现行模板为 "Case 目标"（研究/开发双交付），历史 case 物化为
	// "研究目标"；深验接受任一非空形态，不重写旧 case 文件。
	if strings.TrimSpace(fields["Case 目标"]) == "" && strings.TrimSpace(fields["研究目标"]) == "" {
		return nil, fmt.Errorf("current case marker 缺少 Case 目标")
	}
	if strings.ContainsAny(fields["Case 名称"], "\r\n\x00") {
		return nil, errors.New("current case marker Case 名称无效")
	}
	lines := strings.Split(text, "\n")
	var roster []RosterMember
	inRoster := false
	seen := map[string]bool{}
	activeExecution, activeReviewer := 0, 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "| Member | Kind | Durable state | Member source |" {
			inRoster = true
			continue
		}
		if !inRoster {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			if len(roster) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "|---") {
			continue
		}
		parts := strings.Split(trimmed, "|")
		if len(parts) != 6 {
			return nil, errors.New("current roster 行格式无效")
		}
		member := RosterMember{
			Name: strings.TrimSpace(parts[1]), Kind: strings.TrimSpace(parts[2]), State: strings.TrimSpace(parts[3]),
			Ref: strings.Trim(strings.TrimSpace(parts[4]), "`"),
		}
		if member.Name == "none" && member.Kind == "execution" && member.State == "inactive" && member.Ref == "none" {
			continue
		}
		if !memberNamePattern.MatchString(member.Name) || windowsReservedName(member.Name) || seen[member.Name] {
			return nil, errors.New("current roster member 名称无效或重复")
		}
		seen[member.Name] = true
		if member.Kind != "execution" && member.Kind != "reviewer" {
			return nil, fmt.Errorf("current roster member %s kind 无效", member.Name)
		}
		if member.State != "active" && member.State != "completed" && member.State != "inactive" {
			return nil, fmt.Errorf("current roster member %s state 无效", member.Name)
		}
		wantRef := ".steamai-vnext/members/" + member.Name + "/CLAUDE.md"
		if member.Ref != wantRef {
			return nil, fmt.Errorf("current roster member %s ref 无效", member.Name)
		}
		if member.State == "active" {
			if member.Kind == "execution" {
				activeExecution++
			} else {
				activeReviewer++
			}
		}
		roster = append(roster, member)
	}
	if !inRoster || activeExecution > 3 || activeReviewer > 1 {
		return nil, errors.New("current roster 缺失或 active 成员超过上限")
	}
	return roster, nil
}

func validateMembers(caseRoot string, roster []RosterMember) error {
	membersRoot := filepath.Join(caseRoot, ".steamai-vnext", "members")
	entries, err := os.ReadDir(membersRoot)
	if err != nil {
		return err
	}
	expected := make(map[string]RosterMember, len(roster))
	for _, member := range roster {
		expected[member.Name] = member
	}
	if len(entries) != len(expected) {
		return errors.New("current member 目录集合与 roster 不一致")
	}
	for _, entry := range entries {
		member, ok := expected[entry.Name()]
		if !ok || !entry.IsDir() {
			return fmt.Errorf("current member 目录未在 roster 声明: %s", entry.Name())
		}
		memberRoot := filepath.Join(membersRoot, entry.Name())
		if err := requirePlainPath(caseRoot, memberRoot, true); err != nil {
			return err
		}
		files, err := regularFiles(memberRoot, nil)
		if err != nil {
			return err
		}
		// 成员目录允许承载合同声明的 workspace 文件（含 Claude Code 写入的
		// .claude/settings.local.json）；身份约束是 CLAUDE.md 存在且与 roster ref 同文件。
		if !slices.Contains(files, "CLAUDE.md") {
			return fmt.Errorf("current member %s 缺少身份文件 CLAUDE.md", member.Name)
		}
		refPath := filepath.Join(caseRoot, filepath.FromSlash(member.Ref))
		identityPath := filepath.Join(memberRoot, "CLAUDE.md")
		refInfo, refErr := os.Stat(refPath)
		identityInfo, identityErr := os.Stat(identityPath)
		if refErr != nil || identityErr != nil || !os.SameFile(refInfo, identityInfo) {
			return fmt.Errorf("current member %s roster ref identity 不匹配", member.Name)
		}
	}
	return nil
}

func validateSnapshotPayload(caseRoot string, metadata snapshotMetadata) error {
	expected := make(map[string]SourceRecord, len(metadata.Files))
	localRecords := make([]SourceRecord, 0, len(metadata.Files))
	for _, record := range metadata.Files {
		clean, err := cleanRelativePath(record.Path)
		if err != nil || clean != record.Path || expected[record.Path].Path != "" ||
			!snapshotPayloadPath(record.Path, metadata.Pack, metadata.AuxPack) {
			return errors.New("snapshot files 列表包含重复或越界路径")
		}
		expected[record.Path] = record
		path := filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", filepath.FromSlash(record.Path))
		if err := requirePlainPath(caseRoot, path, false); err != nil {
			return fmt.Errorf("snapshot payload %s 无效: %w", record.Path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) != record.Bytes || hashBytes(data) != record.SHA256 {
			return fmt.Errorf("snapshot payload %s bytes 漂移", record.Path)
		}
		record.Data = data
		localRecords = append(localRecords, record)
	}
	for _, pack := range []string{metadata.Pack, metadata.AuxPack} {
		if pack != "" {
			if err := validateSelectedPack(localRecords, pack); err != nil {
				return fmt.Errorf("snapshot pack %s: %w", pack, err)
			}
		}
	}
	actual, err := regularFiles(filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot"), []string{"snapshot.yml"})
	if err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return errors.New("snapshot payload 路径集合漂移")
	}
	for _, rel := range actual {
		if _, ok := expected[rel]; !ok {
			return fmt.Errorf("snapshot payload 含未声明文件 %s", rel)
		}
	}
	if payloadDigest(localRecords) != metadata.PayloadDigest {
		return errors.New("snapshot payload digest 漂移")
	}
	return nil
}

func SupportsVerifiedLearning(caseRoot string) (bool, error) {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return false, err
	}
	var checked currentSnapshot
	if _, err := inspectCurrent(caseRoot, false, &checked); err != nil {
		return false, err
	}
	if !slices.ContainsFunc(checked.metadata.ImmutableFiles, func(f PlannedWrite) bool { return f.TargetPath == ".steamai-vnext/contracts/verified-learning.md" }) {
		return false, nil
	}
	// 旧布局显式更新可以收到新合同文本，但不因此自动拥有评测工作区或升级历史记录。
	for _, dir := range []string{"specs", "runs", "attestations", "outcomes", "work"} {
		if err := requirePlainPath(caseRoot, filepath.Join(caseRoot, ".steamai-vnext", "evaluations", dir), true); err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, errors.New("评测工作目录无效")
		}
	}
	return true, nil
}

func validateImmutableFiles(caseRoot string, records []PlannedWrite) error {
	if len(records) == 0 {
		return errors.New("snapshot 缺少 immutable-files")
	}
	expectedContracts := map[string]bool{}
	seen := map[string]bool{}
	for _, record := range records {
		clean, err := cleanRelativePath(record.TargetPath)
		if err != nil || clean != record.TargetPath || seen[record.TargetPath] || record.Bytes < 0 || !shaPattern.MatchString(record.SHA256) {
			return errors.New("immutable file record 无效")
		}
		seen[record.TargetPath] = true
		if record.TargetPath != ".claude/skills/steamai/SKILL.md" && !strings.HasPrefix(record.TargetPath, ".steamai-vnext/contracts/") {
			return errors.New("immutable file record 越出 skill/contracts")
		}
		path := filepath.Join(caseRoot, filepath.FromSlash(record.TargetPath))
		if err := requirePlainPath(caseRoot, path, false); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) != record.Bytes || hashBytes(data) != record.SHA256 {
			return fmt.Errorf("immutable file %s 漂移", record.TargetPath)
		}
		if rel, ok := strings.CutPrefix(record.TargetPath, ".steamai-vnext/contracts/"); ok {
			expectedContracts[rel] = true
		}
	}
	actualContracts, err := regularFiles(filepath.Join(caseRoot, ".steamai-vnext", "contracts"), nil)
	if err != nil {
		return err
	}
	if len(actualContracts) != len(expectedContracts) {
		return errors.New("current contracts 路径集合漂移")
	}
	for _, rel := range actualContracts {
		if !expectedContracts[rel] {
			return fmt.Errorf("current contracts 含未声明文件 %s", rel)
		}
	}
	if !seen[".claude/skills/steamai/SKILL.md"] || !expectedContracts["learning-feedback.md"] {
		return errors.New("current immutable files 缺少 canonical skill 或 learning contract")
	}
	return nil
}

func cleanRelativePath(path string) (string, error) {
	if path == "" || strings.Contains(path, "\\") || filepath.IsAbs(filepath.FromSlash(path)) {
		return "", errors.New("相对路径无效")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("相对路径越界")
	}
	return clean, nil
}

func regularFiles(root string, excluded []string) ([]string, error) {
	exclude := map[string]bool{}
	for _, rel := range excluded {
		exclude[filepath.ToSlash(rel)] = true
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := rejectReparse(path); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("current tree 含非普通文件")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !exclude[rel] {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func parseSnapshot(path string) (snapshotMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshotMetadata{}, err
	}
	return parseSnapshotBytes(data)
}

func parseSnapshotBytes(data []byte) (snapshotMetadata, error) {
	var err error
	lines := strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var metadata snapshotMetadata
	packFields := map[string]bool{}
	section := ""
	var source *SourceRecord
	var immutable *PlannedWrite
	flush := func() {
		if source != nil {
			metadata.Files = append(metadata.Files, *source)
			source = nil
		}
		if immutable != nil {
			metadata.ImmutableFiles = append(metadata.ImmutableFiles, *immutable)
			immutable = nil
		}
	}
	for line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			flush()
			key, value, ok := strings.Cut(line, ": ")
			if !ok {
				if line == "files:" {
					section = "files"
					continue
				}
				if line == "immutable-files:" {
					section = "immutable"
					continue
				}
				return snapshotMetadata{}, fmt.Errorf("snapshot 顶层行无效: %s", line)
			}
			if key == "pack" || key == "pack-tree" || key == "aux-pack" || key == "aux-pack-tree" {
				if packFields[key] || strings.TrimSpace(value) == "" {
					return snapshotMetadata{}, fmt.Errorf("snapshot %s 为空或重复", key)
				}
				packFields[key] = true
			}
			switch key {
			case "schema":
				metadata.Schema = value
			case "pack":
				metadata.Pack = value
			case "revision":
				metadata.Revision = value
			case "pack-tree":
				metadata.PackTree = value
			case "aux-pack":
				metadata.AuxPack = value
			case "aux-pack-tree":
				metadata.AuxPackTree = value
			case "common-tree":
				metadata.CommonTree = value
			case "source-digest":
				metadata.SourceDigest = value
			case "payload-digest":
				metadata.PayloadDigest = value
			default:
				return snapshotMetadata{}, fmt.Errorf("snapshot 顶层字段未知: %s", key)
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "  - path: "); ok {
			flush()
			switch section {
			case "files":
				source = &SourceRecord{Path: value}
			case "immutable":
				immutable = &PlannedWrite{TargetPath: value}
			default:
				return snapshotMetadata{}, errors.New("snapshot record 位于未知 section")
			}
			continue
		}
		// 新增 tracked 文件没有 HEAD blob；先保留分隔符，再规范化值。
		key, value, ok := strings.Cut(strings.TrimLeft(line, " \t"), ": ")
		if !ok {
			return snapshotMetadata{}, fmt.Errorf("snapshot record 行无效: %s", line)
		}
		value = strings.TrimSpace(value)
		switch {
		case source != nil:
			switch key {
			case "git-mode":
				source.GitMode = value
			case "head-blob":
				source.HeadBlob = value
			case "content-blob":
				source.ContentBlob = value
			case "sha256":
				source.SHA256 = value
			case "bytes":
				source.Bytes, err = strconv.Atoi(value)
			default:
				return snapshotMetadata{}, fmt.Errorf("snapshot file 字段未知: %s", key)
			}
		case immutable != nil:
			switch key {
			case "sha256":
				immutable.SHA256 = value
			case "bytes":
				immutable.Bytes, err = strconv.Atoi(value)
			default:
				return snapshotMetadata{}, fmt.Errorf("immutable file 字段未知: %s", key)
			}
		default:
			return snapshotMetadata{}, errors.New("snapshot record 字段没有 record")
		}
		if err != nil {
			return snapshotMetadata{}, err
		}
	}
	flush()
	if err := validatePackSelection(metadata.Pack, metadata.AuxPack); err != nil {
		return snapshotMetadata{}, err
	}
	if packFields["aux-pack"] != packFields["aux-pack-tree"] ||
		(packFields["aux-pack-tree"] && !hexIdentityPattern.MatchString(metadata.AuxPackTree)) {
		return snapshotMetadata{}, errors.New("snapshot auxiliary pack identity 缺半或无效")
	}
	if len(metadata.Files) == 0 || len(metadata.ImmutableFiles) == 0 {
		return snapshotMetadata{}, errors.New("snapshot records 为空")
	}
	if !sort.SliceIsSorted(metadata.Files, func(i, j int) bool { return metadata.Files[i].Path < metadata.Files[j].Path }) ||
		!sort.SliceIsSorted(metadata.ImmutableFiles, func(i, j int) bool {
			return metadata.ImmutableFiles[i].TargetPath < metadata.ImmutableFiles[j].TargetPath
		}) {
		return snapshotMetadata{}, errors.New("snapshot records 未按路径排序")
	}
	for _, record := range metadata.Files {
		if (record.GitMode != "100644" && record.GitMode != "100755") || (record.HeadBlob != "" && !hexIdentityPattern.MatchString(record.HeadBlob)) || !hexIdentityPattern.MatchString(record.ContentBlob) || !shaPattern.MatchString(record.SHA256) || record.Bytes < 0 {
			return snapshotMetadata{}, errors.New("snapshot file record identity 无效")
		}
	}
	return metadata, nil
}
