package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/attach"
	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaultdocs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/overview"
	"github.com/shuiyu486/re-context-kits/internal/rekit/promote"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
	"github.com/shuiyu486/re-context-kits/internal/rekit/repair"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type Options struct {
	Command            string
	Target             string
	Pack               string
	Review             bool
	Apply              bool
	CreateCandidates   bool
	WhatIf             bool
	Force              bool
	List               bool
	ReviewOutputDir    string
	PacketPath         string
	ReviewerResultPath string
	DiffPath           string
	ProjectName        string
	Route              string
	TaskType           string
	Items              string
	ItemsFile          string
	ItemsPerAgent      int
	MaxParallel        int
	Format             string
	Gate               gate.Options
	Note               note.Options
	Start              workstream.StartOptions
	Handoff            workstream.HandoffOptions
	Continue           workstream.ContinueOptions
	Reconcile          workstream.ReconcileOptions
}

func Parse(args []string) (Options, error) {
	opt := Options{Command: commands.DefaultCommand, Pack: defaults.DefaultPack}
	for i := 0; i < len(args); i++ {
		if strings.EqualFold(args[i], "--") {
			continue
		}
		switch args[i] {
		case "-Command", "--command":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Command")
			}
			opt.Command = args[i]
		case "-Target", "--target":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Target")
			}
			opt.Target = args[i]
		case "-Pack", "--pack":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Pack")
			}
			opt.Pack = args[i]
		case "-ProjectName", "--project-name":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ProjectName")
			}
			opt.ProjectName = args[i]
		case "-Name", "--name":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Name")
			}
			opt.Start.Name = args[i]
			opt.Start.Selector = args[i]
		case "-Review", "--review":
			opt.Review = true
		case "-Apply", "--apply":
			opt.Apply = true
		case "-CreateCandidates", "--create-candidates":
			opt.CreateCandidates = true
		case "-WhatIf", "--what-if":
			opt.WhatIf = true
		case "-Force", "--force":
			opt.Force = true
		case "-List", "--list":
			opt.List = true
		case "-ReviewOutputDir", "--review-output-dir":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ReviewOutputDir")
			}
			opt.ReviewOutputDir = args[i]
		case "-PacketPath", "--packet-path":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -PacketPath")
			}
			opt.PacketPath = args[i]
		case "-ReviewerResultPath", "--reviewer-result-path":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ReviewerResultPath")
			}
			opt.ReviewerResultPath = args[i]
		case "-DiffPath", "--diff-path":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -DiffPath")
			}
			opt.DiffPath = args[i]
		case "-Action", "--action":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Action")
			}
			opt.Gate.Action = args[i]
			opt.Note.Action = args[i]
		case "-Lane", "--lane":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Lane")
			}
			opt.Gate.Lane = args[i]
			opt.Note.Lane = args[i]
			opt.Continue.Selector = args[i]
			opt.Reconcile.Selector = args[i]
		case "-Kind", "--kind":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Kind")
			}
			opt.Note.Kind = args[i]
		case "-Related", "--related":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Related")
			}
			opt.Note.Related = args[i]
		case "-Confidence", "--confidence":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Confidence")
			}
			opt.Note.Confidence = args[i]
		case "-Decision", "--decision":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Decision")
			}
			opt.Note.Decision = args[i]
		case "-Reason", "--reason":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Reason")
			}
			opt.Note.Reason = args[i]
			opt.Reconcile.Reason = args[i]
			opt.Start.TakeoverReason = args[i]
		case "-Status", "--status":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Status")
			}
			opt.Note.Status = args[i]
		case "-Verifier", "--verifier":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Verifier")
			}
			opt.Note.Verifier = args[i]
		case "-Verdict", "--verdict":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Verdict")
			}
			opt.Note.Verdict = args[i]
		case "-ApprovedBy", "--approved-by":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ApprovedBy")
			}
			opt.Note.ApprovedBy = args[i]
		case "-Expires", "--expires":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Expires")
			}
			opt.Note.Expires = args[i]
		case "-EvidenceRefs", "--evidence-refs":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -EvidenceRefs")
			}
			opt.Note.EvidenceRefs = args[i]
		case "-EventId", "--event-id":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -EventId")
			}
			opt.Note.EventID = args[i]
			opt.Reconcile.InterventionID = args[i]
		case "-InterventionId", "--intervention-id":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -InterventionId")
			}
			opt.Reconcile.InterventionID = args[i]
		case "-Executor", "--executor":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Executor")
			}
			opt.Reconcile.Executor = args[i]
			opt.Start.Executor = args[i]
		case "-Subject", "--subject":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Subject")
			}
			opt.Gate.Subject = args[i]
			opt.Note.Subject = args[i]
		case "-Summary", "--summary":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Summary")
			}
			opt.Gate.Summary = args[i]
			opt.Note.Summary = args[i]
			opt.Reconcile.Summary = args[i]
		case "-Actor", "--actor":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Actor")
			}
			opt.Gate.Actor = args[i]
			opt.Note.Actor = args[i]
			opt.Reconcile.Actor = args[i]
			opt.Start.Actor = args[i]
		case "-Risk", "--risk":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Risk")
			}
			opt.Gate.Risk = args[i]
			opt.Note.Risk = args[i]
		case "-TargetRef", "--target-ref":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -TargetRef")
			}
			opt.Gate.TargetRef = args[i]
			opt.Note.Target = args[i]
		case "-BatchId", "--batch-id":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -BatchId")
			}
			opt.Gate.BatchID = args[i]
			opt.Note.BatchID = args[i]
		case "-Scope", "--scope":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Scope")
			}
			opt.Gate.Scope = args[i]
			opt.Note.Scope = args[i]
		case "-Budget", "--budget":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Budget")
			}
			opt.Gate.Budget = args[i]
		case "-RuntimeSeconds", "--runtime-seconds":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -RuntimeSeconds")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opt, fmt.Errorf("invalid -RuntimeSeconds: %s", args[i])
			}
			opt.Gate.RuntimeSeconds = n
		case "-DiskMB", "--disk-mb":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -DiskMB")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opt, fmt.Errorf("invalid -DiskMB: %s", args[i])
			}
			opt.Gate.DiskMB = n
		case "-Requests", "--requests":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Requests")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opt, fmt.Errorf("invalid -Requests: %s", args[i])
			}
			opt.Gate.Requests = n
		case "-OutputPaths", "--output-paths":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -OutputPaths")
			}
			opt.Gate.OutputPaths = args[i]
		case "-TriedLightSteps", "--tried-light-steps":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -TriedLightSteps")
			}
			opt.Gate.TriedLightSteps = args[i]
		case "-StopConditions", "--stop-conditions":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -StopConditions")
			}
			opt.Gate.StopConditions = args[i]
		case "-GateEventId", "--gate-event-id":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -GateEventId")
			}
			opt.Gate.GateEventID = args[i]
		case "-ExecutionStatus", "--execution-status":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ExecutionStatus")
			}
			opt.Gate.ExecutionStatus = args[i]
		case "-ActualRuntimeSeconds", "--actual-runtime-seconds":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ActualRuntimeSeconds")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opt, fmt.Errorf("invalid -ActualRuntimeSeconds: %s", args[i])
			}
			opt.Gate.ActualRuntimeSeconds = n
		case "-ActualDiskMB", "--actual-disk-mb":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ActualDiskMB")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opt, fmt.Errorf("invalid -ActualDiskMB: %s", args[i])
			}
			opt.Gate.ActualDiskMB = n
		case "-ActualRequests", "--actual-requests":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ActualRequests")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opt, fmt.Errorf("invalid -ActualRequests: %s", args[i])
			}
			opt.Gate.ActualRequests = n
		case "-OutputRefs", "--output-refs":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -OutputRefs")
			}
			opt.Gate.OutputRefs = args[i]
		case "-ExecutionEvidenceRefs", "--execution-evidence-refs":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ExecutionEvidenceRefs")
			}
			opt.Gate.EvidenceRefs = args[i]
		case "-BoundaryHits", "--boundary-hits":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -BoundaryHits")
			}
			opt.Gate.BoundaryHits = args[i]
		case "-Escalation", "--escalation":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Escalation")
			}
			opt.Gate.Escalation = args[i]
		case "-ExecutionReportPath", "--execution-report-path":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ExecutionReportPath")
			}
			opt.Gate.ExecutionReportPath = args[i]
		case "-ExecutionReportContract", "--execution-report-contract":
			opt.Gate.ExecutionReportContract = true
		case "-ValidateExecutionReport", "--validate-execution-report":
			opt.Gate.ValidateExecutionReport = true
		case "-Format", "--format":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Format")
			}
			opt.Format = args[i]
		case "-Route", "--route":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Route")
			}
			opt.Route = args[i]
		case "-TaskType", "--task-type":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -TaskType")
			}
			opt.TaskType = args[i]
		case "-Items", "--items":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Items")
			}
			opt.Items = args[i]
		case "-ItemsFile", "--items-file":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ItemsFile")
			}
			opt.ItemsFile = args[i]
		case "-ItemsPerAgent", "--items-per-agent":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -ItemsPerAgent")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opt, fmt.Errorf("invalid -ItemsPerAgent: %s", args[i])
			}
			opt.ItemsPerAgent = n
		case "-MaxParallel", "--max-parallel":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -MaxParallel")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opt, fmt.Errorf("invalid -MaxParallel: %s", args[i])
			}
			opt.MaxParallel = n
		default:
			if i == 0 && args[i] != "" && args[i][0] != '-' {
				opt.Command = args[i]
			} else if strings.EqualFold(opt.Command, "start") && args[i] != "" && args[i][0] != '-' {
				if opt.Start.Name == "" {
					opt.Start.Name = args[i]
					opt.Start.Selector = args[i]
				} else {
					opt.Start.Name += "-" + args[i]
					opt.Start.Selector += "-" + args[i]
				}
			} else if strings.EqualFold(opt.Command, "handoff") && args[i] != "" && args[i][0] != '-' {
				if opt.Handoff.Selector == "" {
					opt.Handoff.Selector = args[i]
				} else {
					opt.Handoff.Selector += "-" + args[i]
				}
			} else if strings.EqualFold(opt.Command, "continue") && args[i] != "" && args[i][0] != '-' {
				if opt.Continue.Selector == "" {
					opt.Continue.Selector = args[i]
				} else {
					opt.Continue.Selector += "-" + args[i]
				}
			} else if strings.EqualFold(opt.Command, "reconcile") && args[i] != "" && args[i][0] != '-' {
				if opt.Reconcile.Selector == "" {
					opt.Reconcile.Selector = args[i]
				} else {
					opt.Reconcile.Selector += "-" + args[i]
				}
			}
		}
	}
	if opt.Command == "" {
		opt.Command = commands.DefaultCommand
	}
	if opt.Pack == "" {
		opt.Pack = defaults.DefaultPack
	}
	return opt, nil
}

func runtimeCwdOverride(opt Options) string {
	if opt.Command != commands.Gate || strings.TrimSpace(opt.Target) != "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv("REKIT_CALLER_CWD"))
}

func Run(args []string, stdout io.Writer) error {
	opt, err := Parse(args)
	if err != nil {
		return err
	}
	ctx, err := runtime.NewWithCwd(opt.Target, opt.Pack, runtimeCwdOverride(opt))
	if err != nil {
		return err
	}
	if strings.TrimSpace(opt.ReviewerResultPath) != "" && opt.Command != commands.PlanSubagents {
		return fmt.Errorf("-ReviewerResultPath is supported only by plan-subagents reviewer intake")
	}
	switch opt.Command {
	case commands.Status:
		return runStatus(ctx, opt, stdout)
	case commands.Packs:
		return runPacks(ctx, opt, stdout)
	case commands.ReleaseCheck:
		return runReleaseCheck(ctx, opt, stdout)
	case commands.Doctor, commands.Validate:
		return runDoctor(ctx, opt, stdout)
	case commands.Attach:
		return runAttach(ctx, opt, stdout)
	case commands.Repair:
		return runRepair(ctx, opt, stdout)
	case commands.Init, commands.Bootstrap:
		return runInitBootstrap(ctx, opt, stdout)
	case commands.Sync, commands.Update:
		return runSyncReview(ctx, opt, stdout)
	case commands.Promote:
		return runPromoteReview(ctx, opt, stdout)
	case commands.Overview:
		return runOverview(ctx, opt, stdout)
	case commands.Start:
		return runStart(ctx, opt, stdout)
	case commands.Handoff:
		return runHandoff(ctx, opt, stdout)
	case commands.Continue:
		return runContinue(ctx, opt, stdout)
	case commands.Reconcile:
		return runReconcile(ctx, opt, stdout)
	case commands.PlanSubagents:
		return runPlanSubagents(ctx, opt, stdout)
	case commands.Gate:
		return runGate(ctx, opt, stdout)
	case commands.Note:
		return runNote(ctx, opt, stdout)
	default:
		return commands.UnsupportedError(opt.Command)
	}
}

