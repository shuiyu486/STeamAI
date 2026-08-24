package executioncontrol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	SchemaVersion = 1

	ActionPause  = "pause"
	ActionResume = "resume"
	ActionStop   = "stop"

	StateRunning = "running"
	StatePaused  = "paused"
	StateStopped = "stopped"

	controlDir      = "execution-control"
	maxControlFiles = 10000
	maxJSONBytes    = 256 * 1024
)

var controlArtifactName = regexp.MustCompile(`^([0-9]{20})(\.intent)?\.json$`)
var applyAfterIntentHook func() error
var applyAfterReceiptHook func() error

type Options struct {
	Lane               string
	Action             string
	Actor              string
	Reason             string
	PublicationStamp   string
	ExpectedPlanSHA256 string
}

type Plan struct {
	SchemaVersion        int                `json:"schemaVersion"`
	Mode                 string             `json:"mode"`
	CaseRoot             string             `json:"caseRoot"`
	Lane                 string             `json:"lane"`
	Action               string             `json:"action"`
	PreviousState        string             `json:"previousState"`
	State                string             `json:"state"`
	ControlGeneration    int                `json:"controlGeneration"`
	PreviousReceiptSHA   string             `json:"previousReceiptSha256,omitempty"`
	Owner                laneowner.Snapshot `json:"owner"`
	Actor                string             `json:"actor"`
	Reason               string             `json:"reason"`
	PublicationStamp     string             `json:"publicationStamp"`
	IntentPath           string             `json:"intentPath"`
	ReceiptPath          string             `json:"receiptPath"`
	ExpectedPlanSHA256   string             `json:"expectedPlanSha256"`
	ApplyCommand         string             `json:"applyCommand,omitempty"`
	ReviewRequired       bool               `json:"reviewRequired"`
	RequiresConfirmation bool               `json:"requiresConfirmation"`
	Applied              bool               `json:"applied"`
	AlreadyApplied       bool               `json:"alreadyApplied"`
	NoAuthority          bool               `json:"noAuthority"`
	NoConfirmed          bool               `json:"noConfirmed"`
	NoHeavyTool          bool               `json:"noHeavyTool"`
	NoAutoResume         bool               `json:"noAutoResume"`
	Boundary             []string           `json:"boundary"`
}

type Intent struct {
	SchemaVersion      int                `json:"schemaVersion"`
	Kind               string             `json:"kind"`
	Lane               string             `json:"lane"`
	Action             string             `json:"action"`
	PreviousState      string             `json:"previousState"`
	State              string             `json:"state"`
	ControlGeneration  int                `json:"controlGeneration"`
	PreviousReceiptSHA string             `json:"previousReceiptSha256,omitempty"`
	Owner              laneowner.Snapshot `json:"owner"`
	Actor              string             `json:"actor"`
	Reason             string             `json:"reason"`
	PublicationStamp   string             `json:"publicationStamp"`
	IntentPath         string             `json:"intentPath"`
	ReceiptPath        string             `json:"receiptPath"`
	PlanSHA256         string             `json:"planSha256"`
	NoAuthority        bool               `json:"noAuthority"`
	NoConfirmed        bool               `json:"noConfirmed"`
	NoHeavyTool        bool               `json:"noHeavyTool"`
	NoAutoResume       bool               `json:"noAutoResume"`
}

type Receipt struct {
	SchemaVersion      int                `json:"schemaVersion"`
	Kind               string             `json:"kind"`
	Lane               string             `json:"lane"`
	Action             string             `json:"action"`
	PreviousState      string             `json:"previousState"`
	State              string             `json:"state"`
	ControlGeneration  int                `json:"controlGeneration"`
	PreviousReceiptSHA string             `json:"previousReceiptSha256,omitempty"`
	Owner              laneowner.Snapshot `json:"owner"`
	Actor              string             `json:"actor"`
	Reason             string             `json:"reason"`
	PublicationStamp   string             `json:"publicationStamp"`
	CommittedAt        string             `json:"committedAt"`
	IntentPath         string             `json:"intentPath"`
	IntentSHA256       string             `json:"intentSha256"`
	PlanSHA256         string             `json:"planSha256"`
	NoAuthority        bool               `json:"noAuthority"`
	NoConfirmed        bool               `json:"noConfirmed"`
	NoHeavyTool        bool               `json:"noHeavyTool"`
	NoAutoResume       bool               `json:"noAutoResume"`
}

