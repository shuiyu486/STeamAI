package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/promote"
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
}

func Parse(args []string) (Options, error) {
	opt := Options{Command: "status", Pack: "vmp-re"}
	for i := 0; i < len(args); i++ {
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
		case "-Review", "--review":
			opt.Review = true
		case "-Apply", "--apply":
			opt.Apply = true
		case "-CreateCandidates", "--create-candidates":
			opt.CreateCandidates = true
		case "-WhatIf", "--what-if":
			opt.WhatIf = true
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
	case "sync", "update":
		return runSyncReview(ctx, opt, stdout)
	case "promote":
		return runPromoteReview(ctx, opt, stdout)
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
			return fmt.Errorf("go backend does not implement case doctor yet: %s", ctx.Cwd)
		}
		return runPackDoctor(ctx, out)
	}
	if samePath(ctx.Target, ctx.RepoRoot) {
		return runPackDoctor(ctx, out)
	}
	if instance.LooksLikeCase(ctx.Target) {
		return fmt.Errorf("go backend does not implement case doctor yet: %s", ctx.Target)
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

func runSyncReview(ctx runtime.Context, opt Options, out io.Writer) error {
	if opt.Apply || opt.WhatIf {
		return fmt.Errorf("go backend only implements sync review-only planning")
	}
	if !ctx.TargetProvided {
		return fmt.Errorf("sync review requires an explicit -Target attached case")
	}
	plan, err := syncreview.Plan(ctx.RepoRoot, ctx.Target, ctx.Pack)
	if err != nil {
		return err
	}
	return writeReviewPlan(out, plan)
}

func runPromoteReview(ctx runtime.Context, opt Options, out io.Writer) error {
	if opt.Apply || opt.CreateCandidates || opt.WhatIf {
		return fmt.Errorf("go backend only implements promote review-only planning")
	}
	if !ctx.TargetProvided {
		return fmt.Errorf("promote review requires an explicit -Target attached case")
	}
	plan, err := promote.Plan(ctx.RepoRoot, ctx.Target, ctx.Pack)
	if err != nil {
		return err
	}
	return writeReviewPlan(out, plan)
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