func Main() int {
	if err := Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

type packsInventory struct {
	Command       string                 `json:"command"`
	SchemaVersion int                    `json:"schemaVersion"`
	IsMutation    bool                   `json:"isMutation"`
	PackCount     int                    `json:"packCount"`
	Packs         []manifest.PackSummary `json:"packs"`
}

func runReleaseCheck(ctx runtime.Context, opt Options, out io.Writer) error {
	if ctx.TargetProvided {
		return fmt.Errorf("release-check runs against the kit repo; omit -Target")
	}
	if opt.Apply || opt.WhatIf || opt.CreateCandidates || opt.Review || opt.Force || opt.List || wantsReviewArtifacts(opt) {
		return fmt.Errorf("release-check is read-only and does not accept mutation, review artifact, or list flags")
	}
	result, err := releasecheck.Build(ctx.RepoRoot)
	if err != nil {
		return err
	}
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	return writeReleaseCheckResult(out, result, format)
}

func writeReleaseCheckResult(out io.Writer, result releasecheck.Result, format string) error {
	if err := emitReleaseCheckResult(out, result, format); err != nil {
		return err
	}
	if result.Ready {
		return nil
	}
	if releasecheck.ReleaseCheckResultCountsFor(result).Warnings == 0 {
		return fmt.Errorf("release-check not ready")
	}
	return fmt.Errorf("release-check not ready: %s", strings.Join(result.Warnings, "; "))
}

func emitReleaseCheckResult(out io.Writer, result releasecheck.Result, format string) error {
	switch format {
	case "table", "text", "tsv":
		resultCounts := releasecheck.ReleaseCheckResultCountsFor(result)
		fmt.Fprintf(out, "release-check: %s\n", result.Summary)
		fmt.Fprintf(out, "ready: %t\n", result.Ready)
		fmt.Fprintf(out, "gate profile: %s ready=%t steps=%d largeMatrixDefault=%t\n", result.GateProfile.Name, result.GateProfile.Ready, resultCounts.GateProfileSteps, result.GateProfile.LargeMatrixDefault)
		ciGateCounts := releasecheck.CIReleaseGateCountsFor(result.CIReleaseGate)
		fmt.Fprintf(out, "CI release gate: %s ready=%t jobs=%d commands=%d forbidden=%d\n", result.CIReleaseGate.WorkflowPath, result.CIReleaseGate.Ready, ciGateCounts.Jobs, ciGateCounts.RequiredCommands, ciGateCounts.ForbiddenStrings)
		if ciGateCounts.Warnings > 0 {
			fmt.Fprintln(out, "CI release gate warnings:")
			for _, warning := range result.CIReleaseGate.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		fmt.Fprintln(out, "required commands:")
		for _, step := range result.RequiredCommands {
			status := "catalog"
			if !step.InCatalog {
				status = "missing-from-catalog"
			} else if !step.Present {
				status = "missing-path"
			}
			pathSuffix := ""
			if step.RepoPath != "" {
				pathSuffix = fmt.Sprintf(" path=%s", step.RepoPath)
			}
			fmt.Fprintf(out, "- [%s] %s kind=%s%s\n", status, step.Command, step.Kind, pathSuffix)
		}
		fmt.Fprintln(out, "documents:")
		for _, doc := range result.Documents {
			status := "ok"
			if !doc.Present {
				status = "missing"
			}
			fmt.Fprintf(out, "- [%s] %s (%s)\n", status, doc.Path, doc.Purpose)
		}
		fmt.Fprintf(out, "packs: %d\n", resultCounts.Packs)
		fmt.Fprintf(out, "heavy-tool gate actions: %s\n", strings.Join(result.HeavyToolGateActions, ","))
		powerShellCounts := releasecheck.PowerShellDeprecationCountsFor(result.PowerShellDeprecation)
		fmt.Fprintf(out, "PowerShell deprecation: %s ready=%t commands=%d modules=%d freezeGates=%d blocked=%d fallbackRetirement=%t noFallback=%d candidates=%d removalModules=%d retiredModules=%d facadeRuntime=%t legacyImports=%t dispatcher=%t publicFacade=%t retained=%t facadeCommands=%d noFallback=%d moduleRemoval=%t removalCandidates=%d retired=%d facadeDeps=%d undocumented=%d moduleReferences=%t activeTests=%d fixtures=%d blockers=%d unclassified=%d\n", result.PowerShellDeprecation.Summary, result.PowerShellDeprecation.Ready, powerShellCounts.CommandOwnership, powerShellCounts.ModuleStatus, powerShellCounts.FreezeGates, powerShellCounts.BlockedMigrations, result.PowerShellDeprecation.FallbackRetirement.Ready, powerShellCounts.FallbackNoFallbackCommands, powerShellCounts.FallbackCandidateCommands, powerShellCounts.FallbackRemovalCandidateModules, powerShellCounts.FallbackRetiredModules, result.PowerShellDeprecation.FacadeRuntime.Ready, result.PowerShellDeprecation.FacadeRuntime.LegacyModuleImportsPresent, result.PowerShellDeprecation.FacadeRuntime.CommandDispatcherPresent, result.PowerShellDeprecation.PublicFacade.Ready, result.PowerShellDeprecation.PublicFacade.Retained, powerShellCounts.PublicFacadeCommandSurface, powerShellCounts.PublicFacadeNoFallbackCommands, result.PowerShellDeprecation.ModuleRemoval.Ready, powerShellCounts.ModuleRemovalCandidateModules, powerShellCounts.ModuleRemovalRetiredModules, powerShellCounts.ModuleRemovalFacadeRuntimeDependencies, powerShellCounts.ModuleRemovalUndocumentedModules, result.PowerShellDeprecation.ModuleReferences.Ready, powerShellCounts.ModuleReferencesActiveTestDependencies, powerShellCounts.ModuleReferencesCompatibilityFixtures, powerShellCounts.ModuleReferencesRemovalBlockers, powerShellCounts.ModuleReferencesUnclassifiedReferences)
		surfaceCounts := releasecheck.GoNativePublicSurfaceCountsFor(result.GoNativePublicSurface)
		fmt.Fprintf(out, "Go-native public surface: %s ready=%t entrypoint=%s present=%t catalog=%s catalogPresent=%t default=%s commands=%d handlers=%d symbols=%d profiles=%d boundaries=%d boundaryRows=%d policyRows=%d policyViolations=%d facadeRemovalReady=%t facadePrerequisites=%d readOnly=%d mutating=%d writesCase=%d writesKit=%d reviewFirst=%d applyRequired=%d heavyTool=%d authorityConfirmed=%d readOnlyCommands=%s reviewFirstCommands=%s writesKitCommands=%s caseLocalApplyCommands=%s caseLocalReviewWritebackCommands=%s kitReviewFirstCommands=%s alternative=%s unsupportedDiagnostic=%t\n", result.GoNativePublicSurface.Summary, result.GoNativePublicSurface.Ready, result.GoNativePublicSurface.Entrypoint, result.GoNativePublicSurface.EntrypointPresent, result.GoNativePublicSurface.CommandCatalogPath, result.GoNativePublicSurface.CommandCatalogPresent, result.GoNativePublicSurface.DefaultCommand, surfaceCounts.Commands, surfaceCounts.HandlerCommands, surfaceCounts.SymbolCommands, surfaceCounts.CommandProfiles, surfaceCounts.MutationBoundaries, surfaceCounts.BoundaryRows, surfaceCounts.PolicyRows, surfaceCounts.PolicyViolations, result.GoNativePublicSurface.FacadeRemovalReady, surfaceCounts.FacadePrerequisites, surfaceCounts.ReadOnly, surfaceCounts.Mutating, surfaceCounts.WritesCase, surfaceCounts.WritesKit, surfaceCounts.ReviewFirst, surfaceCounts.ApplyRequired, surfaceCounts.HeavyTool, surfaceCounts.AuthorityConfirmed, strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ReadOnly, ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ReviewFirst, ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.WritesKit, ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ByBoundary[commands.BoundaryCaseLocalApply], ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ByBoundary[commands.BoundaryCaseLocalReviewWriteback], ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ByBoundary[commands.BoundaryKitReviewFirst], ","), result.GoNativePublicSurface.AlternativePattern, result.GoNativePublicSurface.UnsupportedCommandDiagnosticPresent)
		facadeRemovalCounts := releasecheck.PublicFacadeRemovalCountsFor(result.PublicFacadeRemoval)
		planCounts := facadeRemovalCounts.Plan
		deletionGateCounts := facadeRemovalCounts.DeletionGates
		executionCounts := facadeRemovalCounts.ExecutionSteps
		impactCounts := facadeRemovalCounts.Impact
		fmt.Fprintf(out, "public facade removal: %s ready=%t prerequisites=%d removalPlan=%t planChecks=%d replacementEntrypoints=%d replacementValidationCommands=%d deletionGates=%d deletionGateValidationCommands=%d deletionGateExitCriteria=%d deletionGateFailureSignals=%d deletionGateEscalationTriggers=%d deletionGateEscalationEvidence=%d deletionGateEscalationRecipients=%d deletionGateEscalationHandoffSteps=%d deletionGateEscalationDecisionOptions=%d deletionGateEscalationRetryConditions=%d deletionGateEscalationStopConditions=%d deletionGateEscalationResolutionArtifacts=%d deletionGateEscalationClosureChecks=%d deletionGateEscalationReopenConditions=%d deletionGateEscalationLedgerEvents=%d deletionGateEscalationStateTransitions=%d deletionGateEscalationBoundaryGuards=%d deletionGateEscalationAuditChecks=%d deletionGateVerificationArtifacts=%d deletionGateBlockedExecutionSteps=%d deletionGateRemediationActions=%d recoverySteps=%d recoveryValidationCommands=%d documentationTargets=%d documentationValidationCommands=%d executionSteps=%d executionFailureSignals=%d executionRemediationActions=%d executionVerificationArtifacts=%d executionLedgerEvents=%d executionStateTransitions=%d executionEscalationTriggers=%d executionEscalationEvidence=%d executionEscalationRecipients=%d executionEscalationHandoffSteps=%d executionEscalationDecisionOptions=%d executionEscalationRetryConditions=%d executionEscalationStopConditions=%d executionEscalationResolutionArtifacts=%d executionEscalationClosureChecks=%d executionEscalationReopenConditions=%d executionEscalationLedgerEvents=%d executionEscalationStateTransitions=%d executionEscalationBoundaryGuards=%d executionEscalationAuditChecks=%d executionBoundaryGuards=%d executionAuditChecks=%d executionValidationCommands=%d boundaryChecks=%d boundaryValidationCommands=%d removalImpact=%t impactReferences=%d impactCategories=%d workItems=%d validationCommands=%d migrationTargets=%d migrationValidationCommands=%d smokeMigrationTargets=%d smokeMigrationValidationCommands=%d unclassified=%d\n", result.PublicFacadeRemoval.Summary, result.PublicFacadeRemoval.Ready, facadeRemovalCounts.Prerequisites, result.PublicFacadeRemoval.RemovalPlan.Ready, planCounts.RequiredPhrases, planCounts.ReplacementEntrypoints, planCounts.ReplacementValidationCommands, deletionGateCounts.Gates, deletionGateCounts.ValidationCommands, deletionGateCounts.ExitCriteria, deletionGateCounts.FailureSignals, deletionGateCounts.EscalationTriggers, deletionGateCounts.EscalationEvidence, deletionGateCounts.EscalationRecipients, deletionGateCounts.EscalationHandoffSteps, deletionGateCounts.EscalationDecisionOptions, deletionGateCounts.EscalationRetryConditions, deletionGateCounts.EscalationStopConditions, deletionGateCounts.EscalationResolutionArtifacts, deletionGateCounts.EscalationClosureChecks, deletionGateCounts.EscalationReopenConditions, deletionGateCounts.EscalationLedgerEvents, deletionGateCounts.EscalationStateTransitions, deletionGateCounts.EscalationBoundaryGuards, deletionGateCounts.EscalationAuditChecks, deletionGateCounts.VerificationArtifacts, deletionGateCounts.BlockedExecutionSteps, deletionGateCounts.RemediationActions, planCounts.RecoverySteps, planCounts.RecoveryValidationCommands, planCounts.DocumentationTargets, planCounts.DocumentationValidationCommands, executionCounts.Steps, executionCounts.FailureSignals, executionCounts.RemediationActions, executionCounts.VerificationArtifacts, executionCounts.LedgerEvents, executionCounts.StateTransitions, executionCounts.EscalationTriggers, executionCounts.EscalationEvidence, executionCounts.EscalationRecipients, executionCounts.EscalationHandoffSteps, executionCounts.EscalationDecisionOptions, executionCounts.EscalationRetryConditions, executionCounts.EscalationStopConditions, executionCounts.EscalationResolutionArtifacts, executionCounts.EscalationClosureChecks, executionCounts.EscalationReopenConditions, executionCounts.EscalationLedgerEvents, executionCounts.EscalationStateTransitions, executionCounts.EscalationBoundaryGuards, executionCounts.EscalationAuditChecks, executionCounts.BoundaryGuards, executionCounts.AuditChecks, executionCounts.ValidationCommands, planCounts.BoundaryChecks, planCounts.BoundaryValidationCommands, result.PublicFacadeRemoval.RemovalImpact.Ready, impactCounts.References, impactCounts.ReferenceCategories, impactCounts.WorkItems, impactCounts.WorkItemValidationCommands, impactCounts.MigrationTargets, impactCounts.MigrationValidationCommands, impactCounts.SmokeMigrationTargets, impactCounts.SmokeMigrationValidationCommands, impactCounts.UnclassifiedReferences)
		caseShimCounts := caseshim.ReadinessCountsFor(result.CaseShim)
		publicDefaultDocCounts := defaultdocs.ReadinessCountsFor(result.PublicDefaultDocs)
		fmt.Fprintf(out, "case shim: %s ready=%t required=%d canonical=%d forbidden=%d\n", result.CaseShim.Summary, result.CaseShim.Ready, caseShimCounts.RequiredPhrases, caseShimCounts.CanonicalSkillPhrases, caseShimCounts.ForbiddenStrings)
		fmt.Fprintf(out, "public default docs: %s ready=%t documents=%d required=%d forbiddenCommands=%d forbiddenShellFences=%d\n", result.PublicDefaultDocs.Summary, result.PublicDefaultDocs.Ready, publicDefaultDocCounts.Documents, publicDefaultDocCounts.RequiredPhrases, publicDefaultDocCounts.ForbiddenCommands, publicDefaultDocCounts.ForbiddenShellFences)
		handoffCounts := releasecheck.ReleaseHandoffCountsFor(result.ReleaseHandoff)
		fmt.Fprintf(out, "release handoff: %s ready=%t readFirst=%d signals=%d knownGaps=%d packMaturity=%d validation=%d releaseNotes=%t latest=%s\n", result.ReleaseHandoff.Summary, result.ReleaseHandoff.Ready, handoffCounts.ReadFirst, handoffCounts.Signals, handoffCounts.KnownGaps, handoffCounts.PackMaturity.Total, handoffCounts.Validation, result.ReleaseHandoff.ReleaseNotes.Covered, result.ReleaseHandoff.LatestBatch.Title)
		if surfaceCounts.Warnings > 0 {
			fmt.Fprintln(out, "Go-native public surface warnings:")
			for _, warning := range result.GoNativePublicSurface.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if facadeRemovalCounts.Warnings > 0 {
			fmt.Fprintln(out, "public facade removal warnings:")
			for _, warning := range result.PublicFacadeRemoval.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if caseShimCounts.Warnings > 0 {
			fmt.Fprintln(out, "case shim warnings:")
			for _, warning := range result.CaseShim.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if publicDefaultDocCounts.Warnings > 0 {
			fmt.Fprintln(out, "public default docs warnings:")
			for _, warning := range result.PublicDefaultDocs.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if handoffCounts.Warnings > 0 {
			fmt.Fprintln(out, "release handoff warnings:")
			for _, warning := range result.ReleaseHandoff.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if powerShellCounts.Warnings > 0 {
			fmt.Fprintln(out, "PowerShell deprecation warnings:")
			for _, warning := range result.PowerShellDeprecation.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if resultCounts.KnownGaps > 0 {
			fmt.Fprintln(out, "known gaps:")
			for _, gap := range result.KnownGaps {
				fmt.Fprintf(out, "- %s\n", gap)
			}
		}
		if resultCounts.Warnings > 0 {
			fmt.Fprintln(out, "warnings:")
			for _, warning := range result.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	default:
		return fmt.Errorf("unsupported release-check format: %s", format)
	}
	return nil
}

func runPacks(ctx runtime.Context, opt Options, out io.Writer) error {
	packs, err := manifest.List(ctx.RepoRoot)
	if err != nil {
		return err
	}
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	switch format {
	case "table", "tsv":
		fmt.Fprintln(out, "pack\tmaturity\tschema\tmanifestSchema\troutes\tmanaged\ttooling\tauthority\tversion\tdescription")
		for _, pack := range packs {
			schema := "ok"
			if !pack.SchemaValid {
				schema = "error"
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\t%s\n", pack.ID, pack.Maturity, schema, pack.SchemaVersion, pack.SubagentRoutes, pack.ManagedFiles, pack.ToolingFiles, pack.DefaultAuthorityLane, pack.Version, pack.Description)
			if pack.Error != "" {
				fmt.Fprintf(out, "  error: %s\n", pack.Error)
			}
		}
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(packsInventory{Command: "packs", SchemaVersion: 1, IsMutation: false, PackCount: len(packs), Packs: packs})
	default:
		return fmt.Errorf("unsupported packs format: %s", opt.Format)
	}
	return nil
}

type statusInventory struct {
	Command        string                 `json:"command"`
	SchemaVersion  int                    `json:"schemaVersion"`
	IsMutation     bool                   `json:"isMutation"`
	RuntimeRoot    string                 `json:"runtimeRoot"`
	TemplateRoot   string                 `json:"templateRoot"`
	Pack           string                 `json:"pack"`
	Target         string                 `json:"target"`
	TargetProvided bool                   `json:"targetProvided"`
	Mode           string                 `json:"mode"`
	Case           *statusCase            `json:"case"`
	Manifest       *statusManifestSummary `json:"manifest"`
	CaseShim       statusCaseShim         `json:"caseShim"`
}

type statusCase struct {
	CaseRoot            string `json:"caseRoot"`
	MetadataSource      string `json:"metadataSource"`
	InstancePath        string `json:"instancePath"`
	TemplateRoot        string `json:"templateRoot"`
	TemplatePack        string `json:"templatePack"`
	ProjectName         string `json:"projectName"`
	ProjectRoot         string `json:"projectRoot"`
	Moved               bool   `json:"moved"`
	ShimPath            string `json:"shimPath"`
	ShimMatchesTemplate bool   `json:"shimMatchesTemplate"`
}

type statusCaseShim struct {
	Ready                 bool     `json:"ready"`
	Summary               string   `json:"summary"`
	TemplatePath          string   `json:"templatePath"`
	CanonicalSkillPath    string   `json:"canonicalSkillPath"`
	InstalledShimPath     string   `json:"installedShimPath,omitempty"`
	InstalledShimMatches  *bool    `json:"installedShimMatchesTemplate,omitempty"`
	RequiredPhrases       int      `json:"requiredPhrases"`
	CanonicalSkillPhrases int      `json:"canonicalSkillPhrases"`
	ForbiddenStrings      int      `json:"forbiddenStrings"`
	Boundaries            int      `json:"boundaries"`
	Warnings              []string `json:"warnings,omitempty"`
}

type statusManifestSummary struct {
	ManifestPath  string `json:"manifestPath"`
	SchemaVersion string `json:"schemaVersion"`
	ManagedFiles  int    `json:"managedFiles"`
	PromoteFiles  int    `json:"promoteFiles"`
	ToolingFiles  int    `json:"toolingFiles"`
}

func runStatus(ctx runtime.Context, opt Options, out io.Writer) error {
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	switch format {
	case "table", "text", "tsv":
		return runStatusText(ctx, out)
	case "json":
		status, err := buildStatusInventory(ctx)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	default:
		return fmt.Errorf("unsupported status format: %s", opt.Format)
	}
}

func runStatusText(ctx runtime.Context, out io.Writer) error {
	caseShim := buildStatusCaseShim(ctx.RepoRoot, "")
	fmt.Fprintf(out, "rekit go backend: %s\n", ctx.RuntimeRoot)
	fmt.Fprintf(out, "template root: %s\n", ctx.RepoRoot)
	fmt.Fprintf(out, "pack: %s\n", ctx.Pack)
	if instance.LooksLikeCase(ctx.Target) {
		inst, err := instance.Read(ctx.Target)
		if err != nil {
			return err
		}
		caseShim = buildStatusCaseShim(ctx.RepoRoot, inst.CaseRoot)
		fmt.Fprintf(out, "case: %s\n", inst.CaseRoot)
		fmt.Fprintf(out, "case metadata: %s %s\n", inst.Source, inst.InstancePath)
		fmt.Fprintf(out, "case templateRoot: %s\n", inst.TemplateRoot)
		fmt.Fprintf(out, "case templatePack: %s\n", inst.TemplatePack)
		fmt.Fprintf(out, "case shim: %s ready=%t installed=%s matchesTemplate=%t\n", caseShim.Summary, caseShim.Ready, caseShim.InstalledShimPath, boolPtrValue(caseShim.InstalledShimMatches))
		if inst.Moved() {
			fmt.Fprintln(out, "detected moved case metadata")
		}
		return nil
	}
	fmt.Fprintf(out, "case shim: %s ready=%t\n", caseShim.Summary, caseShim.Ready)
	m, err := manifest.Load(ctx.RepoRoot, ctx.Pack)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "manifest: %s\n", m.ManifestPath)
	fmt.Fprintf(out, "managed files: %d\n", len(m.ManagedFiles))
	fmt.Fprintf(out, "promote files: %d\n", len(m.PromoteFiles))
	fmt.Fprintf(out, "tooling files: %d\n", len(m.ToolingFiles))
	return nil
}

func buildStatusInventory(ctx runtime.Context) (statusInventory, error) {
	status := statusInventory{
		Command:        "status",
		SchemaVersion:  1,
		IsMutation:     false,
		RuntimeRoot:    ctx.RuntimeRoot,
		TemplateRoot:   ctx.RepoRoot,
		Pack:           ctx.Pack,
		Target:         ctx.Target,
		TargetProvided: ctx.TargetProvided,
		Mode:           "kit",
		CaseShim:       buildStatusCaseShim(ctx.RepoRoot, ""),
	}
	if instance.LooksLikeCase(ctx.Target) {
		inst, err := instance.Read(ctx.Target)
		if err != nil {
			return statusInventory{}, err
		}
		status.Mode = "case"
		caseShim := buildStatusCaseShim(ctx.RepoRoot, inst.CaseRoot)
		status.CaseShim = caseShim
		status.Case = &statusCase{
			CaseRoot:            inst.CaseRoot,
			MetadataSource:      inst.Source,
			InstancePath:        inst.InstancePath,
			TemplateRoot:        inst.TemplateRoot,
			TemplatePack:        inst.TemplatePack,
			ProjectName:         inst.ProjectName,
			ProjectRoot:         inst.ProjectRoot,
			Moved:               inst.Moved(),
			ShimPath:            caseShim.InstalledShimPath,
			ShimMatchesTemplate: boolPtrValue(caseShim.InstalledShimMatches),
		}
		return status, nil
	}
	m, err := manifest.Load(ctx.RepoRoot, ctx.Pack)
	if err != nil {
		return statusInventory{}, err
	}
	status.Manifest = &statusManifestSummary{
		ManifestPath:  m.ManifestPath,
		SchemaVersion: m.SchemaVersion,
		ManagedFiles:  len(m.ManagedFiles),
		PromoteFiles:  len(m.PromoteFiles),
		ToolingFiles:  len(m.ToolingFiles),
	}
	return status, nil
}

func buildStatusCaseShim(repoRoot, caseRoot string) statusCaseShim {
	readiness := caseshim.Inspect(repoRoot)
	counts := caseshim.ReadinessCountsFor(readiness)
	shim := statusCaseShim{
		Ready:                 readiness.Ready,
		Summary:               readiness.Summary,
		TemplatePath:          filepath.Join(repoRoot, filepath.FromSlash(readiness.TemplatePath)),
		CanonicalSkillPath:    filepath.Join(repoRoot, filepath.FromSlash(readiness.CanonicalSkillPath)),
		RequiredPhrases:       counts.RequiredPhrases,
		CanonicalSkillPhrases: counts.CanonicalSkillPhrases,
		ForbiddenStrings:      counts.ForbiddenStrings,
		Boundaries:            counts.Boundaries,
		Warnings:              append([]string{}, readiness.Warnings...),
	}
	if strings.TrimSpace(caseRoot) == "" {
		return shim
	}
	installed := caseshim.InspectInstalled(repoRoot, caseRoot)
	shim.InstalledShimPath = installed.ShimPath
	shim.InstalledShimMatches = &installed.MatchesTemplate
	if !installed.Ready {
		shim.Ready = false
		shim.Warnings = append(shim.Warnings, installed.Warnings...)
	}
	if len(shim.Warnings) > 0 {
		shim.Summary = "case shim readiness has warnings"
	}
	return shim
}

func boolPtrValue(value *bool) bool {
	return value != nil && *value
}

type doctorInventory struct {
	Command       string       `json:"command"`
	SchemaVersion int          `json:"schemaVersion"`
	IsMutation    bool         `json:"isMutation"`
	Pack          string       `json:"pack"`
	Target        string       `json:"target"`
	Mode          string       `json:"mode"`
	Valid         bool         `json:"valid"`
	Summary       string       `json:"summary"`
	Rows          []doctor.Row `json:"rows"`
}

func runDoctor(ctx runtime.Context, opt Options, out io.Writer) error {
	mode, target, err := doctorModeAndTarget(ctx)
	if err != nil {
		return err
	}
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	var rows []doctor.Row
	if mode == "case" {
		rows, err = doctor.Case(ctx.RepoRoot, target, ctx.Pack)
	} else {
		rows, err = doctor.Pack(ctx.RepoRoot, ctx.Pack)
	}
	if err != nil {
		return err
	}
	statusLine := "pack validation ok"
	if mode == "case" {
		statusLine = "instance validation ok"
	}
	switch format {
	case "table", "text", "tsv":
		printRows(out, rows)
		fmt.Fprintln(out, statusLine)
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(doctorInventory{Command: opt.Command, SchemaVersion: 1, IsMutation: false, Pack: ctx.Pack, Target: target, Mode: mode, Valid: true, Summary: statusLine, Rows: rows})
	default:
		return fmt.Errorf("unsupported %s format: %s", opt.Command, opt.Format)
	}
	return nil
}

func doctorModeAndTarget(ctx runtime.Context) (string, string, error) {
	if !ctx.TargetProvided {
		if instance.LooksLikeCase(ctx.Target) && !samePath(ctx.Target, ctx.RepoRoot) {
			return "case", ctx.Target, nil
		}
		return "pack", ctx.RepoRoot, nil
	}
	if samePath(ctx.Target, ctx.RepoRoot) {
		return "pack", ctx.Target, nil
	}
	if instance.LooksLikeCase(ctx.Target) {
		return "case", ctx.Target, nil
	}
	return "", "", fmt.Errorf("target is neither this kit root nor an attached rekit case: %s", ctx.Target)
}

func commandTarget(ctx runtime.Context, command, targetName string) (string, error) {
	if ctx.TargetProvided {
		return ctx.Target, nil
	}
	if instance.LooksLikeCase(ctx.Target) && !samePath(ctx.Target, ctx.RepoRoot) {
		return ctx.Target, nil
	}
	return "", fmt.Errorf("%s requires an explicit -Target %s or a case-local working directory", command, targetName)
}

func runAttach(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("attach requires an explicit -Target case directory")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("attach -WhatIf cannot be combined with -Apply")
	}
	attachOpt := attach.Options{ProjectName: opt.ProjectName}
	var result any
	var err error
	if opt.WhatIf {
		result, err = attach.Preview(ctx.RepoRoot, ctx.Target, ctx.Pack, attachOpt)
	} else if opt.Apply {
		result, err = attach.Apply(ctx.RepoRoot, ctx.Target, ctx.Pack, attachOpt)
	} else {
		return fmt.Errorf("attach write requires -Apply; use -WhatIf for preview")
	}
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

func runRepair(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("repair requires an explicit -Target attached case")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("repair -WhatIf cannot be combined with -Apply")
	}
	repairOpt := repair.Options{ProjectName: opt.ProjectName}
	var result any
	var err error
	if opt.Apply {
		result, err = repair.Apply(ctx.RepoRoot, ctx.Target, ctx.Pack, repairOpt)
	} else {
		result, err = repair.Preview(ctx.RepoRoot, ctx.Target, ctx.Pack, repairOpt)
	}
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

func runInitBootstrap(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("%s requires an explicit -Target case directory", opt.Command)
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("%s -WhatIf cannot be combined with -Apply", opt.Command)
	}
	if opt.CreateCandidates {
		return fmt.Errorf("%s does not support -CreateCandidates", opt.Command)
	}
	if wantsReviewArtifacts(opt) {
		return fmt.Errorf("%s does not support review artifact options; use -WhatIf for preview or -Apply for explicit write", opt.Command)
	}
	applyOpt := syncreview.ApplyOptions{ProjectName: opt.ProjectName, ForceLocalTemplates: opt.Force, CreateLocalFiles: true, Command: opt.Command}
	var result any
	var err error
	if opt.WhatIf {
		result, err = syncreview.InitPreview(ctx.RepoRoot, ctx.Target, ctx.Pack, applyOpt)
	} else if opt.Apply {
		result, err = syncreview.Apply(ctx.RepoRoot, ctx.Target, ctx.Pack, applyOpt)
	} else {
		return fmt.Errorf("%s write requires -Apply; use -WhatIf for preview", opt.Command)
	}
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

func runSyncReview(ctx runtime.Context, opt Options, out io.Writer) error {
	command := strings.TrimSpace(opt.Command)
	if command == "" {
		command = commands.Sync
	}
	if opt.WhatIf && !opt.Apply {
		return fmt.Errorf("%s -WhatIf is only supported with -Apply for non-writing preview", command)
	}
	if opt.Force && !opt.Apply {
		return fmt.Errorf("%s -Force is only supported with -Apply", command)
	}
	target, err := commandTarget(ctx, command, "attached case")
	if err != nil {
		return err
	}
	if opt.Apply {
		if wantsReviewArtifacts(opt) {
			return fmt.Errorf("%s -Apply cannot be combined with review artifact options", command)
		}
		format, err := workstreamFormat(opt.Format)
		if err != nil {
			return fmt.Errorf("unsupported %s format: %s", command, opt.Format)
		}
		applyOpt := syncreview.ApplyOptions{ProjectName: opt.ProjectName, ForceLocalTemplates: opt.Force, Command: command}
		var result syncreview.ApplyResult
		if opt.WhatIf {
			result, err = syncreview.ApplyPreview(ctx.RepoRoot, target, ctx.Pack, applyOpt)
		} else {
			result, err = syncreview.Apply(ctx.RepoRoot, target, ctx.Pack, applyOpt)
		}
		if err != nil {
			return err
		}
		if format == "json" {
			return writeJSON(out, result)
		}
		return writeSyncApplyText(out, result)
	}
	plan, err := syncreview.Plan(ctx.RepoRoot, target, ctx.Pack)
	if err != nil {
		return err
	}
	if wantsReviewArtifacts(opt) {
		return writeReviewArtifacts(out, plan, opt)
	}
	return writeReviewPlan(out, plan)
}

func runOverview(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "overview", "attached case")
	if err != nil {
		return err
	}
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	switch format {
	case "table", "text", "tsv":
		text, err := overview.Render(ctx.RepoRoot, target, ctx.Pack)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, text)
		return err
	case "json":
		result, err := overview.BuildInventory(ctx.RepoRoot, target, ctx.Pack)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	default:
		return fmt.Errorf("unsupported overview format: %s", opt.Format)
	}
}

func runNote(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "note", "attached case")
	if err != nil {
		return err
	}
	if opt.Apply || opt.CreateCandidates {
		return fmt.Errorf("note does not support -Apply or -CreateCandidates; omit write mode flags or use -WhatIf for preview")
	}
	if opt.List {
		if opt.WhatIf {
			return fmt.Errorf("note -List cannot be combined with -WhatIf")
		}
		format := strings.ToLower(strings.TrimSpace(opt.Format))
		if format == "" {
			format = "table"
		}
		switch format {
		case "table", "text", "tsv":
			text, err := note.List(ctx.RepoRoot, target, ctx.Pack, opt.Note)
			if err != nil {
				return err
			}
			_, err = io.WriteString(out, text)
			return err
		case "json":
			result, err := note.ListEvents(ctx.RepoRoot, target, ctx.Pack, opt.Note)
			if err != nil {
				return err
			}
			b, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			_, err = out.Write(append(b, '\n'))
			return err
		default:
			return fmt.Errorf("unsupported note list format: %s", opt.Format)
		}
	}
	result, err := note.Append(ctx.RepoRoot, target, ctx.Pack, opt.Note, opt.WhatIf)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

func runStart(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "start", "attached case")
	if err != nil {
		return err
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("start -WhatIf cannot be combined with -Apply")
	}
	format, err := workstreamFormat(opt.Format)
	if err != nil {
		return fmt.Errorf("unsupported start format: %s", opt.Format)
	}
	startOpt := opt.Start
	startOpt.Force = opt.Force
	if !opt.WhatIf && !opt.Apply && format == "json" {
		return fmt.Errorf("start write requires -Apply; use -WhatIf for preview")
	}
	var result workstream.StartResult
	if opt.WhatIf {
		result, err = workstream.StartPreview(ctx.RepoRoot, target, ctx.Pack, startOpt)
	} else {
		result, err = workstream.StartApply(ctx.RepoRoot, target, ctx.Pack, startOpt)
	}
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, result)
	}
	return writeStartText(out, result)
}

func runHandoff(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "handoff", "attached case")
	if err != nil {
		return err
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("handoff -WhatIf cannot be combined with -Apply")
	}
	format, err := workstreamFormat(opt.Format)
	if err != nil {
		return fmt.Errorf("unsupported handoff format: %s", opt.Format)
	}
	if !opt.WhatIf && !opt.Apply && format == "json" {
		return fmt.Errorf("handoff write requires -Apply; use -WhatIf for preview")
	}
	var result workstream.HandoffResult
	if opt.WhatIf {
		result, err = workstream.HandoffPreview(ctx.RepoRoot, target, ctx.Pack, opt.Handoff)
	} else {
		result, err = workstream.HandoffApply(ctx.RepoRoot, target, ctx.Pack, opt.Handoff)
	}
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, result)
	}
	return writeHandoffText(out, result)
}

