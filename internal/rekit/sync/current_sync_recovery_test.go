package sync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestInspectCurrentSyncRecoveryClassifiesDurableRoutes(t *testing.T) {
	requireCurrentSyncApplyForTest(t)
	tests := []struct {
		name  string
		stage string
		state string
	}{
		{
			name:  "restore",
			stage: "after-intent-publication",
			state: CurrentSyncRecoveryRestore,
		},
		{
			name:  "resume",
			stage: "after-initial-progress-publication",
			state: CurrentSyncRecoveryResume,
		},
		{
			name:  "cleanup",
			stage: "before-terminal-cleanup",
			state: CurrentSyncRecoveryCleanup,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCurrentSyncFixture(t, "")
			opt := CurrentSyncOptions{
				Command:          "sync",
				ProjectName:      "recovery-" + test.name,
				SourceExecutable: fixture.sourceExecutable,
			}
			plan, err := CurrentSyncPreview(
				fixture.repoRoot,
				fixture.caseRoot,
				fixture.pack,
				opt,
			)
			if err != nil {
				t.Fatal(err)
			}
			restore := SetCurrentSyncApplyTransitionHookForTest(
				func(stage string, _ CurrentSyncPlan) error {
					if stage == test.stage {
						return errors.New("simulated recovery interruption")
					}
					return nil
				},
			)
			_, err = CurrentSyncApply(
				fixture.caseRoot,
				fixture.pack,
				CurrentSyncApplyOptions{
					SourceRepoRoot:     fixture.repoRoot,
					ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
					CurrentSyncOptions: opt,
				},
			)
			restore()
			if err == nil || !strings.Contains(err.Error(), "simulated recovery interruption") {
				t.Fatalf("current sync did not stop at %s: %v", test.stage, err)
			}

			recovery, err := InspectCurrentSyncRecovery(fixture.caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if recovery.State != test.state || !recovery.Pending || !recovery.Blocked ||
				!recovery.Recoverable || recovery.Pack != fixture.pack ||
				len(recovery.ApplyArgs) == 0 || recovery.Diagnostic != "" {
				t.Fatalf("unexpected %s recovery: %+v", test.name, recovery)
			}
		})
	}
}

func currentSyncRecoveryExecutableFixture(t *testing.T) currentSyncFixtureState {
	t.Helper()
	requireCurrentSyncApplyForTest(t)
	fixture := newCurrentSyncFixture(t, "")
	opt := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "recovery-executable",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		opt,
	)
	if err != nil {
		t.Fatal(err)
	}
	restore := SetCurrentSyncApplyTransitionHookForTest(
		func(stage string, _ CurrentSyncPlan) error {
			if stage == "after-intent-publication" {
				return errors.New("simulated executable recovery interruption")
			}
			return nil
		},
	)
	_, applyErr := CurrentSyncApply(
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncApplyOptions{
			SourceRepoRoot:     fixture.repoRoot,
			ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
			CurrentSyncOptions: opt,
		},
	)
	restore()
	if applyErr == nil || !strings.Contains(
		applyErr.Error(),
		"simulated executable recovery interruption",
	) {
		t.Fatalf("current sync executable fixture did not stop: %v", applyErr)
	}
	return fixture
}

func TestValidateCurrentSyncRecoveryExecutableBindsDurableOldAndNewBytes(t *testing.T) {
	for _, test := range []struct {
		name        string
		useNewBytes bool
	}{
		{name: "old runtime"},
		{name: "new runtime", useNewBytes: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := currentSyncRecoveryExecutableFixture(t)
			if test.useNewBytes {
				data, err := os.ReadFile(fixture.sourceExecutable)
				if err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(fixture.sourceExecutable)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(
					fixture.targetExecutable,
					data,
					info.Mode().Perm(),
				); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(
					fixture.targetExecutable,
					info.Mode().Perm(),
				); err != nil {
					t.Fatal(err)
				}
			}
			if err := ValidateCurrentSyncRecoveryExecutable(
				fixture.caseRoot,
				fixture.targetExecutable,
			); err != nil {
				t.Fatalf("validate current sync recovery executable: %v", err)
			}
		})
	}
}

