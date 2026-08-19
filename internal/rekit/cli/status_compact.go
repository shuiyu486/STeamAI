package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	statusCompactJSONMaxBytes          = 4096
	statusCompactStateDetailsRequired  = "details-required"
	statusCompactReasonIdentityInvalid = "current-driver-request-identity-invalid"
	statusCompactReasonBudgetExceeded  = "compact-output-budget-exceeded"
	statusCompactFullDiagnosticsFormat = "json"
)

type statusCompactInventory struct {
	Command               string                                   `json:"command"`
	SchemaVersion         int                                      `json:"schemaVersion"`
	IsMutation            bool                                     `json:"isMutation"`
	Pack                  string                                   `json:"pack"`
	PackSource            string                                   `json:"packSource"`
	Target                string                                   `json:"target"`
	TargetProvided        bool                                     `json:"targetProvided"`
	Mode                  string                                   `json:"mode"`
	Case                  *statusCompactCase                       `json:"case,omitempty"`
	CaseMission           *statusCompactMission                    `json:"caseMission,omitempty"`
	Onboarding            *statusCompactOnboarding                 `json:"onboarding,omitempty"`
	MissionControlRunbook *statusCompactMissionControlRunbook      `json:"missionControlRunbook,omitempty"`
	ExecutionControls     []statusCompactExecutionControl          `json:"executionControls,omitempty"`
	CurrentSyncRecovery   *statusCompactCurrentSyncRecovery        `json:"currentSyncRecovery,omitempty"`
	Choices               []mission.MissionCommanderNextActionItem `json:"choices,omitempty"`
}

type statusCompactMissionControlRunbook struct {
	Ready                      bool                                   `json:"ready"`
	Focus                      string                                 `json:"focus,omitempty"`
	Scope                      string                                 `json:"scope,omitempty"`
	CurrentRunLoopStepID       string                                 `json:"currentRunLoopStepId,omitempty"`
	CurrentDriverRequest       *mission.MissionCommanderDriverRequest `json:"currentDriverRequest,omitempty"`
	CurrentDriverRequestSHA256 string                                 `json:"currentDriverRequestSha256,omitempty"`
	RefreshStatusCommand       string                                 `json:"refreshStatusCommand,omitempty"`
}

type statusCompactCase struct {
	CaseRoot            string `json:"caseRoot"`
	ProjectName         string `json:"projectName,omitempty"`
	TemplatePack        string `json:"templatePack,omitempty"`
	PackMatchesMetadata bool   `json:"packMatchesMetadata"`
	Moved               bool   `json:"moved"`
}

type statusCompactMission struct {
	Ready            bool   `json:"ready"`
	Summary          string `json:"summary,omitempty"`
	LaneCount        int    `json:"laneCount"`
	ReadyLaneCount   int    `json:"readyLaneCount"`
	BlockedLaneCount int    `json:"blockedLaneCount"`
}

type statusCompactOnboarding struct {
	State     string `json:"state,omitempty"`
	Committed bool   `json:"committed,omitempty"`
}

type statusCompactExecutionControl struct {
	Lane                 string `json:"lane"`
	State                string `json:"state"`
	CurrentGeneration    int    `json:"currentGeneration"`
	CurrentReceiptSHA256 string `json:"currentReceiptSha256,omitempty"`
	Pending              bool   `json:"pending"`
	PendingGeneration    int    `json:"pendingGeneration,omitempty"`
	PendingAction        string `json:"pendingAction,omitempty"`
	RecoveryCommand      string `json:"recoveryCommand,omitempty"`
	Blocked              bool   `json:"blocked"`
	Reason               string `json:"reason,omitempty"`
}

type statusCompactCurrentSyncRecovery struct {
	State       string `json:"state"`
	Pending     bool   `json:"pending"`
	Blocked     bool   `json:"blocked"`
	Recoverable bool   `json:"recoverable"`
	Now         string `json:"now"`
	Reason      string `json:"reason"`
	Next        string `json:"next"`
}

type statusCompactBlockedEnvelope struct {
	Command           string                       `json:"command"`
	SchemaVersion     int                          `json:"schemaVersion"`
	IsMutation        bool                         `json:"isMutation"`
	State             string                       `json:"state"`
	Blocked           bool                         `json:"blocked"`
	DetailsRequired   bool                         `json:"detailsRequired"`
	CommandExecutable bool                         `json:"commandExecutable"`
	Reason            string                       `json:"reason"`
	FullDiagnostics   statusCompactFullDiagnostics `json:"fullDiagnostics"`
	Boundary          []string                     `json:"boundary"`
}

type statusCompactFullDiagnostics struct {
	Command                string `json:"command"`
	Format                 string `json:"format"`
	OnDemand               bool   `json:"onDemand"`
	ReuseOriginalSelectors bool   `json:"reuseOriginalSelectors"`
}

