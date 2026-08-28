package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

// scopedCommandBinding is the runtime-side binding between parsed Options and
// one exact command scope. The descriptor remains owned by commands; this
// package only attaches CLI shape validation and the existing domain handler.
type scopedCommandBinding struct {
	Descriptor commands.ScopedCommandDescriptor
	Options    Options
}

type scopedCommandBinder func(Options, commands.CommandScope) (scopedCommandBinding, error)
type scopedCommandValidator func(runtime.Context, scopedCommandBinding) error
type scopedCommandHandler func(runtime.Context, Options, io.Writer) error
type scopedCommandModeResolver func(Options) (string, error)

type scopedCommandRoute struct {
	Scope    commands.CommandScope
	Bind     scopedCommandBinder
	Validate scopedCommandValidator
	Handle   scopedCommandHandler
}

// scopedCommandRuntimeOwner binds every descriptor mode for one migrated
// command to its only runtime resolver, binder, validator, and handler.
type scopedCommandRuntimeOwner struct {
	Command     string
	ResolveMode scopedCommandModeResolver
	Routes      []scopedCommandRoute
}

type preRuntimeCommandSelector func(Options) bool
type preRuntimeCommandValidator func(Options) error
type preRuntimeCommandHandler func(Options, io.Writer) (bool, error)

type preRuntimeCommandOwner struct {
	Name     string
	Mode     string
	Commands []string
	Scopes   []commands.CommandScope
	Select   preRuntimeCommandSelector
	Validate preRuntimeCommandValidator
	Handle   preRuntimeCommandHandler
}

var scopedCommandRuntimeOwners = []scopedCommandRuntimeOwner{
	fixedScopedCommandRuntimeOwner(commands.Status, commands.MutationModeDefault, runStatus),
	fixedScopedCommandRuntimeOwner(commands.Packs, commands.MutationModeDefault, runPacks),
	fixedScopedCommandRuntimeOwner(commands.Doctor, commands.MutationModeDefault, runDoctor),
	fixedScopedCommandRuntimeOwner(commands.Validate, commands.MutationModeDefault, runDoctor),
	defaultScopedCommandRuntimeOwner(commands.ReleaseCheck, bindReleaseCheckCommand, validateReleaseCheckCommand, handleReleaseCheckCommand),
	defaultScopedCommandRuntimeOwner(commands.MigrateState, bindMigrateStateCommand, validateMigrateStateCommand, handleMigrateStateCommand),
	defaultScopedCommandRuntimeOwner(commands.NextBatch, bindNextBatchCommand, validateNextBatchCommand, handleNextBatchCommand),
	defaultScopedCommandRuntimeOwner(commands.Control, bindControlCommand, validateControlCommand, handleControlCommand),
	defaultScopedCommandRuntimeOwner(commands.Continue, bindContinueCommand, validateContinueCommand, handleContinueCommand),
	{
		Command:     commands.Sync,
		ResolveMode: resolveSyncCommandMode,
		Routes: []scopedCommandRoute{
			ownedScopedCommandRoute(commands.Sync, commands.MutationModeCurrentSync, bindSyncCommand, validateSyncCommand, handleSyncCommand),
			ownedScopedCommandRoute(commands.Sync, commands.MutationModeSelectedPackMemorySync, bindSyncCommand, validateSyncCommand, handleSyncCommand),
			ownedScopedCommandRoute(commands.Sync, commands.MutationModePackMemoryConsumerVerification, bindSyncCommand, validateSyncCommand, handleSyncCommand),
			ownedScopedCommandRoute(commands.Sync, commands.MutationModeOrdinarySync, bindSyncCommand, validateSyncCommand, handleSyncCommand),
		},
	},
	descriptorScopedCommandRuntimeOwner(commands.Update, resolveUpdateCommandMode, bindUpdateCommand, validateUpdateCommand, handleUpdateCommand),
	descriptorScopedCommandRuntimeOwner(commands.Onboard, resolveOnboardCommandMode, bindOnboardCommand, validateOnboardCommand, handleOnboardCommand),
	descriptorScopedCommandRuntimeOwner(commands.PlanSubagents, resolvePlanSubagentsCommandMode, bindPlanSubagentsCommand, validatePlanSubagentsCommand, handlePlanSubagentsCommand),
	descriptorScopedCommandRuntimeOwner(commands.Promote, resolvePromoteCommandMode, bindPromoteCommand, validatePromoteCommand, handlePromoteCommand),
	descriptorScopedCommandRuntimeOwner(commands.RunCurrentLoop, resolveRunCurrentLoopCommandMode, bindRunCurrentLoopCommand, validateRunCurrentLoopCommand, handleRunCurrentLoopCommand),
	fixedScopedCommandRuntimeOwner(commands.ReleaseRun, commands.MutationModeValidationReceipt, runReleaseRun),
	fixedScopedCommandRuntimeOwner(commands.RunCurrentStep, commands.MutationModeDefault, runCurrentStep),
	fixedScopedCommandRuntimeOwner(commands.RunDriverStep, commands.MutationModeDefault, runDriverStep),
	fixedScopedCommandRuntimeOwner(commands.RunReviewerStep, commands.MutationModeDefault, runReviewerStep),
	fixedScopedCommandRuntimeOwner(commands.RunReviewerWave, commands.MutationModeDefault, runReviewerWave),
	fixedScopedCommandRuntimeOwner(commands.Attach, commands.MutationModeDefault, runAttach),
	fixedScopedCommandRuntimeOwner(commands.Repair, commands.MutationModeDefault, runRepair),
	fixedScopedCommandRuntimeOwner(commands.Init, commands.MutationModeDefault, runInitBootstrap),
	fixedScopedCommandRuntimeOwner(commands.Bootstrap, commands.MutationModeDefault, runInitBootstrap),
	fixedScopedCommandRuntimeOwner(commands.Overview, commands.MutationModeBoardBootstrap, runOverview),
	fixedScopedCommandRuntimeOwner(commands.Start, commands.MutationModeDefault, runStart),
	fixedScopedCommandRuntimeOwner(commands.Handoff, commands.MutationModeDefault, runHandoff),
	fixedScopedCommandRuntimeOwner(commands.Complete, commands.MutationModeDefault, runComplete),
	fixedScopedCommandRuntimeOwner(commands.Reopen, commands.MutationModeDefault, runReopen),
	fixedScopedCommandRuntimeOwner(commands.Reconcile, commands.MutationModeDefault, runReconcile),
	descriptorScopedCommandRuntimeOwner(commands.Note, resolveNoteCommandMode, bindScopedCommand, validateNoteCommand, runNote),
	{
		Command:     commands.Gate,
		ResolveMode: resolveGateCommandMode,
		Routes: []scopedCommandRoute{
			gateScopedCommandRoute(commands.MutationModeProfile),
			gateScopedCommandRoute(commands.MutationModeAdapterDispatch),
			gateScopedCommandRoute(commands.MutationModeAdapterReceipt),
			gateScopedCommandRoute(commands.MutationModeReportScaffold),
			gateScopedCommandRoute(commands.MutationModeReportDraft),
			gateScopedCommandRoute(commands.MutationModeDecision),
			gateScopedCommandRoute(commands.MutationModeExecutionObservation),
		},
	},
}

