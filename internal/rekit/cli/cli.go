package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/attach"
	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/overview"
	"github.com/shuiyu486/re-context-kits/internal/rekit/promote"
	"github.com/shuiyu486/re-context-kits/internal/rekit/repair"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
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
	ReviewOutputDir  string
	PacketPath       string
	DiffPath         string
	ProjectName      string
	Gate             gate.Options
}

func Parse(args []string) (Options, error) {
	opt := Options{Command: "status", Pack: "vmp-re"}
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
		case "-Lane", "--lane":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Lane")
			}
			opt.Gate.Lane = args[i]
		case "-Subject", "--subject":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Subject")
			}
			opt.Gate.Subject = args[i]
		case "-Summary", "--summary":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Summary")
			}
			opt.Gate.Summary = args[i]
		case "-Actor", "--actor":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Actor")
			}
			opt.Gate.Actor = args[i]
		case "-Risk", "--risk":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Risk")
			}
			opt.Gate.Risk = args[i]
		case "-TargetRef", "--target-ref":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -TargetRef")
			}
			opt.Gate.TargetRef = args[i]
		case "-BatchId", "--batch-id":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -BatchId")
			}
			opt.Gate.BatchID = args[i]
		case "-Scope", "--scope":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("missing value for -Scope")
			}
			opt.Gate.Scope = args[i]
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
		default:
			if i == 0 && args[i] != "" && args[i][0] != '-' {
				opt.Command = args[i]
			}
		}
	}
	if opt.Command == "" {
		opt.Command = "status"
	}
	if opt.Pack == "" {
		opt.Pack = "vmp-re"
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
	case "status":
		return runStatus(ctx, stdout)
	case "doctor", "validate":
		return runDoctor(ctx, stdout)
	case "attach":
		return runAttach(ctx, opt, stdout)
	case "repair":
		return runRepair(ctx, opt, stdout)
	case "init", "bootstrap":
		return runInitBootstrap(ctx, opt, stdout)
	case "sync", "update":
		return runSyncReview(ctx, opt, stdout)
	case "promote":
		return runPromoteReview(ctx, opt, stdout)
	case "overview":
		return runOverview(ctx, stdout)
	case "gate":
		return runGate(ctx, opt, stdout)
	default:
		return fmt.Errorf("go backend does not implement command yet: %s", opt.Command)
	}
}

func Main() int {
	if err := Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runStatus(ctx runtime.Context, out io.Writer) error {
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

func runDoctor(ctx runtime.Context, out io.Writer) error {
	if !ctx.TargetProvided {
		if instance.LooksLikeCase(ctx.Cwd) && !samePath(ctx.Cwd, ctx.RepoRoot) {
			return runCaseDoctor(ctx, ctx.Cwd, out)
		}
		return runPackDoctor(ctx, out)
	}
	if samePath(ctx.Target, ctx.RepoRoot) {
		return runPackDoctor(ctx, out)
	}
	if instance.LooksLikeCase(ctx.Target) {
		return runCaseDoctor(ctx, ctx.Target, out)
	}
	return fmt.Errorf("target is neither this kit root nor an attached rekit case: %s", ctx.Target)
}

func runPackDoctor(ctx runtime.Context, out io.Writer) error {
	rows, err := doctor.Pack(ctx.RepoRoot, ctx.Pack)
	if err != nil {
		return err
	}
	printRows(out, rows)
	fmt.Fprintln(out, "pack validation ok")
	return nil
}

func runCaseDoctor(ctx runtime.Context, target string, out io.Writer) error {
	rows, err := doctor.Case(ctx.RepoRoot, target, ctx.Pack)
	if err != nil {
		return err
	}
	printRows(out, rows)
	fmt.Fprintln(out, "instance validation ok")
	return nil
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
	if opt.WhatIf {
		return fmt.Errorf("go backend sync does not implement -WhatIf; run review-only sync without -Apply, or use -Apply for explicit write")
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
		result, err := syncreview.Apply(ctx.RepoRoot, ctx.Target, ctx.Pack, syncreview.ApplyOptions{ProjectName: opt.ProjectName, ForceLocalTemplates: opt.Force})
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

func runOverview(ctx runtime.Context, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("overview requires an explicit -Target attached case")
	}
	text, err := overview.Render(ctx.RepoRoot, ctx.Target, ctx.Pack)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
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
