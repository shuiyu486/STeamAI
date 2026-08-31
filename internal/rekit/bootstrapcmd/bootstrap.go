package bootstrapcmd

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
)

const (
	Command           = "bootstrap"
	confirmationToken = "APPLY"
)

var currentGOOS = runtime.GOOS

type PackChoice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended"`
}

type Continuation struct {
	Executable             string   `json:"executable"`
	ExecutableSHA256       string   `json:"executableSha256"`
	ExecutableBytes        int64    `json:"executableBytes"`
	BundleManifest         string   `json:"bundleManifest"`
	BundleManifestSHA256   string   `json:"bundleManifestSha256"`
	Arguments              []string `json:"arguments"`
	Goal                   string   `json:"goal"`
	RequiresExplicitChoice bool     `json:"requiresExplicitChoice"`
	NoAutoResume           bool     `json:"noAutoResume"`
}

type Result struct {
	SchemaVersion        int                      `json:"schemaVersion"`
	Command              string                   `json:"command"`
	State                string                   `json:"state"`
	Target               string                   `json:"target"`
	Goal                 string                   `json:"goal"`
	Pack                 string                   `json:"pack,omitempty"`
	PackChoices          []PackChoice             `json:"packChoices,omitempty"`
	Preview              *sessionhost.DailyResult `json:"preview,omitempty"`
	Apply                *sessionhost.DailyResult `json:"apply,omitempty"`
	Continuation         *Continuation            `json:"continuation,omitempty"`
	IsMutation           bool                     `json:"isMutation"`
	Applied              bool                     `json:"applied"`
	ReviewRequired       bool                     `json:"reviewRequired"`
	RequiresConfirmation bool                     `json:"requiresConfirmation"`
	NoAutoResume         bool                     `json:"noAutoResume"`
	Boundary             []string                 `json:"boundary"`
}