var scopedCommandRoutes = scopedCommandRoutesForOwners(scopedCommandRuntimeOwners)

var preRuntimeCommandOwners = []preRuntimeCommandOwner{
	{
		Name:     "retired-pack-migration",
		Mode:     commands.MutationModeDefault,
		Commands: []string{commands.MigrateState},
		Scopes: []commands.CommandScope{
			{Command: commands.MigrateState, Mode: commands.MutationModeDefault},
		},
		Select: func(opt Options) bool {
			return opt.Command == commands.MigrateState && opt.PackProvided && packidentity.IsRetired(opt.Pack)
		},
		Validate: validateRetiredPackMigrationOptions,
		Handle: func(opt Options, out io.Writer) (bool, error) {
			return true, runRetiredPackMigration(opt, out)
		},
	},
	{
		Name:     "current-sync-maintenance",
		Mode:     commands.MutationModeCurrentSync,
		Commands: commands.Public(),
		Scopes: []commands.CommandScope{
			{Command: commands.Sync, Mode: commands.MutationModeCurrentSync},
		},
		Select:   wantsCurrentSyncMaintenance,
		Validate: validateCurrentSyncMaintenanceOptions,
		Handle: func(opt Options, out io.Writer) (bool, error) {
			return true, runCurrentSyncMaintenance(opt, out)
		},
	},
	{
		Name:     "current-sync-recovery-front-door",
		Mode:     "pending-current-sync-recovery",
		Commands: commands.Public(),
		Select:   func(Options) bool { return true },
		Validate: func(Options) error { return nil },
		Handle:   runCurrentSyncRecoveryFrontDoor,
	},
}

func resolveDefaultCommandMode(Options) (string, error) {
	return commands.MutationModeDefault, nil
}

