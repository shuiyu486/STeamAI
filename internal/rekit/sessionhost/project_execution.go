package sessionhost

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func supervisionHandoffRequired(opt Options) (bool, error) {
	return projectExecutionHandoffRequired(
		opt.Target,
		opt.projectExecutionLease,
	)
}

func projectExecutionHandoffRequired(
	value string,
	lease *projectexecution.Lease,
) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}
	caseRoot, err := filepath.Abs(value)
	if err != nil {
		return false, err
	}
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return false, err
	}
	current := stateRoot.Existing && !stateRoot.Legacy &&
		stateRoot.Dir == projectstate.CurrentDir
	if !current {
		if lease != nil {
			return false, fmt.Errorf(
				"legacy or unattached Claude supervision must not carry a project execution lease",
			)
		}
		return false, nil
	}
	if lease == nil {
		return false, fmt.Errorf(
			"current Claude supervision requires the parent shared project execution lease",
		)
	}
	if err := lease.ValidateFor(caseRoot); err != nil {
		return false, err
	}
	return true, nil
}

func acquireSharedForCurrentProject(value string) (*projectexecution.Lease, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	caseRoot, err := filepath.Abs(value)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(caseRoot); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return nil, err
	}
	if !stateRoot.Existing || stateRoot.Legacy || stateRoot.Dir != projectstate.CurrentDir {
		return nil, nil
	}
	return projectexecution.AcquireShared(caseRoot)
}
