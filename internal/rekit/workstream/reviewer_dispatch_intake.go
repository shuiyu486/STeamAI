package workstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type ReviewerDispatchIntakeHandoff struct {
	PacketID              string   `json:"packetId,omitempty"`
	PacketPath            string   `json:"packetPath"`
	SummaryPath           string   `json:"summaryPath,omitempty"`
	ResultRoot            string   `json:"resultRoot,omitempty"`
	TargetLane            string   `json:"targetLane,omitempty"`
	ShardID               string   `json:"shardId"`
	State                 string   `json:"state"`
	ReviewerResultPath    string   `json:"reviewerResultPath,omitempty"`
	ReviewerResultPresent bool     `json:"reviewerResultPresent"`
	IntakeAvailable       bool     `json:"intakeAvailable"`
	DispatchOnly          bool     `json:"dispatchOnly"`
	VerificationRecorded  bool     `json:"verificationRecorded"`
	DecisionRecorded      bool     `json:"decisionRecorded"`
	DispatchCommand       string   `json:"dispatchCommand,omitempty"`
	PreviewCommand        string   `json:"previewCommand,omitempty"`
	ApplyCommand          string   `json:"applyCommand,omitempty"`
	OwnerExecutor         string   `json:"ownerExecutor,omitempty"`
	OwnerGeneration       int      `json:"ownerGeneration,omitempty"`
	OwnerBindingMode      string   `json:"ownerBindingMode,omitempty"`
	Evidence              []string `json:"evidence,omitempty"`
	Boundary              []string `json:"boundary,omitempty"`
}

type ReviewerDispatchIntakeSummary struct {
	Total                    int      `json:"total"`
	WaitingForReviewerResult int      `json:"waitingForReviewerResult"`
	ReadyForPreview          int      `json:"readyForPreview"`
	AttachRequired           int      `json:"attachRequired"`
	DispatchOnly             int      `json:"dispatchOnly"`
	LaneCount                int      `json:"laneCount"`
	Lanes                    []string `json:"lanes,omitempty"`
	LatestPacketPath         string   `json:"latestPacketPath,omitempty"`
	LatestShardID            string   `json:"latestShardId,omitempty"`
	LatestState              string   `json:"latestState,omitempty"`
	LatestReviewerResultPath string   `json:"latestReviewerResultPath,omitempty"`
	LatestPreviewCommand     string   `json:"latestPreviewCommand,omitempty"`
	LatestApplyCommand       string   `json:"latestApplyCommand,omitempty"`
	NextAction               string   `json:"nextAction,omitempty"`
	Boundary                 []string `json:"boundary,omitempty"`
}

type reviewerDispatchPacket struct {
	PacketID              string                              `json:"packetId"`
	Command               string                              `json:"command"`
	TargetLane            string                              `json:"targetLane"`
	Observability         reviewerDispatchPacketObservability `json:"observability"`
	ReviewerOrchestration reviewerDispatchPacketOrchestration `json:"reviewerOrchestration"`
}

type reviewerDispatchPacketObservability struct {
	SummaryPath string `json:"summaryPath"`
}

type reviewerDispatchPacketOrchestration struct {
	Mode         string                           `json:"mode"`
	TargetLane   string                           `json:"targetLane"`
	PacketPath   string                           `json:"packetPath"`
	ResultRoot   string                           `json:"resultRoot"`
	OwnerBinding reviewerDispatchPacketOwner      `json:"ownerBinding"`
	Dispatches   []reviewerDispatchPacketDispatch `json:"dispatches"`
}

type reviewerDispatchPacketOwner struct {
	TargetLane         string `json:"targetLane"`
	CurrentExecutor    string `json:"currentExecutor"`
	ExecutorGeneration int    `json:"executorGeneration"`
	BindingMode        string `json:"bindingMode"`
}

type reviewerDispatchPacketDispatch struct {
	ShardID            string `json:"shardId"`
	Status             string `json:"status"`
	ReviewerResultPath string `json:"reviewerResultPath"`
	PreviewCommand     string `json:"previewCommand"`
	ApplyCommand       string `json:"applyCommand"`
}

