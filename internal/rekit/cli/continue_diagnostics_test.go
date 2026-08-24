package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestBuildContinueDiagnosticsDTOProjectsWithoutMutatingDomainResult(t *testing.T) {
	source := workstream.ContinueResult{
		SchemaVersion:      1,
		Command:            "continue",
		ContinuePlanSHA256: strings.Repeat("a", 64),
		NextSteps: []string{
			"/rekit status -Format json",
			"plain user guidance",
		},
	}
	before := mustJSONBytes(t, source)

	currentRoot := t.TempDir()
	current, err := buildContinueDiagnosticsDTO(source, currentRoot)
	if err != nil {
		t.Fatal(err)
	}
	legacyRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(legacyRoot, ".rekit"), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy, err := buildContinueDiagnosticsDTO(source, legacyRoot)
	if err != nil {
		t.Fatal(err)
	}

	after := mustJSONBytes(t, source)
	if string(after) != string(before) {
		t.Fatalf("continue diagnostics projection mutated domain source:\nbefore=%s\nafter=%s", before, after)
	}
	if current.ContinuePlanSHA256 != source.ContinuePlanSHA256 || legacy.ContinuePlanSHA256 != source.ContinuePlanSHA256 {
		t.Fatalf("continue plan identity drifted: source=%q current=%q legacy=%q", source.ContinuePlanSHA256, current.ContinuePlanSHA256, legacy.ContinuePlanSHA256)
	}
	if strings.Join(current.NextSteps, "|") != "/steamai status -Format json|plain user guidance" {
		t.Fatalf("current diagnostics projection drifted: %+v", current.NextSteps)
	}
	if strings.Join(legacy.NextSteps, "|") != "/rekit status -Format json|plain user guidance" {
		t.Fatalf("legacy diagnostics projection drifted: %+v", legacy.NextSteps)
	}
	if strings.Join(source.NextSteps, "|") != "/rekit status -Format json|plain user guidance" {
		t.Fatalf("independent diagnostics projections aliased source: %+v", source.NextSteps)
	}
	if len(mustJSONBytes(t, current)) == 0 || len(mustJSONBytes(t, legacy)) == 0 {
		t.Fatal("continue diagnostics DTO did not preserve its JSON wire")
	}
}

func TestBuildContinueDiagnosticsDTOFailureDoesNotPartiallyMutateDomainResult(t *testing.T) {
	source := workstream.ContinueResult{
		SchemaVersion:      1,
		Command:            "continue",
		ContinuePlanSHA256: strings.Repeat("b", 64),
		MissionBrief: mission.Brief{NextAgentActions: []string{
			"/rekit status -Format json",
			"/not-a-public-entrypoint status",
		}},
	}
	before := mustJSONBytes(t, source)
	_, err := buildContinueDiagnosticsDTO(source, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "public invocation must begin") {
		t.Fatalf("invalid diagnostics command error=%v", err)
	}
	after := mustJSONBytes(t, source)
	if string(after) != string(before) {
		t.Fatalf("failed diagnostics projection partially mutated domain source:\nbefore=%s\nafter=%s", before, after)
	}
	if source.MissionBrief.NextAgentActions[0] != "/rekit status -Format json" {
		t.Fatalf("failed projection leaked clone mutation into source: %+v", source.MissionBrief.NextAgentActions)
	}
}

func mustJSONBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