func runReconcile(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "reconcile", "attached case")
	if err != nil {
		return err
	}
	if opt.CreateCandidates || opt.Review || opt.Force {
		return fmt.Errorf("reconcile does not support -CreateCandidates, -Review, or -Force")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("reconcile cannot combine -WhatIf and -Apply")
	}
	if wantsReviewArtifacts(opt) {
		return fmt.Errorf("reconcile does not support review artifact options")
	}
	format, err := workstreamFormat(opt.Format)
	if err != nil {
		return fmt.Errorf("unsupported reconcile format: %s", opt.Format)
	}
	if !opt.WhatIf && !opt.Apply {
		return fmt.Errorf("reconcile write requires -Apply; use -WhatIf for preview")
	}
	var result workstream.ReconcileResult
	if opt.WhatIf {
		result, err = workstream.ReconcilePreview(ctx.RepoRoot, target, ctx.Pack, opt.Reconcile)
	} else {
		result, err = workstream.ReconcileApply(ctx.RepoRoot, target, ctx.Pack, opt.Reconcile)
	}
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, result)
	}
	return writeReconcileText(out, result)
}

func runContinue(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "continue", "attached case")
	if err != nil {
		return err
	}
	if opt.CreateCandidates {
		return fmt.Errorf("continue does not support -CreateCandidates")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("continue cannot combine -WhatIf and -Apply")
	}
	if wantsReviewArtifacts(opt) {
		return fmt.Errorf("continue does not support review artifact options")
	}
	format, err := workstreamFormat(opt.Format)
	if err != nil {
		return fmt.Errorf("unsupported continue format: %s", opt.Format)
	}
	if !opt.WhatIf && !opt.Apply && format == "json" {
		return fmt.Errorf("go backend continue requires -WhatIf or -Apply")
	}
	var result workstream.ContinueResult
	if opt.WhatIf {
		result, err = workstream.ContinuePreview(ctx.RepoRoot, target, ctx.Pack, opt.Continue)
	} else {
		result, err = workstream.ContinueApply(ctx.RepoRoot, target, ctx.Pack, opt.Continue)
	}
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, result)
	}
	return writeContinueText(out, result)
}

