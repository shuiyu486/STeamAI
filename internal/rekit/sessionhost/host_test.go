package sessionhost

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveClaudePathRejectsWindowsCommandScripts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows executable resolution only")
	}
	path := filepath.Join(t.TempDir(), "claude.cmd")
	if err := os.WriteFile(path, []byte("@exit /b 0\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveClaudePath(path); err == nil || !strings.Contains(err.Error(), "native claude.exe") {
		t.Fatalf("command script resolution error=%v", err)
	}
}
