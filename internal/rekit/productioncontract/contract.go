package productioncontract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/binaryinventory"
	"github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/productioninstruction"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
)

const (
	InstructionModePromptAndPolicy = productioninstruction.ModePromptAndPolicy
	InstructionModePolicyOnly      = productioninstruction.ModePolicyOnly
)

type Contract struct {
	Pack             string
	AdapterIDs       []string
	Fixture          SourceContract
	SemanticVerifier SourceContract
	Instruction      InstructionContract
	ReceiptKind      string
}

type SourceContract struct {
	Identity string
	Bindings []GoSourceBinding
}

type GoSourceBinding struct {
	Path    string
	Symbols []string
}

type InstructionContract struct {
	Mode            string
	RequiredSources []string
}

type InstructionIdentity = instructionpacket.Identity
type Packet = instructionpacket.Packet

func productionInstructionContract(pack string) InstructionContract {
	contract, _ := productioninstruction.ContractFor(pack)
	return InstructionContract{
		Mode:            contract.Mode,
		RequiredSources: append([]string{}, contract.RequiredSources...),
	}
}

func productionInstructionReceiptKind(pack string) string {
	contract, _ := productioninstruction.ContractFor(pack)
	return contract.ReceiptKind
}

type Admission struct {
	Pack                string               `json:"pack"`
	Maturity            string               `json:"maturity"`
	Ready               bool                 `json:"ready"`
	Warnings            []string             `json:"warnings"`
	Adapters            []string             `json:"adapters"`
	Fixture             string               `json:"fixture"`
	Verifier            string               `json:"verifier"`
	Instruction         string               `json:"instruction"`
	ReceiptKind         string               `json:"receiptKind"`
	InstructionIdentity *InstructionIdentity `json:"instructionIdentity,omitempty"`
}

type RegistryAdmission struct {
	Ready              bool     `json:"ready"`
	Warnings           []string `json:"warnings"`
	MaturePacks        []string `json:"maturePacks"`
	ContractPacks      []string `json:"contractPacks"`
	CompiledInAdapters []string `json:"compiledInAdapters"`
	ContractAdapters   []string `json:"contractAdapters"`
}

var registry = []Contract{
	{
		Pack:       packidentity.Canonical,
		AdapterIDs: []string{binaryinventory.AdapterID, adapterhost.VMPIDAIndexAdapterID},
		Fixture: SourceContract{
			Identity: "synthetic PE and ELF parser fixtures",
			Bindings: []GoSourceBinding{
				{Path: "internal/rekit/binaryinventory/inspect_test.go", Symbols: []string{"TestInspectSyntheticPEAndELFMatchGolden", "TestInspectRejectsBindingDriftUnsupportedAndTruncatedInput"}},
				{Path: "internal/rekit/adapterhost/vmp_ida_index_test.go", Symbols: []string{"TestInspectVMPIDAIndexMatchesLiteralsCaseInsensitively", "TestInspectVMPIDAIndexRejectsNonCanonicalRequestAndSourceDrift"}},
			},
		},
		SemanticVerifier: SourceContract{
			Identity: "typed PE/ELF inventory and VMP/IDA packet validators",
			Bindings: []GoSourceBinding{
				{Path: "internal/rekit/binaryinventory/model.go", Symbols: []string{"CanonicalBytes", "Validate"}},
				{Path: "internal/rekit/adapterhost/vmp_ida_index.go", Symbols: []string{"InspectVMPIDAIndex"}},
			},
		},
		Instruction: productionInstructionContract(packidentity.Canonical),
		ReceiptKind: productionInstructionReceiptKind(packidentity.Canonical),
	},
	{
		Pack:       "web-security",
		AdapterIDs: []string{websecurity.InventoryAdapterID, websecurity.ReplayAdapterID},
		Fixture: SourceContract{
			Identity: "synthetic OpenAPI and loopback replay fixtures",
			Bindings: []GoSourceBinding{
				{Path: "internal/rekit/websecurity/openapi_test.go", Symbols: []string{"TestImportOpenAPIProducesCanonicalSecretFreeInventory", "TestImportOpenAPIRejectsAmbiguousOrUnsupportedInputs"}},
				{Path: "internal/rekit/websecurity/replay_test.go", Symbols: []string{"TestExecuteReplayUsesExactLoopbackOnceAndReturnsRedactedDigestDiff", "TestExecuteReplayConnectionFailureIsUncertainAndNeverRetried"}},
			},
		},
		SemanticVerifier: SourceContract{
			Identity: "typed OpenAPI inventory and bounded replay result validators",
			Bindings: []GoSourceBinding{
				{Path: "internal/rekit/websecurity/openapi.go", Symbols: []string{"ImportOpenAPI"}},
				{Path: "internal/rekit/websecurity/model.go", Symbols: []string{"ValidateInventory", "ValidateReplayResult"}},
				{Path: "internal/rekit/websecurity/replay.go", Symbols: []string{"ExecuteReplay"}},
			},
		},
		Instruction: productionInstructionContract("web-security"),
		ReceiptKind: productionInstructionReceiptKind("web-security"),
	},
}

