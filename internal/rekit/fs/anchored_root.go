package fs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// AnchoredRoot exposes only the no-follow filesystem mutations needed to
// stage and swap a current sync transaction.
type AnchoredRoot struct {
	mu       sync.Mutex
	path     string
	root     *os.Root
	identity os.FileInfo
}

var errAnchoredExactMutationUnsupported = errors.New("exact handle-bound filesystem mutation is unsupported on this platform")

var anchoredRootBeforeRemoveHook func() error
var anchoredRootBeforeRenameHook func() error
var anchoredRootReadAfterOpenHook func() error
var anchoredRootExactFileAfterOpenHook func() error
var anchoredRootWriteBeforeFinalValidationHook func() error
var anchoredRootTreeAfterValidationHook func() error

//lint:ignore U1000 Windows-only adversarial tests exercise this hook.
func setAnchoredRootBeforeRemoveHookForTest(hook func() error) func() {
	previous := anchoredRootBeforeRemoveHook
	anchoredRootBeforeRemoveHook = hook
	return func() { anchoredRootBeforeRemoveHook = previous }
}

func setAnchoredRootBeforeRenameHookForTest(hook func() error) func() {
	previous := anchoredRootBeforeRenameHook
	anchoredRootBeforeRenameHook = hook
	return func() { anchoredRootBeforeRenameHook = previous }
}

func setAnchoredRootReadAfterOpenHookForTest(hook func() error) func() {
	previous := anchoredRootReadAfterOpenHook
	anchoredRootReadAfterOpenHook = hook
	return func() { anchoredRootReadAfterOpenHook = previous }
}

func setAnchoredRootExactFileAfterOpenHookForTest(hook func() error) func() {
	previous := anchoredRootExactFileAfterOpenHook
	anchoredRootExactFileAfterOpenHook = hook
	return func() { anchoredRootExactFileAfterOpenHook = previous }
}

//lint:ignore U1000 Windows-only adversarial tests exercise this hook.
func setAnchoredRootWriteBeforeFinalValidationHookForTest(hook func() error) func() {
	previous := anchoredRootWriteBeforeFinalValidationHook
	anchoredRootWriteBeforeFinalValidationHook = hook
	return func() { anchoredRootWriteBeforeFinalValidationHook = previous }
}

//lint:ignore U1000 Windows-only adversarial tests exercise this hook.
func setAnchoredRootTreeAfterValidationHookForTest(hook func() error) func() {
	previous := anchoredRootTreeAfterValidationHook
	anchoredRootTreeAfterValidationHook = hook
	return func() { anchoredRootTreeAfterValidationHook = previous }
}

// OpenAnchoredRoot pins an existing non-reparse directory to its lexical name.
func OpenAnchoredRoot(path string) (*AnchoredRoot, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("anchored root path is empty")
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	full = filepath.Clean(full)
	identity, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !identity.IsDir() || identity.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("anchored root must be a non-symlink directory: %s", full)
	}
	if err := rejectReparseAncestors(full); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(full)
	if err != nil {
		return nil, err
	}
	opened, err := root.Lstat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(identity, opened) {
		_ = root.Close()
		return nil, fmt.Errorf("anchored root changed while opening: %s: %w", full, err)
	}
	if err := rejectReparseAncestors(full); err != nil {
		_ = root.Close()
		return nil, err
	}
	return &AnchoredRoot{path: full, root: root, identity: opened}, nil
}

