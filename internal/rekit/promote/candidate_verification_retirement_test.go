package promote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetireCandidateVerificationWorkspacePreviewsAppliesAndFailsClosedOnRecreation(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement")
	provisionOpt := CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, provisionOpt); err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(verified.RetirementPreviewCommand, "-RetireCandidateVerificationWorkspace -WhatIf") {
		t.Fatalf("final verification omitted retirement preview command: %+v", verified)
	}
	opt := CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true}
	preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.RetirementSHA256 == "" || len(preview.Roots) != 2 || !strings.Contains(preview.ApplyCommand, "ExpectedRetirementSha256") {
		t.Fatalf("unexpected retirement preview: %+v", preview)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("retirement WhatIf changed workspace: %v", err)
	}
	if _, err := os.Stat(preview.RetirementIntentPath); !os.IsNotExist(err) {
		t.Fatalf("retirement WhatIf wrote intent: %v", err)
	}
	opt.WhatIf = false
	opt.ExpectedRetirementSHA256 = preview.RetirementSHA256
	applied, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Replay || applied.Mode != "retired" {
		t.Fatalf("unexpected retirement apply: %+v", applied)
	}
	if _, err := os.Lstat(workspace); !os.IsNotExist(err) {
		t.Fatalf("retirement left workspace: %v", err)
	}
	if _, err := os.Stat(applied.RetirementIntentPath); err != nil {
		t.Fatalf("retirement intent was not retained: %v", err)
	}
	if _, err := os.Stat(applied.RetirementReceiptPath); err != nil {
		t.Fatalf("retirement receipt missing: %v", err)
	}
	for _, bad := range []string{"bad", strings.Repeat("0", 64)} {
		badOpt := opt
		badOpt.ExpectedRetirementSHA256 = bad
		if result, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, badOpt); err == nil || result.Applied {
			t.Fatalf("completed replay accepted wrong expected hash %q: result=%+v err=%v", bad, result, err)
		}
	}
	replay, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt)
	if err != nil || !replay.Applied || !replay.Replay || replay.Mode != "already-retired" {
		t.Fatalf("retirement replay=%+v err=%v", replay, err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if result, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil || result.Applied || !strings.Contains(err.Error(), "reappeared") {
		t.Fatalf("recreated workspace result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("recreated workspace was automatically deleted: %v", err)
	}
}

func TestRetireCandidateVerificationWorkspaceResumesWorkspaceQuarantineCrashAndRejectsDriftOrDual(t *testing.T) {
	for _, state := range []string{"resume", "drift", "dual"} {
		t.Run(state, func(t *testing.T) {
			repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-workspace-quarantine-"+state)
			if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
				t.Fatal(err)
			}
			preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
			if err != nil {
				t.Fatal(err)
			}
			originalHook := candidateVerificationRetirementStageHook
			candidateVerificationRetirementStageHook = func(stage string) error {
				if stage == "after-workspace-quarantine" {
					return os.ErrClosed
				}
				return nil
			}
			t.Cleanup(func() { candidateVerificationRetirementStageHook = originalHook })
			opt := CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}
			if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil {
				t.Fatal("workspace quarantine crash returned success")
			}
			quarantine := filepath.Join(filepath.Dir(workspace), "."+filepath.Base(workspace)+".retiring-"+preview.RetirementSHA256[:16])
			candidateVerificationRetirementStageHook = nil
			switch state {
			case "drift":
				if err := os.WriteFile(filepath.Join(quarantine, "drift"), []byte("drift\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "dual":
				if err := os.Mkdir(workspace, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			result, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt)
			if state == "resume" {
				if err != nil || !result.Applied {
					t.Fatalf("workspace quarantine did not resume: %+v %v", result, err)
				}
				return
			}
			if err == nil || result.Applied {
				t.Fatalf("workspace quarantine %s accepted: %+v %v", state, result, err)
			}
			if _, statErr := os.Stat(quarantine); statErr != nil {
				t.Fatalf("workspace quarantine %s was deleted: %v", state, statErr)
			}
		})
	}
}

func TestRetireCandidateVerificationWorkspaceQuarantineRemoveReplacementFailsClosed(t *testing.T) {
	for _, sameShape := range []bool{true, false} {
		name := "different"
		if sameShape {
			name = "same-empty"
		}
		t.Run(name, func(t *testing.T) {
			repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-quarantine-remove-"+name)
			if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
				t.Fatal(err)
			}
			preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
			if err != nil {
				t.Fatal(err)
			}
			originalHook := candidateVerificationRetirementStageHook
			candidateVerificationRetirementStageHook = func(stage string) error {
				if stage == "after-workspace-quarantine" {
					return os.ErrClosed
				}
				return nil
			}
			t.Cleanup(func() { candidateVerificationRetirementStageHook = originalHook })
			opt := CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}
			if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil {
				t.Fatal("workspace quarantine setup crash returned success")
			}
			candidateVerificationRetirementStageHook = func(stage string) error {
				if stage != "before-workspace-quarantine-remove" {
					return nil
				}
				candidateVerificationRetirementStageHook = nil
				quarantine := filepath.Join(filepath.Dir(workspace), "."+filepath.Base(workspace)+".retiring-"+preview.RetirementSHA256[:16])
				moved := quarantine + "-moved"
				if err := os.Rename(quarantine, moved); err != nil {
					return err
				}
				if err := os.Mkdir(quarantine, 0o755); err != nil {
					return err
				}
				if !sameShape {
					return os.WriteFile(filepath.Join(quarantine, "different"), []byte("different\n"), 0o644)
				}
				return nil
			}
			if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil {
				t.Fatal("workspace quarantine replacement was accepted")
			}
			quarantine := filepath.Join(filepath.Dir(workspace), "."+filepath.Base(workspace)+".retiring-"+preview.RetirementSHA256[:16])
			if _, err := os.Stat(quarantine); err != nil {
				t.Fatalf("workspace quarantine replacement was deleted: %v", err)
			}
		})
	}
}

func TestRetireCandidateVerificationWorkspaceResumesAfterRootCrash(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-crash")
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
		t.Fatal(err)
	}
	preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	originalHook := candidateVerificationRetirementStageHook
	candidateVerificationRetirementStageHook = func(stage string) error {
		if stage == "after-roots" {
			return os.ErrClosed
		}
		return nil
	}
	t.Cleanup(func() { candidateVerificationRetirementStageHook = originalHook })
	opt := CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}
	if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil {
		t.Fatal("simulated retirement crash returned success")
	}
	if _, err := os.Stat(preview.RetirementIntentPath); err != nil {
		t.Fatalf("crash did not retain retirement intent: %v", err)
	}
	candidateVerificationRetirementStageHook = nil
	result, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Mode != "retired" {
		t.Fatalf("retirement crash resume result: %+v", result)
	}
	if _, err := os.Lstat(workspace); !os.IsNotExist(err) {
		t.Fatalf("retirement crash resume left workspace: %v", err)
	}
}

