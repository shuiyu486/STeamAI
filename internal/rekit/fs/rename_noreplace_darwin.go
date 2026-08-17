//go:build darwin

package fs

func renameNoReplaceExactNative(exactRenameRequest) error {
	return errAnchoredExactMutationUnsupported
}
