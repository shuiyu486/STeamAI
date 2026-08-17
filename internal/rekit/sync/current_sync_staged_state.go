package sync

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func validateCurrentSyncStagedState(
	caseRoot string,
	intent currentSyncIntent,
) (retErr error) {
	if err := validateCurrentSyncIntentProject(
		intent,
		caseRoot,
		caseRootStateRoot(caseRoot),
	); err != nil {
		return err
	}
	if err := validateCurrentSyncStagedBundle(caseRoot, intent); err != nil {
		return err
	}
	controlled, err := currentSyncStagedControlledInventory(caseRoot, intent)
	if err != nil {
		return err
	}
	if !currentSyncCanonicalEqual(controlled, intent.Plan.NextControlled) {
		return fmt.Errorf(
			"current sync staged controlled inventory differs from durable intent",
		)
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRootStateRoot(caseRoot))
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, root.Close())
	}()
	if err := validateCurrentSyncStagedLeaves(root, intent); err != nil {
		return err
	}
	return root.Validate()
}

func currentSyncStagedControlledInventory(
	caseRoot string,
	intent currentSyncIntent,
) (CurrentSyncInventory, error) {
	stageRoot, err := currentSyncStagedControlledRoot(caseRoot, intent)
	if err != nil {
		return CurrentSyncInventory{}, err
	}
	entries := []CurrentSyncBinding{}
	for _, root := range intent.Roots {
		paths, err := rekitfs.WalkRegularFilesAnchored(
			stageRoot,
			root.Name,
			"current sync staged controlled tree",
			currentSyncMaxFiles,
		)
		if err != nil {
			return CurrentSyncInventory{}, err
		}
		expected := make(map[string]CurrentSyncBinding, len(root.After.Entries))
		for _, binding := range root.After.Entries {
			expected[strings.ToLower(binding.Path)] = binding
		}
		seen := map[string]bool{}
		for _, path := range paths {
			rel, err := filepath.Rel(stageRoot, path)
			if err != nil {
				return CurrentSyncInventory{}, err
			}
			bindingPath := filepath.ToSlash(filepath.Join(
				projectstate.CurrentDir,
				rel,
			))
			key := strings.ToLower(bindingPath)
			binding, ok := expected[key]
			if !ok || binding.Path != bindingPath || seen[key] {
				return CurrentSyncInventory{}, fmt.Errorf(
					"current sync staged controlled tree contains an unplanned file: %s",
					bindingPath,
				)
			}
			entry, err := currentSyncLiveBinding(
				stageRoot,
				path,
				binding,
				"current sync staged controlled file",
			)
			if err != nil {
				return CurrentSyncInventory{}, err
			}
			entries = append(entries, entry)
			seen[key] = true
		}
		if len(seen) != len(expected) {
			return CurrentSyncInventory{}, fmt.Errorf(
				"current sync staged controlled tree is missing a planned file: %s",
				root.Name,
			)
		}
	}
	return currentSyncInventory(entries)
}

func validateCurrentSyncStagedLeaves(
	root *rekitfs.AnchoredRoot,
	intent currentSyncIntent,
) error {
	expectedPaths := []string{}
	for _, leaf := range intent.Leaves {
		if !leaf.Mutate || !leaf.AfterExists {
			continue
		}
		data, info, err := root.ReadStableFile(leaf.StagePath, leaf.AfterSize)
		if err != nil {
			return fmt.Errorf(
				"read current sync staged leaf %s: %w",
				leaf.Path,
				err,
			)
		}
		if currentSyncSHA(data) != strings.ToLower(leaf.AfterSHA256) ||
			int64(len(data)) != leaf.AfterSize ||
			!currentSyncModeMatches(os.FileMode(leaf.Mode), info.Mode()) {
			return fmt.Errorf(
				"current sync staged leaf differs from durable intent: %s",
				leaf.Path,
			)
		}
		expectedPaths = append(expectedPaths, leaf.StagePath)
	}
	leavesDir := intent.TransactionPath + "/stage/leaves"
	if len(expectedPaths) == 0 {
		if _, err := root.Lstat(leavesDir); errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		return fmt.Errorf("current sync staged leaves contain an unplanned directory")
	}
	actualPaths, err := root.ListRegularFilesNoFollow(
		leavesDir,
		currentSyncMaxFiles+1,
	)
	if err != nil {
		return err
	}
	sort.Strings(expectedPaths)
	sort.Strings(actualPaths)
	if !currentSyncCanonicalEqual(expectedPaths, actualPaths) {
		return fmt.Errorf("current sync staged leaves do not equal the durable intent")
	}
	return nil
}
