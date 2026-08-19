package externalsession

import (
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
)

const (
	SchemaVersion  = 1
	KindJob        = "current-loop-external-session-job"
	KindSubmission = "current-loop-external-session-submission"
	KindReceipt    = "current-loop-external-session-publication"
)

type ReviewerIdentity struct {
	AttemptSHA256  string              `json:"attemptSha256"`
	PacketID       string              `json:"packetId"`
	RouteID        string              `json:"routeId"`
	ShardID        string              `json:"shardId"`
	Items          []string            `json:"items"`
	OutputFields   []string            `json:"outputFields"`
	DispatchPath   string              `json:"dispatchPath"`
	DispatchSHA256 string              `json:"dispatchSha256"`
	DispatchID     string              `json:"dispatchId,omitempty"`
	Harness        string              `json:"harness,omitempty"`
	Session        string              `json:"session,omitempty"`
	TargetLane     string              `json:"targetLane,omitempty"`
	EffectiveOwner *laneowner.Snapshot `json:"effectiveOwner,omitempty"`
}

type Job struct {
	SchemaVersion       int                    `json:"schemaVersion"`
	Kind                string                 `json:"kind"`
	JobID               string                 `json:"jobId"`
	CaseRoot            string                 `json:"caseRoot"`
	Pack                string                 `json:"pack"`
	CheckpointSHA256    string                 `json:"checkpointSha256"`
	SessionKind         string                 `json:"sessionKind"`
	AllowedOutcomes     []string               `json:"allowedOutcomes"`
	SubmissionPath      string                 `json:"submissionPath"`
	SubmissionOutputs   string                 `json:"submissionOutputs,omitempty"`
	SubmissionResult    string                 `json:"submissionResult,omitempty"`
	MemberAttemptID     string                 `json:"memberAttemptId,omitempty"`
	MemberOwner         *memberexecution.Owner `json:"memberOwner,omitempty"`
	MemberManifestPath  string                 `json:"memberManifestPath,omitempty"`
	MemberOutputsRoot   string                 `json:"memberOutputsRoot,omitempty"`
	Reviewer            *ReviewerIdentity      `json:"reviewer,omitempty"`
	RelayResultPath     string                 `json:"relayResultPath,omitempty"`
	PublicationPath     string                 `json:"publicationPath"`
	ObservationPath     string                 `json:"observationPath"`
	SubmissionLast      bool                   `json:"submissionLast"`
	DispatchRequired    bool                   `json:"dispatchRequired,omitempty"`
	NoSessionManagement bool                   `json:"noSessionManagement"`
	NoHeavyTool         bool                   `json:"noHeavyTool"`
	NoAuthority         bool                   `json:"noAuthority"`
	NoConfirmed         bool                   `json:"noConfirmed"`
}

type Submission struct {
	SchemaVersion                int                       `json:"schemaVersion"`
	Kind                         string                    `json:"kind"`
	JobID                        string                    `json:"jobId"`
	JobSHA256                    string                    `json:"jobSha256"`
	Outcome                      string                    `json:"outcome"`
	Actor                        string                    `json:"actor"`
	ObservedAt                   string                    `json:"observedAt,omitempty"`
	Reason                       string                    `json:"reason,omitempty"`
	Summary                      string                    `json:"summary,omitempty"`
	ReviewerItemsPath            string                    `json:"reviewerItemsPath,omitempty"`
	ReviewerHarness              string                    `json:"reviewerHarness,omitempty"`
	ReviewerSession              string                    `json:"reviewerSession,omitempty"`
	ReviewerExitStatus           string                    `json:"reviewerExitStatus,omitempty"`
	AttemptID                    string                    `json:"attemptId"`
	AttemptSHA256                string                    `json:"attemptSha256"`
	LaunchControl                *executioncontrol.Binding `json:"launchControl,omitempty"`
	DispatchClaimSHA256          string                    `json:"dispatchClaimSha256,omitempty"`
	LaunchReceiptSHA256          string                    `json:"launchReceiptSha256,omitempty"`
	TransportReturnReceiptPath   string                    `json:"transportReturnReceiptPath,omitempty"`
	TransportReturnReceiptSHA256 string                    `json:"transportReturnReceiptSha256,omitempty"`
	Harness                      string                    `json:"harness"`
	Session                      string                    `json:"session"`
	NoAuthorityOrConfirmed       bool                      `json:"noAuthorityOrConfirmed"`
	NoHeavyTool                  bool                      `json:"noHeavyTool"`
}

type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Inspection struct {
	Job              Job         `json:"job"`
	JobSHA256        string      `json:"jobSha256"`
	State            string      `json:"state"`
	SubmissionSHA256 string      `json:"submissionSha256,omitempty"`
	Submission       *Submission `json:"submission,omitempty"`
	Warnings         []string    `json:"warnings,omitempty"`
}

type ObservationBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	data   []byte
}

type ReviewerResultBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	data   []byte
}

type Plan struct {
	SchemaVersion        int                                 `json:"schemaVersion"`
	Mode                 string                              `json:"mode"`
	Job                  Job                                 `json:"job"`
	JobSHA256            string                              `json:"jobSha256"`
	Submission           Submission                          `json:"submission"`
	SubmissionSHA256     string                              `json:"submissionSha256"`
	ExpectedPlanSHA256   string                              `json:"expectedPlanSha256"`
	ApplyCommand         string                              `json:"applyCommand,omitempty"`
	Artifacts            []Artifact                          `json:"artifacts"`
	Observation          ObservationBinding                  `json:"observation"`
	ReviewerResult       *ReviewerResultBinding              `json:"reviewerResult,omitempty"`
	ReviewRequired       bool                                `json:"reviewRequired"`
	RequiresConfirmation bool                                `json:"requiresConfirmation"`
	Applied              bool                                `json:"applied"`
	AlreadyApplied       bool                                `json:"alreadyApplied"`
	ResultPublication    *executioncontrol.ResultPublication `json:"resultPublication,omitempty"`
	Boundary             []string                            `json:"boundary"`
	writes               []plannedWrite
	submissionPath       string
	submissionData       []byte
	memberResult         *memberexecution.ResultSnapshot
}

type plannedWrite struct {
	rel  string
	data []byte
}

func (binding ObservationBinding) Data() []byte {
	return append([]byte{}, binding.data...)
}

func (binding ReviewerResultBinding) Data() []byte {
	return append([]byte{}, binding.data...)
}

func (plan Plan) MemberResultSnapshot() *memberexecution.ResultSnapshot {
	if plan.memberResult == nil {
		return nil
	}
	result := &memberexecution.ResultSnapshot{
		ManifestPath: plan.memberResult.ManifestPath,
		ManifestData: append([]byte{}, plan.memberResult.ManifestData...),
		OutputsRoot:  plan.memberResult.OutputsRoot,
		Outputs:      map[string][]byte{},
	}
	for path, data := range plan.memberResult.Outputs {
		result.Outputs[path] = append([]byte{}, data...)
	}
	return result
}
