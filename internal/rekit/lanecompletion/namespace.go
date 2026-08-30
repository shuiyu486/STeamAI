package lanecompletion

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func openMissionNamespaceRoot(caseRoot string, view projectstate.MissionView, components []string, create bool) (*os.Root, error) {
	stateRel, err := filepath.Rel(caseRoot, view.Path)
	if err != nil || filepath.IsAbs(stateRel) || stateRel == "." || stateRel == ".." || strings.HasPrefix(stateRel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("lane completion mission namespace escapes case root: %s", view.Path)
	}
	parts := strings.Split(filepath.Clean(stateRel), string(filepath.Separator))
	return openCaseNamespaceRoot(caseRoot, append(parts, components...), create)
}

func openCaseNamespaceRoot(caseRoot string, components []string, create bool) (*os.Root, error) {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return nil, err
	}
	caseInfo, err := os.Lstat(caseRoot)
	if err != nil {
		return nil, err
	}
	if err := rejectReparsePath(caseRoot); err != nil {
		return nil, err
	}
	if caseInfo.Mode()&os.ModeSymlink != 0 || !caseInfo.IsDir() {
		return nil, fmt.Errorf("case root must be a non-symlink directory")
	}
	current, err := os.OpenRoot(caseRoot)
	if err != nil {
		return nil, err
	}
	openedCase, openErr := current.Stat(".")
	caseAfter, afterErr := os.Lstat(caseRoot)
	if openErr != nil || afterErr != nil || !os.SameFile(caseInfo, openedCase) || !os.SameFile(openedCase, caseAfter) {
		_ = current.Close()
		return nil, fmt.Errorf("case root changed while opening")
	}
	for index, component := range components {
		if component == "" || component == "." || component == ".." || filepath.Base(component) != component {
			_ = current.Close()
			return nil, fmt.Errorf("lane completion namespace contains an invalid component: %s", component)
		}
		before, statErr := current.Lstat(component)
		if os.IsNotExist(statErr) && create && index > 0 {
			if err := current.Mkdir(component, 0o700); err != nil && !os.IsExist(err) {
				_ = current.Close()
				return nil, err
			}
			before, statErr = current.Lstat(component)
		}
		if statErr != nil {
			_ = current.Close()
			return nil, statErr
		}
		componentPath := filepath.Join(append([]string{caseRoot}, components[:index+1]...)...)
		if err := rejectReparsePath(componentPath); err != nil {
			_ = current.Close()
			return nil, err
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("lane completion namespace component must be a non-symlink directory: %s", strings.Join(components[:index+1], "/"))
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		opened, openErr := next.Stat(".")
		after, afterErr := current.Lstat(component)
		if openErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			_ = next.Close()
			_ = current.Close()
			return nil, fmt.Errorf("lane completion namespace changed while opening: %s", strings.Join(components[:index+1], "/"))
		}
		if err := current.Close(); err != nil {
			_ = next.Close()
			return nil, err
		}
		current = next
	}
	return current, nil
}

func openPathParent(caseRoot, path string, create bool) (*os.Root, string, error) {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return nil, "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("lane completion path escapes case root: %s", path)
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	if len(parts) < 2 {
		return nil, "", fmt.Errorf("lane completion path lacks an anchored parent: %s", path)
	}
	root, err := openCaseNamespaceRoot(caseRoot, parts[:len(parts)-1], create)
	if err != nil {
		return nil, "", err
	}
	return root, parts[len(parts)-1], nil
}

func readStrictUnder(caseRoot, path string, target any) ([]byte, error) {
	root, name, err := openPathParent(caseRoot, path, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readStrictRoot(root, name, path, target)
}

func readStrictRoot(root *os.Root, name, displayPath string, target any) ([]byte, error) {
	limit := int64(maxArtifact)
	if filepath.Base(displayPath) == "intent.json" && strings.Contains(filepath.ToSlash(displayPath), "/"+OperationsDir+"/") {
		limit = maxOperation
	}
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if err := rejectReparsePath(displayPath); err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, fmt.Errorf("lane completion artifact must be a bounded regular file: %s", displayPath)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	if statErr != nil || !os.SameFile(before, opened) {
		_ = file.Close()
		return nil, fmt.Errorf("lane completion artifact changed while opening: %s", displayPath)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	after, afterErr := root.Lstat(name)
	if readErr != nil || closeErr != nil || afterErr != nil || int64(len(data)) > limit || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("lane completion artifact changed while reading: %s", displayPath)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return nil, fmt.Errorf("invalid lane completion artifact %s: %w", displayPath, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("lane completion artifact contains trailing JSON: %s", displayPath)
	}
	return data, nil
}

func regularExistsRoot(root *os.Root, name, displayPath string) (bool, error) {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := rejectReparsePath(displayPath); err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxArtifact {
		return false, fmt.Errorf("lane completion artifact must be a bounded regular file and not a symlink: %s", displayPath)
	}
	return true, nil
}

func readNamespaceEntries(root *os.Root) ([]fs.DirEntry, error) {
	return fs.ReadDir(root.FS(), ".")
}

func openChildRootNoFollow(parent *os.Root, name, displayPath string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if err := rejectReparsePath(displayPath); err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("lane completion namespace component must be a non-symlink directory: %s", displayPath)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, openErr := child.Stat(".")
	after, afterErr := parent.Lstat(name)
	if openErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		_ = child.Close()
		return nil, fmt.Errorf("lane completion namespace changed while opening: %s", displayPath)
	}
	return child, nil
}

func WriteExclusiveJSON(caseRoot, path string, value any) error {
	root, name, err := openPathParent(caseRoot, path, true)
	if err != nil {
		return err
	}
	defer root.Close()
	if info, statErr := root.Lstat(name); statErr == nil {
		if err := rejectReparsePath(path); err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("lane completion artifact must be a regular non-reparse file: %s", path)
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = root.Remove(name)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = root.Remove(name)
		return err
	}
	opened, statErr := file.Stat()
	closeErr := file.Close()
	after, afterErr := root.Lstat(name)
	if statErr != nil || closeErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		_ = root.Remove(name)
		return fmt.Errorf("lane completion artifact changed while publishing: %s", path)
	}
	return nil
}