// Close releases the pinned root. It is safe to call more than once.
func (root *AnchoredRoot) Close() error {
	if root == nil {
		return nil
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.root == nil {
		return nil
	}
	opened := root.root
	root.root = nil
	return opened.Close()
}

// Validate proves that the pinned root still occupies its original lexical
// path and that none of its ancestors became a symlink or reparse point.
func (root *AnchoredRoot) Validate() error {
	if root == nil {
		return fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	return root.validateLocked()
}

// Lstat inspects one exact relative path after traversing every parent without
// following symlinks or reparse points.
func (root *AnchoredRoot) Lstat(rel string) (fs.FileInfo, error) {
	if root == nil {
		return nil, fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.validateLocked(); err != nil {
		return nil, err
	}
	clean, err := cleanAnchoredRel(rel, true)
	if err != nil {
		return nil, err
	}
	if clean == "." {
		if err := root.validateLocked(); err != nil {
			return nil, err
		}
		return root.identity, nil
	}
	parent, err := openDirectoryNoFollow(root.root, filepath.Dir(clean), root.path, "anchored lstat")
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	parentIdentity, err := parent.Lstat(".")
	if err != nil {
		return nil, err
	}
	info, err := parent.Lstat(filepath.Base(clean))
	if err != nil {
		return nil, err
	}
	if err := rejectReparsePath(filepath.Join(root.path, clean)); err != nil {
		return nil, err
	}
	if err := root.validateDirectoryBindingLocked(filepath.Dir(clean), parentIdentity, "anchored lstat parent"); err != nil {
		return nil, err
	}
	if err := root.validateLocked(); err != nil {
		return nil, err
	}
	return info, nil
}

// ReadStableFile returns the exact bytes and metadata of a bounded regular
// file while proving that its lexical name and the pinned root stay unchanged.
func (root *AnchoredRoot) ReadStableFile(rel string, limit int64) ([]byte, fs.FileInfo, error) {
	if root == nil {
		return nil, nil, fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if limit < 0 {
		return nil, nil, fmt.Errorf("anchored read limit must not be negative")
	}
	if err := root.validateLocked(); err != nil {
		return nil, nil, err
	}
	clean, err := cleanAnchoredRel(rel, false)
	if err != nil {
		return nil, nil, err
	}
	parent, err := openDirectoryNoFollow(root.root, filepath.Dir(clean), root.path, "anchored read")
	if err != nil {
		return nil, nil, err
	}
	defer parent.Close()
	parentIdentity, err := parent.Lstat(".")
	if err != nil {
		return nil, nil, err
	}
	name := filepath.Base(clean)
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > limit {
		return nil, nil, fmt.Errorf("anchored read target must be a bounded regular non-symlink file: %s", rel)
	}
	if err := rejectReparsePath(filepath.Join(root.path, clean)); err != nil {
		return nil, nil, err
	}
	file, err := openExactFile(parent, name, false)
	if err != nil {
		return nil, nil, err
	}
	if anchoredRootReadAfterOpenHook != nil {
		if err := anchoredRootReadAfterOpenHook(); err != nil {
			_ = file.Close()
			return nil, nil, fmt.Errorf("anchored read target changed: %s: %w", rel, err)
		}
	}
	data, opened, readErr := readExactFileData(file, limit)
	unbrokenErr := exactFileUnbroken(file)
	closeErr := file.Close()
	after, afterErr := parent.Lstat(name)
	if readErr != nil || afterErr != nil || unbrokenErr != nil || closeErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) ||
		before.Size() != opened.Size() || opened.Size() != after.Size() || before.Mode() != opened.Mode() || opened.Mode() != after.Mode() {
		return nil, nil, fmt.Errorf("anchored read target changed: %s: %w", rel, errors.Join(readErr, afterErr, unbrokenErr, closeErr))
	}
	if err := root.validateDirectoryBindingLocked(filepath.Dir(clean), parentIdentity, "anchored read parent"); err != nil {
		return nil, nil, err
	}
	if err := root.validateLocked(); err != nil {
		return nil, nil, err
	}
	return data, after, nil
}

// ListNoFollow lists one directory without descending. Every returned entry
// is validated no-follow and carries the exact metadata observed by the root.
func (root *AnchoredRoot) ListNoFollow(rel string, limit int) ([]fs.DirEntry, error) {
	if root == nil {
		return nil, fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if limit < 1 {
		return nil, fmt.Errorf("anchored list limit must be positive")
	}
	if err := root.validateLocked(); err != nil {
		return nil, err
	}
	clean, err := cleanAnchoredRel(rel, true)
	if err != nil {
		return nil, err
	}
	directory, err := openDirectoryNoFollow(root.root, clean, root.path, "anchored list")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	file, err := directory.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := file.ReadDir(limit + 1)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(entries) > limit {
		return nil, fmt.Errorf("anchored directory contains more than %d entries: %s", limit, rel)
	}
	validated := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := directory.Lstat(entry.Name())
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("anchored directory entry must not be a symlink or reparse point: %s: %w", filepath.Join(rel, entry.Name()), err)
		}
		if err := rejectReparsePath(filepath.Join(root.path, clean, entry.Name())); err != nil {
			return nil, err
		}
		validated = append(validated, fs.FileInfoToDirEntry(info))
	}
	sort.Slice(validated, func(left, right int) bool {
		leftName := strings.ToLower(validated[left].Name())
		rightName := strings.ToLower(validated[right].Name())
		if leftName != rightName {
			return leftName < rightName
		}
		return validated[left].Name() < validated[right].Name()
	})
	if err := root.finishDirectoryOperationLocked(clean, directory, "anchored list directory"); err != nil {
		return nil, err
	}
	return validated, nil
}

// ListRegularFilesNoFollow lists one directory without descending. Every
// returned entry is a regular non-symlink file and the result is name-sorted.
func (root *AnchoredRoot) ListRegularFilesNoFollow(rel string, limit int) ([]string, error) {
	if root == nil {
		return nil, fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if limit < 1 {
		return nil, fmt.Errorf("anchored list limit must be positive")
	}
	if err := root.validateLocked(); err != nil {
		return nil, err
	}
	clean, err := cleanAnchoredRel(rel, true)
	if err != nil {
		return nil, err
	}
	directory, err := openDirectoryNoFollow(root.root, clean, root.path, "anchored list")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	file, err := directory.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := file.ReadDir(limit + 1)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(entries) > limit {
		return nil, fmt.Errorf("anchored directory contains more than %d entries: %s", limit, rel)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		info, err := directory.Lstat(name)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("anchored directory entry must be a regular non-symlink file: %s: %w", filepath.Join(rel, name), err)
		}
		if err := rejectReparsePath(filepath.Join(root.path, clean, name)); err != nil {
			return nil, err
		}
		names = append(names, filepath.ToSlash(filepath.Join(clean, name)))
	}
	sortAnchoredNames(names)
	if err := root.finishDirectoryOperationLocked(clean, directory, "anchored regular-file list directory"); err != nil {
		return nil, err
	}
	return names, nil
}

// MkdirAllNoFollow creates only missing directories and rejects symlink or
// reparse components at every level.
func (root *AnchoredRoot) MkdirAllNoFollow(rel string, mode fs.FileMode) error {
	if root == nil {
		return fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.validateLocked(); err != nil {
		return err
	}
	clean, err := cleanAnchoredRel(rel, true)
	if err != nil {
		return err
	}
	directory, err := openOrCreateDirectoryNoFollowMode(root.root, clean, root.path, "anchored mkdir", mode.Perm())
	if err != nil {
		return err
	}
	if err := root.finishDirectoryOperationLocked(clean, directory, "anchored mkdir directory"); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}

// WriteExclusiveFileWriteThrough creates a regular file with write-through
// semantics. An existing regular file is accepted only when its exact bytes
// and effective mode match. allowExactReplay controls whether that is success.
func (root *AnchoredRoot) WriteExclusiveFileWriteThrough(rel string, data []byte, mode fs.FileMode, allowExactReplay bool) (bool, error) {
	if root == nil {
		return false, fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.validateLocked(); err != nil {
		return false, err
	}
	clean, err := cleanAnchoredRel(rel, false)
	if err != nil {
		return false, err
	}
	parent, err := openDirectoryNoFollow(root.root, filepath.Dir(clean), root.path, "anchored write")
	if err != nil {
		return false, err
	}
	defer parent.Close()
	name := filepath.Base(clean)
	if _, statErr := parent.Lstat(name); statErr == nil {
		if !allowExactReplay {
			return false, fmt.Errorf("anchored write target already exists: %s", rel)
		}
		if err := root.validateReplayLocked(parent, clean, name, data, mode); err != nil {
			return false, err
		}
		return true, root.finishDirectoryOperationLocked(filepath.Dir(clean), parent, "anchored replay parent")
	} else if !os.IsNotExist(statErr) {
		return false, statErr
	}
	file, err := createExactFile(parent, name, mode)
	if os.IsExist(err) {
		if !allowExactReplay {
			return false, fmt.Errorf("anchored write target already exists: %s", rel)
		}
		if err := root.validateReplayLocked(parent, clean, name, data, mode); err != nil {
			return false, err
		}
		return true, root.finishDirectoryOperationLocked(filepath.Dir(clean), parent, "anchored replay parent")
	}
	if err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = file.Abort()
		}
	}()
	written, writeErr := file.Write(data)
	syncErr := file.Sync()
	opened, statErr := file.Stat()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, errors.Join(fmt.Errorf("anchored exclusive write seek failed: %s: %w", rel, err), file.Abort())
	}
	readBack, readErr := io.ReadAll(io.LimitReader(file, int64(len(data))+1))
	afterRead, afterReadErr := file.Stat()
	if writeErr != nil || written != len(data) || syncErr != nil || statErr != nil || readErr != nil || afterReadErr != nil ||
		!opened.Mode().IsRegular() || opened.Mode()&os.ModeSymlink != 0 || opened.Size() != int64(len(data)) || !anchoredModeMatches(mode, opened.Mode()) ||
		!bytes.Equal(readBack, data) || !os.SameFile(opened, afterRead) || opened.Size() != afterRead.Size() || opened.Mode() != afterRead.Mode() {
		abortErr := file.Abort()
		return false, fmt.Errorf("anchored exclusive write failed: %s: %w", rel, errors.Join(writeErr, syncErr, statErr, readErr, afterReadErr, abortErr))
	}
	if anchoredRootWriteBeforeFinalValidationHook != nil {
		if err := anchoredRootWriteBeforeFinalValidationHook(); err != nil {
			return false, errors.Join(err, file.Abort())
		}
	}
	after, afterErr := parent.Lstat(name)
	if afterErr != nil || !after.Mode().IsRegular() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, after) ||
		after.Size() != int64(len(data)) || opened.Mode() != after.Mode() || !anchoredModeMatches(mode, after.Mode()) {
		return false, errors.Join(
			fmt.Errorf("anchored exclusive write failed final validation: %s: %w", rel, afterErr),
			file.Abort(),
		)
	}
	if err := rejectReparsePath(filepath.Join(root.path, clean)); err != nil {
		return false, errors.Join(err, file.Abort())
	}
	if err := syncPublishedDirectory(parent); err != nil {
		return false, errors.Join(fmt.Errorf("anchored write parent sync failed: %s: %w", rel, err), file.Abort())
	}
	if err := root.finishDirectoryOperationLocked(filepath.Dir(clean), parent, "anchored write parent"); err != nil {
		return false, errors.Join(err, file.Abort())
	}
	if err := file.Commit(); err != nil {
		return false, fmt.Errorf("anchored exclusive write commit failed: %s: %w", rel, err)
	}
	committed = true
	return false, nil
}

