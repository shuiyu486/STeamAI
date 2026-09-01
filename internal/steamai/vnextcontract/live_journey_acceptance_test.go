package vnextcontract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const persistentJourneyAcceptanceEnv = "STEAMAI_VNEXT_PERSISTENT_MULTISESSION_ACCEPTANCE"

func TestLivePersistentMemberContextAndCorrection(t *testing.T) {
	if os.Getenv(persistentJourneyAcceptanceEnv) != "1" {
		t.Skip("set " + persistentJourneyAcceptanceEnv + "=1 to run the persistent multi-session journey")
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		t.Fatalf("locate canonical Claude Code CLI: %v", err)
	}

	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	ownerRoot := filepath.Join(stateRoot, "members", "owner")
	reviewerRoot := filepath.Join(stateRoot, "members", "reviewer")
	writeLiveProbeFile(t, filepath.Join(stateRoot, "CLAUDE.md"), "# Synthetic case\n\n- Case marker: persistent-journey\n- Boundary: temporary synthetic files only\n")
	writeLiveProbeFile(t, filepath.Join(ownerRoot, "CLAUDE.md"), "# Owner\n\n- Member: owner\n- Current task: original synthetic task\n- Allowed writes: ../../evidence/E-001.md\n")
	writeLiveProbeFile(t, filepath.Join(reviewerRoot, "CLAUDE.md"), "# Reviewer\n\n- Member: reviewer\n- Current task: review synthetic evidence\n- Allowed writes: ../../reviews/R-001.md only\n")

	ownerSession := "00000000-0000-4000-8000-" + liveJourneySessionSuffix(caseRoot, "owner")
	reviewerSession := "00000000-0000-4000-8000-" + liveJourneySessionSuffix(caseRoot, "reviewer")
	ownerBefore := readLiveProbeFile(t, filepath.Join(ownerRoot, "CLAUDE.md"))
	owner := runPersistentJourneyTurn(t, claude, ownerRoot, caseRoot, ownerSession, false, "", "",
		`Return JSON with member and currentTask from automatically loaded instructions.`)
	if owner.Member != "owner" || owner.CurrentTask != "original synthetic task" {
		t.Fatalf("owner context mismatch: %+v", owner)
	}
	reviewer := runPersistentJourneyTurn(t, claude, reviewerRoot, caseRoot, reviewerSession, false, "", "",
		`Return JSON with member and currentTask from automatically loaded instructions.`)
	if reviewer.Member != "reviewer" || reviewer.CurrentTask != "review synthetic evidence" {
		t.Fatalf("Reviewer context mismatch: %+v", reviewer)
	}
	reviewerResumed := runPersistentJourneyTurn(t, claude, reviewerRoot, caseRoot, reviewerSession, true, "", "",
		`Return JSON with member and currentTask from the same persistent session.`)
	if reviewerResumed.Member != "reviewer" || reviewerResumed.CurrentTask != "review synthetic evidence" {
		t.Fatalf("Reviewer resume mismatch: %+v", reviewerResumed)
	}

	corrected := runPersistentJourneyTurn(t, claude, ownerRoot, caseRoot, ownerSession, true, "Read,Edit", "acceptEdits",
		`This is a synthetic direct-session correction. Change only Current task in CLAUDE.md to corrected synthetic task, then return JSON from the resulting file.`)
	if corrected.Member != "owner" || corrected.CurrentTask != "corrected synthetic task" {
		t.Fatalf("direct correction mismatch: %+v", corrected)
	}
	ownerPath := filepath.Join(ownerRoot, "CLAUDE.md")
	ownerCorrected := strings.Replace(ownerBefore, "Current task: original synthetic task", "Current task: corrected synthetic task", 1)
	if got := readLiveProbeFile(t, ownerPath); got != ownerCorrected {
		t.Fatal("direct correction changed fields outside Current task")
	}
	staleBefore := ownerCorrected
	stale := runPersistentJourneyTurn(t, claude, ownerRoot, caseRoot, ownerSession, true, "Read,Edit", "acceptEdits",
		`Synthetic delayed Commander change: expected current task is original synthetic task; new task is stale overwrite. Compare before editing. If expected mismatches, do not edit and return hold=HOLD_STALE_TASK plus the current member/task.`)
	if stale.Member != "owner" || stale.Hold != "HOLD_STALE_TASK" || stale.CurrentTask != "corrected synthetic task" {
		t.Fatalf("stale task was not held: %+v", stale)
	}
	if got := readLiveProbeFile(t, ownerPath); got != staleBefore {
		t.Fatal("stale delayed change modified the corrected member file")
	}
}

func liveJourneySessionSuffix(caseRoot, member string) string {
	sum := sha256.Sum256([]byte(caseRoot + "\x00" + member))
	return hex.EncodeToString(sum[:6])
}

func readLiveProbeFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type persistentJourneyResult struct {
	Member      string `json:"member"`
	CurrentTask string `json:"currentTask"`
	Hold        string `json:"hold"`
}

func runPersistentJourneyTurn(t *testing.T, claude, cwd, caseRoot, sessionID string, resume bool, tools, permissionMode, prompt string) persistentJourneyResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	schema := `{"type":"object","properties":{"member":{"type":"string"},"currentTask":{"type":"string"},"hold":{"type":"string"}},"required":["member","currentTask","hold"],"additionalProperties":false}`
	args := []string{"-p", prompt, "--add-dir", caseRoot, "--tools", tools, "--output-format", "json", "--json-schema", schema}
	if permissionMode != "" {
		args = append(args, "--permission-mode", permissionMode)
	} else {
		args = append(args, "--permission-mode", "dontAsk")
	}
	if resume {
		args = append(args, "--resume", sessionID)
	} else {
		args = append(args, "--session-id", sessionID)
	}
	cmd := exec.CommandContext(ctx, claude, args...)
	cmd.Dir = cwd
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	stdout, err := cmd.Output()
	if ctx.Err() != nil {
		t.Fatalf("persistent journey timed out: %v", ctx.Err())
	}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("persistent journey failed: %v: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		t.Fatal(err)
	}
	var envelope struct {
		IsError          bool            `json:"is_error"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		t.Fatalf("decode persistent journey envelope: %v: %s", err, strings.TrimSpace(string(stdout)))
	}
	if envelope.IsError {
		t.Fatalf("persistent journey returned error: %s", strings.TrimSpace(string(stdout)))
	}
	payload := envelope.StructuredOutput
	if len(payload) == 0 || string(payload) == "null" {
		payload = json.RawMessage(envelope.Result)
	}
	var result persistentJourneyResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode persistent journey result: %v: %s", err, strings.TrimSpace(string(payload)))
	}
	return result
}
