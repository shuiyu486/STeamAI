package sessionhost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackMemoryAcceptanceExpectedSanitizationAllowsOnlyBaselineCapturesPaths(t *testing.T) {
	if !packMemoryAcceptanceExpectedSanitization(map[string]int{"capturesPath": 2, "absolutePath": 0}) {
		t.Fatal("expected preserved baseline captures paths to be accepted")
	}
	for _, counts := range []map[string]int{
		{"capturesPath": 1},
		{"capturesPath": 3},
		{"capturesPath": 2, "absolutePath": 1},
	} {
		if packMemoryAcceptanceExpectedSanitization(counts) {
			t.Fatalf("unexpected sanitization accepted: %+v", counts)
		}
	}
}

func TestValidatePackMemoryAcceptanceSanitizationRequiresExactPredecessorMatches(t *testing.T) {
	predecessor := []byte("first captures/doc_archive/** then captures/doc_archive\n")
	if err := validatePackMemoryAcceptanceSanitization(predecessor, append(append([]byte{}, predecessor...), []byte("new checklist\n")...)); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string][]byte{
		"equal-count-substitution": []byte("first captures/doc_archive/** then captures/private-case\n"),
		"reordered":                []byte("first captures/doc_archive then captures/doc_archive/**\n"),
		"added":                    []byte("first captures/doc_archive/** then captures/doc_archive plus captures/private\n"),
		"removed":                  []byte("only captures/doc_archive/**\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validatePackMemoryAcceptanceSanitization(predecessor, raw); err == nil {
				t.Fatal("expected non-predecessor capturesPath rejection")
			}
		})
	}
}

