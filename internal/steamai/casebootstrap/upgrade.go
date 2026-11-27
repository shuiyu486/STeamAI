package casebootstrap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const (
	caseMarkerRel   = ".steamai-vnext/CLAUDE.md"
	snapshotRel     = ".steamai-vnext/pack-snapshot/snapshot.yml"
	caseTemplateRel = ".steamai-vnext/contracts/templates/case/CLAUDE.md"
)

type upgradePlan struct {
	old, next              snapshotMetadata
	changes                []upgradeFile
	members                []RosterMember
	id                     string
	manifestSHA, markerSHA string // 与本次确认绑定的发布后原始字节摘要。
}

// UpgradePreview 的摘要、详情和确认属于同一份计划。选择后仍由 ApplyUpgrade 重新核对。
type UpgradePreview struct {
	Summary, Details, Confirmation string
}

func (p UpgradePreview) CommandText() string {
	if p.Confirmation == "" {
		return p.Summary + "\n"
	}
	return p.Details + "要采用这份预览，请保存工作并关闭此项目的会话和其它写者，再在原项目根运行：\nsteamai upgrade --apply " + p.Confirmation + "\n"
}

func PreviewUpgrade(git, source, root, physicalID string) (UpgradePreview, error) {
	p, err := planUpgrade(git, source, root, physicalID)
	if err != nil {
		return UpgradePreview{}, err
	}
	if len(p.changes) == 0 {
		return UpgradePreview{Summary: "本项目用到的框架内容与本机一致；无需更新。"}, nil
	}
	var added, replaced, removed int
	var text strings.Builder
	fmt.Fprintf(&text, "只预览本项目，尚未写入。主包：%s", p.old.Pack)
	if p.old.AuxPack != "" {
		fmt.Fprintf(&text, "；辅助包：%s", p.old.AuxPack)
	}
	fmt.Fprintf(&text, "\n来源提交（历史锚）：%s → %s\n框架来源摘要：%s → %s\n", p.old.Revision, p.next.Revision, p.old.SourceDigest, p.next.SourceDigest)
	for _, f := range p.changes {
		action := "替换"
		switch {
		case f.Before == "":
			action = "新增"
			added++
		case f.After == "":
			action = "移除"
			removed++
		default:
			replaced++
		}
		fmt.Fprintf(&text, "%s  %s\n", action, f.Path)
	}
	text.WriteString("只更新框架文件。项目说明保留授权、团队、交接和自定义内容；现有成员任务、业务文件及历史记录不改。旧审查或经验若绑定旧快照，不自动转为新版有效。\n")
	return UpgradePreview{
		Summary: fmt.Sprintf("本机框架有更新：本项目的框架文件新增 %d、替换 %d、移除 %d。业务文件、交接和成员任务保留。", added, replaced, removed),
		Details: text.String(), Confirmation: p.id,
	}, nil
}

var ErrUpgradeMembersBusy = errors.New("已有成员工作目录被占用")

// CheckUpgradeMembers 是启动前的瞬时探测，不结束会话；用户选择更新时再检查一次。
func CheckUpgradeMembers(root string, members []RosterMember) error {
	release, err := holdUpgradeMemberDirs(root, members)
	if err == nil {
		release()
	}
	return err
}

func ApplyUpgrade(git, source, root, physicalID, confirmation string) error {
	p, err := planUpgrade(git, source, root, physicalID)
	if err != nil {
		return err
	}
	if len(p.changes) == 0 || p.id != confirmation {
		return errors.New("本次确认与项目或本机框架不匹配，未更新。请重新打开项目查看最新提示；手动更新时重新预览。")
	}
	release, err := holdUpgradeMemberDirs(root, p.members)
	if err != nil {
		return err
	}
	defer release()
	return publishUpgrade(root, p.changes, func() error {
		// pending 仍阻止项目启动；先放开目录句柄，让 Windows 允许深验遍历成员目录。
		release()
		var checked currentSnapshot
		if _, err := inspectCurrent(root, true, &checked); err != nil {
			return errors.New(CurrentIssue(err))
		}
		if hashBytes(checked.manifest) != p.manifestSHA || hashBytes(checked.marker) != p.markerSHA {
			return ErrTargetDrift
		}
		return nil
	})
}

