package binaryinventory

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInspectSyntheticPEAndELFMatchGolden(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		path       string
		data       []byte
		goldenFile string
		family     string
	}{
		{name: "pe", path: "inputs/synthetic-pe.bin", data: syntheticPE(), goldenFile: "synthetic-pe.inventory.golden.json", family: "pe"},
		{name: "elf", path: "inputs/synthetic-elf.bin", data: syntheticELF(), goldenFile: "synthetic-elf.inventory.golden.json", family: "elf"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			source, err := BindSource(fixture.path, fixture.data)
			if err != nil {
				t.Fatal(err)
			}
			sidecar, err := Inspect(source, fixture.data)
			if err != nil {
				t.Fatal(err)
			}
			if sidecar.Format.Family != fixture.family || !sidecar.Boundaries.NoSampleExecution || !sidecar.Boundaries.NoCatalogEntryExec {
				t.Fatalf("unexpected sidecar boundary or format: %+v", sidecar)
			}
			data, err := CanonicalBytes(sidecar)
			if err != nil {
				t.Fatal(err)
			}
			goldenPath := filepath.Join("testdata", fixture.goldenFile)
			if os.Getenv("STEAMAI_UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, golden) {
				t.Fatalf("%s inventory differs from golden:\n%s", fixture.name, data)
			}
			decoded, err := Decode(data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, sidecar) {
				t.Fatalf("decoded sidecar differs:\nwant=%+v\ngot=%+v", sidecar, decoded)
			}
		})
	}
}

func TestInspectSyntheticInventoryContainsExpectedShape(t *testing.T) {
	peData := syntheticPE()
	peSource, err := BindSource("inputs/synthetic-pe.bin", peData)
	if err != nil {
		t.Fatal(err)
	}
	peInventory, err := Inspect(peSource, peData)
	if err != nil {
		t.Fatal(err)
	}
	if peInventory.Format.Class != "pe32+" || peInventory.Format.Machine != "amd64" || peInventory.Format.FileType != "executable" || peInventory.Format.EntryPoint != "0x0" || len(peInventory.Sections) != 1 || peInventory.Sections[0].Name != ".text" || peInventory.Sections[0].Permissions != "r-x" {
		t.Fatalf("unexpected PE inventory: %+v", peInventory)
	}

	elfData := syntheticELF()
	elfSource, err := BindSource("inputs/synthetic-elf.bin", elfData)
	if err != nil {
		t.Fatal(err)
	}
	elfInventory, err := Inspect(elfSource, elfData)
	if err != nil {
		t.Fatal(err)
	}
	if elfInventory.Format.Class != "elf64" || elfInventory.Format.Machine != "em_x86_64" || elfInventory.Format.FileType != "et_exec" || elfInventory.Format.EntryPoint != "0x0" || len(elfInventory.Sections) != 3 || elfInventory.Sections[2].Name != ".text" || elfInventory.Sections[2].Permissions != "r-x" {
		t.Fatalf("unexpected ELF inventory: %+v", elfInventory)
	}
}

func TestDecodeRejectsUnknownNonCanonicalAndUnsafeSidecars(t *testing.T) {
	data := syntheticELF()
	source, err := BindSource("inputs/synthetic-elf.bin", data)
	if err != nil {
		t.Fatal(err)
	}
	sidecar, err := Inspect(source, data)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalBytes(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
		want   string
	}{
		{name: "unknown", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"kind":`), []byte(`"unexpected":true,"kind":`), 1)
		}, want: "unknown field"},
		{name: "non-canonical", mutate: func(data []byte) []byte { return bytes.Replace(data, []byte("  \"kind\""), []byte("    \"kind\""), 1) }, want: "not canonical"},
		{name: "trailing", mutate: func(data []byte) []byte { return append(data, []byte("{}\n")...) }, want: "exactly one"},
		{name: "false-boundary", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"noNetwork": true`), []byte(`"noNetwork": false`), 1)
		}, want: "boundaries"},
		{name: "windows-absolute-path", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`inputs/synthetic-elf.bin`), []byte(`C:/synthetic-elf.bin`), 1)
		}, want: "case-relative"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(test.mutate(append([]byte{}, canonical...)))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want %q", err, test.want)
			}
		})
	}
}

func TestInspectRejectsBindingDriftUnsupportedAndTruncatedInput(t *testing.T) {
	data := syntheticPE()
	source, err := BindSource("inputs/synthetic-pe.bin", data)
	if err != nil {
		t.Fatal(err)
	}
	mutated := append([]byte{}, data...)
	mutated[len(mutated)-1] ^= 0xff
	if _, err := Inspect(source, mutated); err == nil || !strings.Contains(err.Error(), "binding") {
		t.Fatalf("binding drift error=%v", err)
	}
	unsupported := []byte("not a binary")
	unsupportedSource, err := BindSource("inputs/unknown.bin", unsupported)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(unsupportedSource, unsupported); err == nil || !strings.Contains(err.Error(), "only PE or ELF") {
		t.Fatalf("unsupported input error=%v", err)
	}
	truncated := []byte{'M', 'Z', 0, 0}
	truncatedSource, err := BindSource("inputs/truncated.bin", truncated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(truncatedSource, truncated); err == nil || !strings.Contains(err.Error(), "parse PE") {
		t.Fatalf("truncated PE error=%v", err)
	}
}

func TestBindSourceRejectsUnsafePathsAndEmptyInput(t *testing.T) {
	for _, path := range []string{"../sample.bin", "/sample.bin", `C:/sample.bin`, `inputs\\sample.bin`, " inputs/sample.bin"} {
		if _, err := BindSource(path, []byte{1}); err == nil || !strings.Contains(err.Error(), "case-relative") {
			t.Fatalf("unsafe path %q error=%v", path, err)
		}
	}
	if _, err := BindSource("inputs/empty.bin", nil); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("empty input error=%v", err)
	}
}

