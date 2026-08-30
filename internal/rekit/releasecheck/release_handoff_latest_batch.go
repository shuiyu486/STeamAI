package releasecheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
)

type ReleaseHandoffLatestBatch struct {
	PlanPath         string                           `json:"planPath"`
	Present          bool                             `json:"present"`
	Title            string                           `json:"title"`
	BatchID          string                           `json:"batchId"`
	Status           string                           `json:"status"`
	Goal             string                           `json:"goal"`
	ValidationResult string                           `json:"validationResult"`
	Handoff          ReleaseHandoffLatestBatchHandoff `json:"handoff"`
}

type ReleaseHandoffLatestBatchHandoff struct {
	Completed                bool                                   `json:"completed"`
	LocalValidationReady     bool                                   `json:"localValidationReady"`
	ReleaseCheckReady        bool                                   `json:"releaseCheckReady"`
	LocalValidationReceipt   *LocalValidationReceiptInspection      `json:"localValidationReceipt,omitempty"`
	RemoteReleaseGate        string                                 `json:"remoteReleaseGate,omitempty"`
	RemoteReleaseGateDetail  *ReleaseHandoffRemoteReleaseGateDetail `json:"remoteReleaseGateDetail,omitempty"`
	ReleaseInspectionCadence ReleaseHandoffReleaseInspectionCadence `json:"releaseInspectionCadence"`
	PostPushReceipt          *ReleaseHandoffPostPushReceipt         `json:"postPushReceipt,omitempty"`
	CommitRefs               []string                               `json:"commitRefs,omitempty"`
	Evidence                 []string                               `json:"evidence,omitempty"`
	ValidationWarnings       []string                               `json:"validationWarnings,omitempty"`
	NextAction               string                                 `json:"nextAction,omitempty"`
}

type ReleaseHandoffReleaseInspectionCadence struct {
	MaxPushes                 int      `json:"maxPushes"`
	ImplementationCommitReady bool     `json:"implementationCommitReady"`
	InspectionCommitReady     bool     `json:"inspectionCommitReady"`
	ThirdInspectionAllowed    bool     `json:"thirdInspectionAllowed"`
	NewRemoteSignal           bool     `json:"newRemoteSignal"`
	State                     string   `json:"state"`
	NextAction                string   `json:"nextAction"`
	Evidence                  []string `json:"evidence,omitempty"`
	Boundary                  []string `json:"boundary,omitempty"`
}

type ReleaseHandoffPostPushReceipt struct {
	Ready               bool                           `json:"ready"`
	State               string                         `json:"state"`
	Subject             *LocalValidationReceiptSubject `json:"subject,omitempty"`
	Branch              string                         `json:"branch,omitempty"`
	Head                string                         `json:"head,omitempty"`
	OriginMain          string                         `json:"originMain,omitempty"`
	WorkingTreeClean    bool                           `json:"workingTreeClean"`
	Synchronized        bool                           `json:"synchronized"`
	ParentBatchID       string                         `json:"parentBatchId,omitempty"`
	ParentBatchComplete bool                           `json:"parentBatchComplete"`
	HeadBatchID         string                         `json:"headBatchId,omitempty"`
	HeadBatchComplete   bool                           `json:"headBatchComplete"`
	ChangedPaths        []string                       `json:"changedPaths,omitempty"`
	Evidence            []string                       `json:"evidence,omitempty"`
	Boundary            []string                       `json:"boundary,omitempty"`
	Warnings            []string                       `json:"warnings,omitempty"`
}

type ReleaseHandoffRemoteReleaseGateDetail struct {
	State            string   `json:"state"`
	RunRefs          []string `json:"runRefs,omitempty"`
	Jobs             []string `json:"jobs,omitempty"`
	EmptySteps       bool     `json:"emptySteps"`
	CompletedFailure bool     `json:"completedFailure"`
	CanClaimGreen    bool     `json:"canClaimGreen"`
	Boundary         []string `json:"boundary,omitempty"`
}

func releaseHandoffLatestBatchWithCurrentInventory(latest ReleaseHandoffLatestBatch, check Result) ReleaseHandoffLatestBatch {
	if !releaseHandoffCurrentInventoryCanCloseLatestBatch(latest, check) {
		return latest
	}
	evidenceSection := latestBatchEvidenceSection(latest.BatchID, strings.Join([]string{latest.Status, latest.ValidationResult}, "\n"))
	updated := latest.Handoff
	updated.ReleaseCheckReady = true
	if latestBatchHasLocalValidationCommandEvidence(evidenceSection) || latestBatchHasLocalValidationEvidenceLabels(updated.Evidence) {
		updated.LocalValidationReady = true
	}
	updated.ValidationWarnings = latestBatchValidationWarnings(evidenceSection, updated)
	updated.NextAction = latestBatchNextAction(updated)
	latest.Handoff = updated
	return latest
}

func releaseHandoffCurrentInventoryCanCloseLatestBatch(latest ReleaseHandoffLatestBatch, check Result) bool {
	if !check.Ready || !latest.Present || !latest.Handoff.Completed {
		return false
	}
	cadence := latest.Handoff.ReleaseInspectionCadence
	if cadence.State != "complete" || !cadence.ImplementationCommitReady || !cadence.InspectionCommitReady || cadence.NewRemoteSignal {
		return false
	}
	if !strings.HasPrefix(latest.Handoff.RemoteReleaseGate, "blocked:") || latest.Handoff.RemoteReleaseGateDetail == nil {
		return false
	}
	detail := latest.Handoff.RemoteReleaseGateDetail
	return detail.EmptySteps && !detail.CanClaimGreen
}

type releaseHandoffGitCommandExecutor func(string, ...string) (int, string, error)

func releaseHandoffWithValidationReceipt(repo string, latest ReleaseHandoffLatestBatch, activeRoute ReleaseHandoffActiveRoute) (ReleaseHandoffLatestBatch, ReleaseHandoffActiveRoute) {
	return releaseHandoffWithValidationReceiptUsing(repo, latest, activeRoute, defaultReleaseHandoffGitCommand)
}

