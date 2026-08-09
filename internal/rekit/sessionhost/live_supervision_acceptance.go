package sessionhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
)

type LiveSupervisionAcceptanceOptions struct {
	CaseRoot    string
	Goal        string
	Model       string
	Actor       string
	Timeout     time.Duration
	MaxAttempts int
	KeepCase    bool
	ReceiptPath string
}

type LiveSupervisionAcceptanceReceipt struct {
	SchemaVersion        int                  `json:"schemaVersion"`
	Kind                 string               `json:"kind"`
	Passed               bool                 `json:"passed"`
	ReceiptPublication   string               `json:"receiptPublication,omitempty"`
	ReceiptError         string               `json:"receiptError,omitempty"`
	CaseRoot             string               `json:"caseRoot"`
	CaseCreated          bool                 `json:"caseCreated"`
	Goal                 string               `json:"goal"`
	CutPoint             string               `json:"cutPoint"`
	Claude               LiveAcceptanceClaude `json:"claude"`
	RunID                string               `json:"runId"`
	SessionID            string               `json:"sessionId"`
	AttemptSHA256        string               `json:"attemptSha256"`
	FirstHostInterrupted bool                 `json:"firstHostInterrupted"`
	InterruptedCutPoints []string             `json:"interruptedCutPoints"`
	FreshHostLaunches    int                  `json:"freshHostLaunches"`
	FreshCompletions     int                  `json:"freshCompletions"`
	TotalStartedReceipts int                  `json:"totalStartedReceipts"`
	OutputPublications   int                  `json:"outputPublications"`
	ManualPlaceholders   int                  `json:"manualPlaceholders"`
	ManualResultWrites   int                  `json:"manualResultWrites"`
	Cleanup              string               `json:"cleanup"`
	Boundary             []string             `json:"boundary"`
}

