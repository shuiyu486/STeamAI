package executioncontrol

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
)

const BindingSchemaVersion = 1

type Binding struct {
	SchemaVersion        int                `json:"schemaVersion"`
	Lane                 string             `json:"lane"`
	ControlGeneration    int                `json:"controlGeneration"`
	ControlReceiptSHA256 string             `json:"controlReceiptSha256,omitempty"`
	Owner                laneowner.Snapshot `json:"owner"`
}

type BindingCurrentness struct {
	Current     bool   `json:"current"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
}

type BindingNotCurrentError struct {
	Currentness BindingCurrentness
}

func (err *BindingNotCurrentError) Error() string {
	if err == nil {
		return "execution control binding is not current"
	}
	return fmt.Sprintf("execution control binding disposition is %s: %s", err.Currentness.Disposition, err.Currentness.Reason)
}

func CloneBinding(binding *Binding) *Binding {
	if binding == nil {
		return nil
	}
	copy := *binding
	return &copy
}

func SameBinding(left, right *Binding) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func ValidateBinding(binding Binding) error {
	if binding.SchemaVersion != BindingSchemaVersion ||
		strings.TrimSpace(binding.Lane) == "" || binding.Lane != strings.TrimSpace(binding.Lane) ||
		binding.ControlGeneration < 0 || binding.Owner.Lane != binding.Lane ||
		strings.TrimSpace(binding.Owner.CurrentExecutor) == "" ||
		binding.Owner.CurrentExecutor != strings.TrimSpace(binding.Owner.CurrentExecutor) ||
		binding.Owner.ExecutorGeneration <= 0 {
		return fmt.Errorf("execution control binding is invalid")
	}
	if binding.ControlGeneration == 0 {
		if strings.TrimSpace(binding.ControlReceiptSHA256) != "" {
			return fmt.Errorf("initial execution control binding must not name a receipt")
		}
		return nil
	}
	if !validSHA256(binding.ControlReceiptSHA256) {
		return fmt.Errorf("execution control binding receipt sha256 is invalid")
	}
	return nil
}

func (binding Binding) Birth() ResultBirth {
	return ResultBirth{
		ControlGeneration:    binding.ControlGeneration,
		ControlReceiptSHA256: binding.ControlReceiptSHA256,
		Owner:                binding.Owner,
	}
}

func CaptureBinding(caseRoot string, owner laneowner.Snapshot) (binding Binding, retErr error) {
	if err := validateBindingOwner(owner); err != nil {
		return Binding{}, err
	}
	lease, err := lanemutation.AcquireLane(caseRoot, owner.Lane)
	if err != nil {
		return Binding{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := lease.ValidateLaneFor(caseRoot, owner.Lane); err != nil {
		return Binding{}, err
	}
	return captureBindingWithValidation(caseRoot, owner, func() error {
		return lease.ValidateLaneFor(caseRoot, owner.Lane)
	})
}

func CaptureBindingWithProjectLease(caseRoot string, lease *lanemutation.Lease, owner laneowner.Snapshot) (Binding, error) {
	if lease == nil {
		return Binding{}, fmt.Errorf("execution control binding capture requires an existing project mutation lease")
	}
	if err := validateBindingOwner(owner); err != nil {
		return Binding{}, err
	}
	validate := func() error { return lease.ValidateProjectFor(caseRoot) }
	if err := validate(); err != nil {
		return Binding{}, err
	}
	return captureBindingWithValidation(caseRoot, owner, validate)
}

func InspectBindingWithLease(caseRoot string, lease *lanemutation.Lease, binding Binding) (BindingCurrentness, error) {
	if lease == nil {
		return BindingCurrentness{}, fmt.Errorf("execution control binding inspection requires an existing lane mutation lease")
	}
	return inspectBindingWithValidation(caseRoot, binding, func() error {
		return lease.ValidateLaneFor(caseRoot, binding.Lane)
	})
}

func RequireCurrentBindingWithLease(caseRoot string, lease *lanemutation.Lease, binding Binding) error {
	currentness, err := InspectBindingWithLease(caseRoot, lease, binding)
	return requireCurrentBinding(currentness, err)
}

func InspectBindingWithProjectLease(caseRoot string, lease *lanemutation.Lease, binding Binding) (BindingCurrentness, error) {
	if lease == nil {
		return BindingCurrentness{}, fmt.Errorf("execution control binding inspection requires an existing project mutation lease")
	}
	return inspectBindingWithValidation(caseRoot, binding, func() error {
		return lease.ValidateProjectFor(caseRoot)
	})
}

func RequireCurrentBindingWithProjectLease(caseRoot string, lease *lanemutation.Lease, binding Binding) error {
	currentness, err := InspectBindingWithProjectLease(caseRoot, lease, binding)
	return requireCurrentBinding(currentness, err)
}

func inspectBindingWithValidation(caseRoot string, binding Binding, validate func() error) (BindingCurrentness, error) {
	if err := ValidateBinding(binding); err != nil {
		return BindingCurrentness{}, err
	}
	if validate == nil {
		return BindingCurrentness{}, fmt.Errorf("execution control binding inspection requires a mutation owner validator")
	}
	if err := validate(); err != nil {
		return BindingCurrentness{}, err
	}
	inspection, err := Inspect(caseRoot, binding.Lane)
	if err != nil {
		return BindingCurrentness{}, err
	}
	owner, err := laneowner.Read(caseRoot, binding.Lane)
	if err != nil {
		return BindingCurrentness{}, err
	}
	if err := validate(); err != nil {
		return BindingCurrentness{}, err
	}
	disposition, reason := classifyResultPublication(binding.Birth(), inspection, owner)
	return BindingCurrentness{
		Current:     disposition == ResultDispositionCurrent,
		Disposition: disposition,
		Reason:      reason,
	}, nil
}

func requireCurrentBinding(currentness BindingCurrentness, err error) error {
	if err != nil {
		return err
	}
	if !currentness.Current {
		return &BindingNotCurrentError{Currentness: currentness}
	}
	return nil
}

func captureBindingWithValidation(caseRoot string, owner laneowner.Snapshot, validate func() error) (Binding, error) {
	if validate == nil {
		return Binding{}, fmt.Errorf("execution control binding capture requires a mutation owner validator")
	}
	if err := validate(); err != nil {
		return Binding{}, err
	}
	inspection, err := Inspect(caseRoot, owner.Lane)
	if err != nil {
		return Binding{}, err
	}
	if inspection.Pending {
		return Binding{}, fmt.Errorf("lane %s has pending execution control generation %d", owner.Lane, inspection.PendingGeneration)
	}
	if inspection.State != StateRunning {
		return Binding{}, fmt.Errorf("lane %s execution control state is %s", owner.Lane, inspection.State)
	}
	currentOwner, err := laneowner.Read(caseRoot, owner.Lane)
	if err != nil {
		return Binding{}, err
	}
	if currentOwner != owner {
		return Binding{}, fmt.Errorf("lane %s durable executor owner is stale", owner.Lane)
	}
	if err := validate(); err != nil {
		return Binding{}, err
	}
	binding := Binding{
		SchemaVersion:        BindingSchemaVersion,
		Lane:                 owner.Lane,
		ControlGeneration:    inspection.CurrentGeneration,
		ControlReceiptSHA256: inspection.CurrentReceiptSHA256,
		Owner:                owner,
	}
	if err := ValidateBinding(binding); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

func validateBindingOwner(owner laneowner.Snapshot) error {
	if strings.TrimSpace(owner.Lane) == "" || owner.Lane != strings.TrimSpace(owner.Lane) ||
		strings.TrimSpace(owner.CurrentExecutor) == "" ||
		owner.CurrentExecutor != strings.TrimSpace(owner.CurrentExecutor) ||
		owner.ExecutorGeneration <= 0 {
		return fmt.Errorf("execution control binding requires an exact durable lane owner")
	}
	return nil
}
