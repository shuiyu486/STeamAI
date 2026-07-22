package workstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type ReviewerDispatchIntakeHandoff struct {
	PacketID                         string   `json:"packetId,omitempty"`
	PacketPath                       string   `json:"packetPath"`
	SummaryPath                      string   `json:"summaryPath,omitempty"`
	ResultRoot                       string   `json:"resultRoot,omitempty"`
	TargetLane                       string   `json:"targetLane,omitempty"`
	ShardID                          string   `json:"shardId"`
	DispatchIndex                    int      `json:"dispatchIndex,omitempty"`
	DispatchTotal                    int      `json:"dispatchTotal,omitempty"`
	DispatchCompleted                int      `json:"dispatchCompleted"`
	DispatchOpen                     int      `json:"dispatchOpen"`
	DispatchWaitingForReviewerResult int      `json:"dispatchWaitingForReviewerResult"`
	DispatchReadyForPreview          int      `json:"dispatchReadyForPreview"`
	DispatchAttachRequired           int      `json:"dispatchAttachRequired"`
	DispatchOnlyOpen                 int      `json:"dispatchOnlyOpen"`
	LatestCompletedShardID           string   `json:"latestCompletedShardId,omitempty"`
	NextOpenShardID                  string   `json:"nextOpenShardId,omitempty"`
	RemainingShardIDs                []string `json:"remainingShardIds,omitempty"`
	State                            string   `json:"state"`
	ReviewerResultPath               string   `json:"reviewerResultPath,omitempty"`
	ReviewerResultPresent            bool     `json:"reviewerResultPresent"`
	ReviewerResultState              string   `json:"reviewerResultState,omitempty"`
	IntakeAvailable                  bool     `json:"intakeAvailable"`
	DispatchOnly                     bool     `json:"dispatchOnly"`
	VerificationRecorded             bool     `json:"verificationRecorded"`
	DecisionRecorded                 bool     `json:"decisionRecorded"`
	DispatchCommand                  string   `json:"dispatchCommand,omitempty"`
	PreviewCommand                   string   `json:"previewCommand,omitempty"`
	ApplyCommand                     string   `json:"applyCommand,omitempty"`
	BatchPreviewCommand              string   `json:"batchPreviewCommand,omitempty"`
	BatchApplyCommand                string   `json:"batchApplyCommand,omitempty"`
	OwnerExecutor                    string   `json:"ownerExecutor,omitempty"`
	OwnerGeneration                  int      `json:"ownerGeneration,omitempty"`
	OwnerBindingMode                 string   `json:"ownerBindingMode,omitempty"`
	Evidence                         []string `json:"evidence,omitempty"`
	Boundary                         []string `json:"boundary,omitempty"`
}

type ReviewerDispatchIntakeSummary struct {
	Total                         int      `json:"total"`
	WaitingForReviewerResult      int      `json:"waitingForReviewerResult"`
	ReadyForPreview               int      `json:"readyForPreview"`
	AttachRequired                int      `json:"attachRequired"`
	DispatchOnly                  int      `json:"dispatchOnly"`
	LaneCount                     int      `json:"laneCount"`
	Lanes                         []string `json:"lanes,omitempty"`
	PacketCount                   int      `json:"packetCount"`
	LatestPacketDispatchTotal     int      `json:"latestPacketDispatchTotal,omitempty"`
	LatestPacketDispatchCompleted int      `json:"latestPacketDispatchCompleted"`
	LatestPacketDispatchOpen      int      `json:"latestPacketDispatchOpen"`
	LatestPacketNextOpenShardID   string   `json:"latestPacketNextOpenShardId,omitempty"`
	LatestCompletedShardID        string   `json:"latestCompletedShardId,omitempty"`
	RemainingShardIDs             []string `json:"remainingShardIds,omitempty"`
	LatestPacketPath              string   `json:"latestPacketPath,omitempty"`
	LatestShardID                 string   `json:"latestShardId,omitempty"`
	LatestState                   string   `json:"latestState,omitempty"`
	LatestReviewerResultPath      string   `json:"latestReviewerResultPath,omitempty"`
	LatestPreviewCommand          string   `json:"latestPreviewCommand,omitempty"`
	LatestApplyCommand            string   `json:"latestApplyCommand,omitempty"`
	LatestBatchPreviewCommand     string   `json:"latestBatchPreviewCommand,omitempty"`
	LatestBatchApplyCommand       string   `json:"latestBatchApplyCommand,omitempty"`
	NextAction                    string   `json:"nextAction,omitempty"`
	Boundary                      []string `json:"boundary,omitempty"`
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
	Mode                string                           `json:"mode"`
	TargetLane          string                           `json:"targetLane"`
	PacketPath          string                           `json:"packetPath"`
	ResultRoot          string                           `json:"resultRoot"`
	OwnerBinding        reviewerDispatchPacketOwner      `json:"ownerBinding"`
	Dispatches          []reviewerDispatchPacketDispatch `json:"dispatches"`
	BatchPreviewCommand string                           `json:"batchPreviewCommand"`
	BatchApplyCommand   string                           `json:"batchApplyCommand"`
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
		items = append(items, reviewerDispatchIntakeHandoffsForPacket(caseRoot, facts, packet, packetPathText, packetTargetLane)...)
	}
	return limitReviewerDispatchIntakeHandoffs(items, maxHandoffRows), nil
}

