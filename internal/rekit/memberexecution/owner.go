package memberexecution

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

const (
	maxJSONBytes   = 256 * 1024
	maxOutputBytes = 4 * 1024 * 1024
	maxOutputs     = 64
)

var segment = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
var errPendingDispatch = errors.New("member execution dispatch publication is pending")
var applyLeaseHook func(Plan) error

func PreviewDispatch(opt DispatchOptions) (Plan, error) {
	caseRoot, owner, err := currentOwner(opt.CaseRoot, opt.Pack, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	launchControl, err := captureMemberHandoffLaunchControl(caseRoot, owner, opt.LaunchControl)
	if err != nil {
		return Plan{}, err
	}
	if !validSHA(opt.RequestSHA256) {
		return Plan{}, fmt.Errorf("member execution dispatch requires request sha256")
	}
	createdAt := strings.TrimSpace(opt.CreatedAt)
	if createdAt == "" {
		createdAt, err = currentOwnerUpdatedAt(caseRoot, owner.Lane)
		if err != nil {
			return Plan{}, err
		}
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return Plan{}, err
	}
	attemptSequence, err := nextAttemptSequence(caseRoot, owner, opt.RequestSHA256)
	if err != nil {
		return Plan{}, err
	}
	attemptID := fmt.Sprintf("g%06d-a%06d-%s", owner.ExecutorGeneration, attemptSequence, strings.ToLower(opt.RequestSHA256[:16]))
	if !segment.MatchString(attemptID) {
		return Plan{}, fmt.Errorf("invalid member execution attempt id")
	}
	root, err := attemptRoot(caseRoot, owner.Lane, attemptID)
	if err != nil {
		return Plan{}, err
	}
	intent := Intent{SchemaVersion: 1, Kind: KindIntent, AttemptID: attemptID, CaseRoot: caseRoot, Pack: opt.Pack, Owner: owner, RequestSHA256: strings.ToLower(opt.RequestSHA256), CreatedAt: created.Format(time.RFC3339Nano), NoSpawn: true, NoPoll: true, NoStop: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true}
	intentBytes, err := canonical(intent)
	if err != nil {
		return Plan{}, err
	}
	taskContext, err := currentTaskContext(caseRoot, opt.Pack, owner, attemptID)
	if err != nil {
		return Plan{}, err
	}
	binding, err := CurrentTaskBinding(caseRoot, owner.Lane)
	if err != nil {
		return Plan{}, err
	}
	taskContext.Binding = binding
	taskContextBytes, err := canonical(taskContext)
	if err != nil {
		return Plan{}, err
	}
	taskContextPath := rel(caseRoot, filepath.Join(root, "task-context.json"))
	handoff := Handoff{SchemaVersion: 1, Kind: KindHandoff, AttemptID: attemptID, Owner: owner, IntentSHA256: hash(intentBytes), TaskContextPath: taskContextPath, TaskContextSHA256: hash(taskContextBytes), ManifestPath: rel(caseRoot, filepath.Join(root, "result", "manifest.json")), OutputsRoot: rel(caseRoot, filepath.Join(root, "result", "outputs")), LaunchControl: executioncontrol.CloneBinding(launchControl), NextSteps: []string{"external harness accepts this handoff", "external member reads the exact immutable task context and writes bounded outputs", "record accepted then returned or failed observation through run-current-step"}, Boundary: boundaries()}
	handoffBytes, err := canonical(handoff)
	if err != nil {
		return Plan{}, err
	}
	commit := Commit{SchemaVersion: 1, Kind: KindCommit, AttemptID: attemptID, IntentSHA256: hash(intentBytes), TaskContextSHA256: hash(taskContextBytes), HandoffSHA256: hash(handoffBytes)}
	commitBytes, err := canonical(commit)
	if err != nil {
		return Plan{}, err
	}
	writes := []plannedWrite{{filepath.Join(root, "intent.json"), intentBytes}, {filepath.Join(root, "task-context.json"), taskContextBytes}, {filepath.Join(root, "handoff.json"), handoffBytes}, {filepath.Join(root, "commit.json"), commitBytes}}
	inspection := Inspection{State: "handoff-ready", AttemptID: attemptID, Owner: owner, Intent: &intent, TaskContext: &taskContext, TaskContextPath: filepath.Join(root, "task-context.json"), TaskContextSHA256: hash(taskContextBytes), Handoff: &handoff, HandoffSHA256: hash(handoffBytes), AttemptRoot: root, ManifestPath: filepath.Join(root, "result", "manifest.json"), OutputsRoot: filepath.Join(root, "result", "outputs")}
	return finishPlan(Plan{SchemaVersion: 1, Mode: "dispatch", CaseRoot: caseRoot, Pack: opt.Pack, AttemptID: attemptID, Owner: owner, ExternalHandoff: &handoff, Inspection: inspection, ReviewRequired: true, RequiresConfirmation: true, Boundary: boundaries(), writes: writes})
}

func captureMemberHandoffLaunchControl(caseRoot string, owner Owner, provided *executioncontrol.Binding) (*executioncontrol.Binding, error) {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return nil, err
	}
	if root.Legacy || !root.Existing {
		if provided != nil {
			return nil, fmt.Errorf("legacy member execution handoff must not carry current execution control lineage")
		}
		return nil, nil
	}
	if provided != nil {
		validated, err := validateMemberHandoffLaunchControl(caseRoot, owner, provided)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentMemberHandoffControl(caseRoot, owner, validated); err != nil {
			return nil, err
		}
		return validated, nil
	}
	transport, err := memberHandoffTransportCapability()
	if err != nil {
		return nil, err
	}
	binding, err := executioncontrol.CaptureBinding(caseRoot, memberExecutionLaneOwner(owner), transport)
	if err != nil {
		return nil, fmt.Errorf("capture member execution handoff control: %w", err)
	}
	return &binding, nil
}

func memberHandoffTransportCapability() (capabilitycontract.Binding, error) {
	return capabilitycontract.Bind(capabilitycontract.Transport())
}

func validateMemberHandoffLaunchControl(caseRoot string, owner Owner, provided *executioncontrol.Binding) (*executioncontrol.Binding, error) {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return nil, err
	}
	if root.Legacy || !root.Existing {
		if provided != nil {
			return nil, fmt.Errorf("legacy member execution handoff must not carry current execution control lineage")
		}
		return nil, nil
	}
	if provided == nil {
		return nil, fmt.Errorf("current member execution handoff requires frozen execution control lineage")
	}
	transport, err := memberHandoffTransportCapability()
	if err != nil {
		return nil, err
	}
	if err := executioncontrol.ValidateBinding(*provided); err != nil {
		return nil, fmt.Errorf("member execution handoff control is invalid: %w", err)
	}
	if provided.Lane != owner.Lane || provided.Owner != memberExecutionLaneOwner(owner) || provided.Capability != transport {
		return nil, fmt.Errorf("member execution handoff control does not match its transport lane owner")
	}
	return executioncontrol.CloneBinding(provided), nil
}

func memberExecutionLaneOwner(owner Owner) laneowner.Snapshot {
	return laneowner.Snapshot{
		Lane:               owner.Lane,
		CurrentExecutor:    owner.Executor,
		ExecutorGeneration: owner.ExecutorGeneration,
	}
}

func requireCurrentMemberHandoffControlWithLease(caseRoot string, lease *lanemutation.Lease, owner Owner, binding *executioncontrol.Binding) error {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	if root.Legacy || !root.Existing {
		if binding != nil {
			return fmt.Errorf("legacy member execution handoff must not carry current execution control lineage")
		}
		return nil
	}
	if binding == nil {
		return fmt.Errorf("current member execution handoff requires frozen execution control lineage")
	}
	if _, err := validateMemberHandoffLaunchControl(caseRoot, owner, binding); err != nil {
		return err
	}
	if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, *binding); err != nil {
		return fmt.Errorf("member execution handoff control is stale: %w", err)
	}
	return nil
}

func requireCurrentMemberHandoffControl(caseRoot string, owner Owner, binding *executioncontrol.Binding) error {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	if root.Legacy || !root.Existing {
		if binding != nil {
			return fmt.Errorf("legacy member execution handoff must not carry current execution control lineage")
		}
		return nil
	}
	if binding == nil {
		return fmt.Errorf("current member execution handoff requires frozen execution control lineage")
	}
	if _, err := validateMemberHandoffLaunchControl(caseRoot, owner, binding); err != nil {
		return err
	}
	currentness, err := executioncontrol.InspectBindingReadOnly(caseRoot, *binding)
	if err != nil {
		return err
	}
	if !currentness.Current {
		return &executioncontrol.BindingNotCurrentError{Currentness: currentness}
	}
	return nil
}

func currentTaskBindingRel(caseRoot, lane string, ownerGeneration int) (string, error) {
	return projectstate.Rel(caseRoot, "lanes", lane, "member-task-bindings", fmt.Sprintf("g%06d.json", ownerGeneration))
}

func currentTaskBindingOwner(caseRoot, lane string) (string, int, error) {
	lane = strings.TrimSpace(lane)
	if !segment.MatchString(lane) {
		return "", 0, fmt.Errorf("member execution task binding lane is invalid")
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return "", 0, err
	}
	current, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || strings.TrimSpace(current.CurrentExecutor) == "" || current.ExecutorGeneration < 1 {
		return "", 0, fmt.Errorf("member execution task binding requires a current durable executor generation")
	}
	return strings.TrimSpace(current.CurrentExecutor), current.ExecutorGeneration, nil
}

func currentTaskBindingOwnerGeneration(caseRoot, lane string) (int, error) {
	_, generation, err := currentTaskBindingOwner(caseRoot, lane)
	return generation, err
}

func CurrentTaskBinding(caseRoot, lane string) (*TaskBinding, error) {
	ownerGeneration, err := currentTaskBindingOwnerGeneration(caseRoot, lane)
	if err != nil {
		return nil, err
	}
	binding, _, _, err := ReadTaskBindingForOwner(caseRoot, lane, ownerGeneration)
	return binding, err
}

