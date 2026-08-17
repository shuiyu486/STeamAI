//go:build linux || darwin

package fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type unixExactFile struct {
	file *os.File
}

func (file *unixExactFile) Stat() (os.FileInfo, error) { return file.file.Stat() }
func (file *unixExactFile) Read(data []byte) (int, error) {
	return file.file.Read(data)
}
func (file *unixExactFile) Seek(offset int64, whence int) (int64, error) {
	return file.file.Seek(offset, whence)
}
func (file *unixExactFile) Sync() error { return file.file.Sync() }
func (file *unixExactFile) Close() error {
	if file == nil || file.file == nil {
		return nil
	}
	err := file.file.Close()
	file.file = nil
	return err
}

type unixGuardedDirectory struct {
	path string
	file *os.File
	info os.FileInfo
}

type unixMutationGuard struct {
	directories []unixGuardedDirectory
}

func (guard *unixMutationGuard) Validate() error {
	if guard == nil {
		return os.ErrInvalid
	}
	for _, directory := range guard.directories {
		opened, openedErr := directory.file.Stat()
		lexical, lexicalErr := os.Lstat(directory.path)
		if openedErr != nil || lexicalErr != nil || !opened.IsDir() ||
			lexical.Mode()&os.ModeSymlink != 0 || !lexical.IsDir() ||
			!os.SameFile(directory.info, opened) || !os.SameFile(opened, lexical) {
			return fmt.Errorf(
				"exact mutation guard path changed: %s: %w",
				directory.path,
				errors.Join(openedErr, lexicalErr),
			)
		}
	}
	return nil
}

func (guard *unixMutationGuard) Close() error {
	if guard == nil {
		return nil
	}
	var errs []error
	for index := len(guard.directories) - 1; index >= 0; index-- {
		errs = append(errs, guard.directories[index].file.Close())
	}
	guard.directories = nil
	return errors.Join(errs...)
}

func openExactFile(parent *os.Root, name string, writeThrough bool) (exactFileHandle, error) {
	parentDirectory, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer parentDirectory.Close()
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	if writeThrough {
		flags |= unix.O_SYNC
	}
	fd, err := unix.Openat(int(parentDirectory.Fd()), name, flags, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return nil, os.ErrInvalid
	}
	return &unixExactFile{file: file}, nil
}

func createExactFile(*os.Root, string, os.FileMode) (exactCreatedFile, error) {
	return nil, errAnchoredExactMutationUnsupported
}

func exactFileUnbroken(file exactFileHandle) error {
	unixFile, ok := file.(*unixExactFile)
	if !ok || unixFile.file == nil {
		return errAnchoredExactMutationUnsupported
	}
	_, err := unixFile.file.Stat()
	return err
}

func openExactMutationGuard(
	rootPath,
	sourceParentPath,
	targetParentPath string,
) (exactMutationGuard, error) {
	guard := &unixMutationGuard{}
	seen := map[string]bool{}
	for _, path := range []string{rootPath, sourceParentPath, targetParentPath} {
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		directory, err := openUnixGuardedDirectory(path)
		if err != nil {
			_ = guard.Close()
			return nil, fmt.Errorf("open exact mutation guard for %s: %w", path, err)
		}
		guard.directories = append(guard.directories, directory)
	}
	if err := guard.Validate(); err != nil {
		_ = guard.Close()
		return nil, err
	}
	return guard, nil
}

func openUnixGuardedDirectory(path string) (unixGuardedDirectory, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return unixGuardedDirectory{}, fmt.Errorf(
			"guard path must be a non-symlink directory: %s: %w",
			path,
			err,
		)
	}
	fd, err := unix.Open(
		path,
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return unixGuardedDirectory{}, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return unixGuardedDirectory{}, os.ErrInvalid
	}
	opened, err := file.Stat()
	if err != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		_ = file.Close()
		return unixGuardedDirectory{}, fmt.Errorf(
			"guard path changed while opening: %s: %w",
			path,
			err,
		)
	}
	return unixGuardedDirectory{path: path, file: file, info: opened}, nil
}

func readExactFileData(
	file exactFileHandle,
	limit int64,
) ([]byte, os.FileInfo, error) {
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 ||
		before.Size() > limit {
		return nil, nil, fmt.Errorf(
			"exact file must be a bounded regular file: %w",
			err,
		)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	after, statErr := file.Stat()
	if readErr != nil || statErr != nil || int64(len(data)) > limit ||
		int64(len(data)) != before.Size() || after.Size() != before.Size() ||
		before.Mode() != after.Mode() || !os.SameFile(before, after) ||
		!before.ModTime().Equal(after.ModTime()) {
		return nil, nil, fmt.Errorf(
			"exact file changed while reading: %w",
			errors.Join(readErr, statErr),
		)
	}
	return data, after, nil
}
