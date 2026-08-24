package sessionhost

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const maxClaudeRecoveryPreflightAttempts = 10000

type heldClaudeResultPreflight struct {
	Options     Options
	Package     mission.CurrentLoopExternalSessionHarnessPackage
	Run         claudeRun
	Publication executioncontrol.ResultPublication
}

type supervisedClaudeRecoveryCandidate struct {
	Options Options
	Package mission.CurrentLoopExternalSessionHarnessPackage
	Run     claudeRun
}

func prepareHeldClaudeResultBeforeStatus(opt Options) (heldClaudeResultPreflight, bool, error) {
	candidate, found, err := discoverSupervisedClaudeRecoveryBeforeStatus(opt)
	if err != nil || !found {
		return heldClaudeResultPreflight{}, false, err
	}
	trusted, ok, err := bindClaudeRecoveryPreflightTrust(opt, candidate.Options)
	if err != nil || !ok {
		return heldClaudeResultPreflight{}, false, err
	}
	publicationOpt, err := claudeResultPublicationOptions(trusted, candidate.Package, candidate.Run)
	if err != nil {
		return heldClaudeResultPreflight{}, false, err
	}
	publication, err := executioncontrol.PrepareResult(opt.Target, publicationOpt)
	if err != nil {
		return heldClaudeResultPreflight{}, false, err
	}
	if !publication.Held {
		return heldClaudeResultPreflight{}, false, nil
	}
	return heldClaudeResultPreflight{
		Options:     trusted,
		Package:     candidate.Package,
		Run:         candidate.Run,
		Publication: publication,
	}, true, nil
}

func prepareHeldClaudeResultWithControl(parent context.Context, opt Options) (Result, string, bool, error) {
	recoveryRoot, err := claudeRecoveryRootPath(opt.Target)
	if err != nil {
		return Result{}, "", false, err
	}
	info, err := os.Lstat(recoveryRoot)
	if os.IsNotExist(err) {
		return Result{}, "", false, nil
	}
	if err != nil {
		return Result{}, "", false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Result{}, "", false, fmt.Errorf("host-owned Claude recovery root must remain a non-symlink directory")
	}
	supervisionRoot, err := supervisionRootPath(opt.Target)
	if err != nil {
		return Result{}, "", false, err
	}
	supervisionInfo, err := os.Lstat(supervisionRoot)
	if os.IsNotExist(err) {
		return Result{}, "", false, nil
	}
	if err != nil {
		return Result{}, "", false, err
	}
	if !supervisionInfo.IsDir() || supervisionInfo.Mode()&os.ModeSymlink != 0 {
		return Result{}, "", false, fmt.Errorf("host-owned Claude supervision root must remain a non-symlink directory")
	}
	control, err := acquireSupervisionControl(parent, opt.Target)
	if err != nil {
		return Result{}, "", false, err
	}
	defer control.Close()
	held, ok, err := prepareHeldClaudeResultBeforeStatus(opt)
	if err != nil || !ok {
		return Result{}, "", false, err
	}
	if held.Run.launchControlBinding == nil {
		return Result{}, "", false, fmt.Errorf("held Claude result preflight omitted its exact lane birth binding")
	}
	return heldClaudeResult(opt, held), held.Run.launchControlBinding.Lane, true, nil
}

func bindClaudeRecoveryPreflightTrust(current, durable Options) (Options, bool, error) {
	if !trustedRecoveryProvenance(durable) {
		return Options{}, false, nil
	}
	if current.requireDailyClaudeTrust {
		dailyOpt := DailyOptions{
			ClaudePath:                        current.ClaudePath,
			ExpectedClaudeExecutableSHA256:    current.ExpectedClaudeExecutableSHA256,
			ExpectedClaudeExecutablePublisher: current.ExpectedClaudeExecutablePublisher,
		}
		if err := bindDailyTrustedClaude(&dailyOpt); err != nil {
			return Options{}, false, err
		}
		current.ClaudePath = dailyOpt.ClaudePath
		current.ExpectedClaudeExecutableSHA256 = dailyOpt.ExpectedClaudeExecutableSHA256
		current.ExpectedClaudeExecutablePublisher = dailyOpt.ExpectedClaudeExecutablePublisher
	}
	if trustedRecoveryProvenance(current) {
		if !strings.EqualFold(strings.TrimSpace(current.ExpectedClaudeExecutableSHA256), strings.TrimSpace(durable.ExpectedClaudeExecutableSHA256)) ||
			!strings.EqualFold(strings.TrimSpace(current.ExpectedClaudeExecutablePublisher), strings.TrimSpace(durable.ExpectedClaudeExecutablePublisher)) {
			return Options{}, false, fmt.Errorf("host-owned Claude recovery trusted executable differs from the current host binding")
		}
		if path := strings.TrimSpace(current.ClaudePath); path != "" && !rekitfs.SamePath(path, durable.ClaudePath) {
			return Options{}, false, fmt.Errorf("host-owned Claude recovery executable path differs from the current host binding")
		}
	}
	durable.Actor = current.Actor
	durable.MaxAttempts = current.MaxAttempts
	durable.projectExecutionLease = current.projectExecutionLease
	return durable, true, nil
}

