package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
)

type skeletonPackSmokeFixture struct {
	pack                   string
	taskType               string
	items                  string
	expectedRoute          string
	expectedOutputContract string
	laneName               string
	expectedLane           string
}

var skeletonPackSmokeFixtures = []skeletonPackSmokeFixture{
	{pack: "android-native", taskType: "jni-triage", items: "app-a,library-b", expectedRoute: "android-native:native-analysis", expectedOutputContract: "jni_symbol_ref", laneName: "jni", expectedLane: "native-analysis-jni"},
	{pack: "ctf", taskType: "pwn-analysis", items: "challenge-a,artifact-b", expectedRoute: "ctf:challenge-analysis", expectedOutputContract: "challenge_ref", laneName: "pwn", expectedLane: "challenge-analysis-pwn"},
	{pack: "malware-analysis", taskType: "static-triage", items: "sample-a,behavior-b", expectedRoute: "malware-analysis:sample-analysis", expectedOutputContract: "sample_ref", laneName: "triage", expectedLane: "sample-analysis-triage"},
	{pack: "ollvm", taskType: "control-flow-triage", items: "function-a,region-b", expectedRoute: "ollvm:obfuscation-analysis", expectedOutputContract: "function_ref", laneName: "cfg", expectedLane: "obfuscation-analysis-cfg"},
	{pack: "unpack-pe", taskType: "loader-triage", items: "sample-a,loader-b", expectedRoute: "unpack-pe:unpack-analysis", expectedOutputContract: "sample_ref", laneName: "loader", expectedLane: "unpack-analysis-loader"},
	{pack: "vuln-research", taskType: "crash-triage", items: "crash-a,patch-b", expectedRoute: "vuln-research:vuln-analysis", expectedOutputContract: "target_ref", laneName: "crash", expectedLane: "vuln-analysis-crash"},
	{pack: "web-security", taskType: "endpoint-analysis", items: "/login,/api/orders", expectedRoute: "web-security:feature-analysis", expectedOutputContract: "endpoint", laneName: "authz", expectedLane: "feature-authz"},
}

var productionPackSmokePacks = map[string]bool{
	"web-security": true,
}

func TestRunGoSkeletonPackSmokeMatrix(t *testing.T) {
	repo := repoRoot(t)
	packs, err := manifest.List(repo)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{}
	for _, fixture := range skeletonPackSmokeFixtures {
		expected[fixture.pack] = true
	}
	discovered := map[string]manifest.PackSummary{}
	for _, pack := range packs {
		if pack.SchemaValid {
			discovered[pack.ID] = pack
		}
	}
	actual := map[string]bool{}
	for _, pack := range packs {
		if pack.SchemaValid && pack.Maturity == "skeleton" {
			actual[pack.ID] = true
		}
	}
	for pack := range productionPackSmokePacks {
		row, ok := discovered[pack]
		if !ok || row.Maturity != "mature" {
			t.Fatalf("production pack smoke %s is not a schema-valid mature pack: %+v", pack, row)
		}
		actual[pack] = true
	}
	if len(actual) != len(expected) {
		t.Fatalf("pack smoke inventory drifted: actual=%v expected=%v", actual, expected)
	}
	for pack := range expected {
		if !actual[pack] {
			t.Fatalf("pack %s is missing from the Go-native smoke matrix", pack)
		}
	}
	for pack := range actual {
		if !expected[pack] {
			t.Fatalf("Go-native smoke matrix is missing discovered pack %s", pack)
		}
	}

	for _, fixture := range skeletonPackSmokeFixtures {
		t.Run(fixture.pack, func(t *testing.T) {
			runSkeletonPackSmoke(t, fixture)
		})
	}
}

