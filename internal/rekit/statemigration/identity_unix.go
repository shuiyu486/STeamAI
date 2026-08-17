//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package statemigration

import (
	"fmt"
	"os"
	"syscall"
)

func identityForRoot(root *os.Root) (Identity, error) {
	info, err := root.Stat(".")
	if err != nil {
		return Identity{}, err
	}
	return identityForInfo(info)
}

func identityForFile(file *os.File) (Identity, error) {
	info, err := file.Stat()
	if err != nil {
		return Identity{}, err
	}
	return identityForInfo(info)
}

func identityForInfo(info os.FileInfo) (Identity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || uint64(stat.Dev) == 0 || uint64(stat.Ino) == 0 {
		return Identity{}, fmt.Errorf("Unix filesystem identity is incomplete")
	}
	return Identity{Scheme: "unix-dev-inode-v1", Device: uint64(stat.Dev), Inode: uint64(stat.Ino)}, nil
}
