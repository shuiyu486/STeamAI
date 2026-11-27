//go:build !windows

package casebootstrap

import "os"

// 此平台不是产品入口；文件操作同样不允许覆盖已有目标。
func upgradeMove(source, target string) error {
	if err := os.Link(source, target); err != nil {
		return err
	}
	return os.Remove(source)
}
