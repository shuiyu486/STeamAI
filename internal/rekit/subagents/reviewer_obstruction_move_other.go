//go:build !windows

package subagents

import "fmt"

func reviewerResultObstructionMoveSupported() bool {
	return false
}

func moveReviewerResultObstructionExact(_, _, _ string, _ func() error) error {
	return fmt.Errorf("exact reviewer result obstruction move is unavailable on this platform")
}