func ReviewerDispatchIntakeHandoffs(caseRoot string, facts mission.LedgerFacts, laneID string) ([]ReviewerDispatchIntakeHandoff, error) {
	packetPaths, err := reviewerDispatchPacketPaths(caseRoot)
	if err != nil {
		return nil, err
	}
	items := []ReviewerDispatchIntakeHandoff{}
	for _, packetPath := range packetPaths {
		packet, ok := readReviewerDispatchPacket(packetPath)
		if !ok || !strings.EqualFold(strings.TrimSpace(packet.Command), "plan-subagents") || len(packet.ReviewerOrchestration.Dispatches) == 0 {
			continue
		}
		packetTargetLane := firstText(packet.ReviewerOrchestration.TargetLane, packet.TargetLane, packet.ReviewerOrchestration.OwnerBinding.TargetLane)
		if strings.TrimSpace(laneID) != "" && packetTargetLane != laneID {
			continue
		}
		packetPathText := firstText(packet.ReviewerOrchestration.PacketPath, packetPath)
		for idx, dispatch := range packet.ReviewerOrchestration.Dispatches {
			item := reviewerDispatchIntakeHandoffFor(caseRoot, facts, packet, packetPathText, packetTargetLane, dispatch, idx)
			if item.VerificationRecorded && item.DecisionRecorded {
				continue
			}
			items = append(items, item)
		}
	}
	if maxHandoffRows > 0 && len(items) > maxHandoffRows {
		items = items[len(items)-maxHandoffRows:]
	}
	return items, nil
}

func ReviewerDispatchIntakeSummaryFor(items []ReviewerDispatchIntakeHandoff) ReviewerDispatchIntakeSummary {
	summary := ReviewerDispatchIntakeSummary{}
	lanes := map[string]bool{}
	for _, item := range items {
		summary.Total++
		if lane := strings.TrimSpace(item.TargetLane); lane != "" {
			lanes[lane] = true
		}
		if item.DispatchOnly {
			summary.DispatchOnly++
		}
		switch item.State {
		case "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
			summary.WaitingForReviewerResult++
		case "ready-for-reviewer-intake-preview":
			summary.ReadyForPreview++
		case "attach-required-before-reviewer-intake":
			summary.AttachRequired++
		}
	}
	for lane := range lanes {
		summary.Lanes = append(summary.Lanes, lane)
	}
	sort.Strings(summary.Lanes)
	summary.LaneCount = len(summary.Lanes)
	if len(items) > 0 {
		latest := items[len(items)-1]
		summary.LatestPacketPath = latest.PacketPath
		summary.LatestShardID = latest.ShardID
		summary.LatestState = latest.State
		summary.LatestReviewerResultPath = latest.ReviewerResultPath
		summary.LatestPreviewCommand = latest.PreviewCommand
		summary.LatestApplyCommand = latest.ApplyCommand
		summary.NextAction = reviewerDispatchIntakeNextAction(latest)
		summary.Boundary = reviewerDispatchIntakeSummaryBoundary()
	}
	return summary
}

func reviewerDispatchPacketPaths(caseRoot string) ([]string, error) {
	root := filepath.Join(caseRoot, ".rekit", "reviews")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packetPath := filepath.Join(root, entry.Name(), "packet.json")
		if refsfExists(packetPath) {
			paths = append(paths, packetPath)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func readReviewerDispatchPacket(path string) (reviewerDispatchPacket, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reviewerDispatchPacket{}, false
	}
	var packet reviewerDispatchPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return reviewerDispatchPacket{}, false
	}
	return packet, true
}

