package kitmutation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Lease struct {
	file   *os.File
	unlock func(uintptr) error
}

func Acquire(repoRoot string) (*Lease, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(repo); resolveErr == nil {
		repo = resolved
	}
	identity := filepath.Clean(repo)
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(identity)
	}
	root, err := stableLockRoot()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if st, err := os.Lstat(root); err != nil {
		return nil, err
	} else if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
		return nil, fmt.Errorf("kit mutation lock root must be a directory and not a symlink: %s", root)
	}
	key := sha256.Sum256([]byte(identity))
	path := filepath.Join(root, "repo-"+hex.EncodeToString(key[:])+".lease")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(file.Fd()); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return &Lease{file: file, unlock: unlockFile}, nil
}

func (lease *Lease) Unlock() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	file := lease.file
	lease.file = nil
	return errors.Join(lease.unlock(file.Fd()), file.Close())
}
