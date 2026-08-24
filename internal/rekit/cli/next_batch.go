package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/kitmutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

var nextBatchAfterWritePassHook func()

type nextBatchResult struct {
	SchemaVersion               int                                    `json:"schemaVersion"`
	Command                     string                                 `json:"command"`
	RepoRoot                    string                                 `json:"repoRoot"`
	IsMutation                  bool                                   `json:"isMutation"`
	Applied                     bool                                   `json:"applied"`
	ReviewRequired              bool                                   `json:"reviewRequired"`
	RequiresConfirmation        bool                                   `json:"requiresConfirmation"`
	LatestCompletedBatch        string                                 `json:"latestCompletedBatch,omitempty"`
	NextBatch                   string                                 `json:"nextBatch"`
	Domain                      string                                 `json:"domain"`
	DomainActionID              string                                 `json:"domainActionId,omitempty"`
	Closure                     string                                 `json:"closure"`
	ExpectedNextBatchPlanSHA256 string                                 `json:"expectedNextBatchPlanSha256"`
	CurrentBatchSection         string                                 `json:"currentBatchSection"`
	ChangelogEntry              string                                 `json:"changelogEntry"`
	Writes                      []nextBatchWritePlan                   `json:"writes"`
	ValidationCommands          []string                               `json:"validationCommands,omitempty"`
	ReleaseCadenceSteps         []string                               `json:"releaseCadenceSteps,omitempty"`
	MissionCommanderAction      mission.MissionCommanderNextActionItem `json:"missionCommanderAction"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue    `json:"missionCommanderActionQueue"`
	Boundary                    []string                               `json:"boundary"`
	NextSteps                   []string                               `json:"nextSteps"`
}

type nextBatchWritePlan struct {
	Path         string `json:"path"`
	Action       string `json:"action"`
	TargetPath   string `json:"targetPath"`
	InsertAfter  string `json:"insertAfter,omitempty"`
	BeforeSHA256 string `json:"beforeSha256,omitempty"`
	AfterSHA256  string `json:"afterSha256"`
	BeforeBytes  int    `json:"beforeBytes"`
	AfterBytes   int    `json:"afterBytes"`
	Changed      bool   `json:"changed"`
	PreviewText  string `json:"previewText,omitempty"`
	PlannedText  string `json:"-"`
}

func runNextBatch(ctx runtime.Context, opt Options, out io.Writer) (resultErr error) {
	var lease *kitmutation.Lease
	var err error
	if opt.Apply {
		lease, err = kitmutation.Acquire(ctx.RepoRoot)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, lease.Unlock()) }()
	}
	result := nextBatchResult{}
	committedReplay := false
	if opt.Apply {
		result, committedReplay, err = buildNextBatchCommittedReplayResult(ctx.RepoRoot, opt)
		if err != nil {
			return err
		}
	}
	if !committedReplay {
		result, err = buildNextBatchResult(ctx.RepoRoot, opt)
		if err != nil {
			return err
		}
	}
	if opt.Apply {
		if !strings.EqualFold(strings.TrimSpace(opt.ExpectedNextBatchPlanSHA256), result.ExpectedNextBatchPlanSHA256) {
			return fmt.Errorf("next-batch expected plan sha256 mismatch: got %s want %s", strings.TrimSpace(opt.ExpectedNextBatchPlanSHA256), result.ExpectedNextBatchPlanSHA256)
		}
		if err := applyNextBatchWrites(result.Writes); err != nil {
			return err
		}
		result.IsMutation = true
		result.Applied = true
		result.ReviewRequired = false
		result.RequiresConfirmation = false
		result.MissionCommanderAction = nextBatchRefreshAction()
		result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{result.MissionCommanderAction})
		result.NextSteps = []string{
			"next-batch planning receipt applied to docs/batch-history.md, CHANGELOG.md, and docs/batch-plan.md only",
			"rerun /rekit status -Format compact-json to refresh Mission Commander current action from durable docs",
			"implement the selected Windows-verifiable product-path slice, then run focused regressions and the full local release minimum",
		}
	}
	if opt.Format == "json" {
		return writeJSON(out, result)
	}
	return writeNextBatchText(out, result)
}

func buildNextBatchCommittedReplayResult(repoRoot string, opt Options) (nextBatchResult, bool, error) {
	domain := nextBatchSingleLine(opt.NextBatchDomain)
	closure := nextBatchSingleLine(opt.NextBatchClosure)
	if domain == "" || closure == "" {
		return nextBatchResult{}, false, nil
	}
	batchPlanPath := filepath.Join(repoRoot, filepath.FromSlash("docs/batch-plan.md"))
	batchHistoryPath := filepath.Join(repoRoot, filepath.FromSlash("docs/batch-history.md"))
	changelogPath := filepath.Join(repoRoot, "CHANGELOG.md")
	batchPlanText, err := os.ReadFile(batchPlanPath)
	if err != nil {
		return nextBatchResult{}, false, err
	}
	batchHistoryText, err := os.ReadFile(batchHistoryPath)
	if err != nil {
		return nextBatchResult{}, false, err
	}
	changelogText, err := os.ReadFile(changelogPath)
	if err != nil {
		return nextBatchResult{}, false, err
	}
	section, title, batchID, found := nextBatchFirstDocumentSection(string(batchPlanText))
	if !found || title != "### "+batchID+"："+closure {
		return nextBatchResult{}, false, nil
	}
	selectedDomain := nextBatchTextBetween(section, "本批选择 `", "`")
	if selectedDomain == "" || !strings.EqualFold(domain, selectedDomain) {
		return nextBatchResult{}, false, nil
	}
	if _, historyFound, historyDuplicate := nextBatchHistorySection(string(batchHistoryText), batchID); historyDuplicate || historyFound {
		return nextBatchResult{}, false, fmt.Errorf("next-batch committed replay conflicts with archived %s", batchID)
	}
	entryPrefix := "- " + batchID + " 新增 " + closure + "：选择 `" + selectedDomain + "` candidate（"
	entry, entryFound, entryDuplicate := nextBatchLineWithPrefix(string(changelogText), entryPrefix)
	if entryDuplicate {
		return nextBatchResult{}, false, fmt.Errorf("next-batch committed replay found duplicate %s changelog entries", batchID)
	}
	if !entryFound {
		return nextBatchResult{}, false, nil
	}
	writes := []nextBatchWritePlan{
		nextBatchWrite("docs/batch-history.md", "archive-completed-batch-sections", batchHistoryPath, "", batchHistoryText, string(batchHistoryText), ""),
		nextBatchWrite("CHANGELOG.md", "insert-unreleased-changelog-entry", changelogPath, "### Changed", changelogText, string(changelogText), entry),
		nextBatchWrite("docs/batch-plan.md", "rotate-and-insert-current-batch-section", batchPlanPath, "### Current batch state", batchPlanText, string(batchPlanText), section),
	}
	expectedSHA := nextBatchPlanningSHA256(writes)
	if !strings.EqualFold(strings.TrimSpace(opt.ExpectedNextBatchPlanSHA256), expectedSHA) {
		return nextBatchResult{}, false, nil
	}
	current := nextBatchApplyAction(batchID, selectedDomain, closure, expectedSHA)
	return nextBatchResult{
		SchemaVersion:               1,
		Command:                     commands.NextBatch,
		RepoRoot:                    repoRoot,
		ReviewRequired:              true,
		RequiresConfirmation:        true,
		NextBatch:                   batchID,
		Domain:                      selectedDomain,
		Closure:                     closure,
		ExpectedNextBatchPlanSHA256: expectedSHA,
		CurrentBatchSection:         section,
		ChangelogEntry:              entry,
		Writes:                      writes,
		MissionCommanderAction:      current,
		MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{current}),
		Boundary: []string{
			"next-batch committed replay is accepted only when all three current document bytes match the reviewed expected hash",
			"committed replay does not reopen next-batch selection or mutate case state, authority/confirmed, heavy tools, commits, pushes, or remote CI",
		},
	}, true, nil
}

func nextBatchTextBetween(text, prefix, suffix string) string {
	start := strings.Index(text, prefix)
	if start < 0 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(text[start:], suffix)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(text[start : start+end])
}

func nextBatchFirstDocumentSection(text string) (string, string, string, bool) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	start := strings.Index(normalized, "### Batch ")
	if start < 0 || (start > 0 && normalized[start-1] != '\n') {
		return "", "", "", false
	}
	end := len(normalized)
	for _, marker := range []string{"\n### Batch ", "\n## "} {
		if idx := strings.Index(normalized[start+1:], marker); idx >= 0 && start+1+idx < end {
			end = start + 1 + idx
		}
	}
	section := strings.TrimSpace(normalized[start:end])
	titleEnd := strings.IndexByte(section, '\n')
	if titleEnd < 0 {
		titleEnd = len(section)
	}
	title := strings.TrimSpace(section[:titleEnd])
	batchID := nextBatchSectionID(title)
	return section, title, batchID, batchID != ""
}

func nextBatchLineWithPrefix(text, prefix string) (string, bool, bool) {
	matched := ""
	found := false
	for line := range strings.SplitSeq(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if found {
			return "", true, true
		}
		matched = line
		found = true
	}
	return matched, found, false
}

func buildNextBatchResult(repoRoot string, opt Options) (nextBatchResult, error) {
	domain := nextBatchSingleLine(opt.NextBatchDomain)
	closure := nextBatchSingleLine(opt.NextBatchClosure)
	if domain == "" {
		return nextBatchResult{}, fmt.Errorf("next-batch requires -Domain with one release handoff candidate domain")
	}
	if closure == "" {
		return nextBatchResult{}, fmt.Errorf("next-batch requires -Closure describing the selected product-path closure")
	}
	if strings.ContainsAny(domain+closure, "<>") {
		return nextBatchResult{}, fmt.Errorf("next-batch -Domain and -Closure must be concrete values, not starter-package placeholders")
	}
	inventory, err := releaseCheckBuild(repoRoot)
	if err != nil {
		return nextBatchResult{}, err
	}
	pkg := inventory.ReleaseHandoff.NextBatchSelectionPackage
	if pkg == nil || !pkg.Ready || pkg.StarterPackage == nil || !pkg.StarterPackage.Ready {
		return nextBatchResult{}, fmt.Errorf("next-batch selection package is not ready; run release-check/status and finish the current batch first")
	}
	selectedAction, err := nextBatchSelectDomain(pkg, domain)
	if err != nil {
		return nextBatchResult{}, err
	}
	selectedDomain := strings.TrimSpace(selectedAction.Label)
	actionID := strings.TrimSpace(selectedAction.ActionID)
	starter := pkg.StarterPackage
	nextBatch := strings.TrimSpace(starter.SuggestedNextBatch)
	if nextBatch == "" || strings.Contains(nextBatch, "<") {
		return nextBatchResult{}, fmt.Errorf("next-batch starter package did not provide a concrete suggested batch id")
	}
	batchPlanPath := filepath.Join(repoRoot, filepath.FromSlash("docs/batch-plan.md"))
	batchHistoryPath := filepath.Join(repoRoot, filepath.FromSlash("docs/batch-history.md"))
	changelogPath := filepath.Join(repoRoot, "CHANGELOG.md")
	batchPlanText, err := os.ReadFile(batchPlanPath)
	if err != nil {
		return nextBatchResult{}, err
	}
	batchHistoryText, err := os.ReadFile(batchHistoryPath)
	if err != nil {
		return nextBatchResult{}, err
	}
	changelogText, err := os.ReadFile(changelogPath)
	if err != nil {
		return nextBatchResult{}, err
	}
	if _, found, duplicate := nextBatchHistorySection(string(batchHistoryText), nextBatch); duplicate {
		return nextBatchResult{}, fmt.Errorf("next-batch history contains duplicate %s sections", nextBatch)
	} else if found {
		return nextBatchResult{}, fmt.Errorf("next-batch history already contains %s; refusing an active/history batch id collision", nextBatch)
	}
	section := nextBatchCurrentBatchSection(nextBatch, selectedAction, closure, starter.ValidationCommands)
	entry := nextBatchChangelogEntry(nextBatch, selectedAction, closure)
	plannedBatchHistory := strings.ReplaceAll(strings.ReplaceAll(string(batchHistoryText), "\r\n", "\n"), "\r", "\n")
	archivedSections := []string{}
	batchNeedle := "### " + nextBatch + "："
	plannedBatchPlan := ""
	existingBatchSection, batchFound, batchDuplicate := nextBatchHistorySection(string(batchPlanText), nextBatch)
	if batchDuplicate {
		return nextBatchResult{}, fmt.Errorf("next-batch active plan contains duplicate %s sections", nextBatch)
	}
	if batchFound {
		if strings.TrimSpace(existingBatchSection) != strings.TrimSpace(section) {
			return nextBatchResult{}, fmt.Errorf("next-batch active plan already contains %s with different content", nextBatch)
		}
		plannedBatchPlan = strings.ReplaceAll(strings.ReplaceAll(string(batchPlanText), "\r\n", "\n"), "\r", "\n")
	} else {
		var rotatedBatchPlan string
		rotatedBatchPlan, plannedBatchHistory, archivedSections, err = nextBatchRotateActiveHistory(string(batchPlanText), string(batchHistoryText))
		if err == nil {
			plannedBatchPlan, err = nextBatchInsertAfterHeading(rotatedBatchPlan, "### Current batch state", batchNeedle, section)
		}
	}
	if err != nil {
		return nextBatchResult{}, err
	}
	plannedChangelog, err := nextBatchInsertAfterHeading(string(changelogText), "### Changed", "- "+nextBatch+" ", entry)
	if err != nil {
		return nextBatchResult{}, err
	}
	beforeBatchHistory := batchHistoryText
	beforeBatchPlan := batchPlanText
	beforeChangelog := changelogText
	if batchFound {
		beforeBatchHistory = []byte(plannedBatchHistory)
		beforeBatchPlan = []byte(plannedBatchPlan)
		beforeChangelog = []byte(plannedChangelog)
	}
	writes := []nextBatchWritePlan{
		nextBatchWrite("docs/batch-history.md", "archive-completed-batch-sections", batchHistoryPath, "", beforeBatchHistory, plannedBatchHistory, strings.Join(archivedSections, "\n\n")),
		nextBatchWrite("CHANGELOG.md", "insert-unreleased-changelog-entry", changelogPath, "### Changed", beforeChangelog, plannedChangelog, entry),
		nextBatchWrite("docs/batch-plan.md", "rotate-and-insert-current-batch-section", batchPlanPath, "### Current batch state", beforeBatchPlan, plannedBatchPlan, section),
	}
	current := nextBatchApplyAction(nextBatch, selectedDomain, closure, nextBatchPlanningSHA256(writes))
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{current})
	return nextBatchResult{
		SchemaVersion:               1,
		Command:                     commands.NextBatch,
		RepoRoot:                    repoRoot,
		IsMutation:                  false,
		Applied:                     false,
		ReviewRequired:              true,
		RequiresConfirmation:        true,
		LatestCompletedBatch:        strings.TrimSpace(starter.LatestCompletedBatch),
		NextBatch:                   nextBatch,
		Domain:                      selectedDomain,
		DomainActionID:              actionID,
		Closure:                     closure,
		ExpectedNextBatchPlanSHA256: nextBatchPlanningSHA256(writes),
		CurrentBatchSection:         section,
		ChangelogEntry:              entry,
		Writes:                      writes,
		ValidationCommands:          mission.UniqueStrings(starter.ValidationCommands),
		ReleaseCadenceSteps:         mission.UniqueStrings(starter.ReleaseCadenceSteps),
		MissionCommanderAction:      current,
		MissionCommanderActionQueue: queue,
		Boundary: []string{
			"next-batch is a kit review-first planning receipt command",
			"WhatIf reads release handoff and previews docs/batch-history.md, CHANGELOG.md, and docs/batch-plan.md writes without mutation",
			"Apply requires the exact ExpectedNextBatchPlanSha256 from WhatIf and writes only those three kit docs",
			"next-batch does not touch case state, authority/confirmed, reviewer/adapter/pack-memory/gate/sync/promote mutation, heavy tools, commits, pushes, or remote CI",
		},
		NextSteps: []string{
			"review this WhatIf output and the selected domain/closure",
			"rerun next-batch with -Apply and -ExpectedNextBatchPlanSha256 to write the planning receipt",
			"after Apply, rerun /rekit status -Format compact-json before implementation",
		},
	}, nil
}

func nextBatchApplyAction(nextBatch, domain, closure, expectedSHA string) mission.MissionCommanderNextActionItem {
	return mission.MissionCommanderNextActionItem{
		Label:          nextBatch,
		ActionID:       "next-batch-planning-receipt",
		State:          "ready-for-next-batch-planning-apply",
		Command:        fmt.Sprintf("/rekit next-batch -Domain %s -Closure %s -ExpectedNextBatchPlanSha256 %s -Apply -Format json", statusQuoteCommandArg(domain), statusQuoteCommandArg(closure), expectedSHA),
		Source:         "nextBatchCommand",
		RequiresReview: true,
		Reasons: []string{
			"release handoff next-batch selection package is ready",
			"planning receipt writes only the bounded active/history/changelog kit docs before implementation",
		},
		Boundary: []string{
			"review the next-batch WhatIf output before Apply",
			"Apply writes only docs/batch-history.md, CHANGELOG.md, and docs/batch-plan.md in the kit repo",
			"Apply does not execute reviewer, adapter, pack-memory, gate, sync, promote, heavy-tool, commit, push, or remote CI inspection",
		},
	}
}

func nextBatchRefreshAction() mission.MissionCommanderNextActionItem {
	return mission.MissionCommanderNextActionItem{
		Label:    "status-refresh",
		ActionID: "next-batch-status-refresh",
		State:    "next-batch-planning-applied-refresh-required",
		Command:  "/rekit status -Format compact-json",
		Source:   "nextBatchCommand",
		Reasons: []string{
			"next-batch planning receipt was applied to kit docs",
			"status must be refreshed before implementation follow-up is selected",
		},
		Boundary: []string{
			"status refresh is read-only",
			"do not infer follow-up work from next-batch Apply output alone",
		},
	}
}

func nextBatchSelectDomain(pkg *releasecheck.ReleaseHandoffNextBatchSelectionPackage, domain string) (mission.MissionCommanderNextActionItem, error) {
	domain = strings.TrimSpace(domain)
	allowed := []string{}
	for _, action := range pkg.MissionCommanderNextActions {
		if strings.TrimSpace(action.State) != "next-batch-candidate-domain" {
			continue
		}
		label := strings.TrimSpace(action.Label)
		actionID := strings.TrimSpace(action.ActionID)
		if label != "" {
			allowed = append(allowed, label)
		}
		if strings.EqualFold(domain, label) || strings.EqualFold(domain, actionID) {
			action.Label = label
			action.ActionID = actionID
			action.Command = nextBatchSingleLine(action.Command)
			action.Reasons = mission.UniqueStrings(action.Reasons)
			action.Boundary = mission.UniqueStrings(action.Boundary)
			return action, nil
		}
	}
	return mission.MissionCommanderNextActionItem{}, fmt.Errorf("next-batch -Domain %q is not in ready candidate domains: %s", domain, strings.Join(mission.UniqueStrings(allowed), ", "))
}

func nextBatchCurrentBatchSection(nextBatch string, action mission.MissionCommanderNextActionItem, closure string, validationCommands []string) string {
	domain := strings.TrimSpace(action.Label)
	candidateCommand := nextBatchCandidateCommand(action)
	validation := nextBatchValidationSummary(validationCommands)
	return strings.Join([]string{
		"### " + nextBatch + "：" + closure,
		"",
		"状态：进行中。本批选择 `" + domain + "`，推进 " + closure + "；release handoff candidate guidance 是：" + candidateCommand + "。上一批完成后仍需要解决的接手断点必须落在该 domain 的可操作 product-path closure 上，并能由 status/handoff/continue/release-check 或必要临时 case 验证；本批不是字段、文案或 summary 投影微调。",
		"",
		"目标：把 `" + domain + "` candidate 收敛成 Windows 本机可验证的闭环：" + closure + "。实现应复用既有 typed handoff/envelope 和 deterministic runtime 边界，让 Mission Commander 或 replacement executor 能从 durable docs/status 消费结果，不依赖上一会话隐性上下文；focused work 必须证明该候选命令所描述的能力：" + candidateCommand + "。",
		"",
		"边界：本批不新增 PowerShell runtime logic，不执行 heavy-tool，不写 authority/confirmed，不自动执行 reviewer/adapter/pack-memory/gate/sync/promote mutation，不自动提交或声明 remote CI green；`/rekit next-batch -Apply` 只在 expected hash 匹配时归档更早 active batch，并写 kit repo `docs/batch-history.md`、`CHANGELOG.md` 与 `docs/batch-plan.md` planning receipt。",
		"",
		"验证标准：focused regressions 覆盖 `" + domain + "` 的 selected product-path closure、durable status/handoff refresh，以及不回归 `/rekit next-batch` WhatIf/Apply/hash guard；随后运行完整本机 release minimum：" + validation + "。实现完成后记录 implementation commit/push 与 push-triggered remote release-gate inspection；远程 `steps=[]` 仍只记录 blocker，不声明 green。",
	}, "\n")
}

func nextBatchChangelogEntry(nextBatch string, action mission.MissionCommanderNextActionItem, closure string) string {
	domain := strings.TrimSpace(action.Label)
	candidateCommand := nextBatchCandidateCommand(action)
	return "- " + nextBatch + " 新增 " + closure + "：选择 `" + domain + "` candidate（" + candidateCommand + "）并将其收敛为 Windows 本机可验证的 product-path planning receipt；`/rekit next-batch` 仍只负责 WhatIf → `expectedNextBatchPlanSha256` → Apply 的 bounded kit docs receipt，并在写入新批次前归档更早 active batch，不触碰 case state、不执行 reviewer/adapter/pack-memory/gate/sync/promote mutation、heavy-tool、authority/confirmed 写入、自动提交或 remote CI green 声明。Focused validation、完整本机 release minimum、implementation commit/push 与 push-triggered remote inspection 待记录。"
}

func nextBatchCandidateCommand(action mission.MissionCommanderNextActionItem) string {
	command := nextBatchSingleLine(action.Command)
	if command == "" {
		return "select a Windows-verifiable product-path closure for " + strings.TrimSpace(action.Label)
	}
	return command
}

func nextBatchValidationSummary(commands []string) string {
	commands = mission.UniqueStrings(commands)
	if len(commands) == 0 {
		return "focused regressions、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`" + releasecheck.CanonicalGoTestCommand + "`、`go vet ./...` 与 `git diff --check`"
	}
	quoted := make([]string, 0, len(commands))
	for _, command := range commands {
		quoted = append(quoted, "`"+command+"`")
	}
	return strings.Join(quoted, "、")
}

func applyNextBatchWrites(writes []nextBatchWritePlan) error {
	for _, write := range writes {
		current, err := os.ReadFile(write.TargetPath)
		if err != nil {
			return err
		}
		currentSHA := nextBatchSHA256(current)
		switch {
		case strings.EqualFold(currentSHA, write.AfterSHA256):
			continue
		case strings.EqualFold(currentSHA, write.BeforeSHA256):
			if err := writeNextBatchFileAtomic(write.TargetPath, []byte(write.PlannedText)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("next-batch write %s drifted from reviewed before/after bytes", write.Path)
		}
	}
	if nextBatchAfterWritePassHook != nil {
		nextBatchAfterWritePassHook()
	}
	for _, write := range writes {
		current, err := os.ReadFile(write.TargetPath)
		if err != nil {
			return err
		}
		if currentSHA := nextBatchSHA256(current); !strings.EqualFold(currentSHA, write.AfterSHA256) {
			return fmt.Errorf("next-batch write %s drifted before final exact-byte validation", write.Path)
		}
	}
	return nil
}

func writeNextBatchFileAtomic(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".next-batch-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}

func nextBatchRotateActiveHistory(planText, historyText string) (string, string, []string, error) {
	plan := strings.ReplaceAll(strings.ReplaceAll(planText, "\r\n", "\n"), "\r", "\n")
	history := strings.ReplaceAll(strings.ReplaceAll(historyText, "\r\n", "\n"), "\r", "\n")
	starts := []int{}
	for offset := 0; offset < len(plan); {
		idx := strings.Index(plan[offset:], "### Batch ")
		if idx < 0 {
			break
		}
		idx += offset
		if idx == 0 || plan[idx-1] == '\n' {
			starts = append(starts, idx)
		}
		offset = idx + len("### Batch ")
	}
	if len(starts) <= 1 {
		return plan, history, nil, nil
	}
	archiveStart := starts[1]
	archiveEnd := len(plan)
	for _, marker := range []string{"\n## 活动文档维护规则", "\n## 验证标准"} {
		if idx := strings.Index(plan[archiveStart:], marker); idx >= 0 && archiveStart+idx < archiveEnd {
			archiveEnd = archiveStart + idx
		}
	}
	sections := strings.TrimSpace(plan[archiveStart:archiveEnd])
	if sections == "" {
		return "", "", nil, fmt.Errorf("next-batch active batch rotation found no sections to archive")
	}
	archived := []string{}
	sectionStarts := []int{}
	for offset := 0; offset < len(sections); {
		idx := strings.Index(sections[offset:], "### Batch ")
		if idx < 0 {
			break
		}
		idx += offset
		if idx == 0 || sections[idx-1] == '\n' {
			sectionStarts = append(sectionStarts, idx)
		}
		offset = idx + len("### Batch ")
	}
	for i, start := range sectionStarts {
		end := len(sections)
		if i+1 < len(sectionStarts) {
			end = sectionStarts[i+1]
		}
		section := strings.TrimSpace(sections[start:end])
		titleEnd := strings.IndexByte(section, '\n')
		if titleEnd < 0 {
			titleEnd = len(section)
		}
		title := strings.TrimSpace(section[:titleEnd])
		batchID := nextBatchSectionID(title)
		if batchID == "" {
			return "", "", nil, fmt.Errorf("next-batch active batch rotation found an invalid batch title %q", title)
		}
		historySection, found, duplicate := nextBatchHistorySection(history, batchID)
		if duplicate {
			return "", "", nil, fmt.Errorf("next-batch history contains duplicate %s sections", batchID)
		}
		if found {
			if strings.TrimSpace(historySection) != section {
				return "", "", nil, fmt.Errorf("next-batch history already contains %q with different content", title)
			}
			continue
		}
		archived = append(archived, section)
	}
	rotatedPlan := strings.TrimSpace(plan[:archiveStart]) + "\n\n" + strings.TrimLeft(plan[archiveEnd:], "\n")
	if len(archived) > 0 {
		history = strings.TrimRight(history, "\n") + "\n\n" + strings.Join(archived, "\n\n") + "\n"
	}
	return rotatedPlan, history, archived, nil
}

func nextBatchSectionID(title string) string {
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(title), "### "))
	if !strings.HasPrefix(value, "Batch ") {
		return ""
	}
	value = strings.TrimPrefix(value, "Batch ")
	end := 0
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == 0 || end >= len(value) || (value[end] != ':' && value[end:] != "：" && !strings.HasPrefix(value[end:], "：")) {
		return ""
	}
	return "Batch " + value[:end]
}

func nextBatchHistorySection(history, batchID string) (string, bool, bool) {
	starts := []int{}
	for offset := 0; offset < len(history); {
		idx := strings.Index(history[offset:], "### Batch ")
		if idx < 0 {
			break
		}
		idx += offset
		if idx == 0 || history[idx-1] == '\n' {
			starts = append(starts, idx)
		}
		offset = idx + len("### Batch ")
	}
	matched := ""
	found := false
	for i, start := range starts {
		end := len(history)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		section := strings.TrimSpace(history[start:end])
		titleEnd := strings.IndexByte(section, '\n')
		if titleEnd < 0 {
			titleEnd = len(section)
		}
		if nextBatchSectionID(section[:titleEnd]) != batchID {
			continue
		}
		if found {
			return "", true, true
		}
		matched = section
		found = true
	}
	return matched, found, false
}

func nextBatchInsertAfterHeading(text, heading, duplicateNeedle, insert string) (string, error) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if strings.Contains(normalized, duplicateNeedle) {
		if strings.Contains(normalized, strings.TrimSpace(insert)) {
			return normalized, nil
		}
		return "", fmt.Errorf("next-batch target already contains %q with different content", strings.TrimSpace(duplicateNeedle))
	}
	idx := strings.Index(normalized, heading)
	if idx < 0 {
		return "", fmt.Errorf("next-batch target heading %q not found", heading)
	}
	lineEnd := strings.Index(normalized[idx:], "\n")
	if lineEnd < 0 {
		return normalized + "\n\n" + strings.TrimSpace(insert) + "\n", nil
	}
	insertAt := idx + lineEnd + 1
	for insertAt < len(normalized) && normalized[insertAt] == '\n' {
		insertAt++
	}
	return normalized[:insertAt] + "\n" + strings.TrimSpace(insert) + "\n\n" + normalized[insertAt:], nil
}

func nextBatchWrite(path, action, targetPath, insertAfter string, before []byte, plannedText, previewText string) nextBatchWritePlan {
	return nextBatchWritePlan{
		Path:         path,
		Action:       action,
		TargetPath:   targetPath,
		InsertAfter:  insertAfter,
		BeforeSHA256: nextBatchSHA256(before),
		AfterSHA256:  nextBatchSHA256([]byte(plannedText)),
		BeforeBytes:  len(before),
		AfterBytes:   len([]byte(plannedText)),
		Changed:      string(before) != plannedText,
		PreviewText:  strings.TrimSpace(previewText),
		PlannedText:  plannedText,
	}
}

func nextBatchPlanningSHA256(writes []nextBatchWritePlan) string {
	h := sha256.New()
	nextBatchHashLength(h, len(writes))
	for _, write := range writes {
		nextBatchHashLength(h, len(write.Path))
		_, _ = io.WriteString(h, write.Path)
		nextBatchHashLength(h, len(write.PlannedText))
		_, _ = io.WriteString(h, write.PlannedText)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func nextBatchHashLength(out io.Writer, value int) {
	var encoded [8]byte
	for i := len(encoded) - 1; i >= 0; i-- {
		encoded[i] = byte(value)
		value >>= 8
	}
	_, _ = out.Write(encoded[:])
}

func nextBatchSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func nextBatchSingleLine(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func writeNextBatchText(out io.Writer, result nextBatchResult) error {
	if _, err := fmt.Fprintf(out, "next-batch：mutation=%t applied=%t reviewRequired=%t repoRoot=%s latestCompletedBatch=%s nextBatch=%s domain=%s closure=%s expectedNextBatchPlanSha256=%s writes=%d\n", result.IsMutation, result.Applied, result.ReviewRequired, result.RepoRoot, result.LatestCompletedBatch, result.NextBatch, result.Domain, result.Closure, result.ExpectedNextBatchPlanSHA256, len(result.Writes)); err != nil {
		return err
	}
	for _, write := range result.Writes {
		if _, err := fmt.Fprintf(out, "next-batch write：path=%s action=%s changed=%t beforeSha256=%s afterSha256=%s beforeBytes=%d afterBytes=%d insertAfter=%s\n", write.Path, write.Action, write.Changed, write.BeforeSHA256, write.AfterSHA256, write.BeforeBytes, write.AfterBytes, write.InsertAfter); err != nil {
			return err
		}
	}
	if err := writePrefixedMultilineText(out, "next-batch current batch section：", result.CurrentBatchSection); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "next-batch changelog entry：%s\n", result.ChangelogEntry); err != nil {
		return err
	}
	if request := result.MissionCommanderActionQueue.CurrentDriverRequest; request != nil {
		if _, err := fmt.Fprintf(out, "next-batch driver：kind=%s executable=%t requiresReview=%t command=%s\n", request.Kind, request.CommandExecutable, request.RequiresReview, request.Command); err != nil {
			return err
		}
	}
	for _, boundary := range result.Boundary {
		if _, err := fmt.Fprintf(out, "next-batch boundary：%s\n", boundary); err != nil {
			return err
		}
	}
	for _, step := range result.NextSteps {
		if _, err := fmt.Fprintf(out, "next-batch next step：%s\n", step); err != nil {
			return err
		}
	}
	return nil
}