func TestRetireCandidateVerificationWorkspaceResumesBeforeReceiptCrash(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-before-receipt")
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
		t.Fatal(err)
	}
	preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	originalHook := candidateVerificationRetirementStageHook
	candidateVerificationRetirementStageHook = func(stage string) error {
		if stage == "before-receipt" {
			return os.ErrClosed
		}
		return nil
	}
	t.Cleanup(func() { candidateVerificationRetirementStageHook = originalHook })
	opt := CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}
	if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil {
		t.Fatal("before-receipt crash returned success")
	}
	if _, err := os.Lstat(workspace); !os.IsNotExist(err) {
		t.Fatalf("before-receipt crash left workspace: %v", err)
	}
	candidateVerificationRetirementStageHook = nil
	result, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Mode != "retired" {
		t.Fatalf("before-receipt crash resume result: %+v", result)
	}
}

func TestRetireCandidateVerificationWorkspaceInvalidExpectedHashWritesNothing(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-invalid-hash")
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
		t.Fatal(err)
	}
	preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(workspace, "sentinel.keep")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"invalid", strings.Repeat("0", 64)} {
		if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: expected}); err == nil {
			t.Fatalf("invalid expected hash %q was accepted", expected)
		}
		if got, err := os.ReadFile(sentinel); err != nil || string(got) != "keep\n" {
			t.Fatalf("invalid hash changed sentinel: %q %v", got, err)
		}
		if _, err := os.Stat(freshRoot); err != nil {
			t.Fatalf("invalid hash removed fresh root: %v", err)
		}
		if _, err := os.Lstat(preview.RetirementIntentPath); !os.IsNotExist(err) {
			t.Fatalf("invalid hash wrote intent: %v", err)
		}
	}
}

