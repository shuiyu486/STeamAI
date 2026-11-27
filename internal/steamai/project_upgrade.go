package steamai

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
)

func (a *app) projectRoot() (string, error) {
	root, err := a.cwd()
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	for part := range strings.SplitSeq(root, string(filepath.Separator)) {
		if strings.EqualFold(part, ".steamai-vnext") {
			return "", errors.New("请在原项目根目录运行，不要在 .steamai-vnext 内运行；成员请使用自己的会话")
		}
	}
	return root, nil
}

func (a *app) checkProject(args []string) error {
	if len(args) != 0 {
		return errors.New("check 只检查当前目录，不接受参数")
	}
	root, err := a.projectRoot()
	if err != nil {
		return err
	}
	state, id, err := inspectProject(root)
	if err != nil {
		return err
	}
	if state == caseFresh {
		return errors.New("当前目录没有 .steamai-vnext，不是已建项目；未初始化或改动任何文件")
	}
	aux := id.AuxPack
	if aux == "" {
		aux = "无"
	}
	_, err = fmt.Fprintf(a.stdout, "项目框架完整。主包：%s；辅助包：%s\n来源提交（历史锚）：%s\n固定框架来源摘要：%s\n固定包内容摘要：%s\n只检查了本项目文件，没有检查会话或业务程序。要与本机框架比较：steamai upgrade\n", id.Pack, aux, id.Revision, id.SourceDigest, id.PayloadDigest)
	return err
}

func (a *app) upgradeProject(args []string) error {
	apply := len(args) == 2 && args[0] == "--apply" && sha256Pattern.MatchString(args[1])
	if len(args) != 0 && !apply {
		return errors.New("用 steamai upgrade 预览；确认时原样运行预览末尾的 steamai upgrade --apply 命令")
	}
	root, err := a.projectRoot()
	if err != nil {
		return err
	}
	physicalID, err := a.platform.CaseIdentity(root)
	if err != nil {
		return err
	}
	if apply {
		lease, err := a.platform.AcquireCommander(commanderMutexName(physicalID))
		if errors.Is(err, errCommanderRunning) {
			return fmt.Errorf("先在原会话保存交接，关闭此项目的 Commander 和成员窗口，再应用；未更新：%w", err)
		}
		if err != nil {
			return err
		}
		defer lease.release()
		if err := a.applyProjectUpgrade(root, physicalID, args[1]); err != nil {
			return err
		}
		_, err = fmt.Fprintln(a.stdout, "此项目的框架已更新并通过完整性检查。回到原项目运行 steamai，按已保存的交接继续。")
		return err
	}
	preview, err := a.previewProjectUpgrade(root, physicalID)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(a.stdout, preview.CommandText())
	return err
}

func (a *app) upgradeSource(root string) (git, source string, err error) {
	source, err = a.platform.CanonicalSource()
	if err != nil {
		return "", "", errors.New("未找到本机安装绑定的框架来源")
	}
	if err := a.validateSource(source); err != nil {
		return "", "", err
	}
	if err := rejectCaseInsideSource(root, source); err != nil {
		return "", "", err
	}
	git, err = a.resolveNativeExecutable("git.exe")
	return git, source, err
}

func (a *app) previewProjectUpgrade(root, id string) (casebootstrap.UpgradePreview, error) {
	git, source, err := a.upgradeSource(root)
	if err != nil {
		return casebootstrap.UpgradePreview{}, err
	}
	return casebootstrap.PreviewUpgrade(git, source, root, id)
}

// 调用方持有本项目的 Commander 锁。等待用户选择时不占用来源锁；确认后重新解析来源与项目身份。
func (a *app) applyProjectUpgrade(root, id, confirmation string) error {
	git, source, err := a.upgradeSource(root)
	if err != nil {
		return err
	}
	lease, err := a.acquireCanonicalMutation(source)
	if err != nil {
		return err
	}
	defer lease.release()
	if now, err := a.platform.CaseIdentity(root); err != nil || now != id {
		return errors.New("项目目录已改变，未更新；请回到原项目重新打开")
	}
	return casebootstrap.ApplyUpgrade(git, source, root, id, confirmation)
}

// 用户可能在提示处停留。沿用旧版时，也不能从已被替换的目录或损坏的项目启动。
func (a *app) openPinnedVersion(root, id, reason string) error {
	if now, err := a.platform.CaseIdentity(root); err != nil || now != id {
		return errors.New("项目目录已改变，请回到原项目重新打开")
	}
	if _, err := casebootstrap.InspectCurrent(root); err != nil {
		return fmt.Errorf("%w：%s", errPartialCase, casebootstrap.CurrentIssue(err))
	}
	_, err := fmt.Fprintln(a.stdout, reason)
	return err
}

// 在 Claude 启动前处理选择，不改变原启动参数或授权目录。
func (a *app) offerProjectUpgrade(root, id string, members []casebootstrap.RosterMember) error {
	preview, err := a.previewProjectUpgrade(root, id)
	if err != nil {
		return a.openPinnedVersion(root, id, "本次未能比较本机框架，仍按项目已保存的版本打开。")
	}
	if preview.Confirmation == "" {
		return nil
	}
	if err := casebootstrap.CheckUpgradeMembers(root, members); err != nil {
		return a.openPinnedVersion(root, id, "本机有框架更新。成员目录被占用或无法确认空闲；本次沿用原版，下次打开再检查。")
	}
	yes, err := chooseUpgrade(a.stdin, a.stdout, preview)
	if err != nil {
		return err
	}
	if !yes {
		return a.openPinnedVersion(root, id, "本次沿用原版，正在打开项目。")
	}
	if err := a.applyProjectUpgrade(root, id, preview.Confirmation); err != nil {
		if !errors.Is(err, casebootstrap.ErrUpgradeMembersBusy) && !errors.Is(err, errCanonicalMutationRunning) {
			return err // 确认过期或发布失败不能当成已更新并启动。
		}
		return a.openPinnedVersion(root, id, "本次没有更新：成员目录或本机来源正在使用，仍按原版打开。")
	}
	_, err = fmt.Fprintln(a.stdout, "此项目的框架已更新并通过完整性检查。正在打开 Commander。")
	return err
}

func chooseUpgrade(in io.Reader, out io.Writer, preview casebootstrap.UpgradePreview) (bool, error) {
	if _, err := fmt.Fprintln(out, preview.Summary); err != nil {
		return false, err
	}
	for {
		if _, err := fmt.Fprintln(out, "输入 Y 更新并打开，直接回车沿用原版，D 看文件清单。如果其它程序正在改项目，请直接回车。"); err != nil {
			return false, err
		}
		// 不用缓冲读取器，避免预读并吞掉用户留给随后 Claude 会话的输入。
		var line strings.Builder
		var b [1]byte
		for {
			if _, err := io.ReadFull(in, b[:]); err != nil {
				return false, err // EOF（含不完整的 Y）或读取失败都不是确认。
			}
			if b[0] == '\n' {
				break
			}
			if line.Len() < 16 {
				line.WriteByte(b[0])
			}
		}
		switch strings.ToLower(strings.TrimSuffix(line.String(), "\r")) {
		case "y":
			return true, nil
		case "d":
			if _, err := fmt.Fprint(out, preview.Details); err != nil {
				return false, err
			}
		default:
			return false, nil
		}
	}
}
