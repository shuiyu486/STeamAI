package productioninstruction

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
)

const (
	ModePromptAndPolicy = instructionpacket.ModePromptAndPolicy
	ModePolicyOnly      = instructionpacket.ModePolicyOnly
)

type Contract struct {
	Pack            string
	Mode            string
	RequiredSources []string
	ReceiptKind     string
}

var registry = []Contract{
	{
		Pack:            packidentity.Canonical,
		Mode:            ModePromptAndPolicy,
		RequiredSources: []string{"common/prompts/lane-main-session.md", "common/prompts/lane-feature-session.md", "common/prompts/lane-merge-review.md", "packs/binary-re/prompts/feature-analysis-session.md"},
		ReceiptKind:     "binary-re-adapter-execution-result",
	},
	{
		Pack:            "web-security",
		Mode:            ModePolicyOnly,
		RequiredSources: []string{"common/policies/agent-team.md", "common/policies/tool-adapters.md", "common/policies/context-budget.md", "common/policies/subagents.md", "common/policies/lane-collaboration.md", "common/policies/review-first.md", "common/policies/write-boundaries.md", "common/policies/verification.md", "common/policies/evidence.md", "common/policies/tool-output.md", "common/policies/handoff.md"},
		ReceiptKind:     "web-security-adapter-execution-result",
	},
}

func Registry() []Contract {
	out := make([]Contract, len(registry))
	for i, contract := range registry {
		out[i] = cloneContract(contract)
	}
	return out
}

func ContractFor(pack string) (Contract, bool) {
	pack = strings.TrimSpace(pack)
	for _, contract := range registry {
		if contract.Pack == pack {
			return cloneContract(contract), true
		}
	}
	return Contract{}, false
}

func BuildPacket(repoRoot, pack string, m *manifest.Manifest) (instructionpacket.Packet, error) {
	contract, ok := ContractFor(pack)
	if !ok {
		return instructionpacket.Packet{}, fmt.Errorf("pack has no production instruction contract: %s", pack)
	}
	return instructionpacket.Build(repoRoot, pack, m, instructionpacket.Spec{
		Mode:            contract.Mode,
		RequiredSources: contract.RequiredSources,
		ReceiptKind:     contract.ReceiptKind,
	})
}

func ValidateIdentity(pack string, identity instructionpacket.Identity) error {
	if err := instructionpacket.ValidateIdentity(identity); err != nil {
		return err
	}
	contract, ok := ContractFor(pack)
	if !ok {
		return fmt.Errorf("pack has no production instruction contract: %s", pack)
	}
	if identity.Pack != contract.Pack || identity.Mode != contract.Mode || identity.ReceiptKind != contract.ReceiptKind {
		return fmt.Errorf("production instruction identity does not match its registered pack, mode, or receipt kind")
	}
	return nil
}

func ReloadPacket(repoRoot string, identity instructionpacket.Identity) (instructionpacket.Packet, error) {
	if err := ValidateIdentity(identity.Pack, identity); err != nil {
		return instructionpacket.Packet{}, err
	}
	return instructionpacket.Reload(repoRoot, identity)
}

func cloneContract(contract Contract) Contract {
	contract.RequiredSources = append([]string{}, contract.RequiredSources...)
	return contract
}