func TestCopyPackMemoryAcceptanceRepoUsesGoNativeAllowlist(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	packRoot := "packs/" + liveAcceptancePack
	files := map[string]string{
		"go.mod":                                 "module github.com/shuiyu486/re-context-kits\n",
		"rekit/tests/catalog.json":               "{}\n",
		"rekit/templates/case-shim/SKILL.md":     "thin shim\n",
		".claude/skills/rekit/SKILL.md":          "canonical skill\n",
		packRoot + "/manifest.yml":               "name: " + liveAcceptancePack + "\n",
		packRoot + "/promote-candidates/drop.md": "mutable candidate\n",
		"common/policies/example.md":             "policy\n",
		"docs/unrelated.md":                      "unrelated\n",
	}
	for rel, content := range files {
		path := filepath.Join(source, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest, manifestSHA256, err := copyPackMemoryAcceptanceRepo(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifestSHA256) != 64 || len(manifest.Files) != 6 {
		t.Fatalf("copy manifest sha=%q files=%+v", manifestSHA256, manifest.Files)
	}
	if err := verifyPackMemoryAcceptanceCopyManifest(target, manifest); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{packRoot + "/promote-candidates/drop.md", "docs/unrelated.md"} {
		if _, err := os.Lstat(filepath.Join(target, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("excluded path was copied: %s: %v", rel, err)
		}
	}
}

func TestVerifyPackMemoryAcceptanceCopyManifestRejectsExtraFile(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module github.com/shuiyu486/re-context-kits\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, _, err := copyPackMemoryAcceptanceRepo(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "unexpected.txt"), []byte("unexpected\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackMemoryAcceptanceCopyManifest(target, manifest); err == nil || !strings.Contains(err.Error(), "outside copy manifest") {
		t.Fatalf("extra-file verification error=%v", err)
	}
}

func TestWritePackMemoryLiveAcceptanceReceiptRedactsLocalPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := PackMemoryLiveAcceptanceReceipt{
		SchemaVersion:   1,
		Kind:            packMemoryLiveAcceptanceKind,
		SourceRepoRoot:  `C:\\private\\source`,
		IsolatedKitRoot: `C:\\private\\isolated-kit`,
		ChildHostPath:   `C:\\private\\rekit-host.exe`,
		Claude:          LiveAcceptanceClaude{Path: `C:\\private\\claude.exe`, Publisher: "Anthropic, PBC", Version: "test", SHA256: strings.Repeat("a", 64)},
		Child: PackMemoryLiveAcceptanceChildResult{
			Kind:         packMemoryLiveAcceptanceChildKind,
			ProducerCase: `C:\\private\\isolated-kit\\producer`,
			ConsumerCase: `C:\\private\\isolated-kit\\consumer`,
		},
	}
	if err := WritePackMemoryLiveAcceptanceReceipt(path, receipt); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private", "claude.exe", "sourceRepoRoot", "isolatedKitRoot", "childHostPath", "producerCase", "consumerCase"} {
		if strings.Contains(string(data), private) {
			t.Fatalf("external receipt retained local path data %q: %s", private, data)
		}
	}
}

func TestRequireReviewerBindingRejectsPacketPromptAndOperatorDrift(t *testing.T) {
	caseRoot := t.TempDir()
	packetPath := filepath.Join(caseRoot, ".rekit", "reviews", "packet-a", "packet.json")
	promptPath := filepath.Join(caseRoot, ".rekit", "reviews", "packet-a", "prompts", "shard-1.md")
	for path, data := range map[string][]byte{
		packetPath: []byte("packet\n"),
		promptPath: []byte("prompt\n"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	opt := Options{Target: caseRoot, reviewerBinding: &reviewerBinding{
		PacketID: "packet-a", PacketPath: packetPath, PacketSHA256: bytesSHA256([]byte("packet\n")),
		Lane: "feature-mission", ShardID: "shard-1", DispatchPromptPath: promptPath,
		DispatchPromptSHA256: bytesSHA256([]byte("prompt\n")),
	}}
	plan := boundReviewerStepPlan{
		PacketID: "packet-a", PacketPath: packetPath, TargetLane: "feature-mission", ShardID: "shard-1",
		ExternalHandoff: &reviewerExternalHandoff{DispatchPromptPath: promptPath, DispatchPromptSHA256: opt.reviewerBinding.DispatchPromptSHA256},
	}
	if err := requireReviewerBinding(opt, plan); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*boundReviewerStepPlan){
		"packet": func(p *boundReviewerStepPlan) { p.PacketID = "packet-b" },
		"lane":   func(p *boundReviewerStepPlan) { p.TargetLane = "other-lane" },
		"shard":  func(p *boundReviewerStepPlan) { p.ShardID = "shard-2" },
		"prompt": func(p *boundReviewerStepPlan) { p.ExternalHandoff.DispatchPromptSHA256 = strings.Repeat("b", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			drifted := plan
			handoff := *plan.ExternalHandoff
			drifted.ExternalHandoff = &handoff
			mutate(&drifted)
			if err := requireReviewerBinding(opt, drifted); err == nil {
				t.Fatal("expected reviewer binding drift rejection")
			}
		})
	}
	if err := os.WriteFile(promptPath, []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireReviewerBinding(opt, plan); err == nil || !strings.Contains(err.Error(), "prompt sha256 changed") {
		t.Fatalf("prompt byte drift error=%v", err)
	}
}

func TestPackMemoryAcceptanceReviewerEvidenceIsCaseRelative(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	diffPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate", "diffs", "combined.diff")
	rel, err := filepath.Rel(caseRoot, diffPath)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.ToSlash(rel) != ".rekit/reviews/candidate/diffs/combined.diff" {
		t.Fatalf("reviewer evidence ref=%q", rel)
	}
}

func TestPackMemoryAcceptanceReviewerLaunchesCountsRealFailedLaunch(t *testing.T) {
	result := Result{Sessions: []Session{
		{Started: true, SessionKind: "reviewer", SessionID: "reviewer-a", RunLaunchOrdinal: 1, Outcome: "replacement-requested"},
		{Started: true, Recovered: true, SessionKind: "reviewer", SessionID: "reviewer-a", Outcome: "returned-recovered"},
		{Started: true, SessionKind: "member", SessionID: "member-a", RunLaunchOrdinal: 2, Outcome: "returned"},
	}}
	if launches := packMemoryAcceptanceReviewerLaunches(result); launches != 1 {
		t.Fatalf("reviewer launches=%d sessions=%+v", launches, result.Sessions)
	}
}

func TestPackMemoryAcceptanceFailuresAreBoundedAndPathRedacted(t *testing.T) {
	root := `C:\Users\Private\Isolated-Kit`
	claude := root + `\bin\claude.exe`
	childHost := root + `\bin\rekit-host.exe`
	spec := PackMemoryLiveAcceptanceChildSpec{
		IsolatedKitRoot: root,
		ClaudePath:      claude,
		ChildHostPath:   childHost,
	}
	detail := "launch failed at C:/USERS/PRIVATE/ISOLATED-KIT/.rh07-cases/producer " + strings.Repeat("x", 2048)
	host := Result{Sessions: []Session{{
		AttemptGeneration: 2,
		Outcome:           "launch-failed",
		Failure: &FailureDiagnosis{
			Code:   "claude-spawn-failed",
			Detail: detail,
		},
		Diagnostics: []string{
			"claude: c:/users/private/isolated-kit/BIN/CLAUDE.EXE " + strings.Repeat("y", 2048),
			`host: C:/Users/Private/Isolated-Kit/bin/rekit-host.exe`,
		},
	}}}
	failures := appendPackMemoryAcceptanceHostFailures(
		nil,
		"producer",
		host,
		spec,
	)
	if len(failures) != 1 ||
		failures[0].Phase != "producer" ||
		failures[0].AttemptGeneration != 2 ||
		len(failures[0].Failure.Detail) > 1024 ||
		len(failures[0].Diagnostics[0]) > 1024 ||
		!strings.Contains(failures[0].Failure.Detail, "<isolated-kit>") ||
		!strings.Contains(failures[0].Diagnostics[0], "<claude-executable>") ||
		!strings.Contains(failures[0].Diagnostics[1], "<child-host>") {
		t.Fatalf("bounded failure projection = %+v", failures)
	}
	for _, projected := range append(
		[]string{failures[0].Failure.Detail},
		failures[0].Diagnostics...,
	) {
		lower := strings.ToLower(strings.ReplaceAll(projected, "/", `\`))
		if strings.Contains(lower, strings.ToLower(root)) ||
			strings.Contains(lower, "claude.exe") ||
			strings.Contains(lower, "rekit-host.exe") {
			t.Fatalf("failure projection retained a Windows path variant: %q", projected)
		}
	}
}

func TestDecodePackMemoryAcceptanceChildResultPreservesFailureReceipt(t *testing.T) {
	input := `{"schemaVersion":1,"kind":"rekit-pack-memory-live-acceptance-child-result","passed":false,"manualResultWrites":0,"cleanup":"owned-by-isolated-kit","boundary":["failed before completion"]}`
	result, err := decodePackMemoryAcceptanceChildResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || result.Kind != packMemoryLiveAcceptanceChildKind || result.Cleanup != "owned-by-isolated-kit" || len(result.Boundary) != 1 {
		t.Fatalf("decoded failure receipt=%+v", result)
	}
	if _, err := decodePackMemoryAcceptanceChildResult(input + "\n{}"); err == nil || !strings.Contains(err.Error(), "trailing output") {
		t.Fatalf("trailing child output error=%v", err)
	}
}