func syntheticPE() []byte {
	const (
		peOffset          = 0x80
		optionalHeaderLen = 0xf0
		sectionOffset     = peOffset + 4 + 20 + optionalHeaderLen
		fileSize          = 0x220
	)
	data := make([]byte, fileSize)
	copy(data[:2], "MZ")
	binary.LittleEndian.PutUint32(data[0x3c:0x40], peOffset)
	copy(data[peOffset:peOffset+4], "PE\x00\x00")
	header := data[peOffset+4 : peOffset+24]
	binary.LittleEndian.PutUint16(header[0:2], 0x8664)
	binary.LittleEndian.PutUint16(header[2:4], 1)
	binary.LittleEndian.PutUint16(header[16:18], optionalHeaderLen)
	binary.LittleEndian.PutUint16(header[18:20], 0x0022)
	optional := data[peOffset+24 : sectionOffset]
	binary.LittleEndian.PutUint16(optional[0:2], 0x20b)
	binary.LittleEndian.PutUint32(optional[16:20], 0)
	binary.LittleEndian.PutUint32(optional[20:24], 0x1000)
	binary.LittleEndian.PutUint64(optional[24:32], 0x140000000)
	binary.LittleEndian.PutUint32(optional[32:36], 0x1000)
	binary.LittleEndian.PutUint32(optional[36:40], 0x200)
	binary.LittleEndian.PutUint32(optional[56:60], 0x2000)
	binary.LittleEndian.PutUint32(optional[60:64], 0x200)
	binary.LittleEndian.PutUint16(optional[68:70], 3)
	binary.LittleEndian.PutUint32(optional[108:112], 16)
	section := data[sectionOffset : sectionOffset+40]
	copy(section[0:8], []byte(".text\x00\x00\x00"))
	binary.LittleEndian.PutUint32(section[8:12], 1)
	binary.LittleEndian.PutUint32(section[12:16], 0x1000)
	binary.LittleEndian.PutUint32(section[16:20], 0x20)
	binary.LittleEndian.PutUint32(section[20:24], 0x200)
	binary.LittleEndian.PutUint32(section[36:40], 0x60000020)
	data[0x200] = 0xc3
	return data
}

func syntheticELF() []byte {
	const (
		sectionHeaderOffset = 0x100
		sectionCount        = 3
		sectionEntrySize    = 64
		fileSize            = sectionHeaderOffset + sectionCount*sectionEntrySize
	)
	data := make([]byte, fileSize)
	copy(data[:4], []byte{0x7f, 'E', 'L', 'F'})
	data[4] = 2
	data[5] = 1
	data[6] = 1
	binary.LittleEndian.PutUint16(data[16:18], 2)
	binary.LittleEndian.PutUint16(data[18:20], 62)
	binary.LittleEndian.PutUint32(data[20:24], 1)
	binary.LittleEndian.PutUint64(data[24:32], 0)
	binary.LittleEndian.PutUint64(data[40:48], sectionHeaderOffset)
	binary.LittleEndian.PutUint16(data[52:54], 64)
	binary.LittleEndian.PutUint16(data[58:60], sectionEntrySize)
	binary.LittleEndian.PutUint16(data[60:62], sectionCount)
	binary.LittleEndian.PutUint16(data[62:64], 1)
	stringTable := []byte("\x00.shstrtab\x00.text\x00")
	copy(data[0x80:], stringTable)
	data[0xa0] = 0xc3
	shstr := data[sectionHeaderOffset+sectionEntrySize : sectionHeaderOffset+2*sectionEntrySize]
	binary.LittleEndian.PutUint32(shstr[0:4], 1)
	binary.LittleEndian.PutUint32(shstr[4:8], 3)
	binary.LittleEndian.PutUint64(shstr[24:32], 0x80)
	binary.LittleEndian.PutUint64(shstr[32:40], uint64(len(stringTable)))
	binary.LittleEndian.PutUint64(shstr[48:56], 1)
	text := data[sectionHeaderOffset+2*sectionEntrySize : sectionHeaderOffset+3*sectionEntrySize]
	binary.LittleEndian.PutUint32(text[0:4], 11)
	binary.LittleEndian.PutUint32(text[4:8], 1)
	binary.LittleEndian.PutUint64(text[8:16], 0x6)
	binary.LittleEndian.PutUint64(text[16:24], 0x401000)
	binary.LittleEndian.PutUint64(text[24:32], 0xa0)
	binary.LittleEndian.PutUint64(text[32:40], 1)
	binary.LittleEndian.PutUint64(text[48:56], 16)
	return data
}