func runSkeletonPackSmoke(t *testing.T, fixture skeletonPackSmokeFixture) {
	t.Helper()
	var out bytes.Buffer
	if err := Run([]string{"-Command", "doctor", "-Pack", fixture.pack, "-Format", "json"}, &out); err != nil {
		t.Fatalf("pack doctor failed: %v", err)
	}

	caseRoot := filepath.Join(t.TempDir(), "smkroot")
	previewArgs := []string{
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", fixture.pack,
		"-ProjectName", fixture.pack + "-go-smoke",
		"-WhatIf", "-Format", "json",
	}
	out.Reset()
	if err := Run(previewArgs, &out); err != nil {
		t.Fatalf("init preview failed: %v", err)
	}
	var preview struct {
		Command            string   `json:"command"`
		IsMutation         bool     `json:"isMutation"`
		ExpectedPlanSHA256 string   `json:"expectedPlanSha256"`
		ApplyArgs          []string `json:"applyArgs"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("decode init preview: %v\n%s", err, out.String())
	}
	if preview.Command != "init" || preview.IsMutation || len(preview.ExpectedPlanSHA256) != 64 ||
		len(preview.ApplyArgs) == 0 || !packSmokeHasArg(preview.ApplyArgs, "-Apply") ||
		packSmokeArgValue(preview.ApplyArgs, "-ExpectedInitPlanSha256") != preview.ExpectedPlanSHA256 {
		t.Fatalf("init preview omitted its exact typed Apply action: %+v", preview)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("init preview mutated the case root: %v", err)
	}

	out.Reset()
	if err := Run(append([]string{}, preview.ApplyArgs...), &out); err != nil {
		t.Fatalf("exact init Apply failed: %v", err)
	}
	var applied struct {
		Command    string `json:"command"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Pack       string `json:"pack"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("decode init Apply: %v\n%s", err, out.String())
	}
	if applied.Command != "init" || !applied.IsMutation || !applied.Applied || applied.Pack != fixture.pack {
		t.Fatalf("unexpected init Apply result: %+v", applied)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai")); err != nil {
		t.Fatalf("current init omitted .steamai: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".rekit")); !os.IsNotExist(err) {
		t.Fatalf("current init wrote legacy .rekit state: %v", err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", fixture.pack, "-Format", "json"}, &out); err != nil {
		t.Fatalf("case doctor failed: %v", err)
	}

	reviewRoot := filepath.Join(t.TempDir(), "plan-review")
	out.Reset()
	if err := Run([]string{
		"-Command", "plan-subagents",
		"-Target", caseRoot,
		"-Pack", fixture.pack,
		"-TaskType", fixture.taskType,
		"-Items", fixture.items,
		"-ReviewOutputDir", reviewRoot,
	}, &out); err != nil {
		t.Fatalf("plan-subagents failed: %v", err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if plan.Command != "plan-subagents" || plan.IsMutation || !plan.WritesReviewArtifacts || !plan.ReviewRequired || plan.ItemCount != 2 {
		t.Fatalf("unexpected plan-subagents result: %+v", plan)
	}
	packetData, err := os.ReadFile(plan.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Route struct {
			ID             string `json:"id"`
			OutputContract string `json:"outputContract"`
		} `json:"route"`
		OutputContract string `json:"outputContract"`
		Observability  struct {
			RouteDebug struct {
				SelectedBy string `json:"selectedBy"`
			} `json:"routeDebug"`
		} `json:"observability"`
	}
	if err := json.Unmarshal(packetData, &packet); err != nil {
		t.Fatalf("decode plan-subagents packet: %v", err)
	}
	if packet.Route.ID != fixture.expectedRoute || packet.Observability.RouteDebug.SelectedBy != "taskType" ||
		(!strings.Contains(packet.OutputContract, fixture.expectedOutputContract) &&
			!strings.Contains(packet.Route.OutputContract, fixture.expectedOutputContract)) {
		t.Fatalf("unexpected pack route packet: %+v", packet)
	}

	workflowRel := filepath.ToSlash(filepath.Join("references", fixture.pack, "workflow-template.md"))
	workflowPath := filepath.Join(caseRoot, filepath.FromSlash(workflowRel))
	file, err := os.OpenFile(workflowPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString("\nReusable safe Go-native pack smoke note.\n")
	closeErr := file.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}

	promoteRoot := filepath.Join(t.TempDir(), "promote-review")
	out.Reset()
	if err := Run([]string{
		"-Command", "promote",
		"-Target", caseRoot,
		"-Pack", fixture.pack,
		"-ReviewOutputDir", promoteRoot,
	}, &out); err != nil {
		t.Fatalf("promote review failed: %v", err)
	}
	artifact := decodeArtifactResult(t, out.Bytes())
	if artifact.Command != "promote" || artifact.IsMutation || !artifact.WritesArtifacts {
		t.Fatalf("unexpected promote review artifact: %+v", artifact)
	}
	promoteData, err := os.ReadFile(artifact.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var promotePlan review.Plan
	if err := json.Unmarshal(promoteData, &promotePlan); err != nil {
		t.Fatalf("decode promote review packet: %v", err)
	}
	found := false
	for _, item := range promotePlan.Items {
		if item.Path == workflowRel && item.Kind == "managed-doc" && item.Action == "candidate-after-llm-review" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("promote review omitted the managed workflow candidate: %+v", promotePlan.Items)
	}

	for _, rel := range []string{"board.json", "facts", "lanes"} {
		path := filepath.Join(caseRoot, ".steamai", rel)
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("pack planning created durable workstream state %s: %v", rel, err)
		}
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "start",
		"-Target", caseRoot,
		"-Pack", fixture.pack,
		"-Name", fixture.laneName,
		"-Apply", "-Format", "json",
	}, &out); err != nil {
		t.Fatalf("start smoke lane failed: %v", err)
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "handoff",
		"-Target", caseRoot,
		"-Pack", fixture.pack,
		fixture.laneName,
		"-WhatIf", "-Format", "json",
	}, &out); err != nil {
		t.Fatalf("handoff preview failed: %v", err)
	}
	var handoffPreview struct {
		Command               string   `json:"command"`
		Selector              string   `json:"selector"`
		IsMutation            bool     `json:"isMutation"`
		PublicationPlanSHA256 string   `json:"publicationPlanSha256"`
		PublicationStamp      string   `json:"publicationStamp"`
		ApplyArgs             []string `json:"applyArgs"`
	}
	if err := json.Unmarshal(out.Bytes(), &handoffPreview); err != nil {
		t.Fatalf("decode handoff preview: %v\n%s", err, out.String())
	}
	if handoffPreview.Command != "handoff" || handoffPreview.IsMutation ||
		handoffPreview.Selector != fixture.expectedLane ||
		len(handoffPreview.PublicationPlanSHA256) != 64 ||
		handoffPreview.PublicationStamp == "" ||
		len(handoffPreview.ApplyArgs) == 0 ||
		packSmokeArgValue(handoffPreview.ApplyArgs, "-Pack") != fixture.pack ||
		packSmokeArgValue(handoffPreview.ApplyArgs, "-Lane") != fixture.expectedLane ||
		packSmokeArgValue(handoffPreview.ApplyArgs, "-ExpectedHandoffPlanSha256") != handoffPreview.PublicationPlanSHA256 ||
		packSmokeArgValue(handoffPreview.ApplyArgs, "-HandoffPublicationStamp") != handoffPreview.PublicationStamp {
		t.Fatalf("handoff preview omitted its exact typed Apply action: %+v", handoffPreview)
	}

	out.Reset()
	if err := Run(append([]string{}, handoffPreview.ApplyArgs...), &out); err != nil {
		t.Fatalf("exact handoff Apply failed: %v", err)
	}
	var handoffApplied struct {
		Command               string `json:"command"`
		Selector              string `json:"selector"`
		IsMutation            bool   `json:"isMutation"`
		Applied               bool   `json:"applied"`
		PublicationPlanSHA256 string `json:"publicationPlanSha256"`
	}
	if err := json.Unmarshal(out.Bytes(), &handoffApplied); err != nil {
		t.Fatalf("decode handoff Apply: %v\n%s", err, out.String())
	}
	if handoffApplied.Command != "handoff" || !handoffApplied.IsMutation || !handoffApplied.Applied ||
		handoffApplied.Selector != fixture.expectedLane ||
		handoffApplied.PublicationPlanSHA256 != handoffPreview.PublicationPlanSHA256 {
		t.Fatalf("unexpected exact handoff Apply result: %+v", handoffApplied)
	}
}

func packSmokeHasArg(args []string, want string) bool {
	return slices.Contains(args, want)
}

func packSmokeArgValue(args []string, name string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == name {
			return args[index+1]
		}
	}
	return ""
}