func releaseHandoffWithValidationReceiptUsing(repo string, latest ReleaseHandoffLatestBatch, activeRoute ReleaseHandoffActiveRoute, executor releaseHandoffGitCommandExecutor) (ReleaseHandoffLatestBatch, ReleaseHandoffActiveRoute) {
	if !latest.Handoff.Completed {
		return latest, activeRoute
	}
	subject, receiptRequired := LocalValidationReceiptSubjectFor(ReleaseHandoff{ActiveRoute: activeRoute, LatestBatch: latest})
	machineReceiptRequired := latestBatchIDNumber(latest.BatchID) >= 817
	if !receiptRequired {
		if activeRoute.Present {
			return latest, activeRoute
		}
		if !machineReceiptRequired {
			if latest.Handoff.LocalValidationReady && latest.Handoff.ReleaseCheckReady && latest.Handoff.ReleaseInspectionCadence.State == "implementation-pending" {
				receipt := releaseHandoffPostPushReceiptFor(repo, latest, activeRoute, executor)
				return releaseHandoffLatestBatchWithReadyPostPushReceipt(latest, receipt), activeRoute
			}
			return latest, activeRoute
		}
		subject = LocalValidationReceiptSubject{Kind: LocalValidationReceiptSubjectNumberedBatch, BatchID: latest.BatchID}
		receiptRequired = true
	}
	validation := InspectLocalValidationReceipt(repo, subject)
	if subject.Kind == LocalValidationReceiptSubjectActiveRoute {
		activeRoute.LocalValidationReceipt = &validation
		activeRoute.LocalValidationReady = validation.Ready
		activeRoute.ReleaseCheckReady = validation.Ready
		activeRoute.Evidence = mission.UniqueStrings(append(activeRoute.Evidence, validation.Evidence...))
	} else {
		latest.Handoff.LocalValidationReceipt = &validation
		latest.Handoff.LocalValidationReady = validation.Ready
		latest.Handoff.ReleaseCheckReady = validation.Ready
		latest.Handoff.Evidence = mission.UniqueStrings(append(latest.Handoff.Evidence, validation.Evidence...))
	}
	if !validation.Ready {
		if validation.State == "recorded-for-implementation-commit" {
			if subject.Kind == LocalValidationReceiptSubjectNumberedBatch {
				latest.Handoff.ReleaseInspectionCadence.State = "implementation-pending"
				latest.Handoff.ReleaseInspectionCadence.ImplementationCommitReady = false
				latest.Handoff.ReleaseInspectionCadence.NextAction = "create and push the one direct implementation commit bound by the numbered-batch validation receipt, then refresh status"
				latest.Handoff.NextAction = latest.Handoff.ReleaseInspectionCadence.NextAction
			}
			return latest, activeRoute
		}
		if subject.Kind == LocalValidationReceiptSubjectNumberedBatch && receiptRequired {
			latest.Handoff.PostPushReceipt = nil
			if latest.Handoff.ReleaseInspectionCadence.State == "complete" {
				latest.Handoff.ReleaseInspectionCadence.State = "implementation-pending"
				latest.Handoff.ReleaseInspectionCadence.ImplementationCommitReady = false
				latest.Handoff.ReleaseInspectionCadence.NextAction = "rerun the full Windows local release minimum and publish a machine validation receipt before next-batch selection"
				latest.Handoff.NextAction = latest.Handoff.ReleaseInspectionCadence.NextAction
			}
		}
		return latest, activeRoute
	}
	receipt := releaseHandoffPostPushReceiptFor(repo, latest, activeRoute, executor)
	if subject.Kind == LocalValidationReceiptSubjectActiveRoute {
		activeRoute.PostPushReceipt = &receipt
		if !receipt.Ready {
			return latest, activeRoute
		}
		activeRoute.CommitRefs = mission.UniqueStrings(append(activeRoute.CommitRefs, receipt.Head))
		activeRoute.Evidence = mission.UniqueStrings(append(activeRoute.Evidence, receipt.Evidence...))
		return latest, activeRoute
	}
	return releaseHandoffLatestBatchWithReadyPostPushReceipt(latest, receipt), activeRoute
}

func releaseHandoffLatestBatchWithReadyPostPushReceipt(latest ReleaseHandoffLatestBatch, receipt ReleaseHandoffPostPushReceipt) ReleaseHandoffLatestBatch {
	latest.Handoff.PostPushReceipt = &receipt
	if !receipt.Ready {
		return latest
	}
	latest.Handoff.ReleaseInspectionCadence.ImplementationCommitReady = true
	latest.Handoff.ReleaseInspectionCadence.State = "complete"
	latest.Handoff.ReleaseInspectionCadence.NextAction = "continue the next batch without polling or waiting for remote CI"
	latest.Handoff.ReleaseInspectionCadence.Evidence = mission.UniqueStrings(append(latest.Handoff.ReleaseInspectionCadence.Evidence,
		"implementation commit reconciled with the locally known origin/main ref",
	))
	latest.Handoff.CommitRefs = mission.UniqueStrings(append(latest.Handoff.CommitRefs, receipt.Head))
	latest.Handoff.Evidence = mission.UniqueStrings(append(latest.Handoff.Evidence, receipt.Evidence...))
	latest.Handoff.NextAction = latestBatchNextAction(latest.Handoff)
	return latest
}

func releaseHandoffLatestBatchWithPostPushReceipt(repo string, latest ReleaseHandoffLatestBatch, activeRoute ReleaseHandoffActiveRoute) ReleaseHandoffLatestBatch {
	latest, _ = releaseHandoffWithValidationReceipt(repo, latest, activeRoute)
	return latest
}

func releaseHandoffLatestBatchWithPostPushReceiptUsing(repo string, latest ReleaseHandoffLatestBatch, activeRoute ReleaseHandoffActiveRoute, executor releaseHandoffGitCommandExecutor) ReleaseHandoffLatestBatch {
	latest, _ = releaseHandoffWithValidationReceiptUsing(repo, latest, activeRoute, executor)
	return latest
}

