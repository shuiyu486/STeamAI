package sessionhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/cli"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type publicDriverRequest = mission.MissionCommanderDriverRequest

type publicLaneAction struct {
	Lane    string `json:"lane,omitempty"`
	Label   string `json:"label,omitempty"`
	Blocked bool   `json:"blocked,omitempty"`
}

type publicActionQueue struct {
	UnblockedActions []publicLaneAction `json:"unblockedActions,omitempty"`
	BlockedActions   []publicLaneAction `json:"blockedActions,omitempty"`
}

type publicMissionControlRunbook struct {
	Scope                      string                                 `json:"scope,omitempty"`
	CurrentDriverRequest       *mission.MissionCommanderDriverRequest `json:"currentDriverRequest,omitempty"`
	CurrentDriverRequestSHA256 string                                 `json:"currentDriverRequestSha256,omitempty"`
}

type publicCaseMission struct {
	MissionCommanderActionQueue       publicActionQueue `json:"missionCommanderActionQueue"`
	ReviewerDispatchIntakeActionQueue publicActionQueue `json:"reviewerDispatchIntakeActionQueue"`
	MissionCompletion                 *struct {
		Ready                 bool   `json:"ready"`
		State                 string `json:"state"`
		OperationallyComplete bool   `json:"operationallyComplete"`
	} `json:"missionCompletion,omitempty"`
}

type publicStatus struct {
	MissionControlRunbook *publicMissionControlRunbook `json:"missionControlRunbook,omitempty"`
	CaseMission           *publicCaseMission           `json:"caseMission,omitempty"`
}

type publicNoteResult struct {
	IsMutation  bool     `json:"isMutation"`
	Applied     bool     `json:"applied"`
	Reason      string   `json:"reason,omitempty"`
	EventID     string   `json:"eventId"`
	EventSHA256 string   `json:"eventSha256"`
	RecordArgs  []string `json:"recordArgs,omitempty"`
}

type publicDriverStep struct {
	IsMutation                   bool   `json:"isMutation"`
	Applied                      bool   `json:"applied"`
	ExpectedDriverStepPlanSHA256 string `json:"expectedDriverStepPlanSha256"`
	PreviewResult                struct {
		Command string `json:"command"`
	} `json:"previewResult"`
	Receipt *struct {
		CommandResultCommand string `json:"commandResultCommand"`
	} `json:"receipt,omitempty"`
}

type publicDriverResult struct {
	ResultCommand      string
	Actor              string
	Executor           string
	PreviousExecutor   string
	ExecutorGeneration int
	Completion         *workstream.CompleteResult
}

func runPublicCLI(args []string, target any) error {
	var out bytes.Buffer
	if err := cli.Run(args, &out); err != nil {
		return err
	}
	if target == nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode public CLI result: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("public CLI result contains trailing JSON")
	}
	return nil
}

func runPublicCommand(command string) error {
	args, err := cli.SplitPublicCommand(command)
	if err != nil {
		return err
	}
	return runPublicCLI(args, nil)
}

