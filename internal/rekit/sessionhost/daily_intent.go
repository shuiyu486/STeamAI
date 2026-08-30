package sessionhost

import (
	"fmt"
	"strings"
)

type DailyOperation string

const (
	DailyOperationResume     DailyOperation = "resume"
	DailyOperationGoal       DailyOperation = "goal"
	DailyOperationCorrection DailyOperation = "correction"
	DailyOperationControl    DailyOperation = "control"
	DailyOperationAdoption   DailyOperation = "adoption"
)

// DailyRequest is the single classification boundary for the public daily
// front door. Selectors such as SelectedLane refine an operation; they never
// choose a different operation by themselves.
type DailyRequest struct {
	Operation         DailyOperation
	Goal              string
	Correction        string
	ControlRequested  bool
	AdoptionRequested bool
}

func ClassifyDailyRequest(opt DailyOptions) DailyRequest {
	goal := strings.TrimSpace(opt.Goal)
	correction := strings.TrimSpace(opt.Correction)
	controlRequested := opt.ControlWhatIf || opt.ControlApply ||
		strings.TrimSpace(opt.Control.Action) != "" ||
		strings.TrimSpace(opt.Control.Reason) != "" ||
		strings.TrimSpace(opt.Control.PublicationStamp) != "" ||
		strings.TrimSpace(opt.Control.ExpectedPlanSHA256) != ""
	adoptionRequested := strings.TrimSpace(opt.DirectoryAdoptionAction) != "" ||
		strings.TrimSpace(opt.DirectoryAdoptionPack) != "" ||
		strings.TrimSpace(opt.ExpectedInitPlanSHA256) != "" ||
		strings.TrimSpace(opt.InitializationRepoRoot) != ""

	request := DailyRequest{
		Operation:         DailyOperationResume,
		Goal:              goal,
		Correction:        correction,
		ControlRequested:  controlRequested,
		AdoptionRequested: adoptionRequested,
	}
	switch {
	case controlRequested:
		request.Operation = DailyOperationControl
	case adoptionRequested:
		request.Operation = DailyOperationAdoption
	case correction != "":
		request.Operation = DailyOperationCorrection
	case goal != "":
		request.Operation = DailyOperationGoal
	}
	return request
}

func ResolveDailyRequest(opt DailyOptions) (DailyRequest, error) {
	request := ClassifyDailyRequest(opt)
	goal := request.Goal
	correction := request.Correction
	controlRequested := request.ControlRequested
	adoptionRequested := request.AdoptionRequested

	if strings.TrimSpace(opt.Control.Lane) != "" {
		return request, fmt.Errorf("daily control lane must use the daily selected lane")
	}
	if strings.TrimSpace(opt.Control.Actor) != "" {
		return request, fmt.Errorf("daily control actor must use the daily actor")
	}
	if goal != "" && correction != "" {
		return request, fmt.Errorf("daily front door accepts either -goal or -correction, not both")
	}
	if controlRequested && (goal != "" || correction != "") {
		return request, fmt.Errorf("daily front door control cannot be combined with -goal or -correction")
	}
	if controlRequested && adoptionRequested {
		return request, fmt.Errorf("daily control cannot be combined with directory adoption controls")
	}
	if adoptionRequested && correction != "" {
		return request, fmt.Errorf("daily directory adoption cannot be combined with -correction")
	}
	return request, nil
}