func releaseHandoffPostPushReceiptFor(repo string, latest ReleaseHandoffLatestBatch, activeRoute ReleaseHandoffActiveRoute, executor releaseHandoffGitCommandExecutor) ReleaseHandoffPostPushReceipt {
	receipt := ReleaseHandoffPostPushReceipt{
		State: "unavailable",
		Boundary: []string{
			"post-push receipt is read-only and uses only local git refs; it does not fetch, pull, push, commit, or inspect remote workflow status",
			"a synchronized HEAD proves only implementation commit publication to the currently known origin/main ref; it does not prove remote CI green",
			"dirty, non-main, missing-ref, diverged, docs-only, or ambiguous batch transitions remain implementation-pending",
		},
	}
	run := func(args ...string) (string, bool) {
		exitCode, output, err := executor(repo, args...)
		if err != nil || exitCode != 0 {
			receipt.Warnings = append(receipt.Warnings, fmt.Sprintf("git %s failed: exitCode=%d error=%v", strings.Join(args, " "), exitCode, err))
			return "", false
		}
		return strings.TrimSpace(output), true
	}
	var ok bool
	if receipt.Branch, ok = run("rev-parse", "--abbrev-ref", "HEAD"); !ok {
		return receipt
	}
	if receipt.Head, ok = run("rev-parse", "HEAD"); !ok {
		return receipt
	}
	if receipt.OriginMain, ok = run("rev-parse", "origin/main"); !ok {
		return receipt
	}
	status, statusOK := run("status", "--short")
	if !statusOK {
		return receipt
	}
	receipt.WorkingTreeClean = strings.TrimSpace(status) == ""
	receipt.Synchronized = receipt.Head != "" && strings.EqualFold(receipt.Head, receipt.OriginMain)
	parentPlan, parentOK := run("show", receipt.Head+"^:docs/batch-plan.md")
	headPlan, headOK := run("show", receipt.Head+":docs/batch-plan.md")
	changed, changedOK := run("diff-tree", "--no-commit-id", "--name-only", "-r", receipt.Head)
	if !parentOK || !headOK || !changedOK {
		return receipt
	}
	parent := latestBatchSummaryFromData("docs/batch-plan.md", []byte(parentPlan))
	head := latestBatchSummaryFromData("docs/batch-plan.md", []byte(headPlan))
	receipt.ParentBatchID = parent.BatchID
	receipt.ParentBatchComplete = parent.Handoff.Completed
	receipt.HeadBatchID = head.BatchID
	receipt.HeadBatchComplete = head.Handoff.Completed
	receipt.ChangedPaths = mission.UniqueStrings(nonEmptyReleaseHandoffLines(changed))
	finalBranch, branchOK := run("rev-parse", "--abbrev-ref", "HEAD")
	finalHead, headOK := run("rev-parse", "HEAD")
	finalOriginMain, originOK := run("rev-parse", "origin/main")
	finalStatus, finalStatusOK := run("status", "--short")
	if !branchOK || !headOK || !originOK || !finalStatusOK {
		return receipt
	}
	if finalBranch != receipt.Branch || !strings.EqualFold(finalHead, receipt.Head) || !strings.EqualFold(finalOriginMain, receipt.OriginMain) || finalStatus != status {
		receipt.State = "stale-repository-snapshot"
		return receipt
	}
	if strings.TrimSpace(receipt.Branch) != "main" {
		receipt.State = "non-main"
		return receipt
	}
	if !receipt.WorkingTreeClean {
		receipt.State = "dirty"
		return receipt
	}
	if !receipt.Synchronized {
		receipt.State = "unsynchronized"
		return receipt
	}
	activeRouteReceipt := activeRoute.Present
	validation := latest.Handoff.LocalValidationReceipt
	if activeRouteReceipt {
		validation = activeRoute.LocalValidationReceipt
	}
	validatedHead := validation != nil && validation.Ready && strings.EqualFold(validation.ValidatedHead, receipt.Head)
	if activeRouteReceipt {
		subject, subjectOK := LocalValidationReceiptSubjectFor(ReleaseHandoff{ActiveRoute: activeRoute, LatestBatch: latest})
		if !subjectOK || !validatedHead || validation == nil || validation.Receipt == nil {
			receipt.State = "ambiguous-route-transition"
			return receipt
		}
		actualSubject, err := localValidationReceiptSubjectForReceipt(*validation.Receipt)
		if err != nil || actualSubject != subject {
			receipt.State = "ambiguous-route-transition"
			return receipt
		}
		receipt.Subject = &subject
	} else {
		machineReceiptRequired := latestBatchIDNumber(latest.BatchID) >= 817
		nextBatchTransition := parent.BatchID != "" && releaseHandoffNextBatchID(parent.BatchID) == latest.BatchID
		validatedSameBatchRepair := machineReceiptRequired && validatedHead && parent.BatchID == latest.BatchID && head.BatchID == latest.BatchID
		if machineReceiptRequired && !validatedHead || (!nextBatchTransition && !validatedSameBatchRepair) || head.BatchID != latest.BatchID || !parent.Handoff.Completed || !head.Handoff.Completed {
			receipt.State = "ambiguous-batch-transition"
			return receipt
		}
	}
	if !slices.Contains(receipt.ChangedPaths, "docs/batch-plan.md") || !slices.Contains(receipt.ChangedPaths, "CHANGELOG.md") || !releaseHandoffHasImplementationPath(receipt.ChangedPaths) {
		receipt.State = "incomplete-implementation-commit"
		return receipt
	}
	if activeRouteReceipt && !slices.Contains(receipt.ChangedPaths, "docs/real-usage-hardening-roadmap.md") {
		receipt.State = "incomplete-route-implementation-commit"
		return receipt
	}
	receipt.Ready = true
	receipt.State = "post-push-complete"
	receipt.Evidence = []string{
		"post-push implementation receipt validated",
		"main HEAD equals the locally known origin/main ref",
		"implementation commit matches the exact typed validation subject",
		"implementation commit includes batch plan, changelog, and product implementation paths",
	}
	return receipt
}

func defaultReleaseHandoffGitCommand(repo string, args ...string) (int, string, error) {
	return releaseHandoffGitCommandWithEnv(repo, nil, args...)
}

func releaseHandoffGitCommandWithEnv(repo string, env []string, args ...string) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if env != nil {
		cmd.Env = env
	}
	stdout, stderr, err := processguard.RunTreeOutputs(
		ctx,
		cmd,
		nil,
		64<<20,
	)
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		err = fmt.Errorf(
			"git %s: %w: %s",
			strings.Join(args, " "),
			err,
			strings.TrimSpace(string(stderr)),
		)
	}
	return exitCode, string(stdout), err
}

func nonEmptyReleaseHandoffLines(text string) []string {
	lines := []string{}
	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if line = filepath.ToSlash(strings.TrimSpace(line)); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func releaseHandoffHasImplementationPath(paths []string) bool {
	implementationRoots := []string{
		".claude/skills/",
		".github/",
		"cmd/",
		"common/",
		"internal/",
		"packs/",
		"rekit/",
	}
	implementationFiles := []string{"go.mod", "go.sum", "go.work", "go.work.sum"}
	for _, path := range paths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if strings.EqualFold(filepath.Base(path), "README.md") {
			continue
		}
		if slices.Contains(implementationFiles, path) {
			return true
		}
		for _, root := range implementationRoots {
			if strings.HasPrefix(path, root) {
				return true
			}
		}
	}
	return false
}

func latestBatchSummary(repo string) ReleaseHandoffLatestBatch {
	const planPath = "docs/batch-plan.md"
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(planPath)))
	if err != nil {
		return ReleaseHandoffLatestBatch{PlanPath: planPath}
	}
	return latestBatchSummaryFromData(planPath, data)
}

