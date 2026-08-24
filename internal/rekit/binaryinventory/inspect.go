package binaryinventory

import (
	"bytes"
	"debug/elf"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

func Inspect(source SourceBinding, data []byte) (Sidecar, error) {
	if err := validateSource(source); err != nil {
		return Sidecar{}, err
	}
	if len(data) != int(source.Bytes) || !strings.EqualFold(SHA256(data), source.SHA256) {
		return Sidecar{}, fmt.Errorf("binary inventory source binding does not match input bytes")
	}
	if len(data) > MaxInputBytes {
		return Sidecar{}, fmt.Errorf("binary inventory source exceeds %d bytes", MaxInputBytes)
	}

	sidecar := Sidecar{
		SchemaVersion: SchemaVersion,
		Kind:          Kind,
		AdapterID:     AdapterID,
		Source:        source,
		Sections:      []Section{},
		Imports:       []Import{},
		Exports:       []Export{},
		Warnings:      []string{},
		Boundaries: Boundaries{
			ReadOnlyInput:        true,
			NoSampleExecution:    true,
			NoNetwork:            true,
			NoCatalogEntryExec:   true,
			NoAuthorityConfirmed: true,
		},
	}

	var err error
	switch {
	case len(data) >= 2 && data[0] == 'M' && data[1] == 'Z':
		err = inspectPE(&sidecar, data)
	case len(data) >= 4 && bytes.Equal(data[:4], []byte{0x7f, 'E', 'L', 'F'}):
		err = inspectELF(&sidecar, data)
	default:
		return Sidecar{}, fmt.Errorf("binary inventory supports only PE or ELF input")
	}
	if err != nil {
		return Sidecar{}, err
	}
	normalize(&sidecar)
	if err := Validate(sidecar); err != nil {
		return Sidecar{}, err
	}
	return sidecar, nil
}

func inspectPE(sidecar *Sidecar, data []byte) error {
	file, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse PE input: %w", err)
	}
	defer file.Close()

	format := FormatInventory{
		Family:     "pe",
		Endianness: "little",
		Machine:    peMachine(file.Machine),
		FileType:   peFileType(file.Characteristics),
	}
	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		format.Class = "pe32"
		format.Bitness = 32
		format.EntryPoint = hexValue(uint64(header.AddressOfEntryPoint))
		format.ImageBase = hexValue(uint64(header.ImageBase))
	case *pe.OptionalHeader64:
		format.Class = "pe32+"
		format.Bitness = 64
		format.EntryPoint = hexValue(uint64(header.AddressOfEntryPoint))
		format.ImageBase = hexValue(header.ImageBase)
	default:
		return fmt.Errorf("PE input has an unsupported optional header")
	}
	sidecar.Format = format

	if len(file.Sections) > MaxSections {
		return fmt.Errorf("PE input has %d sections; limit is %d", len(file.Sections), MaxSections)
	}
	for index, section := range file.Sections {
		name := boundedText(section.Name)
		if name == "" {
			name = fmt.Sprintf("<section-%d>", index)
		}
		sidecar.Sections = append(sidecar.Sections, Section{
			Name:           name,
			Type:           peSectionType(section.Characteristics),
			VirtualAddress: hexValue(uint64(section.VirtualAddress)),
			VirtualSize:    hexValue(uint64(section.VirtualSize)),
			FileOffset:     int64(section.Offset),
			FileSize:       int64(section.Size),
			Permissions:    pePermissions(section.Characteristics),
		})
	}

	libraries, librariesErr := file.ImportedLibraries()
	symbols, symbolsErr := file.ImportedSymbols()
	if librariesErr != nil && !errors.Is(librariesErr, io.EOF) {
		sidecar.Warnings = append(sidecar.Warnings, "PE import library table could not be decoded")
	}
	if symbolsErr != nil && !errors.Is(symbolsErr, io.EOF) {
		sidecar.Warnings = append(sidecar.Warnings, "PE import symbol table could not be decoded")
	}
	for _, library := range libraries {
		if err := appendImport(sidecar, Import{Library: boundedText(library)}); err != nil {
			return err
		}
	}
	for _, raw := range symbols {
		symbol, library := splitPEImport(raw)
		if err := appendImport(sidecar, Import{Library: boundedText(library), Symbol: boundedText(symbol)}); err != nil {
			return err
		}
	}
	if len(file.Symbols) > 0 {
		sidecar.Warnings = append(sidecar.Warnings, "PE COFF symbols are metadata only and are not reported as exports")
	}
	exports, exportErr := peExports(file, data)
	if exportErr != nil {
		sidecar.Warnings = append(sidecar.Warnings, "PE export directory could not be decoded")
	} else {
		for _, item := range exports {
			if err := appendExport(sidecar, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func peExports(file *pe.File, data []byte) ([]Export, error) {
	var directory pe.DataDirectory
	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		directory = header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_EXPORT]
	case *pe.OptionalHeader64:
		directory = header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_EXPORT]
	default:
		return nil, fmt.Errorf("unsupported PE optional header")
	}
	if directory.VirtualAddress == 0 || directory.Size == 0 {
		return []Export{}, nil
	}
	directoryOffset, err := peRVAOffset(file, directory.VirtualAddress)
	if err != nil || directoryOffset+40 > uint64(len(data)) {
		return nil, fmt.Errorf("PE export directory is outside the input")
	}
	read32 := func(offset uint64) (uint32, error) {
		if offset+4 > uint64(len(data)) {
			return 0, io.ErrUnexpectedEOF
		}
		return binary.LittleEndian.Uint32(data[offset : offset+4]), nil
	}
	base, err := read32(directoryOffset + 16)
	if err != nil {
		return nil, err
	}
	functionCount, err := read32(directoryOffset + 20)
	if err != nil {
		return nil, err
	}
	nameCount, err := read32(directoryOffset + 24)
	if err != nil {
		return nil, err
	}
	if functionCount > MaxExports || nameCount > MaxExports {
		return nil, fmt.Errorf("PE export count exceeds %d", MaxExports)
	}
	functionsRVA, err := read32(directoryOffset + 28)
	if err != nil {
		return nil, err
	}
	namesRVA, err := read32(directoryOffset + 32)
	if err != nil {
		return nil, err
	}
	ordinalsRVA, err := read32(directoryOffset + 36)
	if err != nil {
		return nil, err
	}
	functionsOffset, err := peRVAOffset(file, functionsRVA)
	if err != nil {
		return nil, err
	}
	namesOffset, err := peRVAOffset(file, namesRVA)
	if err != nil {
		return nil, err
	}
	ordinalsOffset, err := peRVAOffset(file, ordinalsRVA)
	if err != nil {
		return nil, err
	}
	out := make([]Export, 0, nameCount)
	for index := range nameCount {
		nameRVA, err := read32(namesOffset + uint64(index)*4)
		if err != nil {
			return nil, err
		}
		nameOffset, err := peRVAOffset(file, nameRVA)
		if err != nil {
			return nil, err
		}
		name, err := peCString(data, nameOffset)
		if err != nil {
			return nil, err
		}
		ordinalOffset := ordinalsOffset + uint64(index)*2
		if ordinalOffset+2 > uint64(len(data)) {
			return nil, io.ErrUnexpectedEOF
		}
		functionIndex := uint32(binary.LittleEndian.Uint16(data[ordinalOffset : ordinalOffset+2]))
		if functionIndex >= functionCount {
			return nil, fmt.Errorf("PE export ordinal is outside the function table")
		}
		functionRVA, err := read32(functionsOffset + uint64(functionIndex)*4)
		if err != nil {
			return nil, err
		}
		item := Export{Name: boundedText(name), Type: "export", Ordinal: base + functionIndex, Address: hexValue(uint64(functionRVA))}
		if functionRVA >= directory.VirtualAddress && uint64(functionRVA) < uint64(directory.VirtualAddress)+uint64(directory.Size) {
			forwarderOffset, err := peRVAOffset(file, functionRVA)
			if err != nil {
				return nil, err
			}
			item.Forwarder, err = peCString(data, forwarderOffset)
			if err != nil {
				return nil, err
			}
			item.Forwarder = boundedText(item.Forwarder)
		}
		out = append(out, item)
	}
	return out, nil
}

