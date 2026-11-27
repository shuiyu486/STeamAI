//go:build windows

package casebootstrap

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"
)

// 升级前探测已声明的成员 cwd 是否被占用，不枚举或结束进程。
// 请求 DELETE 访问但绝不设置删除标志；占着 cwd 的旧启动器也能挡住更新。
func holdUpgradeMemberDirs(root string, members []RosterMember) (func(), error) {
	var handles []syscall.Handle
	release := func() {
		for _, h := range handles {
			syscall.CloseHandle(h)
		}
		handles = nil
	}
	for _, member := range members {
		path := filepath.Join(root, ".steamai-vnext", "members", member.Name)
		if err := requirePlainPath(root, path, true); err != nil {
			release()
			return nil, ErrTargetDrift
		}
		name, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			release()
			return nil, err
		}
		const deleteAccess = 0x00010000
		h, err := syscall.CreateFile(name, deleteAccess, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			release()
			if errors.Is(err, syscall.Errno(32)) {
				return nil, fmt.Errorf("%w：%s；未更新", ErrUpgradeMembersBusy, member.Name)
			}
			return nil, fmt.Errorf("不能确认成员目录 %s 可用于更新：%w", member.Name, err)
		}
		handles = append(handles, h)
	}
	return release, nil
}