func latestBatchSummaryFromData(planPath string, data []byte) ReleaseHandoffLatestBatch {
	latest := ReleaseHandoffLatestBatch{PlanPath: planPath, Present: true}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	start, title, batchID := latestBatchSummarySelection(lines)
	if start < 0 {
		return latest
	}
	latest.Title = title
	latest.BatchID = batchID
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "### ") {
			end = i
			break
		}
	}
	sectionLines := lines[start+1 : end]
	operationalLines := sectionLines
	for i, line := range sectionLines {
		if _, ok := markdownFieldValue(strings.TrimSpace(line), "上一批摘要"); ok {
			operationalLines = sectionLines[:i]
			break
		}
	}
	handoffFields := []string{}
	for _, line := range operationalLines {
		trimmed := strings.TrimSpace(line)
		if value, ok := markdownFieldValue(trimmed, "状态"); ok {
			latest.Status = compactHandoffText(value, 160)
			handoffFields = append(handoffFields, value)
		}
		if value, ok := markdownFieldValue(trimmed, "目标"); ok {
			latest.Goal = compactHandoffText(value, 240)
		}
		if value, ok := markdownFieldValue(trimmed, "验证结果"); ok {
			latest.ValidationResult = compactHandoffText(value, 240)
			handoffFields = append(handoffFields, value)
		}
	}
	latest.Handoff = latestBatchHandoff(latest, strings.Join(handoffFields, "\n"))
	return latest
}

func latestBatchSummarySelection(lines []string) (int, string, string) {
	start := -1
	var title string
	var batchID string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "### Batch ") {
			continue
		}
		candidateTitle := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
		candidateBatchID := batchIDFromTitle(candidateTitle)
		if candidateBatchID == "" {
			continue
		}
		if start < 0 || latestBatchIDGreater(candidateBatchID, batchID) {
			start = i
			title = candidateTitle
			batchID = candidateBatchID
		}
	}
	return start, title, batchID
}

func latestBatchIDGreater(candidate, current string) bool {
	if current == "" {
		return true
	}
	return latestBatchIDNumber(candidate) > latestBatchIDNumber(current)
}

func latestBatchIDNumber(batchID string) int {
	batchID = strings.TrimSpace(batchID)
	if !strings.HasPrefix(batchID, "Batch ") {
		return -1
	}
	var value int
	if _, err := fmt.Sscanf(batchID, "Batch %d", &value); err != nil {
		return -1
	}
	return value
}

func latestBatchHandoff(latest ReleaseHandoffLatestBatch, section string) ReleaseHandoffLatestBatchHandoff {
	evidenceSection := latestBatchEvidenceSection(latest.BatchID, section)
	handoff := ReleaseHandoffLatestBatchHandoff{
		Completed:               strings.Contains(latest.Status, "已完成"),
		LocalValidationReady:    latestBatchHasLocalValidation(evidenceSection),
		ReleaseCheckReady:       latestBatchReleaseCheckReady(evidenceSection),
		RemoteReleaseGate:       latestBatchRemoteReleaseGate(evidenceSection),
		RemoteReleaseGateDetail: latestBatchRemoteReleaseGateDetail(evidenceSection),
		CommitRefs:              latestBatchCommitRefs(evidenceSection),
		Evidence:                latestBatchEvidence(evidenceSection),
	}
	handoff.ReleaseInspectionCadence = latestBatchReleaseInspectionCadence(evidenceSection, handoff)
	handoff.ValidationWarnings = latestBatchValidationWarnings(evidenceSection, handoff)
	handoff.NextAction = latestBatchNextAction(handoff)
	return handoff
}

func latestBatchEvidenceSection(batchID, section string) string {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return section
	}
	currentBatch := latestBatchIDNumber(batchID)
	if currentBatch < 0 {
		return section
	}
	clauses := []string{}
	for _, clause := range latestBatchEvidenceClauses(section) {
		if latestBatchClauseReferencesOtherBatch(clause, currentBatch) {
			continue
		}
		clauses = append(clauses, clause)
	}
	return strings.Join(clauses, "\n")
}

func latestBatchClauseReferencesOtherBatch(clause string, currentBatch int) bool {
	for _, token := range strings.FieldsFunc(clause, func(r rune) bool {
		return r < '0' || r > '9'
	}) {
		value := 0
		if _, err := fmt.Sscanf(token, "%d", &value); err != nil {
			continue
		}
		if value == currentBatch {
			continue
		}
		idx := strings.Index(clause, token)
		if idx < 0 {
			continue
		}
		prefixStart := max(0, idx-16)
		prefix := strings.ToLower(clause[prefixStart:idx])
		if strings.Contains(prefix, "batch ") || strings.Contains(prefix, "batch") {
			return true
		}
	}
	return false
}

func latestBatchReleaseCheckReady(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "release-check ready=true") ||
		(strings.Contains(lower, "release-check -format json") && strings.Contains(lower, "ready=true")) ||
		latestBatchHasReleaseRunSuccess(lower)
}

// LatestBatchDocumentsRecordLocalValidation reports whether the tracked latest-batch prose records a successful local release minimum independently of post-commit receipt readiness.
func LatestBatchDocumentsRecordLocalValidation(latest ReleaseHandoffLatestBatch) bool {
	if slices.Contains(latest.Handoff.Evidence, "release-run local release minimum passed") {
		return true
	}
	evidenceSection := latestBatchEvidenceSection(latest.BatchID, strings.Join([]string{latest.Status, latest.ValidationResult}, "\n"))
	return latestBatchHasLocalValidation(evidenceSection)
}

func latestBatchHasLocalValidation(text string) bool {
	lower := strings.ToLower(text)
	for _, pending := range []string{"完整本地 release minimum 待", "本地 release minimum 待", "完整本机 release minimum 待", "本机 release minimum 待", "local release minimum pending", "full local release minimum pending"} {
		if strings.Contains(lower, pending) {
			return false
		}
	}
	if latestBatchHasReleaseRunSuccess(lower) {
		return true
	}
	if !latestBatchReleaseCheckReady(text) {
		return false
	}
	return latestBatchHasLocalValidationCommandEvidence(text)
}

func latestBatchHasLocalValidationCommandEvidence(text string) bool {
	if latestBatchHasLocalValidationEvidenceLabels(latestBatchEvidence(text)) {
		return true
	}
	lower := strings.ToLower(text)
	for _, aliases := range [][]string{
		{"go run ./cmd/rekit -- -command status", "`status`", "status-step"},
		{"go run ./cmd/rekit -- -command packs", "`packs`", "packs-step"},
		{"go run ./cmd/rekit -- -command doctor", "`doctor`", "doctor-step"},
		{CanonicalGoTestCommand, "go test -p=2 -timeout=15m ./...", "go test -timeout=15m ./...", "go test ./..."},
		{"go vet ./..."},
		{"git diff --check"},
	} {
		if !latestBatchContainsAny(lower, aliases...) {
			return false
		}
	}
	return true
}

func latestBatchContainsAny(lower string, aliases ...string) bool {
	for _, alias := range aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias != "" && strings.Contains(lower, alias) {
			return true
		}
	}
	return false
}