func validateRecoveryPreflightBinding(root *rekitfs.AnchoredRoot, rel string, expected []byte, label string) error {
	data, _, err := root.ReadStableFile(rel, 1024)
	if err != nil {
		return fmt.Errorf("read %s: %w", label, err)
	}
	if !bytes.Equal(data, expected) {
		return fmt.Errorf("%s does not match the exact attached case namespace", label)
	}
	return nil
}

func discoverSupervisedClaudeRecoveryBeforeStatus(opt Options) (supervisedClaudeRecoveryCandidate, bool, error) {
	recoveryRootPath, _, _, recoveryCaseSHA, err := claudeRecoveryRootIdentity(opt.Target)
	if err != nil {
		return supervisedClaudeRecoveryCandidate{}, false, err
	}
	recoveryRoot, err := rekitfs.OpenAnchoredRoot(recoveryRootPath)
	if errors.Is(err, os.ErrNotExist) {
		return supervisedClaudeRecoveryCandidate{}, false, nil
	}
	if err != nil {
		return supervisedClaudeRecoveryCandidate{}, false, err
	}
	defer recoveryRoot.Close()
	if err := validateRecoveryPreflightBinding(
		recoveryRoot,
		".binding",
		[]byte("rekit Claude recovery root v1\ncaseSha256="+recoveryCaseSHA+"\n"),
		"Claude recovery root binding",
	); err != nil {
		return supervisedClaudeRecoveryCandidate{}, false, err
	}
	attempts, err := recoveryRoot.ListNoFollow(".", maxClaudeRecoveryPreflightAttempts+1)
	if err != nil {
		return supervisedClaudeRecoveryCandidate{}, false, err
	}

	supervisionRootPath := ""
	var supervisionRoot *rekitfs.AnchoredRoot
	defer func() {
		if supervisionRoot != nil {
			_ = supervisionRoot.Close()
		}
	}()
	openSupervisionRoot := func() (*rekitfs.AnchoredRoot, string, error) {
		if supervisionRoot != nil {
			return supervisionRoot, supervisionRootPath, nil
		}
		var rootErr error
		var supervisionCaseSHA string
		supervisionRootPath, supervisionCaseSHA, rootErr = supervisionRootIdentity(opt.Target)
		if rootErr != nil {
			return nil, "", rootErr
		}
		supervisionRoot, rootErr = rekitfs.OpenAnchoredRoot(supervisionRootPath)
		if rootErr != nil {
			return nil, "", rootErr
		}
		if rootErr = validateRecoveryPreflightBinding(
			supervisionRoot,
			"binding",
			[]byte("rekit Claude supervision root v2\ncaseSha256="+supervisionCaseSHA+"\n"),
			"Claude supervision root binding",
		); rootErr != nil {
			_ = supervisionRoot.Close()
			supervisionRoot = nil
			return nil, "", rootErr
		}
		return supervisionRoot, supervisionRootPath, nil
	}

	var candidate supervisedClaudeRecoveryCandidate
	found := false
	for _, attempt := range attempts {
		info, infoErr := attempt.Info()
		if infoErr != nil {
			return supervisedClaudeRecoveryCandidate{}, false, infoErr
		}
		if attempt.Name() == ".binding" && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !validClaudeLaunchSHA256(attempt.Name()) {
			if info.Mode().IsRegular() && strings.HasSuffix(attempt.Name(), ".json") {
				return supervisedClaudeRecoveryCandidate{}, false, fmt.Errorf("attached project Claude recovery omitted its execution control birth binding")
			}
			return supervisedClaudeRecoveryCandidate{}, false, fmt.Errorf("Claude recovery preflight found an invalid attempt artifact: %s", attempt.Name())
		}
		results, listErr := recoveryRoot.ListNoFollow(attempt.Name(), 64)
		if listErr != nil {
			return supervisedClaudeRecoveryCandidate{}, false, listErr
		}
		for _, result := range results {
			resultInfo, resultInfoErr := result.Info()
			if resultInfoErr != nil || !resultInfo.Mode().IsRegular() || resultInfo.Mode()&os.ModeSymlink != 0 ||
				!strings.HasSuffix(result.Name(), ".json") || !validClaudeLaunchSHA256(strings.TrimSuffix(result.Name(), ".json")) {
				return supervisedClaudeRecoveryCandidate{}, false, errors.Join(
					resultInfoErr,
					fmt.Errorf("Claude recovery preflight found an invalid control-head artifact: %s", result.Name()),
				)
			}
			rel := filepath.Join(attempt.Name(), result.Name())
			recoveryData, _, readErr := recoveryRoot.ReadStableFile(rel, maxClaudeRawArtifactBytes)
			if readErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, readErr
			}
			var recovery claudeRecovery
			if decodeErr := strictJSON(recoveryData, &recovery); decodeErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, fmt.Errorf("decode Claude structured output recovery during preflight: %w", decodeErr)
			}
			identityPackage, identityErr := claudeRecoveryIdentityPackage(recovery)
			if identityErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, identityErr
			}
			runID, identityErr := supervisionRunID(identityPackage, recovery.LaunchControl)
			if identityErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, identityErr
			}
			runRoot, runRootPath, openErr := openSupervisionRoot()
			if openErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, openErr
			}
			specRel := filepath.Join("runs", runID, "spec.json")
			specData, _, readErr := runRoot.ReadStableFile(specRel, 2*1024*1024)
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			if readErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, fmt.Errorf("read exact Claude supervision spec for recovery preflight: %w", readErr)
			}
			var spec supervisionSpec
			if decodeErr := strictJSON(specData, &spec); decodeErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, fmt.Errorf("decode Claude supervision spec for recovery preflight: %w", decodeErr)
			}
			candidateOpt, pkg, paths, specSHA, relevant, bindErr := bindSupervisionRecoveryCandidate(opt, runRootPath, runID, spec, specData)
			if bindErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, bindErr
			}
			if !relevant {
				continue
			}
			expectedRecoveryRel := claudeRecoveryPath(pkg, recovery.LaunchControl)
			if filepath.Clean(expectedRecoveryRel) != filepath.Clean(rel) {
				return supervisedClaudeRecoveryCandidate{}, false, fmt.Errorf("Claude recovery preflight artifact path differs from its exact package and birth identity")
			}
			run, recovered, recoverErr := recoverClaudeRunForCase(opt.Target, candidateOpt, pkg)
			if recoverErr != nil {
				return supervisedClaudeRecoveryCandidate{}, false, recoverErr
			}
			if !recovered {
				return supervisedClaudeRecoveryCandidate{}, false, fmt.Errorf("Claude recovery preflight artifact could not be recovered exactly")
			}
			if err := validateSupervisionRecoveryCandidate(candidateOpt, pkg, paths, spec, specData, specSHA, run); err != nil {
				return supervisedClaudeRecoveryCandidate{}, false, err
			}
			if found {
				return supervisedClaudeRecoveryCandidate{}, false, fmt.Errorf("multiple exact host-owned Claude results require held-result recovery before status")
			}
			candidate = supervisedClaudeRecoveryCandidate{Options: candidateOpt, Package: pkg, Run: run}
			found = true
		}
	}
	return candidate, found, nil
}

