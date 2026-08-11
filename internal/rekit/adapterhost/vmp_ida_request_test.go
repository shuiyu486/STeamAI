package adapterhost

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVMPIDAIndexRequestCanonicalContentAddressedAndExactReplay(t *testing.T) {
	caseRoot := newVMPIDATestCase(t, `{
  "terms": ["VirtualAlloc", "dispatcher"],
  "maxRowsPerIndex": 2,
  "schemaVersion": 1
}
`, map[string]string{
		"function_index.tsv": "rva\tname\n0x1000\tdispatcher\n",
		"strings.tsv":        "address\tvalue\n0x2000\tVirtualAlloc marker\n",
	})

	first, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot)
	if err != nil {
		t.Fatal(err)
	}
	if first.Request.AdapterID != VMPIDAIndexAdapterID {
		t.Fatalf("adapter ID = %q", first.Request.AdapterID)
	}
	if got, want := first.RequestPath, VMPIDAIndexRequestPath(first.RequestSHA256); got != want {
		t.Fatalf("request path = %q, want %q", got, want)
	}
	if got := sha256Hex(first.CanonicalBytes); got != first.RequestSHA256 {
		t.Fatalf("canonical SHA = %q, want %q", got, first.RequestSHA256)
	}
	if filepath.IsAbs(first.Request.ExportRoot) || strings.Contains(string(first.CanonicalBytes), filepath.ToSlash(caseRoot)) {
		t.Fatalf("request leaked an absolute case path: %s", first.CanonicalBytes)
	}
	if len(first.Request.Inputs) != 4 || !first.Request.Inputs[0].Exists || !first.Request.Inputs[1].Exists || first.Request.Inputs[2].Exists || first.Request.Inputs[3].Exists {
		t.Fatalf("fixed input bindings = %#v", first.Request.Inputs)
	}
	if !validSHA256(first.Request.AggregateInputSHA256) {
		t.Fatalf("aggregate input hash = %q", first.Request.AggregateInputSHA256)
	}

	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(VMPIDAIndexQueryPath)), []byte(`{"schemaVersion":1,"maxRowsPerIndex":2,"terms":["VirtualAlloc","dispatcher"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.CanonicalBytes, second.CanonicalBytes) || first.RequestSHA256 != second.RequestSHA256 {
		t.Fatalf("semantic query formatting changed request identity")
	}

	published, err := PublishVMPIDAIndexRequest(caseRoot, first)
	if err != nil {
		t.Fatal(err)
	}
	if published.Replayed {
		t.Fatal("first publication reported replay")
	}
	replayed, err := PublishVMPIDAIndexRequest(caseRoot, second)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed {
		t.Fatal("exact publication did not report replay")
	}

	read, err := ReadVMPIDAIndexRequest(caseRoot, first.RequestPath)
	if err != nil {
		t.Fatal(err)
	}
	if read.RequestSHA256 != first.RequestSHA256 || !bytes.Equal(read.CanonicalBytes, first.CanonicalBytes) {
		t.Fatalf("read request differs from preview")
	}
}

func TestVMPIDAIndexRequestRejectsExclusiveDifferentBytes(t *testing.T) {
	caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1}`, map[string]string{
		"function_index.tsv": "rva\tname\n0x1000\tneedle\n",
	})
	preview, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot)
	if err != nil {
		t.Fatal(err)
	}
	requestFull := filepath.Join(caseRoot, filepath.FromSlash(preview.RequestPath))
	if err := os.MkdirAll(filepath.Dir(requestFull), 0o700); err != nil {
		t.Fatal(err)
	}
	different := []byte("different request bytes\n")
	if err := os.WriteFile(requestFull, different, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishVMPIDAIndexRequest(caseRoot, preview); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("Publish error = %v, want exact replay rejection", err)
	}
	got, err := os.ReadFile(requestFull)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, different) {
		t.Fatalf("different existing bytes were overwritten")
	}
}