func latestBatchHasLocalValidationEvidenceLabels(evidence []string) bool {
	for _, label := range []string{
		"status handoff recorded",
		"packs inventory recorded",
		"doctor validation recorded",
		"go test ./... recorded",
		"go vet ./... recorded",
		"git diff --check recorded",
	} {
		if !slices.Contains(evidence, label) {
			return false
		}
	}
	return true
}

func latestBatchHasReleaseRunSuccess(lower string) bool {
	if !strings.Contains(lower, "release-run") {
		return false
	}
	if strings.Contains(lower, "ready=true") {
		if strings.Contains(lower, "summary=release run ok") || strings.Contains(lower, "release run ok") {
			return true
		}
		if strings.Contains(lower, "passed=7") && strings.Contains(lower, "failed=0") && strings.Contains(lower, "skipped=0") {
			return true
		}
	}
	for _, clause := range latestBatchEvidenceClauses(lower) {
		compact := strings.NewReplacer(" ", "", "\t", "").Replace(clause)
		beforeReleaseRun, afterReleaseRun, found := strings.Cut(compact, "release-run")
		if !found {
			continue
		}
		beforeSuccess, afterSuccess, successMarker, found := latestBatchReleaseRunSuccessParts(afterReleaseRun)
		if !found {
			continue
		}
		if latestBatchContainsAny(compact, "未通过", "待执行", "待完成", "尚未执行", "失败", "failed", "ready=false", "目标", "计划", "预期", "应达到", "不能证明", "并非", "历史", "旧文案") {
			continue
		}
		assertion := beforeReleaseRun + "release-run" + beforeSuccess + successMarker
		if latestBatchContainsAny(assertion, "待统一", "是否", "待确认", "要求", "定义", "标准", "条件", "？", "?") || latestBatchReleaseRunPendingSuffix(afterSuccess) {
			continue
		}
		if latestBatchContainsAny(assertion, "完成态", "最终", "已通过", "成功") {
			return true
		}
		if (beforeReleaseRun == "统一" || beforeReleaseRun == "统一`") && latestBatchReleaseRunTimingSuffix(afterSuccess) {
			return true
		}
	}
	return false
}

func latestBatchReleaseRunSuccessParts(text string) (string, string, string, bool) {
	for _, marker := range []string{"以7/7通过", "7/7均通过", "7/7全部通过"} {
		before, after, found := strings.Cut(text, marker)
		if found {
			return before, after, marker, true
		}
	}
	return "", "", "", false
}

func latestBatchReleaseRunPendingSuffix(text string) bool {
	text = strings.TrimLeft(text, "`，,:：")
	return strings.HasPrefix(text, "结果待确认") ||
		strings.HasPrefix(text, "结果待核实") ||
		strings.HasPrefix(text, "尚待确认") ||
		strings.HasPrefix(text, "待确认") ||
		strings.HasPrefix(text, "待核实") ||
		strings.HasPrefix(text, "？") ||
		strings.HasPrefix(text, "?")
}

func latestBatchReleaseRunTimingSuffix(text string) bool {
	text = strings.Trim(text, "`")
	if text == "" {
		return true
	}
	var inner string
	switch {
	case strings.HasPrefix(text, "（") && strings.HasSuffix(text, "）"):
		inner = strings.TrimSuffix(strings.TrimPrefix(text, "（"), "）")
	case strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")"):
		inner = strings.TrimSuffix(strings.TrimPrefix(text, "("), ")")
	default:
		return false
	}
	if latestBatchContainsAny(inner, "待确认", "待核实", "尚待", "失败", "failed", "？", "?") {
		return false
	}
	return latestBatchContainsAny(inner, "秒", "ms", "毫秒") && strings.ContainsAny(inner, "0123456789")
}

func latestBatchHasReleaseRunTransientRetry(lower string) bool {
	if !strings.Contains(lower, "release-run") {
		return false
	}
	return strings.Contains(lower, "transientretryreason") || strings.Contains(lower, "release-run step retry") || strings.Contains(lower, "attempts=2")
}

func latestBatchValidationWarnings(text string, handoff ReleaseHandoffLatestBatchHandoff) []string {
	lower := strings.ToLower(text)
	warnings := []string{}
	if latestBatchHasReleaseRunTransientRetry(lower) {
		warnings = append(warnings, "release-run local validation passed only after a recorded transient retry; review retry reason and first-attempt output before release handoff")
	}
	if latestBatchHasStalePendingReleaseStepNarrative(text, handoff) {
		warnings = append(warnings, "latest batch validation text still contains stale pending release steps after cadence complete; release inspection cadence evidence wins, clean docs before handoff")
	}
	return mission.UniqueStrings(warnings)
}

func latestBatchHasStalePendingReleaseStepNarrative(text string, handoff ReleaseHandoffLatestBatchHandoff) bool {
	cadence := handoff.ReleaseInspectionCadence
	if cadence.State != "complete" || !cadence.ImplementationCommitReady || !cadence.InspectionCommitReady || cadence.NewRemoteSignal || !handoff.LocalValidationReady || !handoff.ReleaseCheckReady || handoff.RemoteReleaseGate == "not-recorded" {
		return false
	}
	for _, clause := range latestBatchEvidenceClauses(text) {
		clauseLower := strings.ToLower(clause)
		if !latestBatchPendingReleaseStepClause(clause, clauseLower) {
			continue
		}
		if latestBatchImplementationCommitEvidence(clause) || latestBatchRemoteEvidenceClause(clause, clauseLower) || latestBatchRemoteEvidenceDetailClause(clause, clauseLower) {
			continue
		}
		return true
	}
	return false
}

