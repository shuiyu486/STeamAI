//go:build !windows

package subagents

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestRecoverReviewerResultUnsupportedIsZeroWrite(t *testing.T) {
	for _, kind := range []string{"regular-file", "empty-file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
			caseRoot := filepath.Join(t.TempDir(), "case")
			writeReviewerIntakeCase(t, repoRoot, caseRoot)
			plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{
				TaskType: "feature-analysis",
				Items:    "alpha",
				Lane:     reviewerIntakeLane,
			})
			if err != nil {
				t.Fatal(err)
			}
			packet := readReviewerPacket(t, plan.PacketPath)
			if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			handoff := packet.ShardHandoffs[0]
			candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
			if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "regular-file":
				err = os.WriteFile(handoff.ReviewerResultPath, []byte(`{"different":true}`), 0o644)
			case "empty-file":
				err = os.WriteFile(handoff.ReviewerResultPath, nil, 0o644)
			case "symlink":
				target := filepath.Join(filepath.Dir(handoff.ReviewerResultPath), "symlink-target.json")
				if err = os.WriteFile(target, []byte(`{"target":true}`), 0o644); err == nil {
					err = os.Symlink(filepath.Base(target), handoff.ReviewerResultPath)
				}
			}
			if err != nil {
				t.Fatal(err)
			}

			before := reviewerRecoveryTreeSnapshot(t, caseRoot)
			opt := ReviewerResultRecoveryOptions{
				PacketPath: plan.PacketPath,
				ShardID:    handoff.ShardID,
				Lane:       packet.TargetLane,
				Actor:      "mission-commander",
				Reason:     "verify unsupported recovery remains zero-write",
				WhatIf:     true,
			}
			assertUnsupportedReviewerRecovery(t, repoRoot, caseRoot, kind, opt)
			if after := reviewerRecoveryTreeSnapshot(t, caseRoot); !reflect.DeepEqual(after, before) {
				t.Fatalf("unsupported recovery WhatIf mutated case tree:\nbefore=%v\nafter=%v", before, after)
			}

			opt.WhatIf = false
			opt.ExpectedCandidateSHA256 = sha256Hex(candidate)
			opt.ExpectedReviewerResultSHA256 = strings.Repeat("0", 64)
			assertUnsupportedReviewerRecovery(t, repoRoot, caseRoot, kind, opt)
			if after := reviewerRecoveryTreeSnapshot(t, caseRoot); !reflect.DeepEqual(after, before) {
				t.Fatalf("unsupported recovery Apply mutated case tree:\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}

func assertUnsupportedReviewerRecovery(t *testing.T, repoRoot, caseRoot, kind string, opt ReviewerResultRecoveryOptions) {
	t.Helper()
	_, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	want := "exact " + kind + " reviewer result recovery is unavailable on this platform"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("unsupported %s recovery error = %v, want %q", kind, err, want)
	}
}

func reviewerRecoveryTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + target
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(":%d:%s", len(data), sha256Hex(data))
		}
		snapshot[filepath.ToSlash(rel)] = value
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return snapshot
}
