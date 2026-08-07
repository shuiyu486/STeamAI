package sessionhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/cli"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type publicStatus struct {
	MissionControlRunbook *struct {
		CurrentDriverRequest *struct {
			Kind              string `json:"kind,omitempty"`
			RunLoopStepID     string `json:"runLoopStepId,omitempty"`
			Command           string `json:"command"`
			CommandExecutable bool   `json:"commandExecutable"`
			Blocked           bool   `json:"blocked,omitempty"`
		} `json:"currentDriverRequest,omitempty"`
	} `json:"missionControlRunbook,omitempty"`
	CaseMission *struct {
		MissionCompletion *struct {
			Ready                 bool   `json:"ready"`
			State                 string `json:"state"`
			OperationallyComplete bool   `json:"operationallyComplete"`
		} `json:"missionCompletion,omitempty"`
	} `json:"caseMission,omitempty"`
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

func publicCommandName(command string) (string, error) {
	args, err := cli.SplitPublicCommand(command)
	if err != nil {
		return "", err
	}
	if len(args) < 2 || !strings.EqualFold(args[0], "-Command") {
		return "", fmt.Errorf("public command omitted command identity")
	}
	return strings.ToLower(strings.TrimSpace(args[1])), nil
}

func runPublicStatus(caseRoot, pack string) (publicStatus, error) {
	args := []string{"-Command", "status", "-Target", caseRoot}
	if strings.TrimSpace(pack) != "" {
		args = append(args, "-Pack", pack)
	}
	args = append(args, "-Format", "json")
	var status publicStatus
	err := runPublicCLI(args, &status)
	return status, err
}

func runPublicDriverStep(caseRoot, pack string) (publicDriverResult, error) {
	previewArgs := []string{"-Command", "run-driver-step", "-Target", caseRoot}
	if strings.TrimSpace(pack) != "" {
		previewArgs = append(previewArgs, "-Pack", pack)
	}
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