// RenameFileNoReplaceExact atomically moves a regular file only when its
// bytes and effective mode match the caller's reviewed source.
func (root *AnchoredRoot) RenameFileNoReplaceExact(sourceRel, targetRel string, expected []byte, mode fs.FileMode) error {
	return root.renameNoReplaceExact(sourceRel, targetRel, expected, mode, nil)
}

// RenameDirectoryNoReplaceExact atomically moves a directory only when its
// complete relative directory/file tree still matches the caller's binding.
func (root *AnchoredRoot) RenameDirectoryNoReplaceExact(sourceRel, targetRel string, expected ExpectedTree) error {
	validated, err := validateExpectedTree(expected)
	if err != nil {
		return fmt.Errorf("anchored rename expected tree is invalid: %w", err)
	}
	return root.renameNoReplaceExact(sourceRel, targetRel, nil, 0, &validated)
}

func (root *AnchoredRoot) renameNoReplaceExact(sourceRel, targetRel string, expected []byte, mode fs.FileMode, expectedTree *ExpectedTree) error {
	if root == nil {
		return fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.validateLocked(); err != nil {
		return err
	}
	source, err := cleanAnchoredRel(sourceRel, false)
	if err != nil {
		return fmt.Errorf("anchored rename source: %w", err)
	}
	target, err := cleanAnchoredRel(targetRel, false)
	if err != nil {
		return fmt.Errorf("anchored rename target: %w", err)
	}
	if anchoredRelEqual(source, target) {
		return fmt.Errorf("anchored rename source and target are identical: %s", sourceRel)
	}
	sourceParentRel := filepath.Dir(source)
	targetParentRel := filepath.Dir(target)
	sourceParent, err := openDirectoryNoFollow(root.root, sourceParentRel, root.path, "anchored rename source")
	if err != nil {
		return err
	}
	defer sourceParent.Close()
	targetParent, err := openDirectoryNoFollow(root.root, targetParentRel, root.path, "anchored rename target")
	if err != nil {
		return err
	}
	defer targetParent.Close()
	sourceName := filepath.Base(source)
	targetName := filepath.Base(target)
	sourceInfo, err := sourceParent.Lstat(sourceName)
	if err != nil {
		return err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("anchored rename source must not be a symlink or reparse point: %s", sourceRel)
	}
	if expectedTree != nil {
		if !sourceInfo.IsDir() {
			return fmt.Errorf("anchored rename source must be a directory: %s", sourceRel)
		}
	} else if !sourceInfo.Mode().IsRegular() || sourceInfo.Size() != int64(len(expected)) || !anchoredModeMatches(mode, sourceInfo.Mode()) {
		return fmt.Errorf("anchored rename file differs from expected object: %s", sourceRel)
	}
	if err := rejectReparsePath(filepath.Join(root.path, source)); err != nil {
		return err
	}
	if _, err := targetParent.Lstat(targetName); err == nil {
		return fmt.Errorf("anchored rename target already exists: %s", targetRel)
	} else if !os.IsNotExist(err) {
		return err
	}
	guard, err := openExactMutationGuard(root.path, filepath.Join(root.path, sourceParentRel), filepath.Join(root.path, targetParentRel))
	if err != nil {
		return err
	}
	guardClosed := false
	defer func() {
		if !guardClosed {
			_ = guard.Close()
		}
	}()
	if err := root.validateDirectoryBindingLocked(sourceParentRel, mustDirectoryInfo(sourceParent), "anchored rename source parent"); err != nil {
		return err
	}
	if err := root.validateDirectoryBindingLocked(targetParentRel, mustDirectoryInfo(targetParent), "anchored rename target parent"); err != nil {
		return err
	}
	if err := root.validateLocked(); err != nil {
		return err
	}
	if anchoredRootBeforeRenameHook != nil {
		if err := anchoredRootBeforeRenameHook(); err != nil {
			return err
		}
	}
	if err := guard.Validate(); err != nil {
		return err
	}
	current, err := sourceParent.Lstat(sourceName)
	if err != nil || !os.SameFile(sourceInfo, current) {
		return fmt.Errorf("anchored rename source identity changed: %s: %w", sourceRel, err)
	}
	if _, err := targetParent.Lstat(targetName); err == nil {
		return fmt.Errorf("anchored rename target already exists: %s", targetRel)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := root.validateLocked(); err != nil {
		return err
	}
	request := exactRenameRequest{
		RootPath:      root.path,
		SourceRel:     source,
		TargetRel:     target,
		SourceParent:  sourceParent,
		TargetParent:  targetParent,
		SourceName:    sourceName,
		TargetName:    targetName,
		ExpectedInfo:  current,
		ExpectedBytes: expected,
		ExpectedMode:  mode,
		ExpectedTree:  expectedTree,
	}
	if err := renameNoReplaceExactNative(request); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("anchored rename target already exists: %s", targetRel)
		}
		return fmt.Errorf("anchored rename %s to %s: %w", sourceRel, targetRel, err)
	}
	targetInfo, targetErr := targetParent.Lstat(targetName)
	sourceMissing := false
	if _, sourceErr := sourceParent.Lstat(sourceName); os.IsNotExist(sourceErr) {
		sourceMissing = true
	} else if sourceErr != nil {
		return sourceErr
	}
	if targetErr != nil || !sourceMissing || targetInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(sourceInfo, targetInfo) {
		return fmt.Errorf("anchored rename result changed: %s to %s: %w", sourceRel, targetRel, targetErr)
	}
	if err := syncPublishedDirectory(sourceParent); err != nil {
		return err
	}
	if filepath.Clean(sourceParentRel) != filepath.Clean(targetParentRel) {
		if err := syncPublishedDirectory(targetParent); err != nil {
			return err
		}
	}
	if err := guard.Close(); err != nil {
		return fmt.Errorf("anchored rename mutation guard cleanup: %w", err)
	}
	guardClosed = true
	return nil
}