type Inspection struct {
	Lane                 string              `json:"lane"`
	State                string              `json:"state"`
	CurrentGeneration    int                 `json:"currentGeneration"`
	CurrentReceiptSHA256 string              `json:"currentReceiptSha256,omitempty"`
	CurrentReceiptPath   string              `json:"currentReceiptPath,omitempty"`
	CurrentOwner         *laneowner.Snapshot `json:"currentOwner,omitempty"`
	Pending              bool                `json:"pending"`
	PendingGeneration    int                 `json:"pendingGeneration,omitempty"`
	PendingIntentPath    string              `json:"pendingIntentPath,omitempty"`
	PendingIntent        *Intent             `json:"pendingIntent,omitempty"`
	Receipts             []Receipt           `json:"receipts,omitempty"`
}

type planBinding struct {
	SchemaVersion      int                `json:"schemaVersion"`
	Kind               string             `json:"kind"`
	Lane               string             `json:"lane"`
	Action             string             `json:"action"`
	PreviousState      string             `json:"previousState"`
	State              string             `json:"state"`
	ControlGeneration  int                `json:"controlGeneration"`
	PreviousReceiptSHA string             `json:"previousReceiptSha256,omitempty"`
	Owner              laneowner.Snapshot `json:"owner"`
	Actor              string             `json:"actor"`
	Reason             string             `json:"reason"`
	PublicationStamp   string             `json:"publicationStamp"`
	IntentPath         string             `json:"intentPath"`
	ReceiptPath        string             `json:"receiptPath"`
	NoAuthority        bool               `json:"noAuthority"`
	NoConfirmed        bool               `json:"noConfirmed"`
	NoHeavyTool        bool               `json:"noHeavyTool"`
	NoAutoResume       bool               `json:"noAutoResume"`
}

