//go:build windows

package promote

import "syscall"

const processQueryLimitedInformation = 0x1000

func candidateDecisionProcessAlive(pid int) bool {
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(handle)
	return true
}
