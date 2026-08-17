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
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
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

func TestStatusHandoffPublicationGenerationSupportsCurrentAndLegacyRoots(t *testing.T) {
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot, request, takeoverRel, generation := writeStatusHandoffGenerationFixtureForRoot(t, stateDir)
			writeStatusHandoffGeneration(t, caseRoot, generation)
			state, warnings, verified := statusHandoffPublicationGenerationState(caseRoot, "case", request, takeoverRel)
			if state != "fresh" || len(warnings) != 0 || string(verified) != "fixture takeover\n" {
				t.Fatalf("state root %s generation state=%q warnings=%+v bytes=%q", stateDir, state, warnings, verified)
			}
			generationRel, err := statusHandoffPublicationGenerationRel(caseRoot, "case", request)
			if err != nil || !strings.HasPrefix(generationRel, stateDir+"/") {
				t.Fatalf("state root %s generation rel=%q err=%v", stateDir, generationRel, err)
			}
			artifactRel, err := statusReplacementExecutorTakeoverArtifactRel(caseRoot, "case", request)
			if err != nil || artifactRel != takeoverRel {
				t.Fatalf("state root %s artifact rel=%q want=%q err=%v", stateDir, artifactRel, takeoverRel, err)
			}
		})
	}
}

func TestStatusReplacementExecutorTakeoverTargetDocumentsUsesResolvedFactsRoot(t *testing.T) {
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			docs, err := statusReplacementExecutorTakeoverTargetDocuments(
				caseRoot,
				"case",
				mission.MissionCommanderDriverRequest{},
				nil,
				"",
			)
			if err != nil {
				t.Fatal(err)
			}
			want := stateDir + "/facts/*.jsonl"
			found := false
			for _, doc := range docs {
				if doc == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("target documents = %+v, want %q", docs, want)
			}
		})
	}
}

func TestStatusHandoffPublicationGenerationRejectsMixedRoots(t *testing.T) {
	caseRoot, request, takeoverRel, generation := writeStatusHandoffGenerationFixtureForRoot(t, projectstate.CurrentDir)
	if err := os.MkdirAll(filepath.Join(caseRoot, projectstate.LegacyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	state, warnings, _ := statusHandoffPublicationGenerationState(caseRoot, "case", request, takeoverRel)
	if state != "invalid-generation" || len(warnings) == 0 {
		t.Fatalf("mixed roots state=%q warnings=%+v generation=%+v", state, warnings, generation)
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
	return writeStatusHandoffGenerationFixtureForRoot(t, projectstate.CurrentDir)
}

func writeStatusHandoffGenerationFixtureForRoot(t *testing.T, stateDir string) (string, mission.MissionCommanderDriverRequest, string, workstream.HandoffPublicationGeneration) {
	t.Helper()
	caseRoot := t.TempDir()
	stamp := "20260802-010203000"
	planSHA256 := strings.Repeat("a", sha256.Size*2)
	boardPath := filepath.Join(caseRoot, stateDir, "board.json")
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
		{rel: filepath.ToSlash(filepath.Join(stateDir, "lanes", "main", "prompts", "RESUME.md")), role: workstream.HandoffPublicationRoleResume, published: []byte("resume\n"), canonical: []byte("resume\n")},
		{rel: filepath.ToSlash(filepath.Join(stateDir, "lanes", "main", "checkpoints", "latest.json")), role: workstream.HandoffPublicationRoleCheckpoint, published: []byte("{}\n"), canonical: []byte("{}\n")},
		{rel: filepath.ToSlash(filepath.Join(stateDir, "handovers", "main-"+stamp+".md")), role: workstream.HandoffPublicationRoleHandoffStamped, published: []byte("plan " + planSHA256 + "\n"), canonical: []byte("plan <handoff-publication-plan-sha256>\n"), occurrences: 1},
		{rel: filepath.ToSlash(filepath.Join(stateDir, "handovers", "main-latest.md")), role: workstream.HandoffPublicationRoleHandoffLatest, published: []byte("plan " + planSHA256 + "\n"), canonical: []byte("plan <handoff-publication-plan-sha256>\n"), occurrences: 1},
		{rel: filepath.ToSlash(filepath.Join(stateDir, "handovers", "main-"+stamp+"-replacement-executor-takeover.json")), role: workstream.HandoffPublicationRoleTakeoverStamped, published: []byte("fixture takeover\n"), canonical: []byte("fixture takeover\n")},
		{rel: filepath.ToSlash(filepath.Join(stateDir, "handovers", "main-latest-replacement-executor-takeover.json")), role: workstream.HandoffPublicationRoleTakeoverLatest, published: []byte("fixture takeover\n"), canonical: []byte("fixture takeover\n")},
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
	takeoverRel := filepath.ToSlash(filepath.Join(stateDir, "handovers", "main-latest-replacement-executor-takeover.json"))
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
	path, err := projectstate.Join(caseRoot, "handovers", "main-latest-generation.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(generation, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
