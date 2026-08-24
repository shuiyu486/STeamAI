package adapterhost

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
)

const (
	vmpIDAIndexPacketSchemaVersion = 1
	vmpIDAIndexPacketKind          = "vmp-ida-index-packet"
)

type VMPIDAIndexSelectedRow struct {
	Line         int      `json:"line"`
	Row          string   `json:"row"`
	MatchedTerms []string `json:"matchedTerms"`
	EvidenceRef  string   `json:"evidenceRef"`
}

type VMPIDAIndexResult struct {
	Name         string                   `json:"name"`
	Source       VMPIDAIndexInputBinding  `json:"source"`
	TotalRows    int                      `json:"totalRows"`
	MatchedRows  int                      `json:"matchedRows"`
	Selected     []VMPIDAIndexSelectedRow `json:"selected"`
	Truncated    bool                     `json:"truncated"`
	DroppedCount int                      `json:"droppedCount"`
}

type VMPIDAIndexPacket struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Kind          string                    `json:"kind"`
	AdapterID     string                    `json:"adapterId"`
	RequestPath   string                    `json:"requestPath"`
	RequestSHA256 string                    `json:"requestSha256"`
	QuerySHA256   string                    `json:"querySha256"`
	Query         VMPIDAIndexQuery          `json:"query"`
	Sources       []VMPIDAIndexInputBinding `json:"sources"`
	Indexes       []VMPIDAIndexResult       `json:"indexes"`
	Warnings      []string                  `json:"warnings"`
	Errors        []string                  `json:"errors"`
	Truncated     bool                      `json:"truncated"`
	DroppedCount  int                       `json:"droppedCount"`
	EvidenceRefs  []string                  `json:"evidenceRefs"`
	NextActions   []string                  `json:"nextActions"`
}

type VMPIDAIndexInspection struct {
	Packet         VMPIDAIndexPacket `json:"packet"`
	PacketSHA256   string            `json:"packetSha256"`
	CanonicalBytes []byte            `json:"-"`
}

type VMPIDAIndexChildOptions struct {
	RepoRoot                   string
	CaseRoot                   string
	Pack                       string
	GateEventID                string
	ExpectedDispatchSHA256     string
	AdapterSession             string
	Executor                   string
	ExpectedExecutorGeneration int
	RequestPath                string
	InstructionIdentity        *instructionpacket.Identity
}

type vmpIDATSVRow struct {
	line int
	raw  string
}

// InspectVMPIDAIndex validates the content-addressed request and its current
// source snapshot, then performs only case-insensitive literal substring
// matching over the four fixed TSV indexes. It does not write any file.
func InspectVMPIDAIndex(caseRoot, requestPath string) (VMPIDAIndexInspection, error) {
	snapshot, err := readVMPIDARequestSnapshot(caseRoot, requestPath)
	if err != nil {
		return VMPIDAIndexInspection{}, err
	}
	request := snapshot.read.Request
	packet := VMPIDAIndexPacket{
		SchemaVersion: vmpIDAIndexPacketSchemaVersion,
		Kind:          vmpIDAIndexPacketKind,
		AdapterID:     VMPIDAIndexAdapterID,
		RequestPath:   snapshot.read.RequestPath,
		RequestSHA256: snapshot.read.RequestSHA256,
		QuerySHA256:   request.QuerySHA256,
		Query:         request.Query,
		Sources:       []VMPIDAIndexInputBinding{},
		Indexes:       []VMPIDAIndexResult{},
		Warnings:      []string{},
		Errors:        []string{},
		EvidenceRefs:  []string{snapshot.read.RequestPath},
		NextActions:   []string{"Review selected literal matches and cite their evidence refs."},
	}

	for _, binding := range request.Inputs {
		result := VMPIDAIndexResult{
			Name:     binding.Name,
			Source:   binding,
			Selected: []VMPIDAIndexSelectedRow{},
		}
		if !binding.Exists {
			packet.Warnings = append(packet.Warnings, fmt.Sprintf("optional VMP IDA index is absent: %s", binding.Path))
			packet.Indexes = append(packet.Indexes, result)
			continue
		}
		packet.Sources = append(packet.Sources, binding)
		_, rows, err := parseVMPIDATSV(snapshot.inputs[binding.Name], binding.Path)
		if err != nil {
			return VMPIDAIndexInspection{}, err
		}
		result.TotalRows = len(rows)
		for _, row := range rows {
			matched := matchingVMPIDATerms(row.raw, request.Query.Terms)
			if len(matched) == 0 {
				continue
			}
			result.MatchedRows++
			if len(result.Selected) >= request.Limits.MaxRowsPerIndex {
				result.Truncated = true
				result.DroppedCount++
				continue
			}
			evidenceRef := fmt.Sprintf("ida-index:%s:%s#L%d", binding.Name, binding.Path, row.line)
			result.Selected = append(result.Selected, VMPIDAIndexSelectedRow{
				Line:         row.line,
				Row:          row.raw,
				MatchedTerms: matched,
				EvidenceRef:  evidenceRef,
			})
		}
		packet.Indexes = append(packet.Indexes, result)
	}

	if err := fitVMPIDAIndexPacket(&packet); err != nil {
		return VMPIDAIndexInspection{}, err
	}
	packet.EvidenceRefs = collectVMPIDAEvidenceRefs(packet)
	packet.Truncated, packet.DroppedCount = summarizeVMPIDATruncation(packet.Indexes)
	if packet.Truncated {
		packet.NextActions = []string{"Narrow the literal terms or inspect a fresh bounded request for dropped matches."}
	}
	data, err := canonicalJSON(packet)
	if err != nil {
		return VMPIDAIndexInspection{}, err
	}
	if len(data) > VMPIDAIndexMaxPacketBytes {
		return VMPIDAIndexInspection{}, fmt.Errorf("VMP IDA index packet exceeds %d bytes after truncation", VMPIDAIndexMaxPacketBytes)
	}
	return VMPIDAIndexInspection{
		Packet:         packet,
		PacketSHA256:   sha256Hex(data),
		CanonicalBytes: append([]byte{}, data...),
	}, nil
}

