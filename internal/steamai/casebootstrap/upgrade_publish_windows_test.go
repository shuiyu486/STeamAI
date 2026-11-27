//go:build windows

package casebootstrap

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

var upgradeTestShortPathName = syscall.NewLazyDLL("kernel32.dll").NewProc("GetShortPathNameW")

// 目标被别的进程以不带 FILE_SHARE_DELETE 的句柄占用时，移出原件会失败：发布中途失败并完整撤回，
// 被占用的文件与之前已发布的文件都保持原样，也不留下新目录。
func TestPublishUpgradeRollsBackWhenTargetIsInUse(t *testing.T) {
	root := upgradePublishFixture(t)
	changes := upgradeFixtureChanges(t, root)
	before := upgradeTree(t, root)
	name, err := syscall.UTF16PtrFromString(filepath.Join(root, filepath.FromSlash(upgradeFixturePolicy)))
	if err != nil {
		t.Fatal(err)
	}
	var handle syscall.Handle
	upgradeFixtureHook(t, func(_ int, rel string) {
		if rel == upgradeFixtureLearning {
			// 读取仍被允许（前态复核能过），但改名/删除会被拒绝。
			opened, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			handle = opened
		}
	})
	t.Cleanup(func() {
		if handle != 0 {
			_ = syscall.CloseHandle(handle)
		}
	})
	err = publishUpgrade(root, changes, func() error { return errors.New("不应到达深验") })
	if err == nil || errors.Is(err, ErrUpgradePreserved) || !strings.Contains(err.Error(), upgradeFixturePolicy) {
		t.Fatalf("目标被占用时返回 %v", err)
	}
	assertUpgradeTree(t, root, before)
}

// 8.3 短文件名在 Lstat/ReadFile 下与长名无异，只有目录枚举能识别：用它指代已有文件时按前态漂移拒绝。
func TestPublishUpgradeRejectsShortNameAlias(t *testing.T) {
	root := upgradePublishFixture(t)
	const longName = "AVeryLongContractDocumentName.md"
	full := filepath.Join(root, ".steamai-vnext", "contracts", longName)
	writeFile(t, full, []byte("# long\n"))
	pointer, err := syscall.UTF16PtrFromString(full)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 1024)
	length, _, _ := upgradeTestShortPathName.Call(
		uintptr(unsafe.Pointer(pointer)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)),
	)
	short := syscall.UTF16ToString(buffer[:length])
	if length == 0 || strings.EqualFold(short, full) {
		t.Skip("此卷不生成 8.3 短文件名")
	}
	aliased := upgradeFile{Path: ".steamai-vnext/contracts/" + filepath.Base(short), Before: hashBytes([]byte("# long\n")),
		After: hashBytes([]byte("# new\n")), Data: []byte("# new\n")}
	changes := []upgradeFile{aliased, upgradeFixtureReplace(t, root, snapshotRel, "schema: new\n")}
	before := upgradeTree(t, root)
	err = publishUpgrade(root, changes, func() error { return nil })
	if !errors.Is(err, ErrTargetDrift) || !strings.Contains(err.Error(), aliased.Path) {
		t.Fatalf("短文件名别名返回 %v", err)
	}
	assertUpgradeTree(t, root, before)
}
