package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/attach"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
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
	Command          string
	Target           string
	Pack             string
	Review           bool
	Apply            bool
	CreateCandidates bool
	WhatIf           bool
	Force            bool
	List             bool
	ReviewOutputDir  string
	PacketPath       string
	DiffPath         string
	ProjectName      string
	Route            string
	TaskType         string
	Items            string
	ItemsFile        string
	ItemsPerAgent    int
	MaxParallel      int
	Format           string
	Gate             gate.Options
	Note             note.Options
	Start            workstream.StartOptions
	Handoff          workstream.HandoffOptions
	Continue         workstream.ContinueOptions
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
		case "-Actor", "--actor":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Actor")
			}
			opt.Gate.Actor = args[i]
			opt.Note.Actor = args[i]
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
				} else {
					opt.Start.Name += "-" + args[i]
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

func Run(args []string, stdout io.Writer) error {
	opt, err := Parse(args)
	if err != nil {
		return err
	}
	ctx, err := runtime.New(opt.Target, opt.Pack)
	if err != nil {
		return err
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
	if len(result.Warnings) == 0 {
		return fmt.Errorf("release-check not ready")
	}
	return fmt.Errorf("release-check not ready: %s", strings.Join(result.Warnings, "; "))
}

func emitReleaseCheckResult(out io.Writer, result releasecheck.Result, format string) error {
	switch format {
	case "table", "text", "tsv":
		fmt.Fprintf(out, "release-check: %s\n", result.Summary)
		fmt.Fprintf(out, "ready: %t\n", result.Ready)
		fmt.Fprintf(out, "gate profile: %s ready=%t steps=%d largeMatrixDefault=%t\n", result.GateProfile.Name, result.GateProfile.Ready, result.GateProfile.StepCount, result.GateProfile.LargeMatrixDefault)
		fmt.Fprintf(out, "CI release gate: %s ready=%t jobs=%d commands=%d forbidden=%d\n", result.CIReleaseGate.WorkflowPath, result.CIReleaseGate.Ready, len(result.CIReleaseGate.Jobs), len(result.CIReleaseGate.RequiredCommands), len(result.CIReleaseGate.ForbiddenStrings))
		if len(result.CIReleaseGate.Warnings) > 0 {
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
		fmt.Fprintf(out, "packs: %d\n", len(result.Packs))
		fmt.Fprintf(out, "heavy-tool gate actions: %s\n", strings.Join(result.HeavyToolGateActions, ","))
		fmt.Fprintf(out, "PowerShell deprecation: %s ready=%t commands=%d modules=%d freezeGates=%d blocked=%d fallbackRetirement=%t noFallback=%d candidates=%d removalModules=%d retiredModules=%d facadeRuntime=%t legacyImports=%t dispatcher=%t publicFacade=%t retained=%t facadeCommands=%d noFallback=%d moduleRemoval=%t removalCandidates=%d retired=%d facadeDeps=%d undocumented=%d moduleReferences=%t activeTests=%d fixtures=%d blockers=%d unclassified=%d\n", result.PowerShellDeprecation.Summary, result.PowerShellDeprecation.Ready, len(result.PowerShellDeprecation.CommandOwnership), len(result.PowerShellDeprecation.ModuleStatus), len(result.PowerShellDeprecation.FreezeGates), len(result.PowerShellDeprecation.BlockedMigrations), result.PowerShellDeprecation.FallbackRetirement.Ready, len(result.PowerShellDeprecation.FallbackRetirement.NoFallbackCommands), len(result.PowerShellDeprecation.FallbackRetirement.CandidateCommands), len(result.PowerShellDeprecation.FallbackRetirement.RemovalCandidateModules), len(result.PowerShellDeprecation.FallbackRetirement.RetiredModules), result.PowerShellDeprecation.FacadeRuntime.Ready, result.PowerShellDeprecation.FacadeRuntime.LegacyModuleImportsPresent, result.PowerShellDeprecation.FacadeRuntime.CommandDispatcherPresent, result.PowerShellDeprecation.PublicFacade.Ready, result.PowerShellDeprecation.PublicFacade.Retained, len(result.PowerShellDeprecation.PublicFacade.CommandSurface), len(result.PowerShellDeprecation.PublicFacade.NoFallbackCommands), result.PowerShellDeprecation.ModuleRemoval.Ready, len(result.PowerShellDeprecation.ModuleRemoval.CandidateModules), len(result.PowerShellDeprecation.ModuleRemoval.RetiredModules), len(result.PowerShellDeprecation.ModuleRemoval.FacadeRuntimeDependencies), len(result.PowerShellDeprecation.ModuleRemoval.UndocumentedModules), result.PowerShellDeprecation.ModuleReferences.Ready, len(result.PowerShellDeprecation.ModuleReferences.ActiveTestDependencies), len(result.PowerShellDeprecation.ModuleReferences.CompatibilityFixtures), len(result.PowerShellDeprecation.ModuleReferences.RemovalBlockers), len(result.PowerShellDeprecation.ModuleReferences.UnclassifiedReferences))
		fmt.Fprintf(out, "Go-native public surface: %s ready=%t entrypoint=%s present=%t catalog=%s catalogPresent=%t default=%s commands=%d handlers=%d symbols=%d profiles=%d boundaries=%d boundaryRows=%d policyRows=%d policyViolations=%d facadeRemovalReady=%t facadePrerequisites=%d readOnly=%d mutating=%d writesCase=%d writesKit=%d reviewFirst=%d applyRequired=%d heavyTool=%d authorityConfirmed=%d readOnlyCommands=%s reviewFirstCommands=%s writesKitCommands=%s caseLocalApplyCommands=%s kitReviewFirstCommands=%s alternative=%s unsupportedDiagnostic=%t\n", result.GoNativePublicSurface.Summary, result.GoNativePublicSurface.Ready, result.GoNativePublicSurface.Entrypoint, result.GoNativePublicSurface.EntrypointPresent, result.GoNativePublicSurface.CommandCatalogPath, result.GoNativePublicSurface.CommandCatalogPresent, result.GoNativePublicSurface.DefaultCommand, len(result.GoNativePublicSurface.Commands), len(result.GoNativePublicSurface.HandlerCommands), len(result.GoNativePublicSurface.SymbolCommands), len(result.GoNativePublicSurface.CommandProfiles), len(result.GoNativePublicSurface.MutationBoundaries), len(result.GoNativePublicSurface.CommandProfileBoundaries), len(result.GoNativePublicSurface.CommandProfilePolicies), commands.PublicProfilePolicyViolationCount(result.GoNativePublicSurface.CommandProfilePolicies), result.GoNativePublicSurface.FacadeRemovalReady, len(result.GoNativePublicSurface.FacadeRemovalPrerequisites), result.GoNativePublicSurface.CommandProfileSummary.ReadOnly, result.GoNativePublicSurface.CommandProfileSummary.Mutating, result.GoNativePublicSurface.CommandProfileSummary.WritesCase, result.GoNativePublicSurface.CommandProfileSummary.WritesKit, result.GoNativePublicSurface.CommandProfileSummary.ReviewFirst, result.GoNativePublicSurface.CommandProfileSummary.ApplyRequired, result.GoNativePublicSurface.CommandProfileSummary.HeavyTool, result.GoNativePublicSurface.CommandProfileSummary.AuthorityConfirmed, strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ReadOnly, ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ReviewFirst, ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.WritesKit, ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ByBoundary["case-local-apply"], ","), strings.Join(result.GoNativePublicSurface.CommandProfileGroups.ByBoundary["kit-review-first"], ","), result.GoNativePublicSurface.AlternativePattern, result.GoNativePublicSurface.UnsupportedCommandDiagnosticPresent)
		fmt.Fprintf(out, "public facade removal: %s ready=%t prerequisites=%d removalPlan=%t planChecks=%d removalImpact=%t impactReferences=%d impactCategories=%d workItems=%d unclassified=%d\n", result.PublicFacadeRemoval.Summary, result.PublicFacadeRemoval.Ready, len(result.PublicFacadeRemoval.Prerequisites), result.PublicFacadeRemoval.RemovalPlan.Ready, len(result.PublicFacadeRemoval.RemovalPlan.RequiredPhrases), result.PublicFacadeRemoval.RemovalImpact.Ready, len(result.PublicFacadeRemoval.RemovalImpact.References), len(result.PublicFacadeRemoval.RemovalImpact.ReferenceCategories), len(result.PublicFacadeRemoval.RemovalImpact.WorkItems), len(result.PublicFacadeRemoval.RemovalImpact.UnclassifiedReferences))
		fmt.Fprintf(out, "case shim: %s ready=%t required=%d canonical=%d forbidden=%d\n", result.CaseShim.Summary, result.CaseShim.Ready, len(result.CaseShim.RequiredPhrases), len(result.CaseShim.CanonicalSkillPhrases), len(result.CaseShim.ForbiddenStrings))
		fmt.Fprintf(out, "public default docs: %s ready=%t documents=%d required=%d forbiddenCommands=%d forbiddenShellFences=%d\n", result.PublicDefaultDocs.Summary, result.PublicDefaultDocs.Ready, len(result.PublicDefaultDocs.Documents), len(result.PublicDefaultDocs.RequiredPhrases), len(result.PublicDefaultDocs.ForbiddenCommands), len(result.PublicDefaultDocs.ForbiddenShellFences))
		fmt.Fprintf(out, "release handoff: %s ready=%t readFirst=%d signals=%d knownGaps=%d packMaturity=%d validation=%d releaseNotes=%t latest=%s\n", result.ReleaseHandoff.Summary, result.ReleaseHandoff.Ready, len(result.ReleaseHandoff.ReadFirst), len(result.ReleaseHandoff.Signals), len(result.ReleaseHandoff.KnownGaps), result.ReleaseHandoff.PackMaturity.Total, len(result.ReleaseHandoff.Validation), result.ReleaseHandoff.ReleaseNotes.Covered, result.ReleaseHandoff.LatestBatch.Title)
		if len(result.GoNativePublicSurface.Warnings) > 0 {
			fmt.Fprintln(out, "Go-native public surface warnings:")
			for _, warning := range result.GoNativePublicSurface.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if len(result.PublicFacadeRemoval.Warnings) > 0 {
			fmt.Fprintln(out, "public facade removal warnings:")
			for _, warning := range result.PublicFacadeRemoval.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if len(result.CaseShim.Warnings) > 0 {
			fmt.Fprintln(out, "case shim warnings:")
			for _, warning := range result.CaseShim.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if len(result.PublicDefaultDocs.Warnings) > 0 {
			fmt.Fprintln(out, "public default docs warnings:")
			for _, warning := range result.PublicDefaultDocs.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if len(result.ReleaseHandoff.Warnings) > 0 {
			fmt.Fprintln(out, "release handoff warnings:")
			for _, warning := range result.ReleaseHandoff.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if len(result.PowerShellDeprecation.Warnings) > 0 {
			fmt.Fprintln(out, "PowerShell deprecation warnings:")
			for _, warning := range result.PowerShellDeprecation.Warnings {
				fmt.Fprintf(out, "- %s\n", warning)
			}
		}
		if len(result.KnownGaps) > 0 {
			fmt.Fprintln(out, "known gaps:")
			for _, gap := range result.KnownGaps {
				fmt.Fprintf(out, "- %s\n", gap)
			}
		}
		if len(result.Warnings) > 0 {
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
}

type statusCase struct {
	CaseRoot       string `json:"caseRoot"`
	MetadataSource string `json:"metadataSource"`
	InstancePath   string `json:"instancePath"`
	TemplateRoot   string `json:"templateRoot"`
	TemplatePack   string `json:"templatePack"`
	ProjectName    string `json:"projectName"`
	ProjectRoot    string `json:"projectRoot"`
	Moved          bool   `json:"moved"`
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
	fmt.Fprintf(out, "rekit go backend: %s\n", ctx.RuntimeRoot)
	fmt.Fprintf(out, "template root: %s\n", ctx.RepoRoot)
	fmt.Fprintf(out, "pack: %s\n", ctx.Pack)
	if instance.LooksLikeCase(ctx.Target) {
		inst, err := instance.Read(ctx.Target)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "case: %s\n", inst.CaseRoot)
		fmt.Fprintf(out, "case metadata: %s %s\n", inst.Source, inst.InstancePath)
		fmt.Fprintf(out, "case templateRoot: %s\n", inst.TemplateRoot)
		fmt.Fprintf(out, "case templatePack: %s\n", inst.TemplatePack)
		if inst.Moved() {
			fmt.Fprintln(out, "detected moved case metadata")
		}
		return nil
	}
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
	}
	if instance.LooksLikeCase(ctx.Target) {
		inst, err := instance.Read(ctx.Target)
		if err != nil {
			return statusInventory{}, err
		}
		status.Mode = "case"
		status.Case = &statusCase{
			CaseRoot:       inst.CaseRoot,
			MetadataSource: inst.Source,
			InstancePath:   inst.InstancePath,
			TemplateRoot:   inst.TemplateRoot,
			TemplatePack:   inst.TemplatePack,
			ProjectName:    inst.ProjectName,
			ProjectRoot:    inst.ProjectRoot,
			Moved:          inst.Moved(),
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
		if instance.LooksLikeCase(ctx.Cwd) && !samePath(ctx.Cwd, ctx.RepoRoot) {
			return "case", ctx.Cwd, nil
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
	if opt.WhatIf && !opt.Apply {
		return fmt.Errorf("sync -WhatIf is only supported with -Apply for non-writing preview")
	}
	if opt.Force && !opt.Apply {
		return fmt.Errorf("sync -Force is only supported with -Apply")
	}
	if !ctx.TargetProvided {
		return fmt.Errorf("sync requires an explicit -Target attached case")
	}
	if opt.Apply {
		if wantsReviewArtifacts(opt) {
			return fmt.Errorf("sync -Apply cannot be combined with review artifact options")
		}
		applyOpt := syncreview.ApplyOptions{ProjectName: opt.ProjectName, ForceLocalTemplates: opt.Force}
		var result syncreview.ApplyResult
		var err error
		if opt.WhatIf {
			result, err = syncreview.ApplyPreview(ctx.RepoRoot, ctx.Target, ctx.Pack, applyOpt)
		} else {
			result, err = syncreview.Apply(ctx.RepoRoot, ctx.Target, ctx.Pack, applyOpt)
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
	plan, err := syncreview.Plan(ctx.RepoRoot, ctx.Target, ctx.Pack)
	if err != nil {
		return err
	}
	if wantsReviewArtifacts(opt) {
		return writeReviewArtifacts(out, plan, opt)
	}
	return writeReviewPlan(out, plan)
}

func runOverview(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("overview requires an explicit -Target attached case")
	}
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	switch format {
	case "table", "text", "tsv":
		text, err := overview.Render(ctx.RepoRoot, ctx.Target, ctx.Pack)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, text)
		return err
	case "json":
		result, err := overview.BuildInventory(ctx.RepoRoot, ctx.Target, ctx.Pack)
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
	if !ctx.TargetProvided {
		return fmt.Errorf("note requires an explicit -Target attached case")
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
			text, err := note.List(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Note)
			if err != nil {
				return err
			}
			_, err = io.WriteString(out, text)
			return err
		case "json":
			result, err := note.ListEvents(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Note)
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
	result, err := note.Append(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Note, opt.WhatIf)
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
	if !ctx.TargetProvided {
		return fmt.Errorf("start requires an explicit -Target attached case")
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
		result, err = workstream.StartPreview(ctx.RepoRoot, ctx.Target, ctx.Pack, startOpt)
	} else {
		result, err = workstream.StartApply(ctx.RepoRoot, ctx.Target, ctx.Pack, startOpt)
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
	if !ctx.TargetProvided {
		return fmt.Errorf("handoff requires an explicit -Target attached case")
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
		result, err = workstream.HandoffPreview(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Handoff)
	} else {
		result, err = workstream.HandoffApply(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Handoff)
	}
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, result)
	}
	return writeHandoffText(out, result)
}

func runContinue(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("continue requires an explicit -Target attached case")
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
		result, err = workstream.ContinuePreview(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Continue)
	} else {
		result, err = workstream.ContinueApply(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Continue)
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
		_, err := fmt.Fprintf(out, "would create or enter feature workstream: %s\n", result.Lane.ID)
		return err
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
	_, err := fmt.Fprintf(out, "继续此支线：/rekit continue %s\n", label)
	return err
}

func writeHandoffText(out io.Writer, result workstream.HandoffResult) error {
	if !result.Applied {
		if result.Project {
			_, err := fmt.Fprintln(out, "would write project handoff index: .rekit/handovers/latest.md")
			return err
		}
		_, err := fmt.Fprintf(out, "would write workstream handoff: %s\n", handoffTextSelector(result))
		return err
	}
	path := handoffLatestPath(result)
	if result.Project {
		_, err := fmt.Fprintf(out, "项目级接手索引：%s\n", path)
		return err
	}
	_, err := fmt.Fprintf(out, "工作线接手文档：%s\n", path)
	return err
}

func writeContinueText(out io.Writer, result workstream.ContinueResult) error {
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
	_, err := fmt.Fprintf(out, "接续提示：%s\n", resume)
	return err
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

func runPlanSubagents(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("plan-subagents requires an explicit -Target directory")
	}
	if opt.Apply || opt.WhatIf || opt.CreateCandidates {
		return fmt.Errorf("plan-subagents only writes review artifacts; do not combine it with -Apply, -WhatIf, or -CreateCandidates")
	}
	result, err := subagents.WritePlan(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.Options{Route: opt.Route, TaskType: opt.TaskType, Items: opt.Items, ItemsFile: opt.ItemsFile, ItemsPerAgent: opt.ItemsPerAgent, MaxParallel: opt.MaxParallel, ReviewOutputDir: opt.ReviewOutputDir, PacketPath: opt.PacketPath, DiffPath: opt.DiffPath})
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

func runPromoteReview(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("promote review requires an explicit -Target attached case")
	}
	if opt.Apply {
		if opt.CreateCandidates {
			return fmt.Errorf("promote -Apply cannot be combined with -CreateCandidates")
		}
		if wantsReviewArtifacts(opt) {
			return fmt.Errorf("promote -Apply cannot be combined with review artifact options")
		}
		result, err := promote.Apply(ctx.RepoRoot, ctx.Target, ctx.Pack, promote.ApplyOptions{WhatIf: opt.WhatIf})
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
		result, err := promote.CreateCandidates(ctx.RepoRoot, ctx.Target, ctx.Pack, promote.CandidateOptions{WhatIf: opt.WhatIf})
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
	plan, err := promote.Plan(ctx.RepoRoot, ctx.Target, ctx.Pack)
	if err != nil {
		return err
	}
	if wantsReviewArtifacts(opt) {
		return writeReviewArtifacts(out, plan, opt)
	}
	return writeReviewPlan(out, plan)
}

func runGate(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("gate requires an explicit -Target attached case")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("gate -WhatIf cannot be combined with -Apply")
	}
	if opt.WhatIf {
		plan, err := gate.PlanDryRun(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Gate)
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		_, err = out.Write(append(b, '\n'))
		return err
	}
	if !opt.Apply {
		return fmt.Errorf("gate write requires -Apply; use -WhatIf for dry-run preview")
	}
	result, err := gate.Apply(ctx.RepoRoot, ctx.Target, ctx.Pack, opt.Gate)
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
