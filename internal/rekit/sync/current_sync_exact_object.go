package sync

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type currentSyncObservedObject struct {
	State currentSyncExactObjectState
	Data  []byte
	Info  fs.FileInfo
}

func inspectCurrentSyncExpectedObject(
	root *rekitfs.AnchoredRoot,
	path string,
	expected currentSyncExpectedObject,
) (currentSyncObservedObject, error) {
	if root == nil {
		return currentSyncObservedObject{}, fmt.Errorf(
			"current sync exact object root is missing",
		)
	}
	clean, err := currentSyncNormalizeTargetRel(path)
	if err != nil || clean != path {
		return currentSyncObservedObject{}, fmt.Errorf(
			"current sync exact object path is invalid: %s",
			path,
		)
	}
	info, err := root.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return currentSyncObservedObject{State: currentSyncObjectAbsent}, nil
	}
	if err != nil {
		return currentSyncObservedObject{}, err
	}
	switch expected.Kind {
	case currentSyncExpectedFile:
		return inspectCurrentSyncExpectedFile(root, path, info, expected.File)
	case currentSyncExpectedTree:
		return inspectCurrentSyncExpectedTree(root, path, info, expected)
	default:
		return currentSyncObservedObject{}, fmt.Errorf(
			"current sync expected object kind is invalid: %s",
			expected.Kind,
		)
	}
}

func inspectCurrentSyncExpectedFile(
	root *rekitfs.AnchoredRoot,
	path string,
	info fs.FileInfo,
	binding *CurrentSyncBinding,
) (currentSyncObservedObject, error) {
	if binding == nil || !validCurrentSyncSHA(binding.SHA256) ||
		binding.Size < 0 || binding.Mode == 0 {
		return currentSyncObservedObject{}, fmt.Errorf(
			"current sync expected file binding is invalid: %s",
			path,
		)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Size() != binding.Size ||
		!currentSyncModeMatches(os.FileMode(binding.Mode), info.Mode()) {
		return currentSyncObservedObject{
			State: currentSyncObjectDrifted,
			Info:  info,
		}, nil
	}
	data, after, err := root.ReadStableFile(path, binding.Size)
	if err != nil {
		return currentSyncObservedObject{}, err
	}
	if currentSyncSHA(data) != strings.ToLower(binding.SHA256) ||
		int64(len(data)) != binding.Size ||
		!currentSyncModeMatches(os.FileMode(binding.Mode), after.Mode()) {
		return currentSyncObservedObject{
			State: currentSyncObjectDrifted,
			Info:  after,
		}, nil
	}
	return currentSyncObservedObject{
		State: currentSyncObjectExact,
		Data:  data,
		Info:  after,
	}, nil
}

func inspectCurrentSyncExpectedTree(
	root *rekitfs.AnchoredRoot,
	path string,
	info fs.FileInfo,
	expected currentSyncExpectedObject,
) (currentSyncObservedObject, error) {
	if expected.Inventory == nil || expected.BindingBasePath == "" {
		return currentSyncObservedObject{}, fmt.Errorf(
			"current sync expected tree binding is invalid: %s",
			path,
		)
	}
	if err := validateCurrentSyncInventory(*expected.Inventory); err != nil {
		return currentSyncObservedObject{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return currentSyncObservedObject{
			State: currentSyncObjectDrifted,
			Info:  info,
		}, nil
	}
	observed, exact, err := currentSyncObserveTree(
		root,
		path,
		expected.BindingBasePath,
		*expected.Inventory,
	)
	if err != nil {
		return currentSyncObservedObject{}, err
	}
	after, err := root.Lstat(path)
	if err != nil || !after.IsDir() || !os.SameFile(info, after) {
		return currentSyncObservedObject{}, fmt.Errorf(
			"current sync exact tree changed while validating: %s: %w",
			path,
			err,
		)
	}
	if !exact || !currentSyncCanonicalEqual(observed, *expected.Inventory) {
		return currentSyncObservedObject{
			State: currentSyncObjectDrifted,
			Info:  after,
		}, nil
	}
	return currentSyncObservedObject{
		State: currentSyncObjectExact,
		Info:  after,
	}, nil
}

func currentSyncObserveTree(
	root *rekitfs.AnchoredRoot,
	objectPath,
	bindingBasePath string,
	expected CurrentSyncInventory,
) (CurrentSyncInventory, bool, error) {
	expectedByPath := make(map[string]CurrentSyncBinding, len(expected.Entries))
	for _, binding := range expected.Entries {
		expectedByPath[strings.ToLower(binding.Path)] = binding
	}
	entries := make([]CurrentSyncBinding, 0, len(expected.Entries))
	seen := map[string]bool{}
	exact := true
	components := 0
	var walk func(string, bool) (int, error)
	walk = func(directory string, rootDirectory bool) (int, error) {
		listed, err := root.ListNoFollow(directory, currentSyncMaxFiles*4+1)
		if err != nil {
			return 0, err
		}
		files := 0
		for _, entry := range listed {
			components++
			if components > currentSyncMaxFiles*4 {
				return 0, fmt.Errorf(
					"current sync exact tree exceeds its component limit: %s",
					objectPath,
				)
			}
			child := filepath.ToSlash(filepath.Join(directory, entry.Name()))
			if entry.IsDir() {
				childFiles, err := walk(child, false)
				if err != nil {
					return 0, err
				}
				if childFiles == 0 {
					exact = false
				}
				files += childFiles
				if files > currentSyncMaxFiles {
					return 0, fmt.Errorf(
						"current sync exact tree exceeds %d files: %s",
						currentSyncMaxFiles,
						objectPath,
					)
				}
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return 0, err
			}
			if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				exact = false
				continue
			}
			files++
			if files > currentSyncMaxFiles {
				return 0, fmt.Errorf(
					"current sync exact tree exceeds %d files: %s",
					currentSyncMaxFiles,
					objectPath,
				)
			}
			relative := strings.TrimPrefix(child, objectPath+"/")
			if relative == child || relative == "" {
				exact = false
				continue
			}
			bindingPath := bindingBasePath + "/" + relative
			if clean, err := currentSyncNormalizeTargetRel(bindingPath); err != nil || clean != bindingPath {
				exact = false
				continue
			}
			key := strings.ToLower(bindingPath)
			binding, ok := expectedByPath[key]
			if !ok || binding.Path != bindingPath || seen[key] ||
				info.Size() != binding.Size ||
				!currentSyncModeMatches(os.FileMode(binding.Mode), info.Mode()) {
				exact = false
				continue
			}
			data, after, err := root.ReadStableFile(child, binding.Size)
			if err != nil {
				return 0, err
			}
			if currentSyncSHA(data) != strings.ToLower(binding.SHA256) ||
				int64(len(data)) != binding.Size ||
				!currentSyncModeMatches(os.FileMode(binding.Mode), after.Mode()) {
				exact = false
				continue
			}
			entries = append(entries, currentSyncBinding(
				binding.Path,
				binding.Kind,
				data,
				os.FileMode(binding.Mode),
			))
			seen[key] = true
		}
		if !rootDirectory && len(listed) == 0 {
			exact = false
		}
		return files, nil
	}
	if _, err := walk(objectPath, true); err != nil {
		return CurrentSyncInventory{}, false, err
	}
	observed, err := currentSyncInventory(entries)
	if err != nil {
		return CurrentSyncInventory{}, false, err
	}
	return observed, exact && len(seen) == len(expectedByPath), nil
}
