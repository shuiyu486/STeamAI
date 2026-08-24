package instructionpacket

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

const (
	SchemaVersion       = 1
	ModePromptAndPolicy = "prompt-and-policy"
	ModePolicyOnly      = "policy-only"
	maxSourceBytes      = 64 << 10
	maxPacketBytes      = 512 << 10
)

type Spec struct {
	Mode            string
	RequiredSources []string
	ReceiptKind     string
}

type SourceBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Identity struct {
	SchemaVersion int             `json:"schemaVersion"`
	Pack          string          `json:"pack"`
	Mode          string          `json:"mode"`
	ReceiptKind   string          `json:"receiptKind"`
	Sources       []SourceBinding `json:"sources"`
	SHA256        string          `json:"sha256"`
	Bytes         int64           `json:"bytes"`
}

type Packet struct {
	identity Identity
	sources  []loadedSource
}

type loadedSource struct {
	binding SourceBinding
	content []byte
}

func Build(repoRoot, pack string, m *manifest.Manifest, spec Spec) (Packet, error) {
	pack = strings.TrimSpace(pack)
	if m == nil || m.Pack != pack {
		return Packet{}, fmt.Errorf("instruction packet manifest does not match pack %q", pack)
	}
	if err := validateSpec(spec); err != nil {
		return Packet{}, err
	}
	paths, err := manifestInstructionPaths(repoRoot, pack, m, spec.Mode)
	if err != nil {
		return Packet{}, err
	}
	for _, required := range spec.RequiredSources {
		required, err = cleanRepoPath(required)
		if err != nil {
			return Packet{}, fmt.Errorf("production instruction required source is invalid: %w", err)
		}
		if !slices.Contains(paths, required) {
			return Packet{}, fmt.Errorf("instruction packet omitted contract source: %s", required)
		}
	}
	if len(paths) == 0 {
		return Packet{}, fmt.Errorf("production instruction packet is empty")
	}
	identity := Identity{
		SchemaVersion: SchemaVersion,
		Pack:          pack,
		Mode:          spec.Mode,
		ReceiptKind:   strings.TrimSpace(spec.ReceiptKind),
		Sources:       make([]SourceBinding, 0, len(paths)),
	}
	packet := Packet{sources: make([]loadedSource, 0, len(paths))}
	for _, rel := range paths {
		data, err := readSource(repoRoot, rel)
		if err != nil {
			return Packet{}, err
		}
		binding := SourceBinding{Path: rel, SHA256: hash(data), Bytes: int64(len(data))}
		identity.Sources = append(identity.Sources, binding)
		identity.Bytes += binding.Bytes
		packet.sources = append(packet.sources, loadedSource{binding: binding, content: append([]byte{}, data...)})
	}
	if identity.Bytes > maxPacketBytes {
		return Packet{}, fmt.Errorf("production instruction packet exceeds %d bytes", maxPacketBytes)
	}
	identity.SHA256 = identityHash(identity)
	packet.identity = identity
	return packet, nil
}

func Reload(repoRoot string, identity Identity) (Packet, error) {
	if err := ValidateIdentity(identity); err != nil {
		return Packet{}, err
	}
	packet := Packet{identity: cloneIdentity(identity), sources: make([]loadedSource, 0, len(identity.Sources))}
	for _, binding := range identity.Sources {
		data, err := readSource(repoRoot, binding.Path)
		if err != nil {
			return Packet{}, err
		}
		if int64(len(data)) != binding.Bytes || hash(data) != binding.SHA256 {
			return Packet{}, fmt.Errorf("production instruction source drifted: %s", binding.Path)
		}
		packet.sources = append(packet.sources, loadedSource{binding: binding, content: append([]byte{}, data...)})
	}
	return packet, nil
}