func peRVAOffset(file *pe.File, rva uint32) (uint64, error) {
	for _, section := range file.Sections {
		start := uint64(section.VirtualAddress)
		size := max(uint64(section.VirtualSize), uint64(section.Size))
		value := uint64(rva)
		if value >= start && value < start+size {
			return uint64(section.Offset) + value - start, nil
		}
	}
	return 0, fmt.Errorf("PE RVA %#x is not mapped by a section", rva)
}

func peCString(data []byte, offset uint64) (string, error) {
	if offset >= uint64(len(data)) {
		return "", io.ErrUnexpectedEOF
	}
	limit := min(uint64(len(data)), offset+MaxStringBytes+1)
	for index := offset; index < limit; index++ {
		if data[index] == 0 {
			return string(data[offset:index]), nil
		}
	}
	return "", fmt.Errorf("PE string is missing a bounded terminator")
}

func inspectELF(sidecar *Sidecar, data []byte) error {
	file, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse ELF input: %w", err)
	}
	defer file.Close()

	bitness := 0
	class := ""
	switch file.Class {
	case elf.ELFCLASS32:
		bitness = 32
		class = "elf32"
	case elf.ELFCLASS64:
		bitness = 64
		class = "elf64"
	default:
		return fmt.Errorf("ELF input has unsupported class: %s", file.Class)
	}
	endianness := ""
	switch file.Data {
	case elf.ELFDATA2LSB:
		endianness = "little"
	case elf.ELFDATA2MSB:
		endianness = "big"
	default:
		return fmt.Errorf("ELF input has unsupported byte order: %s", file.Data)
	}
	sidecar.Format = FormatInventory{
		Family:     "elf",
		Class:      class,
		Bitness:    bitness,
		Endianness: endianness,
		Machine:    strings.ToLower(file.Machine.String()),
		FileType:   strings.ToLower(file.Type.String()),
		EntryPoint: hexValue(file.Entry),
	}

	if len(file.Sections) > MaxSections {
		return fmt.Errorf("ELF input has %d sections; limit is %d", len(file.Sections), MaxSections)
	}
	for index, section := range file.Sections {
		name := boundedText(section.Name)
		if name == "" {
			name = fmt.Sprintf("<section-%d>", index)
		}
		sidecar.Sections = append(sidecar.Sections, Section{
			Name:           name,
			Type:           strings.ToLower(section.Type.String()),
			VirtualAddress: hexValue(section.Addr),
			VirtualSize:    hexValue(section.Size),
			FileOffset:     int64(section.Offset),
			FileSize:       int64(section.FileSize),
			Permissions:    elfPermissions(section.Flags),
		})
	}

	libraries, librariesErr := file.ImportedLibraries()
	if librariesErr != nil && !errors.Is(librariesErr, elf.ErrNoSymbols) {
		sidecar.Warnings = append(sidecar.Warnings, "ELF imported library table could not be decoded")
	}
	for _, library := range libraries {
		if err := appendImport(sidecar, Import{Library: boundedText(library)}); err != nil {
			return err
		}
	}
	imports, importsErr := file.ImportedSymbols()
	if importsErr != nil && !errors.Is(importsErr, elf.ErrNoSymbols) {
		sidecar.Warnings = append(sidecar.Warnings, "ELF imported symbol table could not be decoded")
	}
	for _, symbol := range imports {
		library := symbol.Library
		if strings.TrimSpace(library) == "" {
			library = "<unresolved>"
		}
		if err := appendImport(sidecar, Import{Library: boundedText(library), Symbol: boundedText(symbol.Name), Version: boundedText(symbol.Version)}); err != nil {
			return err
		}
	}
	for _, reader := range []func() ([]elf.Symbol, error){file.DynamicSymbols, file.Symbols} {
		symbols, symbolErr := reader()
		if symbolErr != nil && !errors.Is(symbolErr, elf.ErrNoSymbols) {
			sidecar.Warnings = append(sidecar.Warnings, "ELF symbol table could not be decoded")
			continue
		}
		for _, symbol := range symbols {
			if strings.TrimSpace(symbol.Name) == "" || symbol.Section == elf.SHN_UNDEF || elf.ST_BIND(symbol.Info) == elf.STB_LOCAL || elf.ST_VISIBILITY(symbol.Other) == elf.STV_HIDDEN {
				continue
			}
			if err := appendExport(sidecar, Export{
				Name:    boundedText(symbol.Name),
				Type:    strings.ToLower(elf.ST_TYPE(symbol.Info).String()),
				Address: hexValue(symbol.Value),
				Size:    hexValue(symbol.Size),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func appendImport(sidecar *Sidecar, item Import) error {
	if strings.TrimSpace(item.Library) == "" {
		item.Library = "<unresolved>"
	}
	if len(sidecar.Imports) >= MaxImports {
		return fmt.Errorf("binary inventory import count exceeds %d", MaxImports)
	}
	sidecar.Imports = append(sidecar.Imports, item)
	return nil
}

func appendExport(sidecar *Sidecar, item Export) error {
	if len(sidecar.Exports) >= MaxExports {
		return fmt.Errorf("binary inventory export count exceeds %d", MaxExports)
	}
	sidecar.Exports = append(sidecar.Exports, item)
	return nil
}

func splitPEImport(value string) (string, string) {
	value = strings.TrimSpace(value)
	if symbol, library, ok := strings.Cut(value, ":"); ok {
		return symbol, library
	}
	return value, "<unresolved>"
}

func boundedText(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(value, "\x00", ""), "\r", " "), "\n", " "))
	if len(value) > MaxStringBytes {
		value = value[:MaxStringBytes]
	}
	return value
}

func peMachine(machine uint16) string {
	switch machine {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return "amd64"
	case pe.IMAGE_FILE_MACHINE_I386:
		return "i386"
	case pe.IMAGE_FILE_MACHINE_ARM64:
		return "arm64"
	default:
		return fmt.Sprintf("machine-%#x", machine)
	}
}

func peFileType(characteristics uint16) string {
	switch {
	case characteristics&pe.IMAGE_FILE_DLL != 0:
		return "dll"
	case characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE != 0:
		return "executable"
	default:
		return "object"
	}
}

func peSectionType(characteristics uint32) string {
	switch {
	case characteristics&pe.IMAGE_SCN_CNT_CODE != 0:
		return "code"
	case characteristics&pe.IMAGE_SCN_CNT_UNINITIALIZED_DATA != 0:
		return "uninitialized-data"
	case characteristics&pe.IMAGE_SCN_CNT_INITIALIZED_DATA != 0:
		return "initialized-data"
	default:
		return "other"
	}
}

func pePermissions(characteristics uint32) string {
	permission := []byte("---")
	if characteristics&pe.IMAGE_SCN_MEM_READ != 0 {
		permission[0] = 'r'
	}
	if characteristics&pe.IMAGE_SCN_MEM_WRITE != 0 {
		permission[1] = 'w'
	}
	if characteristics&pe.IMAGE_SCN_MEM_EXECUTE != 0 {
		permission[2] = 'x'
	}
	return string(permission)
}

func elfPermissions(flags elf.SectionFlag) string {
	permission := []byte("r--")
	if flags&elf.SHF_WRITE != 0 {
		permission[1] = 'w'
	}
	if flags&elf.SHF_EXECINSTR != 0 {
		permission[2] = 'x'
	}
	return string(permission)
}

func hexValue(value uint64) string {
	return fmt.Sprintf("0x%x", value)
}
