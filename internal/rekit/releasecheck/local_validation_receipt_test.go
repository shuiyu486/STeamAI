package releasecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLocalValidationReceiptBindsDirectImplementationCommit(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.name", "rekit-test")
	runPostPushGit(t, repo, "config", "user.email", "rekit-test@example.invalid")
	runPostPushGit(t, repo, "config", "core.autocrlf", "true")
	writePostPushTestFile(t, repo, ".gitattributes", "*.md text eol=crlf\n*.go -text\n")
	writePostPushTestFile(t, repo, "internal/rekit/my fixture.go", "package rekit\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 816：previous\n\n状态：已完成 previous。\n\n目标：previous。\n\n验证结果：release-run 以 7/7 通过。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 816 previous.\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "update-index", "--chmod=+x", "internal/rekit/my fixture.go")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 816")
	baseline := strings.TrimSpace(runPostPushGit(t, repo, "rev-parse", "HEAD"))

	writePostPushTestFile(t, repo, "internal/rekit/my fixture.go", "package rekit\n\nconst batch = 817\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成 fixture。\n\n目标：fixture。\n\n验证结果：machine receipt owns readiness。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817 receipt closure.\n")
	latest := latestBatchSummary(repo)
	input := readyLocalValidationReceiptInput(t, repo, latest.BatchID)
	published, err := PublishLocalValidationReceipt(repo, input)
	if err != nil || !published.Present || published.Ready || published.State != "recorded-for-implementation-commit" || published.Receipt == nil || published.Receipt.BaselineHead != baseline || len(published.Receipt.Artifacts) != 3 {
		t.Fatalf("published receipt=%+v err=%v", published, err)
	}
	if published.Receipt.Artifacts[2].Path != "internal/rekit/my fixture.go" || published.Receipt.Artifacts[2].Mode != "100755" {
		t.Fatalf("tracked executable mode was not preserved: %+v", published.Receipt.Artifacts)
	}
	if got := strings.TrimSpace(runPostPushGit(t, repo, "status", "--short")); !strings.Contains(got, "CHANGELOG.md") || strings.Contains(got, "local-validation-v3.json") {
		t.Fatalf("Git-local receipt polluted working tree: %q", got)
	}
	beforeCommit := InspectLocalValidationReceipt(repo, numberedLocalValidationReceiptSubject(latest.BatchID))
	if beforeCommit.Ready || beforeCommit.State != "recorded-for-implementation-commit" {
		t.Fatalf("pre-commit receipt should route the exact direct implementation commit: %+v", beforeCommit)
	}
	latestBeforeCommit := releaseHandoffLatestBatchWithPostPushReceipt(repo, latest, ReleaseHandoffActiveRoute{})
	if latestBeforeCommit.Handoff.LocalValidationReceipt == nil || latestBeforeCommit.Handoff.LocalValidationReceipt.State != "recorded-for-implementation-commit" || !strings.Contains(latestBeforeCommit.Handoff.NextAction, "direct implementation commit") || strings.Contains(latestBeforeCommit.Handoff.NextAction, "release minimum") {
		t.Fatalf("numbered-batch pre-commit receipt did not publish direct-commit guidance: %+v", latestBeforeCommit.Handoff)
	}

	validatedBytes, err := os.ReadFile(filepath.Join(repo, "docs", "batch-plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 817")
	for _, rel := range []string{"docs/batch-plan.md", "CHANGELOG.md"} {
		if err := os.Remove(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	runPostPushGit(t, repo, "checkout-index", "--", "docs/batch-plan.md", "CHANGELOG.md")
	checkoutBytes, err := os.ReadFile(filepath.Join(repo, "docs", "batch-plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(checkoutBytes) == string(validatedBytes) || !strings.Contains(string(checkoutBytes), "\r\n") {
		t.Fatalf("fixture did not rematerialize validated LF bytes as CRLF checkout bytes")
	}
	ready := InspectLocalValidationReceipt(repo, numberedLocalValidationReceiptSubject(latestBatchSummary(repo).BatchID))
	if !ready.Ready || ready.State != "validated-implementation-commit" || ready.ValidatedHead == "" {
		t.Fatalf("direct implementation commit did not validate: %+v", ready)
	}
	goPath := filepath.Join(repo, "internal", "rekit", "my fixture.go")
	goBytes, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goPath, []byte(strings.ReplaceAll(string(goBytes), "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	binaryDrift := InspectLocalValidationReceipt(repo, numberedLocalValidationReceiptSubject(latestBatchSummary(repo).BatchID))
	if binaryDrift.Ready || binaryDrift.State != "artifact-content-mismatch" {
		t.Fatalf("-text artifact line-ending drift should fail closed: %+v", binaryDrift)
	}
	if err := os.WriteFile(goPath, goBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"release-check -Format json recorded",
		"status handoff recorded",
		"packs inventory recorded",
		"doctor validation recorded",
		"go test ./... recorded",
		"go vet ./... recorded",
		"git diff --check recorded",
	} {
		if !slices.Contains(ready.Evidence, want) {
			t.Fatalf("validated receipt evidence missing %q: %+v", want, ready.Evidence)
		}
	}

	writePostPushTestFile(t, repo, "internal/rekit/extra.go", "package rekit\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Extra commit")
	extra := InspectLocalValidationReceipt(repo, numberedLocalValidationReceiptSubject(latestBatchSummary(repo).BatchID))
	if extra.Ready || extra.State != "non-direct-implementation-commit" {
		t.Fatalf("extra commit should invalidate receipt: %+v", extra)
	}
}

func TestLocalValidationReceiptRejectsPostRunWorktreeByteDriftWithSameBlob(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.name", "rekit-test")
	runPostPushGit(t, repo, "config", "user.email", "rekit-test@example.invalid")
	runPostPushGit(t, repo, "config", "filter.receipt.clean", "grep -v ^DROP:")
	writePostPushTestFile(t, repo, ".gitattributes", "internal/rekit/fixture.go filter=receipt\n")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 816：previous\n\n状态：已完成。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 816")

	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817.\n")
	latest := latestBatchSummary(repo)
	if _, err := PublishLocalValidationReceipt(repo, readyLocalValidationReceiptInput(t, repo, latest.BatchID)); err != nil {
		t.Fatal(err)
	}
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\nDROP: unvalidated worktree bytes\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Commit filtered Batch 817")
	inspection := InspectLocalValidationReceipt(repo, numberedLocalValidationReceiptSubject(latestBatchSummary(repo).BatchID))
	if inspection.Ready || inspection.State != "artifact-content-mismatch" || !strings.Contains(strings.Join(inspection.Warnings, " "), "working-tree artifact bytes changed") {
		t.Fatalf("same blob with changed worktree bytes should fail closed: %+v", inspection)
	}
}

func TestLocalValidationReceiptPromotesExactSameBatchRepair(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	if err := os.Remove(filepath.Join(repo, "docs", "real-usage-hardening-roadmap.md")); err != nil {
		t.Fatal(err)
	}
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.name", "rekit-test")
	runPostPushGit(t, repo, "config", "user.email", "rekit-test@example.invalid")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成 fixture。\n\n目标：fixture。\n\n验证结果：machine receipt owns readiness。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817 receipt closure.\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 817")

	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\nconst receiptRepair = true\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成 fixture repair。\n\n目标：fixture。\n\n验证结果：machine receipt owns readiness。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817 receipt closure and repair.\n")
	latest := latestBatchSummary(repo)
	if _, err := PublishLocalValidationReceipt(repo, readyLocalValidationReceiptInput(t, repo, latest.BatchID)); err != nil {
		t.Fatal(err)
	}
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Repair Batch 817 receipt")
	head := strings.TrimSpace(runPostPushGit(t, repo, "rev-parse", "HEAD"))
	runPostPushGit(t, repo, "update-ref", "refs/remotes/origin/main", head)

	latest = releaseHandoffLatestBatchWithPostPushReceipt(repo, latestBatchSummary(repo), ReleaseHandoffActiveRoute{})
	if latest.Handoff.LocalValidationReceipt == nil || !latest.Handoff.LocalValidationReceipt.Ready {
		t.Fatalf("same-batch repair receipt did not validate: %+v", latest.Handoff.LocalValidationReceipt)
	}
	latest.Handoff.LocalValidationReceipt.ValidatedHead = ""
	withoutExactReceipt := releaseHandoffPostPushReceiptFor(repo, latest, ReleaseHandoffActiveRoute{}, defaultReleaseHandoffGitCommand)
	if withoutExactReceipt.Ready || withoutExactReceipt.State != "ambiguous-batch-transition" {
		t.Fatalf("same-batch repair without exact validated HEAD should fail closed: %+v", withoutExactReceipt)
	}

	handoff, err := BuildProjectHandoff(repo)
	if err != nil {
		t.Fatal(err)
	}
	if handoff.LatestBatch.Handoff.LocalValidationReceipt == nil || !handoff.LatestBatch.Handoff.LocalValidationReceipt.Ready || handoff.LatestBatch.Handoff.PostPushReceipt == nil || !handoff.LatestBatch.Handoff.PostPushReceipt.Ready || handoff.LatestBatch.Handoff.ReleaseInspectionCadence.State != "complete" || handoff.NextBatchSelectionPackage == nil || !handoff.NextBatchSelectionPackage.Ready {
		t.Fatalf("exact same-batch repair did not complete handoff: latest=%+v package=%+v active=%+v ready=%t warnings=%+v", handoff.LatestBatch.Handoff, handoff.NextBatchSelectionPackage, handoff.ActiveRoute, handoff.Ready, handoff.Warnings)
	}
}

func TestLocalValidationReceiptPromotesPostPushNextBatchWithoutProseEvidence(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	if err := os.Remove(filepath.Join(repo, "docs", "real-usage-hardening-roadmap.md")); err != nil {
		t.Fatal(err)
	}
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.name", "rekit-test")
	runPostPushGit(t, repo, "config", "user.email", "rekit-test@example.invalid")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 816：previous\n\n状态：已完成 previous。\n\n目标：previous。\n\n验证结果：release-run 以 7/7 通过。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 816.\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 816")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成 fixture。\n\n目标：fixture。\n\n验证结果：machine receipt owns readiness。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817.\n")
	latest := latestBatchSummary(repo)
	if latest.Handoff.LocalValidationReady || latest.Handoff.ReleaseCheckReady {
		t.Fatalf("fixture prose unexpectedly satisfied legacy validation parsing: %+v", latest.Handoff)
	}
	withoutReceipt := releaseHandoffLatestBatchWithPostPushReceipt(repo, latest, ReleaseHandoffActiveRoute{})
	if withoutReceipt.Handoff.LocalValidationReady || withoutReceipt.Handoff.ReleaseInspectionCadence.State != "implementation-pending" {
		t.Fatalf("missing receipt should remain pending: %+v", withoutReceipt.Handoff)
	}
	if _, err := PublishLocalValidationReceipt(repo, readyLocalValidationReceiptInput(t, repo, latest.BatchID)); err != nil {
		t.Fatal(err)
	}
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 817")
	head := strings.TrimSpace(runPostPushGit(t, repo, "rev-parse", "HEAD"))
	runPostPushGit(t, repo, "update-ref", "refs/remotes/origin/main", head)

	handoff, err := BuildProjectHandoff(repo)
	if err != nil {
		t.Fatal(err)
	}
	latest = handoff.LatestBatch
	if !latest.Handoff.LocalValidationReady || !latest.Handoff.ReleaseCheckReady || latest.Handoff.LocalValidationReceipt == nil || !latest.Handoff.LocalValidationReceipt.Ready || latest.Handoff.PostPushReceipt == nil || !latest.Handoff.PostPushReceipt.Ready || latest.Handoff.ReleaseInspectionCadence.State != "complete" || handoff.NextBatchSelectionPackage == nil || !handoff.NextBatchSelectionPackage.Ready {
		t.Fatalf("machine receipt did not promote post-push next-batch handoff: latest=%+v package=%+v", latest.Handoff, handoff.NextBatchSelectionPackage)
	}
}

func TestLocalValidationReceiptRequiredBatchRejectsProseFallback(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.email", "receipt@example.invalid")
	runPostPushGit(t, repo, "config", "user.name", "Receipt Test")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 816：prior\n\n状态：已完成。\n\n验证结果：release-run 7/7通过；release-check ready=true。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 816")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成。\n\n验证结果：release-run 7/7通过；release-check ready=true。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817.\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 817")
	head := strings.TrimSpace(runPostPushGit(t, repo, "rev-parse", "HEAD"))
	runPostPushGit(t, repo, "update-ref", "refs/remotes/origin/main", head)
	latest := latestBatchSummary(repo)
	latest.Handoff.LocalValidationReady = true
	latest.Handoff.ReleaseCheckReady = true
	latest.Handoff.ReleaseInspectionCadence.State = "complete"
	latest.Handoff.ReleaseInspectionCadence.ImplementationCommitReady = true
	result := releaseHandoffLatestBatchWithPostPushReceipt(repo, latest, ReleaseHandoffActiveRoute{})
	if result.Handoff.LocalValidationReady || result.Handoff.ReleaseCheckReady || result.Handoff.ReleaseInspectionCadence.State != "implementation-pending" || result.Handoff.ReleaseInspectionCadence.ImplementationCommitReady {
		t.Fatalf("Batch 817 missing receipt must override prose-complete readiness: %+v", result.Handoff)
	}
}

func TestLocalValidationReceiptRejectsDriftAndTamper(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.name", "rekit-test")
	runPostPushGit(t, repo, "config", "user.email", "rekit-test@example.invalid")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 816：previous\n\n状态：已完成 previous。\n\n目标：previous。\n\n验证结果：release-run 以 7/7 通过。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 816")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成 fixture。\n\n目标：fixture。\n\n验证结果：machine receipt owns readiness。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817.\n")
	latest := latestBatchSummary(repo)
	if _, err := PublishLocalValidationReceipt(repo, readyLocalValidationReceiptInput(t, repo, latest.BatchID)); err != nil {
		t.Fatal(err)
	}
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\nconst drift = true\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Drifted Batch 817")
	drift := InspectLocalValidationReceipt(repo, numberedLocalValidationReceiptSubject(latestBatchSummary(repo).BatchID))
	if drift.Ready || drift.State != "artifact-content-mismatch" {
		t.Fatalf("post-validation drift should fail closed: %+v", drift)
	}

	path, err := localValidationReceiptPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("{}\n")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	tampered := InspectLocalValidationReceipt(repo, numberedLocalValidationReceiptSubject(latestBatchSummary(repo).BatchID))
	if tampered.Ready || tampered.State != "invalid-contract" {
		t.Fatalf("tampered receipt should fail closed: %+v", tampered)
	}
}

func numberedLocalValidationReceiptSubject(batchID string) LocalValidationReceiptSubject {
	return LocalValidationReceiptSubject{Kind: LocalValidationReceiptSubjectNumberedBatch, BatchID: batchID}
}

func readyLocalValidationReceiptInput(t *testing.T, repo, batchID string) LocalValidationReceiptInput {
	t.Helper()
	return readyLocalValidationReceiptSubjectInput(t, repo, LocalValidationReceiptSubject{
		Kind:    LocalValidationReceiptSubjectNumberedBatch,
		BatchID: batchID,
	})
}

func readyLocalValidationReceiptSubjectInput(t *testing.T, repo string, subject LocalValidationReceiptSubject) LocalValidationReceiptInput {
	t.Helper()
	cat, err := loadCatalog(repo)
	if err != nil {
		t.Fatal(err)
	}
	profile := gateProfile(catalogGateSteps(repo, cat.RecommendedMinimum))
	steps := make([]LocalValidationReceiptStep, 0, len(profile.Steps))
	for index, step := range profile.Steps {
		steps = append(steps, LocalValidationReceiptStep{Index: index + 1, Command: step.Command, Status: "passed", ExitCode: 0, Attempts: 1})
	}
	snapshot, err := CaptureLocalValidationSnapshot(repo)
	if err != nil {
		t.Fatal(err)
	}
	return LocalValidationReceiptInput{Subject: subject, GateProfile: profile.Name, Passed: len(steps), ReleaseCheckReady: true, Steps: steps, Snapshot: snapshot}
}

func TestLocalValidationReceiptV2RemainsNumberedBatchOnly(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.name", "rekit-test")
	runPostPushGit(t, repo, "config", "user.email", "rekit-test@example.invalid")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 816：previous\n\n状态：已完成。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 816")

	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817.\n")
	input := readyLocalValidationReceiptInput(t, repo, "Batch 817")
	published, err := PublishLocalValidationReceipt(repo, input)
	if err != nil || published.Receipt == nil {
		t.Fatal(err)
	}
	legacy := localValidationReceiptV2{
		SchemaVersion: localValidationReceiptLegacySchemaVersion,
		Kind:          localValidationReceiptKind, BatchID: "Batch 817",
		BaselineHead: published.Receipt.BaselineHead, GateProfile: published.Receipt.GateProfile,
		StepCount: published.Receipt.StepCount, Passed: published.Receipt.Passed,
		Failed: published.Receipt.Failed, Skipped: published.Receipt.Skipped,
		ReleaseCheckReady: published.Receipt.ReleaseCheckReady, CreatedAt: published.Receipt.CreatedAt,
		Steps: published.Receipt.Steps, Artifacts: published.Receipt.Artifacts,
		Boundary: published.Receipt.Boundary,
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	v3Path, err := localValidationReceiptPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(v3Path); err != nil {
		t.Fatal(err)
	}
	legacyPath, err := localValidationLegacyReceiptPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	numberedSubject := LocalValidationReceiptSubject{Kind: LocalValidationReceiptSubjectNumberedBatch, BatchID: "Batch 817"}
	numbered := InspectLocalValidationReceipt(repo, numberedSubject)
	if numbered.State != "recorded-for-implementation-commit" || numbered.Receipt == nil || numbered.Receipt.SchemaVersion != 2 {
		t.Fatalf("legacy numbered-batch receipt was not compatibly decoded: %+v", numbered)
	}
	if err := os.WriteFile(v3Path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	wrongPath := InspectLocalValidationReceipt(repo, numberedSubject)
	if wrongPath.State != "invalid-contract" || wrongPath.Receipt != nil {
		t.Fatalf("v2 receipt on the v3 path was accepted or fell back: %+v", wrongPath)
	}
	if err := os.Remove(v3Path); err != nil {
		t.Fatal(err)
	}
	active := InspectLocalValidationReceipt(repo, LocalValidationReceiptSubject{
		Kind: LocalValidationReceiptSubjectActiveRoute, Route: "route-v1", CurrentBatch: "route closure",
		State: "completed", ExclusiveClaim: "route closure", NextBatch: "none",
	})
	if active.Ready || active.State != "stale-or-invalid" || active.Receipt == nil || active.Receipt.SchemaVersion != 2 {
		t.Fatalf("legacy numbered-batch receipt was accepted for an active route: %+v", active)
	}
}

func TestLocalValidationReceiptBindsCompletedActiveRouteDirectCommit(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.name", "rekit-test")
	runPostPushGit(t, repo, "config", "user.email", "rekit-test@example.invalid")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Baseline active route")

	setReleaseHandoffActiveRouteFixture(t, repo, "completed", "路线收口", "无；仅在全部未取消项有证据后关闭路线")
	writePostPushTestFile(t, repo, "internal/rekit/route_receipt_fixture.go", "package rekit\n\nconst activeRouteReceipt = true\n")
	changelogPath := filepath.Join(repo, "CHANGELOG.md")
	changelog, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changelogPath, append(changelog, []byte("\n- active route receipt fixture.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	activeRoute := releaseHandoffActiveRoute(repo)
	subject, ok := LocalValidationReceiptSubjectFor(ReleaseHandoff{
		ActiveRoute: activeRoute,
		LatestBatch: latestBatchSummary(repo),
	})
	if !ok || subject.Kind != LocalValidationReceiptSubjectActiveRoute {
		t.Fatalf("completed active route did not select an active-route receipt subject: route=%+v subject=%+v", activeRoute, subject)
	}
	published, err := PublishLocalValidationReceipt(repo, readyLocalValidationReceiptSubjectInput(t, repo, subject))
	if err != nil || published.Receipt == nil || published.Receipt.SchemaVersion != 3 || published.Receipt.Subject == nil || *published.Receipt.Subject != subject {
		t.Fatalf("active-route receipt publication failed: receipt=%+v err=%v", published, err)
	}
	preCommit, err := BuildProjectHandoff(repo)
	if err != nil {
		t.Fatal(err)
	}
	if preCommit.ActiveRoute.LocalValidationReceipt == nil || preCommit.ActiveRoute.LocalValidationReceipt.State != "recorded-for-implementation-commit" || preCommit.ActiveRoute.CurrentAction == nil || preCommit.ActiveRoute.CurrentAction.ActionID != "active-route-implementation-commit" {
		t.Fatalf("recorded active-route receipt did not route its exact direct commit: %+v", preCommit.ActiveRoute)
	}
	if preCommit.LatestBatch.Handoff.LocalValidationReceipt != nil || preCommit.LatestBatch.Handoff.PostPushReceipt != nil {
		t.Fatalf("active-route pre-commit receipt leaked into latest numbered batch: %+v", preCommit.LatestBatch.Handoff)
	}

	for name, drifted := range map[string]LocalValidationReceiptSubject{
		"route":         {Kind: subject.Kind, Route: subject.Route + "-drift", CurrentBatch: subject.CurrentBatch, State: subject.State, ExclusiveClaim: subject.ExclusiveClaim, NextBatch: subject.NextBatch},
		"current-batch": {Kind: subject.Kind, Route: subject.Route, CurrentBatch: subject.CurrentBatch + " drift", State: subject.State, ExclusiveClaim: subject.ExclusiveClaim, NextBatch: subject.NextBatch},
		"claim":         {Kind: subject.Kind, Route: subject.Route, CurrentBatch: subject.CurrentBatch, State: subject.State, ExclusiveClaim: subject.ExclusiveClaim + "-drift", NextBatch: subject.NextBatch},
		"next":          {Kind: subject.Kind, Route: subject.Route, CurrentBatch: subject.CurrentBatch, State: subject.State, ExclusiveClaim: subject.ExclusiveClaim, NextBatch: subject.NextBatch + " drift"},
	} {
		t.Run(name+"-drift", func(t *testing.T) {
			inspection := InspectLocalValidationReceipt(repo, drifted)
			if inspection.Ready || inspection.State != "stale-or-invalid" {
				t.Fatalf("active-route subject drift was accepted: %+v", inspection)
			}
		})
	}
	legacyLatest := latestBatchSummary(repo)
	legacyLatest.BatchID = "Batch 816"
	if _, ok := LocalValidationReceiptSubjectFor(ReleaseHandoff{LatestBatch: legacyLatest}); ok {
		t.Fatal("pre-v3 numbered batch selected a machine validation receipt subject")
	}
	conflicted := activeRoute
	conflicted.ProjectionConsistent = false
	if _, ok := LocalValidationReceiptSubjectFor(ReleaseHandoff{ActiveRoute: conflicted, LatestBatch: latestBatchSummary(repo)}); ok {
		t.Fatal("projection-inconsistent active route selected a validation subject")
	}
	blocked := activeRoute
	blocked.State = "blocked"
	if _, ok := LocalValidationReceiptSubjectFor(ReleaseHandoff{ActiveRoute: blocked, LatestBatch: latestBatchSummary(repo)}); ok {
		t.Fatal("non-completed active route selected a validation subject")
	}

	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete active route")
	head := strings.TrimSpace(runPostPushGit(t, repo, "rev-parse", "HEAD"))
	runPostPushGit(t, repo, "update-ref", "refs/remotes/origin/main", head)
	handoff, err := BuildProjectHandoff(repo)
	if err != nil {
		t.Fatal(err)
	}
	validation := handoff.ActiveRoute.LocalValidationReceipt
	postPush := handoff.ActiveRoute.PostPushReceipt
	if validation == nil || !validation.Ready || validation.Receipt == nil || validation.Receipt.Subject == nil || validation.Receipt.Subject.Kind != LocalValidationReceiptSubjectActiveRoute {
		t.Fatalf("active-route direct commit did not validate: %+v", validation)
	}
	if postPush == nil || !postPush.Ready || postPush.Subject == nil || postPush.Subject.Kind != LocalValidationReceiptSubjectActiveRoute || postPush.ParentBatchID != postPush.HeadBatchID {
		t.Fatalf("active-route post-push receipt fell back to a numbered batch transition: %+v", postPush)
	}
	if handoff.LatestBatch.Handoff.LocalValidationReceipt != nil || handoff.LatestBatch.Handoff.PostPushReceipt != nil || slices.Contains(handoff.LatestBatch.Handoff.CommitRefs, head) {
		t.Fatalf("active-route post-push evidence leaked into latest numbered batch: %+v", handoff.LatestBatch.Handoff)
	}
	if handoff.LatestBatch.Handoff.NextAction != "" || handoff.LatestBatch.Handoff.ReleaseInspectionCadence.NextAction != "" {
		t.Fatalf("completed active route exposed conflicting latest-batch guidance: %+v", handoff.LatestBatch.Handoff)
	}
	wantNextAction := releaseHandoffCompletedRouteAction(activeRoute).Command
	if handoff.ActiveRoute.CurrentAction == nil || handoff.ActiveRoute.CurrentAction.Command != wantNextAction {
		t.Fatalf("completed route without a next batch lost its route-owned next action: %+v", handoff.ActiveRoute)
	}
}

func TestLocalValidationReceiptRejectsArtifactDriftDuringRun(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	runPostPushGit(t, repo, "init", "-b", "main")
	runPostPushGit(t, repo, "config", "user.email", "receipt@example.invalid")
	runPostPushGit(t, repo, "config", "user.name", "Receipt Test")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817.\n")
	runPostPushGit(t, repo, "add", ".")
	runPostPushGit(t, repo, "commit", "-m", "Complete Batch 816")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\n")
	writePostPushTestFile(t, repo, "docs/batch-plan.md", "### Batch 817：receipt closure\n\n状态：已完成 fixture。\n\n验证结果：machine receipt。\n")
	writePostPushTestFile(t, repo, "CHANGELOG.md", "# Changelog\n\n- Batch 817 changed.\n")
	input := readyLocalValidationReceiptInput(t, repo, "Batch 817")
	writePostPushTestFile(t, repo, "internal/rekit/fixture.go", "package rekit\n\nconst batch = 817\nconst drift = true\n")
	if _, err := PublishLocalValidationReceipt(repo, input); err == nil || !strings.Contains(err.Error(), "changed while release-run") {
		t.Fatalf("artifact drift should reject publication: %v", err)
	}
}

func TestLocalValidationReceiptPathUsesGitMetadata(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	runPostPushGit(t, repo, "init", "-b", "main")
	path, err := localValidationReceiptPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(repo, ".git")
	rel, err := filepath.Rel(gitDir, path)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.Base(path) != "local-validation-v3.json" {
		t.Fatalf("typed receipt path %q is not versioned Git-local metadata", path)
	}
	legacyPath, err := localValidationLegacyReceiptPath(repo)
	if err != nil || filepath.Base(legacyPath) != "local-validation-v2.json" || filepath.Dir(legacyPath) != filepath.Dir(path) {
		t.Fatalf("legacy receipt path %q is not a sibling Git-local metadata path: %v", legacyPath, err)
	}
}
