package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

const (
	CurrentSyncRecoveryRestore = "restore-transaction"
	CurrentSyncRecoveryResume  = "resume-transaction"
	CurrentSyncRecoveryCleanup = "finish-cleanup"
	CurrentSyncRecoveryInvalid = "manual-repair-required"
)

// CurrentSyncRecovery describes one validated durable maintenance transaction
// that must be resolved before normal project work can continue.
type CurrentSyncRecovery struct {
	SchemaVersion int      `json:"schemaVersion"`
	Kind          string   `json:"kind"`
	State         string   `json:"state"`
	Pack          string   `json:"pack"`
	Pending       bool     `json:"pending"`
	Blocked       bool     `json:"blocked"`
	Recoverable   bool     `json:"recoverable"`
	Now           string   `json:"now"`
	Reason        string   `json:"reason"`
	Next          string   `json:"next"`
	ApplyArgs     []string `json:"applyArgs,omitempty"`
	Diagnostic    string   `json:"diagnostic,omitempty"`
}

// InspectCurrentSyncRecovery validates the current-sync namespace without
// consulting the active runtime bundle or the external source repository.
func InspectCurrentSyncRecovery(caseRoot string) (CurrentSyncRecovery, error) {
	caseFull, stateRoot, err := currentSyncRoots(caseRoot)
	if err != nil {
		return CurrentSyncRecovery{}, err
	}
	pending, err := inspectCurrentSyncPendingOwnership(caseFull, stateRoot.Path)
	if err != nil {
		return invalidCurrentSyncRecovery(err), nil
	}
	if !pending.Exists {
		return CurrentSyncRecovery{}, nil
	}
	snapshot, err := readCurrentSyncApplySnapshot(
		caseFull,
		stateRoot.Path,
		pending.Intent.PlanSHA256,
	)
	if err != nil {
		return invalidCurrentSyncRecovery(err), nil
	}
	state, now, reason, next, err := currentSyncRecoveryGuidance(snapshot.Route)
	if err != nil {
		return CurrentSyncRecovery{}, err
	}
	plan := pending.Intent.Plan
	return CurrentSyncRecovery{
		SchemaVersion: currentSyncSchemaVersion,
		Kind:          "steamai-current-sync-recovery",
		State:         state,
		Pack:          plan.Pack,
		Pending:       true,
		Blocked:       true,
		Recoverable:   true,
		Now:           now,
		Reason:        reason,
		Next:          next,
		ApplyArgs:     append([]string{}, plan.ApplyArgs...),
	}, nil
}

// ValidateCurrentSyncRecoveryExecutable permits a project-local process to
// reach only the recovery-aware front doors when ordinary bundle validation is
// unavailable during activation. The running bytes must match exactly one of
// the old or new executable identities bound by the durable reviewed plan.
func ValidateCurrentSyncRecoveryExecutable(caseRoot, executable string) error {
	caseFull, stateRoot, err := currentSyncRoots(caseRoot)
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(strings.TrimSpace(executable))
	if err != nil {
		return err
	}
	assetRoot, projectLocal, err := runtimebundle.AssetRootForExecutable(executable)
	if err != nil {
		return err
	}
	if !projectLocal || !rekitfs.SamePath(caseFull, filepath.Dir(assetRoot)) ||
		!rekitfs.SamePath(stateRoot.Path, assetRoot) {
		return fmt.Errorf(
			"current sync recovery executable does not belong to the exact target project: %s",
			executable,
		)
	}

	pending, err := inspectCurrentSyncPendingOwnership(caseFull, stateRoot.Path)
	if err != nil {
		return fmt.Errorf("current sync recovery ownership is invalid: %w", err)
	}
	if !pending.Exists {
		return fmt.Errorf("current sync recovery has no pending durable transaction")
	}
	if _, err := readCurrentSyncApplySnapshot(
		caseFull,
		stateRoot.Path,
		pending.Intent.PlanSHA256,
	); err != nil {
		return fmt.Errorf("current sync recovery transaction is invalid: %w", err)
	}

	bindings := []CurrentSyncBinding{}
	for _, inventory := range []CurrentSyncInventory{
		pending.Intent.Plan.CurrentControlled,
		pending.Intent.Plan.NextControlled,
	} {
		binding, ok := currentSyncRuntimeExecutableBinding(inventory)
		if !ok {
			return fmt.Errorf(
				"current sync recovery plan omits its runtime executable binding",
			)
		}
		if !strings.EqualFold(
			binding.Path,
			projectstate.CurrentDir+"/runtime/bin/"+runtimebundle.ExecutableName(),
		) {
			return fmt.Errorf(
				"current sync recovery executable binding has a non-canonical path: %s",
				binding.Path,
			)
		}
		bindings = append(bindings, binding)
	}
	return validateCurrentSyncRecoveryExecutableFile(
		caseFull,
		executable,
		bindings,
	)
}