func claudeRecoveryIdentityPackage(recovery claudeRecovery) (mission.CurrentLoopExternalSessionHarnessPackage, error) {
	if recovery.SchemaVersion != 2 || recovery.Kind != claudeRecoveryKind || recovery.LaunchControl == nil ||
		strings.TrimSpace(recovery.SessionKind) == "" || strings.TrimSpace(recovery.AttemptID) == "" ||
		strings.TrimSpace(recovery.AttemptSHA256) == "" || strings.TrimSpace(recovery.SessionID) == "" {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, fmt.Errorf("Claude recovery preflight requires a current exact result birth identity")
	}
	if err := validateClaudeLaunchControlBinding(*recovery.LaunchControl); err != nil {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, err
	}
	if err := validateProductionInstructionBirth(recovery.Pack, recovery.InstructionIdentity); err != nil {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, err
	}
	return mission.CurrentLoopExternalSessionHarnessPackage{
		Pack:        recovery.Pack,
		SessionKind: recovery.SessionKind,
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Capability:          recovery.Capability,
			InstructionIdentity: cloneProductionInstructionIdentityPointer(recovery.InstructionIdentity),
			Attempt: mission.CurrentLoopExternalSessionAttempt{
				AttemptID:     recovery.AttemptID,
				AttemptSHA256: recovery.AttemptSHA256,
				Session:       recovery.SessionID,
			},
		},
	}, nil
}