func RunLiveSupervisionAcceptance(parent context.Context, opt LiveSupervisionAcceptanceOptions) (receipt LiveSupervisionAcceptanceReceipt, retErr error) {
	goal := strings.TrimSpace(opt.Goal)
	if goal == "" {
		return receipt, fmt.Errorf("live supervision acceptance requires a non-empty natural-language goal")
	}
	caseRoot, err := liveAcceptanceCaseRoot(opt.CaseRoot, liveAcceptancePack)
	if err != nil {
		return receipt, err
	}
	claude, err := resolveLiveAcceptanceClaude("")
	if err != nil {
		return receipt, err
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		actor = "rekit-live-supervision-acceptance"
	}
	receipt = LiveSupervisionAcceptanceReceipt{
		SchemaVersion: 1, Kind: "rekit-live-supervision-acceptance-receipt", CaseRoot: caseRoot, Goal: goal,
		CutPoint: "process-start", Claude: claude, ManualPlaceholders: 0, ManualResultWrites: 0, Cleanup: "pending",
		Boundary: []string{
			"the first public daily host stops only after the exact supervisor started receipt; the supervisor and Claude process continue independently",
			"the fresh public daily host must collect the same attempt and session with zero new Claude launches",
			"all member output bytes come from the real signed Claude process and production publication path",
			"no authority/confirmed state or heavy-tool execution is permitted",
		},
	}
	var identity liveAcceptanceCaseIdentity
	var restoreObserver func()
	defer func() {
		if restoreObserver != nil {
			restoreObserver()
		}
		defer identity.Close()
		receipt.CaseCreated = identity.parent != nil
		if opt.KeepCase {
			if receipt.CaseCreated {
				receipt.Cleanup = "retained-by-request"
			} else {
				receipt.Cleanup = "not-created"
			}
			return
		}
		if !receipt.CaseCreated {
			receipt.Cleanup = "not-created"
			return
		}
		if err := removeLiveAcceptanceCase(caseRoot, &identity); err != nil {
			receipt.Cleanup = "failed"
			retErr = errors.Join(retErr, err)
			return
		}
		receipt.Cleanup = "removed"
	}()

	if opt.Timeout <= 0 {
		opt.Timeout = defaultTimeout
	}
	if opt.MaxAttempts <= 0 {
		opt.MaxAttempts = defaultMaxAttempts
	}
	dailyOpt := DailyOptions{
		Target: caseRoot, Goal: goal, Actor: actor, ClaudePath: claude.Path,
		ExpectedClaudeExecutableSHA256: claude.SHA256, ExpectedClaudeExecutablePublisher: claude.Publisher,
		Model: opt.Model, Timeout: opt.Timeout, MaxAttempts: opt.MaxAttempts,
		onCaseReady: func(root string) error {
			if err := captureLiveAcceptanceCaseRoot(root, &identity); err != nil {
				return err
			}
			receipt.CaseCreated = true
			return nil
		},
	}
	cutPoints := []string{"process-start", "output-returned", "result-first", "submission", "intake"}
	startedReceipts := 0
	startedRuns := map[string]struct{}{}
	for _, cutPoint := range cutPoints {
		cutContext, cancelCut := context.WithCancel(parent)
		observed := false
		restoreObserver = setSupervisionAcceptanceObservers(func(started supervisionStarted) error {
			if _, seen := startedRuns[started.RunID]; !seen {
				startedRuns[started.RunID] = struct{}{}
				startedReceipts++
			}
			receipt.RunID = started.RunID
			receipt.SessionID = started.SessionID
			if cutPoint == "process-start" {
				observed = true
				cancelCut()
			}
			return nil
		}, func(stage string) error {
			if stage == cutPoint {
				observed = true
				cancelCut()
				return context.Canceled
			}
			return nil
		})
		interrupted, interruptErr := RunDaily(cutContext, dailyOpt)
		cancelCut()
		restoreObserver()
		restoreObserver = nil
		if interruptErr == nil || !errors.Is(interruptErr, context.Canceled) || !observed {
			return receipt, fmt.Errorf("public daily host did not stop at the %s cut point: result=%+v err=%v", cutPoint, interrupted, interruptErr)
		}
		if cutPoint == "process-start" && (startedReceipts != 1 || receipt.RunID == "" || receipt.SessionID == "" || interrupted.SessionLaunches != 0) {
			return receipt, fmt.Errorf("process-start cut point evidence drifted: started=%d result=%+v", startedReceipts, interrupted)
		}
		receipt.InterruptedCutPoints = append(receipt.InterruptedCutPoints, cutPoint)
	}
	receipt.FirstHostInterrupted = true

	fresh, err := RunDaily(parent, dailyOpt)
	if err != nil {
		return receipt, fmt.Errorf("fresh daily host collection after cut-point interruptions: %w", err)
	}
	if fresh.SessionLaunches != 0 || fresh.SessionCompletions != 0 || fresh.FinalState != "member-intake-ready" || !fresh.Replay {
		return receipt, fmt.Errorf("final daily host was not a zero-launch replay after exact cut-point recovery: %+v", fresh)
	}
	receipt.FreshHostLaunches = fresh.SessionLaunches
	receipt.FreshCompletions = 1
	receipt.TotalStartedReceipts = startedReceipts
	latest, ok, err := memberexecution.Latest(caseRoot, fresh.Lane)
	if err != nil || !ok || latest.State != "intake-ready" || latest.Manifest == nil || latest.AttemptID == "" {
		return receipt, fmt.Errorf("fresh supervised member was not durably intake-ready: found=%t latest=%+v err=%v", ok, latest, err)
	}
	receipt.AttemptSHA256 = latest.ManifestSHA256
	if strings.TrimSpace(latest.ManifestPath) == "" {
		return receipt, fmt.Errorf("fresh supervised member omitted the real manifest publication")
	}
	if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)))); err != nil {
		return receipt, fmt.Errorf("inspect real supervised member manifest: %w", err)
	}
	receipt.OutputPublications = 1

	replay, err := RunDaily(parent, dailyOpt)
	if err != nil || !replay.Replay || replay.SessionLaunches != 0 || replay.SessionCompletions != 0 {
		return receipt, fmt.Errorf("supervised member terminal replay was not zero-launch: %+v err=%v", replay, err)
	}
	receipt.Passed = true
	return receipt, nil
}

func WriteLiveSupervisionAcceptanceReceipt(path string, receipt LiveSupervisionAcceptanceReceipt) error {
	path, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	anchorPath := filepath.VolumeName(path) + string(filepath.Separator)
	if anchorPath == "" {
		anchorPath = string(filepath.Separator)
	}
	rel, err := filepath.Rel(anchorPath, path)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("live supervision acceptance receipt path escapes its volume root: %s", path)
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := rekitfs.WriteNewExclusiveRegularFileAnchored(anchorPath, filepath.ToSlash(rel), "live supervision acceptance receipt", data); err != nil {
		return fmt.Errorf("publish live supervision acceptance receipt %s: %w", path, err)
	}
	return nil
}