func runPublicNotePreviewApply(
	caseRoot,
	pack string,
	eventArgs []string,
	binding executioncontrol.Binding,
) (publicNoteResult, publicNoteResult, error) {
	if err := executioncontrol.ValidateBinding(binding); err != nil {
		return publicNoteResult{}, publicNoteResult{}, err
	}
	bindingData, err := json.Marshal(binding)
	if err != nil {
		return publicNoteResult{}, publicNoteResult{}, err
	}
	previewArgs := []string{"-Command", "note", "-Target", caseRoot}
	if strings.TrimSpace(pack) != "" {
		previewArgs = append(previewArgs, "-Pack", pack)
	}
	previewArgs = append(previewArgs, eventArgs...)
	previewArgs = append(
		previewArgs,
		"-ExpectedExecutionControlBindingJson", string(bindingData),
		"-WhatIf", "-Format", "json",
	)
	var preview publicNoteResult
	if err := runPublicCLI(previewArgs, &preview); err != nil {
		return publicNoteResult{}, publicNoteResult{}, err
	}
	if preview.IsMutation || preview.Applied || strings.TrimSpace(preview.EventID) == "" || len(strings.TrimSpace(preview.EventSHA256)) != 64 || len(preview.RecordArgs) == 0 {
		return publicNoteResult{}, publicNoteResult{}, fmt.Errorf("public note preview omitted its zero-write hash-bound route")
	}
	bound, err := cli.Parse(preview.RecordArgs)
	if err != nil {
		return publicNoteResult{}, publicNoteResult{}, fmt.Errorf("parse public note record route: %w", err)
	}
	if !strings.EqualFold(bound.Command, "note") || bound.Target != caseRoot || bound.Pack != pack ||
		bound.WhatIf || bound.Apply || bound.List || !strings.EqualFold(bound.Format, "json") ||
		bound.Note.EventID != preview.EventID || !strings.EqualFold(bound.Note.ExpectedEventSHA256, preview.EventSHA256) ||
		!executioncontrol.SameBinding(bound.Note.ExpectedExecutionControlBinding, &binding) {
		return publicNoteResult{}, publicNoteResult{}, fmt.Errorf("public note record route drifted from its exact target, event, or execution control binding")
	}
	var applied publicNoteResult
	if err := runPublicCLI(preview.RecordArgs, &applied); err != nil {
		return publicNoteResult{}, publicNoteResult{}, err
	}
	if applied.IsMutation == false || applied.EventID != preview.EventID || !strings.EqualFold(applied.EventSHA256, preview.EventSHA256) ||
		(!applied.Applied && applied.Reason != "duplicate eventId") {
		return publicNoteResult{}, publicNoteResult{}, fmt.Errorf("public note Apply did not replay or apply the exact preview")
	}
	return preview, applied, nil
}

func runPublicApplyCommand(command, expectedCommand, caseRoot, pack string, target any) error {
	args, err := cli.SplitPublicCommand(command)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "override the command identity") {
			return fmt.Errorf("public Apply command must not override the bounded command: %w", err)
		}
		return err
	}
	if len(args) < 2 || !strings.EqualFold(args[0], "-Command") || !strings.EqualFold(args[1], expectedCommand) || !slices.Contains(args[2:], "-Apply") {
		return fmt.Errorf("public Apply command is not the expected bounded %s route", expectedCommand)
	}
	for _, arg := range args[2:] {
		if strings.EqualFold(arg, "-Command") || strings.EqualFold(arg, "--command") {
			return fmt.Errorf("public Apply command must not override the bounded command")
		}
		if strings.EqualFold(arg, "-Target") || strings.EqualFold(arg, "--target") || strings.EqualFold(arg, "-Pack") || strings.EqualFold(arg, "--pack") {
			return fmt.Errorf("public Apply command must not override the bound target or pack")
		}
	}
	args = append(args, "-Target", caseRoot)
	if strings.TrimSpace(pack) != "" {
		args = append(args, "-Pack", pack)
	}
	return runPublicCLI(args, target)
}

func firstSelectedLane(selected []string) string {
	if len(selected) == 0 {
		return ""
	}
	return selected[0]
}

func publicCaseMissionLaneChoices(caseMission *publicCaseMission) []DailyChoice {
	if caseMission == nil {
		return nil
	}
	return publicActionQueueLaneChoices(
		caseMission.MissionCommanderActionQueue,
		caseMission.ReviewerDispatchIntakeActionQueue,
	)
}

func publicActionQueueLaneChoices(queues ...publicActionQueue) []DailyChoice {
	seen := map[string]bool{}
	choices := []DailyChoice{}
	for _, queue := range queues {
		for _, item := range queue.UnblockedActions {
			lane := strings.TrimSpace(item.Lane)
			if lane == "" || item.Blocked || seen[lane] {
				continue
			}
			seen[lane] = true
			label := strings.TrimSpace(item.Label)
			if label == "" {
				label = lane
			}
			choices = append(choices, DailyChoice{ID: lane, Label: label})
		}
	}
	return choices
}