func reviewerDispatchIntakeHandoffFor(caseRoot string, facts mission.LedgerFacts, packet reviewerDispatchPacket, packetPath, targetLane string, dispatch reviewerDispatchPacketDispatch, idx int) ReviewerDispatchIntakeHandoff {
	resultPath := strings.TrimSpace(dispatch.ReviewerResultPath)
	present := resultPath != "" && refsfExists(resultPath)
	intakeAvailable := reviewerDispatchIntakeCommandAvailable(dispatch.PreviewCommand) && reviewerDispatchIntakeCommandAvailable(dispatch.ApplyCommand)
	verificationRecorded := reviewerDispatchWritebackRecorded(facts.Verifications, packet.PacketID, dispatch.ShardID, resultPath)
	decisionRecorded := reviewerDispatchWritebackRecorded(facts.Decisions, packet.PacketID, dispatch.ShardID, resultPath)
	state := reviewerDispatchIntakeState(present, intakeAvailable)
	item := ReviewerDispatchIntakeHandoff{
		PacketID:              packet.PacketID,
		PacketPath:            packetPath,
		SummaryPath:           packet.Observability.SummaryPath,
		ResultRoot:            packet.ReviewerOrchestration.ResultRoot,
		TargetLane:            targetLane,
		ShardID:               dispatch.ShardID,
		State:                 state,
		ReviewerResultPath:    resultPath,
		ReviewerResultPresent: present,
		IntakeAvailable:       intakeAvailable,
		DispatchOnly:          !intakeAvailable,
		VerificationRecorded:  verificationRecorded,
		DecisionRecorded:      decisionRecorded,
		DispatchCommand:       reviewerDispatchCommand(dispatch.ShardID, resultPath, idx),
		PreviewCommand:        dispatch.PreviewCommand,
		ApplyCommand:          dispatch.ApplyCommand,
		OwnerExecutor:         packet.ReviewerOrchestration.OwnerBinding.CurrentExecutor,
		OwnerGeneration:       packet.ReviewerOrchestration.OwnerBinding.ExecutorGeneration,
		OwnerBindingMode:      packet.ReviewerOrchestration.OwnerBinding.BindingMode,
	}
	item.Evidence = reviewerDispatchIntakeEvidence(caseRoot, item)
	item.Boundary = reviewerDispatchIntakeBoundary(item)
	return item
}

func reviewerDispatchIntakeCommandAvailable(command string) bool {
	command = strings.TrimSpace(command)
	return command != "" && !strings.HasPrefix(command, "n/a:")
}

func reviewerDispatchIntakeState(resultPresent, intakeAvailable bool) string {
	switch {
	case !intakeAvailable && resultPresent:
		return "attach-required-before-reviewer-intake"
	case !intakeAvailable:
		return "dispatch-only-waiting-for-result"
	case resultPresent:
		return "ready-for-reviewer-intake-preview"
	default:
		return "waiting-for-reviewer-result"
	}
}

func reviewerDispatchWritebackRecorded(events []map[string]any, packetID, shardID, reviewerResultPath string) bool {
	for _, event := range events {
		if strings.TrimSpace(packetID) != "" && firstObjectText(event, "packetId") != packetID {
			continue
		}
		if strings.TrimSpace(shardID) != "" && firstObjectText(event, "shardId") != shardID {
			continue
		}
		if strings.TrimSpace(reviewerResultPath) != "" {
			if firstObjectText(event, "reviewerResultPath") != reviewerResultPath {
				continue
			}
		}
		if firstObjectText(event, "packetId") != "" || firstObjectText(event, "shardId") != "" || firstObjectText(event, "reviewerResultPath") != "" {
			return true
		}
	}
	return false
}