func workstreamFormat(format string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(format))
	if value == "" {
		return "json", nil
	}
	switch value {
	case "table", "text", "tsv", "json":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func promoteCandidatesFormat(format string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(format))
	if value == "" {
		return "json", nil
	}
	switch value {
	case "text", "json":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func planSubagentsFormat(format string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(format))
	if value == "" {
		return "json", nil
	}
	switch value {
	case "text", "json":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func writeJSON(out io.Writer, result any) error {
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

func writeStartText(out io.Writer, result workstream.StartResult) error {
	label := workstreamTextLabel(result.Lane)
	if !result.Applied {
		if _, err := fmt.Fprintf(out, "would create or enter feature workstream: %s\n", result.Lane.ID); err != nil {
			return err
		}
		return writeStartExecutorActionText(out, result)
	}
	if _, err := fmt.Fprintf(out, "功能支线已准备：%s\n", result.Lane.ID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "工作区：%s\n", result.Lane.Workspace); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "接续提示：%s/prompts/RESUME.md\n", result.Lane.LaneRoot); err != nil {
		return err
	}
	if result.ExecutorAction.Ready {
		if _, err := fmt.Fprintf(out, "继续此支线：/rekit continue %s\n", label); err != nil {
			return err
		}
	} else if err := writeExecutorNextActionsText(out, result.ExecutorAction); err != nil {
		return err
	}
	return writeStartExecutorActionText(out, result)
}

func writeExecutorNextActionsText(out io.Writer, action mission.ExecutorAction) error {
	if err := writeMissionCommanderActionText(out, "executor commander action", action); err != nil {
		return err
	}
	for _, next := range action.NextAgentActions {
		if _, err := fmt.Fprintf(out, "executor next action：%s\n", next); err != nil {
			return err
		}
	}
	return nil
}

func writeMissionCommanderActionText(out io.Writer, prefix string, action mission.ExecutorAction) error {
	commander := action.MissionCommanderAction
	if strings.TrimSpace(commander.State) == "" {
		return nil
	}
	if _, err := fmt.Fprintf(out, "%s：state=%s primary=`%s`\n", prefix, commander.State, commander.PrimaryCommand); err != nil {
		return err
	}
	if strings.TrimSpace(commander.Prompt) != "" {
		if _, err := fmt.Fprintf(out, "%s prompt：%s\n", prefix, commander.Prompt); err != nil {
			return err
		}
	}
	for _, command := range commander.FollowUpCommands {
		if _, err := fmt.Fprintf(out, "%s follow-up：%s\n", prefix, command); err != nil {
			return err
		}
	}
	for _, boundary := range commander.Boundary {
		if _, err := fmt.Fprintf(out, "%s boundary：%s\n", prefix, boundary); err != nil {
			return err
		}
	}
	return nil
}

func writeMissionCommanderNextActionsText(out io.Writer, items []mission.MissionCommanderNextActionItem) error {
	for _, item := range items {
		if _, err := fmt.Fprintf(out, "mission commander next action：state=%s source=%s blocked=%t requiresReview=%t command=`%s`\n", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command); err != nil {
			return err
		}
		for _, reason := range item.Reasons {
			if _, err := fmt.Fprintf(out, "mission commander next action reason：%s\n", reason); err != nil {
				return err
			}
		}
		for _, boundary := range item.Boundary {
			if _, err := fmt.Fprintf(out, "mission commander next action boundary：%s\n", boundary); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeMissionCommanderActionQueueText(out io.Writer, queue mission.MissionCommanderActionQueue) error {
	if _, err := fmt.Fprintf(out, "mission commander action queue：summary=%s\n", queue.Summary); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "mission commander action queue counts：total=%d unblocked=%d blocked=%d requiresReview=%d followUp=%d\n", queue.Counts.Total, queue.Counts.Unblocked, queue.Counts.Blocked, queue.Counts.RequiresReview, queue.Counts.FollowUp); err != nil {
		return err
	}
	if queue.CurrentAction == nil {
		_, err := fmt.Fprintln(out, "mission commander action queue current：none")
		return err
	}
	item := *queue.CurrentAction
	_, err := fmt.Fprintf(out, "mission commander action queue current：state=%s source=%s blocked=%t requiresReview=%t command=`%s`\n", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command)
	return err
}

func writeAuthorizedExecutionFollowThroughText(out io.Writer, prefix string, follow gate.AuthorizedExecutionFollowThrough) error {
	if strings.TrimSpace(follow.State) == "" && len(follow.Outcomes) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(out, "%s follow-through：state=%s gateEventId=%s reportPath=%s outcomes=%d\n", prefix, follow.State, follow.GateEventID, follow.ReportPath, len(follow.Outcomes)); err != nil {
		return err
	}
	for _, boundary := range follow.Boundary {
		if _, err := fmt.Fprintf(out, "%s follow-through boundary：%s\n", prefix, boundary); err != nil {
			return err
		}
	}
	for _, outcome := range follow.Outcomes {
		if _, err := fmt.Fprintf(out, "%s follow-through outcome：name=%s state=%s command=`%s` expected=%s\n", prefix, outcome.Name, outcome.State, outcome.Command, outcome.Expected); err != nil {
			return err
		}
		for _, action := range outcome.Actions {
			if _, err := fmt.Fprintf(out, "%s follow-through action：name=%s action=%s\n", prefix, outcome.Name, action); err != nil {
				return err
			}
		}
		for _, action := range outcome.RepairActions {
			if _, err := fmt.Fprintf(out, "%s follow-through repair action：name=%s action=%s\n", prefix, outcome.Name, action); err != nil {
				return err
			}
		}
		for _, command := range outcome.VerificationCommands {
			if _, err := fmt.Fprintf(out, "%s follow-through verification command：name=%s command=%s\n", prefix, outcome.Name, command); err != nil {
				return err
			}
		}
		for _, boundary := range outcome.Boundary {
			if _, err := fmt.Fprintf(out, "%s follow-through outcome boundary：name=%s boundary=%s\n", prefix, outcome.Name, boundary); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(follow.ActionQueue.Summary) != "" {
		if _, err := fmt.Fprintf(out, "%s follow-through queue：summary=%s\n", prefix, follow.ActionQueue.Summary); err != nil {
			return err
		}
	}
	return nil
}

func writeStartExecutorActionText(out io.Writer, result workstream.StartResult) error {
	action := result.ExecutorAction
	if _, err := fmt.Fprintf(out, "executor action：blocked=%t ready=%t pendingGates=%d openInterventions=%d openDecisions=%d\n", action.Blocked, action.Ready, action.PendingGates, action.OpenInterventions, action.OpenDecisions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "executor requirements：reconcile=%t pendingGate=%t openDecision=%t\n", action.ReconcileRequired, action.PendingGateRequired, action.OpenDecisionRequired); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "executor handoff：continue=`%s` handoff=`%s`\n", action.ResumeCommand, action.HandoffCommand); err != nil {
		return err
	}
	if err := writeMissionCommanderActionText(out, "executor commander action", action); err != nil {
		return err
	}
	if err := writeMissionCommanderActionQueueText(out, result.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions)
}

func writeHandoffText(out io.Writer, result workstream.HandoffResult) error {
	if !result.Applied {
		if result.Project {
			if _, err := fmt.Fprintln(out, "would write project handoff index: .rekit/handovers/latest.md"); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(out, "would write workstream handoff: %s\n", handoffTextSelector(result)); err != nil {
			return err
		}
	} else {
		path := handoffLatestPath(result)
		if result.Project {
			if _, err := fmt.Fprintf(out, "项目级接手索引：%s\n", path); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(out, "工作线接手文档：%s\n", path); err != nil {
			return err
		}
	}
	if err := writeMissionCommanderActionQueueText(out, result.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions)
}

func writeReconcileText(out io.Writer, result workstream.ReconcileResult) error {
	if !result.Applied {
		if _, err := fmt.Fprintf(out, "would reconcile intervention: %s on lane %s\n", result.Intervention.EventID, result.Lane.ID); err != nil {
			return err
		}
		return writeReconcileExecutorActionText(out, result)
	}
	if _, err := fmt.Fprintf(out, "已 reconcile intervention：%s\n", result.Intervention.EventID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "工作线：%s\n", result.Lane.ID); err != nil {
		return err
	}
	if result.Executor != "" {
		if _, err := fmt.Fprintf(out, "executor：%s generation=%d\n", result.Executor, result.ExecutorGeneration); err != nil {
			return err
		}
	}
	if result.ExecutorAction.Ready {
		if _, err := fmt.Fprintf(out, "继续此支线：/rekit continue %s\n", workstreamTextLabel(result.Lane)); err != nil {
			return err
		}
	} else if err := writeExecutorNextActionsText(out, result.ExecutorAction); err != nil {
		return err
	}
	return writeReconcileExecutorActionText(out, result)
}

func writeReconcileExecutorActionText(out io.Writer, result workstream.ReconcileResult) error {
	action := result.ExecutorAction
	if _, err := fmt.Fprintf(out, "executor action：blocked=%t ready=%t pendingGates=%d openInterventions=%d openDecisions=%d\n", action.Blocked, action.Ready, action.PendingGates, action.OpenInterventions, action.OpenDecisions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "executor requirements：reconcile=%t pendingGate=%t openDecision=%t\n", action.ReconcileRequired, action.PendingGateRequired, action.OpenDecisionRequired); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "executor handoff：continue=`%s` handoff=`%s`\n", action.ResumeCommand, action.HandoffCommand); err != nil {
		return err
	}
	if err := writeMissionCommanderActionText(out, "executor commander action", action); err != nil {
		return err
	}
	if err := writeMissionCommanderActionQueueText(out, result.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions)
}

func writeContinueText(out io.Writer, result workstream.ContinueResult) error {
	if result.ExecutorAction.Blocked {
		if _, err := fmt.Fprintf(out, "工作线被 blocker 阻塞：%s reasons=%s\n", result.Lane.ID, strings.Join(result.ExecutorAction.BlockerReasons, ",")); err != nil {
			return err
		}
		for _, item := range result.OpenInterventions {
			if _, err := fmt.Fprintf(out, "- %s | eventId=%s | status=%s\n", textFirst(item.Subject, item.Summary, item.EventID), item.EventID, item.Status); err != nil {
				return err
			}
		}
		if err := writeExecutorNextActionsText(out, result.ExecutorAction); err != nil {
			return err
		}
		if err := writeMissionCommanderActionQueueText(out, result.MissionCommanderActionQueue); err != nil {
			return err
		}
		return writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions)
	}
	if _, err := fmt.Fprintf(out, "已选择工作线：%s\n", result.Lane.ID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "工作区：%s\n", result.Lane.Workspace); err != nil {
		return err
	}
	resume := result.Lane.LaneRoot + "/prompts/RESUME.md"
	for _, write := range result.Writes {
		if write.Kind == "lane-resume" && strings.TrimSpace(write.Path) != "" {
			resume = write.Path
			break
		}
	}
	if _, err := fmt.Fprintf(out, "接续提示：%s\n", resume); err != nil {
		return err
	}
	if err := writeMissionCommanderActionQueueText(out, result.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions)
}

func workstreamTextLabel(lane workstream.Lane) string {
	if lane.Authority {
		return "main"
	}
	if name, ok := strings.CutPrefix(lane.ID, "feature-"); ok {
		return name
	}
	return lane.ID
}

func textFirst(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func handoffTextSelector(result workstream.HandoffResult) string {
	if strings.TrimSpace(result.Selector) != "" {
		return result.Selector
	}
	if result.Lane != nil {
		return workstreamTextLabel(*result.Lane)
	}
	return ""
}

func handoffLatestPath(result workstream.HandoffResult) string {
	wantAction := "write-latest-lane-handoff"
	fallback := ".rekit/handovers/"
	if result.Project {
		wantAction = "write-latest-project-handoff"
		fallback += "latest.md"
	} else if result.Lane != nil {
		fallback += result.Lane.ID + "-latest.md"
	} else {
		fallback += "latest.md"
	}
	for _, write := range result.Writes {
		if write.Action == wantAction && strings.TrimSpace(write.Path) != "" {
			return write.Path
		}
	}
	return fallback
}

var intakeReviewerResult = subagents.IntakeReviewerResult

func runPlanSubagents(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "plan-subagents", "directory")
	if err != nil {
		return err
	}
	if strings.TrimSpace(opt.ReviewerResultPath) != "" {
		if opt.CreateCandidates || opt.Review || opt.Force || strings.TrimSpace(opt.ReviewOutputDir) != "" || strings.TrimSpace(opt.DiffPath) != "" {
			return fmt.Errorf("plan-subagents reviewer intake does not support -CreateCandidates, -Review, -Force, -ReviewOutputDir, or -DiffPath")
		}
		if opt.Apply && opt.WhatIf {
			return fmt.Errorf("plan-subagents reviewer intake cannot combine -Apply and -WhatIf")
		}
		if !opt.Apply && !opt.WhatIf {
			return fmt.Errorf("plan-subagents reviewer intake requires -WhatIf or -Apply")
		}
		format, err := planSubagentsFormat(opt.Format)
		if err != nil {
			return fmt.Errorf("unsupported plan-subagents format: %s", opt.Format)
		}
		result, err := intakeReviewerResult(ctx.RepoRoot, target, ctx.Pack, subagents.ReviewerIntakeOptions{PacketPath: opt.PacketPath, ReviewerResultPath: opt.ReviewerResultPath, Lane: opt.Note.Lane, Actor: opt.Note.Actor, WhatIf: opt.WhatIf})
		if err != nil {
			if result.WritebackStatus != "" {
				var writeErr error
				if format == "json" {
					writeErr = writeJSON(out, result)
				} else {
					writeErr = writePlanSubagentsReviewerIntakeText(out, result)
				}
				if writeErr != nil {
					return fmt.Errorf("%v; write reviewer intake recovery result: %w", err, writeErr)
				}
			}
			return err
		}
		if format == "json" {
			return writeJSON(out, result)
		}
		return writePlanSubagentsReviewerIntakeText(out, result)
	}
	if opt.Apply || opt.WhatIf || opt.CreateCandidates {
		return fmt.Errorf("plan-subagents planning only writes review artifacts; use -ReviewerResultPath with -WhatIf or -Apply for main-agent reviewer intake")
	}
	format, err := planSubagentsFormat(opt.Format)
	if err != nil {
		return fmt.Errorf("unsupported plan-subagents format: %s", opt.Format)
	}
	result, err := subagents.WritePlan(ctx.RepoRoot, target, ctx.Pack, subagents.Options{Route: opt.Route, TaskType: opt.TaskType, Items: opt.Items, ItemsFile: opt.ItemsFile, ItemsPerAgent: opt.ItemsPerAgent, MaxParallel: opt.MaxParallel, ReviewOutputDir: opt.ReviewOutputDir, PacketPath: opt.PacketPath, DiffPath: opt.DiffPath, Lane: opt.Note.Lane})
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, result)
	}
	return writePlanSubagentsText(out, result)
}

func planSubagentsTextInline(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func planSubagentsTextValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func writePlanSubagentsReviewerOrchestrationText(out io.Writer, orchestration subagents.ReviewerOrchestrationPlan, targetLane string) error {
	if _, err := fmt.Fprintf(out, "plan-subagents reviewer orchestration：mode=%s targetLane=%s reviewers=%d maxParallel=%d resultRoot=%s\n", orchestration.Mode, targetLane, orchestration.ReviewerCount, orchestration.MaxParallel, orchestration.ResultRoot); err != nil {
		return err
	}
	if strings.TrimSpace(orchestration.Scope) != "" || strings.TrimSpace(orchestration.PacketPath) != "" {
		if _, err := fmt.Fprintf(out, "plan-subagents reviewer orchestration scope：scope=%s packet=%s resultRoot=%s\n", planSubagentsTextInline(orchestration.Scope), orchestration.PacketPath, orchestration.ResultRoot); err != nil {
			return err
		}
	}
	binding := orchestration.OwnerBinding
	if strings.TrimSpace(binding.TargetLane) != "" || strings.TrimSpace(binding.BindingMode) != "" {
		if _, err := fmt.Fprintf(out, "plan-subagents reviewer orchestration owner：targetLane=%s mode=%s currentExecutor=%s generation=%d requiredForIntake=%t spawnOwner=%s\n", binding.TargetLane, binding.BindingMode, planSubagentsTextValue(binding.CurrentExecutor, "unassigned"), binding.ExecutorGeneration, binding.RequiredForIntake, binding.MainAgentSpawnOwner); err != nil {
			return err
		}
	}
	for _, step := range orchestration.Lifecycle {
		if _, err := fmt.Fprintf(out, "plan-subagents reviewer orchestration lifecycle：step=%s owner=%s inputs=%s mustPass=%s nextOnSuccess=%s nextOnFailure=%s action=%s\n", step.Step, step.Owner, strings.Join(step.Inputs, ","), strings.Join(step.MustPass, ","), step.NextOnSuccess, step.NextOnFailure, planSubagentsTextInline(step.Action)); err != nil {
			return err
		}
	}
	for _, boundary := range orchestration.RuntimeBoundary {
		if _, err := fmt.Fprintf(out, "plan-subagents reviewer orchestration boundary：boundary=%s\n", boundary); err != nil {
			return err
		}
	}
	for _, criteria := range orchestration.CompletionCriteria {
		if _, err := fmt.Fprintf(out, "plan-subagents reviewer orchestration completion：criteria=%s\n", criteria); err != nil {
			return err
		}
	}
	for _, dispatch := range orchestration.Dispatches {
		if _, err := fmt.Fprintf(out, "plan-subagents reviewer dispatch：shard=%s status=%s reviewerResultPath=%s preview=`%s` apply=`%s`\n", dispatch.ShardID, dispatch.Status, dispatch.ReviewerResultPath, dispatch.PreviewCommand, dispatch.ApplyCommand); err != nil {
			return err
		}
	}
	return nil
}

func writePlanSubagentsShardHandoffText(out io.Writer, handoffs []subagents.ShardHandoff) error {
	for _, handoff := range handoffs {
		if _, err := fmt.Fprintf(out, "plan-subagents shard handoff：shard=%s status=%s reviewerResultPath=%s items=%s expected=%s\n", handoff.ShardID, handoff.Status, handoff.ReviewerResultPath, strings.Join(handoff.Items, ","), planSubagentsTextInline(handoff.ExpectedOutput)); err != nil {
			return err
		}
		binding := handoff.OwnerBinding
		if _, err := fmt.Fprintf(out, "plan-subagents shard owner binding：shard=%s targetLane=%s mode=%s currentExecutor=%s generation=%d requiredForIntake=%t spawnOwner=%s\n", handoff.ShardID, binding.TargetLane, binding.BindingMode, planSubagentsTextValue(binding.CurrentExecutor, "unassigned"), binding.ExecutorGeneration, binding.RequiredForIntake, binding.MainAgentSpawnOwner); err != nil {
			return err
		}
		if strings.TrimSpace(binding.LastTakeoverAt) != "" || strings.TrimSpace(binding.LastTakeoverBy) != "" || strings.TrimSpace(binding.LastTakeoverReason) != "" {
			if _, err := fmt.Fprintf(out, "plan-subagents shard owner takeover：shard=%s at=%s by=%s reason=%s\n", handoff.ShardID, binding.LastTakeoverAt, binding.LastTakeoverBy, binding.LastTakeoverReason); err != nil {
				return err
			}
		}
		if strings.TrimSpace(binding.RuntimeSessionBoundary) != "" {
			if _, err := fmt.Fprintf(out, "plan-subagents shard owner boundary：shard=%s boundary=%s\n", handoff.ShardID, binding.RuntimeSessionBoundary); err != nil {
				return err
			}
		}
		if strings.TrimSpace(handoff.ReviewerWriteback) != "" {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer writeback：shard=%s handoff=%s\n", handoff.ShardID, planSubagentsTextInline(handoff.ReviewerWriteback)); err != nil {
				return err
			}
		}
		if strings.TrimSpace(handoff.MainAgentNextAction) != "" {
			if _, err := fmt.Fprintf(out, "plan-subagents shard next action：shard=%s action=%s\n", handoff.ShardID, planSubagentsTextInline(handoff.MainAgentNextAction)); err != nil {
				return err
			}
		}
		if strings.TrimSpace(handoff.DispatchPrompt) != "" {
			if _, err := fmt.Fprintf(out, "plan-subagents shard dispatch prompt：shard=%s prompt=%s\n", handoff.ShardID, planSubagentsTextInline(handoff.DispatchPrompt)); err != nil {
				return err
			}
		}
		for _, boundary := range handoff.ReadOnlyBoundary {
			if _, err := fmt.Fprintf(out, "plan-subagents shard boundary：shard=%s boundary=%s\n", handoff.ShardID, boundary); err != nil {
				return err
			}
		}
		contract := handoff.ReviewerResultContract
		if _, err := fmt.Fprintf(out, "plan-subagents reviewer result contract：shard=%s format=%s required=%s decisions=%s\n", handoff.ShardID, contract.OutputFormat, strings.Join(contract.RequiredFields, ","), strings.Join(contract.AllowedDecisions, ",")); err != nil {
			return err
		}
		for _, rule := range contract.EvidenceRules {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer result evidence rule：shard=%s rule=%s\n", handoff.ShardID, rule); err != nil {
				return err
			}
		}
		for _, signal := range contract.ConflictSignals {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer result conflict signal：shard=%s signal=%s\n", handoff.ShardID, signal); err != nil {
				return err
			}
		}
		commands := handoff.ReviewerIntakeCommands
		if _, err := fmt.Fprintf(out, "plan-subagents reviewer intake command：shard=%s purpose=%s preview=`%s` apply=`%s` required=%s\n", handoff.ShardID, commands.Purpose, commands.PreviewCommand, commands.ApplyCommand, strings.Join(commands.RequiredFields, ",")); err != nil {
			return err
		}
		for _, check := range commands.PreviewChecks {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer intake preview check：shard=%s check=%s\n", handoff.ShardID, check); err != nil {
				return err
			}
		}
		for _, blocked := range commands.BlockedOutputs {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer intake blocked output：shard=%s blocked=%s\n", handoff.ShardID, blocked); err != nil {
				return err
			}
		}
		for _, item := range handoff.IntakeChecklist {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer intake checklist：shard=%s item=%s\n", handoff.ShardID, item); err != nil {
				return err
			}
		}
		for _, mapping := range handoff.ReviewerDecisionMappings {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer decision map：shard=%s reviewer=%s verification=%s main=%s applyWhen=%s fallback=%s\n", handoff.ShardID, mapping.ReviewerDecision, mapping.VerificationVerdict, mapping.MainDecision, strings.Join(mapping.ApplyWhen, ","), mapping.Fallback); err != nil {
				return err
			}
		}
		for _, handling := range handoff.ConflictHandling {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer conflict handling：shard=%s handling=%s\n", handoff.ShardID, handling); err != nil {
				return err
			}
		}
		for _, step := range handoff.WritebackSequence {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer writeback step：shard=%s step=%s owner=%s uses=%s mustPass=%s blockedBy=%s nextOnSuccess=%s nextOnFailure=%s\n", handoff.ShardID, step.Step, step.Owner, strings.Join(step.Uses, ","), strings.Join(step.MustPass, ","), strings.Join(step.BlockedBy, ","), step.NextOnSuccess, step.NextOnFailure); err != nil {
				return err
			}
			for _, binding := range step.CommandBindings {
				if _, err := fmt.Fprintf(out, "plan-subagents reviewer writeback command binding：shard=%s step=%s binding=%s source=%s kind=%s command=`%s` required=%s expected=%s\n", handoff.ShardID, step.Step, binding.Binding, binding.Source, binding.Kind, binding.Command, strings.Join(binding.RequiredFields, ","), binding.ExpectedOutput); err != nil {
					return err
				}
			}
		}
		for _, item := range handoff.PostReviewMerge {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer post-review：shard=%s item=%s\n", handoff.ShardID, item); err != nil {
				return err
			}
		}
		for _, item := range handoff.CompletionCriteria {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer completion：shard=%s item=%s\n", handoff.ShardID, item); err != nil {
				return err
			}
		}
		if strings.TrimSpace(handoff.FailureHandling) != "" {
			if _, err := fmt.Fprintf(out, "plan-subagents reviewer failure handling：shard=%s handling=%s\n", handoff.ShardID, handoff.FailureHandling); err != nil {
				return err
			}
		}
	}
	return nil
}

