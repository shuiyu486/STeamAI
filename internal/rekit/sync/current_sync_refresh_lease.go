package sync

import (
	"errors"

	"github.com/shuiyu486/re-context-kits/internal/rekit/kitmutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
)

type currentSyncRefreshUnlocker interface {
	Unlock() error
}

type currentSyncLaneRefreshLease interface {
	currentSyncRefreshUnlocker
	Validate() error
}

type currentSyncRefreshLease struct {
	executionLease currentSyncLaneRefreshLease
	kitLease       currentSyncRefreshUnlocker
	laneLease      currentSyncLaneRefreshLease
}

type currentSyncExecutionAcquire func(string) (currentSyncLaneRefreshLease, error)
type currentSyncKitRefreshAcquire func(string) (currentSyncRefreshUnlocker, error)
type currentSyncLaneRefreshAcquire func(string) (currentSyncLaneRefreshLease, error)

func acquireCurrentSyncRefreshLease(caseRoot string) (*currentSyncRefreshLease, error) {
	return acquireCurrentSyncRefreshLeaseWith(
		caseRoot,
		func(caseRoot string) (currentSyncLaneRefreshLease, error) {
			return projectexecution.AcquireExclusive(caseRoot)
		},
		func(caseRoot string) (currentSyncRefreshUnlocker, error) {
			return kitmutation.AcquireProjectRefresh(caseRoot)
		},
		func(caseRoot string) (currentSyncLaneRefreshLease, error) {
			return lanemutation.AcquireProjectRefresh(caseRoot)
		},
	)
}

func acquireCurrentSyncRefreshLeaseWith(
	caseRoot string,
	acquireExecution currentSyncExecutionAcquire,
	acquireKit currentSyncKitRefreshAcquire,
	acquireLane currentSyncLaneRefreshAcquire,
) (*currentSyncRefreshLease, error) {
	executionLease, err := acquireExecution(caseRoot)
	if err != nil {
		return nil, err
	}
	kitLease, err := acquireKit(caseRoot)
	if err != nil {
		return nil, errors.Join(err, executionLease.Unlock())
	}
	laneLease, err := acquireLane(caseRoot)
	if err != nil {
		return nil, errors.Join(
			err,
			kitLease.Unlock(),
			executionLease.Unlock(),
		)
	}
	return &currentSyncRefreshLease{
		executionLease: executionLease,
		kitLease:       kitLease,
		laneLease:      laneLease,
	}, nil
}

func (lease *currentSyncRefreshLease) Validate() error {
	if lease == nil || lease.executionLease == nil || lease.laneLease == nil {
		return errors.New("current sync refresh lease is not held")
	}
	return errors.Join(
		lease.executionLease.Validate(),
		lease.laneLease.Validate(),
	)
}

func (lease *currentSyncRefreshLease) Unlock() error {
	if lease == nil {
		return nil
	}
	laneLease := lease.laneLease
	kitLease := lease.kitLease
	executionLease := lease.executionLease
	lease.laneLease = nil
	lease.kitLease = nil
	lease.executionLease = nil

	var laneErr error
	if laneLease != nil {
		laneErr = laneLease.Unlock()
	}
	var kitErr error
	if kitLease != nil {
		kitErr = kitLease.Unlock()
	}
	var executionErr error
	if executionLease != nil {
		executionErr = executionLease.Unlock()
	}
	return errors.Join(laneErr, kitErr, executionErr)
}
