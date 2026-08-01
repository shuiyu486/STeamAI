package testenv

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigureCanonicalTempRoot keeps strict path-identity tests out of platform
// system temp aliases such as macOS /var -> /private/var.
func ConfigureCanonicalTempRoot() (string, error) {
	root, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", fmt.Errorf("resolve test temp root: %w", err)
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		if err := os.Setenv(name, root); err != nil {
			return "", fmt.Errorf("set canonical test temp root %s: %w", name, err)
		}
	}
	return root, nil
}