func TestRetireCandidateVerificationWorkspaceRejectsProvisionDriftBeforeRootMutation(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, _, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-preflight-drift")
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
		t.Fatal(err)
	}
	preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	originalHook := candidateVerificationRetirementStageHook
	candidateVerificationRetirementStageHook = func(stage string) error {
		if stage == "after-intent" {
			return os.WriteFile(preview.ProvisionReceiptPath, []byte("drifted\n"), 0o644)
		}
		return nil
	}
	t.Cleanup(func() { candidateVerificationRetirementStageHook = originalHook })
	if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}); err == nil {
		t.Fatal("provision drift was accepted")
	}
	if _, err := os.Stat(freshRoot); err != nil {
		t.Fatalf("preflight drift removed fresh root: %v", err)
	}
	if _, err := os.Stat(attachedRoot); err != nil {
		t.Fatalf("preflight drift removed attached root: %v", err)
	}
}

func TestRetireCandidateVerificationWorkspaceRejectsWorkspaceRebindBeforeRootApply(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-workspace-rebind")
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
		t.Fatal(err)
	}
	preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	moved := workspace + "-original"
	originalHook := candidateVerificationRetirementStageHook
	candidateVerificationRetirementStageHook = func(stage string) error {
		if stage != "before-root-apply" {
			return nil
		}
		candidateVerificationRetirementStageHook = nil
		if err := os.Rename(workspace, moved); err != nil {
			return err
		}
		return copyCandidateVerificationRetirementTree(workspace, moved)
	}
	t.Cleanup(func() { candidateVerificationRetirementStageHook = originalHook })
	if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}); err == nil || !strings.Contains(err.Error(), "rebound") {
		t.Fatalf("workspace rebind was accepted: %v", err)
	}
	for _, root := range []string{filepath.Join(moved, "fresh"), filepath.Join(moved, "attached"), filepath.Join(workspace, "fresh"), filepath.Join(workspace, "attached")} {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("workspace rebind deleted root %s: %v", root, err)
		}
	}
}

func copyCandidateVerificationRetirementTree(target, source string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, info.Mode().Perm())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, info.Mode().Perm())
	})
}

func TestRetireCandidateVerificationWorkspaceReplayAndResumeRejectProofDrift(t *testing.T) {
	for _, stage := range []string{"intent-resume", "completed-replay"} {
		t.Run(stage, func(t *testing.T) {
			repoRoot, sourceCase, pack, packetPath, decisionPath, _, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-proof-"+stage)
			if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
				t.Fatal(err)
			}
			verified, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot})
			if err != nil {
				t.Fatal(err)
			}
			preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
			if err != nil {
				t.Fatal(err)
			}
			opt := CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}
			if stage == "intent-resume" {
				originalHook := candidateVerificationRetirementStageHook
				candidateVerificationRetirementStageHook = func(current string) error {
					if current == "after-intent" {
						return os.ErrClosed
					}
					return nil
				}
				if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil {
					t.Fatal("intent crash returned success")
				}
				candidateVerificationRetirementStageHook = originalHook
			} else if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(verified.VerificationProofPath, []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil || !strings.Contains(err.Error(), "proof") {
				t.Fatalf("%s accepted drifted proof: %v", stage, err)
			}
		})
	}
}

