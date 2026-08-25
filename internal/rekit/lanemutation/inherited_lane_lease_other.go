//go:build !windows

package lanemutation

import (
	"fmt"
	"os"
)

func (lease *Lease) DuplicateLaneLockForChild() (*os.File, error) {
	return nil, fmt.Errorf("inherited lane mutation lease proof is available only on Windows")
}

func OpenInheritedLaneLease(caseRoot, laneID string, handle uintptr) (*InheritedLaneLease, error) {
	return nil, fmt.Errorf("inherited lane mutation lease proof is available only on Windows")
}
