package subagents

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/overview"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	maxReviewerResultBytes = 64 * 1024
	maxReviewPacketBytes   = 1024 * 1024
)

var appendReviewerNote = note.Append

type ReviewerResult struct {
	PacketID           string         `json:"packetId"`
	RouteID            string         `json:"routeId"`
	ShardID            string         `json:"shardId"`
	Items              []string       `json:"items"`
	ReviewerSession    string         `json:"reviewerSession"`
	Decision           string         `json:"decision"`
	Confidence         string         `json:"confidence"`
	Summary            string         `json:"summary"`
	EvidenceRefs       []string       `json:"evidenceRefs"`
	Risks              []string       `json:"risks"`
	Conflicts          []string       `json:"conflicts"`
	RecommendedVerdict string         `json:"recommendedVerdict"`
	RouteOutput        map[string]any `json:"routeOutput,omitempty"`
}

type ReviewerIntakeOptions struct {
	PacketPath         string
	ReviewerResultPath string
	Lane               string
	Actor              string
	WhatIf             bool
}

type ReviewerIntakeResult struct {
	SchemaVersion         int                         `json:"schemaVersion"`
	Command               string                      `json:"command"`
	Mode                  string                      `json:"mode"`
	CaseRoot              string                      `json:"caseRoot"`
	RepoRoot              string                      `json:"repoRoot"`
	Pack                  string                      `json:"pack"`
	IsMutation            bool                        `json:"isMutation"`
	Applied               bool                        `json:"applied"`
	WritebackStatus       string                      `json:"writebackStatus"`
	ReadyForWriteback     bool                        `json:"readyForWriteback"`
	ReviewRequired        bool                        `json:"reviewRequired"`
	PacketPath            string                      `json:"packetPath"`
	ReviewerResultPath    string                      `json:"reviewerResultPath"`
	IntakeID              string                      `json:"intakeId"`
	Lane                  string                      `json:"lane"`
	ShardID               string                      `json:"shardId"`
	OwnerBinding          OwnerBinding                `json:"ownerBinding"`
	ReviewerSession       string                      `json:"reviewerSession"`
	ReviewerResult        ReviewerResult              `json:"reviewerResult"`
	VerificationVerdict   string                      `json:"verificationVerdict"`
	MainDecision          string                      `json:"mainDecision"`
	BlockedReasons        []string                    `json:"blockedReasons"`
	OrchestrationSnapshot ReviewerOrchestrationIntake `json:"orchestrationSnapshot"`
	Verification          *note.AppendResult          `json:"verification,omitempty"`
	Decision              *note.AppendResult          `json:"decision,omitempty"`
	PostValidation        *ReviewerPostValidation     `json:"postValidation,omitempty"`
	NextSteps             []string                    `json:"nextSteps"`
}

type ReviewerPostValidation struct {
	Overview   overview.Inventory       `json:"overview"`
	Handoff    workstream.HandoffResult `json:"handoff"`
	DoctorRows []doctor.Row             `json:"doctorRows"`
	Valid      bool                     `json:"valid"`
}

type ReviewerOrchestrationIntake struct {
	Mode                 string   `json:"mode"`
	PacketPath           string   `json:"packetPath"`
	ResultRoot           string   `json:"resultRoot"`
	ReviewerCount        int      `json:"reviewerCount"`
	DispatchIndex        int      `json:"dispatchIndex"`
	DispatchTotal        int      `json:"dispatchTotal"`
	ShardStatusBefore    string   `json:"shardStatusBefore"`
	ShardStatusAfter     string   `json:"shardStatusAfter"`
	PreviewRequiredFirst bool     `json:"previewRequiredFirst"`
	MainAgentOwns        []string `json:"mainAgentOwns"`
	RuntimeBoundary      []string `json:"runtimeBoundary"`
	NextDispatches       []string `json:"nextDispatches"`
}

