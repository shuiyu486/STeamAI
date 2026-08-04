//go:build unix

package packmemoryconsumption

import (
	"fmt"
	"syscall"
)

func caseRootIdentity(root rootIdentitySource) (CaseRootIdentity, error) {
	info, err := root.Lstat(".")
	if err != nil {
		return CaseRootIdentity{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return CaseRootIdentity{}, fmt.Errorf("Unix case-root identity is unavailable")
	}
	return CaseRootIdentity{
		Scheme: "unix-dev-inode-v1",
		Device: uint64(stat.Dev),
		Inode:  uint64(stat.Ino),
	}, nil
}
