package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/testenv"
)

func TestMain(m *testing.M) {
	if _, err := testenv.ConfigureCanonicalTempRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	home, err := os.MkdirTemp("", "rekit-releasecheck-git-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, key := range os.Environ() {
		name, _, _ := strings.Cut(key, "=")
		name = strings.ToUpper(strings.TrimSpace(name))
		if strings.HasPrefix(name, "GIT_") {
			_ = os.Unsetenv(name)
		}
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home)
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	_ = os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	_ = os.Setenv("GIT_TERMINAL_PROMPT", "0")
	code := m.Run()
	if err := os.RemoveAll(home); err != nil && code == 0 {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}
