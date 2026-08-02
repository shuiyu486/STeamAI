package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestStatusHandoffPublicationGenerationRejectsInvalidIdentityAndTargets(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*workstream.HandoffPublicationGeneration)
	}{
		{name: "wrong scope", mutate: func(generation *workstream.HandoffPublicationGeneration) {
			generation.Scope = "project"
		}},
		{name: "non-hex plan sha", mutate: func(generation *workstream.HandoffPublicationGeneration) {
			generation.PublicationPlanSHA256 = strings.Repeat("z", sha256.Size*2)
		}},
		{name: "invalid stamp", mutate: func(generation *workstream.HandoffPublicationGeneration) {
			generation.PublicationStamp = "20260231-250000000"
		}},
		{name: "duplicate target", mutate: func(generation *workstream.HandoffPublicationGeneration) {
			generation.Entries = append(generation.Entries, generation.Entries[0])
		}},
		{name: "missing target", mutate: func(generation *workstream.HandoffPublicationGeneration) {
			generation.Entries = generation.Entries[1:]
		}},
		{name: "wrong role", mutate: func(generation *workstream.HandoffPublicationGeneration) {
			generation.Entries[0].Role = workstream.HandoffPublicationRoleCheckpoint
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot, request, takeoverRel, generation := writeStatusHandoffGenerationFixture(t)
			tc.mutate(&generation)
			writeStatusHandoffGeneration(t, caseRoot, generation)
			state, warnings, _ := statusHandoffPublicationGenerationState(caseRoot, "case", request, takeoverRel)
			if state != "invalid-generation" || len(warnings) == 0 {
				t.Fatalf("invalid generation state = %q warnings=%+v", state, warnings)
			}
		})
	}
}