func resolveGateCommandMode(opt Options) (string, error) {
	switch {
	case opt.Gate.ProvisionProfile || opt.Gate.RevokeProfile || gateProfileOnlyFieldsPresent(opt):
		return commands.MutationModeProfile, nil
	case opt.Gate.RecordAdapterExecutionDispatch:
		return commands.MutationModeAdapterDispatch, nil
	case opt.Gate.RecordAdapterExecutionReceipt:
		return commands.MutationModeAdapterReceipt, nil
	case opt.Gate.ScaffoldExecutionReport || opt.Gate.ExecutionReportContract:
		return commands.MutationModeReportScaffold, nil
	case opt.Gate.DraftExecutionReport:
		return commands.MutationModeReportDraft, nil
	case opt.Gate.ValidateExecutionReport || wantsGateExecutionEvidence(opt.Gate):
		return commands.MutationModeExecutionObservation, nil
	default:
		return commands.MutationModeDecision, nil
	}
}

func resolveSyncCommandMode(opt Options) (string, error) {
	switch {
	case wantsCurrentSyncMaintenance(opt):
		return commands.MutationModeCurrentSync, nil
	case opt.VerifyPackMemoryConsumerUse:
		return commands.MutationModePackMemoryConsumerVerification, nil
	case strings.TrimSpace(opt.SelectPackMemoryChange) != "":
		return commands.MutationModeSelectedPackMemorySync, nil
	default:
		return commands.MutationModeOrdinarySync, nil
	}
}

func resolveUpdateCommandMode(Options) (string, error) {
	return commands.MutationModeOrdinarySync, nil
}

func resolveOnboardCommandMode(opt Options) (string, error) {
	if strings.TrimSpace(opt.Target) == "" {
		return commands.MutationModeOrdinaryOnboarding, nil
	}
	attached, err := instance.CheckCase(opt.Target)
	if err != nil {
		return "", err
	}
	if attached {
		return commands.MutationModeAttachedAdoption, nil
	}
	return commands.MutationModeOrdinaryOnboarding, nil
}

func resolvePlanSubagentsCommandMode(opt Options) (string, error) {
	switch {
	case opt.RecordReviewerDispatch:
		return commands.MutationModeRecordReviewerDispatch, nil
	case opt.RecordReviewerCompletion:
		return commands.MutationModeRecordReviewerCompletion, nil
	case opt.SaveReviewerResultInput:
		return commands.MutationModeSaveReviewerResultInput, nil
	case opt.CaptureReviewerResultSource:
		return commands.MutationModeCaptureReviewerResultSource, nil
	case opt.StageReviewerResult:
		return commands.MutationModeStageReviewerResult, nil
	case opt.CollectReviewerResult:
		return commands.MutationModeCollectReviewerResult, nil
	case opt.RetireReviewerResultRecovery:
		return commands.MutationModeRetireReviewerRecovery, nil
	case opt.RetireInvalidReviewerPacket:
		return commands.MutationModeRetireReviewerPacket, nil
	case opt.RecoverReviewerResult:
		return commands.MutationModeRecoverReviewerResult, nil
	case opt.RepairReviewerPromptArtifact:
		return commands.MutationModeRepairReviewerPrompt, nil
	case opt.AdoptReviewerPacket:
		return commands.MutationModeAdoptReviewerPacket, nil
	case opt.ReadyReviewerResults:
		return commands.MutationModeReviewerBatchIntake, nil
	case strings.TrimSpace(opt.ReviewerResultPath) != "":
		return commands.MutationModeReviewerIntake, nil
	default:
		return commands.MutationModePlanArtifacts, nil
	}
}

func resolvePromoteCommandMode(opt Options) (string, error) {
	switch {
	case opt.StageMemberOutput:
		return commands.MutationModeMemberOutputStaging, nil
	case opt.RetireCandidateVerificationWorkspace:
		return commands.MutationModeCandidateRetirement, nil
	case opt.ProvisionCandidateVerificationCases:
		return commands.MutationModeCandidateProvision, nil
	case opt.VerifyCandidateDecision:
		return commands.MutationModeCandidateVerification, nil
	case opt.DraftReviewProof:
		return commands.MutationModeReviewProofDraft, nil
	case opt.DraftCandidateDecision:
		return commands.MutationModeCandidateDecisionDraft, nil
	case strings.TrimSpace(opt.CandidateDecisionPath) != "":
		return commands.MutationModeCandidateDecisionApply, nil
	case opt.CreateCandidates:
		return commands.MutationModeCreateCandidates, nil
	default:
		return commands.MutationModeOrdinaryPromote, nil
	}
}

func resolveRunCurrentLoopCommandMode(opt Options) (string, error) {
	switch {
	case opt.RecordExternalSessionAttempt:
		return commands.MutationModeExternalAttempt, nil
	case opt.ClaimExternalSessionDispatch || opt.RecordExternalSessionLaunch:
		return commands.MutationModeExternalDispatch, nil
	case opt.AdvanceExternalSessionResult:
		return commands.MutationModeExternalTurn, nil
	case opt.RelayExternalSessionSubmission:
		return commands.MutationModeExternalRelay, nil
	default:
		return commands.MutationModeDefault, nil
	}
}