func buildStatusCompactInventory(status statusInventory) (statusCompactInventory, error) {
	compact := statusCompactInventory{
		Command:        status.Command,
		SchemaVersion:  status.SchemaVersion,
		IsMutation:     status.IsMutation,
		Pack:           status.Pack,
		PackSource:     status.PackSource,
		Target:         status.Target,
		TargetProvided: status.TargetProvided,
		Mode:           status.Mode,
	}
	if runbook := status.MissionControlRunbook; runbook != nil {
		compact.MissionControlRunbook = &statusCompactMissionControlRunbook{
			Ready:                      runbook.Ready,
			Focus:                      runbook.Focus,
			Scope:                      runbook.Scope,
			CurrentRunLoopStepID:       runbook.CurrentRunLoopStepID,
			CurrentDriverRequest:       runbook.CurrentDriverRequest,
			CurrentDriverRequestSHA256: runbook.CurrentDriverRequestSHA256,
			RefreshStatusCommand:       runbook.RefreshStatusCommand,
		}
		if err := validateStatusCompactCurrent(*compact.MissionControlRunbook); err != nil {
			return statusCompactInventory{}, err
		}
	}
	if status.CaseMission != nil {
		compact.Choices = statusCompactChoices(status.CaseMission)
		compact.CaseMission = &statusCompactMission{
			Ready:            status.CaseMission.Ready,
			Summary:          status.CaseMission.Summary,
			LaneCount:        status.CaseMission.LaneCount,
			ReadyLaneCount:   status.CaseMission.ReadyLaneCount,
			BlockedLaneCount: status.CaseMission.BlockedLaneCount,
		}
	}
	if status.Case != nil {
		compact.Case = &statusCompactCase{
			CaseRoot:            status.Case.CaseRoot,
			ProjectName:         status.Case.ProjectName,
			TemplatePack:        status.Case.TemplatePack,
			PackMatchesMetadata: status.Case.PackMatchesMetadata,
			Moved:               status.Case.Moved,
		}
	}
	for _, control := range status.ExecutionControls {
		compact.ExecutionControls = append(compact.ExecutionControls, statusCompactExecutionControl{
			Lane:                 control.Lane,
			State:                control.State,
			CurrentGeneration:    control.CurrentGeneration,
			CurrentReceiptSHA256: control.CurrentReceiptSHA256,
			Pending:              control.Pending,
			PendingGeneration:    control.PendingGeneration,
			PendingAction:        control.PendingAction,
			RecoveryCommand:      control.RecoveryCommand,
			Blocked:              control.Blocked,
			Reason:               control.Reason,
		})
	}
	if status.Onboarding != nil {
		compact.Onboarding = &statusCompactOnboarding{
			State:     status.Onboarding.State,
			Committed: status.Onboarding.Committed,
		}
	}
	if recovery := status.CurrentSyncRecovery; recovery != nil {
		compact.CurrentSyncRecovery = &statusCompactCurrentSyncRecovery{
			State:       recovery.State,
			Pending:     recovery.Pending,
			Blocked:     recovery.Blocked,
			Recoverable: recovery.Recoverable,
			Now:         recovery.Now,
			Reason:      recovery.Reason,
			Next:        recovery.Next,
		}
	}
	return compact, nil
}

func validateStatusCompactCurrent(current statusCompactMissionControlRunbook) error {
	request := current.CurrentDriverRequest
	hash := strings.TrimSpace(current.CurrentDriverRequestSHA256)
	if request == nil {
		if hash != "" {
			return fmt.Errorf("status compact-json current driver request identity is incomplete")
		}
		return nil
	}
	if hash == "" {
		return fmt.Errorf("status compact-json current driver request identity is incomplete")
	}
	actual, err := mission.MissionCommanderDriverRequestSHA256(*request)
	if err != nil {
		return fmt.Errorf("status compact-json current driver request is invalid: %w", err)
	}
	if !strings.EqualFold(actual, hash) {
		return fmt.Errorf("status compact-json current driver request SHA-256 is inconsistent")
	}
	return nil
}

func statusCompactChoices(caseMission *statusCaseMission) []mission.MissionCommanderNextActionItem {
	if caseMission == nil {
		return nil
	}
	return statusMissionControlLaneChoices(
		caseMission.MissionCommanderActionQueue,
		caseMission.ReviewerDispatchIntakeActionQueue,
	)
}

func marshalStatusCompactJSON(status statusInventory) ([]byte, error) {
	compact, err := buildStatusCompactInventory(status)
	if err != nil {
		return marshalStatusCompactBlockedJSON(status, statusCompactReasonIdentityInvalid)
	}
	data, err := marshalStatusCompactValue(compact)
	if err != nil {
		return nil, err
	}
	if len(data) <= statusCompactJSONMaxBytes {
		return data, nil
	}
	return marshalStatusCompactBlockedJSON(status, statusCompactReasonBudgetExceeded)
}

func marshalStatusCompactBlockedJSON(status statusInventory, reason string) ([]byte, error) {
	envelope := statusCompactBlockedEnvelope{
		Command:           status.Command,
		SchemaVersion:     status.SchemaVersion,
		IsMutation:        false,
		State:             statusCompactStateDetailsRequired,
		Blocked:           true,
		DetailsRequired:   true,
		CommandExecutable: false,
		Reason:            reason,
		FullDiagnostics: statusCompactFullDiagnostics{
			Command:                statusCompactFullDiagnosticsCommand(status),
			Format:                 statusCompactFullDiagnosticsFormat,
			OnDemand:               true,
			ReuseOriginalSelectors: true,
		},
		Boundary: []string{
			"compact status omitted currentDriverRequest and choices because they cannot be emitted completely and safely",
			"do not reconstruct, truncate, or execute a request or choice from this envelope",
			"rerun the same status invocation with -Format json to inspect full typed diagnostics on demand",
		},
	}
	data, err := marshalStatusCompactValue(envelope)
	if err != nil {
		return nil, err
	}
	if len(data) > statusCompactJSONMaxBytes {
		return nil, fmt.Errorf("status compact-json blocked envelope exceeds %d-byte limit: %d bytes", statusCompactJSONMaxBytes, len(data))
	}
	return data, nil
}

func statusCompactFullDiagnosticsCommand(status statusInventory) string {
	entrypoint := "/steamai"
	if root, err := projectstate.Resolve(status.Target); err == nil && root.Legacy {
		entrypoint = "/rekit"
	}
	return entrypoint + " status -Format " + statusCompactFullDiagnosticsFormat
}

func marshalStatusCompactValue(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeStatusCompactJSON(out io.Writer, status statusInventory) error {
	data, err := marshalStatusCompactJSON(status)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