func TestStatusHandoffPublicationGenerationRejectsLiteralPlanMarker(t *testing.T) {
	caseRoot, request, takeoverRel, generation := writeStatusHandoffGenerationFixture(t)
	entry := generationEntryForRole(t, generation, workstream.HandoffPublicationRoleHandoffLatest)
	path := filepath.Join(caseRoot, filepath.FromSlash(entry.Path))
	if err := os.WriteFile(path, []byte("plan <handoff-publication-plan-sha256>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStatusHandoffGeneration(t, caseRoot, generation)
	state, warnings, _ := statusHandoffPublicationGenerationState(caseRoot, "case", request, takeoverRel)
	if state != "mixed-generation" || len(warnings) == 0 {
		t.Fatalf("literal marker state = %q warnings=%+v", state, warnings)
	}
}

func TestStatusHandoffPublicationGenerationReturnsVerifiedTakeoverSnapshot(t *testing.T) {
	caseRoot, request, takeoverRel, generation := writeStatusHandoffGenerationFixture(t)
	writeStatusHandoffGeneration(t, caseRoot, generation)
	takeoverPath := filepath.Join(caseRoot, filepath.FromSlash(takeoverRel))
	original, err := os.ReadFile(takeoverPath)
	if err != nil {
		t.Fatal(err)
	}
	statusHandoffGenerationAfterTargetsReadHook = func() {
		statusHandoffGenerationAfterTargetsReadHook = nil
		if err := os.WriteFile(takeoverPath, []byte("replacement takeover\n"), 0o644); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { statusHandoffGenerationAfterTargetsReadHook = nil })
	state, warnings, verified := statusHandoffPublicationGenerationState(caseRoot, "case", request, takeoverRel)
	if state != "fresh" || len(warnings) != 0 || string(verified) != string(original) {
		t.Fatalf("verified generation snapshot state=%q warnings=%+v bytes=%q want=%q", state, warnings, verified, original)
	}
}

func TestStatusHandoffPublicationGenerationRejectsCommitReplacementAfterTargetVerification(t *testing.T) {
	caseRoot, request, takeoverRel, generation := writeStatusHandoffGenerationFixture(t)
	generationPath := writeStatusHandoffGeneration(t, caseRoot, generation)
	statusHandoffGenerationAfterTargetsReadHook = func() {
		statusHandoffGenerationAfterTargetsReadHook = nil
		generation.PublicationStamp = "20260802-010204000"
		data, err := json.MarshalIndent(generation, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		if err := os.WriteFile(generationPath, append(data, '\n'), 0o644); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { statusHandoffGenerationAfterTargetsReadHook = nil })
	state, warnings, _ := statusHandoffPublicationGenerationState(caseRoot, "case", request, takeoverRel)
	if state != "mixed-generation" || len(warnings) == 0 {
		t.Fatalf("replaced generation state = %q warnings=%+v", state, warnings)
	}
}

func writeStatusHandoffGenerationFixture(t *testing.T) (string, mission.MissionCommanderDriverRequest, string, workstream.HandoffPublicationGeneration) {
	t.Helper()
	caseRoot := t.TempDir()
	stamp := "20260802-010203000"
	planSHA256 := strings.Repeat("a", sha256.Size*2)
	boardPath := filepath.Join(caseRoot, ".rekit", "board.json")
	if err := os.MkdirAll(filepath.Dir(boardPath), 0o755); err != nil {
		t.Fatal(err)
	}
	board := mission.Board{SchemaVersion: 1, Lanes: []mission.BoardLane{{ID: "main", Status: "active"}}}
	boardBytes, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(boardPath, boardBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	targets := []struct {
		rel         string
		role        string
		published   []byte
		canonical   []byte
		occurrences int
	}{
		{rel: ".rekit/lanes/main/prompts/RESUME.md", role: workstream.HandoffPublicationRoleResume, published: []byte("resume\n"), canonical: []byte("resume\n")},
		{rel: ".rekit/lanes/main/checkpoints/latest.json", role: workstream.HandoffPublicationRoleCheckpoint, published: []byte("{}\n"), canonical: []byte("{}\n")},
		{rel: ".rekit/handovers/main-" + stamp + ".md", role: workstream.HandoffPublicationRoleHandoffStamped, published: []byte("plan " + planSHA256 + "\n"), canonical: []byte("plan <handoff-publication-plan-sha256>\n"), occurrences: 1},
		{rel: ".rekit/handovers/main-latest.md", role: workstream.HandoffPublicationRoleHandoffLatest, published: []byte("plan " + planSHA256 + "\n"), canonical: []byte("plan <handoff-publication-plan-sha256>\n"), occurrences: 1},
		{rel: ".rekit/handovers/main-" + stamp + "-replacement-executor-takeover.json", role: workstream.HandoffPublicationRoleTakeoverStamped, published: []byte("fixture takeover\n"), canonical: []byte("fixture takeover\n")},
		{rel: ".rekit/handovers/main-latest-replacement-executor-takeover.json", role: workstream.HandoffPublicationRoleTakeoverLatest, published: []byte("fixture takeover\n"), canonical: []byte("fixture takeover\n")},
	}
	entries := make([]workstream.HandoffPublicationGenerationEntry, 0, len(targets))
	for _, target := range targets {
		path := filepath.Join(caseRoot, filepath.FromSlash(target.rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, target.published, 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(target.canonical)
		entries = append(entries, workstream.HandoffPublicationGenerationEntry{
			Path:                  target.rel,
			Role:                  target.role,
			Bytes:                 len(target.canonical),
			CanonicalSHA256:       hex.EncodeToString(sum[:]),
			PlanSHA256Occurrences: target.occurrences,
		})
	}
	generation := workstream.HandoffPublicationGeneration{
		SchemaVersion:         1,
		Scope:                 "lane:main",
		PublicationPlanSHA256: planSHA256,
		PublicationStamp:      stamp,
		Entries:               entries,
	}
	takeoverRel := ".rekit/handovers/main-latest-replacement-executor-takeover.json"
	return caseRoot, mission.MissionCommanderDriverRequest{Lane: "main"}, takeoverRel, generation
}

func generationEntryForRole(t *testing.T, generation workstream.HandoffPublicationGeneration, role string) workstream.HandoffPublicationGenerationEntry {
	t.Helper()
	for _, entry := range generation.Entries {
		if entry.Role == role {
			return entry
		}
	}
	t.Fatalf("generation role %q not found", role)
	return workstream.HandoffPublicationGenerationEntry{}
}

func writeStatusHandoffGeneration(t *testing.T, caseRoot string, generation workstream.HandoffPublicationGeneration) string {
	t.Helper()
	path := filepath.Join(caseRoot, ".rekit", "handovers", "main-latest-generation.json")
	data, err := json.MarshalIndent(generation, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
