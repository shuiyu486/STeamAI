package missionintent

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAttachedSnapshotContract(t *testing.T) {
	identity := Identity{SchemaVersion: 1, Target: filepath.Join(t.TempDir(), "case"), Pack: "_template", ProjectName: "demo", Goal: "goal", Actor: "actor", Executor: "executor", InitialLane: "feature-mission"}
	valid := RecoveryEnvelope{
		SchemaVersion: 1,
		RepoRoot:      filepath.Dir(identity.Target),
		CreatedAt:     "2026-08-07T00:00:00Z",
		Mode:          "attached-adoption",
		AttachedSnapshot: []SnapshotArtifact{
			{Path: ".claude/skills/rekit/SKILL.md", Kind: "case-local-thin-shim", SHA256: strings.Repeat("1", 64), Size: 1},
			{Path: ".re-template.yml", Kind: "legacy-metadata", SHA256: strings.Repeat("2", 64), Size: 1},
			{Path: ".rekit/instance.yml", Kind: "instance-metadata", SHA256: strings.Repeat("3", 64), Size: 1},
			{Path: ".rekit/state.json", Kind: "sync-state", SHA256: strings.Repeat("4", 64), Size: 1},
			{Path: "README.md", Kind: "doctor-validated-artifact", SHA256: strings.Repeat("5", 64), Size: 1},
		},
	}
	if err := ValidateRecoveryEnvelope(identity, valid); err != nil {
		t.Fatalf("valid attached snapshot rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*RecoveryEnvelope)
	}{
		{"writes", func(r *RecoveryEnvelope) { r.Writes = []RecoveryWrite{{Path: "README.md"}} }},
		{"fixed-kind", func(r *RecoveryEnvelope) { r.AttachedSnapshot[0].Kind = "doctor-validated-artifact" }},
		{"runtime-control", func(r *RecoveryEnvelope) { r.AttachedSnapshot[4].Path = ".rekit/runs/one.json" }},
		{"ordinary-kind", func(r *RecoveryEnvelope) { r.AttachedSnapshot[4].Kind = "managed-file" }},
		{"missing-required", func(r *RecoveryEnvelope) { r.AttachedSnapshot = r.AttachedSnapshot[1:] }},
		{"unsorted", func(r *RecoveryEnvelope) {
			r.AttachedSnapshot[3], r.AttachedSnapshot[4] = r.AttachedSnapshot[4], r.AttachedSnapshot[3]
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := valid
			candidate.Writes = append([]RecoveryWrite{}, valid.Writes...)
			candidate.AttachedSnapshot = append([]SnapshotArtifact{}, valid.AttachedSnapshot...)
			tc.mutate(&candidate)
			if err := ValidateRecoveryEnvelope(identity, candidate); err == nil {
				t.Fatal("invalid attached snapshot accepted")
			}
		})
	}
}