func limitReviewerDispatchIntakeHandoffs(items []ReviewerDispatchIntakeHandoff, limit int) []ReviewerDispatchIntakeHandoff {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	limited := append([]ReviewerDispatchIntakeHandoff{}, items[len(items)-limit:]...)
	if slices.ContainsFunc(limited, func(item ReviewerDispatchIntakeHandoff) bool {
		return item.State == "ready-for-reviewer-intake-preview"
	}) {
		return limited
	}
	for idx := len(items) - limit - 1; idx >= 0; idx-- {
		if items[idx].State == "ready-for-reviewer-intake-preview" {
			limited[0] = items[idx]
			break
		}
	}
	return limited
}

func reviewerDispatchIntakeHandoffsForPacket(caseRoot string, facts mission.LedgerFacts, packet reviewerDispatchPacket, packetPath, targetLane string) []ReviewerDispatchIntakeHandoff {
	all := make([]ReviewerDispatchIntakeHandoff, 0, len(packet.ReviewerOrchestration.Dispatches))
	for idx, dispatch := range packet.ReviewerOrchestration.Dispatches {
		all = append(all, reviewerDispatchIntakeHandoffFor(caseRoot, facts, packet, packetPath, targetLane, dispatch, idx))
	}
	progress := reviewerDispatchPacketProgress(all)
	open := []ReviewerDispatchIntakeHandoff{}
	for _, item := range all {
		if item.VerificationRecorded && item.DecisionRecorded {
			continue
		}
		item.DispatchCompleted = progress.Completed
		item.DispatchOpen = progress.Open
		item.DispatchWaitingForReviewerResult = progress.WaitingForReviewerResult
		item.DispatchReadyForPreview = progress.ReadyForPreview
		item.DispatchAttachRequired = progress.AttachRequired
		item.DispatchOnlyOpen = progress.DispatchOnly
		item.LatestCompletedShardID = progress.LatestCompletedShardID
		item.NextOpenShardID = progress.NextOpenShardID
		item.RemainingShardIDs = append([]string{}, progress.RemainingShardIDs...)
		open = append(open, item)
	}
	return open
}

type reviewerDispatchProgress struct {
	Completed                int
	Open                     int
	WaitingForReviewerResult int
	ReadyForPreview          int
	AttachRequired           int
	DispatchOnly             int
	LatestCompletedShardID   string
	NextOpenShardID          string
	RemainingShardIDs        []string
}

func reviewerDispatchPacketProgress(items []ReviewerDispatchIntakeHandoff) reviewerDispatchProgress {
	progress := reviewerDispatchProgress{}
	for _, item := range items {
		if item.VerificationRecorded && item.DecisionRecorded {
			progress.Completed++
			progress.LatestCompletedShardID = item.ShardID
			continue
		}
		progress.Open++
		if progress.NextOpenShardID == "" {
			progress.NextOpenShardID = item.ShardID
		}
		progress.RemainingShardIDs = append(progress.RemainingShardIDs, item.ShardID)
		if item.DispatchOnly {
			progress.DispatchOnly++
		}
		switch item.State {
		case "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
			progress.WaitingForReviewerResult++
		case "ready-for-reviewer-intake-preview":
			progress.ReadyForPreview++
		case "attach-required-before-reviewer-intake":
			progress.AttachRequired++
		}
	}
	return progress
}

