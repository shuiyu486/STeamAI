package skillcontract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
)

func TestCapabilitiesUseScopedCommandContracts(t *testing.T) {
	capabilities, err := Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Capability{}
	for _, capability := range capabilities {
		byID[capability.ID] = capability
	}
	for _, fixture := range []struct {
		id          string
		policy      string
		currentness string
		expected    string
	}{
		{id: "public-status", policy: commands.BoundaryReadOnly},
		{id: "public-continue-preview", policy: commands.BoundaryCaseLocalApply, currentness: commands.MutationCurrentnessStrictPlan, expected: "-ExpectedContinuePlanSha256"},
		{id: "runtime-control-preview", policy: commands.BoundaryCaseLocalReviewFirst, currentness: commands.MutationCurrentnessStrictPlan, expected: "-ExpectedControlPlanSha256"},
		{id: "runtime-bounded-autonomy-preview", policy: commands.BoundaryCaseLocalApply, currentness: commands.MutationCurrentnessStrictPlan, expected: "-ExpectedProfilePlanSha256"},
	} {
		capability, ok := byID[fixture.id]
		if !ok {
			t.Fatalf("missing capability %s", fixture.id)
		}
		if capability.Policy != fixture.policy || capability.Currentness != fixture.currentness || capability.ExpectedApplyFlag != fixture.expected {
			t.Fatalf("capability %s drifted from scoped command catalog: %+v", fixture.id, capability)
		}
	}
	capabilities[0].Argv[0] = "tampered"
	fresh, err := Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	if fresh[0].Argv[0] == "tampered" {
		t.Fatal("capability catalog returned aliased argv")
	}
}

func TestRenderMachineAppendixIsDeterministicAndBounded(t *testing.T) {
	first, err := RenderMachineAppendix()
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderMachineAppendix()
	if err != nil {
		t.Fatal(err)
	}
	if first != second || strings.Count(first, MachineAppendixStart) != 1 || strings.Count(first, MachineAppendixEnd) != 1 {
		t.Fatalf("machine appendix is not deterministic: %q", first)
	}
	for _, required := range []string{
		`["status"]`,
		`["continue","--lane","<SELECTOR>"]`,
		"Apply binding=`-ExpectedContinuePlanSha256`",
		"Apply binding=`-ExpectedControlPlanSha256`",
		"Apply binding=`-ExpectedProfilePlanSha256`",
		"固定桥之外只允许 `typed-invocation`",
	} {
		if !strings.Contains(first, required) {
			t.Fatalf("machine appendix missing %q: %s", required, first)
		}
	}
	for _, forbidden := range []string{"authorityConfirmed=true", "-Apply\"]"} {
		if strings.Contains(first, forbidden) {
			t.Fatalf("machine appendix exposed forbidden generated contract %q", forbidden)
		}
	}
}

func TestValidatePairRejectsStaleOrIndependentTemplate(t *testing.T) {
	appendix, err := RenderMachineAppendix()
	if err != nil {
		t.Fatal(err)
	}
	canonical := []byte("# STeamAI\n\n机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner。\n\n" + appendix + "\n")
	if err := ValidatePair(canonical, append([]byte{}, canonical...)); err != nil {
		t.Fatal(err)
	}
	stale := []byte(strings.Replace(string(canonical), "-ExpectedContinuePlanSha256", "-ExpectedStalePlanSha256", 1))
	if err := ValidatePair(canonical, stale); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale generated appendix error=%v", err)
	}
	independent := append(append([]byte{}, canonical...), []byte("template-only prose\n")...)
	if err := ValidatePair(canonical, independent); err == nil || !strings.Contains(err.Error(), "not generated from the canonical skill") {
		t.Fatalf("independent template error=%v", err)
	}
}

func TestValidateDocumentRejectsParallelHumanMachineContract(t *testing.T) {
	appendix, err := RenderMachineAppendix()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`runtime -Command status`,
		`-ExpectedContinuePlanSha256`,
		`["runtime", "-Command", invocation.command]`,
	} {
		for _, placement := range []struct {
			name     string
			document string
		}{
			{name: "before-appendix", document: "# STeamAI\n\n机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner。\n\n" + fragment + "\n\n" + appendix + "\n"},
			{name: "after-appendix", document: "# STeamAI\n\n机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner。\n\n" + appendix + "\n\n" + fragment + "\n"},
		} {
			t.Run(placement.name+"/"+fragment, func(t *testing.T) {
				if err := ValidateDocument([]byte(placement.document)); err == nil || !strings.Contains(err.Error(), "duplicates generated machine contract") {
					t.Fatalf("parallel human contract %q error=%v", fragment, err)
				}
			})
		}
	}
}