func TestRetireCandidateVerificationWorkspaceArtifactReplacementFailsClosed(t *testing.T) {
	for _, sameBytes := range []bool{true, false} {
		name := "different-bytes"
		if sameBytes {
			name = "same-bytes"
		}
		t.Run(name, func(t *testing.T) {
			repoRoot, sourceCase, pack, packetPath, decisionPath, _, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-replace-"+name)
			if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
				t.Fatal(err)
			}
			preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
			if err != nil {
				t.Fatal(err)
			}
			originalHook := candidateVerificationRetirementStageHook
			candidateVerificationRetirementStageHook = func(stage string) error {
				if stage != "before-artifact-quarantine:provision.receipt.json" {
					return nil
				}
				candidateVerificationRetirementStageHook = nil
				data := []byte("different\n")
				if sameBytes {
					var readErr error
					data, readErr = os.ReadFile(preview.ProvisionReceiptPath)
					if readErr != nil {
						return readErr
					}
				}
				if err := os.Remove(preview.ProvisionReceiptPath); err != nil {
					return err
				}
				return os.WriteFile(preview.ProvisionReceiptPath, data, 0o644)
			}
			t.Cleanup(func() { candidateVerificationRetirementStageHook = originalHook })
			if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}); err == nil {
				t.Fatal("replacement provision artifact was accepted")
			}
			if _, err := os.Stat(preview.ProvisionReceiptPath); err != nil {
				t.Fatalf("replacement provision artifact was deleted: %v", err)
			}
		})
	}
}

func TestRetireCandidateVerificationWorkspaceResumesArtifactQuarantineCrashAndRejectsDrift(t *testing.T) {
	for _, drift := range []bool{false, true} {
		name := "resume"
		if drift {
			name = "drift"
		}
		t.Run(name, func(t *testing.T) {
			repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-quarantine-"+name)
			if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
				t.Fatal(err)
			}
			preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
			if err != nil {
				t.Fatal(err)
			}
			originalHook := candidateVerificationRetirementStageHook
			candidateVerificationRetirementStageHook = func(stage string) error {
				if stage == "after-artifact-quarantine:provision.receipt.json" {
					return os.ErrClosed
				}
				return nil
			}
			t.Cleanup(func() { candidateVerificationRetirementStageHook = originalHook })
			opt := CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}
			if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt); err == nil {
				t.Fatal("post-quarantine crash returned success")
			}
			quarantine := filepath.Join(workspace, ".provision.receipt.json.retiring-"+preview.ProvisionReceiptSHA256[:16])
			if drift {
				if err := os.WriteFile(quarantine, []byte("drifted\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			candidateVerificationRetirementStageHook = nil
			result, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, opt)
			if drift {
				if err == nil || result.Applied {
					t.Fatalf("drifted quarantine was accepted: %+v %v", result, err)
				}
				if _, statErr := os.Stat(quarantine); statErr != nil {
					t.Fatalf("drifted quarantine was deleted: %v", statErr)
				}
				return
			}
			if err != nil || !result.Applied {
				t.Fatalf("quarantine crash did not resume: %+v %v", result, err)
			}
		})
	}
}

func TestRetireCandidateVerificationWorkspaceRejectsMissingExtraDifferentAndForgedArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, preview CandidateVerificationRetirementResult, workspace, freshRoot string)
	}{
		{name: "missing-proof", mutate: func(t *testing.T, preview CandidateVerificationRetirementResult, _, _ string) {
			if err := os.Remove(preview.VerificationProofPath); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "extra-root-object", mutate: func(t *testing.T, _ CandidateVerificationRetirementResult, _, freshRoot string) {
			if err := os.WriteFile(filepath.Join(freshRoot, "extra.txt"), []byte("extra"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "different-leaf", mutate: func(t *testing.T, preview CandidateVerificationRetirementResult, _, _ string) {
			if err := os.WriteFile(preview.Roots[0].Deletes[0], []byte("different"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "forged-provision-receipt", mutate: func(t *testing.T, preview CandidateVerificationRetirementResult, _, _ string) {
			if err := os.WriteFile(preview.ProvisionReceiptPath, []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, provision := candidateProvisionFixture(t, "retirement-"+test.name)
			if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provision.ProvisionSHA256}); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot}); err != nil {
				t.Fatal(err)
			}
			preview, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, WhatIf: true})
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, preview, workspace, freshRoot)
			if _, err := RetireCandidateVerificationWorkspace(repoRoot, sourceCase, pack, CandidateVerificationRetirementOptions{PacketPath: packetPath, DecisionPath: decisionPath, ExpectedRetirementSHA256: preview.RetirementSHA256}); err == nil {
				t.Fatal("drifted retirement workspace was accepted")
			}
			if _, err := os.Lstat(preview.RetirementReceiptPath); !os.IsNotExist(err) {
				t.Fatalf("failed retirement wrote receipt: %v", err)
			}
		})
	}
}
