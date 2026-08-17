//go:build !windows && !linux && !darwin

package fs

import "os"

func openExactFile(*os.Root, string, bool) (exactFileHandle, error) {
	return nil, errAnchoredExactMutationUnsupported
}

func createExactFile(*os.Root, string, os.FileMode) (exactCreatedFile, error) {
	return nil, errAnchoredExactMutationUnsupported
}

func exactFileUnbroken(exactFileHandle) error {
	return errAnchoredExactMutationUnsupported
}

func openExactMutationGuard(string, string, string) (exactMutationGuard, error) {
	return nil, errAnchoredExactMutationUnsupported
}

func readExactFileData(exactFileHandle, int64) ([]byte, os.FileInfo, error) {
	return nil, nil, errAnchoredExactMutationUnsupported
}
