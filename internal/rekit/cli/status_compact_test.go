package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestRunStatusCompactJSONCurrentRequestIdentityParity(t *testing.T) {
	caseRoot := attachedCase(t)
	seedEmptyLaneCaseBoard(t, caseRoot)
	args := []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template"}

	full := runStatusIdentityForFormat(t, args, "json")
	compact := runStatusIdentityForFormat(t, args, "compact-json")

	if full.MissionControlRunbook == nil || full.MissionControlRunbook.CurrentDriverRequest == nil {
		t.Fatal("full status fixture omitted current driver request")
	}
	if compact.MissionControlRunbook == nil {
		t.Fatal("compact status omitted missionControlRunbook")
	}
	if !reflect.DeepEqual(compact.MissionControlRunbook.CurrentDriverRequest, full.MissionControlRunbook.CurrentDriverRequest) {
		t.Fatalf("compact current driver request differs from full status:\ncompact=%+v\nfull=%+v", compact.MissionControlRunbook.CurrentDriverRequest, full.MissionControlRunbook.CurrentDriverRequest)
	}
	if compact.MissionControlRunbook.CurrentDriverRequestSHA256 != full.MissionControlRunbook.CurrentDriverRequestSHA256 {
		t.Fatalf("compact current driver request SHA-256 = %q, want %q", compact.MissionControlRunbook.CurrentDriverRequestSHA256, full.MissionControlRunbook.CurrentDriverRequestSHA256)
	}
	actual, err := mission.MissionCommanderDriverRequestSHA256(*compact.MissionControlRunbook.CurrentDriverRequest)
	if err != nil {
		t.Fatal(err)
	}
	if actual != compact.MissionControlRunbook.CurrentDriverRequestSHA256 {
		t.Fatalf("compact request SHA-256 = %q, recomputed %q", compact.MissionControlRunbook.CurrentDriverRequestSHA256, actual)
	}
}

func TestRunStatusCompactJSONPreservesAllTypedChoices(t *testing.T) {
	first := compactStatusChoiceFixture(t, "feature-login", "login", "ready-to-continue")
	second := compactStatusChoiceFixture(t, "main", "main", "ready-to-continue")
	caseMission := &statusCaseMission{
		MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{first, second}),
	}
	status := statusInventory{
		Command:       commands.Status,
		SchemaVersion: 1,
		Mode:          "case",
		CaseMission:   caseMission,
	}
	status.MissionControlRunbook = buildStatusMissionControlRunbook("", caseMission, nil)

	data, err := marshalStatusCompactJSON(status)
	if err != nil {
		t.Fatal(err)
	}
	var compact statusCompactInventory
	if err := json.Unmarshal(data, &compact); err != nil {
		t.Fatal(err)
	}
	want := statusCompactChoices(caseMission)
	if !reflect.DeepEqual(compact.Choices, want) || len(compact.Choices) != 2 {
		t.Fatalf("compact choices = %+v, want all typed choices %+v", compact.Choices, want)
	}
	for _, choice := range compact.Choices {
		if choice.Invocation == nil {
			t.Fatalf("compact choice lost typed invocation: %+v", choice)
		}
		projected, err := choice.Invocation.Render()
		if err != nil || projected != choice.Command {
			t.Fatalf("compact choice invocation parity failed: choice=%+v projected=%q err=%v", choice, projected, err)
		}
	}
	if compact.MissionControlRunbook == nil {
		t.Fatal("ambiguous compact status omitted missionControlRunbook")
	}
	if compact.MissionControlRunbook.CurrentDriverRequest != nil || compact.MissionControlRunbook.CurrentDriverRequestSHA256 != "" {
		t.Fatalf("ambiguous compact status published current request identity: %+v", compact.MissionControlRunbook)
	}
}

func TestStatusCompactJSONBudgetIncludesNewline(t *testing.T) {
	status := statusInventory{
		Command:       commands.Status,
		SchemaVersion: 1,
		Mode:          "case",
		Target:        strings.Repeat("界", 1200),
	}
	data, err := marshalStatusCompactJSON(status)
	if err != nil {
		t.Fatalf("compact UTF-8 status should fit budget: %v", err)
	}
	if len(data) > statusCompactJSONMaxBytes {
		t.Fatalf("compact output = %d bytes, limit %d", len(data), statusCompactJSONMaxBytes)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("compact output omitted final newline: %q", data)
	}

	status.Target = strings.Repeat("界", 1400)
	data, err = marshalStatusCompactJSON(status)
	if err != nil {
		t.Fatalf("oversized compact status should return blocked envelope: %v", err)
	}
	assertStatusCompactBlockedEnvelope(t, data, statusCompactReasonBudgetExceeded, "/steamai")
	var out bytes.Buffer
	if err := writeStatusCompactJSON(&out, status); err != nil {
		t.Fatalf("oversized compact writer should return blocked envelope: %v", err)
	}
	assertStatusCompactBlockedEnvelope(t, out.Bytes(), statusCompactReasonBudgetExceeded, "/steamai")
}

