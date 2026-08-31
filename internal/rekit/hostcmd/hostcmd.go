package hostcmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/cli"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionsuccessor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
)

// RunRecovery exposes only the zero-launch daily recovery front door. It must
// be selected by process-level durable executable validation.
func RunRecovery(args []string, stdout, stderr io.Writer) int {
	return runRecovery(args, stdout, stderr, "")
}

// RunProjectLocalRecovery permits an omitted target only after process-level
// executable-owner validation has selected this bounded recovery route.
func RunProjectLocalRecovery(args []string, stdout, stderr io.Writer, projectRoot string) int {
	return runRecovery(args, stdout, stderr, projectRoot)
}

func runRecovery(args []string, stdout, stderr io.Writer, projectRoot string) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	flags := flag.NewFlagSet("steamai host recovery", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var opt sessionhost.DailyOptions
	daily := flags.Bool("daily", false, "run the daily recovery front door")
	flags.StringVar(&opt.Target, "target", "", "exact project root")
	flags.StringVar(&opt.Goal, "goal", "", "natural-language goal")
	flags.StringVar(&opt.Correction, "correction", "", "human correction")
	flags.StringVar(&opt.SelectedLane, "lane", "", "exact current lane")
	flags.StringVar(&opt.Actor, "actor", "rekit-daily-front-door", "durable host actor")
	flags.StringVar(&opt.ClaudePath, "claude", "", "Claude Code executable path")
	flags.StringVar(&opt.Model, "model", "", "optional Claude model")
	flags.DurationVar(&opt.Timeout, "timeout", 30*time.Minute, "per-session wall-clock timeout")
	flags.IntVar(&opt.MaxAttempts, "max-attempts", 3, "maximum real Claude launches")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 ||
		(!*daily && strings.TrimSpace(opt.Goal) == "" &&
			strings.TrimSpace(opt.Correction) == "") {
		fmt.Fprintln(stderr, "project-local recovery permits only the daily front door")
		return 2
	}
	if strings.TrimSpace(projectRoot) != "" {
		target, err := rekitruntime.ResolveProjectLocalTarget(projectRoot, opt.Target, "")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		opt.Target = target
	} else if strings.TrimSpace(opt.Target) == "" {
		fmt.Fprintln(stderr, "project-local recovery requires an exact -target")
		return 2
	}
	result, err := sessionhost.RunDailyRecovery(
		context.Background(),
		opt,
	)
	if strings.TrimSpace(projectRoot) != "" {
		var projectionErr error
		result, projectionErr = projectDailyResultPublicResponse(result, opt.Target)
		if projectionErr != nil {
			fmt.Fprintln(stderr, errorsJoin(err, projectionErr))
			return 1
		}
	}
	return printResult(stdout, stderr, result, err)
}

// Run executes the rekit-host command surface without owning process exit.
// This lets the project-local steamai executable expose the same host surface
// while the retained rekit-host binary remains a maintenance entrypoint.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, "", "")
}

// RunWithUnifiedExecutable executes the external host surface while binding
// every project initialization path to a verified unified STeamAI runtime image.
func RunWithUnifiedExecutable(args []string, stdout, stderr io.Writer, executable string) int {
	if err := runtimebundle.ValidateUnifiedExecutableRole(executable); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return run(args, stdout, stderr, "", executable)
}

// RunProjectLocal binds every host target, including an internal supervisor
// spec target, to the executable owner project.
func RunProjectLocal(args []string, stdout, stderr io.Writer, projectRoot string) int {
	return run(args, stdout, stderr, projectRoot, "")
}

// RunProjectLocalWithUnifiedExecutable additionally binds project-local
// onboarding to the executable that owns the current project process.
func RunProjectLocalWithUnifiedExecutable(args []string, stdout, stderr io.Writer, projectRoot, executable string) int {
	if err := runtimebundle.ValidateUnifiedExecutableRole(executable); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return run(args, stdout, stderr, projectRoot, executable)
}

