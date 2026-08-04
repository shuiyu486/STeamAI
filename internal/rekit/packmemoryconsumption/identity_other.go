//go:build !unix && !windows

package packmemoryconsumption

import "fmt"

func caseRootIdentity(root rootIdentitySource) (CaseRootIdentity, error) {
	_ = root
	return CaseRootIdentity{}, fmt.Errorf("durable case-root identity is unsupported on this platform")
}