func resolveNoteCommandMode(opt Options) (string, error) {
	if strings.TrimSpace(opt.Note.ExpectedEventSHA256) != "" || opt.Note.ExpectedExecutionControlBinding != nil {
		return commands.MutationModeEventAppend, nil
	}
	return commands.MutationModeDirectAppend, nil
}

func scopedCommandRuntimeOwnerFor(command string) (scopedCommandRuntimeOwner, bool) {
	for _, owner := range scopedCommandRuntimeOwners {
		if owner.Command == command {
			return owner, true
		}
	}
	return scopedCommandRuntimeOwner{}, false
}

func scopedCommandRouteForOwner(owner scopedCommandRuntimeOwner, scope commands.CommandScope) (scopedCommandRoute, bool) {
	for _, route := range owner.Routes {
		if route.Scope == scope {
			return route, true
		}
	}
	return scopedCommandRoute{}, false
}

func scopedCommandRoutesForOwners(owners []scopedCommandRuntimeOwner) []scopedCommandRoute {
	routes := []scopedCommandRoute{}
	for _, owner := range owners {
		routes = append(routes, owner.Routes...)
	}
	return routes
}

func defaultScopedCommandRuntimeOwner(command string, binder scopedCommandBinder, validator scopedCommandValidator, handler scopedCommandHandler) scopedCommandRuntimeOwner {
	return scopedCommandRuntimeOwner{
		Command:     command,
		ResolveMode: resolveDefaultCommandMode,
		Routes: []scopedCommandRoute{{
			Scope:    commands.CommandScope{Command: command, Mode: commands.MutationModeDefault},
			Bind:     binder,
			Validate: validator,
			Handle:   handler,
		}},
	}
}

func fixedScopedCommandRuntimeOwner(command, mode string, handler scopedCommandHandler) scopedCommandRuntimeOwner {
	resolver := func(Options) (string, error) { return mode, nil }
	return scopedCommandRuntimeOwner{
		Command:     command,
		ResolveMode: resolver,
		Routes: []scopedCommandRoute{
			ownedScopedCommandRoute(command, mode, bindScopedCommand, validateFixedScopedCommand, handler),
		},
	}
}

func descriptorScopedCommandRuntimeOwner(command string, resolver scopedCommandModeResolver, binder scopedCommandBinder, validator scopedCommandValidator, handler scopedCommandHandler) scopedCommandRuntimeOwner {
	routes := []scopedCommandRoute{}
	for _, descriptor := range commands.ScopedCommandDescriptorsFor(command) {
		routes = append(routes, ownedScopedCommandRoute(command, descriptor.Scope.Mode, binder, validator, handler))
	}
	return scopedCommandRuntimeOwner{Command: command, ResolveMode: resolver, Routes: routes}
}

func ownedScopedCommandRoute(command, mode string, binder scopedCommandBinder, validator scopedCommandValidator, handler scopedCommandHandler) scopedCommandRoute {
	return scopedCommandRoute{
		Scope:    commands.CommandScope{Command: command, Mode: mode},
		Bind:     binder,
		Validate: validator,
		Handle:   handler,
	}
}

func gateScopedCommandRoute(mode string) scopedCommandRoute {
	return scopedCommandRoute{
		Scope:    commands.CommandScope{Command: commands.Gate, Mode: mode},
		Bind:     bindGateCommand,
		Validate: validateGateCommand,
		Handle:   handleGateCommand,
	}
}

func runPreRuntimeCommand(opt Options, out io.Writer) (bool, error) {
	for _, owner := range preRuntimeCommandOwners {
		if !slices.Contains(owner.Commands, opt.Command) || !owner.Select(opt) {
			continue
		}
		if err := owner.Validate(opt); err != nil {
			return true, err
		}
		handled, err := owner.Handle(opt, out)
		if handled || err != nil {
			return handled, err
		}
	}
	return false, nil
}

func runOwnedCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	if handled, err := runScopedCommand(ctx, opt, out); handled || err != nil {
		return err
	}
	return commands.UnsupportedError(opt.Command)
}

func runScopedCommand(ctx runtime.Context, opt Options, out io.Writer) (bool, error) {
	owner, ok := scopedCommandRuntimeOwnerFor(opt.Command)
	if !ok {
		return false, nil
	}
	mode, err := owner.ResolveMode(opt)
	if err != nil {
		return true, err
	}
	scope := commands.CommandScope{Command: owner.Command, Mode: mode}
	route, ok := scopedCommandRouteForOwner(owner, scope)
	if !ok {
		return true, fmt.Errorf("scoped command runtime owner has no route for %s mode %s", scope.Command, scope.Mode)
	}
	return true, executeScopedCommandRoute(ctx, opt, out, route)
}

