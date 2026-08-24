package binaryinventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestPackSchemaPinsGoOwnedInventoryIdentityAndBoundaries(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "packs", "binary-re", "tooling", "schemas", "binary-inventory-v1.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	if properties["schemaVersion"].(map[string]any)["const"] != float64(SchemaVersion) || properties["kind"].(map[string]any)["const"] != Kind || properties["adapterId"].(map[string]any)["const"] != AdapterID {
		t.Fatalf("pack schema identity drifted from Go owner: %+v", properties)
	}
	boundaries := properties["boundaries"].(map[string]any)
	required := jsonStrings(t, boundaries["required"])
	for _, field := range []string{"readOnlyInput", "noSampleExecution", "noNetwork", "noCatalogEntryExecution", "noAuthorityOrConfirmed"} {
		if !slices.Contains(required, field) || boundaries["properties"].(map[string]any)[field].(map[string]any)["const"] != true {
			t.Fatalf("pack schema omitted required true boundary %s: %+v", field, boundaries)
		}
	}
	format := properties["format"].(map[string]any)["properties"].(map[string]any)
	if !slices.Equal(jsonStrings(t, format["family"].(map[string]any)["enum"]), []string{"pe", "elf"}) || !slices.Equal(jsonStrings(t, format["class"].(map[string]any)["enum"]), []string{"pe32", "pe32+", "elf32", "elf64"}) {
		t.Fatalf("pack schema format enums drifted: %+v", format)
	}
}

func jsonStrings(t *testing.T, value any) []string {
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