func mustDirectoryInfo(root *os.Root) os.FileInfo {
	info, _ := root.Lstat(".")
	return info
}

// RemoveExactFile removes a regular non-symlink leaf only when its bytes and
// effective mode still match the caller's validated object. Missing is replay.
func (root *AnchoredRoot) RemoveExactFile(rel string, expected []byte, mode fs.FileMode) error {
	return root.removeExact(rel, expected, mode, false)
}

// RemoveEmptyDirectory removes one exact empty non-symlink directory. A
// missing target is a successful replay.
func (root *AnchoredRoot) RemoveEmptyDirectory(rel string) error {
	return root.removeExact(rel, nil, 0, true)
}

func (root *AnchoredRoot) removeExact(rel string, expected []byte, mode fs.FileMode, directory bool) error {
	if root == nil {
		return fmt.Errorf("anchored root is missing")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.validateLocked(); err != nil {
		return err
	}
	clean, err := cleanAnchoredRel(rel, false)
	if err != nil {
		return err
	}
	parent, err := openDirectoryNoFollow(root.root, filepath.Dir(clean), root.path, "anchored remove")
	if os.IsNotExist(err) {
		return root.validateLocked()
	}
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(clean)
	info, err := parent.Lstat(name)
	if os.IsNotExist(err) {
		return root.validateLocked()
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		kind := "regular non-symlink file"
		if directory {
			kind = "empty non-symlink directory"
		}
		return fmt.Errorf("anchored remove target must be an exact %s: %s", kind, rel)
	}
	if err := rejectReparsePath(filepath.Join(root.path, clean)); err != nil {
		return err
	}
	if !directory {
		if info.Size() != int64(len(expected)) || !anchoredModeMatches(mode, info.Mode()) {
			return fmt.Errorf("anchored remove target differs from expected object: %s", rel)
		}
		if err := validateAnchoredOpenFile(parent, name, info, expected, mode); err != nil {
			return fmt.Errorf("anchored remove target changed or differs from expected object: %s: %w", rel, err)
		}
	}
	guard, err := openExactMutationGuard(root.path, filepath.Join(root.path, filepath.Dir(clean)), filepath.Join(root.path, filepath.Dir(clean)))
	if err != nil {
		return err
	}
	guardClosed := false
	defer func() {
		if !guardClosed {
			_ = guard.Close()
		}
	}()
	if directory {
		opened, err := parent.OpenRoot(name)
		if err != nil {
			return err
		}
		file, openErr := opened.Open(".")
		var entries []os.DirEntry
		var readErr error
		var fileCloseErr error
		if openErr == nil {
			entries, readErr = file.ReadDir(-1)
			if errors.Is(readErr, io.EOF) {
				readErr = nil
			}
			fileCloseErr = file.Close()
		}
		openedInfo, statErr := opened.Lstat(".")
		closeErr := opened.Close()
		if openErr != nil || readErr != nil || fileCloseErr != nil || statErr != nil || closeErr != nil || !openedInfo.IsDir() || !os.SameFile(info, openedInfo) || len(entries) != 0 {
			return fmt.Errorf("anchored remove directory must remain empty and exact: %s: %w", rel, errors.Join(openErr, readErr, fileCloseErr, statErr, closeErr))
		}
	}
	if err := root.validateLocked(); err != nil {
		return err
	}
	if anchoredRootBeforeRemoveHook != nil {
		if err := anchoredRootBeforeRemoveHook(); err != nil {
			return err
		}
	}
	if err := guard.Validate(); err != nil {
		return err
	}
	current, err := parent.Lstat(name)
	if err != nil || current.IsDir() != directory || !os.SameFile(info, current) {
		return fmt.Errorf("anchored remove target identity changed: %s: %w", rel, err)
	}
	if !directory {
		if err := validateAnchoredOpenFile(parent, name, current, expected, mode); err != nil {
			return fmt.Errorf("anchored remove target changed before deletion: %s: %w", rel, err)
		}
	}
	if err := removeExactObject(parent, name, current, expected, mode, directory); err != nil {
		return err
	}
	if _, err := parent.Lstat(name); !os.IsNotExist(err) {
		return fmt.Errorf("anchored remove target remains: %s: %w", rel, err)
	}
	if err := syncPublishedDirectory(parent); err != nil {
		return err
	}
	if err := root.finishDirectoryOperationLocked(filepath.Dir(clean), parent, "anchored remove parent"); err != nil {
		return err
	}
	if err := guard.Close(); err != nil {
		return fmt.Errorf("anchored remove mutation guard cleanup: %w", err)
	}
	guardClosed = true
	return nil
}

func (root *AnchoredRoot) validateDirectoryBindingLocked(rel string, expected os.FileInfo, label string) error {
	if expected == nil {
		return fmt.Errorf("%s identity is missing", label)
	}
	current, err := openDirectoryNoFollow(root.root, rel, root.path, label)
	if err != nil {
		return err
	}
	actual, statErr := current.Lstat(".")
	closeErr := current.Close()
	if statErr != nil || closeErr != nil || !actual.IsDir() || !os.SameFile(expected, actual) {
		return fmt.Errorf("%s changed: %w", label, errors.Join(statErr, closeErr))
	}
	return nil
}

func (root *AnchoredRoot) validateLocked() error {
	if root.root == nil || root.identity == nil {
		return fmt.Errorf("anchored root is closed")
	}
	if err := rejectReparseAncestors(root.path); err != nil {
		return err
	}
	lexical, err := os.Lstat(root.path)
	if err != nil || !lexical.IsDir() || lexical.Mode()&os.ModeSymlink != 0 || !os.SameFile(root.identity, lexical) {
		return fmt.Errorf("anchored root physical identity changed: %s: %w", root.path, err)
	}
	opened, err := root.root.Lstat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(root.identity, opened) || !os.SameFile(opened, lexical) {
		return fmt.Errorf("anchored root handle identity changed: %s: %w", root.path, err)
	}
	return nil
}

func validateAnchoredOpenFile(parent *os.Root, name string, expectedInfo os.FileInfo, expected []byte, mode fs.FileMode) error {
	file, err := openExactFile(parent, name, false)
	if err != nil {
		return err
	}
	if anchoredRootExactFileAfterOpenHook != nil {
		if err := anchoredRootExactFileAfterOpenHook(); err != nil {
			_ = file.Close()
			return err
		}
	}
	data, opened, readErr := readExactFileData(file, int64(len(expected)))
	unbrokenErr := exactFileUnbroken(file)
	closeErr := file.Close()
	current, currentErr := parent.Lstat(name)
	if readErr != nil || currentErr != nil || unbrokenErr != nil || closeErr != nil || !opened.Mode().IsRegular() ||
		!os.SameFile(expectedInfo, opened) || !os.SameFile(opened, current) || opened.Size() != current.Size() || opened.Mode() != current.Mode() ||
		!bytes.Equal(data, expected) || !anchoredModeMatches(mode, opened.Mode()) {
		return errors.Join(readErr, currentErr, unbrokenErr, closeErr, fmt.Errorf("exact regular file differs"))
	}
	return nil
}

func (root *AnchoredRoot) validateReplayLocked(parent *os.Root, clean, name string, data []byte, mode fs.FileMode) error {
	before, err := parent.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() != int64(len(data)) || !anchoredModeMatches(mode, before.Mode()) {
		return fmt.Errorf("anchored replay target differs: %s", clean)
	}
	if err := rejectReparsePath(filepath.Join(root.path, clean)); err != nil {
		return err
	}
	file, err := openExactFile(parent, name, false)
	if err != nil {
		return err
	}
	if anchoredRootExactFileAfterOpenHook != nil {
		if err := anchoredRootExactFileAfterOpenHook(); err != nil {
			_ = file.Close()
			return err
		}
	}
	current, opened, readErr := readExactFileData(file, int64(len(data)))
	unbrokenErr := exactFileUnbroken(file)
	closeErr := file.Close()
	after, afterErr := parent.Lstat(name)
	if readErr != nil || afterErr != nil || unbrokenErr != nil || closeErr != nil || len(current) != len(data) || !bytes.Equal(current, data) ||
		!opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) || opened.Size() != after.Size() || opened.Mode() != after.Mode() ||
		!anchoredModeMatches(mode, after.Mode()) {
		return fmt.Errorf("anchored replay target changed or differs: %s: %w", clean, errors.Join(readErr, closeErr, afterErr, unbrokenErr))
	}
	return nil
}