func writePlanSubagentsText(out io.Writer, result subagents.Result) error {
	if _, err := fmt.Fprintf(out, "plan-subagents：writesReviewArtifacts=%t reviewRequired=%t items=%d shards=%d packet=%s summary=%s\n", result.WritesReviewArtifacts, result.ReviewRequired, result.ItemCount, result.ShardCount, result.PacketPath, result.SummaryPath); err != nil {
		return err
	}
	if err := writePlanSubagentsReviewerOrchestrationText(out, result.ReviewerOrchestration, result.TargetLane); err != nil {
		return err
	}
	if err := writePlanSubagentsShardHandoffText(out, result.ShardHandoffs); err != nil {
		return err
	}
	if err := writeMissionCommanderActionText(out, "plan-subagents commander action", mission.ExecutorAction{MissionCommanderAction: result.MissionCommanderAction}); err != nil {
		return err
	}
	if err := writeMissionCommanderActionQueueText(out, result.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions)
}

func writePlanSubagentsReviewerResultText(out io.Writer, result subagents.ReviewerResult) error {
	if strings.TrimSpace(result.ShardID) == "" && strings.TrimSpace(result.ReviewerSession) == "" {
		return nil
	}
	if _, err := fmt.Fprintf(out, "reviewer intake result：packetId=%s routeId=%s shard=%s decision=%s confidence=%s reviewerSession=%s recommendedVerdict=%s\n", result.PacketID, result.RouteID, result.ShardID, result.Decision, result.Confidence, result.ReviewerSession, result.RecommendedVerdict); err != nil {
		return err
	}
	if strings.TrimSpace(result.Summary) != "" {
		if _, err := fmt.Fprintf(out, "reviewer intake result summary：%s\n", planSubagentsTextInline(result.Summary)); err != nil {
			return err
		}
	}
	if len(result.Items) > 0 {
		if _, err := fmt.Fprintf(out, "reviewer intake result items：%s\n", strings.Join(result.Items, ",")); err != nil {
			return err
		}
	}
	if len(result.EvidenceRefs) > 0 {
		if _, err := fmt.Fprintf(out, "reviewer intake result evidenceRefs：%s\n", strings.Join(result.EvidenceRefs, ",")); err != nil {
			return err
		}
	}
	if len(result.Risks) > 0 {
		if _, err := fmt.Fprintf(out, "reviewer intake result risks：%s\n", strings.Join(result.Risks, ",")); err != nil {
			return err
		}
	}
	if len(result.Conflicts) > 0 {
		if _, err := fmt.Fprintf(out, "reviewer intake result conflicts：%s\n", strings.Join(result.Conflicts, ",")); err != nil {
			return err
		}
	}
	if len(result.RouteOutput) > 0 {
		keys := make([]string, 0, len(result.RouteOutput))
		for key := range result.RouteOutput {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if _, err := fmt.Fprintf(out, "reviewer intake route output：%s=%s\n", key, planSubagentsTextInline(fmt.Sprint(result.RouteOutput[key]))); err != nil {
				return err
			}
		}
	}
	return nil
}