func TestStatusCompactJSONFailsClosedOnRequestIdentityAndChoiceOverflow(t *testing.T) {
	request := compactStatusRequestFixture(t, "main", "ok")
	for _, fixture := range []struct {
		name    string
		request *mission.MissionCommanderDriverRequest
		hash    string
	}{
		{name: "request without hash", request: &request},
		{name: "hash without request", hash: strings.Repeat("0", 64)},
		{name: "inconsistent hash", request: &request, hash: strings.Repeat("0", 64)},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			status := statusInventory{
				Command:       commands.Status,
				SchemaVersion: 1,
				Target:        strings.Repeat("界", 2000),
				MissionControlRunbook: &statusMissionControlRunbook{
					CurrentDriverRequest:       fixture.request,
					CurrentDriverRequestSHA256: fixture.hash,
				},
			}
			data, err := marshalStatusCompactJSON(status)
			if err != nil {
				t.Fatalf("invalid request identity should return blocked envelope: %v", err)
			}
			assertStatusCompactBlockedEnvelope(t, data, statusCompactReasonIdentityInvalid, "/steamai")
			var out bytes.Buffer
			if err := writeStatusCompactJSON(&out, status); err != nil {
				t.Fatalf("invalid request identity writer should return blocked envelope: %v", err)
			}
			if !bytes.Equal(out.Bytes(), data) {
				t.Fatalf("invalid identity writer emitted non-atomic envelope:\nmarshal=%s\nwriter=%s", data, out.Bytes())
			}
		})
	}

	choices := make([]mission.MissionCommanderNextActionItem, 0, 40)
	for i := 0; i < 40; i++ {
		choices = append(choices, compactStatusChoiceFixture(t, "lane-"+strings.Repeat("x", 80)+string(rune('A'+i)), strings.Repeat("label", 20), "ready-to-continue"))
	}
	caseMission := &statusCaseMission{MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor(choices)}
	status := statusInventory{
		Command:       commands.Status,
		SchemaVersion: 1,
		CaseMission:   caseMission,
	}
	status.MissionControlRunbook = buildStatusMissionControlRunbook("", caseMission, nil)
	if got := len(statusCompactChoices(caseMission)); got != len(choices) {
		t.Fatalf("choice fixture collapsed to %d choices, want %d", got, len(choices))
	}
	data, err := marshalStatusCompactJSON(status)
	if err != nil {
		t.Fatalf("oversized typed choices should return blocked envelope: %v", err)
	}
	assertStatusCompactBlockedEnvelope(t, data, statusCompactReasonBudgetExceeded, "/steamai")
	var out bytes.Buffer
	if err := writeStatusCompactJSON(&out, status); err != nil {
		t.Fatalf("oversized typed choices writer should return blocked envelope: %v", err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("oversized choices writer emitted non-atomic envelope:\nmarshal=%s\nwriter=%s", data, out.Bytes())
	}
}

func TestStatusCompactJSONBlockedEnvelopeSelectsCurrentAndLegacyEntrypoints(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		stateDir   string
		entrypoint string
	}{
		{name: "current project", stateDir: ".steamai", entrypoint: "/steamai"},
		{name: "legacy project", stateDir: ".rekit", entrypoint: "/rekit"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			target := t.TempDir()
			if err := os.Mkdir(filepath.Join(target, fixture.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			status := statusInventory{
				Command:       commands.Status,
				SchemaVersion: 1,
				Target:        target,
				CaseMission: &statusCaseMission{
					Summary: strings.Repeat("details", 700),
				},
			}
			data, err := marshalStatusCompactJSON(status)
			if err != nil {
				t.Fatal(err)
			}
			assertStatusCompactBlockedEnvelope(t, data, statusCompactReasonBudgetExceeded, fixture.entrypoint)
		})
	}
}