func executeScopedCommandRoute(ctx runtime.Context, opt Options, out io.Writer, route scopedCommandRoute) error {
	if err := validateScopedCommandRouteCatalog(); err != nil {
		return err
	}
	binding, err := route.Bind(opt, route.Scope)
	if err != nil {
		return err
	}
	if err := validateScopedCommandPolicy(binding); err != nil {
		return err
	}
	if err := route.Validate(ctx, binding); err != nil {
		return err
	}
	return route.Handle(ctx, binding.Options, out)
}

func scopedCommandRouteFor(scope commands.CommandScope) (scopedCommandRoute, bool) {
	if scope.Mode == "" {
		scope.Mode = commands.MutationModeDefault
	}
	for _, route := range scopedCommandRoutes {
		if route.Scope == scope {
			return route, true
		}
	}
	return scopedCommandRoute{}, false
}

func bindScopedCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	if opt.Command != scope.Command {
		return scopedCommandBinding{}, fmt.Errorf("scoped command binder received %q for %s", opt.Command, scope.Command)
	}
	descriptor, ok := commands.ScopedCommandDescriptorFor(scope.Command, scope.Mode)
	if !ok {
		return scopedCommandBinding{}, fmt.Errorf("scoped command descriptor is unavailable for %s mode %s", scope.Command, scope.Mode)
	}
	return scopedCommandBinding{Descriptor: descriptor, Options: opt}, nil
}

func bindReleaseCheckCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	binding, err := bindScopedCommand(opt, scope)
	if err != nil {
		return scopedCommandBinding{}, err
	}
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	binding.Options.Format = format
	return binding, nil
}

func bindMigrateStateCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "migrate-state")
}

func bindNextBatchCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "next-batch")
}

func bindControlCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "control")
}

func bindContinueCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "continue")
}

func bindGateCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "gate")
}

func bindSyncCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "sync")
}

func bindUpdateCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "update")
}

func bindOnboardCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "onboard")
}

func bindPlanSubagentsCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	return bindWorkstreamScopedCommand(opt, scope, "plan-subagents")
}

func bindPromoteCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	if scope.Mode == commands.MutationModeCreateCandidates {
		binding, err := bindScopedCommand(opt, scope)
		if err != nil {
			return scopedCommandBinding{}, err
		}
		format, err := promoteCandidatesFormat(opt.Format)
		if err != nil {
			return scopedCommandBinding{}, fmt.Errorf("unsupported promote create-candidates format: %s", opt.Format)
		}
		binding.Options.Format = format
		return binding, nil
	}
	return bindWorkstreamScopedCommand(opt, scope, "promote")
}

func bindRunCurrentLoopCommand(opt Options, scope commands.CommandScope) (scopedCommandBinding, error) {
	binding, err := bindScopedCommand(opt, scope)
	if err != nil {
		return scopedCommandBinding{}, err
	}
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "json"
	}
	binding.Options.Format = format
	return binding, nil
}

func bindWorkstreamScopedCommand(opt Options, scope commands.CommandScope, label string) (scopedCommandBinding, error) {
	binding, err := bindScopedCommand(opt, scope)
	if err != nil {
		return scopedCommandBinding{}, err
	}
	format, err := workstreamFormat(opt.Format)
	if err != nil {
		return scopedCommandBinding{}, fmt.Errorf("unsupported %s format: %s", label, opt.Format)
	}
	binding.Options.Format = format
	return binding, nil
}

// validateScopedCommandPolicy is shared by all migrated routes. Command-specific
// phase and shape checks remain attached to the exact scope below.
func validateScopedCommandPolicy(binding scopedCommandBinding) error {
	descriptor := binding.Descriptor
	if descriptor.Scope.Command != binding.Options.Command ||
		descriptor.Scope.Mode == "" || descriptor.Profile.Command != descriptor.Scope.Command {
		return fmt.Errorf("scoped command descriptor identity is invalid for %s", descriptor.Scope.Command)
	}
	if descriptor.Profile.HeavyTool || descriptor.Profile.AuthorityConfirmed {
		return fmt.Errorf("scoped command %s exceeds the no-heavy-tool/no-authority boundary", descriptor.Scope.Command)
	}
	if descriptor.Profile.IsMutation != (descriptor.Mutation != nil) {
		return fmt.Errorf("scoped command %s mode %s has inconsistent mutation policy", descriptor.Scope.Command, descriptor.Scope.Mode)
	}
	if descriptor.Mutation != nil && !descriptor.Mutation.Confirmed {
		return fmt.Errorf("scoped command %s mode %s has an unconfirmed mutation contract", descriptor.Scope.Command, descriptor.Scope.Mode)
	}
	return nil
}

