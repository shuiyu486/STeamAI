//go:build linux

package fs

func renameNoReplaceExactNative(exactRenameRequest) error {
	return errAnchoredExactMutationUnsupported
}