func IntakeReviewerResult(repoRoot, caseRoot, pack string, opt ReviewerIntakeOptions) (ReviewerIntakeResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	caseRoot = inst.CaseRoot
	packetPath, err := requiredAbsolutePath(opt.PacketPath, "review packet")
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	resultPath, err := requiredAbsolutePath(opt.ReviewerResultPath, "reviewer result")
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	if lane == "" {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer intake requires -Lane <lane id>")
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer intake requires -Actor <main-agent actor>")
	}

	packetBytes, err := readBoundedFile(packetPath, "review packet", maxReviewPacketBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	packet, err := decodeIntakePacket(packetBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validateIntakePacketRoute(repoRoot, pack, packet); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if lane != packet.TargetLane {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer intake lane %q does not match packet targetLane %q", lane, packet.TargetLane)
	}
	if err := validateOwnerBinding(caseRoot, packet.OwnerBinding); err != nil {
		return ReviewerIntakeResult{}, err
	}
	resultBytes, err := readBoundedFile(resultPath, "reviewer result", maxReviewerResultBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	reviewerResult, err := decodeReviewerResult(resultBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if reviewerResult.PacketID != packet.PacketID {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result packetId %q does not match packet %q", reviewerResult.PacketID, packet.PacketID)
	}
	if reviewerResult.RouteID != packet.Route.ID {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result routeId %q does not match packet route %q", reviewerResult.RouteID, packet.Route.ID)
	}
	if err := validateRouteOutput(packet.OutputContract, reviewerResult.RouteOutput); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validateRouteOutputBindings(reviewerResult); err != nil {
		return ReviewerIntakeResult{}, err
	}
	shard, ok := shardByID(packet.Shards, reviewerResult.ShardID)
	if !ok {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result shard %q is not present in packet", reviewerResult.ShardID)
	}
	if !slices.Equal(reviewerResult.Items, shard.Items) {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result items do not match packet shard %s: got %v, want %v", shard.ID, reviewerResult.Items, shard.Items)
	}
	orchestrationSnapshot := reviewerOrchestrationIntake(caseRoot, packet, reviewerResult.ShardID, lane, opt.WhatIf)
	mapping, ok := reviewerDecisionMappingByDecision(reviewerResult.Decision)
	if !ok {
		return ReviewerIntakeResult{}, fmt.Errorf("invalid reviewer decision %q; allowed: %s", reviewerResult.Decision, strings.Join(reviewerResultContract().AllowedDecisions, ","))
	}

	intakeID := reviewerIntakeID(packet, reviewerResult, lane)
	blocked, err := reviewerIntakeBlockers(repoRoot, caseRoot, packet, reviewerResult, mapping)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	priorBlocked, err := existingReviewerWritebackBlockers(caseRoot, packet, reviewerResult, lane, intakeID)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	blocked = append(blocked, priorBlocked...)
	result := ReviewerIntakeResult{
		SchemaVersion:         1,
		Command:               commandName,
		Mode:                  "reviewer-intake",
		CaseRoot:              caseRoot,
		RepoRoot:              repoRoot,
		Pack:                  pack,
		IsMutation:            !opt.WhatIf,
		Applied:               false,
		WritebackStatus:       "validated",
		ReadyForWriteback:     len(blocked) == 0,
		ReviewRequired:        true,
		PacketPath:            packetPath,
		ReviewerResultPath:    resultPath,
		IntakeID:              intakeID,
		Lane:                  lane,
		ShardID:               shard.ID,
		OwnerBinding:          packet.OwnerBinding,
		ReviewerSession:       reviewerResult.ReviewerSession,
		ReviewerResult:        reviewerResult,
		VerificationVerdict:   mapping.VerificationVerdict,
		MainDecision:          mapping.MainDecision,
		BlockedReasons:        blocked,
		OrchestrationSnapshot: orchestrationSnapshot,
	}
	if len(blocked) > 0 {
		result.IsMutation = false
		result.WritebackStatus = "blocked"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.NextSteps = []string{"resolve reviewer intake blockers or dispatch a smaller read-only shard; no ledger events were written"}
		validation, validationErr := reviewerPostValidation(repoRoot, caseRoot, pack, lane, false)
		if validationErr != nil {
			return ReviewerIntakeResult{}, validationErr
		}
		result.PostValidation = &validation
		return result, nil
	}

	verificationOpt, decisionOpt := reviewerNoteOptions(packet, reviewerResult, mapping, lane, actor, intakeID, packetPath, resultPath)
	verificationPreview, err := appendReviewerNote(repoRoot, caseRoot, pack, verificationOpt, true)
	if err != nil {
		return ReviewerIntakeResult{}, fmt.Errorf("preview reviewer verification note: %w", err)
	}
	decisionPreview, err := appendReviewerNote(repoRoot, caseRoot, pack, decisionOpt, true)
	if err != nil {
		return ReviewerIntakeResult{}, fmt.Errorf("preview main decision note: %w", err)
	}
	result.Verification = &verificationPreview
	result.Decision = &decisionPreview
	if err := ensureExpectedAppendResult(caseRoot, "verification", verificationPreview); err != nil {
		result.WritebackStatus = "event-id-collision"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.ReadyForWriteback = false
		result.BlockedReasons = append(result.BlockedReasons, err.Error())
		return result, err
	}
	if err := ensureExpectedAppendResult(caseRoot, "decision", decisionPreview); err != nil {
		result.WritebackStatus = "event-id-collision"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.ReadyForWriteback = false
		result.BlockedReasons = append(result.BlockedReasons, err.Error())
		return result, err
	}
	if opt.WhatIf {
		result.IsMutation = false
		result.WritebackStatus = "previewed"
		result.NextSteps = []string{"inspect verification and decision WhatIf events, then rerun the same reviewer intake with -Apply to append both events"}
		if isDuplicateAppend(verificationPreview) && isDuplicateAppend(decisionPreview) {
			result.WritebackStatus = "already-complete"
			result.NextSteps = []string{"reviewer intake writeback is already complete; consume postValidation before handing off or continuing the lane"}
		}
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		validation, err := reviewerPostValidation(repoRoot, caseRoot, pack, lane, false)
		if err != nil {
			return ReviewerIntakeResult{}, err
		}
		result.PostValidation = &validation
		return result, nil
	}

	unlock, err := acquireReviewerIntakeLock(caseRoot, reviewerShardLockID(packet, reviewerResult, lane))
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	defer unlock()
	priorBlocked, err = existingReviewerWritebackBlockers(caseRoot, packet, reviewerResult, lane, intakeID)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if len(priorBlocked) > 0 {
		result.IsMutation = false
		result.ReadyForWriteback = false
		result.WritebackStatus = "blocked"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.BlockedReasons = append(result.BlockedReasons, priorBlocked...)
		result.NextSteps = []string{"resolve existing reviewer writeback conflict before applying a new reviewer result for this shard"}
		validation, validationErr := reviewerPostValidation(repoRoot, caseRoot, pack, lane, false)
		if validationErr != nil {
			return ReviewerIntakeResult{}, validationErr
		}
		result.PostValidation = &validation
		return result, nil
	}

	verificationApply, err := appendReviewerNote(repoRoot, caseRoot, pack, verificationOpt, false)
	result.Verification = &verificationApply
	if err != nil {
		if verificationApply.EventID != "" {
			result.Applied = verificationApply.Applied
			result.WritebackStatus = "verification-recorded"
			result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
			result.NextSteps = []string{"retry the identical reviewer intake apply; deterministic event IDs will preserve the verification event and complete the missing decision idempotently"}
			return result, fmt.Errorf("append reviewer verification note after event %s; writebackStatus=%s and retry is safe: %w", verificationApply.EventID, result.WritebackStatus, err)
		}
		return ReviewerIntakeResult{}, fmt.Errorf("append reviewer verification note: %w", err)
	}
	if err := ensureExpectedAppendResult(caseRoot, "verification", verificationApply); err != nil {
		result.WritebackStatus = "event-id-collision"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.ReadyForWriteback = false
		result.BlockedReasons = append(result.BlockedReasons, err.Error())
		return result, err
	}
	result.Applied = verificationApply.Applied
	result.WritebackStatus = "verification-recorded"
	result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
	decisionApply, err := appendReviewerNote(repoRoot, caseRoot, pack, decisionOpt, false)
	result.Decision = &decisionApply
	if err != nil {
		result.NextSteps = []string{"retry the identical reviewer intake apply; deterministic event IDs will preserve the verification event and complete the missing decision idempotently"}
		return result, fmt.Errorf("append main decision note after verification event %s; writebackStatus=%s and retry is safe: %w", verificationApply.EventID, result.WritebackStatus, err)
	}
	if err := ensureExpectedAppendResult(caseRoot, "decision", decisionApply); err != nil {
		result.WritebackStatus = "event-id-collision"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.ReadyForWriteback = false
		result.BlockedReasons = append(result.BlockedReasons, err.Error())
		return result, err
	}
	result.Applied = verificationApply.Applied || decisionApply.Applied
	result.WritebackStatus = "complete"
	if !verificationApply.Applied && !decisionApply.Applied {
		result.WritebackStatus = "already-complete"
	}
	result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
	validation, err := reviewerPostValidation(repoRoot, caseRoot, pack, lane, true)
	if err != nil {
		result.WritebackStatus = "complete-post-validation-failed"
		result.NextSteps = []string{"ledger writeback completed; rerun the identical reviewer intake apply or run overview/handoff/doctor to recover post-validation snapshots"}
		return result, fmt.Errorf("reviewer ledger writeback completed but post-validation failed; writebackStatus=%s: %w", result.WritebackStatus, err)
	}
	result.PostValidation = &validation
	result.NextSteps = []string{"consume postValidation overview/handoff/doctor snapshots", "continue or hand off the lane only when postValidation shows the expected blocker state"}
	return result, nil
}

func readBoundedFile(path, label string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d-byte limit", label, limit)
	}
	return data, nil
}

func decodeIntakePacket(data []byte) (Packet, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var packet Packet
	if err := dec.Decode(&packet); err != nil {
		return Packet{}, fmt.Errorf("decode review packet: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Packet{}, fmt.Errorf("review packet must contain exactly one JSON object")
	}
	return packet, nil
}

func decodeReviewerResult(data []byte) (ReviewerResult, error) {
	if len(data) > maxReviewerResultBytes {
		return ReviewerResult{}, fmt.Errorf("reviewer result exceeds %d-byte limit", maxReviewerResultBytes)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ReviewerResult{}, fmt.Errorf("reviewer result is empty")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return ReviewerResult{}, fmt.Errorf("reviewer result must contain exactly one JSON object: %w", err)
	}
	for _, field := range reviewerResultContract().RequiredFields {
		if _, ok := fields[field]; !ok {
			return ReviewerResult{}, fmt.Errorf("reviewer result missing required field %q", field)
		}
	}
	for _, field := range []string{"items", "evidenceRefs", "risks", "conflicts"} {
		if err := requireJSONArrayField(fields, field); err != nil {
			return ReviewerResult{}, err
		}
	}
	if err := requireJSONObjectField(fields, "routeOutput"); err != nil {
		return ReviewerResult{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var result ReviewerResult
	if err := dec.Decode(&result); err != nil {
		return ReviewerResult{}, fmt.Errorf("reviewer result contract validation failed: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return ReviewerResult{}, fmt.Errorf("reviewer result must contain exactly one JSON object")
	}
	result.PacketID = strings.TrimSpace(result.PacketID)
	result.RouteID = strings.TrimSpace(result.RouteID)
	result.ShardID = strings.TrimSpace(result.ShardID)
	result.ReviewerSession = strings.TrimSpace(result.ReviewerSession)
	result.Decision = strings.TrimSpace(result.Decision)
	result.Confidence = strings.TrimSpace(result.Confidence)
	result.Summary = strings.TrimSpace(result.Summary)
	result.RecommendedVerdict = strings.TrimSpace(result.RecommendedVerdict)
	result.Items = cleanStrings(result.Items)
	result.EvidenceRefs = cleanStrings(result.EvidenceRefs)
	result.Risks = cleanStrings(result.Risks)
	result.Conflicts = cleanStrings(result.Conflicts)
	if result.PacketID == "" || result.RouteID == "" || result.ShardID == "" || result.ReviewerSession == "" || result.Summary == "" {
		return ReviewerResult{}, fmt.Errorf("reviewer result packetId, routeId, shardId, reviewerSession, and summary must be non-empty")
	}
	if strings.ContainsAny(result.ReviewerSession, "\r\n") {
		return ReviewerResult{}, fmt.Errorf("reviewer result reviewerSession must be a single-line session identifier")
	}
	if result.RouteOutput == nil {
		return ReviewerResult{}, fmt.Errorf("reviewer result routeOutput must be an object, even when no route-specific values are needed")
	}
	if !slices.Contains([]string{"low", "medium", "high"}, result.Confidence) {
		return ReviewerResult{}, fmt.Errorf("invalid reviewer confidence %q; allowed: low,medium,high", result.Confidence)
	}
	return result, nil
}

func requireJSONArrayField(fields map[string]json.RawMessage, field string) error {
	raw := bytes.TrimSpace(fields[field])
	if bytes.Equal(raw, []byte("null")) {
		return fmt.Errorf("reviewer result field %q must be an array, not null", field)
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("reviewer result field %q must be an array", field)
	}
	return nil
}

func requireJSONObjectField(fields map[string]json.RawMessage, field string) error {
	raw := bytes.TrimSpace(fields[field])
	if bytes.Equal(raw, []byte("null")) {
		return fmt.Errorf("reviewer result field %q must be an object, not null", field)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("reviewer result field %q must be an object", field)
	}
	return nil
}

func validateRouteOutput(outputContract string, routeOutput map[string]any) error {
	allowed := map[string]bool{}
	for _, field := range splitCSV(outputContract) {
		allowed[field] = true
		value, ok := routeOutput[field]
		if !ok || value == nil {
			return fmt.Errorf("reviewer result routeOutput missing required outputContract field %q", field)
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("reviewer result routeOutput field %q must be a non-empty string", field)
		}
	}
	for field := range routeOutput {
		if !allowed[field] {
			return fmt.Errorf("reviewer result routeOutput contains unknown field %q", field)
		}
	}
	return nil
}

func validateRouteOutputBindings(result ReviewerResult) error {
	for _, binding := range []struct {
		field string
		want  string
	}{
		{field: "decision", want: result.Decision},
		{field: "confidence", want: result.Confidence},
	} {
		value, ok := result.RouteOutput[binding.field]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("reviewer result routeOutput field %q must be a string", binding.field)
		}
		if !strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(binding.want)) {
			return fmt.Errorf("reviewer result routeOutput field %q %q does not match top-level value %q", binding.field, text, binding.want)
		}
	}
	if item := routeOutputString(result.RouteOutput, "item"); item != "" {
		for _, itemValue := range splitCSV(item) {
			if !slices.Contains(result.Items, itemValue) {
				return fmt.Errorf("reviewer result routeOutput item %q is outside top-level items %v", itemValue, result.Items)
			}
		}
	}
	if evidence := routeOutputString(result.RouteOutput, "evidence"); evidence != "" {
		for _, ref := range splitCSV(evidence) {
			if !slices.Contains(result.EvidenceRefs, ref) {
				return fmt.Errorf("reviewer result routeOutput evidence %q is not present in top-level evidenceRefs", ref)
			}
		}
	}
	return nil
}

func validateOwnerBinding(caseRoot string, binding OwnerBinding) error {
	if strings.TrimSpace(binding.TargetLane) == "" {
		return fmt.Errorf("review packet ownerBinding.targetLane is required for reviewer intake")
	}
	if !binding.RequiredForIntake {
		return nil
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return fmt.Errorf("read board for reviewer owner binding validation: %w", err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, binding.TargetLane, false)
	if !ok {
		return fmt.Errorf("review packet ownerBinding target lane %q is not present in current board", binding.TargetLane)
	}
	currentExecutor := strings.TrimSpace(lane.CurrentExecutor)
	if currentExecutor == "" || binding.CurrentExecutor == "" || currentExecutor != strings.TrimSpace(binding.CurrentExecutor) || lane.ExecutorGeneration != binding.ExecutorGeneration {
		return fmt.Errorf("review packet ownerBinding is stale for lane %s: packet executor=%s generation=%d current executor=%s generation=%d", binding.TargetLane, textOr(binding.CurrentExecutor, "unassigned"), binding.ExecutorGeneration, textOr(currentExecutor, "unassigned"), lane.ExecutorGeneration)
	}
	return nil
}

func validateIntakePacket(packet Packet, packetPath, repoRoot, caseRoot, pack string) error {
	if packet.SchemaVersion != 1 || !packetIdentityMatches(packet) || packet.Command != commandName || !packet.WritesReviewArtifacts || packet.IsMutation || !packet.ReviewRequired {
		return fmt.Errorf("review packet is not a supported non-mutating %s packet", commandName)
	}
	if !samePath(packet.RepoRoot, repoRoot) {
		return fmt.Errorf("review packet repoRoot %q does not match current repo %q", packet.RepoRoot, repoRoot)
	}
	if strings.TrimSpace(packet.PlanRoot) == "" || !samePath(packet.PlanRoot, caseRoot) {
		return fmt.Errorf("review packet planRoot %q does not match current case %q", packet.PlanRoot, caseRoot)
	}
	if strings.TrimSpace(packet.TargetLane) == "" {
		return fmt.Errorf("review packet targetLane is required for reviewer intake")
	}
	if strings.TrimSpace(packet.OwnerBinding.TargetLane) == "" || packet.OwnerBinding.TargetLane != packet.TargetLane {
		return fmt.Errorf("review packet ownerBinding.targetLane %q does not match targetLane %q", packet.OwnerBinding.TargetLane, packet.TargetLane)
	}
	if !strings.EqualFold(strings.TrimSpace(packet.Pack), strings.TrimSpace(pack)) {
		return fmt.Errorf("review packet pack %q does not match current pack %q", packet.Pack, pack)
	}
	if strings.TrimSpace(packet.Observability.PacketPath) == "" || !samePath(packet.Observability.PacketPath, packetPath) {
		return fmt.Errorf("review packet observability packetPath %q does not match current packet path %q", packet.Observability.PacketPath, packetPath)
	}
	return nil
}

func validateIntakePacketRoute(repoRoot, pack string, packet Packet) error {
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return err
	}
	if !samePath(packet.ManifestPath, m.ManifestPath) {
		return fmt.Errorf("review packet manifestPath %q does not match current pack manifest %q", packet.ManifestPath, m.ManifestPath)
	}
	for _, route := range m.SubagentRoutes {
		if !strings.EqualFold(strings.TrimSpace(route.ID), strings.TrimSpace(packet.Route.ID)) {
			continue
		}
		if packet.Route != toRoute(route) {
			return fmt.Errorf("review packet route %q does not match the current pack manifest route definition", packet.Route.ID)
		}
		if packet.MainAgentResponsibilities != packet.Route.MainAgentOwns || packet.SubagentPermissions != packet.Route.SubagentPermissions || packet.OutputContract != packet.Route.OutputContract {
			return fmt.Errorf("review packet top-level route contract does not match route %q", packet.Route.ID)
		}
		return nil
	}
	return fmt.Errorf("review packet route %q is not present in the current pack manifest", packet.Route.ID)
}

func reviewerIntakeBlockers(repoRoot, caseRoot string, packet Packet, result ReviewerResult, mapping ReviewerDecisionMapping) ([]string, error) {
	blocked := []string{}
	if len(result.EvidenceRefs) == 0 {
		blocked = append(blocked, "reviewer result has no inspectable evidenceRefs")
	} else {
		invalidRefs, err := invalidReviewerEvidenceRefs(caseRoot, packet, result.EvidenceRefs)
		if err != nil {
			return nil, err
		}
		for _, ref := range invalidRefs {
			blocked = append(blocked, fmt.Sprintf("reviewer result evidenceRef %q is not the packet id or a non-empty case-local bounded evidence file", ref))
		}
	}
	if len(result.Conflicts) > 0 {
		blocked = append(blocked, "reviewer result reports unresolved conflicts: "+strings.Join(result.Conflicts, "; "))
	}
	if reviewerResultRequestsBlockedAction(repoRoot, packet, result) {
		blocked = append(blocked, "reviewer result requests a blocked write, heavy-tool, authority/confirmed, or external-effect action")
	}
	if result.RecommendedVerdict != mapping.VerificationVerdict {
		blocked = append(blocked, fmt.Sprintf("recommendedVerdict %q conflicts with mapped verification verdict %q", result.RecommendedVerdict, mapping.VerificationVerdict))
	}
	if result.Confidence == "low" && (result.Decision == "accept" || result.Decision == "reject") {
		blocked = append(blocked, "low-confidence accept/reject cannot be written back without independent evidence review")
	}
	return blocked, nil
}

func invalidReviewerEvidenceRefs(caseRoot string, packet Packet, refs []string) ([]string, error) {
	invalid := []string{}
	caseReal, err := filepath.EvalSymlinks(caseRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve case root for reviewer evidence: %w", err)
	}
	for _, ref := range refs {
		value := strings.TrimSpace(ref)
		if value == packet.PacketID {
			continue
		}
		if strings.ContainsAny(value, ",;\r\n") {
			invalid = append(invalid, value)
			continue
		}
		pathPart := strings.TrimSpace(strings.SplitN(value, "#", 2)[0])
		if pathPart == "" || filepath.IsAbs(pathPart) {
			invalid = append(invalid, value)
			continue
		}
		path, joinErr := refsf.SafeJoin(caseRoot, pathPart)
		if joinErr != nil {
			invalid = append(invalid, value)
			continue
		}
		pathReal, evalErr := filepath.EvalSymlinks(path)
		if evalErr != nil || !pathInside(caseReal, pathReal) {
			invalid = append(invalid, value)
			continue
		}
		st, statErr := os.Stat(pathReal)
		if statErr != nil || st.IsDir() || st.Size() == 0 || evidenceFileBlank(pathReal, st.Size()) {
			invalid = append(invalid, value)
		}
	}
	return invalid, nil
}

func evidenceFileBlank(path string, size int64) bool {
	if size > 4096 {
		return false
	}
	data, err := os.ReadFile(path)
	return err != nil || len(bytes.TrimSpace(data)) == 0
}

func reviewerResultRequestsBlockedAction(repoRoot string, packet Packet, result ReviewerResult) bool {
	if scope := strings.ToLower(strings.TrimSpace(routeOutputString(result.RouteOutput, "tool_scope"))); scope != "" && !isNA(scope) && scope != "read-only" {
		return true
	}
	blockedTerms := []string{"write file", "append ledger", "authority", "confirmed", "external effect", "external-effect", "run debugger", "dump memory", "patch binary", "write", "create", "save", "modify", "delete", "curl", "http", "network", "gdb", "debugger", "trace", "dump", "patch", "hook", "inject", "execute"}
	if m, err := manifest.Load(repoRoot, packet.Pack); err == nil {
		blockedTerms = append(blockedTerms, m.HeavyToolGateIDs()...)
	} else {
		return true
	}
	for _, key := range []string{"next_action", "tool_scope"} {
		text := strings.ToLower(routeOutputString(result.RouteOutput, key))
		for _, term := range blockedTerms {
			if containsActionTerm(text, term) {
				return true
			}
		}
	}
	return false
}

func routeOutputString(routeOutput map[string]any, key string) string {
	value, _ := routeOutput[key].(string)
	return strings.TrimSpace(value)
}

func containsActionTerm(text, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	normalizedText := normalizeActionText(text)
	normalizedTerm := normalizeActionText(term)
	if normalizedTerm == "" {
		return false
	}
	if strings.Contains(normalizedTerm, " ") {
		return strings.Contains(normalizedText, normalizedTerm)
	}
	return slices.Contains(strings.Fields(normalizedText), normalizedTerm)
}

func normalizeActionText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}), " ")
}

func reviewerNoteOptions(packet Packet, result ReviewerResult, mapping ReviewerDecisionMapping, lane, actor, intakeID, packetPath, resultPath string) (note.Options, note.Options) {
	target := strings.Join(result.Items, ",")
	evidence := strings.Join(result.EvidenceRefs, ",")
	verificationID := "evt-" + intakeID + "-verification"
	ownerGeneration := ""
	if packet.OwnerBinding.ExecutorGeneration > 0 {
		ownerGeneration = strconv.Itoa(packet.OwnerBinding.ExecutorGeneration)
	}
	verification := note.Options{
		Kind:               "verification",
		Lane:               lane,
		Subject:            "reviewer verdict for " + result.ShardID,
		Summary:            result.Summary,
		Actor:              actor,
		Confidence:         result.Confidence,
		BatchID:            intakeID,
		Target:             target,
		Verifier:           "manual-review",
		Verdict:            mapping.VerificationVerdict,
		EvidenceRefs:       evidence,
		EventID:            verificationID,
		PacketID:           packet.PacketID,
		RouteID:            packet.Route.ID,
		ShardID:            result.ShardID,
		PacketPath:         packetPath,
		ReviewerResultPath: resultPath,
		ReviewerSession:    result.ReviewerSession,
		OwnerExecutor:      packet.OwnerBinding.CurrentExecutor,
		OwnerGeneration:    ownerGeneration,
		OwnerBindingMode:   packet.OwnerBinding.BindingMode,
		OwnerBindingTarget: packet.OwnerBinding.TargetLane,
	}
	decision := note.Options{
		Kind:               "decision",
		Lane:               lane,
		Subject:            "main merge decision for " + result.ShardID,
		Summary:            "mapped reviewer decision " + result.Decision + ": " + result.Summary,
		Actor:              actor,
		Confidence:         result.Confidence,
		Decision:           mapping.MainDecision,
		Reason:             "validated bounded reviewer intake " + intakeID,
		BatchID:            intakeID,
		Target:             target,
		EvidenceRefs:       verificationID + "," + evidence,
		EventID:            "evt-" + intakeID + "-decision",
		PacketID:           packet.PacketID,
		RouteID:            packet.Route.ID,
		ShardID:            result.ShardID,
		PacketPath:         packetPath,
		ReviewerResultPath: resultPath,
		ReviewerSession:    result.ReviewerSession,
		OwnerExecutor:      packet.OwnerBinding.CurrentExecutor,
		OwnerGeneration:    ownerGeneration,
		OwnerBindingMode:   packet.OwnerBinding.BindingMode,
		OwnerBindingTarget: packet.OwnerBinding.TargetLane,
	}
	return verification, decision
}

func reviewerPostValidation(repoRoot, caseRoot, pack, lane string, allowInit bool) (ReviewerPostValidation, error) {
	if !allowInit {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "board.json")); err != nil {
			if os.IsNotExist(err) {
				return ReviewerPostValidation{Valid: false}, nil
			}
			return ReviewerPostValidation{}, err
		}
	}
	overviewResult, err := overview.BuildInventory(repoRoot, caseRoot, pack)
	if err != nil {
		return ReviewerPostValidation{}, fmt.Errorf("post-review overview validation: %w", err)
	}
	handoffResult, err := workstream.HandoffPreview(repoRoot, caseRoot, pack, workstream.HandoffOptions{Selector: lane})
	if err != nil {
		return ReviewerPostValidation{}, fmt.Errorf("post-review handoff validation: %w", err)
	}
	doctorRows, err := doctor.Case(repoRoot, caseRoot, pack)
	if err != nil {
		return ReviewerPostValidation{}, fmt.Errorf("post-review doctor validation: %w", err)
	}
	return ReviewerPostValidation{Overview: overviewResult, Handoff: handoffResult, DoctorRows: doctorRows, Valid: true}, nil
}

