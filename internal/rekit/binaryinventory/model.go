package binaryinventory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	SchemaVersion = 1
	Kind          = "binary-re-pe-elf-inventory"
	AdapterID     = "static-binary-triage-sidecar"

	MaxInputBytes  = 16 << 20
	MaxOutputBytes = 1 << 20
	MaxSections    = 128
	MaxImports     = 4096
	MaxExports     = 4096
	MaxStringBytes = 512
)

var (
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	hexPattern    = regexp.MustCompile(`^0x[0-9a-f]+$`)
)

type SourceBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type FormatInventory struct {
	Family     string `json:"family"`
	Class      string `json:"class"`
	Bitness    int    `json:"bitness"`
	Endianness string `json:"endianness"`
	Machine    string `json:"machine"`
	FileType   string `json:"fileType"`
	EntryPoint string `json:"entryPoint"`
	ImageBase  string `json:"imageBase,omitempty"`
}

type Section struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	VirtualAddress string `json:"virtualAddress"`
	VirtualSize    string `json:"virtualSize"`
	FileOffset     int64  `json:"fileOffset"`
	FileSize       int64  `json:"fileSize"`
	Permissions    string `json:"permissions"`
}

type Import struct {
	Library string `json:"library"`
	Symbol  string `json:"symbol,omitempty"`
	Version string `json:"version,omitempty"`
}

type Export struct {
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Ordinal   uint32 `json:"ordinal,omitempty"`
	Address   string `json:"address"`
	Size      string `json:"size,omitempty"`
	Forwarder string `json:"forwarder,omitempty"`
}

type Boundaries struct {
	ReadOnlyInput        bool `json:"readOnlyInput"`
	NoSampleExecution    bool `json:"noSampleExecution"`
	NoNetwork            bool `json:"noNetwork"`
	NoCatalogEntryExec   bool `json:"noCatalogEntryExecution"`
	NoAuthorityConfirmed bool `json:"noAuthorityOrConfirmed"`
}

type Sidecar struct {
	SchemaVersion int             `json:"schemaVersion"`
	Kind          string          `json:"kind"`
	AdapterID     string          `json:"adapterId"`
	Source        SourceBinding   `json:"source"`
	Format        FormatInventory `json:"format"`
	Sections      []Section       `json:"sections"`
	Imports       []Import        `json:"imports"`
	Exports       []Export        `json:"exports"`
	Warnings      []string        `json:"warnings"`
	Boundaries    Boundaries      `json:"boundaries"`
}

func BindSource(sourcePath string, data []byte) (SourceBinding, error) {
	binding := SourceBinding{
		Path:   sourcePath,
		SHA256: SHA256(data),
		Bytes:  int64(len(data)),
	}
	if err := ValidateSource(binding); err != nil {
		return SourceBinding{}, err
	}
	return binding, nil
}

func ValidateSource(source SourceBinding) error {
	return validateSource(source)
}

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func CanonicalBytes(sidecar Sidecar) ([]byte, error) {
	if err := Validate(sidecar); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if len(data) > MaxOutputBytes {
		return nil, fmt.Errorf("binary inventory exceeds %d bytes", MaxOutputBytes)
	}
	return data, nil
}