func TestValidateCurrentSyncRecoveryExecutableFailsClosed(t *testing.T) {
	t.Run("byte drift", func(t *testing.T) {
		fixture := currentSyncRecoveryExecutableFixture(t)
		if err := os.WriteFile(
			fixture.targetExecutable,
			[]byte("drifted recovery runtime\n"),
			0o700,
		); err != nil {
			t.Fatal(err)
		}
		err := ValidateCurrentSyncRecoveryExecutable(
			fixture.caseRoot,
			fixture.targetExecutable,
		)
		if err == nil || !strings.Contains(err.Error(), "bytes do not match") {
			t.Fatalf("byte drift error = %v", err)
		}
	})

	t.Run("wrong project", func(t *testing.T) {
		fixture := currentSyncRecoveryExecutableFixture(t)
		other := newCurrentSyncFixture(t, "")
		err := ValidateCurrentSyncRecoveryExecutable(
			fixture.caseRoot,
			other.targetExecutable,
		)
		if err == nil || !strings.Contains(err.Error(), "does not belong to the exact target project") {
			t.Fatalf("wrong project error = %v", err)
		}
	})

	t.Run("no pending transaction", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		err := ValidateCurrentSyncRecoveryExecutable(
			fixture.caseRoot,
			fixture.targetExecutable,
		)
		if err == nil || !strings.Contains(err.Error(), "no pending durable transaction") {
			t.Fatalf("no pending transaction error = %v", err)
		}
	})

	t.Run("corrupt namespace", func(t *testing.T) {
		fixture := currentSyncRecoveryExecutableFixture(t)
		activePath := filepath.Join(
			fixture.caseRoot,
			projectstate.CurrentDir,
			filepath.FromSlash(currentSyncIntentRel),
		)
		if err := os.WriteFile(activePath, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := ValidateCurrentSyncRecoveryExecutable(
			fixture.caseRoot,
			fixture.targetExecutable,
		)
		if err == nil || !strings.Contains(err.Error(), "ownership is invalid") {
			t.Fatalf("corrupt namespace error = %v", err)
		}
	})

	t.Run("non-canonical path", func(t *testing.T) {
		fixture := currentSyncRecoveryExecutableFixture(t)
		otherPath := filepath.Join(
			fixture.caseRoot,
			projectstate.CurrentDir,
			"runtime",
			"recovery-bin",
			filepath.Base(fixture.targetExecutable),
		)
		if err := os.MkdirAll(filepath.Dir(otherPath), 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(fixture.targetExecutable)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(otherPath, data, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := ValidateCurrentSyncRecoveryExecutable(
			fixture.caseRoot,
			otherPath,
		); err == nil {
			t.Fatal("non-canonical recovery executable path was accepted")
		}
	})

	t.Run("non-regular executable", func(t *testing.T) {
		fixture := currentSyncRecoveryExecutableFixture(t)
		if err := os.Remove(fixture.targetExecutable); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(fixture.targetExecutable, 0o700); err != nil {
			t.Fatal(err)
		}
		err := ValidateCurrentSyncRecoveryExecutable(
			fixture.caseRoot,
			fixture.targetExecutable,
		)
		if err == nil || !strings.Contains(err.Error(), "bounded regular file") ||
			strings.Contains(err.Error(), "%!w") {
			t.Fatalf("non-regular executable error = %v", err)
		}
	})
}

func TestInspectCurrentSyncRecoveryReturnsTypedInvalidState(t *testing.T) {
	requireCurrentSyncApplyForTest(t)
	fixture := newCurrentSyncFixture(t, "")
	opt := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "invalid-recovery",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		opt,
	)
	if err != nil {
		t.Fatal(err)
	}
	restore := SetCurrentSyncApplyTransitionHookForTest(
		func(stage string, _ CurrentSyncPlan) error {
			if stage == "after-intent-publication" {
				return errors.New("simulated invalid recovery setup")
			}
			return nil
		},
	)
	_, err = CurrentSyncApply(
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncApplyOptions{
			SourceRepoRoot:     fixture.repoRoot,
			ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
			CurrentSyncOptions: opt,
		},
	)
	restore()
	if err == nil {
		t.Fatal("current sync invalid recovery fixture did not stop")
	}
	activePath := filepath.Join(
		fixture.caseRoot,
		projectstate.CurrentDir,
		filepath.FromSlash(currentSyncIntentRel),
	)
	if err := os.WriteFile(activePath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	recovery, err := InspectCurrentSyncRecovery(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.State != CurrentSyncRecoveryInvalid || !recovery.Pending ||
		!recovery.Blocked || recovery.Recoverable || recovery.Diagnostic == "" ||
		len(recovery.ApplyArgs) != 0 {
		t.Fatalf("unexpected invalid recovery: %+v", recovery)
	}
}