func Registry() []Contract {
	out := make([]Contract, len(registry))
	for i, item := range registry {
		out[i] = cloneContract(item)
	}
	return out
}

func ContractFor(pack string) (Contract, bool) {
	for _, item := range registry {
		if item.Pack == strings.TrimSpace(pack) {
			return cloneContract(item), true
		}
	}
	return Contract{}, false
}

func CompiledInProductionAdapterIDs() []string {
	return adapterhost.CompiledInProductionAdapterIDs()
}

func BuildAdmission(repoRoot string, summary manifest.PackSummary) Admission {
	admission := Admission{Pack: summary.ID, Maturity: summary.Maturity, Ready: true, Warnings: []string{}}
	if summary.Maturity != "mature" {
		return admission
	}
	contract, ok := ContractFor(summary.ID)
	if !ok {
		admission.Ready = false
		admission.Warnings = append(admission.Warnings, "mature pack has no production contract registry entry")
		return admission
	}
	admission.Adapters = append([]string{}, contract.AdapterIDs...)
	admission.Fixture = contract.Fixture.Identity
	admission.Verifier = contract.SemanticVerifier.Identity
	admission.Instruction = contract.Instruction.Mode
	admission.ReceiptKind = contract.ReceiptKind
	for _, id := range contract.AdapterIDs {
		if !slices.Contains(CompiledInProductionAdapterIDs(), id) {
			admission.Warnings = append(admission.Warnings, fmt.Sprintf("production contract adapter is not compiled in: %s", id))
		}
	}
	admission.Warnings = append(admission.Warnings, validateGoSourceContract(repoRoot, "fixture", contract.Fixture)...)
	admission.Warnings = append(admission.Warnings, validateGoSourceContract(repoRoot, "semantic verifier", contract.SemanticVerifier)...)
	if contract.Instruction.Mode != InstructionModePromptAndPolicy && contract.Instruction.Mode != InstructionModePolicyOnly {
		admission.Warnings = append(admission.Warnings, "production contract instruction mode is unsupported")
	}
	if len(contract.Instruction.RequiredSources) == 0 {
		admission.Warnings = append(admission.Warnings, "production contract instruction sources are empty")
	}
	for _, source := range contract.Instruction.RequiredSources {
		if !regularRepoFile(repoRoot, source) {
			admission.Warnings = append(admission.Warnings, fmt.Sprintf("production contract instruction source is missing: %s", source))
		}
	}
	if strings.TrimSpace(contract.ReceiptKind) == "" {
		admission.Warnings = append(admission.Warnings, "production contract receipt kind is empty")
	}
	m, err := manifest.Load(repoRoot, summary.ID)
	if err != nil {
		admission.Warnings = append(admission.Warnings, fmt.Sprintf("production instruction manifest cannot be loaded: %v", err))
	} else if packet, err := BuildInstructionPacket(repoRoot, summary.ID, m); err != nil {
		admission.Warnings = append(admission.Warnings, fmt.Sprintf("production instruction packet cannot be built: %v", err))
	} else {
		identity := packet.Identity()
		if err := ValidateInstructionIdentity(identity); err != nil {
			admission.Warnings = append(admission.Warnings, fmt.Sprintf("production instruction identity is invalid: %v", err))
		} else {
			admission.InstructionIdentity = &identity
		}
	}
	admission.Ready = len(admission.Warnings) == 0
	return admission
}

func BuildRegistryAdmission(summaries []manifest.PackSummary) RegistryAdmission {
	admission := RegistryAdmission{Ready: true, Warnings: []string{}}
	for _, summary := range summaries {
		if summary.Maturity == "mature" {
			admission.MaturePacks = append(admission.MaturePacks, summary.ID)
		}
	}
	for _, contract := range registry {
		admission.ContractPacks = append(admission.ContractPacks, contract.Pack)
		admission.ContractAdapters = append(admission.ContractAdapters, contract.AdapterIDs...)
	}
	admission.MaturePacks = uniqueSorted(admission.MaturePacks)
	admission.ContractPacks = uniqueSorted(admission.ContractPacks)
	admission.CompiledInAdapters = uniqueSorted(CompiledInProductionAdapterIDs())
	admission.ContractAdapters = uniqueSorted(admission.ContractAdapters)
	if !slices.Equal(admission.MaturePacks, admission.ContractPacks) {
		admission.Warnings = append(admission.Warnings, fmt.Sprintf("mature pack set does not match production contract registry: mature=%v contracts=%v", admission.MaturePacks, admission.ContractPacks))
	}
	if !slices.Equal(admission.CompiledInAdapters, admission.ContractAdapters) {
		admission.Warnings = append(admission.Warnings, fmt.Sprintf("compiled-in production adapter set does not match production contract registry: compiled=%v contracts=%v", admission.CompiledInAdapters, admission.ContractAdapters))
	}
	admission.Warnings = append(admission.Warnings, duplicateRegistryWarnings()...)
	admission.Ready = len(admission.Warnings) == 0
	return admission
}

