//go:build windows

package steamai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestLiveWindowsUpdateLockedFileRecovery(t *testing.T) {
	if os.Getenv("STEAMAI_WINDOWS_UPDATE_LOCK_LIVE") != "1" {
		t.Skip("真实 Windows 文件锁恢复验收需显式设置 STEAMAI_WINDOWS_UPDATE_LOCK_LIVE=1")
	}
	// 只调用生产文件恢复路径；不调用 ActivateUpdate，不访问 Registry 或真实安装。
	t.Log("范围：临时文件/目录的真实共享锁与生产恢复函数；不代表完整安装、Registry 或 Release 验收")
	for _, locked := range []string{"active", "previous"} {
		t.Run("executable/"+locked, func(t *testing.T) {
			root := t.TempDir()
			active := filepath.Join(root, "steamai.exe")
			previous := filepath.Join(root, "steamai.previous.exe")
			writeLockedUpdateFixture(t, active, "new executable")
			writeLockedUpdateFixture(t, previous, "old executable")
			lockPath := active
			if locked == "previous" {
				lockPath = previous
			}
			release := lockUpdateFixture(t, lockPath, false)
			err := rollbackUpdatedExecutable(active, previous)
			assertRealUpdateLockError(t, err, previous)
			assertLockedUpdateContents(t, previous, "old executable")
			if locked == "active" {
				assertRealUpdateLockError(t, err, active)
				assertLockedUpdateContents(t, active, "new executable")
			} else {
				assertLockedUpdateMissing(t, active)
			}
			t.Logf("真实 %s 共享锁阻止恢复，生产错误保留精确恢复路径", locked)

			release()
			if err := rollbackUpdatedExecutable(active, previous); err != nil {
				t.Fatalf("解除本次锁后恢复旧 executable 失败：%v", err)
			}
			assertLockedUpdateContents(t, active, "old executable")
			assertLockedUpdateMissing(t, previous)
			t.Log("解除本次创建的锁后，旧 executable 原字节恢复，previous 不再存在")
		})
	}
	for _, locked := range []string{"published", "backup"} {
		t.Run("source/"+locked, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			staged := filepath.Join(root, "source.staged")
			backup := filepath.Join(root, "source.steamai-update-backup")
			writeLockedUpdateFixture(t, filepath.Join(source, "marker"), "new source")
			writeLockedUpdateFixture(t, filepath.Join(backup, "marker"), "old source")
			lockPath := source
			if locked == "backup" {
				lockPath = backup
			}
			release := lockUpdateFixture(t, lockPath, true)
			err := rollbackUpdatedSource(source, staged, backup, true)
			assertRealUpdateLockError(t, err, backup)
			assertLockedUpdateContents(t, filepath.Join(backup, "marker"), "old source")
			published := locked == "published"
			if published {
				assertRealUpdateLockError(t, err, source)
				assertLockedUpdateContents(t, filepath.Join(source, "marker"), "new source")
				assertLockedUpdateMissing(t, staged)
			} else {
				assertRealUpdateLockError(t, err, staged)
				assertLockedUpdateContents(t, filepath.Join(staged, "marker"), "new source")
				assertLockedUpdateMissing(t, source)
			}
			t.Logf("真实 %s 目录共享锁阻止重命名，新旧 source 均留在错误报告指定位置", locked)

			release()
			// 第二种失败已经把新 source 移回 staged，按实际文件状态恢复，不重复发布。
			if err := rollbackUpdatedSource(source, staged, backup, published); err != nil {
				t.Fatalf("解除本次锁后恢复旧 source 失败：%v", err)
			}
			assertLockedUpdateContents(t, filepath.Join(source, "marker"), "old source")
			assertLockedUpdateContents(t, filepath.Join(staged, "marker"), "new source")
			assertLockedUpdateMissing(t, backup)
			t.Log("解除本次创建的锁后旧 source 恢复，新 source 仍保留于 staged")
		})
	}
}

func lockUpdateFixture(t *testing.T, path string, directory bool) func() {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	flags := uint32(syscall.FILE_ATTRIBUTE_NORMAL)
	if directory {
		flags = syscall.FILE_FLAG_BACKUP_SEMANTICS
	}
	// 真实 Win32 handle 只共享读取，拒绝写入和删除/重命名共享；不注入模拟错误。
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, flags, 0)
	if err != nil {
		t.Fatalf("创建临时验收共享锁失败：%v", err)
	}
	released := false
	release := func() {
		t.Helper()
		if released {
			return
		}
		if err := syscall.CloseHandle(handle); err != nil {
			t.Fatalf("关闭本次验收 handle 失败：%v", err)
		}
		released = true
	}
	t.Cleanup(release)
	return release
}

func writeLockedUpdateFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRealUpdateLockError(t *testing.T, err error, recoveryPath string) {
	t.Helper()
	if err == nil || (!errors.Is(err, syscall.Errno(32)) && !errors.Is(err, syscall.ERROR_ACCESS_DENIED)) {
		t.Fatalf("真实共享锁未触发预期 Windows 锁错误：%v", err)
	}
	if !strings.Contains(err.Error(), recoveryPath) {
		t.Fatalf("生产恢复错误未包含精确保留路径 %s：%v", recoveryPath, err)
	}
}

func assertLockedUpdateContents(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != want {
		t.Fatalf("恢复文件 %s 的原字节未保留：%q，%v", path, data, err)
	}
}

func assertLockedUpdateMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("恢复后路径状态错误，应不存在 %s：%v", path, err)
	}
}