func ReviewerDispatchIntakeSummaryFor(items []ReviewerDispatchIntakeHandoff) ReviewerDispatchIntakeSummary {
	summary := ReviewerDispatchIntakeSummary{}
	lanes := map[string]bool{}
	packets := map[string]bool{}
	var latestReady *ReviewerDispatchIntakeHandoff
	for idx := range items {
		item := items[idx]
		summary.Total++
		if lane := strings.TrimSpace(item.TargetLane); lane != "" {
			lanes[lane] = true
		}
		packetKey := firstText(item.PacketID, item.PacketPath)
		if packetKey != "" {
			packets[packetKey] = true
		}
		if item.DispatchOnly {
			summary.DispatchOnly++
		}
		switch item.State {
		case "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
			summary.WaitingForReviewerResult++
		case "ready-for-reviewer-intake-preview":
			summary.ReadyForPreview++
			latestReady = &items[idx]
		case "attach-required-before-reviewer-intake":
			summary.AttachRequired++
		}
	}
	for lane := range lanes {
		summary.Lanes = append(summary.Lanes, lane)
	}
	sort.Strings(summary.Lanes)
	summary.LaneCount = len(summary.Lanes)
	summary.PacketCount = len(packets)
	if len(items) > 0 {
		latest := items[len(items)-1]
		summary.LatestPacketPath = latest.PacketPath
		summary.LatestShardID = latest.ShardID
		summary.LatestState = latest.State
		summary.LatestReviewerResultPath = latest.ReviewerResultPath
		summary.LatestPreviewCommand = latest.PreviewCommand
		summary.LatestApplyCommand = latest.ApplyCommand
		summary.LatestBatchPreviewCommand = latest.BatchPreviewCommand
		summary.LatestBatchApplyCommand = latest.BatchApplyCommand
		summary.LatestPacketDispatchTotal = latest.DispatchTotal
		summary.LatestPacketDispatchCompleted = latest.DispatchCompleted
		summary.LatestPacketDispatchOpen = latest.DispatchOpen
		summary.LatestPacketNextOpenShardID = latest.NextOpenShardID
		summary.LatestCompletedShardID = latest.LatestCompletedShardID
		summary.RemainingShardIDs = append([]string{}, latest.RemainingShardIDs...)
		summary.NextAction = reviewerDispatchIntakeNextAction(latest)
		if latestReady != nil {
			summary.LatestBatchPreviewCommand = latestReady.BatchPreviewCommand
			summary.LatestBatchApplyCommand = latestReady.BatchApplyCommand
			if strings.TrimSpace(summary.LatestBatchPreviewCommand) != "" {
				summary.NextAction = summary.LatestBatchPreviewCommand
			} else {
				summary.NextAction = reviewerDispatchIntakeNextAction(*latestReady)
			}
		}
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
	resultState := refsf.RegularFileMissing
	if resultPath != "" {
		classified, err := refsf.ClassifyNonEmptyRegularFile(resultPath)
		if err != nil {
			resultState = refsf.RegularFileWaiting
		} else {
			resultState = classified
		}
	}
	present := resultState == refsf.RegularFileReady
	intakeAvailable := reviewerDispatchIntakeCommandAvailable(dispatch.PreviewCommand) && reviewerDispatchIntakeCommandAvailable(dispatch.ApplyCommand)
	verificationRecorded := reviewerDispatchWritebackRecorded(facts.Verifications, packet.PacketID, dispatch.ShardID, resultPath)
	decisionRecorded := reviewerDispatchWritebackRecorded(facts.Decisions, packet.PacketID, dispatch.ShardID, resultPath)
	state := reviewerDispatchIntakeState(resultState, intakeAvailable)
	item := ReviewerDispatchIntakeHandoff{
		PacketID:              packet.PacketID,
		PacketPath:            packetPath,
		SummaryPath:           packet.Observability.SummaryPath,
		ResultRoot:            packet.ReviewerOrchestration.ResultRoot,
		TargetLane:            targetLane,
		ShardID:               dispatch.ShardID,
		DispatchIndex:         idx + 1,
		DispatchTotal:         len(packet.ReviewerOrchestration.Dispatches),
		State:                 state,
		ReviewerResultPath:    resultPath,
		ReviewerResultPresent: present,
		ReviewerResultState:   string(resultState),
		IntakeAvailable:       intakeAvailable,
		DispatchOnly:          !intakeAvailable,
		VerificationRecorded:  verificationRecorded,
		DecisionRecorded:      decisionRecorded,
		DispatchCommand:       reviewerDispatchCommand(dispatch.ShardID, resultPath, idx),
		PreviewCommand:        dispatch.PreviewCommand,
		ApplyCommand:          dispatch.ApplyCommand,
		BatchPreviewCommand:   packet.ReviewerOrchestration.BatchPreviewCommand,
		BatchApplyCommand:     packet.ReviewerOrchestration.BatchApplyCommand,
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

func reviewerDispatchIntakeState(resultState refsf.RegularFileState, intakeAvailable bool) string {
	switch {
	case resultState == refsf.RegularFileSymlink:
		return "reviewer-result-symlink-blocked"
	case !intakeAvailable && resultState == refsf.RegularFileReady:
		return "attach-required-before-reviewer-intake"
	case !intakeAvailable:
		return "dispatch-only-waiting-for-result"
	case resultState == refsf.RegularFileReady:
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
	if item.ReviewerResultState == string(refsf.RegularFileSymlink) {
		boundary = append(boundary, "reviewer result symlinks are rejected by strict batch intake; replace the path with a regular non-empty file")
	}
	return mission.UniqueStrings(boundary)
}

func reviewerDispatchIntakeSummaryBoundary() []string {
	return []string{
		"reviewer dispatch intake summary is read-only; full packet.json, summary.md, and reviewerOrchestration remain available",
		"runtime does not spawn, stop, monitor, or manage reviewer sessions from status/handoff/continue",
		"run ready-result batch intake -WhatIf before -Apply; packet order, fail-fast, waiting results, no-authority, and no-heavy boundaries remain enforced",
	}
}

func reviewerDispatchIntakeNextAction(item ReviewerDispatchIntakeHandoff) string {
	switch item.State {
	case "ready-for-reviewer-intake-preview":
		return firstText(item.BatchPreviewCommand, item.PreviewCommand, "run reviewer-intake -WhatIf for "+item.ShardID)
	case "attach-required-before-reviewer-intake":
		return "attach or init the target as a rekit case before reviewer-intake writeback for " + item.ShardID
	case "reviewer-result-symlink-blocked":
		return "replace the symlink reviewer result with a regular non-empty file for " + item.ShardID
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

func appendReviewerDispatchIntakeHandoff(lines []string, items []ReviewerDispatchIntakeHandoff) []string {
	lines = append(lines, "", "## Reviewer dispatch intake handoff", "")
	if len(items) == 0 {
		return append(lines, "- none")
	}
	summary := ReviewerDispatchIntakeSummaryFor(items)
	lines = append(lines, fmt.Sprintf("- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d packets=%d latestPacketProgress=%d/%d open=%d nextOpen=%s remaining=%s latestShard=%s latestState=%s nextAction=`%s`", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.PacketCount, summary.LatestPacketDispatchCompleted, summary.LatestPacketDispatchTotal, summary.LatestPacketDispatchOpen, summary.LatestPacketNextOpenShardID, strings.Join(summary.RemainingShardIDs, ","), summary.LatestShardID, summary.LatestState, summary.NextAction))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- dispatch intake: lane=%s shard=%s state=%s progress=%d/%d open=%d nextOpen=%s remaining=%s resultPresent=%t resultState=%s packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s` batchPreview=`%s` batchApply=`%s`", item.TargetLane, item.ShardID, item.State, item.DispatchCompleted, item.DispatchTotal, item.DispatchOpen, item.NextOpenShardID, strings.Join(item.RemainingShardIDs, ","), item.ReviewerResultPresent, item.ReviewerResultState, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand, item.BatchPreviewCommand, item.BatchApplyCommand))
		for _, evidence := range mission.LimitStrings(item.Evidence, maxHandoffRows) {
			lines = append(lines, "  - evidence: "+evidence)
		}
		for _, boundary := range mission.LimitStrings(item.Boundary, maxHandoffRows) {
			lines = append(lines, "  - boundary: "+boundary)
		}
	}
	return lines
}

func WriteReviewerDispatchIntakeHandoffSection(out *bytes.Buffer, title string, items []ReviewerDispatchIntakeHandoff) {
	if len(items) == 0 {
		return
	}
	summary := ReviewerDispatchIntakeSummaryFor(items)
	fmt.Fprintln(out, title)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d packets=%d latestPacketProgress=%d/%d open=%d nextOpen=%s remaining=%s latestShard=%s latestState=%s nextAction=`%s`\n", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.PacketCount, summary.LatestPacketDispatchCompleted, summary.LatestPacketDispatchTotal, summary.LatestPacketDispatchOpen, summary.LatestPacketNextOpenShardID, strings.Join(summary.RemainingShardIDs, ","), summary.LatestShardID, summary.LatestState, summary.NextAction)
	for _, item := range items {
		fmt.Fprintf(out, "- dispatch intake: lane=%s shard=%s state=%s progress=%d/%d open=%d nextOpen=%s remaining=%s resultPresent=%t packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s`\n", item.TargetLane, item.ShardID, item.State, item.DispatchCompleted, item.DispatchTotal, item.DispatchOpen, item.NextOpenShardID, strings.Join(item.RemainingShardIDs, ","), item.ReviewerResultPresent, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand)
		for _, evidence := range item.Evidence {
			fmt.Fprintf(out, "  - evidence: %s\n", evidence)
		}
		for _, boundary := range item.Boundary {
			fmt.Fprintf(out, "  - boundary: %s\n", boundary)
		}
	}
	fmt.Fprintln(out)
}