func bindSupervisionRecoveryCandidate(
	opt Options,
	rootPath,
	runID string,
	spec supervisionSpec,
	specData []byte,
) (Options, mission.CurrentLoopExternalSessionHarnessPackage, supervisionPaths, string, bool, error) {
	if spec.SchemaVersion != 1 || spec.Kind != supervisionSpecKind || spec.RunID != runID ||
		!validClaudeLaunchSHA256(spec.RunID) || strings.TrimSpace(spec.SessionID) == "" || spec.TimeoutNanos <= 0 ||
		spec.LaunchControl == nil || !casePathEqual(opt.Target, spec.Target) || !casePathEqual(opt.Target, spec.Execution.CaseRoot) {
		return Options{}, mission.CurrentLoopExternalSessionHarnessPackage{}, supervisionPaths{}, "", false, nil
	}
	if err := validateClaudeLaunchControlBinding(*spec.LaunchControl); err != nil {
		return Options{}, mission.CurrentLoopExternalSessionHarnessPackage{}, supervisionPaths{}, "", false, nil
	}
	selected := strings.TrimSpace(opt.SelectedLane)
	if selected != "" && spec.LaunchControl.Lane != selected {
		return Options{}, mission.CurrentLoopExternalSessionHarnessPackage{}, supervisionPaths{}, "", false, nil
	}
	pkg := spec.Execution.packageForRun()
	if pkg.Launch == nil || pkg.Launch.Attempt.Session != spec.SessionID {
		return Options{}, mission.CurrentLoopExternalSessionHarnessPackage{}, supervisionPaths{}, "", false, nil
	}
	candidateOpt := Options{
		Target:                            opt.Target,
		Pack:                              spec.Pack,
		SelectedLane:                      selected,
		Actor:                             opt.Actor,
		ClaudePath:                        spec.ClaudePath,
		ExpectedClaudeExecutableSHA256:    spec.ExpectedClaudeExecutableSHA256,
		ExpectedClaudeExecutablePublisher: spec.ExpectedClaudeExecutablePublisher,
		Model:                             spec.Model,
		Timeout:                           time.Duration(spec.TimeoutNanos),
		MaxAttempts:                       opt.MaxAttempts,
		launchControlBinding:              cloneClaudeLaunchControlBinding(spec.LaunchControl),
		projectExecutionLease:             opt.projectExecutionLease,
	}
	if !trustedRecoveryProvenance(candidateOpt) {
		return Options{}, mission.CurrentLoopExternalSessionHarnessPackage{}, supervisionPaths{}, "", false, nil
	}
	paths := supervisionPathsForRun(rootPath, runID)
	return candidateOpt, pkg, paths, bytesSHA256(specData), true, nil
}

func validateSupervisionRecoveryCandidate(
	opt Options,
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
	paths supervisionPaths,
	spec supervisionSpec,
	specData []byte,
	specSHA string,
	run claudeRun,
) error {
	if !run.success() || !sameClaudeLaunchControlBinding(run.launchControlBinding, spec.LaunchControl) {
		return fmt.Errorf("host-owned Claude recovery does not match its successful supervision birth binding")
	}
	if err := validateClaudeStructuredResult(pkg, run); err != nil {
		return fmt.Errorf("validate host-owned Claude recovery result: %w", err)
	}
	expectedPaths, _, expectedData, expectedSHA, err := supervisionSpecForRoot(paths.root, opt, pkg, spec.SessionID)
	if err != nil {
		return err
	}
	if expectedPaths.spec != paths.spec || expectedPaths.terminal != paths.terminal ||
		expectedSHA != specSHA || !bytes.Equal(expectedData, specData) {
		return fmt.Errorf("host-owned Claude recovery spec does not match its exact reconstructed run binding")
	}
	terminal, terminalFound, err := readSupervisionTerminal(paths, spec, specSHA)
	if err != nil {
		return err
	}
	if terminalFound {
		terminalRun := claudeRunFromTerminal(terminal, spec.LaunchControl, true)
		if terminalRun.sessionID != run.sessionID || !bytes.Equal(terminalRun.structuredOutput, run.structuredOutput) {
			return fmt.Errorf("host-owned Claude recovery differs from its exact terminal result")
		}
	}
	return nil
}

func supervisionPathsForRun(root, runID string) supervisionPaths {
	runRoot := filepath.Join(root, "runs", runID)
	return supervisionPaths{
		root: root, runID: runID, runRoot: runRoot,
		spec: filepath.Join(runRoot, "spec.json"), claimed: filepath.Join(runRoot, "claimed.json"), started: filepath.Join(runRoot, "started.json"),
		fenced: filepath.Join(runRoot, "fenced.json"), terminal: filepath.Join(runRoot, "terminal.json"), owner: filepath.Join(runRoot, "owner.lease"),
	}
}
