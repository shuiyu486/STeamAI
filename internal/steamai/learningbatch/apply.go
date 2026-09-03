package learningbatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
)

func Apply(git, source, caseRoot string, request Request, confirmation string) (Preview, error) {
	preview, err := BuildPreview(git, source, caseRoot, request)
	if err != nil {
		return Preview{}, err
	}
	if confirmation != ConfirmationPrefix+preview.Identity {
		return Preview{}, ErrConfirmationRequired
	}
	if err := applyPreview(git, source, caseRoot, preview); err != nil {
		return Preview{}, err
	}
	return preview, nil
}

func applyPreview(git, source, caseRoot string, preview Preview) error {
	beforeHead, beforeIndex, beforeSnapshot, err := immutableState(git, source, caseRoot)
	if err != nil {
		return err
	}
	beforeHead = strings.TrimSpace(beforeHead)
	if beforeHead != preview.CanonicalHead || beforeSnapshot != preview.SnapshotDigest {
		return ErrCaseDrift
	}
	for _, target := range preview.Targets {
		path := filepath.Join(source, filepath.FromSlash(target.Path))
		data, err := os.ReadFile(path)
		if err != nil || hashBytes(data) != target.PreSHA256 || len(data) != target.PreBytes {
			return ErrCanonicalDrift
		}
	}
	if _, err := gitInput(git, source, preview.patchData, "apply", "--check", "-"); err != nil {
		return fmt.Errorf("git apply --check 失败: %w", err)
	}
	if _, err := gitInput(git, source, preview.patchData, "apply", "-"); err != nil {
		if rollbackErr := rollbackTargets(source, preview); rollbackErr != nil {
			return fmt.Errorf("git apply 失败且 rollback 失败: %v; rollback: %w", err, rollbackErr)
		}
		return err
	}
	if err := verifyApplied(preview, source); err != nil {
		if rollbackErr := rollbackTargets(source, preview); rollbackErr != nil {
			return fmt.Errorf("应用后验证失败且 rollback 失败: %v; rollback: %w", err, rollbackErr)
		}
		return err
	}
	afterHead, afterIndex, afterSnapshot, err := immutableState(git, source, caseRoot)
	afterHead = strings.TrimSpace(afterHead)
	if err != nil || afterHead != preview.CanonicalHead || afterIndex != beforeIndex || afterSnapshot != preview.SnapshotDigest {
		rollbackErr := rollbackTargets(source, preview)
		if rollbackErr != nil {
			return fmt.Errorf("应用改变了 HEAD/index/case snapshot 且 rollback 失败: %v", rollbackErr)
		}
		return errors.New("learning batch 不得改变 HEAD、index 或当前 case snapshot")
	}
	return nil
}

func immutableState(git, source, caseRoot string) (head, index, snapshot string, err error) {
	head, err = gitOutput(git, source, "rev-parse", "HEAD")
	if err != nil {
		return "", "", "", err
	}
	index, err = gitOutput(git, source, "write-tree")
	if err != nil {
		return "", "", "", err
	}
	identity, err := inspectCaseIdentity(caseRoot)
	if err != nil {
		return "", "", "", err
	}
	return head, index, identity, nil
}

func inspectCaseIdentity(caseRoot string) (string, error) {
	identity, err := casebootstrap.InspectCurrent(caseRoot)
	if err != nil {
		return "", err
	}
	return identity.PayloadDigest, nil
}

func verifyApplied(preview Preview, source string) error {
	for _, target := range preview.Targets {
		path := filepath.Join(source, filepath.FromSlash(target.Path))
		data, err := os.ReadFile(path)
		if err != nil || hashBytes(data) != target.PostSHA256 || len(data) != target.PostBytes {
			return errors.New("learning batch postimage 与预览不一致")
		}
	}
	return nil
}

func rollbackTargets(source string, preview Preview) error {
	var rollbackErr error
	for _, target := range preview.Targets {
		data, ok := preview.targetData[target.Path]
		if !ok {
			continue
		}
		path := filepath.Join(source, filepath.FromSlash(target.Path))
		info, statErr := os.Stat(path)
		if statErr != nil {
			if rollbackErr == nil {
				rollbackErr = statErr
			}
			continue
		}
		if err := os.WriteFile(path, data, info.Mode().Perm()); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
	}
	return rollbackErr
}