func reviewerIntakeEventTextValue(event map[string]any, key string) string {
	if event == nil {
		return ""
	}
	value, ok := event[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return strings.Join(typed, ",")
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				values = append(values, text)
			}
		}
		return strings.Join(values, ",")
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func writePlanSubagentsReviewerIntakeEventText(out io.Writer, label string, result note.AppendResult) error {
	if len(result.Event) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(out, "reviewer intake %s event：kind=%s eventId=%s applied=%t subject=%s target=%s lane=%s confidence=%s evidenceRefs=%s\n", label, reviewerIntakeEventTextValue(result.Event, "kind"), result.EventID, result.Applied, reviewerIntakeEventTextValue(result.Event, "subject"), reviewerIntakeEventTextValue(result.Event, "target"), reviewerIntakeEventTextValue(result.Event, "lane"), reviewerIntakeEventTextValue(result.Event, "confidence"), reviewerIntakeEventTextValue(result.Event, "evidenceRefs")); err != nil {
		return err
	}
	if summary := reviewerIntakeEventTextValue(result.Event, "summary"); strings.TrimSpace(summary) != "" {
		if _, err := fmt.Fprintf(out, "reviewer intake %s event summary：%s\n", label, planSubagentsTextInline(summary)); err != nil {
			return err
		}
	}
	for _, key := range []string{"verdict", "decision", "reason", "packetId", "routeId", "shardId", "reviewerSession", "ownerExecutor", "ownerGeneration", "ownerBindingMode", "ownerBindingTarget"} {
		if value := reviewerIntakeEventTextValue(result.Event, key); strings.TrimSpace(value) != "" {
			if _, err := fmt.Fprintf(out, "reviewer intake %s event field：%s=%s\n", label, key, planSubagentsTextInline(value)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writePlanSubagentsReviewerIntakeCheckpointText(out io.Writer, result subagents.ReviewerIntakeResult) error {
	if result.Verification == nil && result.Decision == nil {
		return nil
	}
	verificationApplied := false
	verificationEventID := ""
	if result.Verification != nil {
		verificationApplied = result.Verification.Applied
		verificationEventID = result.Verification.EventID
	}
	decisionApplied := false
	decisionEventID := ""
	if result.Decision != nil {
		decisionApplied = result.Decision.Applied
		decisionEventID = result.Decision.EventID
	}
	_, err := fmt.Fprintf(out, "reviewer intake writeback checkpoint：status=%s verificationApplied=%t verificationEventId=%s decisionApplied=%t decisionEventId=%s\n", result.WritebackStatus, verificationApplied, verificationEventID, decisionApplied, decisionEventID)
	return err
}

func writePlanSubagentsReviewerIntakeText(out io.Writer, result subagents.ReviewerIntakeResult) error {
	if _, err := fmt.Fprintf(out, "plan-subagents reviewer intake：status=%s mutation=%t applied=%t readyForWriteback=%t lane=%s shard=%s intakeId=%s\n", result.WritebackStatus, result.IsMutation, result.Applied, result.ReadyForWriteback, result.Lane, result.ShardID, result.IntakeID); err != nil {
		return err
	}
	if err := writePlanSubagentsReviewerResultText(out, result.ReviewerResult); err != nil {
		return err
	}
	if err := writePlanSubagentsReviewerIntakeCheckpointText(out, result); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "reviewer intake orchestration：mode=%s dispatch=%d/%d shardBefore=%s shardAfter=%s\n", result.OrchestrationSnapshot.Mode, result.OrchestrationSnapshot.DispatchIndex, result.OrchestrationSnapshot.DispatchTotal, result.OrchestrationSnapshot.ShardStatusBefore, result.OrchestrationSnapshot.ShardStatusAfter); err != nil {
		return err
	}
	for _, reason := range result.BlockedReasons {
		if _, err := fmt.Fprintf(out, "reviewer intake blocked reason：%s\n", reason); err != nil {
			return err
		}
	}
	if result.Verification != nil {
		if _, err := fmt.Fprintf(out, "reviewer intake verification：applied=%t eventId=%s reason=%s\n", result.Verification.Applied, result.Verification.EventID, result.Verification.Reason); err != nil {
			return err
		}
		if err := writePlanSubagentsReviewerIntakeEventText(out, "verification", *result.Verification); err != nil {
			return err
		}
	}
	if result.Decision != nil {
		if _, err := fmt.Fprintf(out, "reviewer intake decision：applied=%t eventId=%s reason=%s\n", result.Decision.Applied, result.Decision.EventID, result.Decision.Reason); err != nil {
			return err
		}
		if err := writePlanSubagentsReviewerIntakeEventText(out, "decision", *result.Decision); err != nil {
			return err
		}
	}
	if result.PostValidation != nil {
		if err := writeReviewerIntakePostValidationText(out, *result.PostValidation); err != nil {
			return err
		}
	}
	if err := writeMissionCommanderActionText(out, "reviewer intake commander action", mission.ExecutorAction{MissionCommanderAction: result.MissionCommanderAction}); err != nil {
		return err
	}
	if err := writeMissionCommanderActionQueueText(out, result.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions)
}

func writeReviewerIntakePostValidationText(out io.Writer, validation subagents.ReviewerPostValidation) error {
	if _, err := fmt.Fprintf(out, "reviewer intake post-validation：valid=%t overviewVerifications=%d overviewDecisions=%d doctorRows=%d\n", validation.Valid, validation.Overview.Sections.Verifications.Total, validation.Overview.Sections.Decisions.Total, len(validation.DoctorRows)); err != nil {
		return err
	}
	lane := ""
	if validation.Handoff.Lane != nil {
		lane = validation.Handoff.Lane.ID
	}
	if lane != "" || validation.Handoff.ExecutorAction != nil || strings.TrimSpace(validation.Handoff.MissionCommanderActionQueue.Summary) != "" {
		if _, err := fmt.Fprintf(out, "reviewer intake post-validation handoff：lane=%s project=%t executorAction=%t\n", lane, validation.Handoff.Project, validation.Handoff.ExecutorAction != nil); err != nil {
			return err
		}
	}
	queue := validation.Handoff.MissionCommanderActionQueue
	if strings.TrimSpace(queue.Summary) != "" {
		if _, err := fmt.Fprintf(out, "reviewer intake post-validation handoff queue：summary=%s\n", queue.Summary); err != nil {
			return err
		}
		if queue.CurrentAction != nil {
			item := *queue.CurrentAction
			if _, err := fmt.Fprintf(out, "reviewer intake post-validation handoff queue current：state=%s source=%s blocked=%t requiresReview=%t command=`%s`\n", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command); err != nil {
				return err
			}
		}
	}
	for _, item := range validation.Handoff.MissionCommanderNextActions {
		if _, err := fmt.Fprintf(out, "reviewer intake post-validation next action：state=%s source=%s blocked=%t requiresReview=%t command=`%s`\n", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command); err != nil {
			return err
		}
		for _, reason := range item.Reasons {
			if _, err := fmt.Fprintf(out, "reviewer intake post-validation next action reason：%s\n", reason); err != nil {
				return err
			}
		}
		for _, boundary := range item.Boundary {
			if _, err := fmt.Fprintf(out, "reviewer intake post-validation next action boundary：%s\n", boundary); err != nil {
				return err
			}
		}
	}
	return nil
}

func runPromoteReview(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "promote", "attached case")
	if err != nil {
		return err
	}
	if opt.Apply {
		if opt.CreateCandidates {
			return fmt.Errorf("promote -Apply cannot be combined with -CreateCandidates")
		}
		if wantsReviewArtifacts(opt) {
			return fmt.Errorf("promote -Apply cannot be combined with review artifact options")
		}
		result, err := promote.Apply(ctx.RepoRoot, target, ctx.Pack, promote.ApplyOptions{WhatIf: opt.WhatIf})
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		_, err = out.Write(append(b, '\n'))
		return err
	}
	if opt.WhatIf && !opt.CreateCandidates {
		return fmt.Errorf("promote -WhatIf is only supported with -CreateCandidates or -Apply")
	}
	if opt.CreateCandidates {
		if wantsReviewArtifacts(opt) {
			return fmt.Errorf("promote -CreateCandidates cannot be combined with review artifact options")
		}
		format, err := promoteCandidatesFormat(opt.Format)
		if err != nil {
			return fmt.Errorf("unsupported promote create-candidates format: %s", opt.Format)
		}
		result, err := promote.CreateCandidates(ctx.RepoRoot, target, ctx.Pack, promote.CandidateOptions{WhatIf: opt.WhatIf})
		if err != nil {
			return err
		}
		if format == "json" {
			return writeJSON(out, result)
		}
		return writePromoteCandidatesText(out, result)
	}
	plan, err := promote.Plan(ctx.RepoRoot, target, ctx.Pack)
	if err != nil {
		return err
	}
	if wantsReviewArtifacts(opt) {
		return writeReviewArtifacts(out, plan, opt)
	}
	return writeReviewPlan(out, plan)
}

func runGate(ctx runtime.Context, opt Options, out io.Writer) error {
	target, err := commandTarget(ctx, "gate", "attached case")
	if err != nil {
		return err
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("gate -WhatIf cannot be combined with -Apply")
	}
	format, err := workstreamFormat(opt.Format)
	if err != nil {
		return fmt.Errorf("unsupported gate format: %s", opt.Format)
	}
	if opt.Gate.ExecutionReportContract && opt.Gate.ValidateExecutionReport {
		return fmt.Errorf("gate -ExecutionReportContract cannot be combined with -ValidateExecutionReport")
	}
	if opt.Gate.ExecutionReportContract {
		if opt.Apply || opt.WhatIf {
			return fmt.Errorf("gate -ExecutionReportContract is read-only; omit -Apply and -WhatIf")
		}
		if wantsGateExecutionEvidenceDetails(opt.Gate) {
			return fmt.Errorf("gate -ExecutionReportContract cannot be combined with execution evidence fields")
		}
		contract, err := gate.AdapterReportContract(ctx.RepoRoot, target, ctx.Pack, opt.Gate)
		if err != nil {
			return err
		}
		if format == "json" {
			return writeJSON(out, contract)
		}
		return writeGateAdapterReportContractText(out, contract)
	}
	if opt.Gate.ValidateExecutionReport {
		if opt.Apply || opt.WhatIf {
			return fmt.Errorf("gate -ValidateExecutionReport is read-only; omit -Apply and -WhatIf")
		}
		if strings.TrimSpace(opt.Gate.ExecutionReportPath) == "" {
			return fmt.Errorf("gate -ValidateExecutionReport requires -ExecutionReportPath")
		}
		if wantsGateExecutionEvidenceDetailsExceptReportPath(opt.Gate) {
			return fmt.Errorf("gate -ValidateExecutionReport cannot be combined with execution evidence fields other than -ExecutionReportPath")
		}
		opt.Gate.ExecutionReportCwd = ctx.Cwd
		validation, err := gate.ValidateAdapterExecutionReport(ctx.RepoRoot, target, ctx.Pack, opt.Gate)
		if err != nil {
			return err
		}
		if format == "json" {
			return writeJSON(out, validation)
		}
		return writeGateAdapterReportValidationText(out, validation)
	}
	executionEvidence := wantsGateExecutionEvidence(opt.Gate)
	if opt.WhatIf {
		if executionEvidence {
			return fmt.Errorf("gate execution evidence does not support -WhatIf; use -Apply after the authorized action has completed")
		}
		plan, err := gate.PlanDryRun(ctx.RepoRoot, target, ctx.Pack, opt.Gate)
		if err != nil {
			return err
		}
		if format == "json" {
			return writeJSON(out, plan)
		}
		return writeGatePlanText(out, plan)
	}
	if !opt.Apply {
		return fmt.Errorf("gate write requires -Apply; use -WhatIf for dry-run preview")
	}
	var result gate.ApplyResult
	if executionEvidence {
		opt.Gate.ExecutionReportCwd = ctx.Cwd
		result, err = gate.RecordExecution(ctx.RepoRoot, target, ctx.Pack, opt.Gate)
	} else {
		result, err = gate.Apply(ctx.RepoRoot, target, ctx.Pack, opt.Gate)
	}
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, result)
	}
	return writeGateApplyText(out, result)
}

