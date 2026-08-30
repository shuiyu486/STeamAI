package memberexecution

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

type anchoredCase struct {
	path                string
	root                *os.Root
	info                os.FileInfo
	statePrefix         string
	statePath           string
	stateInfo           os.FileInfo
	missionGeneration   int
	missionID           string
	activePointerSHA256 string
}

func openAnchoredCase(path string) (*anchoredCase, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	absolute = filepath.Clean(absolute)
	if err := rejectReparseAncestors(absolute); err != nil {
		return nil, err
	}
	before, err := os.Lstat(absolute)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("member execution case root must be a non-symlink directory: %s", absolute)
	}
	root, err := os.OpenRoot(absolute)
	if err != nil {
		return nil, err
	}
	opened, err := root.Lstat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		root.Close()
		return nil, fmt.Errorf("member execution case root changed while opening: %s", absolute)
	}
	view, err := projectstate.ResolveMissionView(absolute)
	if err != nil {
		root.Close()
		return nil, err
	}
	statePrefix, err := filepath.Rel(absolute, view.Path)
	if err != nil || filepath.IsAbs(statePrefix) || statePrefix == "." || statePrefix == ".." || strings.HasPrefix(statePrefix, ".."+string(filepath.Separator)) {
		root.Close()
		return nil, fmt.Errorf("member execution mission root escapes case root: %s", view.Path)
	}
	stateBefore, err := root.Lstat(statePrefix)
	if err != nil || !stateBefore.IsDir() || stateBefore.Mode()&os.ModeSymlink != 0 {
		root.Close()
		return nil, fmt.Errorf("member execution mission root must be a non-symlink directory: %s", view.Path)
	}
	if err := rejectReparsePath(view.Path); err != nil {
		root.Close()
		return nil, err
	}
	state, err := root.OpenRoot(statePrefix)
	if err != nil {
		root.Close()
		return nil, err
	}
	stateOpened, stateErr := state.Lstat(".")
	closeErr := state.Close()
	stateAfter, afterErr := root.Lstat(statePrefix)
	if stateErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(stateBefore, stateOpened) || !os.SameFile(stateOpened, stateAfter) {
		root.Close()
		return nil, fmt.Errorf("member execution mission root changed while opening: %s", view.Path)
	}
	return &anchoredCase{path: absolute, root: root, info: opened, statePrefix: statePrefix, statePath: view.Path, stateInfo: stateOpened, missionGeneration: view.Generation, missionID: view.MissionID, activePointerSHA256: view.ActivePointerSHA256}, nil
}

func (a *anchoredCase) Close() error { return a.root.Close() }

func (a *anchoredCase) revalidate() error {
	after, err := os.Lstat(a.path)
	if err != nil || !after.IsDir() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(a.info, after) {
		return fmt.Errorf("member execution case root identity changed: %s", a.path)
	}
	if err := rejectReparseAncestors(a.path); err != nil {
		return err
	}
	view, err := projectstate.ResolveMissionView(a.path)
	if err != nil {
		return err
	}
	stateAfter, err := os.Lstat(a.statePath)
	if err != nil || view.Path != a.statePath || view.Generation != a.missionGeneration || view.MissionID != a.missionID || view.ActivePointerSHA256 != a.activePointerSHA256 || !stateAfter.IsDir() || stateAfter.Mode()&os.ModeSymlink != 0 || !os.SameFile(a.stateInfo, stateAfter) {
		return fmt.Errorf("member execution active mission root identity changed: %s", a.statePath)
	}
	if err := rejectReparsePath(a.statePath); err != nil {
		return err
	}
	return nil
}

func (a *anchoredCase) stateRel(parts ...string) string {
	clean := append([]string{a.statePrefix}, parts...)
	return filepath.Join(clean...)
}

func cleanRootRelative(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("member execution path escapes anchored case root: %s", rel)
	}
	return clean, nil
}

func (a *anchoredCase) openDir(rel string, create bool) (*os.Root, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("member execution directory escapes anchored case root: %s", rel)
	}
	current, err := a.root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	if clean == "." {
		return current, nil
	}
	walked := []string{}
	for component := range strings.SplitSeq(clean, string(filepath.Separator)) {
		if component == "" || component == "." || component == ".." {
			current.Close()
			return nil, fmt.Errorf("member execution directory contains invalid component: %s", rel)
		}
		walked = append(walked, component)
		before, statErr := current.Lstat(component)
		if os.IsNotExist(statErr) && create {
			if err := current.Mkdir(component, 0o755); err != nil && !os.IsExist(err) {
				current.Close()
				return nil, err
			}
			before, statErr = current.Lstat(component)
		}
		if statErr != nil {
			current.Close()
			return nil, statErr
		}
		if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, fmt.Errorf("member execution directory component must be a non-symlink directory: %s", component)
		}
		if err := rejectReparsePath(filepath.Join(a.path, filepath.Join(walked...))); err != nil {
			current.Close()
			return nil, err
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			current.Close()
			return nil, err
		}
		opened, openedErr := next.Lstat(".")
		after, afterErr := current.Lstat(component)
		if openedErr != nil || afterErr != nil || !opened.IsDir() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			next.Close()
			current.Close()
			return nil, fmt.Errorf("member execution directory component changed while opening: %s", component)
		}
		current.Close()
		current = next
	}
	return current, nil
}

func (a *anchoredCase) readFile(rel string, limit int64) ([]byte, error) {
	clean, err := cleanRootRelative(rel)
	if err != nil {
		return nil, err
	}
	parent, err := a.openDir(filepath.Dir(clean), false)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	leaf := filepath.Base(clean)
	before, err := parent.Lstat(leaf)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, fmt.Errorf("member execution artifact must be a bounded regular file: %s", rel)
	}
	if err := rejectReparsePath(filepath.Join(a.path, clean)); err != nil {
		return nil, err
	}
	file, err := parent.Open(leaf)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	after, afterErr := parent.Lstat(leaf)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || int64(len(data)) > limit {
		return nil, fmt.Errorf("member execution artifact changed while reading: %s", rel)
	}
	return data, nil
}

func (a *anchoredCase) writeExclusive(rel string, data []byte) error {
	clean, err := cleanRootRelative(rel)
	if err != nil {
		return err
	}
	parent, err := a.openDir(filepath.Dir(clean), true)
	if err != nil {
		return err
	}
	defer parent.Close()
	leaf := filepath.Base(clean)
	file, err := parent.OpenFile(leaf, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || written != len(data) || syncErr != nil || closeErr != nil {
		_ = parent.Remove(leaf)
		return fmt.Errorf("member execution exclusive publication failed: %s", rel)
	}
	return nil
}

func (a *anchoredCase) readDir(rel string) ([]os.DirEntry, error) {
	dir, err := a.openDir(rel, false)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	file, err := dir.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := file.ReadDir(-1)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return entries, nil
}

func relativeToCase(caseRoot, path string) (string, error) {
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("member execution path escapes case root: %s", path)
	}
	return rel, nil
}
