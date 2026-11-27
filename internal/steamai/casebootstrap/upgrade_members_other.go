//go:build !windows

package casebootstrap

import "errors"

func holdUpgradeMemberDirs(string, []RosterMember) (func(), error) {
	return nil, errors.New("项目更新仅支持 Windows 10/11 x64")
}
