package sessionhost

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/productioncontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimeinstruction"
)

func currentProductionInstructionIdentity(caseRoot, pack string) (instructionpacket.Identity, error) {
	identity, err := optionalProductionInstructionIdentity(caseRoot, pack)
	if err != nil {
		return instructionpacket.Identity{}, err
	}
	if identity == nil {
		return instructionpacket.Identity{}, fmt.Errorf("pack has no production instruction contract: %s", pack)
	}
	return instructionpacket.CloneIdentity(*identity), nil
}

func optionalProductionInstructionIdentity(caseRoot, pack string) (*instructionpacket.Identity, error) {
	packet, production, err := runtimeinstruction.Build(caseRoot, pack)
	if err != nil {
		return nil, err
	}
	if !production {
		return nil, nil
	}
	identity := packet.Identity()
	return &identity, nil
}

func validateCurrentProductionInstructionIdentity(caseRoot, pack string, identity instructionpacket.Identity) error {
	if _, ok := productioncontract.ContractFor(pack); !ok {
		return fmt.Errorf("pack has no production instruction contract: %s", pack)
	}
	_, err := runtimeinstruction.Reload(caseRoot, pack, identity)
	return err
}

func cloneProductionInstructionIdentity(identity instructionpacket.Identity) instructionpacket.Identity {
	return instructionpacket.CloneIdentity(identity)
}

func cloneProductionInstructionIdentityPointer(identity *instructionpacket.Identity) *instructionpacket.Identity {
	if identity == nil {
		return nil
	}
	clone := instructionpacket.CloneIdentity(*identity)
	return &clone
}

func equalProductionInstructionIdentityPointers(left, right *instructionpacket.Identity) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return instructionpacket.EqualIdentity(*left, *right)
}

func validateProductionInstructionIdentityForPack(pack string, identity *instructionpacket.Identity) error {
	if identity == nil {
		return nil
	}
	if err := instructionpacket.ValidateIdentity(*identity); err != nil {
		return err
	}
	if identity.Pack != strings.TrimSpace(pack) {
		return fmt.Errorf("production instruction identity names a different pack")
	}
	return nil
}

func validateProductionInstructionBirth(pack string, identity *instructionpacket.Identity) error {
	pack = strings.TrimSpace(pack)
	_, production := productioncontract.ContractFor(pack)
	if !production {
		if identity != nil {
			return fmt.Errorf("non-production Claude launch cannot claim a production instruction identity")
		}
		return nil
	}
	if identity == nil {
		return fmt.Errorf("production Claude launch omitted its durable instruction identity")
	}
	return validateProductionInstructionIdentityForPack(pack, identity)
}

func validateClaudeProductionInstructionBinding(caseRoot, pack string, pkg mission.CurrentLoopExternalSessionHarnessPackage) error {
	resolvedPack := strings.TrimSpace(pack)
	packagePack := strings.TrimSpace(pkg.Pack)
	if resolvedPack == "" {
		resolvedPack = packagePack
	}
	if packagePack != "" && resolvedPack != packagePack {
		return fmt.Errorf("Claude launch package pack does not match the resolved runtime pack")
	}
	if _, production := productioncontract.ContractFor(resolvedPack); production && packagePack == "" {
		return fmt.Errorf("production Claude launch omitted its durable pack identity")
	}
	var identity *instructionpacket.Identity
	if pkg.Launch != nil {
		identity = pkg.Launch.InstructionIdentity
	}
	if err := validateProductionInstructionBirth(resolvedPack, identity); err != nil {
		return err
	}
	if identity == nil {
		return nil
	}
	return validateCurrentProductionInstructionIdentity(caseRoot, packagePack, *identity)
}

func inlineProductionInstructions(caseRoot string, pkg mission.CurrentLoopExternalSessionHarnessPackage) (string, error) {
	_, production := productioncontract.ContractFor(pkg.Pack)
	if !production {
		if pkg.Launch != nil && pkg.Launch.InstructionIdentity != nil {
			return "", fmt.Errorf("non-production Claude launch cannot claim a production instruction identity")
		}
		return "", nil
	}
	if pkg.Launch == nil || pkg.Launch.InstructionIdentity == nil {
		return "", fmt.Errorf("production Claude launch omitted its durable instruction identity")
	}
	packet, err := runtimeinstruction.Reload(caseRoot, pkg.Pack, *pkg.Launch.InstructionIdentity)
	if err != nil {
		return "", err
	}
	return packet.InlineMarkdown()
}