// ReadTaskBindingForOwner inspects one immutable owner-generation binding
// without treating it as the current owner. Callers remain responsible for
// currentness when the binding is used to progress work.
func ReadTaskBindingForOwner(caseRoot, lane string, ownerGeneration int) (*TaskBinding, string, string, error) {
	lane = strings.TrimSpace(lane)
	if !segment.MatchString(lane) || ownerGeneration < 1 {
		return nil, "", "", fmt.Errorf("member execution task binding owner is invalid")
	}
	rel, err := currentTaskBindingRel(caseRoot, lane, ownerGeneration)
	if err != nil {
		return nil, "", "", err
	}
	path, err := rekitfs.SafeJoin(caseRoot, rel)
	if err != nil {
		return nil, "", "", err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, "member execution task binding", maxJSONBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, rel, "", nil
	}
	if err != nil {
		return nil, "", "", err
	}
	var envelope struct {
		SchemaVersion   int         `json:"schemaVersion"`
		Lane            string      `json:"lane"`
		OwnerGeneration int         `json:"ownerGeneration"`
		Binding         TaskBinding `json:"binding"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&envelope); err != nil {
		return nil, "", "", fmt.Errorf("decode member execution task binding: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, "", "", fmt.Errorf("member execution task binding must contain exactly one JSON object")
	}
	if envelope.SchemaVersion != SchemaVersion || envelope.Lane != lane || envelope.OwnerGeneration != ownerGeneration {
		return nil, "", "", fmt.Errorf("member execution task binding does not match the exact lane owner generation")
	}
	binding, err := validateTaskBinding(envelope.Binding)
	if err != nil {
		return nil, "", "", err
	}
	return &binding, rel, hash(data), nil
}

func WriteTaskBinding(caseRoot, lane string, binding TaskBinding) (string, string, error) {
	_, generation, err := currentTaskBindingOwner(caseRoot, lane)
	if err != nil {
		return "", "", err
	}
	return writeTaskBinding(caseRoot, lane, "", generation, false, nil, binding)
}

func WriteTaskBindingForOwner(
	caseRoot,
	lane,
	expectedExecutor string,
	expectedGeneration int,
	binding TaskBinding,
) (string, string, error) {
	return writeTaskBinding(
		caseRoot,
		lane,
		strings.TrimSpace(expectedExecutor),
		expectedGeneration,
		true,
		nil,
		binding,
	)
}

func WriteTaskBindingForOwnerWithControlBinding(
	caseRoot,
	lane,
	expectedExecutor string,
	expectedGeneration int,
	controlBinding executioncontrol.Binding,
	binding TaskBinding,
) (string, string, error) {
	return writeTaskBinding(
		caseRoot,
		lane,
		strings.TrimSpace(expectedExecutor),
		expectedGeneration,
		true,
		&controlBinding,
		binding,
	)
}

func writeTaskBinding(
	caseRoot,
	lane,
	expectedExecutor string,
	expectedGeneration int,
	requireExpectedOwner bool,
	controlBinding *executioncontrol.Binding,
	binding TaskBinding,
) (_ string, _ string, retErr error) {
	binding, err := validateTaskBinding(binding)
	if err != nil {
		return "", "", err
	}
	lane = strings.TrimSpace(lane)
	lease, err := lanemutation.AcquireLane(caseRoot, lane)
	if err != nil {
		return "", "", err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := lease.Validate(); err != nil {
		return "", "", err
	}
	if controlBinding != nil {
		if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, *controlBinding); err != nil {
			return "", "", fmt.Errorf("member execution task binding control is stale: %w", err)
		}
	}
	currentExecutor, ownerGeneration, err := currentTaskBindingOwner(caseRoot, lane)
	if err != nil {
		return "", "", err
	}
	if requireExpectedOwner &&
		(expectedExecutor == "" || expectedGeneration < 1 ||
			currentExecutor != expectedExecutor ||
			ownerGeneration != expectedGeneration) {
		return "", "", fmt.Errorf(
			"member execution task binding owner changed: current executor=%s generation=%d",
			currentExecutor,
			ownerGeneration,
		)
	}
	envelope := struct {
		SchemaVersion   int         `json:"schemaVersion"`
		Lane            string      `json:"lane"`
		OwnerGeneration int         `json:"ownerGeneration"`
		Binding         TaskBinding `json:"binding"`
	}{SchemaVersion: SchemaVersion, Lane: lane, OwnerGeneration: ownerGeneration, Binding: binding}
	data, err := canonical(envelope)
	if err != nil {
		return "", "", err
	}
	rel, err := currentTaskBindingRel(caseRoot, envelope.Lane, ownerGeneration)
	if err != nil {
		return "", "", err
	}
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(caseRoot, rel, "member execution task binding", data); err != nil {
		return "", "", err
	}
	if err := lease.Validate(); err != nil {
		return "", "", fmt.Errorf("member execution task binding may already be durable: %w", err)
	}
	return rel, hash(data), nil
}

func BindCurrentTaskRequestSHA256(caseRoot, lane, requestSHA256 string) (string, error) {
	if !validSHA(requestSHA256) {
		return "", fmt.Errorf("member execution request sha256 is invalid")
	}
	binding, err := CurrentTaskBinding(caseRoot, lane)
	if err != nil {
		return "", err
	}
	if binding == nil {
		return strings.ToLower(requestSHA256), nil
	}
	data, err := canonical(struct {
		RequestSHA256 string      `json:"requestSha256"`
		Binding       TaskBinding `json:"binding"`
	}{RequestSHA256: strings.ToLower(requestSHA256), Binding: *binding})
	if err != nil {
		return "", err
	}
	return hash(data), nil
}

func validateTaskBinding(value TaskBinding) (TaskBinding, error) {
	value.Kind = strings.TrimSpace(value.Kind)
	if !segment.MatchString(value.Kind) || len(value.Values) == 0 || len(value.Values) > 32 {
		return TaskBinding{}, fmt.Errorf("member execution task binding is invalid")
	}
	out := TaskBinding{Kind: value.Kind, Values: make(map[string]string, len(value.Values))}
	for key, item := range value.Values {
		key = strings.TrimSpace(key)
		item = strings.TrimSpace(item)
		if !segment.MatchString(key) || item == "" || len(item) > 4096 || strings.ContainsAny(item, "\r\n") {
			return TaskBinding{}, fmt.Errorf("member execution task binding value is invalid: %s", key)
		}
		out.Values[key] = item
	}
	return out, nil
}

func currentTaskContext(caseRoot, pack string, owner Owner, attemptID string) (TaskContext, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return TaskContext{}, err
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, owner.Lane, false)
	if !ok || lane.CurrentExecutor != owner.Executor || lane.ExecutorGeneration != owner.ExecutorGeneration {
		return TaskContext{}, fmt.Errorf("member execution task context owner changed")
	}
	missionInspection, err := missionintent.Inspect(caseRoot)
	if err != nil {
		return TaskContext{}, fmt.Errorf("inspect member execution mission intent: %w", err)
	}
	goal := strings.TrimSpace(missionInspection.Identity.Goal)
	goalSource := "committed-mission-intent"
	var missionArtifact *TaskArtifact
	if missionInspection.Committed {
		if !strings.EqualFold(missionInspection.Identity.Pack, pack) || !validSHA(missionInspection.MissionIntentSHA256) {
			return TaskContext{}, fmt.Errorf("member execution mission intent identity mismatch")
		}
		paths, err := missionintent.Paths(caseRoot)
		if err != nil {
			return TaskContext{}, err
		}
		missionArtifact = &TaskArtifact{Path: paths.MissionIntent, SHA256: strings.ToLower(missionInspection.MissionIntentSHA256)}
	} else if missionInspection.State == "absent" {
		goal = "Continue the exact current lane work described by the immutable resume and checkpoint artifacts."
		goalSource = "lane-resume-fallback"
	} else {
		return TaskContext{}, fmt.Errorf("member execution requires a committed mission intent; state=%s", missionInspection.State)
	}
	resumeRel, err := projectstate.Rel(caseRoot, "lanes", owner.Lane, "prompts", "RESUME.md")
	if err != nil {
		return TaskContext{}, err
	}
	resume, err := taskArtifact(caseRoot, resumeRel, "member execution lane resume")
	if err != nil {
		return TaskContext{}, err
	}
	checkpointRel, err := projectstate.Rel(caseRoot, "lanes", owner.Lane, "checkpoints", "latest.json")
	if err != nil {
		return TaskContext{}, err
	}
	checkpoint, err := taskArtifact(caseRoot, checkpointRel, "member execution lane checkpoint")
	if err != nil {
		return TaskContext{}, err
	}
	correction, err := currentTaskCorrection(caseRoot, owner.Lane, lane.LastReconciledIntervention)
	if err != nil {
		return TaskContext{}, err
	}
	outputContract, err := currentOutputContract(caseRoot, pack)
	if err != nil {
		return TaskContext{}, err
	}
	expectedOutput := []string{
		"return at least one bounded output derived from the stated goal and current lane context",
		"include the pack output-contract fields: " + strings.Join(outputContract.Fields, ","),
	}
	if correction != nil {
		expectedOutput = append(expectedOutput, "explain how the latest reconciled correction was applied")
	}
	return TaskContext{
		SchemaVersion: TaskContextSchemaVersion, Kind: KindTaskContext, AttemptID: attemptID, Pack: pack,
		ProjectName: strings.TrimSpace(missionInspection.Identity.ProjectName), Goal: goal, GoalSource: goalSource,
		MissionIntent: missionArtifact, Owner: owner, LaneTitle: lane.Title, LaneWorkspace: lane.Workspace,
		Resume: resume, Checkpoint: checkpoint, Correction: correction,
		ExpectedOutput: expectedOutput, OutputContract: &outputContract,
		NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
	}, nil
}

func currentOutputContract(caseRoot, pack string) (OutputContract, error) {
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return OutputContract{}, fmt.Errorf("bind member execution pack output contract: %w", err)
	}
	if inst.Source == "missing" || inst.Moved() || strings.TrimSpace(inst.TemplateRoot) == "" || !strings.EqualFold(inst.TemplatePack, pack) {
		return OutputContract{}, fmt.Errorf("member execution pack output contract does not match attached case metadata")
	}
	m, err := manifest.Load(inst.TemplateRoot, pack)
	if err != nil {
		return OutputContract{}, err
	}
	if err := m.ValidateSchema(); err != nil {
		return OutputContract{}, err
	}
	route, err := m.ExactSubagentRouteForTaskType("feature-analysis")
	if err != nil {
		return OutputContract{}, err
	}
	manifestData, err := rekitfs.ReadStableRegularFileAnchored(inst.TemplateRoot, m.ManifestPath, "member execution pack manifest", maxJSONBytes)
	if err != nil {
		return OutputContract{}, err
	}
	fields := splitOutputContractFields(route.OutputContract)
	if len(fields) == 0 {
		return OutputContract{}, fmt.Errorf("member execution pack output contract is empty: %s", route.ID)
	}
	return OutputContract{
		ManifestPath:   filepath.ToSlash(filepath.Join("packs", pack, "manifest.yml")),
		ManifestSHA256: hash(manifestData), TaskType: "feature-analysis", RouteID: route.ID, Fields: fields,
	}, nil
}

func splitOutputContractFields(value string) []string {
	fields := []string{}
	seen := map[string]bool{}
	for _, field := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' }) {
		field = strings.TrimSpace(field)
		key := strings.ToLower(field)
		if field != "" && !seen[key] {
			seen[key] = true
			fields = append(fields, field)
		}
	}
	return fields
}

func taskArtifact(caseRoot, path, label string) (TaskArtifact, error) {
	full, err := rekitfs.SafeJoin(caseRoot, path)
	if err != nil {
		return TaskArtifact{}, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, full, label, maxJSONBytes)
	if err != nil {
		return TaskArtifact{}, err
	}
	return TaskArtifact{Path: path, SHA256: hash(data), Content: string(data)}, nil
}

func currentTaskCorrection(caseRoot, laneID, sourceID string) (*TaskCorrection, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil, nil
	}
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		return nil, err
	}
	var source, resolution map[string]any
	for _, item := range items {
		if mission.Value(item, "lane") != laneID {
			continue
		}
		if mission.Value(item, "eventId") == sourceID {
			if source != nil {
				return nil, fmt.Errorf("member execution correction source is duplicated: %s", sourceID)
			}
			source = item
		}
		if mission.Value(item, "resolvesEventId") == sourceID && mission.Value(item, "action") == "reconcile" && strings.EqualFold(mission.Value(item, "status"), "resolved") {
			if resolution != nil {
				return nil, fmt.Errorf("member execution correction resolution is duplicated: %s", sourceID)
			}
			resolution = item
		}
	}
	if source == nil || resolution == nil {
		return nil, fmt.Errorf("member execution correction is not durably reconciled: %s", sourceID)
	}
	correction := &TaskCorrection{
		SourceEventID: sourceID, SourceSubject: mission.Value(source, "subject"), SourceSummary: mission.Value(source, "summary"), SourceTarget: mission.Value(source, "target"),
		ResolutionEventID: mission.Value(resolution, "eventId"), ResolutionSummary: mission.Value(resolution, "summary"), ResolutionReason: mission.Value(resolution, "reason"), ResolutionActor: mission.Value(resolution, "actor"), ResolutionTime: mission.Value(resolution, "time"),
	}
	if mission.Value(source, "reviewerDecisionEventId") != "" || mission.Value(source, "reviewerVerificationEventId") != "" {
		rejection, err := currentTaskReviewerRejection(caseRoot, laneID, source)
		if err != nil {
			return nil, fmt.Errorf("member execution correction does not match canonical reviewer rejection %s: %w", sourceID, err)
		}
		correction.ReviewerRejection = rejection
	}
	return correction, nil
}

func currentTaskReviewerRejection(caseRoot, laneID string, source map[string]any) (*ReviewerRejectionContext, error) {
	generation, err := strconv.Atoi(mission.Value(source, "ownerGeneration"))
	if err != nil || generation <= 0 {
		return nil, fmt.Errorf("invalid historical owner generation")
	}
	inputBytes, err := strconv.ParseInt(mission.Value(source, "reviewerResultInputBytes"), 10, 64)
	if err != nil || inputBytes <= 0 {
		return nil, fmt.Errorf("invalid reviewer result input bytes")
	}
	ctx := &ReviewerRejectionContext{
		ManifestRef: mission.Value(source, "target"), ManifestSHA256: mission.Value(source, "reviewerManifestSha256"), PacketID: mission.Value(source, "packetId"), RouteID: mission.Value(source, "routeId"), ShardID: mission.Value(source, "shardId"), PacketPath: mission.Value(source, "packetPath"),
		ReviewerResultPath: mission.Value(source, "reviewerResultPath"), ReviewerResultSHA256: mission.Value(source, "reviewerResultSha256"), ReviewerResultInputPath: mission.Value(source, "reviewerResultInputPath"), ReviewerResultInputSHA256: mission.Value(source, "reviewerResultInputSha256"), ReviewerResultInputBytes: inputBytes,
		ReviewerSession: mission.Value(source, "reviewerSession"), ReviewerDispatchPath: mission.Value(source, "reviewerDispatchReceiptPath"), ReviewerDispatchSHA256: mission.Value(source, "reviewerDispatchReceiptSha256"), ReviewerCompletionPath: mission.Value(source, "reviewerCompletionReceiptPath"), ReviewerCompletionSHA256: mission.Value(source, "reviewerCompletionReceiptSha256"),
		VerificationEventID: mission.Value(source, "reviewerVerificationEventId"), DecisionEventID: mission.Value(source, "reviewerDecisionEventId"), OwnerExecutor: mission.Value(source, "ownerExecutor"), OwnerGeneration: generation,
	}
	for _, value := range []string{ctx.ManifestRef, ctx.ManifestSHA256, ctx.PacketID, ctx.RouteID, ctx.ShardID, ctx.PacketPath, ctx.ReviewerResultPath, ctx.ReviewerResultSHA256, ctx.ReviewerResultInputPath, ctx.ReviewerResultInputSHA256, ctx.ReviewerSession, ctx.ReviewerDispatchPath, ctx.ReviewerDispatchSHA256, ctx.ReviewerCompletionPath, ctx.ReviewerCompletionSHA256, ctx.VerificationEventID, ctx.DecisionEventID, ctx.OwnerExecutor} {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("incomplete reviewer rejection binding")
		}
	}
	verification, err := exactTaskCorrectionFact(caseRoot, "verification", ctx.VerificationEventID)
	if err != nil {
		return nil, err
	}
	decision, err := exactTaskCorrectionFact(caseRoot, "decision", ctx.DecisionEventID)
	if err != nil {
		return nil, err
	}
	if mission.Value(verification, "lane") != laneID || mission.Value(decision, "lane") != laneID || !strings.EqualFold(mission.Value(verification, "verdict"), "rejected") || !strings.EqualFold(mission.Value(decision, "decision"), "reject") || !taskCorrectionEventReferences(decision, ctx.VerificationEventID) {
		return nil, fmt.Errorf("reviewer rejection ledger pair is not canonical")
	}
	for key, expected := range map[string]string{
		"packetId": ctx.PacketID, "routeId": ctx.RouteID, "shardId": ctx.ShardID, "packetPath": ctx.PacketPath, "reviewerResultPath": ctx.ReviewerResultPath, "reviewerSession": ctx.ReviewerSession,
		"reviewerDispatchReceiptPath": ctx.ReviewerDispatchPath, "reviewerDispatchReceiptSha256": ctx.ReviewerDispatchSHA256, "reviewerCompletionReceiptPath": ctx.ReviewerCompletionPath, "reviewerCompletionReceiptSha256": ctx.ReviewerCompletionSHA256,
		"reviewerResultInputPath": ctx.ReviewerResultInputPath, "reviewerResultInputSha256": ctx.ReviewerResultInputSHA256, "ownerExecutor": ctx.OwnerExecutor, "ownerGeneration": fmt.Sprint(ctx.OwnerGeneration), "reviewerDecision": "reject", "recommendedVerdict": "rejected",
	} {
		if mission.Value(verification, key) != expected || mission.Value(decision, key) != expected {
			return nil, fmt.Errorf("reviewer rejection ledger %s binding changed", key)
		}
	}
	ctx.Summary = mission.Value(verification, "summary")
	ctx.EvidenceRefs = taskCorrectionStringList(verification["evidenceRefs"])
	ctx.Risks = taskCorrectionStringList(verification["reviewerRisks"])
	ctx.Conflicts = taskCorrectionStringList(verification["reviewerConflicts"])
	if !sameTaskCorrectionStrings(ctx.EvidenceRefs, taskCorrectionStringList(source["evidenceRefs"])) {
		return nil, fmt.Errorf("reviewer rejection intervention evidence refs differ from canonical verification")
	}
	if ctx.Summary == "" || len(ctx.EvidenceRefs) == 0 {
		return nil, fmt.Errorf("reviewer rejection summary or evidence refs are missing")
	}
	if err := validateTaskCorrectionRejectionFiles(caseRoot, ctx); err != nil {
		return nil, err
	}
	return ctx, nil
}

func exactTaskCorrectionFact(caseRoot, kind, eventID string) (map[string]any, error) {
	items, err := mission.ReadStrictFact(caseRoot, kind)
	if err != nil {
		return nil, err
	}
	var match map[string]any
	for _, item := range items {
		if mission.Value(item, "eventId") != eventID {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("duplicate %s event %s", kind, eventID)
		}
		match = item
	}
	if match == nil {
		return nil, fmt.Errorf("missing %s event %s", kind, eventID)
	}
	return match, nil
}

func taskCorrectionEventReferences(event map[string]any, expected string) bool {
	for _, value := range []any{event["evidenceRefs"], event["related"]} {
		for _, item := range taskCorrectionStringList(value) {
			if strings.EqualFold(item, expected) {
				return true
			}
		}
	}
	return false
}

func taskCorrectionStringList(value any) []string {
	out := []string{}
	add := func(value string) {
		for _, item := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\r' || r == '\n' }) {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
	}
	switch items := value.(type) {
	case []string:
		for _, item := range items {
			add(item)
		}
	case []any:
		for _, item := range items {
			add(fmt.Sprint(item))
		}
	case string:
		add(items)
	}
	return mission.UniqueStrings(out)
}

func sameTaskCorrectionStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameTaskCorrectionPath(left, right string) bool {
	left = filepath.Clean(filepath.FromSlash(strings.TrimSpace(left)))
	right = filepath.Clean(filepath.FromSlash(strings.TrimSpace(right)))
	return strings.EqualFold(left, right)
}

func taskCorrectionResultBindsManifest(caseRoot string, items []string, manifestRef string) bool {
	manifestPath, err := taskCorrectionAnchoredPath(caseRoot, manifestRef)
	if err != nil {
		return false
	}
	for _, item := range items {
		item, _, _ = strings.Cut(strings.TrimSpace(item), "#")
		itemPath, err := taskCorrectionAnchoredPath(caseRoot, item)
		if err == nil && sameTaskCorrectionPath(itemPath, manifestPath) {
			return true
		}
	}
	return false
}

func taskCorrectionAnchoredPath(caseRoot, path string) (string, error) {
	path = filepath.FromSlash(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		return rekitfs.SafeJoin(caseRoot, path)
	}
	rootFull, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootFull, pathFull)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reviewer rejection path escapes case root: %s", path)
	}
	return pathFull, nil
}

func validateTaskCorrectionRejectionFiles(caseRoot string, ctx *ReviewerRejectionContext) error {
	read := func(path, label, expected string) ([]byte, error) {
		full, err := taskCorrectionAnchoredPath(caseRoot, path)
		if err != nil {
			return nil, err
		}
		data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, full, label, maxJSONBytes)
		if err != nil || !strings.EqualFold(hash(data), expected) {
			return nil, fmt.Errorf("%s changed after reviewer rejection", label)
		}
		return data, nil
	}
	manifest, err := read(ctx.ManifestRef, "rejected member manifest", ctx.ManifestSHA256)
	if err != nil || len(manifest) == 0 {
		return err
	}
	input, err := read(ctx.ReviewerResultInputPath, "reviewer result input", ctx.ReviewerResultInputSHA256)
	if err != nil || int64(len(input)) != ctx.ReviewerResultInputBytes {
		return fmt.Errorf("reviewer result input size changed after rejection")
	}
	result, err := reviewerresult.Decode(input)
	if err != nil || result.PacketID != ctx.PacketID || result.RouteID != ctx.RouteID || result.ShardID != ctx.ShardID || result.ReviewerSession != ctx.ReviewerSession || result.Decision != "reject" || result.RecommendedVerdict != "rejected" || !taskCorrectionResultBindsManifest(caseRoot, result.Items, ctx.ManifestRef) {
		return fmt.Errorf("reviewer result input does not match canonical rejection")
	}
	collected, err := read(ctx.ReviewerResultPath, "collected reviewer result", ctx.ReviewerResultSHA256)
	if err != nil {
		return err
	}
	collectedResult, err := reviewerresult.Decode(collected)
	if err != nil {
		return err
	}
	left, _ := json.Marshal(result)
	right, _ := json.Marshal(collectedResult)
	if !bytes.Equal(left, right) {
		return fmt.Errorf("collected reviewer result differs from canonical rejection input")
	}
	dispatchBytes, err := read(ctx.ReviewerDispatchPath, "reviewer dispatch receipt", ctx.ReviewerDispatchSHA256)
	if err != nil {
		return err
	}
	dispatch, err := reviewersession.DecodeDispatch(dispatchBytes)
	if err != nil || dispatch.PacketID != ctx.PacketID || dispatch.RouteID != ctx.RouteID || dispatch.ShardID != ctx.ShardID || dispatch.ReviewerSession != ctx.ReviewerSession || dispatch.TargetLane == "" || dispatch.EffectiveOwner.CurrentExecutor != ctx.OwnerExecutor || dispatch.EffectiveOwner.ExecutorGeneration != ctx.OwnerGeneration {
		return fmt.Errorf("reviewer dispatch receipt does not match historical rejection owner")
	}
	completionBytes, err := read(ctx.ReviewerCompletionPath, "reviewer completion receipt", ctx.ReviewerCompletionSHA256)
	if err != nil {
		return err
	}
	completion, err := reviewersession.DecodeCompletion(completionBytes)
	if err != nil || reviewersession.ValidateCompletionDispatchLineage(completion, dispatch, ctx.ReviewerDispatchPath, ctx.ReviewerDispatchSHA256) != nil || completion.Outcome != "succeeded" || !sameTaskCorrectionPath(completion.ReviewerResultInputPath, ctx.ReviewerResultInputPath) || !strings.EqualFold(completion.ReviewerResultInputSHA256, ctx.ReviewerResultInputSHA256) || int64(completion.ReviewerResultInputBytes) != ctx.ReviewerResultInputBytes {
		return fmt.Errorf("reviewer completion receipt does not match canonical rejection input")
	}
	return nil
}

func PreviewObservation(opt ObservationOptions) (Plan, error) {
	return previewObservationWithLease(opt, nil)
}

func previewObservationWithLease(opt ObservationOptions, lease *lanemutation.Lease) (Plan, error) {
	caseRoot, owner, err := currentOwner(opt.CaseRoot, opt.Pack, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	if !segment.MatchString(opt.AttemptID) {
		return Plan{}, fmt.Errorf("invalid member execution attempt id")
	}
	inspection, err := Inspect(caseRoot, owner.Lane, opt.AttemptID)
	if err != nil {
		return Plan{}, err
	}
	if err := ValidateActionableTaskContext(caseRoot, inspection); err != nil {
		return Plan{}, err
	}
	if inspection.Owner != owner {
		return Plan{}, fmt.Errorf("member execution attempt owner is stale; current executor generation differs")
	}
	if !opt.DeferControlCurrentness {
		if lease != nil {
			if err := requireCurrentMemberHandoffControlWithLease(caseRoot, lease, owner, inspection.Handoff.LaunchControl); err != nil {
				return Plan{}, err
			}
		} else if err := requireCurrentMemberHandoffControl(caseRoot, owner, inspection.Handoff.LaunchControl); err != nil {
			return Plan{}, err
		}
	}
	outcome := strings.ToLower(strings.TrimSpace(opt.Outcome))
	if outcome != "accepted" && outcome != "returned" && outcome != "failed" {
		return Plan{}, fmt.Errorf("member execution outcome must be accepted, returned, or failed")
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		return Plan{}, fmt.Errorf("member execution observation requires actor")
	}
	if strings.TrimSpace(opt.ObservedAt) == "" {
		return Plan{}, fmt.Errorf("member execution observation requires observedAt")
	}
	observed, err := parseTime(opt.ObservedAt)
	if err != nil {
		return Plan{}, err
	}
	if inspection.Latest != nil {
		if inspection.Latest.Outcome == outcome && inspection.Latest.Actor == actor && inspection.Latest.Reason == opt.Reason && inspection.Latest.ObservedAt == observed.Format(time.RFC3339Nano) {
			return finishPlan(Plan{SchemaVersion: 1, Mode: "observe", CaseRoot: caseRoot, Pack: opt.Pack, AttemptID: opt.AttemptID, Owner: owner, Outcome: outcome, Actor: actor, Reason: opt.Reason, Inspection: inspection, ReviewRequired: true, RequiresConfirmation: true, Boundary: boundaries()})
		}
		if inspection.Latest.Outcome == "returned" || inspection.Latest.Outcome == "failed" {
			return Plan{}, fmt.Errorf("member execution attempt is final: %s", inspection.Latest.Outcome)
		}
		if outcome == "accepted" {
			return Plan{}, fmt.Errorf("member execution accepted observation already exists")
		}
	}
	manifestSHA := ""
	resultWrites := []plannedWrite{}
	if outcome == "returned" {
		manifest, sum, writes, err := snapshotResultPlan(inspection, opt.ResultSnapshot)
		if err != nil {
			return Plan{}, err
		}
		inspection.Manifest, inspection.ManifestSHA256, manifestSHA = &manifest, sum, sum
		inspection.ManifestPath, inspection.OutputsRoot = canonicalResultPaths(inspection.AttemptRoot)
		resultWrites = writes
	}
	sequence := 1
	if inspection.Latest != nil {
		sequence = inspection.Latest.Sequence + 1
	}
	observation := Observation{SchemaVersion: 1, Kind: KindObservation, Sequence: sequence, AttemptID: opt.AttemptID, Owner: owner, Outcome: outcome, Actor: actor, Reason: opt.Reason, ObservedAt: observed.Format(time.RFC3339Nano), ManifestSHA256: manifestSHA, IntentSHA256: hashMustCanonical(*inspection.Intent), NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	data, err := canonical(observation)
	if err != nil {
		return Plan{}, err
	}
	path := filepath.Join(inspection.AttemptRoot, "observations", fmt.Sprintf("%020d-%s.json", sequence, outcome))
	resultWrites = append(resultWrites, plannedWrite{path, data})
	inspection.Latest = &observation
	state := outcome
	if outcome == "returned" {
		state = "intake-ready"
	}
	inspection.State = state
	return finishPlan(Plan{SchemaVersion: 1, Mode: "observe", CaseRoot: caseRoot, Pack: opt.Pack, AttemptID: opt.AttemptID, Owner: owner, Outcome: outcome, Actor: actor, Reason: opt.Reason, Inspection: inspection, ReviewRequired: true, RequiresConfirmation: true, Boundary: boundaries(), writes: resultWrites})
}

func Apply(plan Plan, expected string) (Result, error) {
	return ApplyCurrent(plan, expected, nil)
}

func ApplyCurrent(plan Plan, expected string, validateCurrent func() error) (Result, error) {
	if validateCurrent == nil {
		return ApplyCurrentWithLease(plan, expected, nil)
	}
	return ApplyCurrentWithLease(plan, expected, func(_ *lanemutation.Lease) error {
		return validateCurrent()
	})
}

func ApplyCurrentWithLease(plan Plan, expected string, validateCurrent func(*lanemutation.Lease) error) (_ Result, retErr error) {
	if !validSHA(expected) || !strings.EqualFold(expected, plan.ExpectedPlanSHA256) {
		return Result{}, fmt.Errorf("member execution expected plan sha256 mismatch")
	}
	lease, err := lanemutation.AcquireOpenLane(plan.CaseRoot, plan.Owner.Lane, "member execution")
	if err != nil {
		return Result{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if applyLeaseHook != nil {
		if err := applyLeaseHook(plan); err != nil {
			return Result{}, err
		}
	}
	if validateCurrent != nil {
		if err := validateCurrent(lease); err != nil {
			return Result{}, err
		}
	}
	facts, err := mission.ReadStrictLedgerFacts(plan.CaseRoot)
	if err != nil {
		return Result{}, err
	}
	if len(mission.EffectiveOpenLaneInterventions(facts.Facts, plan.Owner.Lane)) > 0 {
		return Result{}, fmt.Errorf("member execution refuses lane with an open intervention")
	}
	rebuilt, err := rebuildApplyPlan(plan, lease)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(rebuilt.ExpectedPlanSHA256, expected) {
		return Result{}, fmt.Errorf("member execution plan changed before Apply")
	}
	plan = rebuilt
	if err := lease.Validate(); err != nil {
		return Result{}, err
	}
	anchor, err := openAnchoredCase(plan.CaseRoot)
	if err != nil {
		return Result{}, err
	}
	defer anchor.Close()
	current, err := currentOwnerAnchored(anchor, plan.Pack, plan.Owner.Lane)
	if err != nil {
		return Result{}, err
	}
	if current != plan.Owner {
		return Result{}, fmt.Errorf("member execution owner changed before Apply")
	}
	caseRoot := anchor.path
	if plan.Mode == "observe" {
		currentInspection, err := inspectAnchored(anchor, plan.Owner.Lane, plan.AttemptID)
		if err != nil {
			return Result{}, err
		}
		if currentInspection.Owner != plan.Owner {
			return Result{}, fmt.Errorf("member execution attempt owner changed before Apply")
		}
		if err := requireCurrentMemberHandoffControlWithLease(caseRoot, lease, plan.Owner, currentInspection.Handoff.LaunchControl); err != nil {
			return Result{}, err
		}
		if plan.Outcome == "returned" {
			_, sum, err := inspectManifestAnchored(anchor, currentInspection)
			if err != nil {
				return Result{}, err
			}
			if !strings.EqualFold(sum, plan.Inspection.ManifestSHA256) {
				return Result{}, fmt.Errorf("member execution result manifest changed after preview")
			}
		}
	}
	if len(plan.writes) == 0 {
		if err := anchor.revalidate(); err != nil {
			return Result{}, err
		}
		return Result{Plan: plan, AlreadyApplied: true}, nil
	}
	rels := make([]string, len(plan.writes))
	firstMissing := len(plan.writes)
	for index, write := range plan.writes {
		rel, err := relativeToCase(caseRoot, write.path)
		if err != nil {
			return Result{}, err
		}
		rels[index] = rel
		existing, err := anchor.readFile(rel, int64(len(write.data))+1)
		if err == nil {
			if firstMissing != len(plan.writes) {
				return Result{}, fmt.Errorf("member execution publication is non-prefix at %s", rel)
			}
			if !bytes.Equal(existing, write.data) {
				return Result{}, fmt.Errorf("member execution existing artifact differs: %s", rel)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return Result{}, err
		}
		if firstMissing == len(plan.writes) {
			firstMissing = index
		}
	}
	written := 0
	for index := firstMissing; index < len(plan.writes); index++ {
		if err := anchor.writeExclusive(rels[index], plan.writes[index].data); err != nil {
			return Result{}, err
		}
		written++
	}
	inspection, err := inspectAnchored(anchor, plan.Owner.Lane, plan.AttemptID)
	if err != nil {
		return Result{}, err
	}
	if err := anchor.revalidate(); err != nil {
		return Result{}, err
	}
	if err := lease.Validate(); err != nil {
		return Result{}, err
	}
	plan.Inspection = inspection
	return Result{Plan: plan, Applied: written > 0, AlreadyApplied: written == 0}, nil
}

func rebuildApplyPlan(plan Plan, lease *lanemutation.Lease) (Plan, error) {
	if plan.Mode == "dispatch" && plan.Inspection.Intent != nil {
		var launchControl *executioncontrol.Binding
		if plan.Inspection.Handoff != nil {
			launchControl = executioncontrol.CloneBinding(plan.Inspection.Handoff.LaunchControl)
		}
		root, err := projectstate.Resolve(plan.CaseRoot)
		if err != nil {
			return Plan{}, err
		}
		if root.Existing && !root.Legacy {
			if launchControl == nil {
				return Plan{}, fmt.Errorf("current member execution dispatch plan omitted frozen execution control lineage")
			}
			if err := requireCurrentMemberHandoffControlWithLease(
				plan.CaseRoot,
				lease,
				plan.Owner,
				launchControl,
			); err != nil {
				return Plan{}, err
			}
		}
		return PreviewDispatch(DispatchOptions{CaseRoot: plan.CaseRoot, Pack: plan.Pack, Lane: plan.Owner.Lane, RequestSHA256: plan.Inspection.Intent.RequestSHA256, CreatedAt: plan.Inspection.Intent.CreatedAt, LaunchControl: launchControl})
	}
	if plan.Mode == "observe" && plan.Inspection.Latest != nil {
		observation := plan.Inspection.Latest
		return previewObservationWithLease(ObservationOptions{CaseRoot: plan.CaseRoot, Pack: plan.Pack, Lane: plan.Owner.Lane, AttemptID: plan.AttemptID, Outcome: plan.Outcome, Actor: plan.Actor, Reason: plan.Reason, ObservedAt: observation.ObservedAt, DeferControlCurrentness: plan.Inspection.Handoff != nil && plan.Inspection.Handoff.LaunchControl != nil}, lease)
	}
	return Plan{}, fmt.Errorf("member execution plan cannot be rebuilt")
}

func Inspect(caseRoot, lane, attemptID string) (Inspection, error) {
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return Inspection{}, err
	}
	defer anchor.Close()
	inspection, err := inspectAnchored(anchor, lane, attemptID)
	if err != nil {
		return Inspection{}, err
	}
	if err := anchor.revalidate(); err != nil {
		return Inspection{}, err
	}
	return inspection, nil
}

func inspectAnchored(anchor *anchoredCase, lane, attemptID string) (Inspection, error) {
	root, err := attemptRoot(anchor.path, lane, attemptID)
	if err != nil {
		return Inspection{}, err
	}
	rootRel, err := relativeToCase(anchor.path, root)
	if err != nil {
		return Inspection{}, err
	}
	intentBytes, err := anchor.readFile(filepath.Join(rootRel, "intent.json"), maxJSONBytes)
	if err != nil {
		return Inspection{}, err
	}
	var intent Intent
	if err := strictCanonical(intentBytes, &intent); err != nil {
		return Inspection{}, err
	}
	taskContextBytes, err := anchor.readFile(filepath.Join(rootRel, "task-context.json"), maxJSONBytes)
	if err != nil {
		return Inspection{}, err
	}
	var taskContext TaskContext
	if err := strictCanonical(taskContextBytes, &taskContext); err != nil {
		return Inspection{}, err
	}
	handoffBytes, err := anchor.readFile(filepath.Join(rootRel, "handoff.json"), maxJSONBytes)
	if err != nil {
		return Inspection{}, err
	}
	var handoff Handoff
	if err := strictCanonical(handoffBytes, &handoff); err != nil {
		return Inspection{}, err
	}
	commitBytes, err := anchor.readFile(filepath.Join(rootRel, "commit.json"), maxJSONBytes)
	if err != nil {
		return Inspection{}, err
	}
	var commit Commit
	if err := strictCanonical(commitBytes, &commit); err != nil {
		return Inspection{}, err
	}
	if intent.SchemaVersion != SchemaVersion || intent.Kind != KindIntent || intent.AttemptID != attemptID || !samePath(intent.CaseRoot, anchor.path) || intent.Owner.Lane != lane || !validSHA(intent.RequestSHA256) || strings.TrimSpace(intent.Pack) == "" || !intent.NoSpawn || !intent.NoPoll || !intent.NoStop || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool {
		return Inspection{}, fmt.Errorf("invalid member execution intent")
	}
	taskContextRel := filepath.ToSlash(filepath.Join(rootRel, "task-context.json"))
	if err := validateTaskContextContract(anchor.path, intent, taskContext); err != nil {
		return Inspection{}, err
	}
	if handoff.SchemaVersion != SchemaVersion || handoff.Kind != KindHandoff || handoff.AttemptID != attemptID || handoff.Owner != intent.Owner || !strings.EqualFold(handoff.IntentSHA256, hash(intentBytes)) || handoff.TaskContextPath != taskContextRel || !strings.EqualFold(handoff.TaskContextSHA256, hash(taskContextBytes)) {
		return Inspection{}, fmt.Errorf("invalid member execution handoff binding")
	}
	if handoff.LaunchControl != nil {
		if _, err := validateMemberHandoffLaunchControl(anchor.path, intent.Owner, handoff.LaunchControl); err != nil {
			return Inspection{}, err
		}
	}
	manifestRel := filepath.ToSlash(filepath.Join(rootRel, "result", "manifest.json"))
	outputsRel := filepath.ToSlash(filepath.Join(rootRel, "result", "outputs"))
	if handoff.ManifestPath != manifestRel || handoff.OutputsRoot != outputsRel {
		return Inspection{}, fmt.Errorf("invalid member execution handoff result paths")
	}
	if commit.SchemaVersion != SchemaVersion || commit.Kind != KindCommit || commit.AttemptID != attemptID || !strings.EqualFold(commit.IntentSHA256, hash(intentBytes)) || !strings.EqualFold(commit.TaskContextSHA256, hash(taskContextBytes)) || !strings.EqualFold(commit.HandoffSHA256, hash(handoffBytes)) {
		return Inspection{}, fmt.Errorf("invalid member execution commit binding")
	}
	inspection := Inspection{State: "handoff-ready", AttemptID: attemptID, Owner: intent.Owner, Intent: &intent, TaskContext: &taskContext, TaskContextPath: filepath.Join(root, "task-context.json"), TaskContextSHA256: hash(taskContextBytes), Handoff: &handoff, HandoffSHA256: hash(handoffBytes), AttemptRoot: root, ManifestPath: filepath.Join(root, "result", "manifest.json"), OutputsRoot: filepath.Join(root, "result", "outputs")}
	observationRel := filepath.Join(rootRel, "observations")
	entries, err := anchor.readDir(observationRel)
	if err != nil && !os.IsNotExist(err) {
		return Inspection{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	previous := ""
	for index, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !entry.Type().IsRegular() {
			return Inspection{}, fmt.Errorf("member execution observations contain invalid entry: %s", entry.Name())
		}
		data, err := anchor.readFile(filepath.Join(observationRel, entry.Name()), maxJSONBytes)
		if err != nil {
			return Inspection{}, err
		}
		var observation Observation
		if err := strictCanonical(data, &observation); err != nil {
			return Inspection{}, err
		}
		expectedName := fmt.Sprintf("%020d-%s.json", index+1, observation.Outcome)
		validTransition := (index == 0 && (observation.Outcome == "accepted" || observation.Outcome == "returned" || observation.Outcome == "failed")) || (index == 1 && previous == "accepted" && (observation.Outcome == "returned" || observation.Outcome == "failed"))
		if entry.Name() != expectedName || !validTransition || observation.SchemaVersion != SchemaVersion || observation.Kind != KindObservation || observation.AttemptID != attemptID || observation.Owner != intent.Owner || observation.Sequence != index+1 || !strings.EqualFold(observation.IntentSHA256, hash(intentBytes)) || !observation.NoAuthority || !observation.NoConfirmed || !observation.NoHeavyTool {
			return Inspection{}, fmt.Errorf("invalid member execution observation chain")
		}
		if observation.Outcome == "returned" && !validSHA(observation.ManifestSHA256) {
			return Inspection{}, fmt.Errorf("returned member execution observation requires manifest sha256")
		}
		if observation.Outcome != "returned" && observation.ManifestSHA256 != "" {
			return Inspection{}, fmt.Errorf("non-returned member execution observation must not bind a manifest")
		}
		copy := observation
		inspection.Latest = &copy
		inspection.State = observation.Outcome
		previous = observation.Outcome
	}
	if inspection.Latest != nil && inspection.Latest.Outcome == "returned" {
		inspection.ManifestPath, inspection.OutputsRoot = canonicalResultPaths(inspection.AttemptRoot)
		manifest, sum, err := inspectManifestAnchored(anchor, inspection)
		if err != nil {
			return Inspection{}, err
		}
		if !strings.EqualFold(sum, inspection.Latest.ManifestSHA256) {
			return Inspection{}, fmt.Errorf("member execution manifest drift after returned observation")
		}
		inspection.Manifest, inspection.ManifestSHA256, inspection.State = &manifest, sum, "intake-ready"
	}
	return inspection, nil
}

func validateCurrentTaskBinding(task TaskContext) error {
	if task.Binding == nil {
		return nil
	}
	binding, err := validateTaskBinding(*task.Binding)
	if err != nil {
		return err
	}
	if binding.Kind != task.Binding.Kind || !reflect.DeepEqual(binding.Values, task.Binding.Values) {
		return fmt.Errorf("member execution task binding is not canonical")
	}
	return nil
}

func ValidateCurrentTaskContext(caseRoot string, inspection Inspection) error {
	if inspection.Intent == nil || inspection.TaskContext == nil {
		return fmt.Errorf("member execution attempt omitted its immutable task context")
	}
	if err := validateCurrentTaskBinding(*inspection.TaskContext); err != nil {
		return err
	}
	return validateTaskContext(caseRoot, *inspection.Intent, *inspection.TaskContext)
}

func ValidateActionableTaskContext(caseRoot string, inspection Inspection) error {
	if inspection.TaskContext == nil || inspection.TaskContext.SchemaVersion != TaskContextSchemaVersion || inspection.TaskContext.OutputContract == nil {
		return fmt.Errorf("legacy member execution task context is read-only; dispatch a current task-context generation before continuing execution")
	}
	if err := ValidateCurrentTaskContext(caseRoot, inspection); err != nil {
		return err
	}
	task := *inspection.TaskContext
	current, err := CurrentTaskBinding(caseRoot, task.Owner.Lane)
	if err != nil {
		return fmt.Errorf("read current member execution task binding: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(task.Pack), defaults.DefaultPack) && task.Binding == nil {
		return fmt.Errorf("binary-re member execution requires a current owner-generation task binding")
	}
	if !equalTaskBinding(task.Binding, current) {
		return fmt.Errorf("member execution task binding changed after dispatch")
	}
	if task.Binding != nil && IsTaskInputBinding(*task.Binding) {
		return ValidateTaskInputBinding(caseRoot, *task.Binding)
	}
	return nil
}

func equalTaskBinding(left, right *TaskBinding) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Kind == right.Kind && reflect.DeepEqual(left.Values, right.Values)
}

func ValidateTaskContextPackContract(caseRoot string, inspection Inspection) error {
	if inspection.Intent == nil || inspection.TaskContext == nil {
		return fmt.Errorf("member execution attempt omitted its immutable task context")
	}
	task := *inspection.TaskContext
	if task.SchemaVersion != TaskContextSchemaVersion || task.OutputContract == nil {
		return fmt.Errorf("legacy member execution task context does not bind a current pack output contract")
	}
	if err := validateTaskContextContract(caseRoot, *inspection.Intent, task); err != nil {
		return err
	}
	if err := validateCurrentTaskBinding(task); err != nil {
		return err
	}
	if err := validateTaskContextMission(caseRoot, task); err != nil {
		return err
	}
	if err := validateTaskContextOutputContract(caseRoot, task); err != nil {
		return err
	}
	if task.Binding != nil && IsTaskInputBinding(*task.Binding) {
		return ValidateTaskInputBinding(caseRoot, *task.Binding)
	}
	return nil
}

func validateTaskContextContract(caseRoot string, intent Intent, task TaskContext) error {
	if (task.SchemaVersion != legacyTaskContextVersion && task.SchemaVersion != TaskContextSchemaVersion) || task.Kind != KindTaskContext || task.AttemptID != intent.AttemptID || task.Owner != intent.Owner || !strings.EqualFold(task.Pack, intent.Pack) || strings.TrimSpace(task.Goal) == "" || strings.TrimSpace(task.GoalSource) == "" || strings.TrimSpace(task.Resume.Content) == "" || strings.TrimSpace(task.Checkpoint.Content) == "" || !task.NoAuthority || !task.NoConfirmed || !task.NoHeavyTool || len(task.ExpectedOutput) == 0 {
		return fmt.Errorf("invalid member execution task context")
	}
	if task.SchemaVersion == legacyTaskContextVersion && task.OutputContract != nil {
		return fmt.Errorf("legacy member execution task context must not contain a pack output contract")
	}
	if task.SchemaVersion == TaskContextSchemaVersion {
		if task.OutputContract == nil || !validSHA(task.OutputContract.ManifestSHA256) || !validRelative(task.OutputContract.ManifestPath) || task.OutputContract.TaskType != "feature-analysis" || strings.TrimSpace(task.OutputContract.RouteID) == "" || len(task.OutputContract.Fields) == 0 {
			return fmt.Errorf("invalid member execution task context")
		}
		canonicalFields := splitOutputContractFields(strings.Join(task.OutputContract.Fields, ","))
		if !reflect.DeepEqual(canonicalFields, task.OutputContract.Fields) {
			return fmt.Errorf("invalid member execution pack output contract fields")
		}
	}
	for _, artifact := range []TaskArtifact{task.Resume, task.Checkpoint} {
		if !validRelative(artifact.Path) || !validSHA(artifact.SHA256) || !strings.EqualFold(hash([]byte(artifact.Content)), artifact.SHA256) {
			return fmt.Errorf("invalid member execution task artifact binding")
		}
	}
	return validateTaskContextMissionBinding(caseRoot, task)
}

func validateTaskContextMissionBinding(caseRoot string, task TaskContext) error {
	if task.MissionIntent != nil {
		paths, err := missionintent.Paths(caseRoot)
		if err != nil {
			return fmt.Errorf("resolve member execution mission intent path: %w", err)
		}
		if task.GoalSource != "committed-mission-intent" || task.MissionIntent.Path != paths.MissionIntent || !validSHA(task.MissionIntent.SHA256) {
			return fmt.Errorf("invalid member execution mission intent binding")
		}
	} else if task.GoalSource != "lane-resume-fallback" {
		return fmt.Errorf("invalid member execution goal source")
	}
	return nil
}

func validateTaskContext(caseRoot string, intent Intent, task TaskContext) error {
	if err := validateTaskContextContract(caseRoot, intent, task); err != nil {
		return err
	}
	if err := validateCurrentTaskBinding(task); err != nil {
		return err
	}
	for _, artifact := range []TaskArtifact{task.Resume, task.Checkpoint} {
		full, err := rekitfs.SafeJoin(caseRoot, artifact.Path)
		if err != nil {
			return err
		}
		data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, full, "member execution current task artifact", maxJSONBytes)
		if err != nil || !strings.EqualFold(hash(data), artifact.SHA256) {
			return fmt.Errorf("member execution task artifact changed: %s", artifact.Path)
		}
	}
	if err := validateTaskContextMission(caseRoot, task); err != nil {
		return err
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return err
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, task.Owner.Lane, false)
	if !ok || lane.CurrentExecutor != task.Owner.Executor || lane.ExecutorGeneration != task.Owner.ExecutorGeneration {
		return fmt.Errorf("member execution task context owner is stale")
	}
	currentCorrection, err := currentTaskCorrection(caseRoot, task.Owner.Lane, lane.LastReconciledIntervention)
	if err != nil {
		return err
	}
	if !equalTaskCorrection(task.Correction, currentCorrection) {
		return fmt.Errorf("member execution correction changed")
	}
	return validateTaskContextOutputContract(caseRoot, task)
}

func validateTaskContextMission(caseRoot string, task TaskContext) error {
	if err := validateTaskContextMissionBinding(caseRoot, task); err != nil {
		return err
	}
	if task.MissionIntent != nil {
		inspection, err := missionintent.Inspect(caseRoot)
		if err != nil || !inspection.Committed || inspection.Identity.Goal != task.Goal || inspection.Identity.ProjectName != task.ProjectName || !strings.EqualFold(inspection.Identity.Pack, task.Pack) || !strings.EqualFold(inspection.MissionIntentSHA256, task.MissionIntent.SHA256) {
			return fmt.Errorf("member execution mission intent changed")
		}
	}
	return nil
}

func validateTaskContextOutputContract(caseRoot string, task TaskContext) error {
	if task.SchemaVersion != TaskContextSchemaVersion {
		return nil
	}
	currentContract, err := currentOutputContract(caseRoot, task.Pack)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(*task.OutputContract, currentContract) {
		return fmt.Errorf("member execution pack output contract changed")
	}
	return nil
}

func equalTaskCorrection(left, right *TaskCorrection) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}

func CurrentOwnerMatches(caseRoot, pack string, owner Owner) (bool, error) {
	_, current, err := currentOwner(caseRoot, pack, owner.Lane)
	if err != nil {
		return false, err
	}
	return current == owner, nil
}

func currentOwner(caseRoot, pack, lane string) (string, Owner, error) {
	if !segment.MatchString(lane) {
		return "", Owner{}, fmt.Errorf("invalid member execution lane")
	}
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return "", Owner{}, err
	}
	defer anchor.Close()
	owner, err := currentOwnerAnchored(anchor, pack, lane)
	if err != nil {
		return "", Owner{}, err
	}
	if err := anchor.revalidate(); err != nil {
		return "", Owner{}, err
	}
	return anchor.path, owner, nil
}

func currentOwnerAnchored(anchor *anchoredCase, pack, lane string) (Owner, error) {
	data, err := anchor.readFile(anchor.stateRel("board.json"), maxJSONBytes)
	if err != nil {
		return Owner{}, err
	}
	var board mission.Board
	if err := json.Unmarshal(data, &board); err != nil {
		return Owner{}, fmt.Errorf("invalid member execution board: %w", err)
	}
	if !samePath(board.CaseRoot, anchor.path) || !strings.EqualFold(board.Pack, pack) {
		return Owner{}, fmt.Errorf("member execution board identity mismatch")
	}
	entry, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok {
		return Owner{}, fmt.Errorf("member execution lane is not on board")
	}
	if strings.TrimSpace(entry.CurrentExecutor) == "" || entry.ExecutorGeneration < 1 {
		return Owner{}, fmt.Errorf("member execution lane has no current executor generation")
	}
	return Owner{Lane: entry.ID, Executor: entry.CurrentExecutor, ExecutorGeneration: entry.ExecutorGeneration}, nil
}

func currentOwnerUpdatedAt(caseRoot, lane string) (string, error) {
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return "", err
	}
	defer anchor.Close()
	data, err := anchor.readFile(anchor.stateRel("board.json"), maxJSONBytes)
	if err != nil {
		return "", err
	}
	var board mission.Board
	if err := json.Unmarshal(data, &board); err != nil {
		return "", err
	}
	entry, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || strings.TrimSpace(entry.UpdatedAt) == "" {
		return "", fmt.Errorf("member execution lane has no durable updatedAt")
	}
	return entry.UpdatedAt, nil
}

func canonicalResultPaths(attemptRoot string) (string, string) {
	root := filepath.Join(attemptRoot, "evidence")
	return filepath.Join(root, "manifest.json"), filepath.Join(root, "outputs")
}

func snapshotResultPlan(inspection Inspection, snapshot *ResultSnapshot) (ResultManifest, string, []plannedWrite, error) {
	if snapshot != nil {
		return snapshotResultPlanFromSnapshot(inspection, *snapshot)
	}
	anchor, err := openAnchoredCase(inspection.Intent.CaseRoot)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	defer anchor.Close()
	manifest, sum, err := inspectManifestAnchored(anchor, inspection)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	manifestData, err := canonical(manifest)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	manifestPath, outputsRoot := canonicalResultPaths(inspection.AttemptRoot)
	sourceOutputsRoot, err := relativeToCase(anchor.path, inspection.OutputsRoot)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	writes := []plannedWrite{}
	for _, output := range manifest.Outputs {
		data, err := anchor.readFile(filepath.Join(sourceOutputsRoot, filepath.FromSlash(output.Path)), output.Bytes)
		if err != nil {
			return ResultManifest{}, "", nil, err
		}
		if int64(len(data)) != output.Bytes || !strings.EqualFold(hash(data), output.SHA256) {
			return ResultManifest{}, "", nil, fmt.Errorf("member execution output changed while snapshotting: %s", output.Path)
		}
		writes = append(writes, plannedWrite{filepath.Join(outputsRoot, filepath.FromSlash(output.Path)), data})
	}
	if err := anchor.revalidate(); err != nil {
		return ResultManifest{}, "", nil, err
	}
	writes = append(writes, plannedWrite{manifestPath, manifestData})
	return manifest, sum, writes, nil
}

func snapshotResultPlanFromSnapshot(inspection Inspection, snapshot ResultSnapshot) (ResultManifest, string, []plannedWrite, error) {
	if !samePath(snapshot.ManifestPath, inspection.ManifestPath) || !samePath(snapshot.OutputsRoot, inspection.OutputsRoot) {
		return ResultManifest{}, "", nil, fmt.Errorf("member execution result snapshot paths do not match the current handoff")
	}
	var manifest ResultManifest
	if err := strictCanonical(snapshot.ManifestData, &manifest); err != nil {
		return ResultManifest{}, "", nil, err
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Kind != KindManifest || manifest.AttemptID != inspection.AttemptID || manifest.Owner != inspection.Owner || strings.TrimSpace(manifest.Summary) == "" || len(manifest.Outputs) == 0 || len(manifest.Outputs) > maxOutputs || !manifest.NoAuthority || !manifest.NoConfirmed || !manifest.NoHeavyTool {
		return ResultManifest{}, "", nil, fmt.Errorf("invalid member execution result manifest")
	}
	seen := map[string]bool{}
	writes := []plannedWrite{}
	_, outputsRoot := canonicalResultPaths(inspection.AttemptRoot)
	for _, output := range manifest.Outputs {
		key := strings.ToLower(output.Path)
		if !validRelative(output.Path) || seen[key] || !validSHA(output.SHA256) || output.Bytes < 1 || output.Bytes > maxOutputBytes {
			return ResultManifest{}, "", nil, fmt.Errorf("invalid member execution output contract: %s", output.Path)
		}
		seen[key] = true
		data, ok := snapshot.Outputs[key]
		if !ok || int64(len(data)) != output.Bytes || !strings.EqualFold(hash(data), output.SHA256) {
			return ResultManifest{}, "", nil, fmt.Errorf("member execution output hash or size drift: %s", output.Path)
		}
		writes = append(writes, plannedWrite{filepath.Join(outputsRoot, filepath.FromSlash(output.Path)), append([]byte{}, data...)})
	}
	if len(snapshot.Outputs) != len(seen) {
		return ResultManifest{}, "", nil, fmt.Errorf("member execution outputs do not exactly match manifest")
	}
	for path := range snapshot.Outputs {
		if !seen[strings.ToLower(filepath.ToSlash(path))] {
			return ResultManifest{}, "", nil, fmt.Errorf("member execution output component is not declared by manifest: %s", path)
		}
	}
	if manifest.ReviewerItemsPath != "" && (!validRelative(manifest.ReviewerItemsPath) || !seen[strings.ToLower(manifest.ReviewerItemsPath)]) {
		return ResultManifest{}, "", nil, fmt.Errorf("reviewerItemsPath must name a declared output")
	}
	manifestData, err := canonical(manifest)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	manifestPath, _ := canonicalResultPaths(inspection.AttemptRoot)
	writes = append(writes, plannedWrite{manifestPath, manifestData})
	return manifest, hash(snapshot.ManifestData), writes, nil
}

func inspectManifestAnchored(anchor *anchoredCase, inspection Inspection) (ResultManifest, string, error) {
	manifestRel, err := relativeToCase(anchor.path, inspection.ManifestPath)
	if err != nil {
		return ResultManifest{}, "", err
	}
	data, err := anchor.readFile(manifestRel, maxJSONBytes)
	if err != nil {
		return ResultManifest{}, "", err
	}
	var manifest ResultManifest
	if err := strictCanonical(data, &manifest); err != nil {
		return ResultManifest{}, "", err
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Kind != KindManifest || manifest.AttemptID != inspection.AttemptID || manifest.Owner != inspection.Owner || strings.TrimSpace(manifest.Summary) == "" || len(manifest.Outputs) == 0 || len(manifest.Outputs) > maxOutputs || !manifest.NoAuthority || !manifest.NoConfirmed || !manifest.NoHeavyTool {
		return ResultManifest{}, "", fmt.Errorf("invalid member execution result manifest")
	}
	outputsRel, err := relativeToCase(anchor.path, inspection.OutputsRoot)
	if err != nil {
		return ResultManifest{}, "", err
	}
	seen := map[string]bool{}
	for _, output := range manifest.Outputs {
		key := strings.ToLower(output.Path)
		if !validRelative(output.Path) || seen[key] || !validSHA(output.SHA256) || output.Bytes < 1 || output.Bytes > maxOutputBytes {
			return ResultManifest{}, "", fmt.Errorf("invalid member execution output contract: %s", output.Path)
		}
		seen[key] = true
		content, err := anchor.readFile(filepath.Join(outputsRel, filepath.FromSlash(output.Path)), output.Bytes)
		if err != nil {
			return ResultManifest{}, "", err
		}
		if int64(len(content)) != output.Bytes || !strings.EqualFold(hash(content), output.SHA256) {
			return ResultManifest{}, "", fmt.Errorf("member execution output hash or size drift: %s", output.Path)
		}
	}
	expectedEntries := map[string]bool{}
	for path := range seen {
		expectedEntries[path] = true
		for parent := filepath.ToSlash(filepath.Dir(path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			expectedEntries[strings.ToLower(parent)+"/"] = true
		}
	}
	actual, err := memberOutputPathsAnchored(anchor, outputsRel, "")
	if err != nil {
		return ResultManifest{}, "", err
	}
	if len(actual) != len(expectedEntries) {
		return ResultManifest{}, "", fmt.Errorf("member execution outputs do not exactly match manifest")
	}
	for _, path := range actual {
		if !expectedEntries[strings.ToLower(filepath.ToSlash(path))] {
			return ResultManifest{}, "", fmt.Errorf("member execution output component is not declared by manifest: %s", path)
		}
	}
	if manifest.ReviewerItemsPath != "" && (!validRelative(manifest.ReviewerItemsPath) || !seen[strings.ToLower(manifest.ReviewerItemsPath)]) {
		return ResultManifest{}, "", fmt.Errorf("reviewerItemsPath must name a declared output")
	}
	return manifest, hash(data), nil
}

func memberOutputPathsAnchored(anchor *anchoredCase, rootRel, prefix string) ([]string, error) {
	entries, err := anchor.readDir(rootRel)
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, entry := range entries {
		name := filepath.ToSlash(filepath.Join(prefix, entry.Name()))
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("member execution outputs contain symlink entry: %s", name)
		}
		if entry.IsDir() {
			paths = append(paths, name+"/")
			nested, err := memberOutputPathsAnchored(anchor, filepath.Join(rootRel, entry.Name()), name)
			if err != nil {
				return nil, err
			}
			paths = append(paths, nested...)
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("member execution outputs contain invalid entry: %s", name)
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths, nil
}

func Latest(caseRoot, lane string) (Inspection, bool, error) {
	if !segment.MatchString(lane) {
		return Inspection{}, false, fmt.Errorf("invalid member execution lane")
	}
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return Inspection{}, false, err
	}
	defer anchor.Close()
	rootRel := anchor.stateRel("lanes", lane, "member-executions")
	entries, err := anchor.readDir(rootRel)
	if os.IsNotExist(err) {
		return Inspection{}, false, nil
	}
	if err != nil {
		return Inspection{}, false, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() || !segment.MatchString(entry.Name()) {
			return Inspection{}, false, fmt.Errorf("member execution root contains invalid attempt entry: %s", entry.Name())
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return Inspection{}, false, nil
	}
	sort.Strings(names)
	latestName := names[len(names)-1]
	inspection, err := inspectAnchored(anchor, lane, latestName)
	if err != nil {
		pending, pendingErr := inspectPendingDispatchPrefix(anchor, lane, latestName)
		if pendingErr != nil {
			return Inspection{}, false, pendingErr
		}
		if pending {
			return Inspection{}, false, fmt.Errorf("%w: %s", errPendingDispatch, latestName)
		}
		return Inspection{}, false, err
	}
	if err := anchor.revalidate(); err != nil {
		return Inspection{}, false, err
	}
	return inspection, true, nil
}

func inspectPendingDispatchPrefix(anchor *anchoredCase, lane, attemptID string) (bool, error) {
	root, err := attemptRoot(anchor.path, lane, attemptID)
	if err != nil {
		return false, err
	}
	rootRel, err := relativeToCase(anchor.path, root)
	if err != nil {
		return false, err
	}
	entries, err := anchor.readDir(rootRel)
	if err != nil {
		return false, err
	}
	entryByName := make(map[string]os.DirEntry, len(entries))
	for _, entry := range entries {
		if _, duplicate := entryByName[entry.Name()]; duplicate {
			return false, fmt.Errorf("member execution pending dispatch contains duplicate entry: %s", entry.Name())
		}
		entryByName[entry.Name()] = entry
	}
	for name, entry := range entryByName {
		if name != "intent.json" && name != "task-context.json" && name != "handoff.json" {
			return false, nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return false, fmt.Errorf("member execution pending dispatch contains invalid entry: %s", name)
		}
	}
	intentEntry, hasIntent := entryByName["intent.json"]
	if !hasIntent || intentEntry == nil {
		return false, nil
	}
	intentBytes, err := anchor.readFile(filepath.Join(rootRel, "intent.json"), maxJSONBytes)
	if err != nil {
		return false, err
	}
	var intent Intent
	if err := strictCanonical(intentBytes, &intent); err != nil || intent.SchemaVersion != SchemaVersion || intent.Kind != KindIntent || intent.AttemptID != attemptID || !samePath(intent.CaseRoot, anchor.path) || intent.Owner.Lane != lane || !validSHA(intent.RequestSHA256) || strings.TrimSpace(intent.Pack) == "" || !intent.NoSpawn || !intent.NoPoll || !intent.NoStop || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool {
		return false, fmt.Errorf("invalid pending member execution intent")
	}
	created, err := time.Parse(time.RFC3339Nano, intent.CreatedAt)
	if err != nil || created.Format(time.RFC3339Nano) != intent.CreatedAt {
		return false, fmt.Errorf("invalid pending member execution intent createdAt")
	}
	parts := strings.Split(attemptID, "-")
	if len(parts) != 3 {
		return false, fmt.Errorf("invalid pending member execution attempt id")
	}
	sequence, err := strconv.Atoi(strings.TrimPrefix(parts[1], "a"))
	if err != nil || sequence < 1 || attemptID != fmt.Sprintf("g%06d-a%06d-%s", intent.Owner.ExecutorGeneration, sequence, strings.ToLower(intent.RequestSHA256[:16])) {
		return false, fmt.Errorf("pending member execution attempt id does not match intent")
	}
	owner, err := currentOwnerAnchored(anchor, intent.Pack, lane)
	if err != nil {
		return false, err
	}
	if owner != intent.Owner {
		return false, fmt.Errorf("pending member execution owner generation is stale")
	}
	taskContextEntry, hasTaskContext := entryByName["task-context.json"]
	if !hasTaskContext || taskContextEntry == nil {
		if _, hasHandoff := entryByName["handoff.json"]; hasHandoff {
			return false, fmt.Errorf("pending member execution handoff exists without task context")
		}
		return true, nil
	}
	taskContextBytes, err := anchor.readFile(filepath.Join(rootRel, "task-context.json"), maxJSONBytes)
	if err != nil {
		return false, err
	}
	var taskContext TaskContext
	if err := strictCanonical(taskContextBytes, &taskContext); err != nil {
		return false, fmt.Errorf("invalid pending member execution task context: %w", err)
	}
	if err := validateTaskContext(anchor.path, intent, taskContext); err != nil {
		return false, err
	}
	if _, hasHandoff := entryByName["handoff.json"]; hasHandoff {
		handoffBytes, err := anchor.readFile(filepath.Join(rootRel, "handoff.json"), maxJSONBytes)
		if err != nil {
			return false, err
		}
		var handoff Handoff
		if err := strictCanonical(handoffBytes, &handoff); err != nil {
			return false, fmt.Errorf("invalid pending member execution handoff: %w", err)
		}
		launchControl, err := validateMemberHandoffLaunchControl(
			anchor.path,
			intent.Owner,
			handoff.LaunchControl,
		)
		if err != nil {
			return false, err
		}
		expected := Handoff{
			SchemaVersion:     SchemaVersion,
			Kind:              KindHandoff,
			AttemptID:         attemptID,
			Owner:             intent.Owner,
			IntentSHA256:      hash(intentBytes),
			TaskContextPath:   filepath.ToSlash(filepath.Join(rootRel, "task-context.json")),
			TaskContextSHA256: hash(taskContextBytes),
			ManifestPath:      filepath.ToSlash(filepath.Join(rootRel, "result", "manifest.json")),
			OutputsRoot:       filepath.ToSlash(filepath.Join(rootRel, "result", "outputs")),
			LaunchControl:     executioncontrol.CloneBinding(launchControl),
			NextSteps:         []string{"external harness accepts this handoff", "external member reads the exact immutable task context and writes bounded outputs", "record accepted then returned or failed observation through run-current-step"},
			Boundary:          boundaries(),
		}
		expectedBytes, err := canonical(expected)
		if err != nil {
			return false, err
		}
		if !bytes.Equal(handoffBytes, expectedBytes) {
			return false, fmt.Errorf("pending member execution handoff differs from canonical reviewed dispatch")
		}
	}
	if err := anchor.revalidate(); err != nil {
		return false, err
	}
	return true, nil
}

func MarshalResultManifest(manifest ResultManifest) ([]byte, error) {
	return canonical(manifest)
}

func IsPendingDispatch(err error) bool {
	return errors.Is(err, errPendingDispatch)
}

func nextAttemptSequence(caseRoot string, owner Owner, requestSHA string) (int, error) {
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return 0, err
	}
	defer anchor.Close()
	rootRel := anchor.stateRel("lanes", owner.Lane, "member-executions")
	entries, readErr := anchor.readDir(rootRel)
	if os.IsNotExist(readErr) {
		return 1, nil
	}
	if readErr != nil {
		return 0, readErr
	}
	maxSequence := 0
	latestName := ""
	prefix := fmt.Sprintf("g%06d-a", owner.ExecutorGeneration)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() || !segment.MatchString(entry.Name()) {
			return 0, fmt.Errorf("member execution root contains invalid attempt entry: %s", entry.Name())
		}
		parts := strings.Split(entry.Name(), "-")
		if !strings.HasPrefix(entry.Name(), prefix) || len(parts) < 3 {
			continue
		}
		sequence, parseErr := strconv.Atoi(strings.TrimPrefix(parts[1], "a"))
		if parseErr != nil || sequence < 1 {
			return 0, fmt.Errorf("invalid member execution attempt sequence: %s", entry.Name())
		}
		if sequence > maxSequence {
			maxSequence = sequence
			latestName = entry.Name()
		}
	}
	if latestName == "" {
		return 1, nil
	}
	inspection, inspectErr := inspectAnchored(anchor, owner.Lane, latestName)
	if inspectErr == nil {
		if inspection.Owner == owner && inspection.Intent != nil && strings.EqualFold(inspection.Intent.RequestSHA256, requestSHA) && inspection.Latest == nil {
			return maxSequence, nil
		}
		return maxSequence + 1, nil
	}
	intentData, intentErr := anchor.readFile(filepath.Join(rootRel, latestName, "intent.json"), maxJSONBytes)
	if intentErr != nil {
		return 0, fmt.Errorf("member execution attempt is missing durable intent: %s: %w", latestName, intentErr)
	}
	var intent Intent
	if err := strictCanonical(intentData, &intent); err != nil || intent.SchemaVersion != SchemaVersion || intent.Kind != KindIntent || intent.AttemptID != latestName || !samePath(intent.CaseRoot, anchor.path) || intent.Owner != owner || !strings.EqualFold(intent.RequestSHA256, requestSHA) || !intent.NoSpawn || !intent.NoPoll || !intent.NoStop || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool {
		return 0, fmt.Errorf("pending member execution attempt does not match reviewed dispatch identity: %s", latestName)
	}
	if err := anchor.revalidate(); err != nil {
		return 0, err
	}
	return maxSequence, nil
}

func attemptRoot(caseRoot, lane, attempt string) (string, error) {
	if !segment.MatchString(lane) || !segment.MatchString(attempt) {
		return "", fmt.Errorf("invalid member execution path segment")
	}
	root, err := projectstate.Join(caseRoot, "lanes", lane, "member-executions", attempt)
	if err != nil {
		return "", err
	}
	if !contained(caseRoot, root) {
		return "", fmt.Errorf("member execution root escapes case")
	}
	return root, nil
}

func finishPlan(plan Plan) (Plan, error) {
	value := struct {
		SchemaVersion                   int `json:"schemaVersion"`
		Mode, CaseRoot, Pack, AttemptID string
		Owner                           Owner
		Outcome, Actor, Reason          string
		Inspection                      Inspection
	}{plan.SchemaVersion, plan.Mode, plan.CaseRoot, plan.Pack, plan.AttemptID, plan.Owner, plan.Outcome, plan.Actor, plan.Reason, plan.Inspection}
	data, err := json.Marshal(value)
	if err != nil {
		return Plan{}, err
	}
	plan.ExpectedPlanSHA256 = hash(data)
	return plan, nil
}

func strictCanonical(data []byte, out any) error {
	if err := decodeStrict(data, out); err != nil {
		return err
	}
	expected, err := canonical(out)
	if err != nil || !bytes.Equal(data, expected) {
		return fmt.Errorf("member execution JSON is not canonical")
	}
	return nil
}
func decodeStrict(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("member execution JSON contains trailing data")
	}
	return nil
}
func canonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
func hash(data []byte) string            { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func hashMustCanonical(value any) string { data, _ := canonical(value); return hash(data) }
func validSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func parseTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid member execution time: %w", err)
	}
	return parsed.UTC(), nil
}
func validRelative(value string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	return value != "" && clean == value && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && !filepath.IsAbs(filepath.FromSlash(value))
}
func contained(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func samePath(left, right string) bool {
	left, _ = filepath.Abs(left)
	right, _ = filepath.Abs(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
func rel(root, path string) string {
	value, _ := filepath.Rel(root, path)
	return filepath.ToSlash(value)
}
func boundaries() []string {
	return []string{"external harness owns member session lifecycle; Go does not spawn, poll, stop, or manage sessions", "member execution does not execute heavy tools and does not write authority or confirmed state", "owner executor generation and exact output hashes are required for intake"}
}
