//go:build windows

package currentloop

import "os"

func syncCurrentLoopDirectory(string) error {
	return nil
}

func syncCurrentLoopRoot(*os.Root) error {
	return nil
}
