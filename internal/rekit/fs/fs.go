package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func FullPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return filepath.Abs(".")
	}
	return filepath.Abs(path)
}

func SamePath(left, right string) bool {
	left, err := lexicalPath(left)
	if err != nil {
		return false
	}
	right, err = lexicalPath(right)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func SameExistingPath(left, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false, err
	}
	return os.SameFile(leftInfo, rightInfo), nil
}

func lexicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(filepath.Clean(absolute), string(filepath.Separator)), nil
}

func SafeJoin(root, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("relative path is empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative: %s", rel)
	}
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathFull, err := filepath.Abs(filepath.Join(rootFull, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	rootClean := strings.TrimRight(filepath.Clean(rootFull), string(filepath.Separator))
	pathClean := strings.TrimRight(filepath.Clean(pathFull), string(filepath.Separator))
	if strings.EqualFold(pathClean, rootClean) {
		return pathClean, nil
	}
	prefix := rootClean + string(filepath.Separator)
	if !strings.HasPrefix(strings.ToLower(pathClean), strings.ToLower(prefix)) {
		return "", fmt.Errorf("path escapes root: %s", rel)
	}
	return pathClean, nil
}

func ReadStableRegularFileAnchored(caseRoot, path, label string, limit int64) ([]byte, error) {
	rootPath, err := filepath.Abs(caseRoot)
	if err != nil {
		return nil, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(rootPath, path)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%s path escapes anchored case root: %s", label, path)
	}
	beforeRoot, err := os.Lstat(rootPath)
	if err != nil || !beforeRoot.IsDir() || beforeRoot.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s case root must be a non-symlink directory: %s", label, rootPath)
	}
	if err := rejectReparseAncestors(rootPath); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	openedRoot, err := root.Lstat(".")
	if err != nil || !openedRoot.IsDir() || !os.SameFile(beforeRoot, openedRoot) {
		return nil, fmt.Errorf("%s case root changed while opening: %s", label, rootPath)
	}
	parent, err := openDirectoryNoFollow(root, filepath.Dir(rel), rootPath, label)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	defer parent.Close()
	name := filepath.Base(rel)
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > limit {
		return nil, fmt.Errorf("%s must be a bounded non-empty regular file: %s", label, path)
	}
	if err := rejectReparsePath(path); err != nil {
		return nil, err
	}
	file, err := parent.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	after, afterErr := parent.Lstat(name)
	currentRoot, rootErr := os.Lstat(rootPath)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || rootErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) || !os.SameFile(openedRoot, currentRoot) || int64(len(data)) != opened.Size() || int64(len(data)) > limit {
		return nil, fmt.Errorf("%s changed while reading: %s", label, path)
	}
	if err := rejectReparseAncestors(rootPath); err != nil {
		return nil, err
	}
	return data, nil
}

func openDirectoryNoFollow(root *os.Root, rel, rootPath, label string) (*os.Root, error) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%s directory escapes anchored case root: %s", label, rel)
	}
	current, err := root.OpenRoot(".")
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
			return nil, fmt.Errorf("%s directory contains an invalid component: %s", label, rel)
		}
		walked = append(walked, component)
		before, err := current.Lstat(component)
		if err != nil {
			current.Close()
			return nil, err
		}
		if !before.Mode().IsDir() || before.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, fmt.Errorf("%s directory component must be a non-symlink directory: %s", label, component)
		}
		if err := rejectReparsePath(filepath.Join(rootPath, filepath.Join(walked...))); err != nil {
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
		if openedErr != nil || afterErr != nil || !opened.Mode().IsDir() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			next.Close()
			current.Close()
			return nil, fmt.Errorf("%s directory component changed while opening: %s", label, component)
		}
		current.Close()
		current = next
	}
	return current, nil
}

func ReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func FileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func FileSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type RegularFileState string

const (
	RegularFileMissing RegularFileState = "missing"
	RegularFileWaiting RegularFileState = "waiting"
	RegularFileReady   RegularFileState = "ready"
	RegularFileSymlink RegularFileState = "symlink"
)

func ClassifyNonEmptyRegularFile(path string) (RegularFileState, error) {
	st, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return RegularFileMissing, nil
	}
	if err != nil {
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return RegularFileSymlink, nil
	}
	if !st.Mode().IsRegular() || st.Size() == 0 {
		return RegularFileWaiting, nil
	}
	return RegularFileReady, nil
}

func IsTextNonEmptyUnder(path string, limit int64) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("missing file: %s", path)
	}
	if strings.TrimSpace(string(b)) == "" {
		return 0, fmt.Errorf("empty file: %s", path)
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if st.Size() > limit {
		return st.Size(), fmt.Errorf("file too large: %s %d > %d", path, st.Size(), limit)
	}
	return st.Size(), nil
}
