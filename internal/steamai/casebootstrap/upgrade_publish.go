package casebootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// upgradePendingDir 是升级期间的固定保留目录（位于 .steamai-vnext 下）：发布时移出的原件、尚未
// 放入项目的新内容、清单与手动处理说明都放在这里。正常结束（成功或完整撤回）时删除；进程异常
// 中断时它会留下，公共深验据此 fail-closed。预先存在时本模块既不复用也不删除。
const upgradePendingDir = ".upgrade-pending"

const upgradeManifestSchema = "steamai-upgrade-pending-v1"

var (
	ErrUpgradeInvalid   = errors.New("升级变更列表无效")
	ErrUpgradePending   = errors.New("上次升级未收尾，保留目录仍在")
	ErrUpgradePreserved = errors.New("升级保留目录仍在")
)

// upgradeStepHook 仅供测试：每个目标发布前调用，用来观察发布顺序或注入并发改动。
var upgradeStepHook func(index int, rel string)

// upgradeFile 是一个目标文件的全部变更。Before/After 为不带前缀的小写 SHA-256，
// 空串表示该状态下文件不存在；Data 是发布后的完整内容。
type upgradeFile struct {
	Path   string
	Before string
	After  string
	Data   []byte
}

type upgradeChange struct {
	upgradeFile
	action string // create / replace / delete
}

type upgradeManifest struct {
	Schema  string                  `json:"schema"`
	ID      string                  `json:"id"`
	Changes []upgradeManifestChange `json:"changes"`
}

type upgradeManifestChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Before string `json:"before"`
	After  string `json:"after"`
}

const upgradePendingREADME = `本目录由 STeamAI 升级在未正常结束时留下；正常结束（成功或完整撤回）时会自动删除。
它存在期间 STeamAI 不把本项目当作可用的已建项目。

内容：
- manifest.json：本次升级计划。每个条目给出项目内的相对路径、动作（create/replace/delete）、
  升级前 before 与升级后 after 的 SHA-256；空值表示该状态下文件不存在（delete 条目没有 after，
  create 条目没有 before）。
- backup/<路径>：发布时从项目里移出来的原件（被替换或删除的文件），SHA-256 应等于 before。
- new/<路径>：尚未放入项目、或撤回时收回的新内容，SHA-256 应等于 after；delete 条目没有 new。
- failure.txt（如有）：本次失败的原因与未能撤回的条目。

手动处理（没有自动修复命令）：
1. 保存并关闭本项目的会话和其它写者。先把整个项目（含本目录）备份到别处，保留原件。
2. 逐项核对项目文件、backup 和 new 中的 SHA-256。与目标版本相符的条目不用动。
   回到旧版：仅将确认等于 after 的新文件另存移走；仅当原位置已空、备份等于 before 时才把原件放回。
   保留新版：逐项确认项目文件等于 after；after 为空时，该文件应不存在。
   遇到未列出的改动、摘要不符、所需原件缺失或清单不全就停下。不要覆盖或删除不确定的文件。
3. 完成核对后，将整个保留目录移到项目外的空位置，不要删除它。回到项目根运行 steamai check；
   若检查失败，不要启动项目或再次升级，保留备份处理。
`

// validUpgradePath 只做字符串检查：slash 分隔的相对路径，落在允许发布的框架文件内，且每个
// 组件在 Windows 上不会被静默改写或指向别名（ADS 冒号、尾部点或空格、保留设备名、通配/控制字符）。
func validUpgradePath(rel string) bool {
	if rel == "" || strings.Contains(rel, "\\") || filepath.IsAbs(filepath.FromSlash(rel)) {
		return false
	}
	for part := range strings.SplitSeq(rel, "/") {
		if !upgradeSafeComponent(part) {
			return false
		}
	}
	return rel == ".claude/skills/steamai/SKILL.md" || rel == caseMarkerRel ||
		strings.HasPrefix(rel, ".steamai-vnext/contracts/") || strings.HasPrefix(rel, ".steamai-vnext/pack-snapshot/")
}