func validateCurrentSyncRecoveryExecutableFile(
	caseRoot,
	executable string,
	bindings []CurrentSyncBinding,
) error {
	if err := rekitfs.ValidateNoReparseComponents(executable); err != nil {
		return fmt.Errorf("validate current sync recovery executable path: %w", err)
	}
	before, err := os.Lstat(executable)
	if err != nil {
		return fmt.Errorf(
			"stat current sync recovery executable %s: %w",
			executable,
			err,
		)
	}
	if before.Mode()&os.ModeSymlink != 0 ||
		!before.Mode().IsRegular() || before.Size() < 1 ||
		before.Size() > currentSyncMaxFileBytes {
		return fmt.Errorf(
			"current sync recovery executable must be a bounded regular file: %s",
			executable,
		)
	}
	data, err := currentSyncReadFile(
		caseRoot,
		executable,
		"current sync recovery executable",
		currentSyncMaxFileBytes,
		false,
	)
	if err != nil {
		return err
	}
	if err := rekitfs.ValidateNoReparseComponents(executable); err != nil {
		return fmt.Errorf("revalidate current sync recovery executable path: %w", err)
	}
	after, err := os.Lstat(executable)
	if err != nil {
		return fmt.Errorf(
			"re-stat current sync recovery executable %s: %w",
			executable,
			err,
		)
	}
	if !os.SameFile(before, after) ||
		after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() ||
		after.Size() != int64(len(data)) {
		return fmt.Errorf(
			"current sync recovery executable changed while validating: %s",
			executable,
		)
	}
	for _, binding := range bindings {
		if binding.Kind == "runtime-executable" &&
			binding.Size == int64(len(data)) &&
			strings.EqualFold(binding.SHA256, currentSyncSHA(data)) &&
			currentSyncModeMatches(os.FileMode(binding.Mode), after.Mode()) {
			return nil
		}
	}
	return fmt.Errorf(
		"current sync recovery executable bytes do not match the durable old or new runtime identity: %s",
		executable,
	)
}

func invalidCurrentSyncRecovery(cause error) CurrentSyncRecovery {
	diagnostic := ""
	if cause != nil {
		diagnostic = cause.Error()
	}
	return CurrentSyncRecovery{
		SchemaVersion: currentSyncSchemaVersion,
		Kind:          "steamai-current-sync-recovery",
		State:         CurrentSyncRecoveryInvalid,
		Pending:       true,
		Blocked:       true,
		Recoverable:   false,
		Now:           "STeamAI 更新记录不完整或彼此冲突，项目已安全停下。",
		Reason:        "系统无法唯一确认原更新的下一步，因此不会猜测、另起更新或启动 Claude。",
		Next:          "请让 STeamAI 检查项目更新记录并给出修复预览；不要手动删除或改写维护文件。",
		Diagnostic:    diagnostic,
	}
}

func currentSyncRecoveryGuidance(route currentSyncApplyRoute) (
	state,
	now,
	reason,
	next string,
	err error,
) {
	switch route {
	case currentSyncApplyRestoreActive:
		return CurrentSyncRecoveryRestore,
			"STeamAI 更新在写入执行进度前中断，项目已安全停下。",
			"已保存并验证原更新计划；系统不会另起一次更新，也不会启动 Claude。",
			"请让 STeamAI 使用原计划继续恢复；不需要手填任务、SHA 或事务信息。",
			nil
	case currentSyncApplyResume:
		return CurrentSyncRecoveryResume,
			"STeamAI 更新执行到一半后中断，项目已安全停下。",
			"更新日志和下一步均已验证；继续时只会推进原来的有界更新。",
			"请让 STeamAI 继续未完成的项目更新；不要开始新的同步。",
			nil
	case currentSyncApplyCleanup:
		return CurrentSyncRecoveryCleanup,
			"STeamAI 更新内容已经提交，只剩最后的安全收尾。",
			"提交收据和终态日志已匹配；在清理完成前不会启动 Claude。",
			"请让 STeamAI 完成更新收尾，然后重新查看状态。",
			nil
	default:
		return "", "", "", "", fmt.Errorf(
			"current sync pending ownership resolved to a non-recovery route: %s",
			route,
		)
	}
}
