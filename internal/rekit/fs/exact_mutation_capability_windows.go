//go:build windows

package fs

// HandleBoundExactMutationSupported reports whether exact writes, renames, and
// removals are bound to validated object handles on this platform.
func HandleBoundExactMutationSupported() bool {
	return true
}
