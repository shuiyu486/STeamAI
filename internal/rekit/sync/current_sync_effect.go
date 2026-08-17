package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

func currentSyncOperationEffectFor(
	caseRoot string,
	intent currentSyncIntent,
) currentSyncOperationEffect {
	return func(
		progress currentSyncProgress,
		operation currentSyncProgressOperation,
	) error {
		mode, err := currentSyncModeForOperation(operation)
		if err != nil {
			return err
		}
		if mode == currentSyncOperationValidate {
			return executeCurrentSyncValidateOperation(
				caseRoot,
				intent,
				progress,
				operation,
			)
		}

		var stagedReceipt *CurrentSyncBinding
		if operation.Kind == "receipt-stage-to-live" {
			_, binding, err := stageCurrentSyncReceipt(
				caseRoot,
				intent,
				progress,
			)
			if err != nil {
				return err
			}
			stagedReceipt = &binding
		}
		spec, err := currentSyncRenameOperationSpec(
			intent,
			operation,
			stagedReceipt,
		)
		if err != nil {
			return err
		}
		return executeCurrentSyncRename(caseRoot, spec)
	}
}

func executeCurrentSyncRename(
	caseRoot string,
	spec currentSyncRenameSpec,
) (retErr error) {
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, root.Close())
	}()

	source, err := inspectCurrentSyncExpectedObject(
		root,
		spec.SourcePath,
		spec.Expected,
	)
	if err != nil {
		return err
	}
	destination, err := inspectCurrentSyncExpectedObject(
		root,
		spec.DestinationPath,
		spec.Expected,
	)
	if err != nil {
		return err
	}
	disposition, err := currentSyncClassifyRename(
		source.State,
		destination.State,
	)
	if err != nil {
		return err
	}
	if disposition == currentSyncRenameApply {
		switch spec.Expected.Kind {
		case currentSyncExpectedFile:
			if spec.Expected.File == nil {
				return fmt.Errorf("current sync rename file binding is missing")
			}
			if err := root.RenameFileNoReplaceExact(
				spec.SourcePath,
				spec.DestinationPath,
				source.Data,
				os.FileMode(spec.Expected.File.Mode),
			); err != nil {
				return err
			}
		case currentSyncExpectedTree:
			tree, err := currentSyncExpectedTreeForRename(
				root,
				spec.SourcePath,
				spec.Expected,
			)
			if err != nil {
				return err
			}
			if err := root.RenameDirectoryNoReplaceExact(
				spec.SourcePath,
				spec.DestinationPath,
				tree,
			); err != nil {
				return err
			}
		default:
			return fmt.Errorf(
				"current sync rename expected object kind is invalid: %s",
				spec.Expected.Kind,
			)
		}
	}

	source, err = inspectCurrentSyncExpectedObject(
		root,
		spec.SourcePath,
		spec.Expected,
	)
	if err != nil {
		return err
	}
	destination, err = inspectCurrentSyncExpectedObject(
		root,
		spec.DestinationPath,
		spec.Expected,
	)
	if err != nil {
		return err
	}
	if source.State != currentSyncObjectAbsent ||
		destination.State != currentSyncObjectExact {
		return fmt.Errorf(
			"current sync rename is not exact after effect: source=%s destination=%s",
			source.State,
			destination.State,
		)
	}
	return root.Validate()
}

func currentSyncExpectedTreeForRename(
	root *rekitfs.AnchoredRoot,
	sourcePath string,
	expected currentSyncExpectedObject,
) (rekitfs.ExpectedTree, error) {
	if root == nil || expected.Inventory == nil ||
		expected.BindingBasePath == "" {
		return rekitfs.ExpectedTree{}, fmt.Errorf(
			"current sync rename expected tree binding is missing",
		)
	}
	if err := validateCurrentSyncInventory(*expected.Inventory); err != nil {
		return rekitfs.ExpectedTree{}, err
	}
	rootInfo, err := root.Lstat(sourcePath)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return rekitfs.ExpectedTree{}, fmt.Errorf(
			"current sync rename source tree is invalid: %s: %w",
			sourcePath,
			err,
		)
	}
	tree := rekitfs.ExpectedTree{
		Directories: []rekitfs.ExpectedDirectory{
			rekitfs.NewExpectedDirectory(".", rootInfo.Mode()),
		},
	}
	seenDirectories := map[string]bool{".": true}
	for _, binding := range expected.Inventory.Entries {
		prefix := expected.BindingBasePath + "/"
		relative := strings.TrimPrefix(binding.Path, prefix)
		if relative == binding.Path || relative == "" {
			return rekitfs.ExpectedTree{}, fmt.Errorf(
				"current sync rename tree path is outside its binding base: %s",
				binding.Path,
			)
		}
		sourceFile := filepath.ToSlash(filepath.Join(sourcePath, relative))
		data, info, err := root.ReadStableFile(sourceFile, binding.Size)
		if err != nil || currentSyncSHA(data) != strings.ToLower(binding.SHA256) ||
			int64(len(data)) != binding.Size ||
			!currentSyncModeMatches(os.FileMode(binding.Mode), info.Mode()) {
			return rekitfs.ExpectedTree{}, fmt.Errorf(
				"current sync rename tree file differs from durable intent: %s: %w",
				binding.Path,
				err,
			)
		}
		for parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(relative))); parent != "."; parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent))) {
			if seenDirectories[parent] {
				continue
			}
			directoryInfo, err := root.Lstat(
				filepath.ToSlash(filepath.Join(sourcePath, parent)),
			)
			if err != nil || !directoryInfo.IsDir() ||
				directoryInfo.Mode()&os.ModeSymlink != 0 {
				return rekitfs.ExpectedTree{}, fmt.Errorf(
					"current sync rename tree directory is invalid: %s: %w",
					parent,
					err,
				)
			}
			seenDirectories[parent] = true
			tree.Directories = append(
				tree.Directories,
				rekitfs.NewExpectedDirectory(parent, directoryInfo.Mode()),
			)
		}
		tree.Files = append(
			tree.Files,
			rekitfs.NewExpectedFile(relative, data, os.FileMode(binding.Mode)),
		)
	}
	sort.Slice(tree.Directories, func(left, right int) bool {
		return tree.Directories[left].Path < tree.Directories[right].Path
	})
	sort.Slice(tree.Files, func(left, right int) bool {
		return tree.Files[left].Path < tree.Files[right].Path
	})
	return tree, nil
}