func reviewerDecisionMappingByDecision(decision string) (ReviewerDecisionMapping, bool) {
	for _, mapping := range reviewerDecisionMappings() {
		if mapping.ReviewerDecision == strings.TrimSpace(decision) {
			return mapping, true
		}
	}
	return ReviewerDecisionMapping{}, false
}

func reviewerOrchestrationIntake(caseRoot string, packet Packet, shardID, lane string, whatIf bool) ReviewerOrchestrationIntake {
	completed := reviewerCompletedShards(caseRoot, packet, lane)
	normalizedShard := strings.TrimSpace(shardID)
	before := "planned"
	if completed[normalizedShard] {
		before = "already-complete"
	}
	after := "previewed"
	if !whatIf {
		after = "writeback-attempted"
	}
	snapshot := ReviewerOrchestrationIntake{
		Mode:                 packet.ReviewerOrchestration.Mode,
		PacketPath:           packet.Observability.PacketPath,
		ResultRoot:           packet.Observability.ResultRoot,
		ReviewerCount:        len(packet.ReviewerOrchestration.Dispatches),
		DispatchTotal:        len(packet.ReviewerOrchestration.Dispatches),
		ShardStatusBefore:    before,
		ShardStatusAfter:     after,
		PreviewRequiredFirst: true,
		MainAgentOwns:        append([]string{}, packet.ReviewLoop.MainAgentOwns...),
		RuntimeBoundary:      append([]string{}, packet.Observability.BlockedActions...),
	}
	if len(packet.ReviewerOrchestration.Dispatches) > 0 {
		for idx, dispatch := range packet.ReviewerOrchestration.Dispatches {
			if dispatch.ShardID == normalizedShard {
				snapshot.DispatchIndex = idx + 1
				continue
			}
			if dispatch.Status == "planned" && !completed[dispatch.ShardID] {
				snapshot.NextDispatches = append(snapshot.NextDispatches, dispatch.ShardID)
			}
		}
	} else {
		for idx, shard := range packet.Shards {
			if shard.ID == normalizedShard {
				snapshot.DispatchIndex = idx + 1
				continue
			}
			if !completed[shard.ID] {
				snapshot.NextDispatches = append(snapshot.NextDispatches, shard.ID)
			}
		}
	}
	if snapshot.Mode == "" {
		snapshot.Mode = "manual-main-agent-intake"
	}
	if snapshot.PacketPath == "" {
		snapshot.PacketPath = packet.Observability.PacketPath
	}
	if snapshot.ResultRoot == "" {
		snapshot.ResultRoot = packet.Observability.ResultRoot
	}
	if snapshot.ReviewerCount == 0 {
		snapshot.ReviewerCount = len(packet.Shards)
		snapshot.DispatchTotal = len(packet.Shards)
	}
	return snapshot
}

