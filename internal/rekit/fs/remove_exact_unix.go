//go:build unix && !linux && !darwin

package fs

import (
	"io/fs"
	"os"
)

func removeExactObject(*os.Root, string, os.FileInfo, []byte, fs.FileMode, bool) error {
	return errAnchoredExactMutationUnsupported
}