func latestBatchPendingReleaseStepNarrativeClause(clause, lower string) bool {
	for _, marker := range []string{
		"残留",
		"过期",
		"误读",
		"误导",
		"历史",
		"旧句",
		"这类",
		"例如",
		"可能把",
		"可能导致",
		"保留真正",
		"场景",
		"不被误标",
		"已验证当前",
		"-run",
		"testlatestbatch",
		"regression",
		"focused",
		"fail-closed",
		"would otherwise",
		"stale pending",
		"narrative",
	} {
		if strings.Contains(clause, marker) || strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func latestBatchPendingReleaseStepClause(clause, lower string) bool {
	pending := strings.Contains(clause, "待执行") || strings.Contains(clause, "待检查") || strings.Contains(clause, "待完成") || strings.Contains(clause, "尚未") || strings.Contains(lower, "pending")
	if !pending || latestBatchPendingReleaseStepNarrativeClause(clause, lower) {
		return false
	}
	return strings.Contains(lower, "implementation commit") ||
		strings.Contains(lower, "commit/push") ||
		strings.Contains(lower, "push-triggered") ||
		strings.Contains(lower, "remote release-gate") ||
		strings.Contains(lower, "release inspection") ||
		strings.Contains(lower, "workflow run") ||
		strings.Contains(lower, "release minimum") ||
		strings.Contains(clause, "完整本机 release minimum") ||
		strings.Contains(clause, "完整本地 release minimum") ||
		strings.Contains(clause, "远程 release-gate")
}

func latestBatchRemoteReleaseGate(text string) string {
	remoteText := latestBatchRemoteEvidenceText(text)
	if strings.TrimSpace(remoteText) == "" {
		return "not-recorded"
	}
	remoteLower := strings.ToLower(remoteText)
	emptySteps := latestBatchRemoteHasEmptySteps(remoteText, remoteLower)
	switch {
	case emptySteps && strings.Contains(remoteLower, "completed failure"):
		return "blocked: completed failure with jobs steps=[]"
	case emptySteps:
		return "blocked: jobs steps=[]"
	case latestBatchRemoteGreen(remoteText, remoteLower):
		return "green"
	case strings.Contains(remoteText, "远程 release-gate") || strings.Contains(remoteLower, "release-gate run") || strings.Contains(remoteLower, "workflow run") || strings.Contains(remoteLower, "pr run") || strings.Contains(remoteLower, "implementation run") || strings.Contains(remoteLower, "push run"):
		return "inspected"
	default:
		return "not-recorded"
	}
}

func latestBatchRemoteEvidenceText(text string) string {
	clauses := []string{}
	remoteContext := false
	for _, clause := range latestBatchEvidenceClauses(text) {
		lower := strings.ToLower(clause)
		if latestBatchRemoteInspectionPending(clause, lower) {
			remoteContext = false
			continue
		}
		if latestBatchRemoteEvidenceClause(clause, lower) {
			clauses = append(clauses, clause)
			remoteContext = true
			continue
		}
		if remoteContext && latestBatchRemoteEvidenceDetailClause(clause, lower) {
			clauses = append(clauses, clause)
			continue
		}
		remoteContext = false
	}
	return strings.Join(clauses, "\n")
}

func latestBatchRemoteEvidenceNarrativeClause(clause, lower string) bool {
	if len(latestBatchRemoteRunRefs(clause)) > 0 {
		return false
	}
	for _, marker := range []string{
		"regression",
		"parser",
		"目标：",
		"边界：",
		"验证标准",
		"回归",
		"测试",
		"覆盖",
		"被分成",
		"分在不同",
		"误判",
		"诱导",
		"仍识别",
		"仍触发",
	} {
		if strings.Contains(lower, marker) || strings.Contains(clause, marker) {
			return true
		}
	}
	return false
}

func latestBatchRemoteEvidenceDetailClause(clause, lower string) bool {
	if latestBatchRemoteInspectionPending(clause, lower) || latestBatchRemoteEvidenceNarrativeClause(clause, lower) {
		return false
	}
	return strings.Contains(lower, "job") ||
		strings.Contains(lower, "steps=[]") ||
		strings.Contains(lower, "steps: []") ||
		strings.Contains(clause, "steps=[]") ||
		strings.Contains(clause, "steps 为空") ||
		strings.Contains(clause, "steps为空") ||
		strings.Contains(lower, "log not found") ||
		strings.Contains(lower, "billing") ||
		strings.Contains(lower, "spending limit") ||
		strings.Contains(clause, "无 logs")
}

func latestBatchRemoteEvidenceClause(clause, lower string) bool {
	if latestBatchRemoteInspectionPending(clause, lower) || latestBatchRemoteEvidenceNarrativeClause(clause, lower) {
		return false
	}
	if latestBatchRemoteGreen(clause, lower) {
		return true
	}
	remoteContext := strings.Contains(lower, "release-gate") || strings.Contains(lower, "remote") || strings.Contains(clause, "远程")
	jobContext := strings.Contains(lower, "job") || strings.Contains(lower, "jobs")
	completed := strings.Contains(lower, "completed") || strings.Contains(lower, "failure") || strings.Contains(lower, "success")
	runContext := strings.Contains(lower, "release-gate run") || strings.Contains(lower, "workflow run") || strings.Contains(lower, "pr run") || strings.Contains(lower, "implementation run") || strings.Contains(lower, "push run")
	if runContext {
		return len(latestBatchRemoteRunRefs(clause)) > 0 || jobContext || completed
	}
	if strings.Contains(clause, "远程 release-gate") && (strings.Contains(clause, "已检查") || strings.Contains(clause, "已记录")) {
		return true
	}
	return remoteContext && jobContext && completed
}

func latestBatchEvidenceClauses(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.NewReplacer("。", "\n", "；", "\n", ";", "\n").Replace(text)
	clauses := []string{}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			clauses = append(clauses, line)
		}
	}
	return clauses
}

func latestBatchRemoteInspectionPending(text, lower string) bool {
	for _, pending := range []string{
		"远程 release-gate inspection 待",
		"remote release-gate inspection pending",
		"release-gate inspection pending",
		"remote workflow run pending",
		"workflow run pending",
	} {
		if strings.Contains(text, pending) || strings.Contains(lower, pending) {
			return true
		}
	}
	pendingChinese := strings.Contains(text, "尚未检查") || strings.Contains(text, "尚未记录") || strings.Contains(text, "待检查")
	remoteRun := strings.Contains(text, "远程") && (strings.Contains(lower, "workflow run") || strings.Contains(lower, "release-gate run") || strings.Contains(text, "release-gate"))
	return pendingChinese && remoteRun
}

func latestBatchRemoteReleaseGateDetail(text string) *ReleaseHandoffRemoteReleaseGateDetail {
	remoteText := latestBatchRemoteEvidenceText(text)
	remoteLower := strings.ToLower(remoteText)
	state := latestBatchRemoteReleaseGate(text)
	detail := ReleaseHandoffRemoteReleaseGateDetail{
		State:         state,
		CanClaimGreen: state == "green",
	}
	if state != "not-recorded" {
		detail.RunRefs = latestBatchRemoteRunRefs(remoteText)
		detail.Jobs = latestBatchRemoteJobs(remoteLower)
		detail.EmptySteps = latestBatchRemoteHasEmptySteps(remoteText, remoteLower)
		detail.CompletedFailure = strings.Contains(remoteLower, "completed failure")
	}
	switch {
	case state == "green":
		detail.Boundary = []string{"remote CI green is claimable only because the latest batch explicitly records green jobs"}
	case strings.HasPrefix(state, "blocked:"):
		detail.Boundary = []string{"treat remote release-gate steps=[] as a known runner/billing blocker", "do not claim remote CI green", "continue only Windows-verifiable local product-path work"}
	case state == "not-recorded":
		detail.Boundary = []string{"inspect the remote release-gate run before claiming remote CI status", "release-check inventory ready is not remote CI green"}
	default:
		detail.Boundary = []string{"remote release-gate was inspected, but do not claim remote CI green without explicit green jobs"}
	}
	return &detail
}

