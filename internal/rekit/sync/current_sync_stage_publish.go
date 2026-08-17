package sync

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

func stageCurrentSyncMaterial(
	caseRoot string,
	intent currentSyncIntent,
	fresh CurrentSyncPlan,
) (retErr error) {
	stateRoot := caseRootStateRoot(caseRoot)
	if err := validateCurrentSyncIntentProject(
		intent,
		caseRoot,
		stateRoot,
	); err != nil {
		return err
	}
	files, err := currentSyncStageMaterial(intent, fresh)
	if err != nil {
		return err
	}
	root, err := rekitfs.OpenAnchoredRoot(stateRoot)
	if err != nil {
		return fmt.Errorf("open current sync staging root: %w", err)
	}
	defer func() {
		retErr = errors.Join(retErr, root.Close())
	}()
	for _, directory := range []string{
		intent.TransactionPath + "/stage/controlled",
		intent.TransactionPath + "/previous",
		intent.TransactionPath + "/previous/leaves",
	} {
		if err := root.MkdirAllNoFollow(directory, 0o700); err != nil {
			return err
		}
	}
	for _, binding := range intent.Roots {
		if err := root.MkdirAllNoFollow(binding.StagePath, 0o700); err != nil {
			return err
		}
	}
	for _, file := range files {
		if err := root.MkdirAllNoFollow(
			filepath.ToSlash(filepath.Dir(filepath.FromSlash(file.Path))),
			0o700,
		); err != nil {
			return err
		}
		if _, err := root.WriteExclusiveFileWriteThrough(
			file.Path,
			file.Data,
			fs.FileMode(file.Mode),
			true,
		); err != nil {
			return fmt.Errorf("stage current sync file %s: %w", file.Path, err)
		}
	}
	if err := validateCurrentSyncStagedMaterial(root, intent, files); err != nil {
		return err
	}
	if err := validateCurrentSyncStagedBundle(caseRoot, intent); err != nil {
		return err
	}
	return root.Validate()
}

func validateCurrentSyncStagedMaterial(
	root *rekitfs.AnchoredRoot,
	intent currentSyncIntent,
	files []currentSyncStagedFile,
) error {
	if root == nil {
		return fmt.Errorf("current sync staged material root is missing")
	}
	expectedLeaves := []string{}
	leafPrefix := intent.TransactionPath + "/stage/leaves/"
	for _, file := range files {
		if !strings.HasPrefix(file.Path, leafPrefix) {
			continue
		}
		data, info, err := root.ReadStableFile(
			file.Path,
			int64(len(file.Data)),
		)
		if err != nil || !currentSyncExactStagedFile(file, data, info.Mode()) {
			return fmt.Errorf(
				"current sync staged leaf differs from durable intent: %s: %w",
				file.Path,
				err,
			)
		}
		expectedLeaves = append(expectedLeaves, file.Path)
	}
	if len(expectedLeaves) == 0 {
		if _, err := root.Lstat(intent.TransactionPath + "/stage/leaves"); errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		return fmt.Errorf("current sync staged leaves contain an unplanned directory")
	}
	actualLeaves, err := root.ListRegularFilesNoFollow(
		intent.TransactionPath+"/stage/leaves",
		currentSyncMaxFiles+1,
	)
	if err != nil {
		return err
	}
	sort.Strings(expectedLeaves)
	sort.Strings(actualLeaves)
	if !currentSyncCanonicalEqual(expectedLeaves, actualLeaves) {
		return fmt.Errorf(
			"current sync staged leaves do not equal the durable intent",
		)
	}
	return nil
}

func currentSyncExactStagedFile(
	file currentSyncStagedFile,
	data []byte,
	mode fs.FileMode,
) bool {
	return currentSyncSHA(data) == currentSyncSHA(file.Data) &&
		int64(len(data)) == int64(len(file.Data)) &&
		currentSyncModeMatches(fs.FileMode(file.Mode), mode)
}