func run(args []string, stdout, stderr io.Writer, projectRoot, initializationSourceExecutable string) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	flags := flag.NewFlagSet("steamai host", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var opt sessionhost.Options
	var liveOpt sessionhost.LiveAcceptanceOptions
	var controlOpt executioncontrol.Options
	internalSupervisor := flags.String("internal-supervisor", "", "internal exact host-run supervision spec")
	internalSupervisorSHA256 := flags.String("internal-supervisor-sha256", "", "internal exact host-run supervision spec sha256")
	daily := flags.Bool("daily", false, "run the natural-language daily front door; implied by goal, correction, or lane control")
	liveAcceptance := flags.Bool("live-acceptance", false, "run the explicit default or allowlisted cross-pack real-Claude acceptance gate")
	liveSupervisionAcceptance := flags.Bool("live-supervision-acceptance", false, "run the explicit RH-04 real-Claude process-start recovery gate")
	livePackMemoryAcceptance := flags.Bool("live-pack-memory-acceptance", false, "run the explicit RH-07 cross-case pack-memory real-Claude acceptance gate")
	liveSoakAcceptance := flags.Bool("live-soak-acceptance", false, "run the explicit RH-09 Windows three-task real-Claude soak and recovery gate")
	internalPackMemoryAcceptance := flags.String("internal-pack-memory-live-acceptance", "", "internal RH-07 isolated child spec")
	internalPackMemoryAcceptanceSHA256 := flags.String("internal-pack-memory-live-acceptance-sha256", "", "internal RH-07 isolated child spec sha256")
	flags.StringVar(&opt.Target, "target", "", "fresh or attached case root")
	flags.StringVar(&opt.Pack, "pack", "", "optional attached pack override")
	flags.StringVar(&opt.SelectedLane, "lane", "", "exact current lane selected for this invocation")
	inputMode := flags.String("input-mode", "", "typed binary-re input mode: artifact-analysis or workspace-inventory")
	inputArtifact := flags.String("input-artifact", "", "case-relative artifact path for artifact-analysis")
	inputScope := flags.String("input-scope", "", "case-relative directory for workspace-inventory")
	flags.StringVar(&controlOpt.Action, "control-action", "", "typed lane control action: pause, resume, or stop")
	flags.StringVar(&controlOpt.Reason, "control-reason", "", "bounded reason for the reviewed lane control")
	controlWhatIf := flags.Bool("control-what-if", false, "preview the exact lane control without writing")
	controlApply := flags.Bool("control-apply", false, "apply the exact reviewed lane control")
	flags.StringVar(&controlOpt.PublicationStamp, "control-publication-stamp", "", "exact lane control preview publication stamp")
	flags.StringVar(&controlOpt.ExpectedPlanSHA256, "expected-control-plan-sha256", "", "exact reviewed lane control plan sha256")
	directoryAdoptionAction := flags.String("directory-adoption-action", "", "typed ordinary-directory adoption choice")
	directoryAdoptionPack := flags.String("directory-adoption-pack", "", "mature pack selected for ordinary-directory adoption")
	expectedInitPlanSHA256 := flags.String("expected-init-plan-sha256", "", "exact ordinary-directory init preview sha256")
	successorWhatIf := flags.Bool("successor-what-if", false, "preview a new mission after terminal completion")
	successorApply := flags.Bool("successor-apply", false, "apply the exact reviewed successor mission plan")
	successorPublicationStamp := flags.String("successor-publication-stamp", "", "exact successor mission preview publication stamp")
	expectedSuccessorPlanSHA256 := flags.String("expected-successor-plan-sha256", "", "exact reviewed successor mission plan sha256")
	flags.StringVar(&opt.ExpectedCurrentDriverRequestSHA256, "expected-current-driver-request-sha256", "", "exact fresh missionControlRunbook current driver request sha256")
	flags.StringVar(&opt.Actor, "actor", "rekit-claude-host", "durable host actor")
	flags.StringVar(&opt.ClaudePath, "claude", "", "Claude Code executable path")
	flags.StringVar(&opt.Model, "model", "", "optional Claude model")
	flags.DurationVar(&opt.Timeout, "timeout", 30*time.Minute, "per-session wall-clock timeout")
	flags.IntVar(&opt.MaxAttempts, "max-attempts", 3, "maximum real Claude launches")
	flags.StringVar(&liveOpt.Goal, "goal", "", "natural-language goal for daily or explicit live acceptance mode")
	flags.StringVar(&liveOpt.Correction, "correction", "", "human correction for daily or explicit live acceptance mode")
	flags.BoolVar(&liveOpt.KeepCase, "keep-case", false, "retain the fresh acceptance case after the gate")
	flags.StringVar(&liveOpt.ReceiptPath, "receipt", "", "machine-readable acceptance receipt path; required by -live-soak-acceptance")
	flags.StringVar(&liveOpt.AdapterPath, "adapter", "", "built rekit-adapter-host executable required by default-pack live acceptance")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *successorApply {
		if err := missionsuccessor.ValidateApplyArgs(args, opt.Target, missionsuccessor.Options{
			Goal:               liveOpt.Goal,
			Actor:              opt.Actor,
			PublicationStamp:   *successorPublicationStamp,
			ExpectedPlanSHA256: *expectedSuccessorPlanSHA256,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	dailyRequest := sessionhost.ClassifyDailyRequest(sessionhost.DailyOptions{
		Goal:                        liveOpt.Goal,
		Correction:                  liveOpt.Correction,
		SelectedLane:                opt.SelectedLane,
		Control:                     controlOpt,
		ControlWhatIf:               *controlWhatIf,
		ControlApply:                *controlApply,
		DirectoryAdoptionAction:     *directoryAdoptionAction,
		DirectoryAdoptionPack:       *directoryAdoptionPack,
		ExpectedInitPlanSHA256:      *expectedInitPlanSHA256,
		SuccessorWhatIf:             *successorWhatIf,
		SuccessorApply:              *successorApply,
		SuccessorPublicationStamp:   *successorPublicationStamp,
		ExpectedSuccessorPlanSHA256: *expectedSuccessorPlanSHA256,
	})
	controlRequested := dailyRequest.ControlRequested
	adoptionRequested := dailyRequest.AdoptionRequested
	laneSelected := strings.TrimSpace(opt.SelectedLane) != ""
	maintenanceMode := *liveAcceptance || *liveSupervisionAcceptance || *livePackMemoryAcceptance || *liveSoakAcceptance ||
		strings.TrimSpace(*internalSupervisor) != "" || strings.TrimSpace(*internalSupervisorSHA256) != "" ||
		strings.TrimSpace(*internalPackMemoryAcceptance) != "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) != ""
	inputRequested := strings.TrimSpace(*inputMode) != "" || strings.TrimSpace(*inputArtifact) != "" || strings.TrimSpace(*inputScope) != ""
	if laneSelected && maintenanceMode {
		fmt.Fprintln(stderr, "-lane is supported only by the daily or ordinary host route")
		return 2
	}
	if inputRequested && maintenanceMode {
		fmt.Fprintln(stderr, "typed input readiness is supported only by the external daily front door")
		return 2
	}

	if err := ValidateAdapterFlag(*liveAcceptance, opt.Pack, liveOpt.AdapterPath); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if controlRequested && (*liveAcceptance || *liveSupervisionAcceptance || *livePackMemoryAcceptance || *liveSoakAcceptance || strings.TrimSpace(*internalSupervisor) != "" || strings.TrimSpace(*internalSupervisorSHA256) != "" || strings.TrimSpace(*internalPackMemoryAcceptance) != "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) != "") {
		fmt.Fprintln(stderr, "lane control is supported only by the external daily front door")
		return 2
	}
	if controlRequested && adoptionRequested {
		fmt.Fprintln(stderr, "lane control and directory adoption controls are mutually exclusive")
		return 2
	}
	if adoptionRequested && (*liveAcceptance || *liveSupervisionAcceptance || *livePackMemoryAcceptance || *liveSoakAcceptance || strings.TrimSpace(*internalSupervisor) != "" || strings.TrimSpace(*internalSupervisorSHA256) != "" || strings.TrimSpace(*internalPackMemoryAcceptance) != "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) != "") {
		fmt.Fprintln(stderr, "directory adoption controls are supported only by the external daily front door")
		return 2
	}
	if adoptionRequested && strings.TrimSpace(projectRoot) != "" {
		fmt.Fprintln(stderr, "project-local STeamAI executable cannot adopt an ordinary directory")
		return 2
	}
	ordinaryHostRequested := !PublicModeRequested(*daily, *liveAcceptance, *liveSupervisionAcceptance, *livePackMemoryAcceptance, *liveSoakAcceptance) &&
		strings.TrimSpace(liveOpt.Goal) == "" && strings.TrimSpace(liveOpt.Correction) == "" && !adoptionRequested && !controlRequested &&
		strings.TrimSpace(*internalSupervisor) == "" && strings.TrimSpace(*internalSupervisorSHA256) == "" &&
		strings.TrimSpace(*internalPackMemoryAcceptance) == "" && strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) == ""
	if err := ValidateExpectedCurrentDriverRequestFlag(opt.ExpectedCurrentDriverRequestSHA256, ordinaryHostRequested); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if strings.TrimSpace(projectRoot) != "" &&
		(*livePackMemoryAcceptance || *liveSupervisionAcceptance ||
			*liveAcceptance || *liveSoakAcceptance ||
			strings.TrimSpace(*internalPackMemoryAcceptance) != "" ||
			strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) != "") {
		fmt.Fprintln(stderr, "project-local STeamAI executable does not run maintenance acceptance modes")
		return 2
	}

	if strings.TrimSpace(*internalSupervisor) != "" || strings.TrimSpace(*internalSupervisorSHA256) != "" {
		if strings.TrimSpace(*internalSupervisor) == "" || strings.TrimSpace(*internalSupervisorSHA256) == "" || strings.TrimSpace(*internalPackMemoryAcceptance) != "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) != "" || PublicModeRequested(*daily, *liveAcceptance, *liveSupervisionAcceptance, *livePackMemoryAcceptance, *liveSoakAcceptance) || flags.NArg() != 0 {
			fmt.Fprintln(stderr, "internal supervisor requires an exact spec path and sha256")
			return 2
		}
		if strings.TrimSpace(projectRoot) != "" {
			if err := sessionhost.ValidateSupervisorProjectRoot(
				*internalSupervisor,
				*internalSupervisorSHA256,
				projectRoot,
			); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
		if err := sessionhost.RunSupervisorChild(
			context.Background(),
			*internalSupervisor,
			*internalSupervisorSHA256,
		); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	if strings.TrimSpace(*internalPackMemoryAcceptance) != "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) != "" {
		if strings.TrimSpace(*internalPackMemoryAcceptance) == "" || strings.TrimSpace(*internalPackMemoryAcceptanceSHA256) == "" || PublicModeRequested(*daily, *liveAcceptance, *liveSupervisionAcceptance, *livePackMemoryAcceptance, *liveSoakAcceptance) || flags.NArg() != 0 {
			fmt.Fprintln(stderr, "internal pack-memory acceptance requires an exact spec path and sha256")
			return 2
		}
		result, err := sessionhost.RunPackMemoryLiveAcceptanceChild(context.Background(), *internalPackMemoryAcceptance, *internalPackMemoryAcceptanceSHA256)
		return printResult(stdout, stderr, result, err)
	}

	acceptanceInitializesProject := *livePackMemoryAcceptance || *liveSupervisionAcceptance || *liveAcceptance || *liveSoakAcceptance
	if acceptanceInitializesProject && strings.TrimSpace(initializationSourceExecutable) == "" {
		fmt.Fprintln(stderr, "project-initializing acceptance requires a verified unified runtime executable source")
		return 1
	}

	if *livePackMemoryAcceptance {
		if *daily || *liveAcceptance || *liveSupervisionAcceptance || *liveSoakAcceptance {
			fmt.Fprintln(stderr, "-live-pack-memory-acceptance, -live-supervision-acceptance, -live-soak-acceptance, -live-acceptance, and -daily are mutually exclusive")
			return 2
		}
		if strings.TrimSpace(opt.Pack) != "" && !strings.EqualFold(strings.TrimSpace(opt.Pack), defaults.DefaultPack) {
			fmt.Fprintf(stderr, "pack-memory live acceptance always uses the default %s pack\n", defaults.DefaultPack)
			return 2
		}
		result, err := sessionhost.RunPackMemoryLiveAcceptance(context.Background(), sessionhost.PackMemoryLiveAcceptanceOptions{
			Goal: liveOpt.Goal, ClaudePath: opt.ClaudePath, Model: opt.Model, Actor: opt.Actor,
			Timeout: opt.Timeout, MaxAttempts: opt.MaxAttempts, KeepCase: liveOpt.KeepCase, ReceiptPath: liveOpt.ReceiptPath,
			InitializationSourceExecutable: initializationSourceExecutable,
		})
		result, err = PublishPackMemoryLiveAcceptanceReceipt(liveOpt.ReceiptPath, result, err)
		return printResult(stdout, stderr, result, err)
	}

	if *liveSupervisionAcceptance {
		if *daily || *liveAcceptance || *liveSoakAcceptance {
			fmt.Fprintln(stderr, "-live-supervision-acceptance, -live-soak-acceptance, -live-acceptance, and -daily are mutually exclusive")
			return 2
		}
		result, err := sessionhost.RunLiveSupervisionAcceptance(context.Background(), sessionhost.LiveSupervisionAcceptanceOptions{
			CaseRoot: opt.Target, Goal: liveOpt.Goal, Model: opt.Model, Actor: opt.Actor,
			Timeout: opt.Timeout, MaxAttempts: opt.MaxAttempts, KeepCase: liveOpt.KeepCase, ReceiptPath: liveOpt.ReceiptPath,
			InitializationSourceExecutable: initializationSourceExecutable,
		})
		if strings.TrimSpace(liveOpt.ReceiptPath) != "" {
			result.ReceiptPublication = "published"
			if writeErr := sessionhost.WriteLiveSupervisionAcceptanceReceipt(liveOpt.ReceiptPath, result); writeErr != nil {
				result.Passed = false
				result.ReceiptPublication = "failed"
				result.ReceiptError = writeErr.Error()
				err = errorsJoin(err, writeErr)
			}
		}
		return printResult(stdout, stderr, result, err)
	}

	if *liveAcceptance {
		if *daily || *liveSoakAcceptance {
			fmt.Fprintln(stderr, "-live-acceptance, -live-soak-acceptance, and -daily are mutually exclusive")
			return 2
		}
		liveOpt.CaseRoot = opt.Target
		liveOpt.Pack = opt.Pack
		liveOpt.ClaudePath = opt.ClaudePath
		liveOpt.Model = opt.Model
		liveOpt.Actor = opt.Actor
		liveOpt.Timeout = opt.Timeout
		liveOpt.MaxAttempts = opt.MaxAttempts
		liveOpt.InitializationSourceExecutable = initializationSourceExecutable
		result, err := sessionhost.RunLiveAcceptance(context.Background(), liveOpt)
		result, err = PublishLiveAcceptanceReceipt(liveOpt.ReceiptPath, result, err)
		return printResult(stdout, stderr, result, err)
	}

	if *liveSoakAcceptance {
		if *daily {
			fmt.Fprintln(stderr, "-live-soak-acceptance and -daily are mutually exclusive")
			return 2
		}
		if strings.TrimSpace(opt.Target) != "" || strings.TrimSpace(opt.Pack) != "" || strings.TrimSpace(opt.ClaudePath) != "" || liveOpt.KeepCase {
			fmt.Fprintln(stderr, "live soak acceptance owns disposable cases and canonical Claude discovery; omit -target, -pack, -claude, and -keep-case")
			return 2
		}
		result, err := sessionhost.RunLiveSoakAcceptance(context.Background(), sessionhost.LiveSoakAcceptanceOptions{
			Goal: liveOpt.Goal, Correction: liveOpt.Correction, Model: opt.Model, Actor: opt.Actor,
			Timeout: opt.Timeout, MaxAttempts: opt.MaxAttempts, ReceiptPath: liveOpt.ReceiptPath,
			InitializationSourceExecutable: initializationSourceExecutable,
		})
		result, err = PublishLiveSoakAcceptanceReceipt(liveOpt.ReceiptPath, result, err)
		return printResult(stdout, stderr, result, err)
	}

	if *daily || strings.TrimSpace(liveOpt.Goal) != "" || strings.TrimSpace(liveOpt.Correction) != "" || adoptionRequested || controlRequested || inputRequested || *successorWhatIf || *successorApply || strings.TrimSpace(*successorPublicationStamp) != "" || strings.TrimSpace(*expectedSuccessorPlanSHA256) != "" {
		if strings.TrimSpace(projectRoot) != "" {
			resolved, err := rekitruntime.ResolveProjectLocalTarget(
				projectRoot,
				opt.Target,
				"",
			)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			opt.Target = resolved
		}
		if strings.TrimSpace(opt.ExpectedCurrentDriverRequestSHA256) != "" {
			fmt.Fprintln(stderr, "-expected-current-driver-request-sha256 is supported only by the ordinary rekit-host mode")
			return 2
		}
		if strings.TrimSpace(liveOpt.ReceiptPath) != "" || liveOpt.KeepCase {
			fmt.Fprintln(stderr, "-receipt and -keep-case are supported only by -live-acceptance")
			return 2
		}
		if strings.TrimSpace(opt.Pack) != "" {
			fmt.Fprintln(stderr, "daily front door derives pack from onboarding or attached case metadata; omit -pack")
			return 2
		}
		initializationRepoRoot := ""
		adoptionAction := strings.ToLower(strings.TrimSpace(*directoryAdoptionAction))
		if adoptionAction == "initialize-in-place" || adoptionAction == "confirm-exact-plan" {
			ctx, resolveErr := rekitruntime.New(opt.Target, defaults.DefaultPack)
			if resolveErr != nil {
				fmt.Fprintln(stderr, resolveErr)
				return 1
			}
			initializationRepoRoot = ctx.RepoRoot
		}
		result, err := sessionhost.RunDaily(context.Background(), sessionhost.DailyOptions{
			Target:                      opt.Target,
			Goal:                        liveOpt.Goal,
			Correction:                  liveOpt.Correction,
			SelectedLane:                opt.SelectedLane,
			Control:                     controlOpt,
			ControlWhatIf:               *controlWhatIf,
			ControlApply:                *controlApply,
			DirectoryAdoptionAction:     *directoryAdoptionAction,
			DirectoryAdoptionPack:       *directoryAdoptionPack,
			ExpectedInitPlanSHA256:      *expectedInitPlanSHA256,
			SuccessorWhatIf:             *successorWhatIf,
			SuccessorApply:              *successorApply,
			SuccessorPublicationStamp:   *successorPublicationStamp,
			ExpectedSuccessorPlanSHA256: *expectedSuccessorPlanSHA256,
			Input: sessionhost.DailyInputRequest{
				Mode: *inputMode, ArtifactPath: *inputArtifact, Scope: *inputScope,
			},
			InitializationRepoRoot:         initializationRepoRoot,
			InitializationSourceExecutable: initializationSourceExecutable,
			Actor:                          opt.Actor,
			ClaudePath:                     opt.ClaudePath,
			Model:                          opt.Model,
			Timeout:                        opt.Timeout,
			MaxAttempts:                    opt.MaxAttempts,
		})
		if strings.TrimSpace(projectRoot) != "" {
			projected, projectionErr := projectDailyResultPublicResponse(result, opt.Target)
			if projectionErr != nil {
				fmt.Fprintln(stderr, errorsJoin(err, projectionErr))
				return 1
			}
			result = projected
		}
		return printResult(stdout, stderr, result, err)
	}

	if strings.TrimSpace(liveOpt.ReceiptPath) != "" || liveOpt.KeepCase {
		fmt.Fprintln(stderr, "-receipt and -keep-case are supported only by -live-acceptance")
		return 2
	}
	if strings.TrimSpace(projectRoot) != "" {
		resolved, err := rekitruntime.ResolveProjectLocalTarget(
			projectRoot,
			opt.Target,
			"",
		)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		opt.Target = resolved
	}
	opt.RequireCurrentDriverRequest()
	result, err := sessionhost.Run(context.Background(), opt)
	return printResult(stdout, stderr, result, err)
}

func projectDailyResultPublicResponse(result sessionhost.DailyResult, caseRoot string) (sessionhost.DailyResult, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return sessionhost.DailyResult{}, fmt.Errorf("clone daily public response: %w", err)
	}
	var projected sessionhost.DailyResult
	if err := json.Unmarshal(data, &projected); err != nil {
		return sessionhost.DailyResult{}, fmt.Errorf("clone daily public response: %w", err)
	}
	caseRoot = strings.TrimSpace(caseRoot)
	if caseRoot == "" {
		caseRoot = strings.TrimSpace(projected.CaseRoot)
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return sessionhost.DailyResult{}, fmt.Errorf("resolve daily public entrypoint: %w", err)
	}
	if projected.Completion != nil {
		if err := cli.ProjectCompleteResultForPublicEntrypoint(projected.Completion, entrypoint); err != nil {
			return sessionhost.DailyResult{}, fmt.Errorf("project daily completion public entrypoint: %w", err)
		}
	}
	if projected.CurrentDriverRequest != nil {
		request, err := mission.MissionCommanderDriverRequestForEntrypoint(*projected.CurrentDriverRequest, entrypoint)
		if err != nil {
			return sessionhost.DailyResult{}, fmt.Errorf("project daily current driver request: %w", err)
		}
		projected.CurrentDriverRequest = &request
		projected.CurrentDriverRequestSHA256, err = mission.MissionCommanderDriverRequestSHA256(request)
		if err != nil {
			return sessionhost.DailyResult{}, fmt.Errorf("bind projected daily current driver request: %w", err)
		}
	}
	if projected.ExecutionControl != nil {
		projected.ExecutionControl.ApplyCommand, err = projectstate.ProjectPublicCommand(
			caseRoot,
			projected.ExecutionControl.ApplyCommand,
		)
		if err != nil {
			return sessionhost.DailyResult{}, fmt.Errorf("project daily execution control Apply command: %w", err)
		}
	}
	if adoption := projected.DirectoryAdoption; adoption != nil && adoption.Plan != nil {
		adoption.Plan.ApplyCommand, err = projectstate.ProjectPublicCommand(
			caseRoot,
			adoption.Plan.ApplyCommand,
		)
		if err != nil {
			return sessionhost.DailyResult{}, fmt.Errorf("project daily directory adoption Apply command: %w", err)
		}
	}
	return projected, nil
}