func fitVMPIDAIndexPacket(packet *VMPIDAIndexPacket) error {
	for {
		packet.EvidenceRefs = collectVMPIDAEvidenceRefs(*packet)
		packet.Truncated, packet.DroppedCount = summarizeVMPIDATruncation(packet.Indexes)
		if packet.Truncated {
			packet.NextActions = []string{"Narrow the literal terms or inspect a fresh bounded request for dropped matches."}
		}
		data, err := canonicalJSON(packet)
		if err != nil {
			return err
		}
		if len(data) <= VMPIDAIndexMaxPacketBytes {
			return nil
		}
		removed := false
		for index := len(packet.Indexes) - 1; index >= 0; index-- {
			result := &packet.Indexes[index]
			if len(result.Selected) == 0 {
				continue
			}
			result.Selected = result.Selected[:len(result.Selected)-1]
			result.DroppedCount++
			result.Truncated = true
			removed = true
			break
		}
		if !removed {
			return fmt.Errorf("VMP IDA index packet fixed metadata exceeds %d bytes", VMPIDAIndexMaxPacketBytes)
		}
	}
}

func collectVMPIDAEvidenceRefs(packet VMPIDAIndexPacket) []string {
	refs := []string{packet.RequestPath}
	for _, result := range packet.Indexes {
		for _, row := range result.Selected {
			refs = append(refs, row.EvidenceRef)
		}
	}
	return refs
}

func summarizeVMPIDATruncation(indexes []VMPIDAIndexResult) (bool, int) {
	truncated := false
	dropped := 0
	for _, result := range indexes {
		truncated = truncated || result.Truncated
		dropped += result.DroppedCount
	}
	return truncated, dropped
}

func matchingVMPIDATerms(row string, terms []string) []string {
	folded := strings.ToLower(row)
	matched := make([]string, 0, len(terms))
	for _, term := range terms {
		if strings.Contains(folded, strings.ToLower(term)) {
			matched = append(matched, term)
		}
	}
	return matched
}

// parseVMPIDATSV uses a fixed-capacity scanner. It requires a non-empty TSV
// header, valid UTF-8, bounded physical lines, and a fixed column count.
func parseVMPIDATSV(data []byte, sourcePath string) ([]string, []vmpIDATSVRow, error) {
	if len(data) < 1 || len(data) > VMPIDAIndexMaxInputBytes {
		return nil, nil, fmt.Errorf("VMP IDA TSV must contain 1-%d bytes: %s", VMPIDAIndexMaxInputBytes, sourcePath)
	}
	if !utf8.Valid(data) {
		return nil, nil, fmt.Errorf("VMP IDA TSV must be valid UTF-8: %s", sourcePath)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), VMPIDAIndexMaxLineBytes+2)
	lineNumber := 0
	var header []string
	rows := []vmpIDATSVRow{}
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if len(line) > VMPIDAIndexMaxLineBytes {
			return nil, nil, fmt.Errorf("VMP IDA TSV line %d exceeds %d bytes: %s", lineNumber, VMPIDAIndexMaxLineBytes, sourcePath)
		}
		if strings.IndexByte(line, 0) >= 0 {
			return nil, nil, fmt.Errorf("VMP IDA TSV line %d contains NUL: %s", lineNumber, sourcePath)
		}
		columns := strings.Split(line, "\t")
		if lineNumber == 1 {
			header = columns
			seen := map[string]bool{}
			for _, column := range header {
				if column == "" || strings.TrimSpace(column) != column || utf8.RuneCountInString(column) > 128 {
					return nil, nil, fmt.Errorf("VMP IDA TSV header has an invalid column at line 1: %s", sourcePath)
				}
				folded := strings.ToLower(column)
				if seen[folded] {
					return nil, nil, fmt.Errorf("VMP IDA TSV header columns must be unique: %s", sourcePath)
				}
				seen[folded] = true
			}
			continue
		}
		if len(columns) != len(header) {
			return nil, nil, fmt.Errorf("VMP IDA TSV line %d has %d columns; header has %d: %s", lineNumber, len(columns), len(header), sourcePath)
		}
		rows = append(rows, vmpIDATSVRow{line: lineNumber, raw: line})
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) || strings.Contains(err.Error(), "token too long") {
			return nil, nil, fmt.Errorf("VMP IDA TSV line exceeds %d bytes: %s", VMPIDAIndexMaxLineBytes, sourcePath)
		}
		return nil, nil, fmt.Errorf("read VMP IDA TSV %s: %w", sourcePath, err)
	}
	if lineNumber == 0 || len(header) == 0 {
		return nil, nil, fmt.Errorf("VMP IDA TSV requires a header: %s", sourcePath)
	}
	return header, rows, nil
}
