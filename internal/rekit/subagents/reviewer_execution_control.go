package subagents

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

func requireReviewerMutationLease(caseRoot, lane string, lease *lanemutation.Lease) error {
	state, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	if !state.Existing || state.Legacy {
		return nil
	}
	if lease == nil {
		return fmt.Errorf("current reviewer lifecycle mutation requires an existing lane mutation lease")
	}
	return lease.ValidateLaneFor(caseRoot, strings.TrimSpace(lane))
}

func requireReviewerDispatchControlCurrent(caseRoot string, lease *lanemutation.Lease, dispatch ReviewerSessionDispatchReceipt) error {
	state, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	if !state.Existing || state.Legacy {
		return nil
	}
	if dispatch.Capability != reviewersession.ReadOnlyCapability() {
		return fmt.Errorf("reviewer session dispatch does not carry the exact read-only capability contract")
	}
	if dispatch.LaunchControl == nil {
		return fmt.Errorf("current reviewer session dispatch lacks launch control lineage")
	}
	if lease == nil {
		return fmt.Errorf("current reviewer lifecycle mutation requires an existing lane mutation lease")
	}
	if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, *dispatch.LaunchControl); err != nil {
		return fmt.Errorf("reviewer session dispatch launch control is not current: %w", err)
	}
	return nil
}

func requireReviewerResultControlCurrent(
	caseRoot string,
	lease *lanemutation.Lease,
	packetPath string,
	packet Packet,
	packetBytes []byte,
	handoff ShardHandoff,
	result ReviewerResult,
) error {
	state, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	if !state.Existing || state.Legacy {
		return nil
	}
	_, _, dispatch, err := findReviewerResultInputDispatch(caseRoot, packetPath, packet, packetBytes, handoff, result)
	if err != nil {
		return err
	}
	return requireReviewerDispatchControlCurrent(caseRoot, lease, dispatch)
}