func reviewerCompletedShards(caseRoot string, packet Packet, lane string) map[string]bool {
	verificationByShard := map[string]bool{}
	decisionByShard := map[string]bool{}
	for _, kind := range []struct {
		name string
		seen map[string]bool
	}{
		{name: "verification", seen: verificationByShard},
		{name: "decision", seen: decisionByShard},
	} {
		events, err := mission.ReadStrictFact(caseRoot, kind.name)
		if err != nil {
			continue
		}
		for _, event := range events {
			if mission.Value(event, "packetId") != packet.PacketID || mission.Value(event, "routeId") != packet.Route.ID || mission.Value(event, "lane") != strings.TrimSpace(lane) {
				continue
			}
			if shard := mission.Value(event, "shardId"); shard != "" {
				kind.seen[shard] = true
			}
		}
	}
	completed := map[string]bool{}
	for shard := range verificationByShard {
		if decisionByShard[shard] {
			completed[shard] = true
		}
	}
	return completed
}

func shardByID(shards []Shard, shardID string) (Shard, bool) {
	for _, shard := range shards {
		if shard.ID == strings.TrimSpace(shardID) {
			return shard, true
		}
	}
	return Shard{}, false
}

func reviewerIntakeID(packet Packet, result ReviewerResult, lane string) string {
	canonical := struct {
		PacketID string         `json:"packetId"`
		Result   ReviewerResult `json:"result"`
		Lane     string         `json:"lane"`
	}{PacketID: packet.PacketID, Result: result, Lane: strings.TrimSpace(lane)}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(encoded)
	return "review-" + hex.EncodeToString(sum[:])[:16]
}

