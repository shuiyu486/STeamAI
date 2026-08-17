//go:build !linux && !darwin && !windows

package fs

func renameNoReplaceExactNative(exactRenameRequest) error {
	return errAnchoredExactMutationUnsupported
}
