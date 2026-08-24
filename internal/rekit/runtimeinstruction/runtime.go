package runtimeinstruction

import (
	"fmt"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/productioninstruction"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func Build(caseRoot, pack string) (instructionpacket.Packet, bool, error) {
	contract, production := productioninstruction.ContractFor(pack)
	if !production {
		return instructionpacket.Packet{}, false, nil
	}
	ctx, err := resolve(caseRoot, pack)
	if err != nil {
		return instructionpacket.Packet{}, true, fmt.Errorf("resolve production instruction runtime: %w", err)
	}
	if !strings.EqualFold(ctx.Pack, contract.Pack) {
		return instructionpacket.Packet{}, true, fmt.Errorf("resolved production instruction pack drifted: runtime=%s contract=%s", ctx.Pack, contract.Pack)
	}
	m, err := manifest.Load(ctx.RepoRoot, ctx.Pack)
	if err != nil {
		return instructionpacket.Packet{}, true, fmt.Errorf("load resolved production instruction manifest: %w", err)
	}
	packet, err := productioninstruction.BuildPacket(ctx.RepoRoot, ctx.Pack, m)
	if err != nil {
		return instructionpacket.Packet{}, true, err
	}
	return packet, true, nil
}

func Reload(caseRoot, pack string, identity instructionpacket.Identity) (instructionpacket.Packet, error) {
	if err := instructionpacket.ValidateIdentity(identity); err != nil {
		return instructionpacket.Packet{}, err
	}
	current, production, err := Build(caseRoot, pack)
	if err != nil {
		return instructionpacket.Packet{}, err
	}
	if !production {
		return instructionpacket.Packet{}, fmt.Errorf("instruction identity names a non-production runtime pack: %s", pack)
	}
	if !instructionpacket.EqualIdentity(current.Identity(), identity) {
		return instructionpacket.Packet{}, fmt.Errorf("production instruction identity no longer matches the resolved runtime manifest")
	}
	ctx, err := resolve(caseRoot, pack)
	if err != nil {
		return instructionpacket.Packet{}, err
	}
	return productioninstruction.ReloadPacket(ctx.RepoRoot, identity)
}

func resolve(caseRoot, pack string) (rekitruntime.Context, error) {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return rekitruntime.Context{}, err
	}
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return rekitruntime.Context{}, err
	}
	if root.Existing && !root.Legacy {
		if inst.Source != "steamai" || inst.SchemaVersion < 2 || inst.Mode != "project-local-bundle" {
			return rekitruntime.Context{}, fmt.Errorf("current STeamAI production instructions require schema v2 project-local-bundle metadata: %s", inst.InstancePath)
		}
		assetRoot := filepath.Join(inst.CaseRoot, projectstate.CurrentDir)
		if !refsf.SamePath(inst.TemplateRoot, assetRoot) || !refsf.SamePath(inst.BundleRoot, filepath.Join(assetRoot, "runtime")) || strings.TrimSpace(inst.BundleManifestSHA256) == "" {
			return rekitruntime.Context{}, fmt.Errorf("current STeamAI production instruction metadata does not bind its project-local asset and runtime roots: %s", inst.InstancePath)
		}
	}
	return rekitruntime.NewWithCwd(caseRoot, pack, caseRoot)
}
