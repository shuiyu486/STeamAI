package sync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

type currentSyncRefreshLeaseFixture struct {
	events      *[]string
	name        string
	validateErr error
	unlockErr   error
}

func (lease *currentSyncRefreshLeaseFixture) Validate() error {
	*lease.events = append(*lease.events, lease.name+"-validate")
	return lease.validateErr
}

func (lease *currentSyncRefreshLeaseFixture) Unlock() error {
	*lease.events = append(*lease.events, lease.name+"-unlock")
	return lease.unlockErr
}

func TestAcquireCurrentSyncRefreshLeaseReleasesKitWhenLaneAcquireFails(t *testing.T) {
	laneAcquireErr := errors.New("lane acquire fixture")
	kitUnlockErr := errors.New("kit unlock fixture")
	var events []string

	lease, err := acquireCurrentSyncRefreshLeaseWith(
		"case-root",
		func(caseRoot string) (currentSyncLaneRefreshLease, error) {
			events = append(events, "execution-acquire:"+caseRoot)
			return &currentSyncRefreshLeaseFixture{events: &events, name: "execution"}, nil
		},
		func(caseRoot string) (currentSyncRefreshUnlocker, error) {
			events = append(events, "kit-acquire:"+caseRoot)
			return &currentSyncRefreshLeaseFixture{events: &events, name: "kit", unlockErr: kitUnlockErr}, nil
		},
		func(caseRoot string) (currentSyncLaneRefreshLease, error) {
			events = append(events, "lane-acquire:"+caseRoot)
			return nil, laneAcquireErr
		},
	)
	if lease != nil {
		t.Fatal("lane acquisition failure returned a refresh lease")
	}
	if !errors.Is(err, laneAcquireErr) || !errors.Is(err, kitUnlockErr) {
		t.Fatalf("acquisition error = %v, want lane acquisition and kit cleanup errors", err)
	}
	if got, want := strings.Join(events, ","), "execution-acquire:case-root,kit-acquire:case-root,lane-acquire:case-root,kit-unlock,execution-unlock"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
}

func TestCurrentSyncRefreshLeaseUnlocksInReverseOrderAndJoinsErrors(t *testing.T) {
	laneUnlockErr := errors.New("lane unlock fixture")
	kitUnlockErr := errors.New("kit unlock fixture")
	var events []string

	lease, err := acquireCurrentSyncRefreshLeaseWith(
		"case-root",
		func(string) (currentSyncLaneRefreshLease, error) {
			events = append(events, "execution-acquire")
			return &currentSyncRefreshLeaseFixture{events: &events, name: "execution"}, nil
		},
		func(string) (currentSyncRefreshUnlocker, error) {
			events = append(events, "kit-acquire")
			return &currentSyncRefreshLeaseFixture{events: &events, name: "kit", unlockErr: kitUnlockErr}, nil
		},
		func(string) (currentSyncLaneRefreshLease, error) {
			events = append(events, "lane-acquire")
			return &currentSyncRefreshLeaseFixture{events: &events, name: "lane", unlockErr: laneUnlockErr}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Unlock(); !errors.Is(err, laneUnlockErr) || !errors.Is(err, kitUnlockErr) {
		t.Fatalf("unlock error = %v, want lane and kit errors", err)
	}
	if got, want := strings.Join(events, ","), "execution-acquire,kit-acquire,lane-acquire,lane-unlock,kit-unlock,execution-unlock"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
}

func TestCurrentSyncRefreshLeaseValidateDelegatesToLaneLease(t *testing.T) {
	validateErr := errors.New("lane validate fixture")
	var events []string
	lease := &currentSyncRefreshLease{
		executionLease: &currentSyncRefreshLeaseFixture{events: &events, name: "execution"},
		kitLease:       &currentSyncRefreshLeaseFixture{events: &events, name: "kit"},
		laneLease:      &currentSyncRefreshLeaseFixture{events: &events, name: "lane", validateErr: validateErr},
	}

	if err := lease.Validate(); !errors.Is(err, validateErr) {
		t.Fatalf("validate error = %v, want %v", err, validateErr)
	}
	if got, want := strings.Join(events, ","), "execution-validate,lane-validate"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
}

func TestAcquireCurrentSyncRefreshLeaseAllowsInitialAndPendingIntent(t *testing.T) {
	for _, pending := range []bool{false, true} {
		name := "initial"
		if pending {
			name = "pending-intent"
		}
		t.Run(name, func(t *testing.T) {
			caseRoot := t.TempDir()
			stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
			if err := os.MkdirAll(stateRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			if pending {
				intentPath := filepath.Join(stateRoot, filepath.FromSlash(currentSyncIntentRel))
				if err := os.MkdirAll(filepath.Dir(intentPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(intentPath, []byte("{}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			lease, err := acquireCurrentSyncRefreshLease(caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if err := lease.Validate(); err != nil {
				_ = lease.Unlock()
				t.Fatal(err)
			}
			if err := lease.Unlock(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