func reviewerShardLockID(packet Packet, result ReviewerResult, lane string) string {
	seed := strings.Join([]string{packet.PacketID, packet.Route.ID, result.ShardID, strings.TrimSpace(lane)}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "reviewer-shard-" + hex.EncodeToString(sum[:])[:16]
}

func requiredAbsolutePath(path, label string) (string, error) {
	value := strings.TrimSpace(path)
	if value == "" {
		flag := "ReviewerResultPath"
		if label == "review packet" {
			flag = "PacketPath"
		}
		return "", fmt.Errorf("reviewer intake requires -%s", flag)
	}
	full, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return full, nil
}

func samePath(left, right string) bool {
	leftFull, leftErr := filepath.Abs(left)
	rightFull, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftClean := filepath.Clean(leftFull)
	rightClean := filepath.Clean(rightFull)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftClean, rightClean)
	}
	return leftClean == rightClean
}

func pathInside(root, path string) bool {
	if samePath(root, path) {
		return true
	}
	rootClean := filepath.Clean(root)
	pathClean := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		rootClean = strings.ToLower(rootClean)
		pathClean = strings.ToLower(pathClean)
	}
	rel, err := filepath.Rel(rootClean, pathClean)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func isNA(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "n/a", "na", "none", "no-op", "noop":
		return true
	default:
		return false
	}
}