func ValidateIdentity(identity Identity) error {
	if identity.SchemaVersion != SchemaVersion || strings.TrimSpace(identity.Pack) == "" || strings.ContainsAny(identity.Pack, "\r\n") {
		return fmt.Errorf("production instruction packet identity has invalid schema or pack")
	}
	if identity.Mode != ModePromptAndPolicy && identity.Mode != ModePolicyOnly {
		return fmt.Errorf("production instruction packet identity has unsupported mode")
	}
	if strings.TrimSpace(identity.ReceiptKind) == "" || strings.ContainsAny(identity.ReceiptKind, "\r\n") || len(identity.Sources) == 0 {
		return fmt.Errorf("production instruction packet identity is incomplete")
	}
	var total int64
	previous := ""
	for _, source := range identity.Sources {
		clean, err := cleanRepoPath(source.Path)
		if err != nil || clean != source.Path || source.Bytes < 1 || source.Bytes > maxSourceBytes || !validHash(source.SHA256) {
			return fmt.Errorf("production instruction source binding is invalid: %s", source.Path)
		}
		if previous != "" && source.Path <= previous {
			return fmt.Errorf("production instruction source bindings are not unique and sorted")
		}
		previous = source.Path
		total += source.Bytes
	}
	if total != identity.Bytes || total > maxPacketBytes || !validHash(identity.SHA256) || identityHash(identity) != identity.SHA256 {
		return fmt.Errorf("production instruction packet aggregate identity drifted")
	}
	return nil
}

func (p Packet) Identity() Identity {
	return cloneIdentity(p.identity)
}

func (p Packet) InlineMarkdown() (string, error) {
	if err := validateLoadedPacket(p); err != nil {
		return "", err
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "## Production instruction packet\n\nPack: `%s`  \nMode: `%s`  \nReceipt kind: `%s`  \nPacket SHA-256: `%s`\n\n", p.identity.Pack, p.identity.Mode, p.identity.ReceiptKind, p.identity.SHA256)
	out.WriteString("The following verified project-local sources are mandatory instructions for this session. Apply all of them; do not replace them with machine-global or central-kit content.\n")
	for _, source := range p.sources {
		fmt.Fprintf(&out, "\n### Instruction source `%s`\n\n", source.binding.Path)
		out.Write(source.content)
		if len(source.content) == 0 || source.content[len(source.content)-1] != '\n' {
			out.WriteByte('\n')
		}
	}
	return out.String(), nil
}

func validateLoadedPacket(packet Packet) error {
	if err := ValidateIdentity(packet.identity); err != nil {
		return err
	}
	if len(packet.sources) != len(packet.identity.Sources) {
		return fmt.Errorf("production instruction packet content count drifted")
	}
	for i, source := range packet.sources {
		binding := packet.identity.Sources[i]
		if source.binding != binding || int64(len(source.content)) != binding.Bytes || hash(source.content) != binding.SHA256 || !utf8.Valid(source.content) {
			return fmt.Errorf("production instruction packet content drifted: %s", binding.Path)
		}
	}
	return nil
}

func manifestInstructionPaths(repoRoot, pack string, m *manifest.Manifest, mode string) ([]string, error) {
	policies, err := commonPolicyPaths(repoRoot, m.CommonPolicies)
	if err != nil {
		return nil, err
	}
	paths := append([]string{}, policies...)
	for _, rel := range m.PolicyOverlays {
		paths = append(paths, filepath.ToSlash(filepath.Join("packs", pack, rel)))
	}
	switch mode {
	case ModePromptAndPolicy:
		if len(m.PromptFiles) == 0 || len(policies)+len(m.PolicyOverlays) == 0 {
			return nil, fmt.Errorf("prompt-and-policy instruction contract requires declared prompts and policies")
		}
		paths = append(paths, m.PromptFiles...)
	case ModePolicyOnly:
		if len(m.PromptFiles) != 0 {
			return nil, fmt.Errorf("policy-only instruction contract cannot silently omit declared prompts")
		}
		if len(policies)+len(m.PolicyOverlays) == 0 {
			return nil, fmt.Errorf("policy-only instruction contract requires declared policies")
		}
	default:
		return nil, fmt.Errorf("production instruction mode is unsupported: %s", mode)
	}
	return cleanUniqueSorted(paths)
}

