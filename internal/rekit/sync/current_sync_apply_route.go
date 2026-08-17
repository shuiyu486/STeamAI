package sync

import "fmt"

type currentSyncApplyRoute string

const (
	currentSyncApplyFresh         currentSyncApplyRoute = "fresh"
	currentSyncApplyRestoreActive currentSyncApplyRoute = "restore-active-intent"
	currentSyncApplyResume        currentSyncApplyRoute = "resume-forward"
	currentSyncApplyCleanup       currentSyncApplyRoute = "strict-replay-and-cleanup"
	currentSyncApplyReplay        currentSyncApplyRoute = "strict-replay"
)

type currentSyncApplyArtifacts struct {
	ActiveIntent     bool
	ArchivedIntent   bool
	Progress         bool
	ProgressTerminal bool
	MatchingReceipt  bool
}

func currentSyncSelectApplyRoute(
	artifacts currentSyncApplyArtifacts,
) (currentSyncApplyRoute, error) {
	if artifacts.ProgressTerminal && !artifacts.Progress {
		return "", fmt.Errorf(
			"current sync terminal progress is present without a durable journal",
		)
	}
	if artifacts.ActiveIntent {
		if !artifacts.ArchivedIntent {
			return "", fmt.Errorf(
				"current sync active intent has no exact archived transaction",
			)
		}
		if artifacts.MatchingReceipt && !artifacts.ProgressTerminal {
			return "", fmt.Errorf(
				"current sync committed receipt precedes terminal progress",
			)
		}
		if artifacts.ProgressTerminal {
			if !artifacts.MatchingReceipt {
				return "", fmt.Errorf(
					"current sync terminal journal has no matching committed receipt",
				)
			}
			return currentSyncApplyCleanup, nil
		}
		if !artifacts.Progress {
			return currentSyncApplyRestoreActive, nil
		}
		return currentSyncApplyResume, nil
	}
	if artifacts.MatchingReceipt {
		if !artifacts.ArchivedIntent || !artifacts.ProgressTerminal {
			return "", fmt.Errorf(
				"current sync committed receipt lacks its exact terminal transaction",
			)
		}
		return currentSyncApplyReplay, nil
	}
	if artifacts.Progress {
		return "", fmt.Errorf(
			"current sync nonterminal journal exists without its active intent",
		)
	}
	if artifacts.ArchivedIntent {
		return currentSyncApplyRestoreActive, nil
	}
	return currentSyncApplyFresh, nil
}
