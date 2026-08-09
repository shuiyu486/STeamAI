package workstream

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestReviewerResultBindsManifestAcrossCaseLocalPathForms(t *testing.T) {
	caseRoot := t.TempDir()
	manifestRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", "feature-mission", "member-execution", "attempts", "attempt-1", "result-manifest.json"))
	manifestPath := filepath.Join(caseRoot, filepath.FromSlash(manifestRef))
	if !reviewerResultBindsManifest(caseRoot, []string{manifestPath}, manifestRef) {
		t.Fatal("absolute case-local reviewer item did not bind relative manifest ref")
	}
	if !reviewerResultBindsManifest(caseRoot, []string{manifestRef + "#review"}, manifestPath) {
		t.Fatal("relative case-local reviewer item did not bind absolute manifest ref")
	}
	outside := filepath.Join(filepath.Dir(caseRoot), "outside-manifest.json")
	if reviewerResultBindsManifest(caseRoot, []string{outside}, manifestRef) {
		t.Fatal("outside reviewer item bound case-local manifest ref")
	}
}

func TestMemberManifestReviewerRejectionSelectsCanonicalTargetNotEvidenceRefs(t *testing.T) {
	caseRoot := t.TempDir()
	manifestRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", "feature-mission", "member-executions", "attempt-1", "evidence", "manifest.json"))
	manifestPath := filepath.Join(caseRoot, filepath.FromSlash(manifestRef))
	verification := map[string]any{
		"lane":         "feature-mission",
		"verdict":      "rejected",
		"target":       manifestRef,
		"evidenceRefs": []string{"packet-1"},
		"packetId":     "packet-1",
		"eventId":      "verification-1",
	}
	decision := map[string]any{
		"lane":         "feature-mission",
		"decision":     "reject",
		"packetId":     "packet-1",
		"evidenceRefs": []string{"verification-1", "packet-1"},
	}
	facts := mission.LedgerFacts{
		Facts:         mission.Facts{Decisions: []map[string]any{decision}},
		Verifications: []map[string]any{verification},
	}

	if !eventTargetBindsPath(verification, manifestRef, manifestPath) {
		t.Fatal("canonical reviewer target did not bind the reviewed member manifest")
	}
	if completionEvidencePathsEqual(verification["evidenceRefs"].([]string)[0], manifestRef) {
		t.Fatal("packet-only evidenceRefs unexpectedly matched the reviewed member manifest")
	}
	_, found, err := memberManifestReviewerRejection(caseRoot, "feature-mission", manifestRef, facts)
	if err == nil || found || !strings.Contains(err.Error(), "reviewer rejection ledger semantics are not canonical reject/rejected") {
		t.Fatalf("target-bound rejection candidate was not selected for strict lineage validation: found=%t err=%v", found, err)
	}
}

func TestReviewerWritebackItemsRequireCurrentManifestAndExactShard(t *testing.T) {
	caseRoot := t.TempDir()
	manifestRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", "feature-mission", "member-executions", "attempt-1", "evidence", "manifest.json"))
	otherRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", "feature-mission", "member-executions", "attempt-2", "evidence", "manifest.json"))

	if err := validateReviewerWritebackItems(caseRoot, manifestRef, []string{otherRef}, []string{otherRef}); err == nil || !strings.Contains(err.Error(), "current member manifest") {
		t.Fatalf("writeback accepted a different reviewed manifest: %v", err)
	}
	if err := validateReviewerWritebackItems(caseRoot, manifestRef, []string{manifestRef}, []string{manifestRef, otherRef}); err == nil || !strings.Contains(err.Error(), "current member manifest") {
		t.Fatalf("writeback accepted result items that differ from the packet shard: %v", err)
	}
	if err := validateReviewerWritebackItems(caseRoot, manifestRef, []string{manifestRef}, []string{manifestRef}); err != nil {
		t.Fatalf("writeback rejected the exact current manifest shard: %v", err)
	}
}

func TestMemberManifestReviewerAcceptanceSelectsCanonicalTargetNotEvidenceRefs(t *testing.T) {
	caseRoot := t.TempDir()
	manifestRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", "feature-mission", "member-executions", "attempt-1", "evidence", "manifest.json"))
	manifestPath := filepath.Join(caseRoot, filepath.FromSlash(manifestRef))
	packetPath := filepath.Join(caseRoot, ".rekit", "missing-packet.json")
	inputSHA256 := strings.Repeat("a", 64)
	verification := map[string]any{
		"lane":                      "feature-mission",
		"verdict":                   "accepted",
		"target":                    manifestRef,
		"evidenceRefs":              []string{"packet-1"},
		"packetId":                  "packet-1",
		"shardId":                   "shard-01",
		"packetPath":                packetPath,
		"reviewerResultInputSha256": inputSHA256,
		"eventId":                   "verification-1",
	}
	decision := map[string]any{
		"lane":                      "feature-mission",
		"decision":                  "accept",
		"reviewerDecision":          "accept",
		"packetId":                  "packet-1",
		"shardId":                   "shard-01",
		"packetPath":                packetPath,
		"reviewerResultInputSha256": inputSHA256,
		"evidenceRefs":              []string{"verification-1", "packet-1"},
	}
	facts := mission.LedgerFacts{
		Facts:         mission.Facts{Decisions: []map[string]any{decision}},
		Verifications: []map[string]any{verification},
	}

	if !eventTargetBindsPath(verification, manifestRef, manifestPath) {
		t.Fatal("canonical reviewer target did not bind the reviewed member manifest")
	}
	if completionEvidencePathsEqual(verification["evidenceRefs"].([]string)[0], manifestRef) {
		t.Fatal("packet-only evidenceRefs unexpectedly matched the reviewed member manifest")
	}
	_, err := requireMemberManifestReviewerWriteback(caseRoot, "feature-mission", manifestRef, facts)
	if err == nil || errors.Is(err, errMemberManifestReviewerWritebackRequired) || !strings.Contains(err.Error(), "complete reviewer packet binding is invalid") {
		t.Fatalf("target-bound acceptance candidate was not selected for strict packet lineage validation: %v", err)
	}
}
