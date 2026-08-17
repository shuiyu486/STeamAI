package sync

import (
	"errors"
	"fmt"
	"os"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

func stageCurrentSyncReceipt(
	caseRoot string,
	intent currentSyncIntent,
	progress currentSyncProgress,
) (_ currentSyncReceipt, _ CurrentSyncBinding, retErr error) {
	if err := validateCurrentSyncIntentProject(
		intent,
		caseRoot,
		caseRootStateRoot(caseRoot),
	); err != nil {
		return currentSyncReceipt{}, CurrentSyncBinding{}, err
	}
	if progress.Pending == nil ||
		progress.Pending.Kind != "receipt-stage-to-live" {
		return currentSyncReceipt{}, CurrentSyncBinding{}, fmt.Errorf(
			"current sync receipt staging requires pending receipt publication",
		)
	}
	receipt, err := buildCurrentSyncReceipt(intent, progress)
	if err != nil {
		return currentSyncReceipt{}, CurrentSyncBinding{}, err
	}
	binding, data, err := currentSyncReceiptBinding(receipt)
	if err != nil {
		return currentSyncReceipt{}, CurrentSyncBinding{}, err
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRootStateRoot(caseRoot))
	if err != nil {
		return currentSyncReceipt{}, CurrentSyncBinding{}, err
	}
	defer func() {
		retErr = errors.Join(retErr, root.Close())
	}()
	if _, err := root.WriteExclusiveFileWriteThrough(
		currentSyncStagedReceiptPath(intent),
		data,
		0o600,
		true,
	); err != nil {
		return currentSyncReceipt{}, CurrentSyncBinding{}, fmt.Errorf(
			"stage current sync receipt: %w",
			err,
		)
	}
	staged, info, err := root.ReadStableFile(
		currentSyncStagedReceiptPath(intent),
		int64(len(data)),
	)
	if err != nil {
		return currentSyncReceipt{}, CurrentSyncBinding{}, err
	}
	if currentSyncSHA(staged) != strings.ToLower(binding.SHA256) ||
		int64(len(staged)) != binding.Size ||
		!currentSyncModeMatches(os.FileMode(binding.Mode), info.Mode()) {
		return currentSyncReceipt{}, CurrentSyncBinding{}, fmt.Errorf(
			"current sync staged receipt differs from its durable binding",
		)
	}
	if err := root.Validate(); err != nil {
		return currentSyncReceipt{}, CurrentSyncBinding{}, err
	}
	return receipt, binding, nil
}
