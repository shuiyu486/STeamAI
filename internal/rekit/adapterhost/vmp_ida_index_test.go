package adapterhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectVMPIDAIndexMatchesLiteralsCaseInsensitively(t *testing.T) {
	caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["virtualalloc","DISPATCHER"],"maxRowsPerIndex":5}`, map[string]string{
		"function_index.tsv": "rva\tname\tsize\n0x1000\tDispatcherMain\t32\n0x1100\tworker\t16\n",
		"strings.tsv":        "address\tvalue\n0x2000\tCalls VirtualAlloc here\n0x2010\tnothing\n",
		"imports.tsv":        "module\tname\nKERNEL32.dll\tVirtualAlloc\n",
		"xrefs.tsv":          "from\tto\n0x1000\tDispatcherMain\n",
	})
	preview := publishVMPIDATestRequest(t, caseRoot)
	inspection, err := InspectVMPIDAIndex(caseRoot, preview.RequestPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := sha256Hex(inspection.CanonicalBytes); got != inspection.PacketSHA256 {
		t.Fatalf("packet SHA = %q, want %q", inspection.PacketSHA256, got)
	}
	if len(inspection.CanonicalBytes) > VMPIDAIndexMaxPacketBytes {
		t.Fatalf("packet bytes = %d", len(inspection.CanonicalBytes))
	}
	if filepath.IsAbs(inspection.Packet.RequestPath) || strings.Contains(string(inspection.CanonicalBytes), filepath.ToSlash(caseRoot)) {
		t.Fatalf("packet leaked an absolute case path: %s", inspection.CanonicalBytes)
	}
	if len(inspection.Packet.Sources) != 4 || len(inspection.Packet.Indexes) != 4 {
		t.Fatalf("sources/indexes = %d/%d", len(inspection.Packet.Sources), len(inspection.Packet.Indexes))
	}
	wantMatches := map[string]string{
		"functions": "DISPATCHER",
		"strings":   "virtualalloc",
		"imports":   "virtualalloc",
		"xrefs":     "DISPATCHER",
	}
	for _, result := range inspection.Packet.Indexes {
		if len(result.Selected) != 1 {
			t.Fatalf("%s selected = %#v", result.Name, result.Selected)
		}
		row := result.Selected[0]
		if row.Line != 2 || len(row.MatchedTerms) != 1 || row.MatchedTerms[0] != wantMatches[result.Name] {
			t.Fatalf("%s selected row = %#v", result.Name, row)
		}
		if !strings.HasSuffix(row.EvidenceRef, "#L2") || filepath.IsAbs(row.EvidenceRef) {
			t.Fatalf("evidence ref = %q", row.EvidenceRef)
		}
	}
	if inspection.Packet.Truncated || inspection.Packet.DroppedCount != 0 || len(inspection.Packet.Errors) != 0 {
		t.Fatalf("unexpected packet status: %#v", inspection.Packet)
	}
}

func TestInspectVMPIDAIndexOptionalWarningsAndPerIndexTruncation(t *testing.T) {
	caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":2}`, map[string]string{
		"function_index.tsv": "rva\tname\n0x1000\tneedle-a\n0x1001\tNEEDLE-b\n0x1002\tneedle-c\n0x1003\tother\n",
	})
	preview := publishVMPIDATestRequest(t, caseRoot)
	inspection, err := InspectVMPIDAIndex(caseRoot, preview.RequestPath)
	if err != nil {
		t.Fatal(err)
	}
	functions := inspection.Packet.Indexes[0]
	if functions.TotalRows != 4 || functions.MatchedRows != 3 || len(functions.Selected) != 2 || !functions.Truncated || functions.DroppedCount != 1 {
		t.Fatalf("function result = %#v", functions)
	}
	if !inspection.Packet.Truncated || inspection.Packet.DroppedCount != 1 || len(inspection.Packet.Warnings) != 3 {
		t.Fatalf("packet truncation/warnings = %#v", inspection.Packet)
	}
	for index := 1; index < 4; index++ {
		if inspection.Packet.Indexes[index].Source.Exists {
			t.Fatalf("optional source unexpectedly exists: %#v", inspection.Packet.Indexes[index])
		}
	}
}