func wantsGateExecutionEvidence(opt gate.Options) bool {
	return strings.TrimSpace(opt.GateEventID) != "" || wantsGateExecutionEvidenceDetails(opt)
}

func wantsGateExecutionEvidenceDetails(opt gate.Options) bool {
	return wantsGateExecutionEvidenceDetailsExceptReportPath(opt) || strings.TrimSpace(opt.ExecutionReportPath) != ""
}

func wantsGateExecutionEvidenceDetailsExceptReportPath(opt gate.Options) bool {
	return strings.TrimSpace(opt.ExecutionStatus) != "" || opt.ActualRuntimeSeconds != 0 || opt.ActualDiskMB != 0 || opt.ActualRequests != 0 || strings.TrimSpace(opt.OutputRefs) != "" || strings.TrimSpace(opt.EvidenceRefs) != "" || strings.TrimSpace(opt.BoundaryHits) != "" || strings.TrimSpace(opt.Escalation) != ""
}

func writeGatePlanText(out io.Writer, plan gate.Plan) error {
	if _, err := fmt.Fprintf(out, "gate preview：action=%s lane=%s status=%s requiresConfirmation=%t\n", plan.EventPreview.Gate.Action, plan.EventPreview.Lane, plan.EventPreview.Status, plan.RequiresConfirmation); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "gate decision：authorization=%s profile=%s\n", plan.EventPreview.Gate.Authorization.Decision, plan.EventPreview.Gate.Authorization.ProfileID); err != nil {
		return err
	}
	if err := writeMissionExecutorActionText(out, "current executor action", plan.ExecutorAction); err != nil {
		return err
	}
	if err := writeMissionExecutorActionText(out, "would executor action", plan.WouldExecutorAction); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, plan.MissionCommanderNextActions)
}

func writeGateAdapterReportLiveValidationText(out io.Writer, live gate.AdapterReportLiveValidation) error {
	if _, err := fmt.Fprintf(out, "gate adapter report live validation：cwd=%s reportFileName=%s caseRelativeReportPath=%s replay=%s\n", live.InvocationCwd, live.ReportFileName, live.CaseRelativeReportPath, live.ReplayBehavior); err != nil {
		return err
	}
	if len(live.AuthorizedWorkspaces) > 0 {
		if _, err := fmt.Fprintf(out, "gate adapter report authorized workspaces：%s\n", strings.Join(live.AuthorizedWorkspaces, ",")); err != nil {
			return err
		}
	}
	template := live.SidecarTemplate
	if _, err := fmt.Fprintf(out, "gate adapter report sidecar template：kind=%s adapterId=%s action=%s status=%s gateEventId=%s\n", template.Kind, template.AdapterID, template.Action, template.Status, template.GateEventID); err != nil {
		return err
	}
	if len(template.OutputRefs) > 0 {
		if _, err := fmt.Fprintf(out, "gate adapter report sidecar outputRefs：%s\n", strings.Join(template.OutputRefs, ",")); err != nil {
			return err
		}
	}
	if len(template.EvidenceRefs) > 0 {
		if _, err := fmt.Fprintf(out, "gate adapter report sidecar evidenceRefs：%s\n", strings.Join(template.EvidenceRefs, ",")); err != nil {
			return err
		}
	}
	if len(template.BoundaryHits) > 0 {
		if _, err := fmt.Fprintf(out, "gate adapter report sidecar boundaryHits：%s\n", strings.Join(template.BoundaryHits, ",")); err != nil {
			return err
		}
	}
	if strings.TrimSpace(template.Escalation) != "" || strings.TrimSpace(template.Summary) != "" {
		if _, err := fmt.Fprintf(out, "gate adapter report sidecar summary：escalation=%s summary=%s\n", template.Escalation, template.Summary); err != nil {
			return err
		}
	}
	for _, candidate := range live.AdapterCandidates {
		if _, err := fmt.Fprintf(out, "gate adapter report adapter candidate：id=%s status=%s entry=%s gateActions=%s recordOnlyAfterGate=%t toolingCatalogPath=%s\n", candidate.ID, candidate.Status, candidate.Entry, strings.Join(candidate.GateActions, ","), candidate.RecordOnlyAfterGate, candidate.ToolingCatalogPath); err != nil {
			return err
		}
		if strings.TrimSpace(candidate.Purpose) != "" {
			if _, err := fmt.Fprintf(out, "gate adapter report adapter candidate purpose：id=%s purpose=%s\n", candidate.ID, candidate.Purpose); err != nil {
				return err
			}
		}
		if len(candidate.SideEffects) > 0 {
			if _, err := fmt.Fprintf(out, "gate adapter report adapter candidate side effects：id=%s sideEffects=%s\n", candidate.ID, strings.Join(candidate.SideEffects, ",")); err != nil {
				return err
			}
		}
		for _, guidance := range candidate.ReportGuidance {
			if _, err := fmt.Fprintf(out, "gate adapter report adapter candidate report guidance：id=%s guidance=%s\n", candidate.ID, guidance); err != nil {
				return err
			}
		}
		for _, guidance := range candidate.EvidenceGuidance {
			if _, err := fmt.Fprintf(out, "gate adapter report adapter candidate evidence guidance：id=%s guidance=%s\n", candidate.ID, guidance); err != nil {
				return err
			}
		}
		if len(candidate.StopConditionHints) > 0 {
			if _, err := fmt.Fprintf(out, "gate adapter report adapter candidate stop conditions：id=%s hints=%s\n", candidate.ID, strings.Join(candidate.StopConditionHints, ",")); err != nil {
				return err
			}
		}
	}
	for _, note := range live.Notes {
		if _, err := fmt.Fprintf(out, "gate adapter report live validation note：%s\n", note); err != nil {
			return err
		}
	}
	return nil
}

func writeGateAdapterReportContractText(out io.Writer, contract gate.AdapterExecutionReportContract) error {
	if _, err := fmt.Fprintf(out, "gate adapter report contract：gateEventId=%s action=%s lane=%s reportPath=%s mutation=%t\n", contract.GateEventID, contract.Action, contract.Lane, contract.DefaultReportPath, contract.IsMutation); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "gate adapter report contract validation：readOnly=true recordRequired=%t allowedStatuses=%s\n", contract.RecordRequired, strings.Join(contract.AllowedStatuses, ",")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "gate adapter report contract outputPaths：%s\n", strings.Join(contract.AllowedOutputPaths, ",")); err != nil {
		return err
	}
	if strings.TrimSpace(contract.LiveValidation.CaseRelativeValidateCommand) != "" {
		if _, err := fmt.Fprintf(out, "gate adapter report validate command：%s\n", contract.LiveValidation.CaseRelativeValidateCommand); err != nil {
			return err
		}
	} else if strings.TrimSpace(contract.LiveValidation.ValidateCommand) != "" {
		if _, err := fmt.Fprintf(out, "gate adapter report validate command：%s\n", contract.LiveValidation.ValidateCommand); err != nil {
			return err
		}
	}
	if strings.TrimSpace(contract.LiveValidation.CaseRelativeRecordCommand) != "" {
		if _, err := fmt.Fprintf(out, "gate adapter report record command：%s\n", contract.LiveValidation.CaseRelativeRecordCommand); err != nil {
			return err
		}
	} else if strings.TrimSpace(contract.LiveValidation.RecordCommand) != "" {
		if _, err := fmt.Fprintf(out, "gate adapter report record command：%s\n", contract.LiveValidation.RecordCommand); err != nil {
			return err
		}
	}
	if err := writeGateAdapterReportLiveValidationText(out, contract.LiveValidation); err != nil {
		return err
	}
	if err := writeAuthorizedExecutionFollowThroughText(out, "adapter report contract", contract.AuthorizedExecutionFollowThrough); err != nil {
		return err
	}
	if err := writeMissionCommanderActionText(out, "adapter report commander action", mission.ExecutorAction{MissionCommanderAction: contract.MissionCommanderAction}); err != nil {
		return err
	}
	if err := writeMissionCommanderActionQueueText(out, contract.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, contract.MissionCommanderNextActions)
}

func writeGateAdapterReportValidationText(out io.Writer, validation gate.AdapterExecutionReportValidation) error {
	if _, err := fmt.Fprintf(out, "gate adapter report validation：valid=%t gateEventId=%s reportPath=%s mutation=%t applied=%t\n", validation.Valid, validation.GateEventID, validation.ReportPath, validation.IsMutation, validation.Applied); err != nil {
		return err
	}
	if strings.TrimSpace(validation.FailureCode) != "" || strings.TrimSpace(validation.FailureStage) != "" {
		if _, err := fmt.Fprintf(out, "gate adapter report validation failure：code=%s stage=%s error=%s\n", validation.FailureCode, validation.FailureStage, validation.Error); err != nil {
			return err
		}
	} else if strings.TrimSpace(validation.Error) != "" {
		if _, err := fmt.Fprintf(out, "gate adapter report validation error：%s\n", validation.Error); err != nil {
			return err
		}
	}
	if validation.Report != nil {
		report := validation.Report
		if _, err := fmt.Fprintf(out, "gate adapter report sidecar：kind=%s adapterId=%s action=%s status=%s gateEventId=%s actualBudget=runtimeSeconds=%d,diskMB=%d,requests=%d\n", report.Kind, report.AdapterID, report.Action, report.Status, report.GateEventID, report.ActualBudget.RuntimeSeconds, report.ActualBudget.DiskMB, report.ActualBudget.Requests); err != nil {
			return err
		}
		if len(report.OutputRefs) > 0 {
			if _, err := fmt.Fprintf(out, "gate adapter report sidecar outputRefs：%s\n", strings.Join(report.OutputRefs, ",")); err != nil {
				return err
			}
		}
		if len(report.EvidenceRefs) > 0 {
			if _, err := fmt.Fprintf(out, "gate adapter report sidecar evidenceRefs：%s\n", strings.Join(report.EvidenceRefs, ",")); err != nil {
				return err
			}
		}
		if len(report.BoundaryHits) > 0 {
			if _, err := fmt.Fprintf(out, "gate adapter report sidecar boundaryHits：%s\n", strings.Join(report.BoundaryHits, ",")); err != nil {
				return err
			}
		}
		if strings.TrimSpace(report.Escalation) != "" || strings.TrimSpace(report.Summary) != "" {
			if _, err := fmt.Fprintf(out, "gate adapter report sidecar summary：escalation=%s summary=%s\n", report.Escalation, report.Summary); err != nil {
				return err
			}
		}
	}
	for _, hint := range validation.RepairHints {
		if _, err := fmt.Fprintf(out, "gate adapter report repair hint：action=%s recordBlocked=%t rerunValidation=%t code=%s stage=%s fields=%s allowedValues=%s allowedOutputPaths=%s allowedStopConditions=%s maxBytes=%d escalateToMain=%t detail=%s\n", hint.RepairAction, hint.RecordBlocked, hint.RerunValidation, hint.Code, hint.Stage, strings.Join(hint.Fields, ","), strings.Join(hint.AllowedValues, ","), strings.Join(hint.AllowedOutputPaths, ","), strings.Join(hint.AllowedStopConditions, ","), hint.MaxBytes, hint.EscalateToMain, hint.Detail); err != nil {
			return err
		}
	}
	if err := writeAuthorizedExecutionFollowThroughText(out, "adapter report validation", validation.AuthorizedExecutionFollowThrough); err != nil {
		return err
	}
	if err := writeMissionCommanderActionText(out, "adapter report validation commander action", mission.ExecutorAction{MissionCommanderAction: validation.MissionCommanderAction}); err != nil {
		return err
	}
	if err := writeMissionCommanderActionQueueText(out, validation.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, validation.MissionCommanderNextActions)
}

