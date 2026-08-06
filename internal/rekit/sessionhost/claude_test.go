package sessionhost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
)

func TestPublishMemberOutputsCopiesValidatedFixtureBytes(t *testing.T) {
	caseRoot := t.TempDir()
	response := memberResponse{
		Outcome: "returned",
		Summary: "fixture result",
		Outputs: []memberOutput{
			{Path: "analysis/result.txt", Content: "fixture bytes\n"},
			{Path: "review-items.txt", Content: "item-a\n"},
		},
		ReviewerItemsPath: "review-items.txt",
	}
	if err := publishMemberOutputs(caseRoot, ".rekit/host/outputs", response); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"analysis/result.txt": "fixture bytes\n",
		"review-items.txt":    "item-a\n",
	} {
		got, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "host", "outputs", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("output %s = %q, want exact returned bytes %q", path, string(got), want)
		}
	}
}

func TestPublishMemberOutputsRejectsUnsafeAndPartialResults(t *testing.T) {
	tests := []memberResponse{
		{Outcome: "returned"},
		{Outcome: "returned", Outputs: []memberOutput{{Path: "../escape.txt", Content: "x"}}},
		{Outcome: "returned", Outputs: []memberOutput{{Path: "C:/escape.txt", Content: "x"}}},
		{Outcome: "returned", Outputs: []memberOutput{{Path: "result.txt", Content: ""}}},
		{Outcome: "returned", Outputs: []memberOutput{{Path: "result.txt", Content: "x"}}, ReviewerItemsPath: "missing.txt"},
	}
	for index, response := range tests {
		if err := publishMemberOutputs(t.TempDir(), ".rekit/host/outputs", response); err == nil {
			t.Fatalf("unsafe response %d was accepted: %+v", index, response)
		}
	}
}

func TestReviewerResultFixturePreservesSessionField(t *testing.T) {
	result := reviewerresult.Result{
		PacketID: "packet", RouteID: "route", ShardID: "shard", Items: []string{"item"},
		ReviewerSession: "fixture-session", Decision: "needs-more-evidence", Confidence: "low",
		Summary: "fixture review", EvidenceRefs: []string{}, Risks: []string{}, Conflicts: []string{},
		RecommendedVerdict: "defer", RouteOutput: map[string]any{"status": "bounded"},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := reviewerresult.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ReviewerSession != "fixture-session" {
		t.Fatalf("reviewer session = %q", decoded.ReviewerSession)
	}
}

func TestReviewerDispatchReadyRequiresExactBootstrapState(t *testing.T) {
	plan := currentStepPlan{ReviewerStep: &reviewerStep{ExternalHandoff: &reviewerExternalHandoff{
		State: "ready-for-reviewer-dispatch", RunLoopStepID: "spawn-reviewer",
	}}}
	if !reviewerDispatchReady(plan) {
		t.Fatal("exact reviewer bootstrap state was not recognized")
	}
	plan.ReviewerStep.ExternalHandoff.RunLoopStepID = "save-result-input"
	if reviewerDispatchReady(plan) {
		t.Fatal("non-bootstrap reviewer handoff was accepted")
	}
}

func TestClaudeSuccessRequiresReturnedSessionIdentity(t *testing.T) {
	run := claudeRun{
		envelope: claudeEnvelope{Type: "result", Subtype: "success"},
		exitCode: 0, structuredOutput: json.RawMessage(`{"outcome":"returned"}`),
	}
	if run.success() {
		t.Fatal("Claude success accepted a missing durable session identity")
	}
	run.sessionID = "session-id"
	if !run.success() {
		t.Fatal("Claude success rejected a bound durable session identity")
	}
}

func TestLimitedBufferFailsClosedWithoutTruncationSuccess(t *testing.T) {
	buffer := limitedBuffer{limit: 8}
	if _, err := buffer.Write([]byte("1234567890")); err != nil {
		t.Fatal(err)
	}
	if !buffer.exceeded || string(buffer.Bytes()) != "12345678" {
		t.Fatalf("limited buffer = exceeded:%t bytes:%q", buffer.exceeded, string(buffer.Bytes()))
	}
	if _, err := buffer.Write([]byte("ignored")); err != nil {
		t.Fatal(err)
	}
	if string(buffer.Bytes()) != "12345678" {
		t.Fatalf("limited buffer changed after overflow: %q", string(buffer.Bytes()))
	}
}

func TestClaudeSchemasBindImmutableInputAndDurableSession(t *testing.T) {
	caseRoot := t.TempDir()
	inputRel := ".rekit/handoff.md"
	input := []byte("bounded task\n")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(caseRoot, filepath.FromSlash(inputRel))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(inputRel)), input, 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := hostPackageForTest(caseRoot, inputRel, input)
	prompt, schema, err := claudeRequest(caseRoot, pkg, "session-id")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, strconv.Quote(filepath.Join(caseRoot, filepath.FromSlash(inputRel)))) || !strings.Contains(prompt, strconv.Quote(caseRoot)) || !strings.Contains(prompt, "using the Read tool") || !strings.Contains(schema, `"outputs"`) {
		t.Fatalf("member Claude request omitted input or output contract: prompt=%q schema=%s", prompt, schema)
	}
	pkg.Launch.Attempt.Session = "different"
	if _, _, err := claudeRequest(caseRoot, pkg, "session-id"); err == nil {
		t.Fatal("Claude request accepted a session different from the durable attempt")
	}
}

func hostPackageForTest(caseRoot, inputRel string, input []byte) mission.CurrentLoopExternalSessionHarnessPackage {
	return mission.CurrentLoopExternalSessionHarnessPackage{
		CaseRoot:    caseRoot,
		SessionKind: "member",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready:   true,
			Input:   mission.CurrentLoopExternalSessionHarnessInput{Path: inputRel, SHA256: bytesSHA256(input)},
			Attempt: mission.CurrentLoopExternalSessionAttempt{Session: "session-id"},
		},
	}
}
