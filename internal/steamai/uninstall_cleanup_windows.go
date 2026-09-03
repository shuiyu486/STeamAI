//go:build windows

package steamai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const uninstallCleanupTimeout = 30 * time.Second

func runUninstallCleanup(args []string) error {
	if err := requireUninstallCleanupExecutable(); err != nil {
		return err
	}
	parentPID, paths, err := parseUninstallCleanupArgs(args)
	if err != nil {
		return err
	}
	if err := waitForProcessExit(uint32(parentPID), uninstallCleanupTimeout); err != nil {
		return err
	}
	deadline := time.Now().Add(uninstallCleanupTimeout)
	for _, path := range paths {
		for {
			err := os.Remove(path)
			if err == nil || os.IsNotExist(err) {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("删除卸载文件 %s: %w", path, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

func requireUninstallCleanupExecutable() error {
	current, err := os.Executable()
	if err != nil {
		return err
	}
	active, err := (nativePlatform{}).ActiveExecutable()
	if err != nil {
		return err
	}
	name := strings.ToLower(filepath.Base(current))
	if !sameExecutablePath(filepath.Dir(current), filepath.Dir(active)) ||
		!strings.HasPrefix(name, ".steamai-uninstall-helper-") || !strings.HasSuffix(name, ".exe") {
		return errors.New("内部 uninstall cleanup 只能由临时原生 helper 执行")
	}
	if err := requirePlainPath(filepath.Dir(active), current, false); err != nil {
		return errors.New("内部 uninstall cleanup 只能由临时原生 helper 执行")
	}
	return nil
}

func parseUninstallCleanupArgs(args []string) (int, []string, error) {
	if len(args) < 4 || args[0] != "--parent-pid" {
		return 0, nil, errors.New("内部 uninstall cleanup 参数无效")
	}
	parentPID, err := strconv.Atoi(args[1])
	if err != nil || parentPID <= 0 {
		return 0, nil, errors.New("内部 uninstall cleanup parent PID 无效")
	}
	paths := make([]string, 0, (len(args)-2)/2)
	for index := 2; index < len(args); index += 2 {
		if index+1 >= len(args) || args[index] != "--path" || args[index+1] == "" {
			return 0, nil, errors.New("内部 uninstall cleanup path 参数无效")
		}
		paths = append(paths, args[index+1])
	}
	if len(paths) == 0 {
		return 0, nil, errors.New("内部 uninstall cleanup 缺少 path")
	}
	active, err := (nativePlatform{}).ActiveExecutable()
	if err != nil {
		return 0, nil, err
	}
	binDir := filepath.Dir(active)
	allowed := map[string]bool{
		strings.ToLower(filepath.Clean(filepath.Join(binDir, "steamai.uninstalling.exe"))): true,
		strings.ToLower(filepath.Clean(filepath.Join(binDir, "steamai.previous.exe"))):     true,
		strings.ToLower(filepath.Clean(filepath.Join(binDir, "steamai.next.exe"))):         true,
	}
	seen := map[string]bool{}
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		key := strings.ToLower(filepath.Clean(absolute))
		if err != nil || !allowed[key] || seen[key] {
			return 0, nil, errors.New("内部 uninstall cleanup path 不在安装目录 allowlist")
		}
		if _, statErr := os.Lstat(absolute); statErr == nil {
			if err := requirePlainPath(binDir, absolute, false); err != nil {
				return 0, nil, errors.New("内部 uninstall cleanup path 不是普通安装文件")
			}
		} else if !os.IsNotExist(statErr) {
			return 0, nil, statErr
		}
		seen[key] = true
	}
	return parentPID, paths, nil
}

func waitForProcessExit(pid uint32, timeout time.Duration) error {
	const synchronize = 0x00100000
	handle, err := syscall.OpenProcess(synchronize, false, pid)
	if err != nil {
		if errors.Is(err, syscall.Errno(87)) {
			return nil
		}
		return err
	}
	defer syscall.CloseHandle(handle)
	result, err := syscall.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	if err != nil {
		return err
	}
	switch result {
	case syscall.WAIT_OBJECT_0:
		return nil
	case syscall.WAIT_TIMEOUT:
		return errors.New("等待父 steamai.exe 退出超时")
	default:
		return fmt.Errorf("等待父 steamai.exe 返回未知状态 %#x", result)
	}
}
