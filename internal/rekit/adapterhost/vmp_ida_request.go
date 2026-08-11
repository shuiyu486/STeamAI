package adapterhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

const (
	VMPIDAIndexAdapterID         = "vmp-ida-index-inspector"
	VMPIDAIndexQueryPath         = "tooling/ida-agent-bridge/query.json"
	VMPIDAIndexDefaultExportRoot = "tooling/ida-agent-bridge/export"
	VMPIDAIndexRequestRoot       = "tooling/ida-agent-bridge/requests"

	VMPIDAIndexMaxInputBytes  = 1 << 20
	VMPIDAIndexMaxLineBytes   = 64 << 10
	VMPIDAIndexMaxPacketBytes = 256 << 10
)

const (
	vmpIDARequestSchemaVersion = 1
	vmpIDARequestKind          = "vmp-ida-index-request"
	vmpIDAMaxQueryBytes        = 16 << 10
	vmpIDAMaxRequestBytes      = 64 << 10
	vmpIDADefaultRowsPerIndex  = 50
	vmpIDAMaxRowsPerIndex      = 200
)

var vmpIDAFixedInputs = []struct {
	name     string
	fileName string
	required bool
}{
	{name: "functions", fileName: "function_index.tsv", required: true},
	{name: "strings", fileName: "strings.tsv"},
	{name: "imports", fileName: "imports.tsv"},
	{name: "xrefs", fileName: "xrefs.tsv"},
}

type VMPIDAIndexQuery struct {
	SchemaVersion   int      `json:"schemaVersion"`
	Terms           []string `json:"terms"`
	MaxRowsPerIndex int      `json:"maxRowsPerIndex"`
}

type VMPIDAIndexLimits struct {
	MaxInputBytes   int64 `json:"maxInputBytes"`
	MaxLineBytes    int   `json:"maxLineBytes"`
	MaxPacketBytes  int   `json:"maxPacketBytes"`
	MaxRowsPerIndex int   `json:"maxRowsPerIndex"`
}

