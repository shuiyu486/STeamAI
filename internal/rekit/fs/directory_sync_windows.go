//go:build windows

package fs

import "os"

// Windows does not expose directory FlushFileBuffers through os.File.Sync.
// Exclusive creation and file.Sync still provide process-level no-overwrite
// semantics; Unix receives the directory-entry durability barrier.
func syncPublishedDirectory(*os.Root) error {
	return nil
}