func validateGoSourceContract(repoRoot, role string, source SourceContract) []string {
	warnings := []string{}
	if strings.TrimSpace(source.Identity) == "" {
		warnings = append(warnings, fmt.Sprintf("production contract %s identity is empty", role))
	}
	if len(source.Bindings) == 0 {
		return append(warnings, fmt.Sprintf("production contract %s bindings are empty", role))
	}
	seenPaths := map[string]bool{}
	for _, binding := range source.Bindings {
		rel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(binding.Path)))
		if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") || filepath.Ext(rel) != ".go" {
			warnings = append(warnings, fmt.Sprintf("production contract %s source path is invalid: %s", role, binding.Path))
			continue
		}
		if seenPaths[rel] {
			warnings = append(warnings, fmt.Sprintf("production contract %s source is duplicated: %s", role, rel))
			continue
		}
		seenPaths[rel] = true
		data, err := fs.ReadStableRegularFileAnchored(repoRoot, filepath.Join(repoRoot, filepath.FromSlash(rel)), "production contract Go source", 2<<20)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("production contract %s source is missing: %s", role, rel))
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), rel, data, 0)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("production contract %s source cannot be parsed: %s", role, rel))
			continue
		}
		declared := map[string]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.FuncDecl)
			if ok && declaration.Recv == nil {
				declared[declaration.Name.Name] = true
			}
			return true
		})
		if len(binding.Symbols) == 0 {
			warnings = append(warnings, fmt.Sprintf("production contract %s source has no symbol markers: %s", role, rel))
			continue
		}
		seenSymbols := map[string]bool{}
		for _, symbol := range binding.Symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol == "" || seenSymbols[symbol] {
				warnings = append(warnings, fmt.Sprintf("production contract %s source has an empty or duplicate symbol marker: %s", role, rel))
				continue
			}
			seenSymbols[symbol] = true
			if !declared[symbol] {
				warnings = append(warnings, fmt.Sprintf("production contract %s symbol is missing: %s:%s", role, rel, symbol))
			}
		}
	}
	return warnings
}

func duplicateRegistryWarnings() []string {
	warnings := []string{}
	packs := map[string]bool{}
	adapters := map[string]string{}
	receipts := map[string]string{}
	for _, contract := range registry {
		pack := strings.TrimSpace(contract.Pack)
		if pack == "" || packs[pack] {
			warnings = append(warnings, fmt.Sprintf("production contract registry contains an empty or duplicate pack: %q", pack))
		} else {
			packs[pack] = true
		}
		for _, adapterID := range contract.AdapterIDs {
			adapterID = strings.TrimSpace(adapterID)
			if previous, exists := adapters[adapterID]; adapterID == "" || exists {
				warnings = append(warnings, fmt.Sprintf("production contract adapter is empty or duplicated across %s and %s: %q", previous, pack, adapterID))
			} else {
				adapters[adapterID] = pack
			}
		}
		receiptKind := strings.TrimSpace(contract.ReceiptKind)
		if previous, exists := receipts[receiptKind]; receiptKind == "" || exists {
			warnings = append(warnings, fmt.Sprintf("production contract receipt kind is empty or duplicated across %s and %s: %q", previous, pack, receiptKind))
		} else {
			receipts[receiptKind] = pack
		}
	}
	return warnings
}

func BuildInstructionPacket(repoRoot, pack string, m *manifest.Manifest) (Packet, error) {
	return productioninstruction.BuildPacket(repoRoot, pack, m)
}

func ReloadInstructionPacket(repoRoot string, identity InstructionIdentity) (Packet, error) {
	return productioninstruction.ReloadPacket(repoRoot, identity)
}

func ValidateInstructionIdentity(identity InstructionIdentity) error {
	return instructionpacket.ValidateIdentity(identity)
}

func regularRepoFile(root, rel string) bool {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func cloneContract(value Contract) Contract {
	value.AdapterIDs = append([]string{}, value.AdapterIDs...)
	value.Fixture = cloneSourceContract(value.Fixture)
	value.SemanticVerifier = cloneSourceContract(value.SemanticVerifier)
	value.Instruction.RequiredSources = append([]string{}, value.Instruction.RequiredSources...)
	return value
}

func cloneSourceContract(value SourceContract) SourceContract {
	value.Bindings = append([]GoSourceBinding{}, value.Bindings...)
	for i := range value.Bindings {
		value.Bindings[i].Symbols = append([]string{}, value.Bindings[i].Symbols...)
	}
	return value
}
