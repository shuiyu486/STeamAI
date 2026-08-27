package manifest

import (
	"strings"
	"testing"
)

func TestParseToolCatalogValidatesTypedIdentityAndRows(t *testing.T) {
	valid := []byte("schemaVersion: 1\npack: binary-re\npurpose: Bounded catalog.\npaths:\n  caseRoot: <caseRoot>\ntools:\n  - id: inspector\n    status: supported\n    entry: compiled-in child\n    purpose: Inspect existing data.\n    sideEffects: filesystem-read\n    gateActions: inspect\n    reusableNotes:\n      - Keep output bounded.\n")
	catalog, err := ParseToolCatalog(valid, "binary-re")
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Pack != "binary-re" || catalog.SchemaVersion != "1" || len(catalog.Tools) != 1 || catalog.Tools[0]["id"] != "inspector" {
		t.Fatalf("unexpected catalog projection: %+v", catalog)
	}

	for _, test := range []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{
			name: "wrong pack",
			mutate: func(text string) string {
				return strings.Replace(text, "pack: binary-re", "pack: web-security", 1)
			},
			want: "pack differs from manifest",
		},
		{
			name: "duplicate top-level identity",
			mutate: func(text string) string {
				return strings.Replace(text, "purpose: Bounded catalog.", "pack: binary-re\npurpose: Bounded catalog.", 1)
			},
			want: "duplicate top-level key",
		},
		{
			name: "malformed nested value",
			mutate: func(text string) string {
				return strings.Replace(text, "    purpose: Inspect existing data.", "    purpose: Inspect existing data.\n    metadata: [", 1)
			},
			want: "unsupported key",
		},
		{
			name: "unsupported indentation",
			mutate: func(text string) string {
				return strings.Replace(text, "    status: supported", "     status: supported", 1)
			},
			want: "unsupported indentation",
		},
		{
			name: "empty notes before another field",
			mutate: func(text string) string {
				return strings.Replace(text, "    reusableNotes:\n      - Keep output bounded.", "    reusableNotes:\n    gateActions: inspect", 1)
			},
			want: "empty reusableNotes",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseToolCatalog([]byte(test.mutate(string(valid))), "binary-re"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseToolCatalog error = %v, want contains %q", err, test.want)
			}
		})
	}
}