func acquireReviewerIntakeLock(caseRoot, key string) (func(), error) {
	lockDir := filepath.Join(caseRoot, ".rekit", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(lockDir, key+".lock")
	deadline := time.Now().Add(10 * time.Second)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "pid=%d\n", os.Getpid())
			_ = file.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for reviewer intake lock %s", lockPath)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func existingReviewerWritebackBlockers(caseRoot string, packet Packet, result ReviewerResult, lane, intakeID string) ([]string, error) {
	blocked := []string{}
	for _, kind := range []string{"verification", "decision"} {
		events, err := mission.ReadStrictFact(caseRoot, kind)
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			if !reviewerEventMatches(event, packet, result.ShardID, lane) {
				continue
			}
			if mission.Value(event, "batchId") != intakeID {
				blocked = append(blocked, fmt.Sprintf("existing reviewer %s event %q already records packet %s shard %s for lane %s", kind, mission.Value(event, "eventId"), packet.PacketID, result.ShardID, lane))
			}
		}
	}
	return blocked, nil
}

func reviewerEventMatches(event map[string]any, packet Packet, shardID, lane string) bool {
	return mission.Value(event, "packetId") == packet.PacketID && mission.Value(event, "routeId") == packet.Route.ID && mission.Value(event, "shardId") == shardID && mission.Value(event, "lane") == lane
}