func Preview(caseRoot string, opt Options) (Plan, error) {
	caseRoot, opt, err := normalizeOptions(caseRoot, opt, true)
	if err != nil {
		return Plan{}, err
	}
	if _, err := plancontract.ValidatePhase(
		commands.Control,
		"-ExpectedControlPlanSha256",
		true,
		false,
		opt.ExpectedPlanSHA256,
	); err != nil {
		return Plan{}, err
	}
	inspection, err := Inspect(caseRoot, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	if inspection.Pending {
		return Plan{}, fmt.Errorf("lane %s control publication is pending; recover the exact original control Apply", opt.Lane)
	}
	owner, err := laneowner.Read(caseRoot, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	return buildPlan(caseRoot, opt, inspection, owner)
}

func Apply(caseRoot string, opt Options) (result Plan, retErr error) {
	caseRoot, opt, err := normalizeOptions(caseRoot, opt, false)
	if err != nil {
		return Plan{}, err
	}
	expectedPlanSHA256, err := plancontract.RequireApplyBinding(
		commands.Control,
		"-ExpectedControlPlanSha256",
		opt.ExpectedPlanSHA256,
	)
	if err != nil {
		return Plan{}, err
	}
	opt.ExpectedPlanSHA256 = expectedPlanSHA256
	lease, err := lanemutation.AcquireLane(caseRoot, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := lease.Validate(); err != nil {
		return Plan{}, err
	}
	inspection, err := Inspect(caseRoot, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	if replay, ok := committedReplay(caseRoot, opt, inspection); ok {
		if err := lease.Validate(); err != nil {
			return Plan{}, err
		}
		return replay, nil
	}

	var intent Intent
	if inspection.Pending {
		if inspection.PendingIntent == nil {
			return Plan{}, fmt.Errorf("lane %s control publication is pending without a durable intent", opt.Lane)
		}
		if _, err := plancontract.Match(
			commands.Control,
			"-ExpectedControlPlanSha256",
			opt.ExpectedPlanSHA256,
			inspection.PendingIntent.PlanSHA256,
		); err != nil {
			return Plan{}, err
		}
		if !intentMatchesOptions(*inspection.PendingIntent, opt) {
			return Plan{}, fmt.Errorf("lane %s control publication is pending; recover the exact original control Apply", opt.Lane)
		}
		intent = *inspection.PendingIntent
	} else {
		owner, err := laneowner.Read(caseRoot, opt.Lane)
		if err != nil {
			return Plan{}, err
		}
		plan, err := buildPlan(caseRoot, opt, inspection, owner)
		if err != nil {
			return Plan{}, err
		}
		if _, err := plancontract.Match(
			commands.Control,
			"-ExpectedControlPlanSha256",
			opt.ExpectedPlanSHA256,
			plan.ExpectedPlanSHA256,
		); err != nil {
			return Plan{}, err
		}
		intent = intentFromPlan(plan)
		intentBytes, err := canonical(intent)
		if err != nil {
			return Plan{}, err
		}
		if err := publish(caseRoot, intent.IntentPath, "lane execution control intent", intentBytes); err != nil {
			return Plan{}, err
		}
		if err := lease.Validate(); err != nil {
			return Plan{}, fmt.Errorf("control intent may already be durable: %w", err)
		}
		if applyAfterIntentHook != nil {
			if err := applyAfterIntentHook(); err != nil {
				return Plan{}, err
			}
		}
		inspection, err = Inspect(caseRoot, opt.Lane)
		if err != nil {
			return Plan{}, err
		}
		if !inspection.Pending || inspection.PendingIntent == nil || !intentEqual(intent, *inspection.PendingIntent) {
			return Plan{}, fmt.Errorf("control intent did not become the exact pending head")
		}
	}

	if err := validateIntent(caseRoot, intent, inspection.CurrentGeneration, inspection.CurrentReceiptSHA256, inspection.State); err != nil {
		return Plan{}, err
	}
	if err := lease.Validate(); err != nil {
		return Plan{}, err
	}
	currentOwner, err := laneowner.Read(caseRoot, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	if currentOwner != intent.Owner {
		return Plan{}, fmt.Errorf("lane %s durable executor owner changed during control Apply", opt.Lane)
	}
	intentBytes, err := canonical(intent)
	if err != nil {
		return Plan{}, err
	}
	receipt := receiptFromIntent(intent, hash(intentBytes))
	receiptBytes, err := canonical(receipt)
	if err != nil {
		return Plan{}, err
	}
	if err := publish(caseRoot, intent.ReceiptPath, "lane execution control receipt", receiptBytes); err != nil {
		return Plan{}, err
	}
	if err := lease.Validate(); err != nil {
		return Plan{}, fmt.Errorf("control receipt may already be durable: %w", err)
	}
	if applyAfterReceiptHook != nil {
		if err := applyAfterReceiptHook(); err != nil {
			return Plan{}, err
		}
	}
	committed, err := Inspect(caseRoot, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	if committed.Pending || committed.CurrentGeneration != intent.ControlGeneration || !strings.EqualFold(committed.CurrentReceiptSHA256, hash(receiptBytes)) {
		return Plan{}, fmt.Errorf("control receipt did not become the durable head")
	}
	return planFromIntent(caseRoot, intent, true, false), nil
}

func Inspect(caseRoot, lane string) (Inspection, error) {
	caseRoot, err := canonicalCaseRoot(caseRoot)
	if err != nil {
		return Inspection{}, err
	}
	lane = strings.TrimSpace(lane)
	if _, _, err := laneowner.Path(caseRoot, lane); err != nil {
		return Inspection{}, err
	}
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return Inspection{}, err
	}
	out := Inspection{Lane: lane, State: StateRunning}
	root, err := rekitfs.OpenAnchoredRoot(stateRoot.Path)
	if err != nil {
		return Inspection{}, err
	}
	defer root.Close()
	controlRel := filepath.Join("lanes", lane, controlDir)
	entries, err := root.ListNoFollow(controlRel, maxControlFiles)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return Inspection{}, err
	}
	type artifacts struct {
		intent  string
		receipt string
	}
	byGeneration := map[int]artifacts{}
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if entry.Name() == heldResultDir && infoErr == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		match := controlArtifactName.FindStringSubmatch(entry.Name())
		if match == nil || infoErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return Inspection{}, fmt.Errorf("unexpected lane execution control artifact: %s", entry.Name())
		}
		value, parseErr := parseGeneration(match[1])
		if parseErr != nil || value < 1 {
			return Inspection{}, fmt.Errorf("invalid lane execution control generation: %s", entry.Name())
		}
		item := byGeneration[value]
		if match[2] == ".intent" {
			if item.intent != "" {
				return Inspection{}, fmt.Errorf("duplicate lane execution control intent generation: %d", value)
			}
			item.intent = entry.Name()
		} else {
			if item.receipt != "" {
				return Inspection{}, fmt.Errorf("duplicate lane execution control receipt generation: %d", value)
			}
			item.receipt = entry.Name()
		}
		byGeneration[value] = item
	}
	generations := make([]int, 0, len(byGeneration))
	for generation := range byGeneration {
		generations = append(generations, generation)
	}
	sort.Ints(generations)
	for index, generation := range generations {
		if generation != index+1 {
			return Inspection{}, fmt.Errorf("lane execution control generation gap: got=%d want=%d", generation, index+1)
		}
		item := byGeneration[generation]
		if item.intent == "" {
			return Inspection{}, fmt.Errorf("lane execution control generation lacks intent: %d", generation)
		}
		intentRel := filepath.Join(controlRel, item.intent)
		intentBytes, _, err := root.ReadStableFile(intentRel, maxJSONBytes)
		if err != nil {
			return Inspection{}, err
		}
		var intent Intent
		if err := decodeStrict(intentBytes, &intent); err != nil {
			return Inspection{}, fmt.Errorf("invalid lane execution control intent generation %d: %w", generation, err)
		}
		if err := validateIntent(caseRoot, intent, out.CurrentGeneration, out.CurrentReceiptSHA256, out.State); err != nil {
			return Inspection{}, err
		}
		if item.receipt == "" {
			if index != len(generations)-1 {
				return Inspection{}, fmt.Errorf("non-latest lane execution control generation is pending: %d", generation)
			}
			intentCopy := intent
			out.Pending = true
			out.PendingGeneration = generation
			out.PendingIntentPath = intent.IntentPath
			out.PendingIntent = &intentCopy
			return out, nil
		}
		receiptRel := filepath.Join(controlRel, item.receipt)
		receiptBytes, _, err := root.ReadStableFile(receiptRel, maxJSONBytes)
		if err != nil {
			return Inspection{}, err
		}
		var receipt Receipt
		if err := decodeStrict(receiptBytes, &receipt); err != nil {
			return Inspection{}, fmt.Errorf("invalid lane execution control receipt generation %d: %w", generation, err)
		}
		if err := validateReceipt(intent, intentBytes, receipt); err != nil {
			return Inspection{}, fmt.Errorf("invalid lane execution control receipt generation %d: %w", generation, err)
		}
		owner := receipt.Owner
		out.State = receipt.State
		out.CurrentGeneration = generation
		out.CurrentReceiptSHA256 = hash(receiptBytes)
		out.CurrentReceiptPath = intent.ReceiptPath
		out.CurrentOwner = &owner
		out.Receipts = append(out.Receipts, receipt)
	}
	return out, nil
}

func buildPlan(caseRoot string, opt Options, inspection Inspection, owner laneowner.Snapshot) (Plan, error) {
	state, err := transition(inspection.State, opt.Action)
	if err != nil {
		return Plan{}, err
	}
	generation := inspection.CurrentGeneration + 1
	intentPath, receiptPath, err := artifactPaths(caseRoot, opt.Lane, generation)
	if err != nil {
		return Plan{}, err
	}
	binding := planBinding{
		SchemaVersion:      SchemaVersion,
		Kind:               "lane-execution-control-plan",
		Lane:               opt.Lane,
		Action:             opt.Action,
		PreviousState:      inspection.State,
		State:              state,
		ControlGeneration:  generation,
		PreviousReceiptSHA: inspection.CurrentReceiptSHA256,
		Owner:              owner,
		Actor:              opt.Actor,
		Reason:             opt.Reason,
		PublicationStamp:   opt.PublicationStamp,
		IntentPath:         intentPath,
		ReceiptPath:        receiptPath,
		NoAuthority:        true,
		NoConfirmed:        true,
		NoHeavyTool:        true,
		NoAutoResume:       true,
	}
	planSHA, err := canonicalHash(binding)
	if err != nil {
		return Plan{}, err
	}
	applyCommand, err := controlApplyCommand(caseRoot, binding, planSHA)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		SchemaVersion: SchemaVersion, Mode: "control", CaseRoot: caseRoot, Lane: opt.Lane,
		Action: opt.Action, PreviousState: inspection.State, State: state,
		ControlGeneration: generation, PreviousReceiptSHA: inspection.CurrentReceiptSHA256,
		Owner: owner, Actor: opt.Actor, Reason: opt.Reason, PublicationStamp: opt.PublicationStamp,
		IntentPath: intentPath, ReceiptPath: receiptPath, ExpectedPlanSHA256: planSHA, ApplyCommand: applyCommand,
		ReviewRequired: true, RequiresConfirmation: true,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
		Boundary: boundaries(),
	}, nil
}

// PendingApplyCommand returns the exact reviewed Apply command for the durable
// pending head. It never creates or repairs control artifacts.
func PendingApplyCommand(caseRoot string, inspection Inspection) (string, error) {
	caseRoot, err := canonicalCaseRoot(caseRoot)
	if err != nil {
		return "", err
	}
	if !inspection.Pending || inspection.PendingIntent == nil {
		return "", fmt.Errorf("lane %s has no pending control publication", strings.TrimSpace(inspection.Lane))
	}
	intent := *inspection.PendingIntent
	if inspection.Lane != intent.Lane || inspection.PendingGeneration != intent.ControlGeneration {
		return "", fmt.Errorf("pending lane execution control identity is inconsistent")
	}
	if err := validateIntent(caseRoot, intent, inspection.CurrentGeneration, inspection.CurrentReceiptSHA256, inspection.State); err != nil {
		return "", err
	}
	return controlApplyCommand(caseRoot, planBindingFromIntent(intent), intent.PlanSHA256)
}

func controlApplyCommand(caseRoot string, binding planBinding, planSHA string) (string, error) {
	invocation, err := commands.NewPublicInvocation(
		commands.Control,
		"-Lane", binding.Lane,
		"-Action", binding.Action,
		"-Actor", binding.Actor,
		"-Reason", binding.Reason,
		"-ControlPublicationStamp", binding.PublicationStamp,
		"-ExpectedControlPlanSha256", planSHA,
		"-Apply", "-Format", "json",
	)
	if err != nil {
		return "", err
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return "", err
	}
	return invocation.RenderForEntrypoint(entrypoint)
}

func planBindingFromIntent(intent Intent) planBinding {
	return planBinding{
		SchemaVersion: SchemaVersion, Kind: "lane-execution-control-plan",
		Lane: intent.Lane, Action: intent.Action, PreviousState: intent.PreviousState, State: intent.State,
		ControlGeneration: intent.ControlGeneration, PreviousReceiptSHA: intent.PreviousReceiptSHA,
		Owner: intent.Owner, Actor: intent.Actor, Reason: intent.Reason, PublicationStamp: intent.PublicationStamp,
		IntentPath: intent.IntentPath, ReceiptPath: intent.ReceiptPath,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
}

func intentFromPlan(plan Plan) Intent {
	return Intent{
		SchemaVersion: SchemaVersion, Kind: "lane-execution-control-intent",
		Lane: plan.Lane, Action: plan.Action, PreviousState: plan.PreviousState, State: plan.State,
		ControlGeneration: plan.ControlGeneration, PreviousReceiptSHA: plan.PreviousReceiptSHA,
		Owner: plan.Owner, Actor: plan.Actor, Reason: plan.Reason, PublicationStamp: plan.PublicationStamp,
		IntentPath: plan.IntentPath, ReceiptPath: plan.ReceiptPath, PlanSHA256: plan.ExpectedPlanSHA256,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
}

func receiptFromIntent(intent Intent, intentSHA string) Receipt {
	return Receipt{
		SchemaVersion: SchemaVersion, Kind: "lane-execution-control-receipt",
		Lane: intent.Lane, Action: intent.Action, PreviousState: intent.PreviousState, State: intent.State,
		ControlGeneration: intent.ControlGeneration, PreviousReceiptSHA: intent.PreviousReceiptSHA,
		Owner: intent.Owner, Actor: intent.Actor, Reason: intent.Reason,
		PublicationStamp: intent.PublicationStamp, CommittedAt: intent.PublicationStamp,
		IntentPath: intent.IntentPath, IntentSHA256: intentSHA, PlanSHA256: intent.PlanSHA256,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
}

func planFromIntent(caseRoot string, intent Intent, applied, replay bool) Plan {
	return Plan{
		SchemaVersion: SchemaVersion, Mode: "control", CaseRoot: caseRoot,
		Lane: intent.Lane, Action: intent.Action, PreviousState: intent.PreviousState, State: intent.State,
		ControlGeneration: intent.ControlGeneration, PreviousReceiptSHA: intent.PreviousReceiptSHA,
		Owner: intent.Owner, Actor: intent.Actor, Reason: intent.Reason, PublicationStamp: intent.PublicationStamp,
		IntentPath: intent.IntentPath, ReceiptPath: intent.ReceiptPath, ExpectedPlanSHA256: intent.PlanSHA256,
		ReviewRequired: true, RequiresConfirmation: true, Applied: applied, AlreadyApplied: replay,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
		Boundary: boundaries(),
	}
}

func committedReplay(caseRoot string, opt Options, inspection Inspection) (Plan, bool) {
	for _, receipt := range inspection.Receipts {
		if !strings.EqualFold(receipt.PlanSHA256, opt.ExpectedPlanSHA256) || receipt.Lane != opt.Lane || receipt.Action != opt.Action || receipt.Actor != opt.Actor || receipt.Reason != opt.Reason || receipt.PublicationStamp != opt.PublicationStamp {
			continue
		}
		intent := Intent{
			SchemaVersion: SchemaVersion, Kind: "lane-execution-control-intent",
			Lane: receipt.Lane, Action: receipt.Action, PreviousState: receipt.PreviousState, State: receipt.State,
			ControlGeneration: receipt.ControlGeneration, PreviousReceiptSHA: receipt.PreviousReceiptSHA,
			Owner: receipt.Owner, Actor: receipt.Actor, Reason: receipt.Reason,
			PublicationStamp: receipt.PublicationStamp, IntentPath: receipt.IntentPath,
			PlanSHA256: receipt.PlanSHA256, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
		}
		_, receiptPath, err := artifactPaths(caseRoot, receipt.Lane, receipt.ControlGeneration)
		if err != nil {
			return Plan{}, false
		}
		intent.ReceiptPath = receiptPath
		return planFromIntent(caseRoot, intent, true, true), true
	}
	return Plan{}, false
}

func validateIntent(caseRoot string, intent Intent, previousGeneration int, previousReceiptSHA, previousState string) error {
	if intent.SchemaVersion != SchemaVersion || intent.Kind != "lane-execution-control-intent" || intent.ControlGeneration != previousGeneration+1 || intent.PreviousReceiptSHA != previousReceiptSHA || intent.PreviousState != previousState || intent.Owner.Lane != intent.Lane || strings.TrimSpace(intent.Owner.CurrentExecutor) == "" || intent.Owner.ExecutorGeneration < 1 || strings.TrimSpace(intent.Actor) == "" || strings.TrimSpace(intent.Reason) == "" || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool || !intent.NoAutoResume {
		return fmt.Errorf("invalid lane execution control intent generation %d", intent.ControlGeneration)
	}
	state, err := transition(intent.PreviousState, intent.Action)
	if err != nil || state != intent.State {
		return fmt.Errorf("invalid lane execution control transition generation %d", intent.ControlGeneration)
	}
	if _, err := time.Parse(time.RFC3339Nano, intent.PublicationStamp); err != nil {
		return fmt.Errorf("invalid lane execution control publication stamp: %w", err)
	}
	intentPath, receiptPath, err := artifactPaths(caseRoot, intent.Lane, intent.ControlGeneration)
	if err != nil {
		return err
	}
	if intent.IntentPath != intentPath || intent.ReceiptPath != receiptPath || !validSHA256(intent.PlanSHA256) {
		return fmt.Errorf("invalid lane execution control artifact binding generation %d", intent.ControlGeneration)
	}
	binding := planBinding{
		SchemaVersion: SchemaVersion, Kind: "lane-execution-control-plan",
		Lane: intent.Lane, Action: intent.Action, PreviousState: intent.PreviousState, State: intent.State,
		ControlGeneration: intent.ControlGeneration, PreviousReceiptSHA: intent.PreviousReceiptSHA,
		Owner: intent.Owner, Actor: intent.Actor, Reason: intent.Reason, PublicationStamp: intent.PublicationStamp,
		IntentPath: intent.IntentPath, ReceiptPath: intent.ReceiptPath,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
	planSHA, err := canonicalHash(binding)
	if err != nil || !strings.EqualFold(planSHA, intent.PlanSHA256) {
		return fmt.Errorf("lane execution control plan hash mismatch generation %d", intent.ControlGeneration)
	}
	return nil
}

func validateReceipt(intent Intent, intentBytes []byte, receipt Receipt) error {
	if receipt.SchemaVersion != SchemaVersion || receipt.Kind != "lane-execution-control-receipt" || receipt.Lane != intent.Lane || receipt.Action != intent.Action || receipt.PreviousState != intent.PreviousState || receipt.State != intent.State || receipt.ControlGeneration != intent.ControlGeneration || receipt.PreviousReceiptSHA != intent.PreviousReceiptSHA || receipt.Owner != intent.Owner || receipt.Actor != intent.Actor || receipt.Reason != intent.Reason || receipt.PublicationStamp != intent.PublicationStamp || receipt.CommittedAt != intent.PublicationStamp || receipt.IntentPath != intent.IntentPath || !strings.EqualFold(receipt.IntentSHA256, hash(intentBytes)) || !strings.EqualFold(receipt.PlanSHA256, intent.PlanSHA256) || !receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool || !receipt.NoAutoResume {
		return fmt.Errorf("receipt does not match its exact intent")
	}
	return nil
}

func normalizeOptions(caseRoot string, opt Options, allowGeneratedStamp bool) (string, Options, error) {
	caseRoot, err := canonicalCaseRoot(caseRoot)
	if err != nil {
		return "", Options{}, err
	}
	opt.Lane = strings.TrimSpace(opt.Lane)
	opt.Action = strings.ToLower(strings.TrimSpace(opt.Action))
	opt.Actor = strings.TrimSpace(opt.Actor)
	opt.Reason = strings.TrimSpace(opt.Reason)
	opt.PublicationStamp = strings.TrimSpace(opt.PublicationStamp)
	opt.ExpectedPlanSHA256 = strings.ToLower(strings.TrimSpace(opt.ExpectedPlanSHA256))
	if _, _, err := laneowner.Path(caseRoot, opt.Lane); err != nil {
		return "", Options{}, err
	}
	if opt.Action != ActionPause && opt.Action != ActionResume && opt.Action != ActionStop {
		return "", Options{}, fmt.Errorf("control action must be pause, resume, or stop")
	}
	if opt.Actor == "" || opt.Reason == "" {
		return "", Options{}, fmt.Errorf("control requires non-empty -Actor and -Reason")
	}
	if opt.PublicationStamp == "" && allowGeneratedStamp {
		opt.PublicationStamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if opt.PublicationStamp == "" {
		return "", Options{}, fmt.Errorf("control apply requires -ControlPublicationStamp from the reviewed preview")
	}
	if _, err := time.Parse(time.RFC3339Nano, opt.PublicationStamp); err != nil {
		return "", Options{}, fmt.Errorf("invalid control publication stamp: %w", err)
	}
	return caseRoot, opt, nil
}

func transition(previous, action string) (string, error) {
	switch previous {
	case StateRunning:
		switch action {
		case ActionPause:
			return StatePaused, nil
		case ActionStop:
			return StateStopped, nil
		}
	case StatePaused:
		switch action {
		case ActionResume:
			return StateRunning, nil
		case ActionStop:
			return StateStopped, nil
		}
	case StateStopped:
	}
	return "", fmt.Errorf("control action %s is invalid while lane state is %s", action, previous)
}

func artifactPaths(caseRoot, lane string, generation int) (string, string, error) {
	name := fmt.Sprintf("%020d", generation)
	intent, err := projectstate.Rel(caseRoot, "lanes", lane, controlDir, name+".intent.json")
	if err != nil {
		return "", "", err
	}
	receipt, err := projectstate.Rel(caseRoot, "lanes", lane, controlDir, name+".json")
	return intent, receipt, err
}

func publish(caseRoot, rel, label string, data []byte) error {
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	root, err := rekitfs.OpenAnchoredRoot(stateRoot.Path)
	if err != nil {
		return err
	}
	defer root.Close()
	stateRel, err := filepath.Rel(stateRoot.Path, filepath.Join(caseRoot, filepath.FromSlash(rel)))
	if err != nil || filepath.IsAbs(stateRel) || stateRel == "." || stateRel == ".." || strings.HasPrefix(stateRel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s path is outside selected state root: %s", label, rel)
	}
	if err := root.MkdirAllNoFollow(filepath.Dir(stateRel), 0o700); err != nil {
		return err
	}
	_, err = root.WriteExclusiveFileWriteThrough(stateRel, data, 0o600, true)
	return err
}

func canonicalCaseRoot(caseRoot string) (string, error) {
	caseRoot = strings.TrimSpace(caseRoot)
	if caseRoot == "" {
		return "", fmt.Errorf("control project root is empty")
	}
	full, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	full = filepath.Clean(full)
	if _, err := projectstate.Resolve(full); err != nil {
		return "", err
	}
	return full, nil
}

func intentMatchesOptions(intent Intent, opt Options) bool {
	return intent.Lane == opt.Lane && intent.Action == opt.Action && intent.Actor == opt.Actor && intent.Reason == opt.Reason && intent.PublicationStamp == opt.PublicationStamp && strings.EqualFold(intent.PlanSHA256, opt.ExpectedPlanSHA256)
}

func intentEqual(left, right Intent) bool {
	leftBytes, _ := json.Marshal(left)
	rightBytes, _ := json.Marshal(right)
	return bytes.Equal(leftBytes, rightBytes)
}

func canonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func canonicalHash(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return hash(data), nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("artifact must contain exactly one JSON object")
	}
	return nil
}

func parseGeneration(value string) (int, error) {
	var generation int
	_, err := fmt.Sscanf(value, "%d", &generation)
	return generation, err
}

func boundaries() []string {
	return []string{
		"control generation is independent from executor, external-attempt, supervisor-run, and gate generations",
		"control records no authority, confirmed state, or heavy-tool authorization",
		"resume changes only durable control state and does not launch or accept prior work",
		"stop becomes durable before any optional process actuation",
	}
}