func latestBatchRemoteHasEmptySteps(text, lower string) bool {
	compact := strings.NewReplacer(" ", "", "\t", "", "`", "").Replace(lower)
	return strings.Contains(compact, "steps:[]") || strings.Contains(compact, "steps=[]") || strings.Contains(text, "steps 为空") || strings.Contains(text, "steps为空")
}

func latestBatchReleaseInspectionCadence(text string, handoff ReleaseHandoffLatestBatchHandoff) ReleaseHandoffReleaseInspectionCadence {
	lower := strings.ToLower(text)
	cadence := ReleaseHandoffReleaseInspectionCadence{
		MaxPushes:                 1,
		ImplementationCommitReady: latestBatchImplementationCommitReady(text),
		InspectionCommitReady:     latestBatchInspectionCommitReady(text, handoff),
		NewRemoteSignal:           latestBatchHasNewRemoteSignal(lower, handoff),
		Boundary: []string{
			"normal Windows-first batches stop after one implementation commit/push",
			"remote Linux/macOS/Windows workflow is asynchronous and non-blocking for normal batches",
			"wait for and record remote results only for release, cross-platform work, or periodic review",
		},
	}
	if cadence.ImplementationCommitReady {
		cadence.Evidence = append(cadence.Evidence, "implementation commit/push recorded")
	}
	if cadence.InspectionCommitReady {
		cadence.Evidence = append(cadence.Evidence, "asynchronous remote release-gate observation recorded")
	}
	if handoff.RemoteReleaseGateDetail != nil && handoff.RemoteReleaseGateDetail.EmptySteps {
		cadence.Evidence = append(cadence.Evidence, "remote release-gate steps=[] blocker recorded")
	}
	if cadence.NewRemoteSignal {
		cadence.Evidence = append(cadence.Evidence, "asynchronous new remote signal recorded")
	}
	if !cadence.ImplementationCommitReady {
		cadence.State = "implementation-pending"
		cadence.NextAction = "create/push the implementation commit after Windows local validation"
	} else {
		cadence.State = "complete"
		cadence.NextAction = "continue the next batch without polling or waiting for remote CI"
	}
	return cadence
}

func latestBatchImplementationCommitReady(text string) bool {
	return latestBatchImplementationCommitEvidence(text)
}

func latestBatchImplementationCommitEvidence(text string) bool {
	for _, clause := range latestBatchEvidenceClauses(text) {
		clauseLower := strings.ToLower(clause)
		if latestBatchContainsAny(clauseLower,
			"do not", "不要", "不为",
			"尚未推送", "未推送", "待推送", "推送待", "push pending", "pending push", "not pushed", "without push",
			"尚未提交推送", "尚未创建本批代码提交",
		) {
			continue
		}
		if strings.Contains(clause, "已推送") || strings.Contains(clause, "已提交并推送") || strings.Contains(clauseLower, "implementation commit/push recorded") {
			return true
		}
	}
	return false
}

func latestBatchInspectionCommitReady(_ string, handoff ReleaseHandoffLatestBatchHandoff) bool {
	return handoff.RemoteReleaseGate != "not-recorded"
}

func latestBatchHasNewRemoteSignal(lower string, handoff ReleaseHandoffLatestBatchHandoff) bool {
	if strings.Contains(lower, "new remote signal recorded") || strings.Contains(lower, "新远程信号已记录") || strings.Contains(lower, "新信号已记录") {
		return true
	}
	if handoff.RemoteReleaseGate == "green" {
		return true
	}
	if handoff.RemoteReleaseGateDetail == nil || handoff.RemoteReleaseGate == "not-recorded" {
		return false
	}
	return handoff.RemoteReleaseGateDetail.CompletedFailure && !handoff.RemoteReleaseGateDetail.EmptySteps
}

func latestBatchRemoteJobs(lower string) []string {
	jobs := []string{}
	for _, candidate := range []struct {
		match string
		name  string
	}{
		{match: "linux", name: "Linux"},
		{match: "windows", name: "Windows"},
		{match: "macos", name: "macOS"},
	} {
		if strings.Contains(lower, candidate.match) {
			jobs = append(jobs, candidate.name)
		}
	}
	return jobs
}

func latestBatchRemoteRunRefs(text string) []string {
	refs := []string{}
	seen := map[string]bool{}
	for {
		start := strings.Index(text, "`")
		if start < 0 {
			break
		}
		text = text[start+1:]
		end := strings.Index(text, "`")
		if end < 0 {
			break
		}
		token := strings.TrimSpace(text[:end])
		if looksLikeRunRef(token) && !seen[token] {
			seen[token] = true
			refs = append(refs, token)
		}
		text = text[end+1:]
	}
	return refs
}

