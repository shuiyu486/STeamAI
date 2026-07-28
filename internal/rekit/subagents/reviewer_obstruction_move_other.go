//go:build !windows

package subagents

import "fmt"

func reviewerResultExactMoveSupported(string) bool {
	return false
}

func moveReviewerResultExact(_, _, _ string, _ reviewerResultExactMoveExpectation, _ func() error) error {
	return fmt.Errorf("exact reviewer result move is unavailable on this platform")
}