func validateReleaseCheckCommand(ctx runtime.Context, binding scopedCommandBinding) error {
	opt := binding.Options
	if ctx.TargetProvided {
		return fmt.Errorf("release-check runs against the kit repo; omit -Target")
	}
	if opt.Apply || opt.WhatIf || opt.CreateCandidates || opt.Review || opt.Force || opt.List || wantsReviewArtifacts(opt) {
		return fmt.Errorf("release-check is read-only and does not accept mutation, review artifact, or list flags")
	}
	if !slices.Contains([]string{"table", "tsv", "text", "json"}, opt.Format) {
		return fmt.Errorf("unsupported release-check format: %s", opt.Format)
	}
	return nil
}

func validateMigrateStateCommand(ctx runtime.Context, binding scopedCommandBinding) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("migrate-state requires an explicit -Target project directory")
	}
	return validateMigrateStateBinding(binding)
}

func validateRetiredPackMigrationOptions(opt Options) error {
	if !opt.targetProvided || strings.TrimSpace(opt.Target) == "" {
		return fmt.Errorf("migrate-state requires an explicit -Target project directory")
	}
	owner, ok := scopedCommandRuntimeOwnerFor(commands.MigrateState)
	if !ok {
		return fmt.Errorf("migrate-state scoped runtime owner is unavailable")
	}
	scope := commands.CommandScope{Command: commands.MigrateState, Mode: commands.MutationModeDefault}
	route, ok := scopedCommandRouteForOwner(owner, scope)
	if !ok {
		return fmt.Errorf("migrate-state scoped runtime route is unavailable")
	}
	binding, err := route.Bind(opt, scope)
	if err != nil {
		return err
	}
	if err := validateScopedCommandPolicy(binding); err != nil {
		return err
	}
	return validateMigrateStateBinding(binding)
}

func validateMigrateStateBinding(binding scopedCommandBinding) error {
	opt := binding.Options
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("migrate-state -WhatIf cannot be combined with -Apply")
	}
	if opt.CreateCandidates || opt.Force || opt.Review || opt.List || wantsReviewArtifacts(opt) {
		return fmt.Errorf("migrate-state supports only zero-write preview or exact hash-bound -Apply")
	}
	contract := binding.Descriptor.Mutation
	if contract == nil || contract.Currentness != commands.MutationCurrentnessStrictPlan {
		return fmt.Errorf("migrate-state scoped descriptor is not strict-plan")
	}
	_, err := plancontract.ValidatePhase(
		binding.Descriptor.Scope.Command,
		contract.ExpectedFlag,
		!opt.Apply,
		opt.Apply,
		opt.ExpectedMigrationPlanSHA256,
	)
	return err
}

func validateNextBatchCommand(ctx runtime.Context, binding scopedCommandBinding) error {
	opt := binding.Options
	if ctx.TargetProvided {
		return fmt.Errorf("next-batch writes kit repo docs; omit -Target")
	}
	if opt.Apply == opt.WhatIf {
		return fmt.Errorf("next-batch requires exactly one of -WhatIf or -Apply")
	}
	if opt.CreateCandidates || opt.Review || opt.Force || opt.List || wantsReviewArtifacts(opt) {
		return fmt.Errorf("next-batch accepts only planning receipt flags; omit create/review/force/list/review artifact flags")
	}
	if opt.WhatIf && strings.TrimSpace(opt.ExpectedNextBatchPlanSHA256) != "" {
		return fmt.Errorf("next-batch -WhatIf does not accept -ExpectedNextBatchPlanSha256")
	}
	if opt.Apply && strings.TrimSpace(opt.ExpectedNextBatchPlanSHA256) == "" {
		return fmt.Errorf("next-batch -Apply requires -ExpectedNextBatchPlanSha256 from -WhatIf")
	}
	return nil
}

func validateControlCommand(_ runtime.Context, binding scopedCommandBinding) error {
	opt := binding.Options
	if opt.CreateCandidates || opt.Review || opt.Force || opt.List || wantsReviewArtifacts(opt) {
		return fmt.Errorf("control supports only review-first WhatIf or exact hash-bound Apply")
	}
	if opt.WhatIf == opt.Apply {
		return fmt.Errorf("control requires exactly one of -WhatIf or -Apply")
	}
	if opt.Format != "json" && opt.Format != "text" {
		return fmt.Errorf("control supports only -Format json or text")
	}
	if opt.WhatIf && strings.TrimSpace(opt.Control.PublicationStamp) != "" {
		return fmt.Errorf("control preview creates its own publication stamp")
	}
	return nil
}