func isDuplicateAppend(result note.AppendResult) bool {
	return result.Reason == "duplicate eventId"
}

func ensureExpectedAppendResult(caseRoot, kind string, result note.AppendResult) error {
	if !isDuplicateAppend(result) {
		return nil
	}
	event, ok, err := existingEventByID(caseRoot, kind, result.EventID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("duplicate reviewer %s eventId %q exists outside expected %s ledger", kind, result.EventID, kind)
	}
	if diff := eventMismatch(result.Event, event); diff != "" {
		return fmt.Errorf("duplicate reviewer %s eventId %q does not match expected intake event: %s", kind, result.EventID, diff)
	}
	return nil
}

func existingEventByID(caseRoot, kind, eventID string) (map[string]any, bool, error) {
	events, err := mission.ReadStrictFact(caseRoot, kind)
	if err != nil {
		return nil, false, err
	}
	for _, event := range events {
		if mission.Value(event, "eventId") == eventID {
			return event, true, nil
		}
	}
	return nil, false, nil
}

func eventMismatch(expected map[string]any, existing map[string]any) string {
	for _, key := range []string{"schemaVersion", "kind", "lane", "subject", "summary", "actor", "risk", "related", "confidence", "decision", "reason", "status", "batchId", "evidenceRefs", "target", "verifier", "verdict", "action", "approvedBy", "scope", "expires", "eventId", "packetId", "routeId", "shardId", "packetPath", "reviewerResultPath", "reviewerSession", "ownerExecutor", "ownerGeneration", "ownerBindingMode", "ownerBindingTarget"} {
		left := eventValue(expected, key)
		right := eventValue(existing, key)
		if left != right {
			return fmt.Sprintf("field %s got %q want %q", key, right, left)
		}
	}
	return ""
}

func eventValue(event map[string]any, key string) string {
	value, ok := event[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		parts := append([]string{}, typed...)
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return strings.Join(parts, "\x1f")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, strings.TrimSpace(fmt.Sprint(item)))
		}
		return strings.Join(parts, "\x1f")
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprint(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