func looksLikeRunRef(value string) bool {
	if len(value) < 6 || len(value) > 20 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func latestBatchRemoteGreen(text, _ string) bool {
	for _, clause := range latestBatchEvidenceClauses(text) {
		compact := strings.NewReplacer(" ", "", "\t", "", "`", "").Replace(strings.ToLower(clause))
		for _, token := range []string{"remotecigreen", "远程cigreen"} {
			remaining := compact
			for {
				index := strings.Index(remaining, token)
				if index < 0 {
					break
				}
				prefix := remaining[:index]
				if !latestBatchRemoteGreenClaimNegated(prefix) {
					return true
				}
				remaining = remaining[index+len(token):]
			}
		}
	}
	return false
}

func latestBatchRemoteGreenClaimNegated(prefix string) bool {
	for _, marker := range []string{
		"不能声明", "不得声明", "不声明", "不声称", "不能声称", "不得声称",
		"不证明", "不能证明", "不等于", "并非", "而不是", "不是",
		"cannotclaim", "donotclaim", "doesnotclaim", "mustnotclaim", "shouldnotclaim", "notclaim",
		"cannotprove", "donotprove", "doesnotprove", "notprove", "isnot", "arenot", "wasnot", "werenot", "not",
	} {
		index := strings.LastIndex(prefix, marker)
		if index < 0 {
			continue
		}
		scope := prefix[index+len(marker):]
		if len([]rune(scope)) > 64 || strings.ContainsAny(scope, "，,。；;：:") {
			continue
		}
		if latestBatchContainsAny(scope, "但", "然而", "不过", "however", "but", "yet") {
			continue
		}
		return true
	}
	return false
}

func latestBatchCommitRefs(text string) []string {
	refs := []string{}
	seen := map[string]bool{}
	for _, clause := range latestBatchEvidenceClauses(text) {
		if !latestBatchCommitEvidenceClause(clause) {
			continue
		}
		for _, token := range backtickTokens(latestBatchCommitRefScope(clause)) {
			if looksLikeCommitRef(token) && !seen[token] {
				seen[token] = true
				refs = append(refs, token)
			}
		}
	}
	return refs
}

func latestBatchCommitRefScope(clause string) string {
	lower := strings.ToLower(clause)
	if start := latestBatchCommitMarkerIndex(lower); start >= 0 {
		clause = clause[start:]
		lower = lower[start:]
	}
	cutoff := len(clause)
	for _, marker := range []string{
		"pr #",
		"remote",
		"远程",
		"release-gate run",
		"workflow run",
		"pr run",
		"jobs",
		"job ",
	} {
		if idx := strings.Index(lower, marker); idx >= 0 && idx < cutoff {
			cutoff = idx
		}
	}
	return strings.TrimSpace(clause[:cutoff])
}

func latestBatchCommitMarkerIndex(lower string) int {
	best := -1
	for _, marker := range []string{
		"implementation commits",
		"implementation commit",
		"commits `",
		"commit `",
	} {
		if idx := strings.Index(lower, marker); idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

func latestBatchCommitEvidenceClause(clause string) bool {
	lower := strings.ToLower(clause)
	if strings.Contains(lower, "do not") || strings.Contains(lower, "不要") || strings.Contains(lower, "不为") {
		return false
	}
	return strings.Contains(lower, "implementation commit") ||
		strings.Contains(lower, "implementation commits") ||
		strings.Contains(lower, "implementation commit/push recorded") ||
		strings.Contains(clause, "已提交并推送") ||
		strings.Contains(clause, "已推送")
}

func backtickTokens(text string) []string {
	tokens := []string{}
	for {
		start := strings.Index(text, "`")
		if start < 0 {
			break
		}
		text = text[start+1:]
		end := strings.Index(text, "`")
		if end < 0 {
			break
		}
		tokens = append(tokens, strings.TrimSpace(text[:end]))
		text = text[end+1:]
	}
	return tokens
}

func looksLikeCommitRef(value string) bool {
	if len(value) < 7 || len(value) > 40 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func latestBatchEvidence(text string) []string {
	lower := strings.ToLower(text)
	remoteText := latestBatchRemoteEvidenceText(text)
	remoteLower := strings.ToLower(remoteText)
	evidence := []string{}
	for _, candidate := range []struct {
		match string
		label string
	}{
		{match: "public cli", label: "public CLI product-path validation recorded"},
		{match: "go run ./cmd/rekit -- -command release-check -format json", label: "release-check -Format json recorded"},
		{match: "release-check -format json", label: "release-check -Format json recorded"},
		{match: "releasecheck-ready", label: "release-check -Format json recorded"},
		{match: "releasecheck-step", label: "release-check -Format json recorded"},
		{match: "go run ./cmd/rekit -- -command status", label: "status handoff recorded"},
		{match: "`status`", label: "status handoff recorded"},
		{match: "status-step", label: "status handoff recorded"},
		{match: "go run ./cmd/rekit -- -command packs", label: "packs inventory recorded"},
		{match: "`packs`", label: "packs inventory recorded"},
		{match: "packs-step", label: "packs inventory recorded"},
		{match: "go run ./cmd/rekit -- -command doctor", label: "doctor validation recorded"},
		{match: "`doctor`", label: "doctor validation recorded"},
		{match: "doctor-step", label: "doctor validation recorded"},
		{match: CanonicalGoTestCommand, label: "go test ./... recorded"},
		{match: "go test -p=2 -timeout=15m ./...", label: "go test ./... recorded"},
		{match: "go test -timeout=15m ./...", label: "go test ./... recorded"},
		{match: "go test ./...", label: "go test ./... recorded"},
		{match: "go vet ./...", label: "go vet ./... recorded"},
		{match: "git diff --check", label: "git diff --check recorded"},
		{match: "release-run", label: "release-run local release minimum recorded"},
		{match: "release-run-success", label: "release-run local release minimum passed"},
		{match: "release-run-retry", label: "release-run transient retry recorded"},
		{match: "release-check ready=true", label: "release-check ready=true recorded"},
		{match: "release-run-ready", label: "release-check ready=true recorded"},
		{match: "steps: []", label: "remote release-gate jobs steps=[] recorded"},
	} {
		if !latestBatchEvidenceMatched(candidate.match, text, lower, remoteText, remoteLower) {
			continue
		}
		evidence = append(evidence, candidate.label)
	}
	return mission.UniqueStrings(evidence)
}

func latestBatchEvidenceMatched(match, text, lower, remoteText, remoteLower string) bool {
	switch match {
	case "steps: []":
		return latestBatchRemoteReleaseGate(text) != "not-recorded" && latestBatchRemoteHasEmptySteps(remoteText, remoteLower)
	case "release-run-ready", "release-run-success":
		return latestBatchHasReleaseRunSuccess(lower)
	case "releasecheck-ready":
		return latestBatchReleaseCheckReady(text)
	case "release-run-retry":
		return latestBatchHasReleaseRunTransientRetry(lower)
	case "releasecheck-step":
		return latestBatchHasReleaseRunSuccess(lower) && strings.Contains(lower, "release-check")
	case "status-step":
		return latestBatchHasReleaseRunSuccess(lower) && strings.Contains(lower, "status")
	case "packs-step":
		return latestBatchHasReleaseRunSuccess(lower) && strings.Contains(lower, "packs")
	case "doctor-step":
		return latestBatchHasReleaseRunSuccess(lower) && strings.Contains(lower, "doctor")
	default:
		return strings.Contains(lower, match)
	}
}

func latestBatchNextAction(handoff ReleaseHandoffLatestBatchHandoff) string {
	switch {
	case !handoff.Completed:
		return "finish the current batch before treating status as a handoff"
	case !handoff.LocalValidationReady:
		return "run the full Windows local release minimum and update docs/batch-plan.md"
	case handoff.ReleaseInspectionCadence.State == "implementation-pending":
		return handoff.ReleaseInspectionCadence.NextAction
	default:
		return "select the next Windows-verifiable product-path batch from docs/context-routing.md and docs/batch-plan.md without waiting for remote CI"
	}
}

func batchIDFromTitle(title string) string {
	title = strings.TrimSpace(title)
	rest, ok := strings.CutPrefix(title, "Batch")
	if !ok {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	for i, r := range rest {
		if r == '：' || r == ':' || r == ' ' || r == '\t' {
			if i == 0 {
				return ""
			}
			return "Batch " + rest[:i]
		}
	}
	return "Batch " + rest
}

func markdownFieldValue(line, key string) (string, bool) {
	prefixes := []string{key + "：", key + ":"}
	for _, prefix := range prefixes {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}