func TestVMPIDATSVRequiresHeaderAndBoundsLines(t *testing.T) {
	for name, content := range map[string][]byte{
		"empty":           {},
		"invalid utf8":    {0xff, '\n'},
		"empty header":    []byte("\n"),
		"duplicate":       []byte("name\tNAME\nvalue\tvalue\n"),
		"column mismatch": []byte("a\tb\nonly-one\n"),
		"oversized line":  append([]byte("name\n"), append(bytes.Repeat([]byte("x"), VMPIDAIndexMaxLineBytes+1), '\n')...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parseVMPIDATSV(content, "tooling/ida-agent-bridge/export/function_index.tsv"); err == nil {
				t.Fatalf("parse accepted %s", name)
			}
		})
	}

	caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1}`, map[string]string{
		"function_index.tsv": "name\n" + strings.Repeat("x", VMPIDAIndexMaxLineBytes+1) + "\n",
	})
	if _, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot); err == nil || !strings.Contains(err.Error(), "line") {
		t.Fatalf("Preview error = %v, want oversized line", err)
	}
}

func TestInspectVMPIDAIndexTruncatesToPacketLimit(t *testing.T) {
	longValue := strings.Repeat("X", 4000) + "needle"
	var rows strings.Builder
	rows.WriteString("rva\tname\n")
	for index := range 100 {
		fmt.Fprintf(&rows, "0x%04x\t%s-%03d\n", index, longValue, index)
	}
	caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":200}`, map[string]string{
		"function_index.tsv": rows.String(),
	})
	preview := publishVMPIDATestRequest(t, caseRoot)
	inspection, err := InspectVMPIDAIndex(caseRoot, preview.RequestPath)
	if err != nil {
		t.Fatal(err)
	}
	functions := inspection.Packet.Indexes[0]
	if len(inspection.CanonicalBytes) > VMPIDAIndexMaxPacketBytes {
		t.Fatalf("packet size = %d, limit = %d", len(inspection.CanonicalBytes), VMPIDAIndexMaxPacketBytes)
	}
	if !inspection.Packet.Truncated || !functions.Truncated || functions.DroppedCount < 1 || inspection.Packet.DroppedCount != functions.DroppedCount {
		t.Fatalf("packet size truncation not reported: packet=%#v functions=%#v", inspection.Packet, functions)
	}
	if got, want := len(functions.Selected)+functions.DroppedCount, functions.MatchedRows; got != want {
		t.Fatalf("selected+dropped = %d, matched = %d", got, want)
	}
}

func TestPreviewVMPIDAIndexRequestForQueryDoesNotRequireQueryFile(t *testing.T) {
	caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["unused"],"maxRowsPerIndex":1}`, map[string]string{
		"function_index.tsv": "rva\tname\n0x1000\tneedle\n",
	})
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(VMPIDAIndexQueryPath))); err != nil {
		t.Fatal(err)
	}
	query := VMPIDAIndexQuery{SchemaVersion: 1, Terms: []string{"needle"}, MaxRowsPerIndex: 3}
	preview, err := PreviewVMPIDAIndexRequestForQuery(caseRoot, VMPIDAIndexDefaultExportRoot, query)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Request.Query.Terms[0] != "needle" || preview.Request.Limits.MaxRowsPerIndex != 3 {
		t.Fatalf("direct query preview = %#v", preview.Request)
	}
	if _, err := PublishVMPIDAIndexRequest(caseRoot, preview); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectVMPIDAIndex(caseRoot, preview.RequestPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := inspection.Packet.Indexes[0].Selected; len(got) != 1 || !strings.Contains(got[0].Row, "needle") {
		t.Fatalf("direct query selected = %#v", got)
	}
}

func TestInspectVMPIDAIndexRejectsNonCanonicalRequestAndSourceDrift(t *testing.T) {
	caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1}`, map[string]string{
		"function_index.tsv": "rva\tname\n0x1000\tneedle\n",
	})
	preview := publishVMPIDATestRequest(t, caseRoot)

	var request VMPIDAIndexRequest
	if err := json.Unmarshal(preview.CanonicalBytes, &request); err != nil {
		t.Fatal(err)
	}
	nonCanonical, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	writeContentAddressedVMPIDARequest(t, caseRoot, nonCanonical)
	if _, err := InspectVMPIDAIndex(caseRoot, VMPIDAIndexRequestPath(sha256Hex(nonCanonical))); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("Inspect error = %v, want canonical rejection", err)
	}

	functionPath := filepath.Join(caseRoot, filepath.FromSlash(pathJoinVMPIDATest(VMPIDAIndexDefaultExportRoot, "function_index.tsv")))
	if err := os.WriteFile(functionPath, []byte("rva\tname\n0x1000\tdrifted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectVMPIDAIndex(caseRoot, preview.RequestPath); err == nil || !strings.Contains(err.Error(), "drift") {
		t.Fatalf("Inspect error = %v, want source drift", err)
	}
}