func bindHostCurrentDriverRequest(opt *Options) (*mission.MissionCommanderDriverRequest, error) {
	if opt == nil {
		return nil, fmt.Errorf("host options are missing")
	}
	status, err := runPublicStatus(opt.Target, opt.Pack, opt.SelectedLane)
	if err != nil {
		return nil, err
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return nil, fmt.Errorf("fresh status omitted the current driver request")
	}
	request := *status.MissionControlRunbook.CurrentDriverRequest
	if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
		return nil, err
	}
	sha256, err := mission.MissionCommanderDriverRequestSHA256(request)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(sha256, strings.TrimSpace(status.MissionControlRunbook.CurrentDriverRequestSHA256)) {
		return nil, fmt.Errorf("fresh status current driver request SHA-256 is inconsistent")
	}
	opt.ExpectedCurrentDriverRequestSHA256 = sha256
	opt.requireCurrentDriverRequest = true
	return &request, nil
}

func runPublicStatus(caseRoot, pack, selected string) (publicStatus, error) {
	args := []string{"-Command", "status", "-Target", caseRoot}
	if strings.TrimSpace(pack) != "" {
		args = append(args, "-Pack", pack)
	}
	args = appendSelectedLaneArg(args, selected)
	args = append(args, "-Format", "json")
	var status publicStatus
	err := runPublicCLI(args, &status)
	return status, err
}

func runPublicDriverStep(caseRoot, pack string, selected ...string) (publicDriverResult, error) {
	lane := firstSelectedLane(selected)
	previewArgs := []string{"-Command", "run-driver-step", "-Target", caseRoot}
	if strings.TrimSpace(pack) != "" {
		previewArgs = append(previewArgs, "-Pack", pack)
	}
	previewArgs = appendSelectedLaneArg(previewArgs, lane)
	previewArgs = append(previewArgs, "-WhatIf", "-Format", "json")
	var preview publicDriverStep
	if err := runPublicCLI(previewArgs, &preview); err != nil {
		return publicDriverResult{}, err
	}
	if preview.IsMutation || preview.Applied || len(strings.TrimSpace(preview.ExpectedDriverStepPlanSHA256)) != 64 || strings.TrimSpace(preview.PreviewResult.Command) == "" {
		return publicDriverResult{}, fmt.Errorf("public driver preview omitted the zero-write hash-bound route")
	}
	applyArgs := []string{"-Command", "run-driver-step", "-Target", caseRoot}
	if strings.TrimSpace(pack) != "" {
		applyArgs = append(applyArgs, "-Pack", pack)
	}
	applyArgs = appendSelectedLaneArg(applyArgs, lane)
	applyArgs = append(applyArgs, "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json")
	var applied struct {
		publicDriverStep
		PreviewResult json.RawMessage `json:"previewResult"`
	}
	if err := runPublicCLI(applyArgs, &applied); err != nil {
		return publicDriverResult{}, err
	}
	command := preview.PreviewResult.Command
	if applied.Receipt == nil || applied.Receipt.CommandResultCommand != command || !applied.IsMutation || !applied.Applied {
		return publicDriverResult{}, fmt.Errorf("public driver Apply did not return a matching refreshed receipt")
	}
	result := publicDriverResult{ResultCommand: command}
	switch command {
	case "reconcile":
		var reconciled struct {
			Actor              string `json:"actor"`
			Executor           string `json:"executor"`
			PreviousExecutor   string `json:"previousExecutor"`
			ExecutorGeneration int    `json:"executorGeneration"`
		}
		if err := json.Unmarshal(applied.PreviewResult, &reconciled); err != nil {
			return publicDriverResult{}, err
		}
		result.Actor = reconciled.Actor
		result.Executor = reconciled.Executor
		result.PreviousExecutor = reconciled.PreviousExecutor
		result.ExecutorGeneration = reconciled.ExecutorGeneration
	case "complete":
		var completed workstream.CompleteResult
		if err := json.Unmarshal(applied.PreviewResult, &completed); err != nil {
			return publicDriverResult{}, err
		}
		result.Completion = &completed
	}
	return result, nil
}