func upgradeSafeComponent(part string) bool {
	if part == "" || part == "." || part == ".." || strings.TrimSpace(part) != part || strings.HasSuffix(part, ".") {
		return false
	}
	for _, r := range part {
		if r < 0x20 || strings.ContainsRune(`<>:"|?*\`, r) {
			return false
		}
	}
	// Windows 拿第一个点之前的名字与设备名比较（NUL.txt、COM¹ 都算）。
	base, _, _ := strings.Cut(part, ".")
	base = strings.ToLower(upgradeSuperscriptDigits.Replace(base))
	return !windowsReservedName(base) && base != "com0" && base != "lpt0" && base != "conin$" && base != "conout$"
}

var upgradeSuperscriptDigits = strings.NewReplacer("¹", "1", "²", "2", "³", "3")

// buildUpgradeOrder 校验变更列表并固定发布顺序：普通文件在前，snapshot.yml 其次，项目说明若在列
// 则最后发布。同一 HEAD 下只改 skill 或 contract 时项目说明可以不变，因此不强制它在列。
func buildUpgradeOrder(changes []upgradeFile) ([]upgradeChange, error) {
	if len(changes) == 0 {
		return nil, fmt.Errorf("%w: 变更列表为空", ErrUpgradeInvalid)
	}
	ordered := make([]upgradeChange, 0, len(changes))
	seen := make(map[string]bool, len(changes))
	var snapshot, marker *upgradeChange
	for _, change := range changes {
		if !validUpgradePath(change.Path) {
			return nil, fmt.Errorf("%w: %s 不在允许发布的路径内或不是 Windows 安全的相对路径", ErrUpgradeInvalid, change.Path)
		}
		key := strings.ToLower(change.Path)
		if seen[key] {
			return nil, fmt.Errorf("%w: %s 重复", ErrUpgradeInvalid, change.Path)
		}
		seen[key] = true
		if change.Before == change.After {
			return nil, fmt.Errorf("%w: %s 没有前后变化", ErrUpgradeInvalid, change.Path)
		}
		for _, digest := range []string{change.Before, change.After} {
			if digest != "" && !shaPattern.MatchString(digest) {
				return nil, fmt.Errorf("%w: %s 的 SHA-256 不是小写十六进制", ErrUpgradeInvalid, change.Path)
			}
		}
		planned := upgradeChange{upgradeFile: change, action: "replace"}
		switch {
		case change.After == "":
			if len(change.Data) != 0 {
				return nil, fmt.Errorf("%w: %s 删除时不应携带内容", ErrUpgradeInvalid, change.Path)
			}
			planned.action = "delete"
		case hashBytes(change.Data) != change.After:
			return nil, fmt.Errorf("%w: %s 的内容与 after 不一致", ErrUpgradeInvalid, change.Path)
		case change.Before == "":
			planned.action = "create"
		}
		switch change.Path {
		case snapshotRel:
			snapshot = &planned
		case caseMarkerRel:
			marker = &planned
		default:
			ordered = append(ordered, planned)
		}
	}
	if snapshot == nil {
		return nil, fmt.Errorf("%w: 变更列表必须包含 %s", ErrUpgradeInvalid, snapshotRel)
	}
	ordered = append(ordered, *snapshot)
	if marker != nil {
		ordered = append(ordered, *marker)
	}
	return ordered, nil
}

// publishUpgrade 把已确认的变更发布到 root 下的项目。全程只有同卷 no-replace 改名，没有任何覆盖写：
//
//  1. 保留目录已存在（上次升级未收尾）就立即以 ErrUpgradePending 停止；否则零写入复核全部前态，
//     任何漂移直接停止。
//  2. 独占创建 .steamai-vnext/.upgrade-pending，写入说明、清单和每个新内容（写完 Sync 落盘并回读
//     核对）。此阶段项目文件不变。
//  3. 再整体复核一次前态，然后逐个发布（snapshot.yml 其次、项目说明最后）：已有文件先被移入
//     backup/<路径>，核对它仍是确认时的原件，不是就原样放回并停止；再把新内容 no-replace 移入
//     项目。所以确认后到的并发改动（包括项目说明里的交接）不会被盖掉。
//  4. 调用 verify 做新项目深验，此时保留目录仍在。verify 之后没有额外 Sync，只有删除保留目录。
//
// 第 2–4 步任何一步出错都逆序撤回自己移动过的文件：只有项目内文件仍等于 after 时才收回，原件
// 核对摘要后放回。完整撤回后删除保留目录并以原错误返回；有条目不能安全撤回时保留目录与原件、
// 写入 failure.txt 并返回 ErrUpgradePreserved。进程被硬中断时保留目录留下，由公共深验 fail-closed；
// 本模块不提供自动修复。
func publishUpgrade(root string, changes []upgradeFile, verify func() error) error {
	if verify == nil {
		return fmt.Errorf("%w: 必须提供发布后的深验回调", ErrUpgradeInvalid)
	}
	ordered, err := buildUpgradeOrder(changes)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	if err := requirePlainDirectory(root); err != nil {
		return fmt.Errorf("case 根目录无效: %w", err)
	}
	// 保留目录已在说明上次升级未收尾，这是比前态漂移更重要的信号：先于前态复核报告。
	pending := filepath.Join(root, ".steamai-vnext", upgradePendingDir)
	if present, err := exists(pending); err != nil {
		return err
	} else if present {
		return upgradePendingError(pending)
	}
	if err := verifyUpgradePreStates(root, ordered); err != nil {
		return err
	}
	run, err := newUpgradeRun(root, ordered)
	if err != nil {
		return err
	}
	return run.execute(verify)
}

type upgradeRun struct {
	root    string
	pending string
	id      string
	changes []upgradeChange
	touched []int    // 已移动过文件（原件已移出或新内容已放入）的变更下标，按顺序；撤回时逆序处理
	created []string // 发布时新建的父目录（slash 相对路径，由浅到深），撤回时逆序删除
}

func newUpgradeRun(root string, ordered []upgradeChange) (*upgradeRun, error) {
	pending := filepath.Join(root, ".steamai-vnext", upgradePendingDir)
	if err := requirePlainPath(root, filepath.Dir(pending), true); err != nil {
		return nil, fmt.Errorf("无法定位升级保留目录: %w", err)
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	// 独占创建：已存在（任何类型）都说明上次升级未收尾，不复用也不删除。
	if err := os.Mkdir(pending, 0o755); err != nil {
		if os.IsExist(err) {
			return nil, upgradePendingError(pending)
		}
		return nil, err
	}
	return &upgradeRun{root: root, pending: pending, id: hex.EncodeToString(nonce[:]), changes: ordered}, nil
}

func upgradePendingError(pending string) error {
	return fmt.Errorf("%w: %s；STeamAI 不会复用或删除它，请按其中 README.txt 手动处理后再试", ErrUpgradePending, pending)
}

func (run *upgradeRun) execute(verify func() error) error {
	if err := run.stage(); err != nil {
		// 未开始发布，也可能还没写出能证明目录归属的清单。
		return fmt.Errorf("%w: 准备升级文件失败，尚未发布任何变更；请保留 %s：%w", ErrUpgradePreserved, run.pending, err)
	}
	// staging 期间项目可能被其它写者改动：发布前再整体复核一遍前态。
	if err := verifyUpgradePreStates(run.root, run.changes); err != nil {
		return run.abort(err)
	}
	for index := range run.changes {
		if err := run.publishOne(index); err != nil {
			return run.abort(err)
		}
	}
	if err := verify(); err != nil {
		return run.abort(fmt.Errorf("更新后完整性检查未通过: %w", err))
	}
	return run.finish()
}

func (run *upgradeRun) abs(rel string) string {
	return filepath.Join(run.root, filepath.FromSlash(rel))
}

func (run *upgradeRun) pendingFile(kind, rel string) string {
	return filepath.Join(run.pending, kind, filepath.FromSlash(rel))
}

// stage 在保留目录内写好说明、清单和每个新内容（落盘并回读核对），并为原件备好空槽位。
// 发布只从这里同卷改名，所以这里完成之前项目文件不会被改动。
func (run *upgradeRun) stage() error {
	manifest := upgradeManifest{Schema: upgradeManifestSchema, ID: run.id}
	for _, change := range run.changes {
		manifest.Changes = append(manifest.Changes, upgradeManifestChange{
			Path: change.Path, Action: change.action, Before: change.Before, After: change.After,
		})
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := writeUpgradeStaged(filepath.Join(run.pending, "README.txt"), []byte(upgradePendingREADME)); err != nil {
		return err
	}
	if err := writeUpgradeStaged(filepath.Join(run.pending, "manifest.json"), append(encoded, '\n')); err != nil {
		return err
	}
	for index := range run.changes {
		change := &run.changes[index]
		if change.action != "create" {
			if err := os.MkdirAll(filepath.Dir(run.pendingFile("backup", change.Path)), 0o755); err != nil {
				return err
			}
		}
		if change.action != "delete" {
			if err := writeUpgradeStaged(run.pendingFile("new", change.Path), change.Data); err != nil {
				return err
			}
		}
	}
	return nil
}

// publishOne 再次核对这个目标的前态，按由浅到深的完整相对路径补齐缺少的父目录，然后发布：
// 已有文件先 no-replace 移入 backup 槽位并核对仍是确认时的原件，再把新内容 no-replace 移入项目。
func (run *upgradeRun) publishOne(index int) error {
	change := &run.changes[index]
	if upgradeStepHook != nil {
		upgradeStepHook(index, change.Path)
	}
	missing, err := upgradeTargetState(run.root, change, newUpgradeDirListings())
	if err != nil {
		return err
	}
	for _, dir := range missing {
		if err := os.Mkdir(run.abs(dir), 0o755); err != nil {
			return fmt.Errorf("创建 %s 的父目录 %s 失败: %w", change.Path, dir, err)
		}
		run.created = append(run.created, dir)
	}
	target, backup, fresh := run.abs(change.Path), run.pendingFile("backup", change.Path), run.pendingFile("new", change.Path)
	if change.action != "create" {
		moved, err := upgradeTakeOriginal(target, backup, change.Before, change.Path)
		if moved {
			run.touched = append(run.touched, index)
		}
		if err != nil {
			return err
		}
	}
	if change.action == "delete" {
		return nil
	}
	if err := upgradeMove(fresh, target); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s 在发布前被其它写入创建，未覆盖", ErrTargetDrift, change.Path)
		}
		return fmt.Errorf("发布 %s 失败: %w", change.Path, err)
	}
	if change.action == "create" {
		run.touched = append(run.touched, index)
	}
	return nil
}

// upgradeTakeOriginal 把此刻实际在 target 的文件同卷 no-replace 移入 backup 槽位，再核对它是否仍是
// 确认时的原件 want。不是就原样 no-replace 放回并按漂移拒绝；放不回则原件留在槽位（moved=true），
// 由撤回处理并保留。moved 表示原件此刻已不在项目里。
func upgradeTakeOriginal(target, backup, want, rel string) (moved bool, err error) {
	if err := upgradeMove(target, backup); err != nil {
		return false, fmt.Errorf("移出 %s 的现有文件失败: %w", rel, err)
	}
	if got, err := upgradeFileHash(backup); err == nil && got == want {
		return true, nil
	}
	if err := upgradeMove(backup, target); err != nil {
		return true, fmt.Errorf("%w: %s 的现有文件不是确认时的原件且无法放回，它保留在 %s", ErrUpgradePreserved, rel, backup)
	}
	return false, fmt.Errorf("%w: %s 的内容在确认后被改动，已原样放回，未发布", ErrTargetDrift, rel)
}

// abort 撤回本次已发布的全部变更。完整撤回后删除保留目录并以原错误返回；有任何条目不能安全
// 撤回时保留目录与原件、写入 failure.txt 并返回 ErrUpgradePreserved。
func (run *upgradeRun) abort(cause error) error {
	if err := run.checkPending(); err != nil {
		return fmt.Errorf("%w；未使用这个目录撤回或写入诊断；原因为 %v", err, cause)
	}
	leftovers := run.rollback()
	if len(leftovers) > 0 {
		run.writeFailure(cause, leftovers)
		return fmt.Errorf("%w: %v；未能撤回: %s；原件在 %s，新内容在 %s，见 README.txt 与 failure.txt",
			ErrUpgradePreserved, cause, strings.Join(leftovers, "；"), run.pendingFile("backup", ""), run.pendingFile("new", ""))
	}
	if err := os.RemoveAll(run.pending); err != nil {
		return fmt.Errorf("%w: %v；变更已全部撤回，但删除保留目录失败，确认项目完整后可手动删除 %s",
			ErrUpgradePreserved, cause, run.pending)
	}
	return fmt.Errorf("%w；变更已全部撤回，原文件保持不变", cause)
}

// rollback 逆序撤回自己移动过的文件和新建的父目录，返回需要人工处理的条目说明。
func (run *upgradeRun) rollback() []string {
	var leftovers []string
	for i := len(run.touched) - 1; i >= 0; i-- {
		if item, ok := run.rollbackOne(&run.changes[run.touched[i]]); !ok {
			leftovers = append(leftovers, item)
		}
	}
	for i := len(run.created) - 1; i >= 0; i-- {
		// 只删除自己新建、从根起每一级仍是普通目录且为空的目录；别人放入了文件或父目录被替换时保留。
		if requirePlainPath(run.root, run.abs(run.created[i]), true) == nil {
			_ = os.Remove(run.abs(run.created[i]))
		}
	}
	return leftovers
}

// rollbackOne 撤回一个变更。项目内文件等于 after 时先把这份新内容 no-replace 收回 new 槽位并核对，
// 再把核对过摘要的原件从 backup 槽位 no-replace 放回；任何一步对不上就停下并报告，不替换、不删除
// 别的写入。操作前确认从 case 根到目标每一级仍是普通目录/文件，不沿被替换的父目录写出 case。
func (run *upgradeRun) rollbackOne(change *upgradeChange) (string, bool) {
	target, backup, fresh := run.abs(change.Path), run.pendingFile("backup", change.Path), run.pendingFile("new", change.Path)
	digest, err := upgradeFileHash(target)
	if err != nil {
		return fmt.Sprintf("%s（无法读取，未改动）", change.Path), false
	}
	plain := requirePlainPath(run.root, target, false)
	if digest == "" {
		plain = requirePlainPath(run.root, filepath.Dir(target), true)
	}
	if plain != nil {
		return fmt.Sprintf("%s（从项目根到它的路径经过非普通目录，未改动）", change.Path), false
	}
	original, err := upgradeFileHash(backup)
	if err != nil {
		return fmt.Sprintf("%s（原件备份无法读取，未改动）", change.Path), false
	}
	if change.action == "replace" && digest == change.After && original != change.Before {
		// 原件备份不可用时不能先收回新内容，否则项目里会缺文件。
		return fmt.Sprintf("%s（原件备份不是确认时的原件，项目内的新内容未动）", change.Path), false
	}
	if change.action != "delete" && digest == change.After {
		if item, ok := upgradeWithdraw(target, fresh, change); !ok {
			return item, false
		}
		if change.action == "create" {
			return "", true
		}
		digest = ""
	}
	switch {
	case digest == change.Before:
		return "", true // 项目里已经是原件。
	case digest != "":
		return fmt.Sprintf("%s（项目内现在是 %s 而不是本次放入的内容，未覆盖）", change.Path, digest), false
	case original != change.Before:
		return fmt.Sprintf("%s（项目内缺失，%s 里的文件不是确认时的原件，未放回）", change.Path, backup), false
	}
	if err := upgradeMove(backup, target); err != nil {
		return fmt.Sprintf("%s（原件仍在 %s，放回失败）", change.Path, backup), false
	}
	return "", true
}

// upgradeWithdraw 把本次放入 target 的新内容 no-replace 收回 fresh 槽位并核对摘要；对不上就放回
// target 并报告。
func upgradeWithdraw(target, fresh string, change *upgradeChange) (string, bool) {
	if err := upgradeMove(target, fresh); err != nil {
		return fmt.Sprintf("%s（收回本次放入的新内容失败，未动）", change.Path), false
	}
	if got, err := upgradeFileHash(fresh); err == nil && got == change.After {
		return "", true
	}
	if err := upgradeMove(fresh, target); err != nil {
		return fmt.Sprintf("%s（内容在收回时又变了，现位于 %s，未能放回）", change.Path, fresh), false
	}
	return fmt.Sprintf("%s（内容在收回时又变了，已放回原位，未动）", change.Path), false
}

func (run *upgradeRun) writeFailure(cause error, leftovers []string) {
	var text strings.Builder
	fmt.Fprintf(&text, "STeamAI 升级失败，且未能安全撤回。\n时间: %s\n原因: %v\n", time.Now().Format(time.RFC3339), cause)
	text.WriteString("未撤回或需要人工判断的条目:\n")
	for _, item := range leftovers {
		fmt.Fprintf(&text, "  - %s\n", item)
	}
	text.WriteString("各目标当前状态（absent 表示文件不存在）:\n")
	for index := range run.changes {
		change := &run.changes[index]
		current, err := upgradeFileHash(run.abs(change.Path))
		if err != nil {
			current = "unreadable"
		}
		stored, _ := upgradeFileHash(run.pendingFile("backup", change.Path))
		fmt.Fprintf(&text, "  %s\taction=%s\tbefore=%s\tafter=%s\tcurrent=%s\tbackup=%s\n", change.Path, change.action,
			upgradeDescribe(change.Before), upgradeDescribe(change.After), upgradeDescribe(current), upgradeDescribe(stored))
	}
	text.WriteString("处理步骤见 README.txt；恢复完成前不要删除本目录。\n")
	_ = writeUpgradeStaged(filepath.Join(run.pending, "failure.txt"), []byte(text.String()))
}

// 撤回和清理只能操作本次创建的保留目录。
func (run *upgradeRun) checkPending() error {
	path := filepath.Join(run.pending, "manifest.json")
	if err := requirePlainPath(run.root, path, false); err == nil {
		data, err := os.ReadFile(path)
		var manifest upgradeManifest
		if err == nil && json.Unmarshal(data, &manifest) == nil && manifest.Schema == upgradeManifestSchema && manifest.ID == run.id {
			return nil
		}
	}
	return fmt.Errorf("%w: 不能确认 %s 仍是本次升级的保留目录，未清理", ErrUpgradePreserved, run.pending)
}

func (run *upgradeRun) finish() error {
	if err := run.checkPending(); err != nil {
		return err
	}
	if err := os.RemoveAll(run.pending); err != nil {
		return fmt.Errorf("%w: 升级已完成并通过完整性检查，但删除保留目录失败（%v）；确认后可手动删除 %s",
			ErrUpgradePreserved, err, run.pending)
	}
	return nil
}

func upgradeDescribe(digest string) string {
	if digest == "" {
		return "absent"
	}
	return digest
}

// writeUpgradeStaged 在保留目录内独占创建文件，写入、Sync 落盘、关闭后回读核对摘要。
func writeUpgradeStaged(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	got, err := upgradeFileHash(path)
	if err != nil {
		return err
	}
	if want := hashBytes(data); got != want {
		return fmt.Errorf("%s 回读摘要不符", path)
	}
	return nil
}

// upgradeFileHash 返回普通文件的内容摘要，不存在时返回空串；经过 symlink/junction/reparse 或
// 不是普通文件时返回错误而不是摘要。
func upgradeFileHash(path string) (string, error) {
	present, err := exists(path)
	if err != nil || !present {
		return "", err
	}
	if err := requirePlainFile(path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

// upgradeDirListings 缓存一次复核内读到的目录条目。Windows 会把短文件名、大小写变体、尾部点或
// 空格解析到同一个文件，Lstat/ReadFile 看不出来；目录枚举给出的才是磁盘上的真实名字。
type upgradeDirListings struct {
	entries map[string][]os.DirEntry
}

func newUpgradeDirListings() *upgradeDirListings {
	return &upgradeDirListings{entries: map[string][]os.DirEntry{}}
}

// exactEntry 报告 name 是否是 dir 下条目的真实名字；只以大小写不同的名字出现时按别名拒绝。
func (l *upgradeDirListings) exactEntry(dir, name, rel string) (bool, error) {
	entries, cached := l.entries[dir]
	if !cached {
		var err error
		entries, err = os.ReadDir(dir)
		if err != nil {
			return false, err
		}
		l.entries[dir] = entries
	}
	for _, entry := range entries {
		if entry.Name() == name {
			return true, nil
		}
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			return false, fmt.Errorf("%w: %s 用 %s 指向磁盘上的 %s（别名），未使用", ErrTargetDrift, rel, name, entry.Name())
		}
	}
	return false, nil
}

// upgradeTargetState 复核一个目标的前态：从 case 根到目标的每一级目录都存在、是普通目录、名字与
// 磁盘条目完全一致且不经过 reparse；目标存在时必须是普通文件且摘要等于 Before，不存在时 Before
// 必须为空。返回发布前还需新建的父目录（从根算起的 slash 相对完整路径，由浅到深）。
func upgradeTargetState(root string, change *upgradeChange, listings *upgradeDirListings) ([]string, error) {
	parts := strings.Split(change.Path, "/")
	current := root
	var missing []string
	for i, name := range parts[:len(parts)-1] {
		present, err := listings.exactEntry(current, name, change.Path)
		if err != nil {
			return nil, err
		}
		if !present {
			// 这一级不存在，更深的目录与目标必然都不存在。
			for j := i; j < len(parts)-1; j++ {
				missing = append(missing, strings.Join(parts[:j+1], "/"))
			}
			break
		}
		current = filepath.Join(current, name)
		if err := requirePlainDirectory(current); err != nil {
			return nil, fmt.Errorf("%w: %s 经过的 %s 不是普通目录: %v", ErrTargetDrift, change.Path, name, err)
		}
	}
	digest := ""
	if missing == nil {
		present, err := listings.exactEntry(current, parts[len(parts)-1], change.Path)
		if err != nil {
			return nil, err
		}
		if present {
			if digest, err = upgradeFileHash(filepath.Join(current, parts[len(parts)-1])); err != nil {
				return nil, fmt.Errorf("%w: %s 不是普通文件: %v", ErrTargetDrift, change.Path, err)
			}
		}
	}
	if digest != change.Before {
		return nil, fmt.Errorf("%w: %s 现在是 %s，确认时是 %s", ErrTargetDrift, change.Path, upgradeDescribe(digest), upgradeDescribe(change.Before))
	}
	return missing, nil
}

func verifyUpgradePreStates(root string, changes []upgradeChange) error {
	listings := newUpgradeDirListings()
	for index := range changes {
		if _, err := upgradeTargetState(root, &changes[index], listings); err != nil {
			return err
		}
	}
	return nil
}