func planUpgrade(git, source, root, physicalID string) (upgradePlan, error) {
	var p upgradePlan
	var checked currentSnapshot
	id, err := inspectCurrent(root, false, &checked)
	if err != nil {
		return p, fmt.Errorf("只能更新完整的已建项目，检查未通过：%s；不会用新版覆盖现状", CurrentIssue(err))
	}
	p.members = id.Roster
	frozen, err := freezeSource(git, source, id.Pack, id.AuxPack)
	if err != nil {
		return p, fmt.Errorf("无法准备本机框架的所选包：%w", err)
	}
	if frozen.Revision != id.Revision {
		if _, err := gitOutput(git, source, "merge-base", "--is-ancestor", id.Revision, frozen.Revision); err != nil {
			return p, errors.New("本机来源不能证明是项目来源提交的后续版本；不降级或跨分支覆盖")
		}
	}
	writes, err := frameworkWrites(frozen, id.Pack, id.AuxPack)
	if err != nil {
		return p, err
	}
	template := writeByTarget(writes, caseTemplateRel)
	if template.TargetPath == "" {
		return p, errors.New("本机框架缺少 case 模板")
	}
	payload := payloadDigest(snapshotPayloadWritesFromSource(frozen.Records, id.Pack, id.AuxPack))
	manifest := []byte(renderSnapshot(frozen, id.Pack, id.AuxPack, payload, immutableRecords(writes)))
	p.next, err = parseSnapshotBytes(manifest)
	if err != nil {
		return p, errors.New("本机框架生成的快照无法核验")
	}
	p.old = checked.metadata
	// 只有来源提交或暂存记录变化时，保留项目原锚点及其证据绑定；不发布新元数据。
	if p.old.PayloadDigest == p.next.PayloadDigest && slices.EqualFunc(p.old.ImmutableFiles, p.next.ImmutableFiles, func(a, b PlannedWrite) bool {
		return a.TargetPath == b.TargetPath && a.SHA256 == b.SHA256 && a.Bytes == b.Bytes
	}) {
		return p, nil
	}
	before := map[string]string{snapshotRel: hashBytes(checked.manifest)}
	for _, f := range p.old.Files {
		before[".steamai-vnext/pack-snapshot/"+f.Path] = f.SHA256
	}
	for _, f := range p.old.ImmutableFiles {
		before[f.TargetPath] = f.SHA256
	}
	var oldTemplate []byte
	if digest, ok := before[caseTemplateRel]; ok {
		path := filepath.Join(root, filepath.FromSlash(caseTemplateRel))
		if err := requirePlainPath(root, path, false); err != nil {
			return p, ErrTargetDrift
		}
		oldTemplate, err = os.ReadFile(path)
		if err != nil || hashBytes(oldTemplate) != digest {
			return p, ErrTargetDrift
		}
	}
	marker := checked.marker
	updated, err := upgradeMarker(marker, oldTemplate, template.Data, p.old, p.next)
	if err != nil {
		return p, err
	}
	before[caseMarkerRel] = hashBytes(marker)
	writes = append(writes, generatedWrite("snapshot", snapshotRel, manifest), generatedWrite("case", caseMarkerRel, updated))
	paths := map[string]string{}
	add := func(f upgradeFile) error {
		key := strings.ToLower(f.Path)
		if !validUpgradePath(f.Path) || paths[key] != "" {
			return errors.New("框架文件路径重复、冲突或不适用于 Windows；不更新")
		}
		paths[key] = f.Path
		if f.Before != f.After {
			p.changes = append(p.changes, f)
		}
		return nil
	}
	for _, w := range writes {
		if err := add(upgradeFile{Path: w.TargetPath, Before: before[w.TargetPath], After: w.SHA256, Data: w.Data}); err != nil {
			return p, err
		}
		delete(before, w.TargetPath)
	}
	for path, digest := range before {
		if err := add(upgradeFile{Path: path, Before: digest}); err != nil {
			return p, err
		}
	}
	sort.Slice(p.changes, func(i, j int) bool { return p.changes[i].Path < p.changes[j].Path })
	// 绑定本次来源、物理目录及旧/新完整快照和项目说明；即使两份工作树的 HEAD 相同也不能互用确认。
	p.manifestSHA, p.markerSHA = hashBytes(manifest), hashBytes(updated)
	p.id = hashBytes([]byte(strings.Join([]string{"steamai-upgrade-v1", physicalID, filepath.Clean(root), frozen.Root, hashBytes(checked.manifest), p.manifestSHA, hashBytes(marker), p.markerSHA}, "\x00")))
	return p, nil
}
