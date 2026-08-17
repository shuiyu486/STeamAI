package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func inspectCurrentSyncRecoveryForTarget(target string) (
	syncreview.CurrentSyncRecovery,
	bool,
	error,
) {
	root, err := projectstate.Resolve(target)
	if err != nil {
		return syncreview.CurrentSyncRecovery{}, false, err
	}
	if !root.Existing || root.Legacy || root.Dir != projectstate.CurrentDir {
		return syncreview.CurrentSyncRecovery{}, false, nil
	}
	recovery, err := syncreview.InspectCurrentSyncRecovery(target)
	if err != nil {
		return syncreview.CurrentSyncRecovery{}, true, fmt.Errorf(
			"inspect current project update recovery: %w",
			err,
		)
	}
	return recovery, true, nil
}

// RunRecovery dispatches only the pre-runtime current-sync recovery surface.
// It never falls through to ordinary runtime command handling.
func RunRecovery(args []string, out io.Writer) error {
	return runRecovery(args, out, "")
}

// RunProjectLocalRecovery permits an omitted public target only after the
// process entrypoint has bound the running executable to its owner project.
func RunProjectLocalRecovery(args []string, out io.Writer, projectRoot string) error {
	return runRecovery(args, out, projectRoot)
}

func runRecovery(args []string, out io.Writer, projectRoot string) error {
	opt, err := Parse(args)
	if err != nil {
		return err
	}
	if err := validateCurrentSyncMaintenanceOptions(opt); err != nil {
		return err
	}
	if wantsCurrentSyncMaintenance(opt) {
		return fmt.Errorf(
			"project-local recovery cannot execute current project maintenance Apply; use the exact external maintenance executable",
		)
	}
	if strings.TrimSpace(projectRoot) != "" {
		target, err := runtime.ResolveProjectLocalTarget(projectRoot, opt.Target, "")
		if err != nil {
			return err
		}
		opt.Target = target
	}
	handled, err := runCurrentSyncRecoveryFrontDoor(opt, out)
	if err != nil {
		return err
	}
	if !handled {
		return fmt.Errorf(
			"project-local recovery front door requires a pending durable current project update",
		)
	}
	return nil
}

func runCurrentSyncRecoveryFrontDoor(opt Options, out io.Writer) (bool, error) {
	target := strings.TrimSpace(opt.Target)
	if target == "" {
		return false, nil
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	recovery, current, err := inspectCurrentSyncRecoveryForTarget(target)
	if err != nil {
		return current, err
	}
	if !current || !recovery.Pending {
		return false, nil
	}
	switch opt.Command {
	case commands.Status:
		return true, writeCurrentSyncRecoveryStatus(out, opt, target, recovery)
	case commands.Doctor, commands.Validate:
		return true, writeCurrentSyncRecoveryDoctor(out, opt, target, recovery)
	default:
		return true, fmt.Errorf(
			"%s; %s",
			recovery.Now,
			recovery.Next,
		)
	}
}

type currentSyncRecoveryStatus struct {
	Command             string                         `json:"command"`
	SchemaVersion       int                            `json:"schemaVersion"`
	IsMutation          bool                           `json:"isMutation"`
	Target              string                         `json:"target"`
	TargetProvided      bool                           `json:"targetProvided"`
	Pack                string                         `json:"pack"`
	Mode                string                         `json:"mode"`
	CurrentSyncRecovery syncreview.CurrentSyncRecovery `json:"currentSyncRecovery"`
}

func writeCurrentSyncRecoveryStatus(
	out io.Writer,
	opt Options,
	target string,
	recovery syncreview.CurrentSyncRecovery,
) error {
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	result := currentSyncRecoveryStatus{
		Command: "status", SchemaVersion: 1, IsMutation: false,
		Target: target, TargetProvided: opt.targetProvided, Pack: recovery.Pack,
		Mode: "case-maintenance-recovery", CurrentSyncRecovery: recovery,
	}
	switch format {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "compact-json":
		compact := struct {
			Command             string                           `json:"command"`
			SchemaVersion       int                              `json:"schemaVersion"`
			IsMutation          bool                             `json:"isMutation"`
			Target              string                           `json:"target"`
			Mode                string                           `json:"mode"`
			CurrentSyncRecovery statusCompactCurrentSyncRecovery `json:"currentSyncRecovery"`
		}{
			Command:       result.Command,
			SchemaVersion: result.SchemaVersion,
			IsMutation:    result.IsMutation,
			Target:        result.Target,
			Mode:          result.Mode,
			CurrentSyncRecovery: statusCompactCurrentSyncRecovery{
				State:       recovery.State,
				Pending:     recovery.Pending,
				Blocked:     recovery.Blocked,
				Recoverable: recovery.Recoverable,
				Now:         recovery.Now,
				Reason:      recovery.Reason,
				Next:        recovery.Next,
			},
		}
		data, err := marshalStatusCompactValue(compact)
		if err != nil {
			return err
		}
		if len(data) > statusCompactJSONMaxBytes {
			data, err = marshalStatusCompactBlockedJSON(statusInventory{
				Command:       result.Command,
				SchemaVersion: result.SchemaVersion,
				Target:        result.Target,
			}, statusCompactReasonBudgetExceeded)
			if err != nil {
				return err
			}
		}
		_, err = out.Write(data)
		return err
	case "table", "tsv", "text":
		_, err := fmt.Fprintf(out, "现在：%s\n原因：%s\n下一步：%s\n", recovery.Now, recovery.Reason, recovery.Next)
		return err
	default:
		return fmt.Errorf("unsupported status format: %s", opt.Format)
	}
}

func writeCurrentSyncRecoveryDoctor(
	out io.Writer,
	opt Options,
	target string,
	recovery syncreview.CurrentSyncRecovery,
) error {
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "table"
	}
	result := doctorInventory{
		Command: opt.Command, SchemaVersion: 1, IsMutation: false,
		Pack: recovery.Pack, Target: target, Mode: "case-maintenance-recovery",
		Valid: false, Summary: recovery.Now, CurrentSyncRecovery: &recovery,
	}
	switch format {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "text":
		return writeDoctorText(out, result)
	case "table", "tsv":
		_, err := fmt.Fprintf(out, "%s\n%s\n", recovery.Now, recovery.Next)
		return err
	default:
		return fmt.Errorf("unsupported %s format: %s", opt.Command, opt.Format)
	}
}