func reviewerDispatchIntakeEvidence(caseRoot string, item ReviewerDispatchIntakeHandoff) []string {
	evidence := []string{}
	if strings.TrimSpace(item.PacketPath) != "" {
		evidence = append(evidence, "packet "+reviewerDispatchDisplayPath(caseRoot, item.PacketPath))
	}
	if strings.TrimSpace(item.SummaryPath) != "" {
		evidence = append(evidence, "summary "+reviewerDispatchDisplayPath(caseRoot, item.SummaryPath))
	}
	if strings.TrimSpace(item.ResultRoot) != "" {
		evidence = append(evidence, "resultRoot "+reviewerDispatchDisplayPath(caseRoot, item.ResultRoot))
	}
	if strings.TrimSpace(item.ReviewerResultPath) != "" {
		state := "missing"
		if item.ReviewerResultPresent {
			state = "present"
		}
		evidence = append(evidence, "reviewerResult "+state+" "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultPath))
	}
	if item.VerificationRecorded {
		evidence = append(evidence, "verification writeback already recorded")
	}
	if item.DecisionRecorded {
		evidence = append(evidence, "decision writeback already recorded")
	}
	return mission.UniqueStrings(evidence)
}

func reviewerDispatchIntakeBoundary(item ReviewerDispatchIntakeHandoff) []string {
	boundary := []string{
		"reviewer dispatch intake handoff is read-only; full packet.json and reviewerOrchestration remain source of truth",
		"runtime does not spawn, stop, monitor, or manage reviewer sessions",
		"reviewer result JSON must be collected before reviewer-intake preview/apply",
		"reviewer intake must run -WhatIf before -Apply and must not write authority/confirmed state or execute heavy tools",
	}
	if item.DispatchOnly {
		boundary = append(boundary, "dispatch-only packets require an attached rekit case before reviewer-intake writeback")
	}
	return mission.UniqueStrings(boundary)
}

func reviewerDispatchIntakeSummaryBoundary() []string {
	return []string{
		"reviewer dispatch intake summary is read-only; full packet.json, summary.md, and reviewerOrchestration remain available",
		"runtime does not spawn, stop, monitor, or manage reviewer sessions from status/handoff/continue",
		"run reviewer-intake -WhatIf before -Apply; do not write authority/confirmed or execute heavy tools",
	}
}

func reviewerDispatchIntakeNextAction(item ReviewerDispatchIntakeHandoff) string {
	switch item.State {
	case "ready-for-reviewer-intake-preview":
		return firstText(item.PreviewCommand, "run reviewer-intake -WhatIf for "+item.ShardID)
	case "attach-required-before-reviewer-intake":
		return "attach or init the target as a rekit case before reviewer-intake writeback for " + item.ShardID
	default:
		return "collect read-only reviewer JSON for " + item.ShardID + " at " + item.ReviewerResultPath
	}
}

func reviewerDispatchCommand(shardID, reviewerResultPath string, idx int) string {
	return "dispatch read-only reviewer for " + shardID + " using reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].dispatchPrompt; collect JSON at " + reviewerDispatchQuoteCommandArg(reviewerResultPath)
}

func reviewerDispatchQuoteCommandArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "\"\""
	}
	if !strings.ContainsAny(value, " \t\r\n\"") {
		return value
	}
	return strconv.Quote(value)
}

func reviewerDispatchDisplayPath(caseRoot, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if rel := relativePath(caseRoot, path); rel != "" && rel != path {
		return rel
	}
	return path
}

func refsfExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func WriteReviewerDispatchIntakeHandoffSection(out *bytes.Buffer, title string, items []ReviewerDispatchIntakeHandoff) {
	if len(items) == 0 {
		return
	}
	summary := ReviewerDispatchIntakeSummaryFor(items)
	fmt.Fprintln(out, title)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d latestShard=%s latestState=%s nextAction=`%s`\n", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.LatestShardID, summary.LatestState, summary.NextAction)
	for _, item := range items {
		fmt.Fprintf(out, "- dispatch intake: lane=%s shard=%s state=%s resultPresent=%t packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s`\n", item.TargetLane, item.ShardID, item.State, item.ReviewerResultPresent, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand)
		for _, evidence := range item.Evidence {
			fmt.Fprintf(out, "  - evidence: %s\n", evidence)
		}
		for _, boundary := range item.Boundary {
			fmt.Fprintf(out, "  - boundary: %s\n", boundary)
		}
	}
	fmt.Fprintln(out)
}