func validateContinueCommand(_ runtime.Context, binding scopedCommandBinding) error {
	opt := binding.Options
	if opt.CreateCandidates {
		return fmt.Errorf("continue does not support -CreateCandidates")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("continue cannot combine -WhatIf and -Apply")
	}
	if wantsReviewArtifacts(opt) {
		return fmt.Errorf("continue does not support review artifact options")
	}
	if !opt.WhatIf && !opt.Apply && opt.Format == "json" {
		return fmt.Errorf("go backend continue requires -WhatIf or -Apply")
	}
	contract := binding.Descriptor.Mutation
	if contract == nil || contract.Currentness != commands.MutationCurrentnessStrictPlan ||
		contract.ExpectedFlag != "-ExpectedContinuePlanSha256" || !contract.ExecutablePlanValidation {
		return fmt.Errorf("continue scoped descriptor is not the executable strict-plan owner")
	}
	return nil
}

func validateFixedScopedCommand(_ runtime.Context, _ scopedCommandBinding) error {
	return nil
}

func validateNoteCommand(_ runtime.Context, binding scopedCommandBinding) error {
	return validateResolvedScopedCommandMode(binding, resolveNoteCommandMode)
}

func validateGateCommand(_ runtime.Context, binding scopedCommandBinding) error {
	return validateResolvedScopedCommandMode(binding, resolveGateCommandMode)
}

func validateSyncCommand(_ runtime.Context, binding scopedCommandBinding) error {
	return validateResolvedScopedCommandMode(binding, resolveSyncCommandMode)
}

func validateUpdateCommand(_ runtime.Context, binding scopedCommandBinding) error {
	return validateResolvedScopedCommandMode(binding, resolveUpdateCommandMode)
}

func validateOnboardCommand(_ runtime.Context, binding scopedCommandBinding) error {
	return validateResolvedScopedCommandMode(binding, resolveOnboardCommandMode)
}

func validatePlanSubagentsCommand(_ runtime.Context, binding scopedCommandBinding) error {
	return validateResolvedScopedCommandMode(binding, resolvePlanSubagentsCommandMode)
}

func validatePromoteCommand(_ runtime.Context, binding scopedCommandBinding) error {
	return validateResolvedScopedCommandMode(binding, resolvePromoteCommandMode)
}

func validateRunCurrentLoopCommand(_ runtime.Context, binding scopedCommandBinding) error {
	return validateResolvedScopedCommandMode(binding, resolveRunCurrentLoopCommandMode)
}

func validateResolvedScopedCommandMode(binding scopedCommandBinding, resolver scopedCommandModeResolver) error {
	resolved, err := resolver(binding.Options)
	if err != nil {
		return err
	}
	if resolved != binding.Descriptor.Scope.Mode {
		return fmt.Errorf("%s mode drifted after scoped binding: resolved=%s bound=%s", binding.Options.Command, resolved, binding.Descriptor.Scope.Mode)
	}
	return nil
}

func handleReleaseCheckCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runReleaseCheck(ctx, opt, out)
}

func handleMigrateStateCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runMigrateState(ctx, opt, out)
}

func handleNextBatchCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runNextBatch(ctx, opt, out)
}

func handleControlCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return executeControl(ctx, opt, out)
}

func handleContinueCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return executeContinue(ctx, opt, out)
}

func handleGateCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return executeGate(ctx, opt, out)
}

func handleSyncCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runSyncReview(ctx, opt, out)
}

func handleUpdateCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runSyncReview(ctx, opt, out)
}

func handleOnboardCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runOnboard(ctx, opt, out)
}

func handlePlanSubagentsCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runPlanSubagents(ctx, opt, out)
}

func handlePromoteCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runPromoteReview(ctx, opt, out)
}

func handleRunCurrentLoopCommand(ctx runtime.Context, opt Options, out io.Writer) error {
	return runCurrentLoop(ctx, opt, out)
}

func validateScopedCommandRouteCatalog() error {
	if err := validateScopedCommandRuntimeOwners(scopedCommandRuntimeOwners); err != nil {
		return err
	}
	if err := validatePreRuntimeCommandOwners(preRuntimeCommandOwners); err != nil {
		return err
	}
	if err := validateScopedPreRuntimeOwnership(scopedCommandRuntimeOwners, preRuntimeCommandOwners); err != nil {
		return err
	}
	return validateScopedCommandRoutes(scopedCommandRoutes)
}