func PublishPackMemoryLiveAcceptanceReceipt(path string, result sessionhost.PackMemoryLiveAcceptanceReceipt, err error) (sessionhost.PackMemoryLiveAcceptanceReceipt, error) {
	if strings.TrimSpace(path) == "" {
		return result, err
	}
	result.ReceiptPublication = "published"
	if writeErr := sessionhost.WritePackMemoryLiveAcceptanceReceipt(path, result); writeErr != nil {
		result.Passed = false
		result.ReceiptPublication = "failed"
		result.ReceiptError = writeErr.Error()
		err = errorsJoin(err, writeErr)
	}
	return result, err
}

func PublishLiveAcceptanceReceipt(path string, result sessionhost.LiveAcceptanceReceipt, err error) (sessionhost.LiveAcceptanceReceipt, error) {
	if strings.TrimSpace(path) == "" {
		return result, err
	}
	result.ReceiptPublication = "published"
	if writeErr := sessionhost.WriteLiveAcceptanceReceipt(path, result); writeErr != nil {
		result.Passed = false
		result.ReceiptPublication = "failed"
		result.ReceiptError = writeErr.Error()
		err = errorsJoin(err, writeErr)
	}
	return result, err
}

func PublishLiveSoakAcceptanceReceipt(path string, result sessionhost.LiveSoakAcceptanceReceipt, err error) (sessionhost.LiveSoakAcceptanceReceipt, error) {
	if strings.TrimSpace(path) == "" {
		return result, err
	}
	result.ReceiptPublication = "published"
	if writeErr := sessionhost.WriteLiveSoakAcceptanceReceipt(path, result); writeErr != nil {
		result.Passed = false
		result.ReceiptPublication = "failed"
		result.ReceiptError = writeErr.Error()
		err = errorsJoin(err, writeErr)
	}
	return result, err
}