func TestReplaceMachineAppendixPreservesHumanOwnedTextAndLineEndings(t *testing.T) {
	input := []byte("# human\r\n\r\n机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner。\r\n\r\n" + MachineAppendixStart + "\r\nstale\r\n" + MachineAppendixEnd + "\r\n\r\n## human tail\r\n")
	updated, err := ReplaceMachineAppendix(input)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if !strings.HasPrefix(text, "# human\r\n\r\n") || !strings.HasSuffix(text, "\r\n## human tail\r\n") || strings.Contains(text, "\nstale\n") {
		t.Fatalf("generated update changed human text or line endings: %q", text)
	}
	if err := ValidateDocument(updated); err != nil {
		t.Fatal(err)
	}
}

func TestSynchronizeRejectsParallelHumanContractBeforeWriting(t *testing.T) {
	repo := t.TempDir()
	canonicalPath := filepath.Join(repo, filepath.FromSlash(CanonicalSkillPath))
	if err := os.MkdirAll(filepath.Dir(canonicalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("# human\n\n机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner。\n\nruntime -Command status\n\n" + MachineAppendixStart + "\nstale\n" + MachineAppendixEnd + "\n")
	if err := os.WriteFile(canonicalPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Synchronize(repo); err == nil || !strings.Contains(err.Error(), "duplicates generated machine contract") {
		t.Fatalf("parallel human contract synchronize error=%v", err)
	}
	current, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(original) {
		t.Fatal("failed synchronize changed canonical skill")
	}
	if _, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(ProjectTemplatePath))); !os.IsNotExist(err) {
		t.Fatalf("failed synchronize published project template: %v", err)
	}
}

func TestSynchronizeCopiesCanonicalSkillToDeliveryTemplate(t *testing.T) {
	repo := t.TempDir()
	canonicalPath := filepath.Join(repo, filepath.FromSlash(CanonicalSkillPath))
	if err := os.MkdirAll(filepath.Dir(canonicalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(canonicalPath, []byte("# human\n\n机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner。\n\n"+MachineAppendixStart+"\nstale\n"+MachineAppendixEnd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Synchronize(repo); err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(ProjectTemplatePath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != string(template) {
		t.Fatalf("delivery template differs from canonical skill:\ncanonical=%s\ntemplate=%s", canonical, template)
	}
	if err := ValidatePair(canonical, template); err != nil {
		t.Fatal(err)
	}
}

func TestSynchronizeRollsBackCanonicalWhenTemplateWriteFails(t *testing.T) {
	repo := t.TempDir()
	canonicalPath := filepath.Join(repo, filepath.FromSlash(CanonicalSkillPath))
	templatePath := filepath.Join(repo, filepath.FromSlash(ProjectTemplatePath))
	for _, path := range []string{canonicalPath, templatePath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	originalCanonical := []byte("# canonical\n\n机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner。\n\n" + MachineAppendixStart + "\nstale\n" + MachineAppendixEnd + "\n")
	originalTemplate := []byte("template-before-failure\n")
	if err := os.WriteFile(canonicalPath, originalCanonical, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templatePath, originalTemplate, 0o644); err != nil {
		t.Fatal(err)
	}
	writes := 0
	write := func(path string, data []byte) error {
		writes++
		if writes == 2 {
			return fmt.Errorf("injected template write failure")
		}
		return writeIfChanged(path, data)
	}
	if err := synchronize(repo, write); err == nil || !strings.Contains(err.Error(), "injected template write failure") {
		t.Fatalf("synchronize failure=%v", err)
	}
	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != string(originalCanonical) || string(template) != string(originalTemplate) {
		t.Fatalf("failed synchronize left a partial pair:\ncanonical=%s\ntemplate=%s", canonical, template)
	}
}

func TestSynchronizeLeavesBothTargetsUnchangedWhenTemplateCannotBeWritten(t *testing.T) {
	repo := t.TempDir()
	canonicalPath := filepath.Join(repo, filepath.FromSlash(CanonicalSkillPath))
	if err := os.MkdirAll(filepath.Dir(canonicalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	originalCanonical := []byte("# human\n\n机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner。\n\n" + MachineAppendixStart + "\nstale\n" + MachineAppendixEnd + "\n")
	if err := os.WriteFile(canonicalPath, originalCanonical, 0o644); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(repo, filepath.FromSlash(ProjectTemplatePath))
	if err := os.MkdirAll(templatePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Synchronize(repo); err == nil {
		t.Fatal("synchronize unexpectedly wrote through a directory template target")
	}
	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != string(originalCanonical) {
		t.Fatal("failed synchronize changed canonical skill before template preflight")
	}
	info, err := os.Lstat(templatePath)
	if err != nil || !info.IsDir() {
		t.Fatalf("failed synchronize changed template target: info=%v err=%v", info, err)
	}
}
