package sync

import "testing"

func TestCurrentSyncSelectApplyRouteIsClosed(t *testing.T) {
	tests := []struct {
		name      string
		artifacts currentSyncApplyArtifacts
		want      currentSyncApplyRoute
		wantError bool
	}{
		{name: "fresh", want: currentSyncApplyFresh},
		{
			name: "archived-publication-window",
			artifacts: currentSyncApplyArtifacts{
				ArchivedIntent: true,
			},
			want: currentSyncApplyRestoreActive,
		},
		{
			name: "active-before-journal",
			artifacts: currentSyncApplyArtifacts{
				ActiveIntent:   true,
				ArchivedIntent: true,
			},
			want: currentSyncApplyRestoreActive,
		},
		{
			name: "active-nonterminal",
			artifacts: currentSyncApplyArtifacts{
				ActiveIntent:   true,
				ArchivedIntent: true,
				Progress:       true,
			},
			want: currentSyncApplyResume,
		},
		{
			name: "active-terminal-cleanup",
			artifacts: currentSyncApplyArtifacts{
				ActiveIntent:     true,
				ArchivedIntent:   true,
				Progress:         true,
				ProgressTerminal: true,
				MatchingReceipt:  true,
			},
			want: currentSyncApplyCleanup,
		},
		{
			name: "lost-response-replay",
			artifacts: currentSyncApplyArtifacts{
				ArchivedIntent:   true,
				Progress:         true,
				ProgressTerminal: true,
				MatchingReceipt:  true,
			},
			want: currentSyncApplyReplay,
		},
		{
			name: "active-without-archive",
			artifacts: currentSyncApplyArtifacts{
				ActiveIntent: true,
			},
			wantError: true,
		},
		{
			name: "active-receipt-before-terminal",
			artifacts: currentSyncApplyArtifacts{
				ActiveIntent:    true,
				ArchivedIntent:  true,
				Progress:        true,
				MatchingReceipt: true,
			},
			wantError: true,
		},
		{
			name: "terminal-without-receipt",
			artifacts: currentSyncApplyArtifacts{
				ActiveIntent:     true,
				ArchivedIntent:   true,
				Progress:         true,
				ProgressTerminal: true,
			},
			wantError: true,
		},
		{
			name: "orphan-nonterminal-progress",
			artifacts: currentSyncApplyArtifacts{
				ArchivedIntent: true,
				Progress:       true,
			},
			wantError: true,
		},
		{
			name: "receipt-without-archive",
			artifacts: currentSyncApplyArtifacts{
				Progress:         true,
				ProgressTerminal: true,
				MatchingReceipt:  true,
			},
			wantError: true,
		},
		{
			name: "receipt-without-terminal",
			artifacts: currentSyncApplyArtifacts{
				ArchivedIntent:  true,
				Progress:        true,
				MatchingReceipt: true,
			},
			wantError: true,
		},
		{
			name: "terminal-flag-without-progress",
			artifacts: currentSyncApplyArtifacts{
				ArchivedIntent:   true,
				ProgressTerminal: true,
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := currentSyncSelectApplyRoute(test.artifacts)
			if test.wantError {
				if err == nil {
					t.Fatalf("current sync accepted invalid artifact state as %s", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("current sync apply route = %s err=%v, want %s", got, err, test.want)
			}
		})
	}
}