func TestVMPIDAIndexRequestRequiredOptionalAndStrictJSON(t *testing.T) {
	t.Run("required input", func(t *testing.T) {
		caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1}`, nil)
		if _, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot); err == nil || !strings.Contains(err.Error(), "required") {
			t.Fatalf("Preview error = %v, want required input failure", err)
		}
	})

	for name, query := range map[string]string{
		"unknown":  `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1,"regex":true}`,
		"trailing": `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			caseRoot := newVMPIDATestCase(t, query, map[string]string{"function_index.tsv": "rva\tname\n"})
			if _, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot); err == nil {
				t.Fatal("Preview accepted non-strict query JSON")
			}
		})
	}

	t.Run("request unknown and trailing", func(t *testing.T) {
		caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1}`, map[string]string{"function_index.tsv": "rva\tname\n"})
		preview, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]any
		if err := json.Unmarshal(preview.CanonicalBytes, &raw); err != nil {
			t.Fatal(err)
		}
		raw["unknown"] = true
		data, err := canonicalJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		writeContentAddressedVMPIDARequest(t, caseRoot, data)
		if _, err := ReadVMPIDAIndexRequest(caseRoot, VMPIDAIndexRequestPath(sha256Hex(data))); err == nil {
			t.Fatal("Read accepted an unknown request field")
		}

		trailing := append(append([]byte{}, preview.CanonicalBytes...), []byte("{}\n")...)
		writeContentAddressedVMPIDARequest(t, caseRoot, trailing)
		if _, err := ReadVMPIDAIndexRequest(caseRoot, VMPIDAIndexRequestPath(sha256Hex(trailing))); err == nil {
			t.Fatal("Read accepted trailing request data")
		}
	})
}

func TestVMPIDAIndexRequestLiteralRulesAndLimits(t *testing.T) {
	invalidTerms := []string{"", " needle", "needle*", "foo|bar", "foo(bar)", "foo\nbar", strings.Repeat("x", 129)}
	for _, term := range invalidTerms {
		t.Run(strings.ReplaceAll(term, "\n", "newline"), func(t *testing.T) {
			query, err := json.Marshal(VMPIDAIndexQuery{SchemaVersion: 1, Terms: []string{term}, MaxRowsPerIndex: 1})
			if err != nil {
				t.Fatal(err)
			}
			caseRoot := newVMPIDATestCase(t, string(query), map[string]string{"function_index.tsv": "rva\tname\n"})
			if _, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot); err == nil {
				t.Fatalf("Preview accepted invalid literal term %q", term)
			}
		})
	}

	caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":201}`, map[string]string{"function_index.tsv": "rva\tname\n"})
	if _, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot); err == nil {
		t.Fatal("Preview accepted maxRowsPerIndex above 200")
	}
}

func TestVMPIDAIndexRequestDetectsSourceDriftAndOversizedFile(t *testing.T) {
	t.Run("source drift", func(t *testing.T) {
		caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1}`, map[string]string{
			"function_index.tsv": "rva\tname\n0x1000\tneedle\n",
		})
		preview, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := PublishVMPIDAIndexRequest(caseRoot, preview); err != nil {
			t.Fatal(err)
		}
		functionPath := filepath.Join(caseRoot, filepath.FromSlash(pathJoinVMPIDATest(VMPIDAIndexDefaultExportRoot, "function_index.tsv")))
		if err := os.WriteFile(functionPath, []byte("rva\tname\n0x2000\tchanged\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadVMPIDAIndexRequest(caseRoot, preview.RequestPath); err == nil || !strings.Contains(err.Error(), "drift") {
			t.Fatalf("Read error = %v, want source drift", err)
		}
		if _, err := PublishVMPIDAIndexRequest(caseRoot, preview); err == nil || !strings.Contains(err.Error(), "changed") {
			t.Fatalf("Publish error = %v, want pre-publication source drift", err)
		}
	})

	t.Run("oversized file", func(t *testing.T) {
		caseRoot := newVMPIDATestCase(t, `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":1}`, map[string]string{
			"function_index.tsv": "rva\tname\n",
		})
		oversized := append([]byte("rva\tname\n"), bytes.Repeat([]byte("x"), VMPIDAIndexMaxInputBytes)...)
		if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(pathJoinVMPIDATest(VMPIDAIndexDefaultExportRoot, "function_index.tsv"))), oversized, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot); err == nil {
			t.Fatal("Preview accepted an input larger than 1 MiB")
		}
	})
}

func newVMPIDATestCase(t *testing.T, query string, inputs map[string]string) string {
	t.Helper()
	caseRoot := t.TempDir()
	exportRoot := filepath.Join(caseRoot, filepath.FromSlash(VMPIDAIndexDefaultExportRoot))
	if err := os.MkdirAll(exportRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	queryPath := filepath.Join(caseRoot, filepath.FromSlash(VMPIDAIndexQueryPath))
	if err := os.MkdirAll(filepath.Dir(queryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queryPath, []byte(query), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, content := range inputs {
		if err := os.WriteFile(filepath.Join(exportRoot, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return caseRoot
}

func publishVMPIDATestRequest(t *testing.T, caseRoot string) VMPIDAIndexRequestPreview {
	t.Helper()
	preview, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishVMPIDAIndexRequest(caseRoot, preview); err != nil {
		t.Fatal(err)
	}
	return preview
}

func writeContentAddressedVMPIDARequest(t *testing.T, caseRoot string, data []byte) {
	t.Helper()
	rel := VMPIDAIndexRequestPath(sha256Hex(data))
	full := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func pathJoinVMPIDATest(parts ...string) string {
	return strings.Join(parts, "/")
}
