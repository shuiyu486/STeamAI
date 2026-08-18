//go:build windows

package promote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func promotePhysicalPath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	buffer := make([]uint16, 32768)
	length, err := windows.GetFinalPathNameByHandle(windows.Handle(file.Fd()), &buffer[0], uint32(len(buffer)), 0)
	if err != nil || length == 0 || length >= uint32(len(buffer)) {
		return "", fmt.Errorf("resolve final directory path: %w", err)
	}
	resolved := syscall.UTF16ToString(buffer[:length])
	resolved = strings.TrimPrefix(resolved, `\\?\`)
	if strings.HasPrefix(strings.ToUpper(resolved), `UNC\`) {
		resolved = `\\` + resolved[len(`UNC\`):]
	}
	return filepath.Abs(resolved)
}
