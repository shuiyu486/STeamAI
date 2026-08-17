package fs

import (
	"crypto/sha256"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// ExpectedFile binds one regular file in an ExpectedTree by relative name,
// exact bytes, SHA-256 digest, size, and effective mode.
type ExpectedFile struct {
	Path   string
	Data   []byte
	SHA256 [sha256.Size]byte
	Size   int64
	Mode   iofs.FileMode
}

// ExpectedTree binds every relative directory and regular file below the
// directory passed to RenameDirectoryNoReplaceExact. Directories includes ".".
type ExpectedTree struct {
	Directories []ExpectedDirectory
	Files       []ExpectedFile
}

// ExpectedDirectory binds one directory relative to an ExpectedTree root.
type ExpectedDirectory struct {
	Path string
	Mode iofs.FileMode
}

// NewExpectedFile derives the redundant exact bindings expected by tree rename.
func NewExpectedFile(path string, data []byte, mode iofs.FileMode) ExpectedFile {
	copyData := append([]byte(nil), data...)
	return ExpectedFile{
		Path:   filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))),
		Data:   copyData,
		SHA256: sha256.Sum256(copyData),
		Size:   int64(len(copyData)),
		Mode:   expectedTreeMode(mode),
	}
}

// NewExpectedDirectory derives the normalized binding expected by tree rename.
func NewExpectedDirectory(path string, mode iofs.FileMode) ExpectedDirectory {
	return ExpectedDirectory{
		Path: filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))),
		Mode: expectedTreeMode(mode),
	}
}

func validateExpectedTree(tree ExpectedTree) (ExpectedTree, error) {
	if len(tree.Directories) == 0 {
		return ExpectedTree{}, fmt.Errorf("expected tree must bind its root directory")
	}
	directories := append([]ExpectedDirectory(nil), tree.Directories...)
	files := make([]ExpectedFile, len(tree.Files))
	for index, file := range tree.Files {
		files[index] = file
		files[index].Data = append([]byte(nil), file.Data...)
	}
	sort.Slice(directories, func(left, right int) bool { return directories[left].Path < directories[right].Path })
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	seen := map[string]string{}
	rootSeen := false
	for index := range directories {
		clean, err := cleanExpectedTreePath(directories[index].Path, true)
		if err != nil {
			return ExpectedTree{}, fmt.Errorf("expected tree directory: %w", err)
		}
		directories[index].Path = filepath.ToSlash(clean)
		directories[index].Mode = expectedTreeMode(directories[index].Mode)
		if directories[index].Mode&iofs.ModeType != iofs.ModeDir {
			return ExpectedTree{}, fmt.Errorf("expected tree directory has non-directory mode: %s", directories[index].Path)
		}
		key := expectedTreePathKey(directories[index].Path)
		if prior := seen[key]; prior != "" {
			return ExpectedTree{}, fmt.Errorf("expected tree path collision: %s and %s", prior, directories[index].Path)
		}
		seen[key] = directories[index].Path
		rootSeen = rootSeen || directories[index].Path == "."
	}
	if !rootSeen {
		return ExpectedTree{}, fmt.Errorf("expected tree must include the root directory")
	}
	for index := range files {
		clean, err := cleanExpectedTreePath(files[index].Path, false)
		if err != nil {
			return ExpectedTree{}, fmt.Errorf("expected tree file: %w", err)
		}
		files[index].Path = filepath.ToSlash(clean)
		files[index].Mode = expectedTreeMode(files[index].Mode)
		if files[index].Mode&iofs.ModeType != 0 {
			return ExpectedTree{}, fmt.Errorf("expected tree file has non-regular mode: %s", files[index].Path)
		}
		if files[index].Size != int64(len(files[index].Data)) || files[index].SHA256 != sha256.Sum256(files[index].Data) {
			return ExpectedTree{}, fmt.Errorf("expected tree file bytes, hash, or size disagree: %s", files[index].Path)
		}
		key := expectedTreePathKey(files[index].Path)
		if prior := seen[key]; prior != "" {
			return ExpectedTree{}, fmt.Errorf("expected tree path collision: %s and %s", prior, files[index].Path)
		}
		seen[key] = files[index].Path
	}
	for _, directory := range directories {
		if directory.Path == "." {
			continue
		}
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(directory.Path)))
		if seen[expectedTreePathKey(parent)] != parent {
			return ExpectedTree{}, fmt.Errorf("expected tree directory parent is missing: %s", directory.Path)
		}
	}
	for _, file := range files {
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(file.Path)))
		if seen[expectedTreePathKey(parent)] != parent {
			return ExpectedTree{}, fmt.Errorf("expected tree file parent is missing: %s", file.Path)
		}
	}
	return ExpectedTree{Directories: directories, Files: files}, nil
}

func expectedTreePathKey(path string) string {
	if filepath.Separator == '\\' {
		return strings.ToLower(path)
	}
	return path
}

func cleanExpectedTreePath(path string, allowRoot bool) (string, error) {
	if path != strings.TrimSpace(path) || path == "" {
		return "", fmt.Errorf("relative path is empty or has surrounding whitespace: %q", path)
	}
	local := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(local) || filepath.VolumeName(local) != "" || local == ".." || strings.HasPrefix(local, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("relative path escapes tree: %s", path)
	}
	if local == "." && !allowRoot {
		return "", fmt.Errorf("file path must name a child")
	}
	return local, nil
}