func writeGateExecutionEvidenceDetailText(out io.Writer, evidence gate.ExecutionEvidencePreview) error {
	if _, err := fmt.Fprintf(out, "gate execution evidence detail：subject=%s summary=%s target=%s recordRequired=%t reportPath=%s\n", evidence.Subject, evidence.Summary, evidence.Target, evidence.Execution.RecordRequired, evidence.Execution.ExecutionReportPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "gate execution evidence budget：runtimeSeconds=%d diskMB=%d requests=%d\n", evidence.Execution.ActualBudget.RuntimeSeconds, evidence.Execution.ActualBudget.DiskMB, evidence.Execution.ActualBudget.Requests); err != nil {
		return err
	}
	if len(evidence.Execution.OutputRefs) > 0 {
		if _, err := fmt.Fprintf(out, "gate execution evidence outputRefs：%s\n", strings.Join(evidence.Execution.OutputRefs, ",")); err != nil {
			return err
		}
	}
	if len(evidence.EvidenceRefs) > 0 {
		if _, err := fmt.Fprintf(out, "gate execution evidence evidenceRefs：%s\n", strings.Join(evidence.EvidenceRefs, ",")); err != nil {
			return err
		}
	}
	if len(evidence.Execution.BoundaryHits) > 0 {
		if _, err := fmt.Fprintf(out, "gate execution evidence boundaryHits：%s\n", strings.Join(evidence.Execution.BoundaryHits, ",")); err != nil {
			return err
		}
	}
	if strings.TrimSpace(evidence.Execution.Escalation) != "" {
		if _, err := fmt.Fprintf(out, "gate execution evidence escalation：%s\n", evidence.Execution.Escalation); err != nil {
			return err
		}
	}
	return nil
}

func writeGateApplyText(out io.Writer, result gate.ApplyResult) error {
	if result.ExecutionEvidence != nil {
		if _, err := fmt.Fprintf(out, "gate execution evidence：applied=%t status=%s eventId=%s path=%s gateEventId=%s\n", result.Applied, result.ExecutionEvidence.Execution.Status, result.EventID, result.Path, result.ExecutionEvidence.Execution.GateEventID); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "gate decision：authorization=%s action=%s\n", result.ExecutionEvidence.Execution.Authorization, result.ExecutionEvidence.Gate.Action); err != nil {
			return err
		}
		if err := writeGateExecutionEvidenceDetailText(out, *result.ExecutionEvidence); err != nil {
			return err
		}
		if err := writeAuthorizedExecutionFollowThroughText(out, "execution evidence", result.AuthorizedExecutionFollowThrough); err != nil {
			return err
		}
		if err := writeMissionCommanderActionText(out, "evidence commander action", mission.ExecutorAction{MissionCommanderAction: result.MissionCommanderAction}); err != nil {
			return err
		}
		if err := writeMissionCommanderActionQueueText(out, result.MissionCommanderActionQueue); err != nil {
			return err
		}
		if err := writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions); err != nil {
			return err
		}
		return writeMissionExecutorActionText(out, "executor action", result.ExecutorAction)
	}
	if result.Event == nil {
		return fmt.Errorf("gate apply result omitted event")
	}
	if _, err := fmt.Fprintf(out, "gate ledger：applied=%t status=%s eventId=%s path=%s\n", result.Applied, result.Event.Status, result.EventID, result.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "gate decision：authorization=%s profile=%s\n", result.Event.Gate.Authorization.Decision, result.Event.Gate.Authorization.ProfileID); err != nil {
		return err
	}
	if err := writeMissionExecutorActionText(out, "executor action", result.ExecutorAction); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, result.MissionCommanderNextActions)
}

func writeMissionExecutorActionText(out io.Writer, prefix string, action mission.ExecutorAction) error {
	if _, err := fmt.Fprintf(out, "%s：blocked=%t ready=%t pendingGates=%d openInterventions=%d openDecisions=%d\n", prefix, action.Blocked, action.Ready, action.PendingGates, action.OpenInterventions, action.OpenDecisions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s requirements：reconcile=%t pendingGate=%t openDecision=%t\n", prefix, action.ReconcileRequired, action.PendingGateRequired, action.OpenDecisionRequired); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s handoff：continue=`%s` handoff=`%s`\n", prefix, action.ResumeCommand, action.HandoffCommand); err != nil {
		return err
	}
	return writeMissionCommanderActionText(out, prefix+" commander action", action)
}

func writePromoteCandidateExecutionPlanText(out io.Writer, steps []promote.CandidateExecutionStep) error {
	for _, step := range steps {
		if _, err := fmt.Fprintf(out, "promote candidates execution step：name=%s when=%s expected=%s\n", step.Name, step.When, step.Expected); err != nil {
			return err
		}
		if len(step.AppliesTo) > 0 {
			if _, err := fmt.Fprintf(out, "promote candidates execution applies-to：name=%s paths=%s\n", step.Name, strings.Join(step.AppliesTo, ",")); err != nil {
				return err
			}
		}
		for _, action := range step.Actions {
			if _, err := fmt.Fprintf(out, "promote candidates execution action：name=%s action=%s\n", step.Name, action); err != nil {
				return err
			}
		}
		for _, command := range step.Commands {
			if _, err := fmt.Fprintf(out, "promote candidates execution command：name=%s command=%s\n", step.Name, command); err != nil {
				return err
			}
		}
		for _, evidence := range step.Evidence {
			if _, err := fmt.Fprintf(out, "promote candidates execution evidence：name=%s evidence=%s\n", step.Name, evidence); err != nil {
				return err
			}
		}
		for _, boundary := range step.Boundary {
			if _, err := fmt.Fprintf(out, "promote candidates execution boundary：name=%s boundary=%s\n", step.Name, boundary); err != nil {
				return err
			}
		}
	}
	return nil
}

func writePromoteCandidateReviewPlanText(out io.Writer, items []promote.CandidateReviewItem, checklist []promote.CandidateDecisionChecklist) error {
	for _, item := range items {
		if _, err := fmt.Fprintf(out, "promote candidates review item：path=%s kind=%s decision=%s action=%s candidatePath=%s packTarget=%s cleanupPath=%s\n", item.Path, item.Kind, item.ReviewDecision, item.Action, item.CandidatePath, item.PackTarget, item.CleanupPath); err != nil {
			return err
		}
		if strings.TrimSpace(item.MergeTargetHint) != "" {
			if _, err := fmt.Fprintf(out, "promote candidates review item merge hint：path=%s hint=%s\n", item.Path, item.MergeTargetHint); err != nil {
				return err
			}
		}
		if strings.TrimSpace(item.RejectTargetHint) != "" {
			if _, err := fmt.Fprintf(out, "promote candidates review item reject hint：path=%s hint=%s\n", item.Path, item.RejectTargetHint); err != nil {
				return err
			}
		}
		for _, action := range item.MainAgentActions {
			if _, err := fmt.Fprintf(out, "promote candidates review item action：path=%s action=%s\n", item.Path, action); err != nil {
				return err
			}
		}
	}
	for _, item := range checklist {
		if _, err := fmt.Fprintf(out, "promote candidates review checklist：path=%s decision=%s reviewAction=%s candidatePath=%s packTarget=%s\n", item.Path, item.ReviewDecision, item.ReviewAction, item.CandidatePath, item.PackTarget); err != nil {
			return err
		}
		for _, action := range item.AcceptActions {
			if _, err := fmt.Fprintf(out, "promote candidates checklist accept action：path=%s action=%s\n", item.Path, action); err != nil {
				return err
			}
		}
		for _, action := range item.RejectActions {
			if _, err := fmt.Fprintf(out, "promote candidates checklist reject action：path=%s action=%s\n", item.Path, action); err != nil {
				return err
			}
		}
		for _, action := range item.CleanupActions {
			if _, err := fmt.Fprintf(out, "promote candidates checklist cleanup action：path=%s action=%s\n", item.Path, action); err != nil {
				return err
			}
		}
		for _, command := range item.VerificationCommands {
			if _, err := fmt.Fprintf(out, "promote candidates checklist verification command：path=%s command=%s\n", item.Path, command); err != nil {
				return err
			}
		}
		for _, boundary := range item.Boundary {
			if _, err := fmt.Fprintf(out, "promote candidates checklist boundary：path=%s boundary=%s\n", item.Path, boundary); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeSyncApplyText(out io.Writer, result syncreview.ApplyResult) error {
	if _, err := fmt.Fprintf(out, "%s apply：mutation=%t applied=%t writes=%d backupRoot=%s\n", result.Command, result.IsMutation, result.Applied, len(result.Writes), result.BackupRoot); err != nil {
		return err
	}
	for _, write := range result.Writes {
		if _, err := fmt.Fprintf(out, "%s apply write：path=%s kind=%s action=%s source=%s target=%s backup=%s\n", result.Command, write.Path, write.Kind, write.Action, write.SourcePath, write.TargetPath, write.BackupPath); err != nil {
			return err
		}
	}
	for _, step := range result.NextSteps {
		if _, err := fmt.Fprintf(out, "%s apply next step：%s\n", result.Command, step); err != nil {
			return err
		}
	}
	return nil
}

func writePromoteCandidatesText(out io.Writer, result promote.CandidateResult) error {
	if _, err := fmt.Fprintf(out, "promote candidates：applied=%t created=%d blocked=%d cleanup=%t\n", result.Applied, result.Created, result.Blocked, result.RequiresCleanup); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "promote candidates review plan：mode=%s items=%d candidateRoot=%s toolingRoot=%s indexPath=%s\n", result.ReviewPlan.Mode, result.ReviewPlan.ItemCount, result.CandidateRoot, result.ToolingRoot, result.IndexPath); err != nil {
		return err
	}
	if err := writePromoteCandidateReviewPlanText(out, result.ReviewPlan.ReviewItems, result.ReviewPlan.DecisionChecklist); err != nil {
		return err
	}
	if err := writePromoteCandidateExecutionPlanText(out, result.ReviewPlan.MainAgentExecutionPlan); err != nil {
		return err
	}
	for _, item := range result.ReviewPlan.DecisionFollowThrough {
		if _, err := fmt.Fprintf(out, "promote candidates decision follow-through：path=%s decision=%s candidatePath=%s packTarget=%s\n", item.Path, item.ReviewDecision, item.CandidatePath, item.PackTarget); err != nil {
			return err
		}
		for _, outcome := range item.Outcomes {
			if _, err := fmt.Fprintf(out, "promote candidates decision outcome：path=%s decision=%s state=%s expected=%s\n", item.Path, outcome.Decision, outcome.State, outcome.Expected); err != nil {
				return err
			}
			for _, action := range outcome.Actions {
				if _, err := fmt.Fprintf(out, "promote candidates decision action：path=%s decision=%s action=%s\n", item.Path, outcome.Decision, action); err != nil {
					return err
				}
			}
			for _, action := range outcome.CleanupActions {
				if _, err := fmt.Fprintf(out, "promote candidates decision cleanup action：path=%s decision=%s action=%s\n", item.Path, outcome.Decision, action); err != nil {
					return err
				}
			}
			for _, command := range outcome.VerificationCommands {
				if _, err := fmt.Fprintf(out, "promote candidates decision verification command：path=%s decision=%s command=%s\n", item.Path, outcome.Decision, command); err != nil {
					return err
				}
			}
			for _, boundary := range outcome.Boundary {
				if _, err := fmt.Fprintf(out, "promote candidates decision boundary：path=%s decision=%s boundary=%s\n", item.Path, outcome.Decision, boundary); err != nil {
					return err
				}
			}
		}
	}
	for _, item := range result.ReviewPlan.CleanupTargets {
		if _, err := fmt.Fprintf(out, "promote candidates cleanup target：path=%s candidatePath=%s indexPath=%s when=%s\n", item.Path, item.CandidatePath, item.IndexPath, item.CleanupWhen); err != nil {
			return err
		}
		for _, action := range item.CleanupActions {
			if _, err := fmt.Fprintf(out, "promote candidates cleanup action：%s\n", action); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(out, "promote candidates reconsume：mode=%s\n", result.ReviewPlan.Reconsume.Mode); err != nil {
		return err
	}
	for _, check := range result.ReviewPlan.Reconsume.VerificationChecklist {
		if _, err := fmt.Fprintf(out, "promote candidates reconsume check：name=%s expected=%s\n", check.Name, check.Expected); err != nil {
			return err
		}
		for _, command := range check.Commands {
			if _, err := fmt.Fprintf(out, "promote candidates reconsume command：%s\n", command); err != nil {
				return err
			}
		}
		for _, boundary := range check.Boundary {
			if _, err := fmt.Fprintf(out, "promote candidates reconsume boundary：%s\n", boundary); err != nil {
				return err
			}
		}
	}
	if err := writeMissionCommanderActionText(out, "promote candidates commander action", mission.ExecutorAction{MissionCommanderAction: result.ReviewPlan.MissionCommanderAction}); err != nil {
		return err
	}
	if err := writeMissionCommanderActionQueueText(out, result.ReviewPlan.MissionCommanderActionQueue); err != nil {
		return err
	}
	return writeMissionCommanderNextActionsText(out, result.ReviewPlan.MissionCommanderNextActions)
}

func writeReviewPlan(out io.Writer, plan review.Plan) error {
	plan.IsMutation = false
	plan.Summary = review.Summary{Changed: plan.ChangedItems(), Blocked: plan.BlockedItems(), ReviewRequired: true}
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

func wantsReviewArtifacts(opt Options) bool {
	return opt.Review || strings.TrimSpace(opt.ReviewOutputDir) != "" || strings.TrimSpace(opt.PacketPath) != "" || strings.TrimSpace(opt.DiffPath) != ""
}

func writeReviewArtifacts(out io.Writer, plan review.Plan, opt Options) error {
	result, err := review.WriteArtifacts(plan, review.ArtifactOptions{ReviewOutputDir: opt.ReviewOutputDir, PacketPath: opt.PacketPath, DiffPath: opt.DiffPath})
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

func printRows(out io.Writer, rows []doctor.Row) {
	for _, row := range rows {
		fmt.Fprintf(out, "%s\t%d/%d\n", row.File, row.Bytes, row.Limit)
	}
}

func samePath(a, b string) bool {
	left := strings.TrimRight(filepath.Clean(a), string(filepath.Separator))
	right := strings.TrimRight(filepath.Clean(b), string(filepath.Separator))
	return strings.EqualFold(left, right)
}
