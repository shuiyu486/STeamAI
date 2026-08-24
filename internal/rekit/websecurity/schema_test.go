package websecurity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestPackSchemasPinGoOwnedWebSecurityIdentityAndBoundaries(t *testing.T) {
	root := webSecurityRepositoryRoot(t)
	for _, test := range []struct {
		file       string
		kind       string
		adapterID  string
		boundaries []string
	}{
		{file: "openapi-inventory-v1.schema.json", kind: InventoryKind, adapterID: InventoryAdapterID, boundaries: []string{"readOnlyInput", "noNetwork", "noSecretsPersisted", "noCatalogEntryExecution", "noAuthorityOrConfirmed"}},
		{file: "bounded-replay-request-v1.schema.json", kind: ReplayRequestKind, adapterID: ReplayAdapterID, boundaries: []string{"loopbackOnly", "noAmbientProxy", "noRedirects", "noRetries", "noRequestBody", "noSecretsPersisted", "noCatalogEntryExecution", "noAuthorityOrConfirmed"}},
		{file: "bounded-replay-result-v1.schema.json", kind: ReplayResultKind, adapterID: ReplayAdapterID, boundaries: []string{"loopbackOnly", "noAmbientProxy", "noRedirects", "noRetries", "noRequestBody", "noSecretsPersisted", "noCatalogEntryExecution", "noAuthorityOrConfirmed"}},
	} {
		t.Run(test.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "packs", "web-security", "tooling", "schemas", test.file))
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatal(err)
			}
			properties := schema["properties"].(map[string]any)
			if properties["schemaVersion"].(map[string]any)["const"] != float64(SchemaVersion) || properties["kind"].(map[string]any)["const"] != test.kind || properties["adapterId"].(map[string]any)["const"] != test.adapterID {
				t.Fatalf("schema identity drifted: %+v", properties)
			}
			boundary := properties["boundaries"].(map[string]any)
			required := webSecurityJSONStrings(t, boundary["required"])
			for _, field := range test.boundaries {
				if !slices.Contains(required, field) || boundary["properties"].(map[string]any)[field].(map[string]any)["const"] != true {
					t.Fatalf("schema omitted required true boundary %s: %+v", field, boundary)
				}
			}
		})
	}
}

func webSecurityRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func webSecurityJSONStrings(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("schema value is not an array: %#v", value)
	}
	out := make([]string, len(items))
	for index, item := range items {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("schema array item is not a string: %#v", item)
		}
		out[index] = text
	}
	return out
}
