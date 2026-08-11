package sessionhost

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

type dailyTargetKind string

const (
	dailyTargetMissing  dailyTargetKind = "missing"
	dailyTargetOrdinary dailyTargetKind = "ordinary-directory"
	dailyTargetAttached dailyTargetKind = "attached-case"
	dailyTargetMission  dailyTargetKind = "mission-case"
	dailyTargetInvalid  dailyTargetKind = "invalid"
)

type dailyTarget struct {
	Root string
	Kind dailyTargetKind
}

func classifyDailyTarget(value string) (dailyTarget, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return dailyTarget{Kind: dailyTargetInvalid}, fmt.Errorf("daily front door requires -target <case root>")
	}
	root, err := filepath.Abs(value)
	if err != nil {
		return dailyTarget{Kind: dailyTargetInvalid}, err
	}
	root = filepath.Clean(root)
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		if err := refsf.ValidateNoReparseComponents(filepath.Dir(root)); err != nil {
			return dailyTarget{Root: root, Kind: dailyTargetInvalid}, err
		}
		return dailyTarget{Root: root, Kind: dailyTargetMissing}, nil
	} else if err != nil {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, err
	}
	if _, err := refsf.ValidateNonReparseDirectory(root, "daily target"); err != nil {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, err
	}

	inst, err := instance.Read(root)
	if err != nil {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, err
	}
	if inst.Source == "missing" {
		if err := refsf.ValidateTreeNoReparse(root, "daily ordinary target"); err != nil {
			return dailyTarget{Root: root, Kind: dailyTargetInvalid}, err
		}
		if info, statErr := os.Lstat(filepath.Join(root, ".rekit")); statErr == nil {
			return dailyTarget{Root: root, Kind: dailyTargetInvalid}, fmt.Errorf("daily target contains partial .rekit state: %s", info.Name())
		} else if !os.IsNotExist(statErr) {
			return dailyTarget{Root: root, Kind: dailyTargetInvalid}, statErr
		}
		if _, statErr := os.Lstat(filepath.Join(root, ".claude", "skills", "rekit", "SKILL.md")); statErr == nil {
			return dailyTarget{Root: root, Kind: dailyTargetInvalid}, fmt.Errorf("daily target contains a partial case-local rekit shim")
		} else if !os.IsNotExist(statErr) {
			return dailyTarget{Root: root, Kind: dailyTargetInvalid}, statErr
		}
		return dailyTarget{Root: root, Kind: dailyTargetOrdinary}, nil
	}
	if inst.Moved() {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, instance.MovedRepairPreviewError(root, inst.TemplatePack)
	}
	if strings.TrimSpace(inst.TemplateRoot) == "" {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, fmt.Errorf("daily target is attached to a missing templateRoot")
	}
	ctx, err := rekitruntime.New(root, inst.TemplatePack)
	if err != nil {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, fmt.Errorf("daily target binding is invalid: %w", err)
	}
	if !casebind.SameExistingPath(inst.TemplateRoot, ctx.RepoRoot) {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, fmt.Errorf("daily target is attached to a different templateRoot or it is missing: %s", inst.TemplateRoot)
	}
	m, err := manifest.Load(ctx.RepoRoot, inst.TemplatePack)
	if err != nil {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, fmt.Errorf("daily target templatePack is unavailable: %w", err)
	}
	if err := m.ValidateSchema(); err != nil {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, fmt.Errorf("daily target templatePack is invalid: %w", err)
	}
	inspection, err := missionintent.Inspect(root)
	if err != nil {
		return dailyTarget{Root: root, Kind: dailyTargetInvalid}, err
	}
	if inspection.State == "absent" {
		return dailyTarget{Root: root, Kind: dailyTargetAttached}, nil
	}
	return dailyTarget{Root: root, Kind: dailyTargetMission}, nil
}