type Options struct {
	Target string
	Goal   string
	Pack   string
	Format string
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, repoRoot, executable string) int {
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	flags := flag.NewFlagSet("steamai bootstrap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var opt Options
	flags.StringVar(&opt.Target, "target", "", "existing ordinary project directory")
	flags.StringVar(&opt.Goal, "goal", "", "natural-language project goal")
	flags.StringVar(&opt.Pack, "pack", "", "selected mature pack")
	flags.StringVar(&opt.Format, "format", "", "output format: json (preview-only)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "bootstrap does not accept positional arguments")
		return 2
	}
	if strings.TrimSpace(opt.Format) != "" && !strings.EqualFold(strings.TrimSpace(opt.Format), "json") {
		fmt.Fprintln(stderr, "bootstrap supports only -format json")
		return 2
	}
	result, err := Execute(stdin, stderr, repoRoot, executable, opt)
	if err != nil && !result.IsMutation {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr != nil {
		fmt.Fprintln(stderr, marshalErr)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	if _, writeErr := fmt.Fprintln(stdout, string(data)); writeErr != nil {
		fmt.Fprintln(stderr, writeErr)
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}

func Execute(stdin io.Reader, previewOut io.Writer, repoRoot, executable string, opt Options) (Result, error) {
	boundary := []string{
		"bootstrap is Windows-only and runs from one verified unified source-clone executable",
		"pack selection and preview are zero-write; Apply consumes only the exact preview hash in the same process",
		"Apply delegates to the canonical daily directory-adoption owner and publishes only project-local assets",
		"bootstrap never discovers or launches Claude, executes heavy tools, or writes authority/confirmed state",
		"the returned typed continuation preserves the original goal and is never executed automatically",
	}
	if currentGOOS != "windows" {
		return Result{}, fmt.Errorf("STeamAI bootstrap is currently supported only on Windows")
	}
	if err := runtimebundle.ValidateUnifiedExecutableRole(executable); err != nil {
		return Result{}, err
	}
	repoRoot, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(opt.Target) == "" {
		return Result{}, fmt.Errorf("bootstrap requires an exact existing -target directory")
	}
	target, err := filepath.Abs(strings.TrimSpace(opt.Target))
	if err != nil {
		return Result{}, err
	}
	if _, err := rekitfs.ValidateNonReparseDirectory(target, "STeamAI bootstrap target"); err != nil {
		return Result{}, err
	}
	goal, err := validateBootstrapGoal(opt.Goal)
	if err != nil {
		return Result{}, err
	}
	choices, err := maturePackChoices(repoRoot)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		SchemaVersion:  1,
		Command:        Command,
		Target:         target,
		Goal:           goal,
		PackChoices:    choices,
		ReviewRequired: true,
		NoAutoResume:   true,
		Boundary:       boundary,
	}
	pack := strings.TrimSpace(opt.Pack)
	if pack == "" {
		classifiedRoot, classifyErr := sessionhost.RequireOrdinaryDirectoryTarget(target)
		if classifyErr != nil {
			return Result{}, fmt.Errorf("classify bootstrap target: %w", classifyErr)
		}
		if !strings.EqualFold(filepath.Clean(classifiedRoot), filepath.Clean(target)) {
			return Result{}, fmt.Errorf("bootstrap target classification changed its exact root")
		}
		result.State = "pack-selection-required"
		return result, nil
	}
	if !selectablePack(choices, pack) {
		return Result{}, fmt.Errorf("bootstrap pack %q is not a schema-valid mature selectable pack", pack)
	}
	result.Pack = pack
	preview, err := sessionhost.RunDaily(context.Background(), sessionhost.DailyOptions{
		Target:                         target,
		Goal:                           goal,
		DirectoryAdoptionAction:        "initialize-in-place",
		DirectoryAdoptionPack:          pack,
		InitializationRepoRoot:         repoRoot,
		InitializationSourceExecutable: executable,
	})
	if err != nil {
		return Result{}, fmt.Errorf("preview bootstrap adoption: %w", err)
	}
	result.Preview = &preview
	if preview.FinalState != sessionhost.DailyActionConfirmationRequired ||
		preview.DirectoryAdoption == nil || preview.DirectoryAdoption.Plan == nil ||
		!preview.DirectoryAdoption.Plan.AdoptionReady ||
		strings.TrimSpace(preview.DirectoryAdoption.Plan.ExpectedPlanSHA256) == "" ||
		preview.SessionLaunches != 0 || preview.SessionCompletions != 0 {
		result.State = "bootstrap-blocked"
		return result, nil
	}
	result.State = sessionhost.DailyActionConfirmationRequired
	result.RequiresConfirmation = true
	if strings.EqualFold(strings.TrimSpace(opt.Format), "json") {
		return result, nil
	}
	if err := writePreview(previewOut, preview); err != nil {
		return Result{}, err
	}
	token, err := readConfirmation(stdin)
	if err != nil {
		return Result{}, err
	}
	if token != confirmationToken {
		result.State = "bootstrap-cancelled"
		result.RequiresConfirmation = false
		return result, nil
	}
	applied, err := sessionhost.RunDaily(context.Background(), sessionhost.DailyOptions{
		Target:                         target,
		Goal:                           goal,
		DirectoryAdoptionAction:        "confirm-exact-plan",
		DirectoryAdoptionPack:          pack,
		ExpectedInitPlanSHA256:         preview.DirectoryAdoption.Plan.ExpectedPlanSHA256,
		InitializationRepoRoot:         repoRoot,
		InitializationSourceExecutable: executable,
	})
	if err != nil {
		return Result{}, fmt.Errorf("apply exact bootstrap adoption: %w", err)
	}
	result.Apply = &applied
	result.IsMutation = applied.DirectoryAdoption != nil && applied.DirectoryAdoption.Apply != nil && applied.DirectoryAdoption.Apply.Applied
	result.Applied = result.IsMutation
	verificationFailed := func(err error) (Result, error) {
		result.State = "bootstrap-verification-failed"
		result.ReviewRequired = false
		result.RequiresConfirmation = false
		return result, err
	}
	if applied.FinalState != sessionhost.DailyActionReadyToContinue ||
		applied.DirectoryAdoption == nil || applied.DirectoryAdoption.Apply == nil ||
		!applied.DirectoryAdoption.Apply.Applied || applied.OnboardingApplied ||
		applied.SessionLaunches != 0 || applied.SessionCompletions != 0 {
		return verificationFailed(fmt.Errorf("bootstrap adoption Apply crossed its bounded no-launch contract"))
	}
	inst, err := instance.Read(target)
	if err != nil {
		return verificationFailed(fmt.Errorf("verify bootstrapped project metadata: %w", err))
	}
	if inst.SchemaVersion < 2 || inst.Mode != "project-local-bundle" || inst.TemplatePack != pack {
		return verificationFailed(fmt.Errorf("bootstrap did not publish the selected project-local pack binding"))
	}
	localExecutable := filepath.Join(target, ".steamai", "runtime", "bin", runtimebundle.ExecutableName())
	if err := runtimebundle.ValidateUnifiedExecutableRole(localExecutable); err != nil {
		return verificationFailed(fmt.Errorf("verify bootstrapped project-local executable: %w", err))
	}
	assetRoot, projectLocal, err := runtimebundle.AssetRootForExecutable(localExecutable)
	if err != nil {
		return verificationFailed(fmt.Errorf("verify bootstrapped executable ownership: %w", err))
	}
	assetRootAbs, err := filepath.Abs(assetRoot)
	if err != nil {
		return verificationFailed(fmt.Errorf("resolve bootstrapped asset root: %w", err))
	}
	expectedAssetRootAbs, err := filepath.Abs(filepath.Join(target, ".steamai"))
	if err != nil {
		return verificationFailed(fmt.Errorf("resolve expected bootstrapped asset root: %w", err))
	}
	if !projectLocal || !strings.EqualFold(filepath.Clean(assetRootAbs), filepath.Clean(expectedAssetRootAbs)) {
		return verificationFailed(fmt.Errorf("bootstrapped executable is not owned by the exact target project"))
	}
	manifest, err := runtimebundle.Validate(assetRoot, inst.BundleManifest, inst.BundleManifestSHA256, pack)
	if err != nil {
		return verificationFailed(fmt.Errorf("verify bootstrapped project-local bundle: %w", err))
	}
	if err := runtimebundle.ValidateExecutableIdentity(assetRoot, localExecutable, manifest); err != nil {
		return verificationFailed(fmt.Errorf("verify bootstrapped project-local executable identity: %w", err))
	}
	result.State = sessionhost.DailyActionReadyToContinue
	result.Apply = &applied
	result.PackChoices = nil
	result.IsMutation = true
	result.Applied = true
	result.ReviewRequired = false
	result.RequiresConfirmation = false
	result.Continuation = &Continuation{
		Executable:             localExecutable,
		ExecutableSHA256:       manifest.Executable.SHA256,
		ExecutableBytes:        manifest.Executable.Size,
		BundleManifest:         inst.BundleManifest,
		BundleManifestSHA256:   inst.BundleManifestSHA256,
		Arguments:              []string{"host", "-daily", "-goal", goal},
		Goal:                   goal,
		RequiresExplicitChoice: true,
		NoAutoResume:           true,
	}
	return result, nil
}

func validateBootstrapGoal(value string) (string, error) {
	goal := strings.TrimSpace(value)
	if goal == "" {
		return "", fmt.Errorf("bootstrap requires -goal with the original natural-language project goal")
	}
	if !utf8.ValidString(goal) || len([]byte(goal)) > 4096 {
		return "", fmt.Errorf("bootstrap goal must be valid UTF-8 and at most 4096 bytes")
	}
	return goal, nil
}

func maturePackChoices(repoRoot string) ([]PackChoice, error) {
	packs, err := manifest.List(repoRoot)
	if err != nil {
		return nil, err
	}
	choices := make([]PackChoice, 0, len(packs))
	for _, pack := range packs {
		if !pack.SchemaValid || !strings.EqualFold(strings.TrimSpace(pack.Maturity), "mature") {
			continue
		}
		choices = append(choices, PackChoice{
			ID:          pack.ID,
			Name:        pack.Name,
			Description: pack.Description,
			Recommended: strings.EqualFold(pack.ID, defaults.DefaultPack),
		})
	}
	sort.Slice(choices, func(i, j int) bool { return strings.ToLower(choices[i].ID) < strings.ToLower(choices[j].ID) })
	if len(choices) == 0 {
		return nil, fmt.Errorf("bootstrap source clone has no schema-valid mature packs")
	}
	return choices, nil
}

func selectablePack(choices []PackChoice, selected string) bool {
	for _, choice := range choices {
		if choice.ID == selected {
			return true
		}
	}
	return false
}

func writePreview(out io.Writer, preview sessionhost.DailyResult) error {
	if out == nil {
		out = io.Discard
	}
	data, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "STeamAI bootstrap exact preview:\n%s\n", data); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Type %s to apply this exact plan; any other input cancels: ", confirmationToken)
	return err
}

func readConfirmation(in io.Reader) (string, error) {
	if in == nil {
		return "", nil
	}
	line, err := bufio.NewReader(io.LimitReader(in, 64)).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read bootstrap confirmation: %w", err)
	}
	return strings.TrimSpace(line), nil
}