func Decode(data []byte) (Sidecar, error) {
	if len(data) == 0 || len(data) > MaxOutputBytes {
		return Sidecar{}, fmt.Errorf("binary inventory sidecar size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var sidecar Sidecar
	if err := decoder.Decode(&sidecar); err != nil {
		return Sidecar{}, fmt.Errorf("decode binary inventory sidecar: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Sidecar{}, fmt.Errorf("binary inventory sidecar must contain exactly one JSON object")
	}
	if err := Validate(sidecar); err != nil {
		return Sidecar{}, err
	}
	canonical, err := CanonicalBytes(sidecar)
	if err != nil {
		return Sidecar{}, err
	}
	if !bytes.Equal(data, canonical) {
		return Sidecar{}, fmt.Errorf("binary inventory sidecar is not canonical JSON")
	}
	return sidecar, nil
}

func Validate(sidecar Sidecar) error {
	if sidecar.SchemaVersion != SchemaVersion || sidecar.Kind != Kind || sidecar.AdapterID != AdapterID {
		return fmt.Errorf("binary inventory identity is invalid")
	}
	if err := validateSource(sidecar.Source); err != nil {
		return err
	}
	if err := validateFormat(sidecar.Format); err != nil {
		return err
	}
	if sidecar.Sections == nil || len(sidecar.Sections) > MaxSections {
		return fmt.Errorf("binary inventory sections are missing or exceed %d", MaxSections)
	}
	if sidecar.Imports == nil || len(sidecar.Imports) > MaxImports {
		return fmt.Errorf("binary inventory imports are missing or exceed %d", MaxImports)
	}
	if sidecar.Exports == nil || len(sidecar.Exports) > MaxExports {
		return fmt.Errorf("binary inventory exports are missing or exceed %d", MaxExports)
	}
	if sidecar.Warnings == nil || len(sidecar.Warnings) > 32 {
		return fmt.Errorf("binary inventory warnings are missing or exceed 32")
	}
	if err := validateSections(sidecar.Sections); err != nil {
		return err
	}
	if err := validateImports(sidecar.Imports); err != nil {
		return err
	}
	if err := validateExports(sidecar.Exports); err != nil {
		return err
	}
	if err := validateWarnings(sidecar.Warnings); err != nil {
		return err
	}
	boundary := sidecar.Boundaries
	if !boundary.ReadOnlyInput || !boundary.NoSampleExecution || !boundary.NoNetwork || !boundary.NoCatalogEntryExec || !boundary.NoAuthorityConfirmed {
		return fmt.Errorf("binary inventory safety boundaries must all be true")
	}
	return nil
}

func validateSource(source SourceBinding) error {
	if source.Path == "" || source.Path != strings.TrimSpace(source.Path) || strings.ContainsAny(source.Path, `:\`) || path.IsAbs(source.Path) || path.Clean(source.Path) != source.Path || source.Path == "." || strings.HasPrefix(source.Path, "../") {
		return fmt.Errorf("binary inventory source path must be canonical and case-relative")
	}
	if !sha256Pattern.MatchString(source.SHA256) {
		return fmt.Errorf("binary inventory source sha256 is invalid")
	}
	if source.Bytes < 1 || source.Bytes > MaxInputBytes {
		return fmt.Errorf("binary inventory source size must be within 1..%d bytes", MaxInputBytes)
	}
	return nil
}

func validateFormat(format FormatInventory) error {
	if format.Family != "pe" && format.Family != "elf" {
		return fmt.Errorf("binary inventory format family is invalid: %s", format.Family)
	}
	if format.Bitness != 32 && format.Bitness != 64 {
		return fmt.Errorf("binary inventory bitness is invalid: %d", format.Bitness)
	}
	wantClass := map[string]map[int]string{"pe": {32: "pe32", 64: "pe32+"}, "elf": {32: "elf32", 64: "elf64"}}[format.Family][format.Bitness]
	if format.Class != wantClass {
		return fmt.Errorf("binary inventory class %s does not match %s/%d", format.Class, format.Family, format.Bitness)
	}
	if format.Endianness != "little" && format.Endianness != "big" {
		return fmt.Errorf("binary inventory endianness is invalid: %s", format.Endianness)
	}
	for label, value := range map[string]string{"machine": format.Machine, "fileType": format.FileType} {
		if err := validateText(label, value, false); err != nil {
			return err
		}
	}
	if !hexPattern.MatchString(format.EntryPoint) {
		return fmt.Errorf("binary inventory entry point is invalid")
	}
	if format.Family == "pe" {
		if !hexPattern.MatchString(format.ImageBase) {
			return fmt.Errorf("PE inventory image base is invalid")
		}
	} else if format.ImageBase != "" {
		return fmt.Errorf("ELF inventory must not contain a PE image base")
	}
	return nil
}

func validateSections(sections []Section) error {
	previous := ""
	seen := map[string]bool{}
	for _, section := range sections {
		if err := validateText("section name", section.Name, false); err != nil {
			return err
		}
		if err := validateText("section type", section.Type, false); err != nil {
			return err
		}
		if !hexPattern.MatchString(section.VirtualAddress) || !hexPattern.MatchString(section.VirtualSize) {
			return fmt.Errorf("binary inventory section address or size is invalid: %s", section.Name)
		}
		if section.FileOffset < 0 || section.FileSize < 0 || section.FileOffset > MaxInputBytes || section.FileSize > MaxInputBytes {
			return fmt.Errorf("binary inventory section file bounds are invalid: %s", section.Name)
		}
		if len(section.Permissions) != 3 || (section.Permissions[0] != 'r' && section.Permissions[0] != '-') || (section.Permissions[1] != 'w' && section.Permissions[1] != '-') || (section.Permissions[2] != 'x' && section.Permissions[2] != '-') {
			return fmt.Errorf("binary inventory section permissions are invalid: %s", section.Name)
		}
		key := sectionKey(section)
		if seen[key] || (previous != "" && key < previous) {
			return fmt.Errorf("binary inventory sections must be unique and sorted")
		}
		seen[key] = true
		previous = key
	}
	return nil
}

func validateImports(imports []Import) error {
	previous := ""
	seen := map[string]bool{}
	for _, item := range imports {
		if err := validateText("import library", item.Library, false); err != nil {
			return err
		}
		if err := validateText("import symbol", item.Symbol, true); err != nil {
			return err
		}
		if err := validateText("import version", item.Version, true); err != nil {
			return err
		}
		key := importKey(item)
		if seen[key] || (previous != "" && key < previous) {
			return fmt.Errorf("binary inventory imports must be unique and sorted")
		}
		seen[key] = true
		previous = key
	}
	return nil
}

func validateExports(exports []Export) error {
	previous := ""
	seen := map[string]bool{}
	for _, item := range exports {
		if err := validateText("export name", item.Name, false); err != nil {
			return err
		}
		if err := validateText("export type", item.Type, true); err != nil {
			return err
		}
		if !hexPattern.MatchString(item.Address) || (item.Size != "" && !hexPattern.MatchString(item.Size)) {
			return fmt.Errorf("binary inventory export address or size is invalid: %s", item.Name)
		}
		if err := validateText("export forwarder", item.Forwarder, true); err != nil {
			return err
		}
		key := exportKey(item)
		if seen[key] || (previous != "" && key < previous) {
			return fmt.Errorf("binary inventory exports must be unique and sorted")
		}
		seen[key] = true
		previous = key
	}
	return nil
}

func validateWarnings(warnings []string) error {
	previous := ""
	for _, warning := range warnings {
		if err := validateText("warning", warning, false); err != nil {
			return err
		}
		if warning <= previous {
			return fmt.Errorf("binary inventory warnings must be unique and sorted")
		}
		previous = warning
	}
	return nil
}

func validateText(label, value string, allowEmpty bool) error {
	if value == "" && allowEmpty {
		return nil
	}
	if value == "" || value != strings.TrimSpace(value) || len(value) > MaxStringBytes || !utf8.ValidString(value) {
		return fmt.Errorf("binary inventory %s is invalid", label)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("binary inventory %s contains a control character", label)
		}
	}
	return nil
}

func normalize(sidecar *Sidecar) {
	sort.Slice(sidecar.Sections, func(i, j int) bool { return sectionKey(sidecar.Sections[i]) < sectionKey(sidecar.Sections[j]) })
	sort.Slice(sidecar.Imports, func(i, j int) bool { return importKey(sidecar.Imports[i]) < importKey(sidecar.Imports[j]) })
	sort.Slice(sidecar.Exports, func(i, j int) bool { return exportKey(sidecar.Exports[i]) < exportKey(sidecar.Exports[j]) })
	sort.Strings(sidecar.Warnings)
	sidecar.Sections = dedupeSections(sidecar.Sections)
	sidecar.Imports = dedupeImports(sidecar.Imports)
	sidecar.Exports = dedupeExports(sidecar.Exports)
	sidecar.Warnings = dedupeStrings(sidecar.Warnings)
}

func dedupeSections(items []Section) []Section {
	out := items[:0]
	previous := ""
	for _, item := range items {
		key := sectionKey(item)
		if key != previous {
			out = append(out, item)
			previous = key
		}
	}
	return out
}

func dedupeImports(items []Import) []Import {
	out := items[:0]
	previous := ""
	for _, item := range items {
		key := importKey(item)
		if key != previous {
			out = append(out, item)
			previous = key
		}
	}
	return out
}

func dedupeExports(items []Export) []Export {
	out := items[:0]
	previous := ""
	for _, item := range items {
		key := exportKey(item)
		if key != previous {
			out = append(out, item)
			previous = key
		}
	}
	return out
}

func dedupeStrings(items []string) []string {
	out := items[:0]
	previous := ""
	for _, item := range items {
		if item != previous {
			out = append(out, item)
			previous = item
		}
	}
	return out
}

func sectionKey(section Section) string {
	return fmt.Sprintf("%020d\x00%s\x00%s", section.FileOffset, strings.ToLower(section.Name), section.VirtualAddress)
}

func importKey(item Import) string {
	return strings.ToLower(item.Library) + "\x00" + strings.ToLower(item.Symbol) + "\x00" + item.Version
}

func exportKey(item Export) string {
	return strings.ToLower(item.Name) + "\x00" + item.Address + "\x00" + item.Type
}