func TestRunStatusCompactJSONRejectsDualStateRoots(t *testing.T) {
	target := t.TempDir()
	for _, stateDir := range []string{".steamai", ".rekit"} {
		if err := os.Mkdir(filepath.Join(target, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	err := Run([]string{"-Command", "status", "-Target", target, "-Pack", "_template", "-Format", "compact-json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "both .steamai and .rekit") {
		t.Fatalf("dual-root compact status error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("dual-root compact status emitted partial output: %s", out.String())
	}
}

func TestRunStatusCompactJSONIsAdditiveToLegacyFormats(t *testing.T) {
	var compactOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "compact-json"}, &compactOut); err != nil {
		t.Fatal(err)
	}
	if compactOut.Len() > statusCompactJSONMaxBytes || compactOut.Len() == 0 || compactOut.Bytes()[compactOut.Len()-1] != '\n' {
		t.Fatalf("compact output length/newline = %d/%q", compactOut.Len(), compactOut.Bytes())
	}
	var compact statusCompactInventory
	if err := json.Unmarshal(compactOut.Bytes(), &compact); err != nil {
		t.Fatalf("compact status did not decode: %v\n%s", err, compactOut.String())
	}
	if compact.Command != commands.Status || compact.SchemaVersion != 1 || compact.IsMutation || compact.Mode != "kit" {
		t.Fatalf("unexpected compact status envelope: %+v", compact)
	}
	for _, forbidden := range []string{"projectHandoff", "replacementExecutorTakeoverPackage", "queues", "missionCommanderActionQueue"} {
		if bytes.Contains(compactOut.Bytes(), []byte(`"`+forbidden+`"`)) {
			t.Fatalf("compact status leaked %s: %s", forbidden, compactOut.String())
		}
	}

	var fullOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "json"}, &fullOut); err != nil {
		t.Fatal(err)
	}
	var full map[string]json.RawMessage
	if err := json.Unmarshal(fullOut.Bytes(), &full); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"projectHandoff", "missionControlRunbook", "caseShim"} {
		if _, ok := full[required]; !ok {
			t.Fatalf("full status JSON lost %s", required)
		}
	}

	for _, format := range []string{"", "table", "tsv", "text"} {
		var out bytes.Buffer
		args := []string{"-Command", "status", "-Pack", "_template"}
		if format != "" {
			args = append(args, "-Format", format)
		}
		if err := Run(args, &out); err != nil {
			t.Fatalf("legacy format %q failed: %v", format, err)
		}
		if out.Len() == 0 {
			t.Fatalf("legacy format %q returned empty output", format)
		}
	}
}

func assertStatusCompactBlockedEnvelope(t *testing.T, data []byte, reason, entrypoint string) {
	t.Helper()
	if len(data) == 0 || len(data) > statusCompactJSONMaxBytes || data[len(data)-1] != '\n' {
		t.Fatalf("blocked compact envelope length/newline = %d/%q", len(data), data)
	}
	var envelope statusCompactBlockedEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("blocked compact envelope did not decode: %v\n%s", err, data)
	}
	if envelope.Command != commands.Status || envelope.SchemaVersion != 1 || envelope.IsMutation || envelope.State != statusCompactStateDetailsRequired || !envelope.Blocked || !envelope.DetailsRequired || envelope.CommandExecutable || envelope.Reason != reason {
		t.Fatalf("unexpected blocked compact envelope: %+v", envelope)
	}
	if envelope.FullDiagnostics.Command != entrypoint+" status -Format json" || envelope.FullDiagnostics.Format != statusCompactFullDiagnosticsFormat || !envelope.FullDiagnostics.OnDemand || !envelope.FullDiagnostics.ReuseOriginalSelectors {
		t.Fatalf("blocked compact envelope omitted full diagnostics route: %+v", envelope.FullDiagnostics)
	}
	if len(envelope.Boundary) == 0 || !strings.Contains(strings.Join(envelope.Boundary, "\n"), "same status invocation") || bytes.Contains(data, []byte(`"currentDriverRequest"`)) || bytes.Contains(data, []byte(`"choices"`)) {
		t.Fatalf("blocked compact envelope leaked partial request/choices or omitted selector-preserving boundary: %s", data)
	}
}

type statusIdentitySnapshot struct {
	MissionControlRunbook *statusCompactMissionControlRunbook `json:"missionControlRunbook"`
}

func runStatusIdentityForFormat(t *testing.T, baseArgs []string, format string) statusIdentitySnapshot {
	t.Helper()
	args := append(append([]string{}, baseArgs...), "-Format", format)
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatalf("status %s failed: %v", format, err)
	}
	if format == "compact-json" && len(out.Bytes()) > statusCompactJSONMaxBytes {
		t.Fatalf("compact status = %d bytes", len(out.Bytes()))
	}
	var snapshot statusIdentitySnapshot
	if err := json.Unmarshal(out.Bytes(), &snapshot); err != nil {
		t.Fatalf("status %s did not decode: %v\n%s", format, err, out.String())
	}
	return snapshot
}

func compactStatusChoiceFixture(t *testing.T, lane, label, state string) mission.MissionCommanderNextActionItem {
	t.Helper()
	invocation, err := commands.NewPublicInvocation(commands.Continue, "-Lane", lane, "-WhatIf", "-Format", "json")
	if err != nil {
		t.Fatal(err)
	}
	command, err := invocation.Render()
	if err != nil {
		t.Fatal(err)
	}
	return mission.MissionCommanderNextActionItem{
		Lane:       lane,
		Label:      label,
		ActionID:   "continue-" + lane,
		State:      state,
		Invocation: &invocation,
		Command:    command,
		Source:     "missionCommanderActions",
	}
}

func compactStatusRequestFixture(t *testing.T, lane, marker string) mission.MissionCommanderDriverRequest {
	t.Helper()
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{compactStatusChoiceFixture(t, lane, marker, "ready-to-continue")})
	if queue.CurrentDriverRequest == nil {
		t.Fatal("request fixture omitted current driver request")
	}
	return *queue.CurrentDriverRequest
}