func commonPolicyPaths(repoRoot string, ids []string) ([]string, error) {
	entries, err := manifest.ObjectListFromFile(filepath.Join(repoRoot, "common", "policies", "manifest.yml"), "policies")
	if err != nil {
		return nil, fmt.Errorf("read common policy registry: %w", err)
	}
	registry := map[string]string{}
	for _, entry := range entries {
		id := strings.TrimSpace(entry["id"])
		path := strings.TrimSpace(entry["path"])
		if id == "" || path == "" {
			return nil, fmt.Errorf("common policy registry contains an incomplete entry")
		}
		if _, exists := registry[id]; exists {
			return nil, fmt.Errorf("common policy registry contains duplicate id: %s", id)
		}
		registry[id] = filepath.ToSlash(filepath.Join("common", "policies", path))
	}
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		path, ok := registry[id]
		if !ok {
			return nil, fmt.Errorf("manifest references unknown common policy: %s", id)
		}
		paths = append(paths, path)
	}
	return cleanUniqueSorted(paths)
}

func readSource(repoRoot, rel string) ([]byte, error) {
	data, err := fs.ReadStableRegularFileAnchored(repoRoot, filepath.Join(repoRoot, filepath.FromSlash(rel)), "production instruction source", maxSourceBytes)
	if err != nil {
		return nil, fmt.Errorf("read instruction source %s: %w", rel, err)
	}
	if len(data) == 0 || !utf8.Valid(data) {
		return nil, fmt.Errorf("production instruction source must be non-empty UTF-8: %s", rel)
	}
	return data, nil
}

func validateSpec(spec Spec) error {
	if spec.Mode != ModePromptAndPolicy && spec.Mode != ModePolicyOnly {
		return fmt.Errorf("production instruction mode is unsupported: %s", spec.Mode)
	}
	if strings.TrimSpace(spec.ReceiptKind) == "" || strings.ContainsAny(spec.ReceiptKind, "\r\n") {
		return fmt.Errorf("production instruction receipt kind is invalid")
	}
	if len(spec.RequiredSources) == 0 {
		return fmt.Errorf("production instruction required sources are empty")
	}
	return nil
}

func cleanUniqueSorted(values []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		clean, err := cleanRepoPath(value)
		if err != nil {
			return nil, err
		}
		if seen[clean] {
			return nil, fmt.Errorf("production instruction source is duplicated: %s", clean)
		}
		seen[clean] = true
		out = append(out, clean)
	}
	slices.Sort(out)
	return out, nil
}

func cleanRepoPath(value string) (string, error) {
	raw := strings.TrimSpace(value)
	clean := filepath.Clean(filepath.FromSlash(raw))
	if raw == "" || clean == "." || filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("production instruction source path is invalid: %s", value)
	}
	return filepath.ToSlash(clean), nil
}

func CloneIdentity(value Identity) Identity {
	value.Sources = append([]SourceBinding{}, value.Sources...)
	return value
}

func EqualIdentity(left, right Identity) bool {
	return left.SchemaVersion == right.SchemaVersion &&
		left.Pack == right.Pack &&
		left.Mode == right.Mode &&
		left.ReceiptKind == right.ReceiptKind &&
		left.SHA256 == right.SHA256 &&
		left.Bytes == right.Bytes &&
		slices.Equal(left.Sources, right.Sources)
}

func cloneIdentity(value Identity) Identity {
	return CloneIdentity(value)
}

func identityHash(identity Identity) string {
	data, err := json.Marshal(struct {
		SchemaVersion int             `json:"schemaVersion"`
		Pack          string          `json:"pack"`
		Mode          string          `json:"mode"`
		ReceiptKind   string          `json:"receiptKind"`
		Sources       []SourceBinding `json:"sources"`
		Bytes         int64           `json:"bytes"`
	}{identity.SchemaVersion, identity.Pack, identity.Mode, identity.ReceiptKind, identity.Sources, identity.Bytes})
	if err != nil {
		panic(err)
	}
	return hash(data)
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}