func ReadExactJSON(caseRoot, path string, target any) ([]byte, error) {
	return readStrictUnder(caseRoot, path, target)
}

func ReadCaseFile(caseRoot, path string) ([]byte, error) {
	root, name, err := openPathParent(caseRoot, path, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readBoundedRegularRoot(root, name, path)
}

func readBoundedRegularRoot(root *os.Root, name, displayPath string) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if err := rejectReparsePath(displayPath); err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() > maxArtifact {
		return nil, fmt.Errorf("reopen publication input must be a bounded regular file: %s", displayPath)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	if statErr != nil || !os.SameFile(before, opened) {
		_ = file.Close()
		return nil, fmt.Errorf("reopen publication input changed while opening: %s", displayPath)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxArtifact+1))
	closeErr := file.Close()
	after, afterErr := root.Lstat(name)
	if readErr != nil || closeErr != nil || afterErr != nil || len(data) > maxArtifact || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("reopen publication input changed while reading: %s", displayPath)
	}
	return data, nil
}

func ApplyOperationPublication(caseRoot string, publication OperationPublication) (bool, error) {
	if len(publication.Bytes) == 0 || len(publication.Bytes) > maxArtifact || !strings.EqualFold(bytesSHA(publication.Bytes), publication.AfterSHA256) {
		return false, fmt.Errorf("invalid reopen publication bytes: %s", publication.Path)
	}
	path, err := safeOperationTargetPath(caseRoot, publication.Path)
	if err != nil {
		return false, err
	}
	root, name, err := openPathParent(caseRoot, path, publication.Mode == PublicationCreateExclusive)
	if err != nil {
		return false, err
	}
	defer root.Close()
	current, readErr := readBoundedRegularRoot(root, name, path)
	if readErr == nil {
		if string(current) == string(publication.Bytes) {
			return true, nil
		}
		if publication.Mode == PublicationCreateExclusive || !publication.BeforeExists || !strings.EqualFold(bytesSHA(current), publication.BeforeSHA256) {
			return false, fmt.Errorf("reopen publication predecessor differs from reviewed bytes: %s", publication.Path)
		}
	} else if !os.IsNotExist(readErr) {
		return false, readErr
	} else if publication.BeforeExists {
		return false, fmt.Errorf("reopen publication predecessor is missing: %s", publication.Path)
	}
	if publication.Mode == PublicationCreateExclusive {
		return false, writeExclusiveBytesRoot(root, name, path, publication.Bytes)
	}
	if publication.Mode != PublicationReplaceExact {
		return false, fmt.Errorf("invalid reopen publication mode: %s", publication.Mode)
	}
	tempName := "." + name + ".reopen-" + publication.AfterSHA256[:16] + ".tmp"
	if err := writeExclusiveBytesRoot(root, tempName, filepath.Join(filepath.Dir(path), tempName), publication.Bytes); err != nil {
		if !os.IsExist(err) {
			return false, err
		}
		tempBytes, readErr := readBoundedRegularRoot(root, tempName, filepath.Join(filepath.Dir(path), tempName))
		if readErr != nil || string(tempBytes) != string(publication.Bytes) {
			return false, fmt.Errorf("reopen publication temporary file differs from exact bytes: %s", publication.Path)
		}
	}
	if err := root.Rename(tempName, name); err != nil {
		return false, err
	}
	published, err := readBoundedRegularRoot(root, name, path)
	if err != nil || string(published) != string(publication.Bytes) {
		return false, fmt.Errorf("reopen publication differs after atomic replace: %s", publication.Path)
	}
	return false, nil
}

func writeExclusiveBytesRoot(root *os.Root, name, displayPath string, data []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = root.Remove(name)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = root.Remove(name)
		return err
	}
	opened, statErr := file.Stat()
	closeErr := file.Close()
	after, afterErr := root.Lstat(name)
	if statErr != nil || closeErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		_ = root.Remove(name)
		return fmt.Errorf("reopen publication changed while publishing: %s", displayPath)
	}
	return rejectReparsePath(displayPath)
}