func ValidateAdapterFlag(liveAcceptance bool, pack, adapterPath string) error {
	if err := packidentity.Validate(pack); err != nil {
		return err
	}
	adapterPath = strings.TrimSpace(adapterPath)
	if adapterPath != "" && !liveAcceptance {
		return fmt.Errorf("-adapter is supported only by -live-acceptance")
	}
	pack = strings.ToLower(strings.TrimSpace(pack))
	if liveAcceptance && (pack == "" || pack == defaults.DefaultPack) && adapterPath == "" {
		return fmt.Errorf("%s live acceptance requires -adapter with the built rekit-adapter-host executable", defaults.DefaultPack)
	}
	return nil
}

func ValidateExpectedCurrentDriverRequestFlag(value string, ordinaryHost bool) error {
	if strings.TrimSpace(value) != "" && !ordinaryHost {
		return fmt.Errorf("-expected-current-driver-request-sha256 is supported only by the ordinary rekit-host mode")
	}
	return nil
}

func PublicModeRequested(modes ...bool) bool {
	for _, mode := range modes {
		if mode {
			return true
		}
	}
	return false
}

func IsInternalInvocation(args []string) bool {
	for _, arg := range args {
		name := strings.ToLower(strings.TrimSpace(arg))
		if before, _, ok := strings.Cut(name, "="); ok {
			name = before
		}
		switch name {
		case "-internal-supervisor", "--internal-supervisor", "-internal-supervisor-sha256", "--internal-supervisor-sha256",
			"-internal-pack-memory-live-acceptance", "--internal-pack-memory-live-acceptance",
			"-internal-pack-memory-live-acceptance-sha256", "--internal-pack-memory-live-acceptance-sha256":
			return true
		}
	}
	return false
}

func printResult(stdout, stderr io.Writer, result any, err error) int {
	data, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr == nil {
		fmt.Fprintln(stdout, string(data))
	} else {
		err = errorsJoin(err, marshalErr)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func errorsJoin(a, b error) error {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return fmt.Errorf("%v; %w", a, b)
}