func cleanAnchoredRel(rel string, allowRoot bool) (string, error) {
	if rel != strings.TrimSpace(rel) {
		return "", fmt.Errorf("anchored relative path must not contain surrounding whitespace: %q", rel)
	}
	if rel == "" {
		return "", fmt.Errorf("anchored relative path is empty")
	}
	local := filepath.FromSlash(rel)
	if filepath.IsAbs(local) || filepath.VolumeName(local) != "" {
		return "", fmt.Errorf("anchored path must be relative: %s", rel)
	}
	clean := filepath.Clean(local)
	if clean == "." {
		if allowRoot {
			return clean, nil
		}
		return "", fmt.Errorf("anchored path must name a child: %s", rel)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("anchored path escapes root: %s", rel)
	}
	for _, component := range splitPathComponents(clean) {
		if component == "" || component == "." || component == ".." {
			return "", fmt.Errorf("anchored path contains an invalid component: %s", rel)
		}
	}
	return clean, nil
}

func anchoredRelEqual(left, right string) bool {
	if filepath.Separator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func anchoredModeMatches(expected, actual fs.FileMode) bool {
	if filepath.Separator == '\\' {
		return expected.Perm()&0o200 == actual.Perm()&0o200
	}
	return expected.Perm() == actual.Perm()
}

func (root *AnchoredRoot) finishDirectoryOperationLocked(rel string, directory *os.Root, label string) error {
	expected, err := directory.Lstat(".")
	if err != nil {
		return err
	}
	if err := root.validateDirectoryBindingLocked(rel, expected, label); err != nil {
		return err
	}
	return root.validateLocked()
}

func sortAnchoredNames(names []string) {
	sort.Slice(names, func(left, right int) bool {
		leftFolded := strings.ToLower(names[left])
		rightFolded := strings.ToLower(names[right])
		if leftFolded != rightFolded {
			return leftFolded < rightFolded
		}
		return names[left] < names[right]
	})
}