func validateScopedCommandRuntimeOwners(owners []scopedCommandRuntimeOwner) error {
	seenCommands := map[string]bool{}
	for _, owner := range owners {
		if owner.Command == "" || owner.Command != strings.ToLower(strings.TrimSpace(owner.Command)) || !commands.IsPublic(owner.Command) || owner.ResolveMode == nil {
			return fmt.Errorf("scoped command runtime owner is invalid: %q", owner.Command)
		}
		if seenCommands[owner.Command] {
			return fmt.Errorf("duplicate scoped command runtime owner: %s", owner.Command)
		}
		seenCommands[owner.Command] = true
		descriptors := commands.ScopedCommandDescriptorsFor(owner.Command)
		if len(descriptors) == 0 || len(owner.Routes) != len(descriptors) {
			return fmt.Errorf("scoped command runtime owner route coverage drifted for %s: routes=%d descriptors=%d", owner.Command, len(owner.Routes), len(descriptors))
		}
		seenScopes := map[commands.CommandScope]bool{}
		for _, route := range owner.Routes {
			if route.Scope.Command != owner.Command {
				return fmt.Errorf("scoped command runtime owner %s contains foreign route %s mode %s", owner.Command, route.Scope.Command, route.Scope.Mode)
			}
			seenScopes[route.Scope] = true
		}
		for _, descriptor := range descriptors {
			if !seenScopes[descriptor.Scope] {
				return fmt.Errorf("scoped command runtime owner %s omits descriptor mode %s", owner.Command, descriptor.Scope.Mode)
			}
		}
	}
	for _, command := range commands.Public() {
		if !seenCommands[command] {
			return fmt.Errorf("public command has no scoped runtime callback owner: %s", command)
		}
	}
	return nil
}

func validatePreRuntimeCommandOwners(owners []preRuntimeCommandOwner) error {
	seenNames := map[string]bool{}
	seenScopes := map[commands.CommandScope]string{}
	for _, owner := range owners {
		if strings.TrimSpace(owner.Name) == "" || strings.TrimSpace(owner.Mode) == "" || owner.Select == nil || owner.Validate == nil || owner.Handle == nil || len(owner.Commands) == 0 {
			return fmt.Errorf("pre-runtime command owner is invalid: %q", owner.Name)
		}
		if seenNames[owner.Name] {
			return fmt.Errorf("duplicate pre-runtime command owner: %s", owner.Name)
		}
		seenNames[owner.Name] = true
		seenCommands := map[string]bool{}
		for _, command := range owner.Commands {
			if !commands.IsPublic(command) || seenCommands[command] {
				return fmt.Errorf("pre-runtime command owner %s has invalid command %q", owner.Name, command)
			}
			seenCommands[command] = true
		}
		for _, scope := range owner.Scopes {
			if !seenCommands[scope.Command] || scope.Mode == "" {
				return fmt.Errorf("pre-runtime command owner %s has invalid scope %s mode %s", owner.Name, scope.Command, scope.Mode)
			}
			if previous := seenScopes[scope]; previous != "" {
				return fmt.Errorf("pre-runtime command scope %s mode %s is owned by both %s and %s", scope.Command, scope.Mode, previous, owner.Name)
			}
			seenScopes[scope] = owner.Name
		}
	}
	return nil
}

func validateScopedPreRuntimeOwnership(scoped []scopedCommandRuntimeOwner, preRuntime []preRuntimeCommandOwner) error {
	for _, preRuntimeOwner := range preRuntime {
		for _, scope := range preRuntimeOwner.Scopes {
			found := false
			for _, scopedOwner := range scoped {
				if scopedOwner.Command != scope.Command {
					continue
				}
				if _, ok := scopedCommandRouteForOwner(scopedOwner, scope); ok {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("pre-runtime command owner %s scope %s mode %s has no descriptor-backed fallback route", preRuntimeOwner.Name, scope.Command, scope.Mode)
			}
		}
	}
	return nil
}

func validateScopedCommandRoutes(routes []scopedCommandRoute) error {
	descriptors, err := commands.ScopedCommandDescriptors()
	if err != nil {
		return err
	}
	if err := commands.ValidateScopedCommandDescriptors(descriptors); err != nil {
		return err
	}
	seen := map[commands.CommandScope]bool{}
	for _, route := range routes {
		if route.Bind == nil || route.Validate == nil || route.Handle == nil {
			return fmt.Errorf("scoped command route is missing binder, validator, or handler: %s mode %s", route.Scope.Command, route.Scope.Mode)
		}
		if seen[route.Scope] {
			return fmt.Errorf("duplicate scoped command route: %s mode %s", route.Scope.Command, route.Scope.Mode)
		}
		seen[route.Scope] = true
		if _, ok := commands.ScopedCommandDescriptorFor(route.Scope.Command, route.Scope.Mode); !ok {
			return fmt.Errorf("scoped command route has no descriptor: %s mode %s", route.Scope.Command, route.Scope.Mode)
		}
	}
	return nil
}
