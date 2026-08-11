package cli

import (
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestSelectedLaneCommandBindsExactLane(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		command  string
		selected string
		want     string
	}{
		{
			name:     "insert before what-if",
			command:  `/rekit status -Target "C:\case root" -Format json`,
			selected: "main",
			want:     `/rekit status -Target "C:\case root" -Format json -Lane main`,
		},
		{
			name:     "normalize positional continue",
			command:  `/rekit continue -Target "C:\case root" main -Executor session-1 -ExpectedExecutorGeneration 1 -WhatIf -Format json`,
			selected: "main",
			want:     `/rekit continue -Target "C:\case root" -Executor session-1 -ExpectedExecutorGeneration 1 -Lane main -WhatIf -Format json`,
		},
		{
			name:     "normalize feature label continue",
			command:  `/rekit continue -Target "C:\case root" mission -Executor session-1 -ExpectedExecutorGeneration 1 -WhatIf -Format json`,
			selected: "feature-mission",
			want:     `/rekit continue -Target "C:\case root" -Executor session-1 -ExpectedExecutorGeneration 1 -Lane feature-mission -WhatIf -Format json`,
		},
		{
			name:     "keep positional complete",
			command:  `/rekit complete -Target "C:\case root" main -Summary done -WhatIf -Format json`,
			selected: "main",
			want:     `/rekit complete -Target "C:\case root" main -Summary done -WhatIf -Format json`,
		},
		{
			name:     "keep feature label reconcile",
			command:  `/rekit reconcile -Target "C:\case root" review -InterventionId event-1 -WhatIf -Format json`,
			selected: "feature-review",
			want:     `/rekit reconcile -Target "C:\case root" review -InterventionId event-1 -WhatIf -Format json`,
		},
		{
			name:     "keep exact flag idempotent",
			command:  `/rekit continue -Target "C:\case root" -Lane main -WhatIf -Format json`,
			selected: "main",
			want:     `/rekit continue -Target "C:\case root" -Lane main -WhatIf -Format json`,
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			got := selectedLaneCommand(fixture.command, fixture.selected)
			if got != fixture.want {
				t.Fatalf("selectedLaneCommand() = %q, want %q", got, fixture.want)
			}
			if again := selectedLaneCommand(got, fixture.selected); again != got {
				t.Fatalf("selectedLaneCommand() is not idempotent: first=%q second=%q", got, again)
			}
		})
	}
}

func TestSelectedLaneCommandFailsClosedOnLaneDrift(t *testing.T) {
	for _, command := range []string{
		`/rekit continue feature-login -WhatIf -Format json`,
		`/rekit continue login -WhatIf -Format json`,
		`/rekit continue -Lane feature-login -WhatIf -Format json`,
		`/rekit complete -Target "C:\case root" feature-login -WhatIf -Format json`,
	} {
		if got := selectedLaneCommand(command, "main"); got != "" {
			t.Fatalf("selected lane drift returned command %q for %q", got, command)
		}
	}
}

func TestSelectedLaneCommandPositionalParserSkipsBoundedFlagValues(t *testing.T) {
	fields, err := splitDriverCommand(`/rekit continue -Target "C:\case root" -Reason "keep main selected" main -WhatIf -Format json`)
	if err != nil {
		t.Fatal(err)
	}
	index, ok := selectedLaneCommandPositionalIndex(fields)
	if !ok || index >= len(fields) || strings.TrimSpace(fields[index]) != "main" {
		t.Fatalf("positional lane index=%d ok=%t fields=%q", index, ok, fields)
	}
	if !selectedLaneCommandValueFlag("-Target") || !selectedLaneCommandValueFlag("--expected-executor-generation") || selectedLaneCommandValueFlag("-WhatIf") {
		t.Fatal("selected lane value-flag allowlist drifted")
	}
}

func TestValidateStatusSelectedCurrentLaneFailsClosedOnNestedReviewerWaveDrift(t *testing.T) {
	const selected = "feature-review"
	for _, fixture := range []struct {
		name    string
		command string
	}{
		{
			name:    "conflicting lane",
			command: `/rekit reviewer-result -Target "C:\case root" -Lane feature-other -WhatIf -Format json`,
		},
		{
			name: "empty command",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			attempt := &mission.CurrentLoopReviewerAttempt{
				Identity: mission.CurrentLoopReviewerAttemptIdentity{
					Lane: selected,
				},
				SelectedAction: mission.CurrentLoopReviewerAttemptAction{
					ObservationContract: mission.CurrentLoopObservationContract{
						Alternatives: []mission.CurrentLoopObservationAlternative{
							{PreviewCommandTemplate: fixture.command},
						},
					},
				},
			}
			status := statusInventory{
				MissionControlRunbook: &statusMissionControlRunbook{
					Scope: "reviewer",
					CurrentDriverRequest: &mission.MissionCommanderDriverRequest{
						Lane:     selected,
						Guidance: "run the selected reviewer wave",
					},
					RefreshStatusCommand: `/rekit status -Target "C:\case root" -Format json -Lane feature-review`,
					CurrentLoopOperator: &mission.CurrentLoopOperatorPackage{
						Lane: selected,
						ExternalReviewerHandoff: &mission.CurrentLoopExternalReviewerHandoff{
							Wave: &mission.CurrentLoopReviewerWave{
								Lane:   selected,
								Shards: []*mission.CurrentLoopReviewerAttempt{attempt},
							},
						},
					},
				},
			}

			if err := validateStatusSelectedCurrentLane(status, selected, false); err == nil || !strings.Contains(err.Error(), "reviewer wave shard 1") {
				t.Fatalf("validateStatusSelectedCurrentLane() error = %v, want nested shard failure", err)
			}
		})
	}
}