type VMPIDAIndexInputBinding struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Required bool   `json:"required"`
	Exists   bool   `json:"exists"`
	SHA256   string `json:"sha256,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
}

type VMPIDAIndexRequest struct {
	SchemaVersion        int                       `json:"schemaVersion"`
	Kind                 string                    `json:"kind"`
	AdapterID            string                    `json:"adapterId"`
	Query                VMPIDAIndexQuery          `json:"query"`
	QuerySHA256          string                    `json:"querySha256"`
	ExportRoot           string                    `json:"exportRoot"`
	Inputs               []VMPIDAIndexInputBinding `json:"inputs"`
	AggregateInputSHA256 string                    `json:"aggregateInputSha256"`
	Limits               VMPIDAIndexLimits         `json:"limits"`
}

type VMPIDAIndexRequestPreview struct {
	Request        VMPIDAIndexRequest `json:"request"`
	RequestPath    string             `json:"requestPath"`
	RequestSHA256  string             `json:"requestSha256"`
	CanonicalBytes []byte             `json:"-"`
}

type VMPIDAIndexRequestPublication struct {
	Request       VMPIDAIndexRequest `json:"request"`
	RequestPath   string             `json:"requestPath"`
	RequestSHA256 string             `json:"requestSha256"`
	Replayed      bool               `json:"replayed"`
}

type VMPIDAIndexRequestRead struct {
	Request        VMPIDAIndexRequest `json:"request"`
	RequestPath    string             `json:"requestPath"`
	RequestSHA256  string             `json:"requestSha256"`
	CanonicalBytes []byte             `json:"-"`
}

type vmpIDARequestSnapshot struct {
	read   VMPIDAIndexRequestRead
	inputs map[string][]byte
}

// PreviewVMPIDAIndexRequest reads the optional convenience query file and
// creates a canonical, content-addressed request without writing the case.
func PreviewVMPIDAIndexRequest(caseRoot, exportRoot string) (VMPIDAIndexRequestPreview, error) {
	queryData, err := readVMPIDAFile(caseRoot, VMPIDAIndexQueryPath, "VMP IDA literal query", vmpIDAMaxQueryBytes)
	if err != nil {
		return VMPIDAIndexRequestPreview{}, err
	}
	var query VMPIDAIndexQuery
	if err := decodeVMPIDAStrictJSON(queryData, &query); err != nil {
		return VMPIDAIndexRequestPreview{}, fmt.Errorf("decode VMP IDA literal query: %w", err)
	}
	return PreviewVMPIDAIndexRequestForQuery(caseRoot, exportRoot, query)
}

// PreviewVMPIDAIndexRequestForQuery is the parent-facing API. It binds a typed
// literal query directly to the exact current input bytes and does not depend
// on a mutable query.json file.
func PreviewVMPIDAIndexRequestForQuery(caseRoot, exportRoot string, query VMPIDAIndexQuery) (VMPIDAIndexRequestPreview, error) {
	exportRoot, err := cleanVMPIDACaseRelative(exportRoot)
	if err != nil {
		return VMPIDAIndexRequestPreview{}, fmt.Errorf("invalid VMP IDA export root: %w", err)
	}
	if query.MaxRowsPerIndex == 0 {
		query.MaxRowsPerIndex = vmpIDADefaultRowsPerIndex
	}
	if err := validateVMPIDAQuery(query); err != nil {
		return VMPIDAIndexRequestPreview{}, err
	}
	queryCanonical, err := canonicalJSON(query)
	if err != nil {
		return VMPIDAIndexRequestPreview{}, err
	}
	bindings, _, err := snapshotVMPIDAInputs(caseRoot, exportRoot)
	if err != nil {
		return VMPIDAIndexRequestPreview{}, err
	}
	aggregateCanonical, err := canonicalJSON(bindings)
	if err != nil {
		return VMPIDAIndexRequestPreview{}, err
	}
	request := VMPIDAIndexRequest{
		SchemaVersion:        vmpIDARequestSchemaVersion,
		Kind:                 vmpIDARequestKind,
		AdapterID:            VMPIDAIndexAdapterID,
		Query:                query,
		QuerySHA256:          sha256Hex(queryCanonical),
		ExportRoot:           exportRoot,
		Inputs:               bindings,
		AggregateInputSHA256: sha256Hex(aggregateCanonical),
		Limits: VMPIDAIndexLimits{
			MaxInputBytes:   VMPIDAIndexMaxInputBytes,
			MaxLineBytes:    VMPIDAIndexMaxLineBytes,
			MaxPacketBytes:  VMPIDAIndexMaxPacketBytes,
			MaxRowsPerIndex: query.MaxRowsPerIndex,
		},
	}
	requestData, err := canonicalJSON(request)
	if err != nil {
		return VMPIDAIndexRequestPreview{}, err
	}
	requestSHA := sha256Hex(requestData)
	return VMPIDAIndexRequestPreview{
		Request:        request,
		RequestPath:    VMPIDAIndexRequestPath(requestSHA),
		RequestSHA256:  requestSHA,
		CanonicalBytes: append([]byte{}, requestData...),
	}, nil
}

// PublishVMPIDAIndexRequest re-snapshots all sources before publishing. The
// content-addressed leaf is create-only; an existing leaf is accepted only when
// its bytes are exactly identical.
func PublishVMPIDAIndexRequest(caseRoot string, preview VMPIDAIndexRequestPreview) (VMPIDAIndexRequestPublication, error) {
	if err := validateVMPIDAPreview(preview); err != nil {
		return VMPIDAIndexRequestPublication{}, err
	}
	current, err := PreviewVMPIDAIndexRequestForQuery(caseRoot, preview.Request.ExportRoot, preview.Request.Query)
	if err != nil {
		return VMPIDAIndexRequestPublication{}, err
	}
	if current.RequestPath != preview.RequestPath || current.RequestSHA256 != preview.RequestSHA256 || !bytes.Equal(current.CanonicalBytes, preview.CanonicalBytes) {
		return VMPIDAIndexRequestPublication{}, fmt.Errorf("VMP IDA query or source snapshot changed before request publication")
	}
	replayed, err := refsf.WriteExclusiveRegularFileAnchored(caseRoot, preview.RequestPath, "VMP IDA index request", preview.CanonicalBytes)
	if err != nil {
		return VMPIDAIndexRequestPublication{}, err
	}
	return VMPIDAIndexRequestPublication{
		Request:       preview.Request,
		RequestPath:   preview.RequestPath,
		RequestSHA256: preview.RequestSHA256,
		Replayed:      replayed,
	}, nil
}

// ReadVMPIDAIndexRequest accepts only canonical content-addressed request files
// and verifies that every current source still matches its exact binding.
func ReadVMPIDAIndexRequest(caseRoot, requestPath string) (VMPIDAIndexRequestRead, error) {
	snapshot, err := readVMPIDARequestSnapshot(caseRoot, requestPath)
	if err != nil {
		return VMPIDAIndexRequestRead{}, err
	}
	return snapshot.read, nil
}

func VMPIDAIndexRequestPath(requestSHA256 string) string {
	return path.Join(VMPIDAIndexRequestRoot, requestSHA256+".json")
}

func readVMPIDAIndexRequestArtifact(caseRoot, requestPath string) (VMPIDAIndexRequestRead, error) {
	requestPath, err := cleanVMPIDACaseRelative(requestPath)
	if err != nil {
		return VMPIDAIndexRequestRead{}, fmt.Errorf("invalid VMP IDA request path: %w", err)
	}
	if path.Dir(requestPath) != VMPIDAIndexRequestRoot {
		return VMPIDAIndexRequestRead{}, fmt.Errorf("VMP IDA request must be in %s", VMPIDAIndexRequestRoot)
	}
	fileName := path.Base(requestPath)
	if path.Ext(fileName) != ".json" {
		return VMPIDAIndexRequestRead{}, fmt.Errorf("VMP IDA request filename must be a semantic SHA-256 JSON name")
	}
	fileSHA := strings.TrimSuffix(fileName, ".json")
	if !validSHA256(fileSHA) || strings.ToLower(fileSHA) != fileSHA {
		return VMPIDAIndexRequestRead{}, fmt.Errorf("VMP IDA request filename must contain a lowercase SHA-256")
	}
	data, err := readVMPIDAFile(caseRoot, requestPath, "VMP IDA index request", vmpIDAMaxRequestBytes)
	if err != nil {
		return VMPIDAIndexRequestRead{}, err
	}
	if got := sha256Hex(data); got != fileSHA {
		return VMPIDAIndexRequestRead{}, fmt.Errorf("VMP IDA request filename hash mismatch: expected %s got %s", fileSHA, got)
	}
	var request VMPIDAIndexRequest
	if err := decodeVMPIDAStrictJSON(data, &request); err != nil {
		return VMPIDAIndexRequestRead{}, fmt.Errorf("decode VMP IDA index request: %w", err)
	}
	if err := validateVMPIDARequest(request); err != nil {
		return VMPIDAIndexRequestRead{}, err
	}
	canonical, err := canonicalJSON(request)
	if err != nil {
		return VMPIDAIndexRequestRead{}, err
	}
	if !bytes.Equal(canonical, data) {
		return VMPIDAIndexRequestRead{}, fmt.Errorf("VMP IDA index request is not strict canonical JSON")
	}
	return VMPIDAIndexRequestRead{
		Request:        request,
		RequestPath:    requestPath,
		RequestSHA256:  fileSHA,
		CanonicalBytes: append([]byte{}, data...),
	}, nil
}

func readVMPIDARequestSnapshot(caseRoot, requestPath string) (vmpIDARequestSnapshot, error) {
	read, err := readVMPIDAIndexRequestArtifact(caseRoot, requestPath)
	if err != nil {
		return vmpIDARequestSnapshot{}, err
	}
	bindings, inputData, err := snapshotVMPIDAInputs(caseRoot, read.Request.ExportRoot)
	if err != nil {
		return vmpIDARequestSnapshot{}, err
	}
	bindingsCanonical, err := canonicalJSON(bindings)
	if err != nil {
		return vmpIDARequestSnapshot{}, err
	}
	if !equalVMPIDAInputBindings(bindings, read.Request.Inputs) ||
		sha256Hex(bindingsCanonical) != read.Request.AggregateInputSHA256 {
		return vmpIDARequestSnapshot{}, fmt.Errorf("VMP IDA source drift detected for request %s", read.RequestPath)
	}
	return vmpIDARequestSnapshot{read: read, inputs: inputData}, nil
}

func snapshotVMPIDAInputs(caseRoot, exportRoot string) ([]VMPIDAIndexInputBinding, map[string][]byte, error) {
	exportPath, err := refsf.SafeJoin(caseRoot, exportRoot)
	if err != nil {
		return nil, nil, err
	}
	if _, err := refsf.ValidateNonReparseDirectory(exportPath, "VMP IDA export root"); err != nil {
		return nil, nil, err
	}
	bindings := make([]VMPIDAIndexInputBinding, 0, len(vmpIDAFixedInputs))
	inputData := make(map[string][]byte, len(vmpIDAFixedInputs))
	for _, fixed := range vmpIDAFixedInputs {
		rel := path.Join(exportRoot, fixed.fileName)
		binding := VMPIDAIndexInputBinding{Name: fixed.name, Path: rel, Required: fixed.required}
		data, readErr := readVMPIDAFile(caseRoot, rel, "VMP IDA "+fixed.name+" index", VMPIDAIndexMaxInputBytes)
		if errors.Is(readErr, os.ErrNotExist) {
			if fixed.required {
				return nil, nil, fmt.Errorf("required VMP IDA input is missing: %s", rel)
			}
			bindings = append(bindings, binding)
			continue
		}
		if readErr != nil {
			return nil, nil, readErr
		}
		if _, _, err := parseVMPIDATSV(data, rel); err != nil {
			return nil, nil, err
		}
		binding.Exists = true
		binding.SHA256 = sha256Hex(data)
		binding.Bytes = int64(len(data))
		bindings = append(bindings, binding)
		inputData[fixed.name] = append([]byte{}, data...)
	}
	return bindings, inputData, nil
}

func validateVMPIDAQuery(query VMPIDAIndexQuery) error {
	if query.SchemaVersion != 1 {
		return fmt.Errorf("VMP IDA query schemaVersion must be 1")
	}
	if len(query.Terms) < 1 || len(query.Terms) > 16 {
		return fmt.Errorf("VMP IDA query requires 1-16 literal terms")
	}
	if query.MaxRowsPerIndex < 1 || query.MaxRowsPerIndex > vmpIDAMaxRowsPerIndex {
		return fmt.Errorf("VMP IDA maxRowsPerIndex must be 1-%d", vmpIDAMaxRowsPerIndex)
	}
	seen := map[string]bool{}
	for _, term := range query.Terms {
		if !utf8.ValidString(term) || strings.TrimSpace(term) != term {
			return fmt.Errorf("VMP IDA literal terms must be valid UTF-8 without surrounding whitespace")
		}
		count := utf8.RuneCountInString(term)
		if count < 1 || count > 128 {
			return fmt.Errorf("VMP IDA literal terms must contain 1-128 UTF-8 characters")
		}
		for _, value := range term {
			if unicode.IsControl(value) || unicode.IsSpace(value) && value != ' ' || strings.ContainsRune(`*?[]{}()|^$+=!<>&;~\\\"'`+"`", value) {
				return fmt.Errorf("VMP IDA terms allow literals only; regex, glob, DSL, controls, and quoting are rejected: %q", term)
			}
		}
		folded := strings.ToLower(term)
		if seen[folded] {
			return fmt.Errorf("VMP IDA literal terms must be unique case-insensitively")
		}
		seen[folded] = true
	}
	return nil
}

func validateVMPIDARequest(request VMPIDAIndexRequest) error {
	if request.SchemaVersion != vmpIDARequestSchemaVersion || request.Kind != vmpIDARequestKind || request.AdapterID != VMPIDAIndexAdapterID {
		return fmt.Errorf("VMP IDA index request schema, kind, or adapter mismatch")
	}
	if err := validateVMPIDAQuery(request.Query); err != nil {
		return err
	}
	exportRoot, err := cleanVMPIDACaseRelative(request.ExportRoot)
	if err != nil || exportRoot != request.ExportRoot {
		return fmt.Errorf("VMP IDA index request export root is not canonical case-relative")
	}
	queryCanonical, err := canonicalJSON(request.Query)
	if err != nil || request.QuerySHA256 != sha256Hex(queryCanonical) {
		return fmt.Errorf("VMP IDA index request query hash mismatch")
	}
	wantLimits := VMPIDAIndexLimits{VMPIDAIndexMaxInputBytes, VMPIDAIndexMaxLineBytes, VMPIDAIndexMaxPacketBytes, request.Query.MaxRowsPerIndex}
	if request.Limits != wantLimits {
		return fmt.Errorf("VMP IDA index request limits are not fixed")
	}
	if len(request.Inputs) != len(vmpIDAFixedInputs) {
		return fmt.Errorf("VMP IDA index request must bind exactly four fixed inputs")
	}
	for index, fixed := range vmpIDAFixedInputs {
		binding := request.Inputs[index]
		if binding.Name != fixed.name || binding.Path != path.Join(request.ExportRoot, fixed.fileName) || binding.Required != fixed.required {
			return fmt.Errorf("VMP IDA index request input binding %d is not fixed", index)
		}
		if binding.Exists {
			if !validSHA256(binding.SHA256) || strings.ToLower(binding.SHA256) != binding.SHA256 || binding.Bytes < 1 || binding.Bytes > VMPIDAIndexMaxInputBytes {
				return fmt.Errorf("VMP IDA index request input binding is invalid: %s", binding.Name)
			}
		} else if binding.Required || binding.SHA256 != "" || binding.Bytes != 0 {
			return fmt.Errorf("VMP IDA index request missing input binding is invalid: %s", binding.Name)
		}
	}
	bindingsCanonical, err := canonicalJSON(request.Inputs)
	if err != nil || request.AggregateInputSHA256 != sha256Hex(bindingsCanonical) {
		return fmt.Errorf("VMP IDA aggregate input hash mismatch")
	}
	return nil
}

func validateVMPIDAPreview(preview VMPIDAIndexRequestPreview) error {
	if err := validateVMPIDARequest(preview.Request); err != nil {
		return err
	}
	canonical, err := canonicalJSON(preview.Request)
	if err != nil {
		return err
	}
	requestSHA := sha256Hex(canonical)
	if preview.RequestSHA256 != requestSHA || preview.RequestPath != VMPIDAIndexRequestPath(requestSHA) || !bytes.Equal(preview.CanonicalBytes, canonical) {
		return fmt.Errorf("VMP IDA request preview is not canonical or content-addressed")
	}
	return nil
}

func readVMPIDAFile(caseRoot, rel, label string, limit int64) ([]byte, error) {
	rel, err := cleanVMPIDACaseRelative(rel)
	if err != nil {
		return nil, fmt.Errorf("%s path is invalid: %w", label, err)
	}
	full, err := refsf.SafeJoin(caseRoot, rel)
	if err != nil {
		return nil, err
	}
	return refsf.ReadStableRegularFileAnchored(caseRoot, full, label, limit)
}

func cleanVMPIDACaseRelative(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(filepath.FromSlash(value)) {
		return "", fmt.Errorf("path must be non-empty and relative")
	}
	clean := path.Clean(filepath.ToSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", fmt.Errorf("path escapes the case root")
	}
	return clean, nil
}

func decodeVMPIDAStrictJSON(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return fmt.Errorf("unexpected trailing data: %w", err)
	}
	return nil
}

func equalVMPIDAInputBindings(left, right []VMPIDAIndexInputBinding) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
