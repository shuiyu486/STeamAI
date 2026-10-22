//go:build windows

package steamai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const (
	movefileReplaceExisting = 0x1
	movefileWriteThrough    = 0x8
	fileReadAttributes      = 0x80
)

var (
	kernel32DLL    = syscall.NewLazyDLL("kernel32.dll")
	moveFileExProc = kernel32DLL.NewProc("MoveFileExW")
)

func moveFileEx(source, target *uint16, flags uint32) error {
	result, _, callErr := moveFileExProc.Call(
		uintptr(unsafe.Pointer(source)),
		uintptr(unsafe.Pointer(target)),
		uintptr(flags),
	)
	if result == 0 {
		return callErr
	}
	return nil
}

// claudeConfigLock 记录自己持有的锁目录身份：释放前核对创建时间，避免误删
// Claude Code 在 stale 接管后重建的锁。
type claudeConfigLock struct {
	dir          string
	creationTime syscall.Filetime
	lockedHere   bool
}

// acquireClaudeConfigLock 用与原生一致的 proper-lockfile 语义获取 `.claude.json.lock`
// 目录锁：目录创建成功即持有；已存在说明另一进程在写，短暂重试（总窗口远小于
// CLI 的 10 秒 stale 接管），仍忙则 fail-closed，绝不强制接管。
func acquireClaudeConfigLock(configPath string, timeout time.Duration) (claudeConfigLock, error) {
	lockDir := configPath + ".lock"
	pointer, err := syscall.UTF16PtrFromString(lockDir)
	if err != nil {
		return claudeConfigLock{}, err
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.CreateDirectory(pointer, nil)
		if err == nil {
			lock := claudeConfigLock{dir: lockDir, lockedHere: true}
			if creation, statErr := lockCreationTime(lockDir); statErr == nil {
				lock.creationTime = creation
			}
			return lock, nil
		}
		if !errors.Is(err, syscall.ERROR_ALREADY_EXISTS) && !errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
			return claudeConfigLock{}, fmt.Errorf("创建配置锁: %w", err)
		}
		if time.Now().After(deadline) {
			return claudeConfigLock{}, errConfigLockBusy
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// release 删除自己创建的锁目录。创建时间不一致说明锁已被原生进程接管，
// 此时不能删除它的锁。
func (lock claudeConfigLock) release() {
	if !lock.lockedHere {
		return
	}
	if creation, err := lockCreationTime(lock.dir); err != nil || creation != lock.creationTime {
		return
	}
	pointer, err := syscall.UTF16PtrFromString(lock.dir)
	if err != nil {
		return
	}
	_ = syscall.RemoveDirectory(pointer)
}

func lockCreationTime(lockDir string) (syscall.Filetime, error) {
	pointer, err := syscall.UTF16PtrFromString(lockDir)
	if err != nil {
		return syscall.Filetime{}, err
	}
	handle, err := syscall.CreateFile(pointer,
		fileReadAttributes, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return syscall.Filetime{}, err
	}
	defer syscall.CloseHandle(handle)
	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &data); err != nil {
		return syscall.Filetime{}, err
	}
	return data.CreationTime, nil
}

// replaceClaudeConfigAtomically 以 原子替换 的语义发布新内容：同卷 tmp 写入并
// 同步后 MoveFileEx REPLACE_EXISTING|WRITE_THROUGH；主文件要么完整旧内容、
// 要么完整新内容，失败时保持原件不动。
func replaceClaudeConfigAtomically(configPath string, contents []byte) error {
	tmp := filepath.Join(filepath.Dir(configPath), fmt.Sprintf(".claude.json.tmp.steamai.%d", os.Getpid()))
	defer os.Remove(tmp)
	pointer, err := syscall.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	handle, err := syscall.CreateFile(pointer,
		syscall.GENERIC_WRITE, 0, nil, syscall.CREATE_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return fmt.Errorf("创建临时文件: %w", err)
	}
	written := 0
	for written < len(contents) {
		var n uint32
		chunk := contents[written:]
		if len(chunk) > 1<<30 {
			chunk = chunk[:1<<30]
		}
		if err := syscall.WriteFile(handle, chunk, &n, nil); err != nil {
			syscall.CloseHandle(handle)
			return fmt.Errorf("写入临时文件: %w", err)
		}
		written += int(n)
	}
	if err := syscall.FlushFileBuffers(handle); err != nil {
		syscall.CloseHandle(handle)
		return fmt.Errorf("同步临时文件: %w", err)
	}
	if err := syscall.CloseHandle(handle); err != nil {
		return fmt.Errorf("关闭临时文件: %w", err)
	}
	source, err := syscall.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	target, err := syscall.UTF16PtrFromString(configPath)
	if err != nil {
		return err
	}
	if err := moveFileEx(source, target, movefileReplaceExisting|movefileWriteThrough); err != nil {
		return fmt.Errorf("原子替换配置: %w", err)
	}
	return nil
}
