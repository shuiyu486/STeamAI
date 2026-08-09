//go:build windows

package sessionhost

import (
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32DLL                    = syscall.NewLazyDLL("kernel32.dll")
	shell32DLL                     = syscall.NewLazyDLL("shell32.dll")
	shGetFolderPathW               = shell32DLL.NewProc("SHGetFolderPathW")
	getFinalPathNameByHandleW      = kernel32DLL.NewProc("GetFinalPathNameByHandleW")
	queryFullProcessImageNameW     = kernel32DLL.NewProc("QueryFullProcessImageNameW")
	createJobObjectW               = kernel32DLL.NewProc("CreateJobObjectW")
	setInformationJobObject        = kernel32DLL.NewProc("SetInformationJobObject")
	assignProcessToJobObject       = kernel32DLL.NewProc("AssignProcessToJobObject")
	getCurrentProcess              = kernel32DLL.NewProc("GetCurrentProcess")
	readProcessMemory              = kernel32DLL.NewProc("ReadProcessMemory")
	ntdllDLL                       = syscall.NewLazyDLL("ntdll.dll")
	ntOpenFile                     = ntdllDLL.NewProc("NtOpenFile")
	ntResumeProcess                = ntdllDLL.NewProc("NtResumeProcess")
	wintrustDLL                    = syscall.NewLazyDLL("wintrust.dll")
	winVerifyTrustEx               = wintrustDLL.NewProc("WinVerifyTrustEx")
	wTHelperProvDataFromStateData  = wintrustDLL.NewProc("WTHelperProvDataFromStateData")
	wTHelperGetProvSignerFromChain = wintrustDLL.NewProc("WTHelperGetProvSignerFromChain")
	wTHelperGetProvCertFromChain   = wintrustDLL.NewProc("WTHelperGetProvCertFromChain")
	crypt32DLL                     = syscall.NewLazyDLL("crypt32.dll")
	certGetNameStringW             = crypt32DLL.NewProc("CertGetNameStringW")
)

type winTrustFileInfo struct {
	StructSize   uint32
	FilePath     *uint16
	FileHandle   syscall.Handle
	KnownSubject *syscall.GUID
}

type winTrustData struct {
	StructSize         uint32
	PolicyCallbackData uintptr
	SIPClientData      uintptr
	UIChoice           uint32
	RevocationChecks   uint32
	UnionChoice        uint32
	FileInfo           *winTrustFileInfo
	StateAction        uint32
	StateData          syscall.Handle
	URLReference       *uint16
	ProviderFlags      uint32
	UIContext          uint32
	SignatureSettings  uintptr
}

type cryptProviderCert struct {
	StructSize uint32
	Cert       *certContext
}

type certContext struct {
	EncodingType uint32
	Encoded      *byte
	EncodedSize  uint32
	CertInfo     uintptr
	CertStore    syscall.Handle
}

type unicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

type objectAttributes struct {
	Length                   uint32
	RootDirectory            syscall.Handle
	ObjectName               *unicodeString
	Attributes               uint32
	SecurityDescriptor       uintptr
	SecurityQualityOfService uintptr
}

type ioStatusBlock struct {
	Status      uintptr
	Information uintptr
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IOInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type claudeProcessContainment struct {
	handle syscall.Handle
}

func (containment *claudeProcessContainment) Close() error {
	if containment == nil || containment.handle == 0 {
		return nil
	}
	handle := containment.handle
	containment.handle = 0
	return syscall.CloseHandle(handle)
}

type versionNode struct {
	key      string
	value    []byte
	children []versionNode
}

func windowsKnownFolderLocalAppData() (string, error) {
	const csidlLocalAppData = 0x001c
	buffer := make([]uint16, syscall.MAX_PATH)
	result, _, callErr := shGetFolderPathW.Call(
		0,
		csidlLocalAppData,
		0,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if result != 0 {
		return "", fmt.Errorf("resolve Windows LocalAppData known folder: HRESULT 0x%x: %v", result, callErr)
	}
	path := syscall.UTF16ToString(buffer)
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("Windows LocalAppData known folder is empty")
	}
	return path, nil
}

func trustedClaudeInstallationCandidates() ([]string, error) {
	profile, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve Windows user profile for Claude Code installation: %w", err)
	}
	candidates := []string{filepath.Join(profile, ".local", "bin", "claude.exe")}
	localAppData, err := windowsKnownFolderLocalAppData()
	if err == nil {
		candidates = append(candidates,
			filepath.Join(localAppData, "AnthropicClaude", "claude.exe"),
			filepath.Join(localAppData, "Programs", "Claude", "claude.exe"),
		)
	}
	return candidates, nil
}

func validateClaudeExecutablePathComponents(path string) error {
	full, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(full)
	current := volume + string(filepath.Separator)
	rel := strings.TrimPrefix(full, current)
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		pointer, err := syscall.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attrs, err := syscall.GetFileAttributes(pointer)
		if err != nil {
			return err
		}
		if attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("trusted Claude executable path contains a reparse point: %s", current)
		}
	}
	return nil
}

func configureTrustedClaudeCommand(cmd *exec.Cmd, binding *claudeExecutableLock) error {
	if binding == nil {
		return nil
	}
	if cmd == nil {
		return fmt.Errorf("trusted Claude command is missing")
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	const createSuspended = 0x00000004
	cmd.SysProcAttr.CreationFlags |= createSuspended
	return nil
}

func validateAndResumeTrustedClaudeProcess(process *os.Process, binding *claudeExecutableLock) error {
	containment, err := validateContainAndResumeTrustedClaudeProcess(process, binding)
	if containment != nil {
		_ = containment.Close()
	}
	return err
}

func validateContainAndResumeTrustedClaudeProcess(process *os.Process, binding *claudeExecutableLock) (*claudeProcessContainment, error) {
	if binding == nil {
		return nil, nil
	}
	if process == nil {
		return nil, fmt.Errorf("trusted Claude process is missing")
	}
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	const (
		processNameNative              = 0x1
		processQueryLimitedInformation = 0x1000
		processSuspendResume           = 0x0800
		processSetQuota                = 0x0100
		processTerminate               = 0x0001
	)
	handle, err := syscall.OpenProcess(processQueryLimitedInformation|processSuspendResume|processSetQuota|processTerminate, false, uint32(process.Pid))
	if err != nil {
		return nil, fmt.Errorf("open suspended Claude process for identity validation: %w", err)
	}
	defer syscall.CloseHandle(handle)
	buffer := make([]uint16, 32768)
	length := uint32(len(buffer))
	result, _, callErr := queryFullProcessImageNameW.Call(
		uintptr(handle),
		processNameNative,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&length)),
	)
	if result == 0 {
		return nil, fmt.Errorf("query suspended Claude process image: %v", callErr)
	}
	nativePath := syscall.UTF16ToString(buffer[:length])
	if !validClaudeNativeImagePath(nativePath) {
		return nil, fmt.Errorf("suspended Claude process returned an invalid native image path: %s", nativePath)
	}
	if !strings.EqualFold(nativePath, binding.nativePath) {
		return nil, fmt.Errorf("suspended Claude process native image path does not match the verified executable")
	}
	actual, err := openNativeClaudeExecutableReadLock(nativePath)
	if err != nil {
		return nil, fmt.Errorf("open suspended Claude process image by native path: %w", err)
	}
	defer actual.Close()
	expectedInfo, err := binding.file.Stat()
	if err != nil {
		return nil, err
	}
	actualInfo, err := actual.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(expectedInfo, actualInfo) {
		return nil, fmt.Errorf("suspended Claude process image does not match the verified executable")
	}
	job, _, callErr := createJobObjectW.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("create Claude containment job: %v", callErr)
	}
	containment := &claudeProcessContainment{handle: syscall.Handle(job)}
	limits := jobObjectExtendedLimitInformation{}
	const jobObjectLimitKillOnJobClose = 0x00002000
	const jobObjectExtendedLimitInformationClass = 9
	limits.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	set, _, callErr := setInformationJobObject.Call(job, jobObjectExtendedLimitInformationClass, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if set == 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("set Claude containment job limits: %v", callErr)
	}
	assigned, _, callErr := assignProcessToJobObject.Call(job, uintptr(handle))
	if assigned == 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("assign verified Claude process to containment job: %v", callErr)
	}
	status, _, callErr := ntResumeProcess.Call(uintptr(handle))
	if int32(status) < 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("resume verified Claude process: NTSTATUS 0x%s: %v", strconv.FormatUint(uint64(uint32(status)), 16), callErr)
	}
	return containment, nil
}

func openNativeClaudeExecutableReadLock(path string) (*os.File, error) {
	if !validClaudeNativeImagePath(path) {
		return nil, fmt.Errorf("invalid native Claude executable path: %s", path)
	}
	utf16Path, err := syscall.UTF16FromString(path)
	if err != nil {
		return nil, err
	}
	if len(utf16Path) > 32767 {
		return nil, fmt.Errorf("native Claude executable path is too long")
	}
	name := unicodeString{
		Length:        uint16((len(utf16Path) - 1) * 2),
		MaximumLength: uint16(len(utf16Path) * 2),
		Buffer:        &utf16Path[0],
	}
	const (
		objectCaseInsensitive     = 0x00000040
		objectDontReparse         = 0x00001000
		fileGenericRead           = 0x00120089
		fileShareRead             = 0x00000001
		fileSynchronousIONonAlert = 0x00000020
		fileNonDirectoryFile      = 0x00000040
		fileOpenReparsePoint      = 0x00200000
	)
	attributes := objectAttributes{
		Length:     uint32(unsafe.Sizeof(objectAttributes{})),
		ObjectName: &name,
		Attributes: objectCaseInsensitive | objectDontReparse,
	}
	var handle syscall.Handle
	var statusBlock ioStatusBlock
	status, _, _ := ntOpenFile.Call(
		uintptr(unsafe.Pointer(&handle)),
		fileGenericRead,
		uintptr(unsafe.Pointer(&attributes)),
		uintptr(unsafe.Pointer(&statusBlock)),
		fileShareRead,
		fileSynchronousIONonAlert|fileNonDirectoryFile|fileOpenReparsePoint,
	)
	if int32(uint32(status)) < 0 {
		return nil, fmt.Errorf("NtOpenFile %s: NTSTATUS 0x%s", path, strconv.FormatUint(uint64(uint32(status)), 16))
	}
	if handle == 0 || handle == syscall.InvalidHandle {
		return nil, fmt.Errorf("NtOpenFile returned an invalid handle for %s", path)
	}
	return os.NewFile(uintptr(handle), path), nil
}

func trustedClaudeVersion(locked *os.File) (string, error) {
	image, err := pe.NewFile(locked)
	if err != nil {
		return "", fmt.Errorf("open trusted Claude PE metadata from locked handle: %w", err)
	}
	defer image.Close()
	resourceRVA, resourceSize, err := peResourceDirectory(image)
	if err != nil {
		return "", err
	}
	versionData, err := peVersionResource(locked, image, resourceRVA, resourceSize)
	if err != nil {
		return "", err
	}
	root, err := parseVersionNode(versionData, 0, len(versionData), 0)
	if err != nil {
		return "", fmt.Errorf("parse trusted Claude VERSIONINFO: %w", err)
	}
	majorMinor, buildRevision, err := claudeVersionMetadata(root)
	if err != nil {
		return "", err
	}
	parts := []string{
		strconv.Itoa(int(majorMinor >> 16)),
		strconv.Itoa(int(majorMinor & 0xffff)),
		strconv.Itoa(int(buildRevision >> 16)),
		strconv.Itoa(int(buildRevision & 0xffff)),
	}
	for len(parts) > 3 && parts[len(parts)-1] == "0" {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, ".") + " (Claude Code)", nil
}

func nativeClaudeExecutablePath(locked *os.File) (string, error) {
	const volumeNameNT = 0x2
	buffer := make([]uint16, 32768)
	length, _, callErr := getFinalPathNameByHandleW.Call(
		locked.Fd(),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		volumeNameNT,
	)
	if length == 0 || length >= uintptr(len(buffer)) {
		return "", fmt.Errorf("resolve trusted Claude native path: %v", callErr)
	}
	path := syscall.UTF16ToString(buffer[:length])
	if !validClaudeNativeImagePath(path) {
		return "", fmt.Errorf("trusted Claude executable did not resolve to a native NT path: %s", path)
	}
	return path, nil
}

func validClaudeNativeImagePath(path string) bool {
	return strings.HasPrefix(path, `\Device\`) &&
		strings.HasSuffix(strings.ToLower(path), `\claude.exe`) &&
		!strings.ContainsAny(path, "\x00\r\n")
}

func claudeVersionMetadata(root versionNode) (uint32, uint32, error) {
	if root.key != "VS_VERSION_INFO" || len(root.value) < 52 || binary.LittleEndian.Uint32(root.value[:4]) != 0xfeef04bd {
		return 0, 0, fmt.Errorf("trusted Claude VERSIONINFO omitted a valid fixed file info")
	}
	productNames := versionStringValues(root, "ProductName")
	if len(productNames) == 0 {
		return 0, 0, fmt.Errorf("trusted Claude VERSIONINFO omitted ProductName")
	}
	for _, productName := range productNames {
		if !strings.EqualFold(strings.TrimSpace(productName), "Claude Code") {
			return 0, 0, fmt.Errorf("trusted executable version metadata is not Claude Code: %q", productName)
		}
	}
	return binary.LittleEndian.Uint32(root.value[8:12]), binary.LittleEndian.Uint32(root.value[12:16]), nil
}

func peResourceDirectory(image *pe.File) (uint32, uint32, error) {
	const resourceDirectoryIndex = 2
	var directory pe.DataDirectory
	switch optional := image.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		if len(optional.DataDirectory) <= resourceDirectoryIndex {
			return 0, 0, fmt.Errorf("trusted Claude PE omitted its resource directory")
		}
		directory = optional.DataDirectory[resourceDirectoryIndex]
	case *pe.OptionalHeader64:
		if len(optional.DataDirectory) <= resourceDirectoryIndex {
			return 0, 0, fmt.Errorf("trusted Claude PE omitted its resource directory")
		}
		directory = optional.DataDirectory[resourceDirectoryIndex]
	default:
		return 0, 0, fmt.Errorf("trusted Claude PE has an unsupported optional header")
	}
	if directory.VirtualAddress == 0 || directory.Size < 24 || directory.Size > 32<<20 {
		return 0, 0, fmt.Errorf("trusted Claude PE has an invalid resource directory")
	}
	return directory.VirtualAddress, directory.Size, nil
}

func peVersionResource(locked *os.File, image *pe.File, resourceRVA, resourceSize uint32) ([]byte, error) {
	rootOffset, err := peRVAFileOffset(image, resourceRVA, resourceSize)
	if err != nil {
		return nil, err
	}
	typeEntries, err := peResourceEntries(locked, rootOffset, 0, resourceSize)
	if err != nil {
		return nil, err
	}
	var typeEntry uint32
	foundType := false
	for _, entry := range typeEntries {
		if entry.name&0x80000000 == 0 && entry.name&0xffff == 16 {
			if foundType {
				return nil, fmt.Errorf("trusted Claude PE has duplicate RT_VERSION directories")
			}
			typeEntry = entry.offset
			foundType = true
		}
	}
	if !foundType || typeEntry&0x80000000 == 0 {
		return nil, fmt.Errorf("trusted Claude PE omitted RT_VERSION")
	}
	nameEntries, err := peResourceEntries(locked, rootOffset, typeEntry&0x7fffffff, resourceSize)
	if err != nil {
		return nil, err
	}
	resources := [][]byte{}
	for _, nameEntry := range nameEntries {
		if nameEntry.offset&0x80000000 == 0 {
			return nil, fmt.Errorf("trusted Claude RT_VERSION name entry is not a directory")
		}
		languageEntries, err := peResourceEntries(locked, rootOffset, nameEntry.offset&0x7fffffff, resourceSize)
		if err != nil {
			return nil, err
		}
		for _, languageEntry := range languageEntries {
			if languageEntry.offset&0x80000000 != 0 {
				return nil, fmt.Errorf("trusted Claude RT_VERSION language entry is not data")
			}
			dataEntry, err := readAtBounded(locked, int64(rootOffset)+int64(languageEntry.offset), 16, int64(rootOffset)+int64(resourceSize))
			if err != nil {
				return nil, err
			}
			dataRVA := binary.LittleEndian.Uint32(dataEntry[:4])
			dataSize := binary.LittleEndian.Uint32(dataEntry[4:8])
			if dataSize < 16 || dataSize > 1<<20 {
				return nil, fmt.Errorf("trusted Claude VERSIONINFO has invalid size %d", dataSize)
			}
			dataOffset, err := peRVAFileOffset(image, dataRVA, dataSize)
			if err != nil {
				return nil, err
			}
			data, err := readAtBounded(locked, int64(dataOffset), int(dataSize), int64(dataOffset)+int64(dataSize))
			if err != nil {
				return nil, err
			}
			resources = append(resources, data)
		}
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("trusted Claude PE has no VERSIONINFO data")
	}
	canonical := resources[0]
	base, err := parseVersionNode(canonical, 0, len(canonical), 0)
	if err != nil {
		return nil, err
	}
	baseMajorMinor, baseBuildRevision, err := claudeVersionMetadata(base)
	if err != nil {
		return nil, err
	}
	for _, candidate := range resources[1:] {
		root, err := parseVersionNode(candidate, 0, len(candidate), 0)
		if err != nil {
			return nil, err
		}
		majorMinor, buildRevision, err := claudeVersionMetadata(root)
		if err != nil {
			return nil, err
		}
		if majorMinor != baseMajorMinor || buildRevision != baseBuildRevision {
			return nil, fmt.Errorf("trusted Claude PE has inconsistent VERSIONINFO resources")
		}
	}
	return canonical, nil
}

type peResourceEntry struct {
	name   uint32
	offset uint32
}

func peResourceEntries(locked *os.File, rootOffset uint32, relative uint32, resourceSize uint32) ([]peResourceEntry, error) {
	if relative > resourceSize || resourceSize-relative < 16 {
		return nil, fmt.Errorf("trusted Claude PE resource directory is out of bounds")
	}
	header, err := readAtBounded(locked, int64(rootOffset)+int64(relative), 16, int64(rootOffset)+int64(resourceSize))
	if err != nil {
		return nil, err
	}
	count := uint32(binary.LittleEndian.Uint16(header[12:14])) + uint32(binary.LittleEndian.Uint16(header[14:16]))
	if count == 0 || count > 4096 || count > (resourceSize-relative-16)/8 {
		return nil, fmt.Errorf("trusted Claude PE resource entry count is invalid")
	}
	data, err := readAtBounded(locked, int64(rootOffset)+int64(relative)+16, int(count)*8, int64(rootOffset)+int64(resourceSize))
	if err != nil {
		return nil, err
	}
	entries := make([]peResourceEntry, count)
	for i := range entries {
		entries[i] = peResourceEntry{
			name:   binary.LittleEndian.Uint32(data[i*8 : i*8+4]),
			offset: binary.LittleEndian.Uint32(data[i*8+4 : i*8+8]),
		}
	}
	return entries, nil
}

func peRVAFileOffset(image *pe.File, rva, size uint32) (uint32, error) {
	for _, section := range image.Sections {
		span := section.VirtualSize
		if section.Size > span {
			span = section.Size
		}
		if rva < section.VirtualAddress || uint64(rva)+uint64(size) > uint64(section.VirtualAddress)+uint64(span) {
			continue
		}
		delta := rva - section.VirtualAddress
		if uint64(delta)+uint64(size) > uint64(section.Size) {
			return 0, fmt.Errorf("trusted Claude PE RVA points outside raw section data")
		}
		return section.Offset + delta, nil
	}
	return 0, fmt.Errorf("trusted Claude PE RVA 0x%x is unmapped", rva)
}

func readAtBounded(reader io.ReaderAt, offset int64, size int, limit int64) ([]byte, error) {
	if offset < 0 || size < 0 || int64(size) > limit-offset {
		return nil, fmt.Errorf("trusted Claude metadata read is out of bounds")
	}
	data := make([]byte, size)
	if _, err := reader.ReadAt(data, offset); err != nil {
		return nil, err
	}
	return data, nil
}

func parseVersionNode(data []byte, start, limit, depth int) (versionNode, error) {
	if depth > 16 || start < 0 || limit > len(data) || start+6 > limit {
		return versionNode{}, fmt.Errorf("VERSIONINFO node is out of bounds")
	}
	length := int(binary.LittleEndian.Uint16(data[start : start+2]))
	valueLength := int(binary.LittleEndian.Uint16(data[start+2 : start+4]))
	valueType := binary.LittleEndian.Uint16(data[start+4 : start+6])
	if length < 6 || start+length > limit {
		return versionNode{}, fmt.Errorf("VERSIONINFO node length is invalid")
	}
	end := start + length
	keyEnd := start + 6
	units := []uint16{}
	for {
		if keyEnd+2 > end {
			return versionNode{}, fmt.Errorf("VERSIONINFO key is unterminated")
		}
		unit := binary.LittleEndian.Uint16(data[keyEnd : keyEnd+2])
		keyEnd += 2
		if unit == 0 {
			break
		}
		units = append(units, unit)
		if len(units) > 1024 {
			return versionNode{}, fmt.Errorf("VERSIONINFO key is too long")
		}
	}
	valueStart := alignFour(keyEnd)
	valueBytes := valueLength
	if valueType == 1 {
		valueBytes *= 2
	}
	if valueStart > end || valueBytes > end-valueStart {
		return versionNode{}, fmt.Errorf("VERSIONINFO value is out of bounds")
	}
	node := versionNode{
		key:   string(utf16.Decode(units)),
		value: append([]byte{}, data[valueStart:valueStart+valueBytes]...),
	}
	childStart := alignFour(valueStart + valueBytes)
	for childStart+6 <= end {
		childLength := int(binary.LittleEndian.Uint16(data[childStart : childStart+2]))
		if childLength == 0 {
			for _, trailing := range data[childStart:end] {
				if trailing != 0 {
					return versionNode{}, fmt.Errorf("VERSIONINFO has nonzero trailing data")
				}
			}
			break
		}
		child, err := parseVersionNode(data, childStart, end, depth+1)
		if err != nil {
			return versionNode{}, err
		}
		node.children = append(node.children, child)
		childStart = alignFour(childStart + childLength)
	}
	return node, nil
}

func versionStringValues(node versionNode, key string) []string {
	values := []string{}
	if strings.EqualFold(node.key, key) && len(node.value)%2 == 0 {
		units := make([]uint16, len(node.value)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(node.value[i*2 : i*2+2])
		}
		value := strings.TrimSpace(strings.TrimRight(string(utf16.Decode(units)), "\x00"))
		if value != "" {
			values = append(values, value)
		}
	}
	for _, child := range node.children {
		values = append(values, versionStringValues(child, key)...)
	}
	return values
}

func alignFour(value int) int {
	return (value + 3) &^ 3
}

func lockClaudeExecutablePathNamespace(path string, locked *os.File) ([]*os.File, error) {
	full, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	volume := filepath.VolumeName(full)
	current := volume + string(filepath.Separator)
	rel := strings.TrimPrefix(full, current)
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 {
		return nil, fmt.Errorf("trusted Claude executable path has no parent namespace: %s", full)
	}
	handles := []*os.File{}
	closeAll := func() {
		for i := len(handles) - 1; i >= 0; i-- {
			_ = handles[i].Close()
		}
	}
	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		pointer, err := syscall.UTF16PtrFromString(current)
		if err != nil {
			closeAll()
			return nil, err
		}
		handle, err := syscall.CreateFile(
			pointer,
			syscall.GENERIC_READ,
			syscall.FILE_SHARE_READ,
			nil,
			syscall.OPEN_EXISTING,
			syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT,
			0,
		)
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("lock trusted Claude executable parent namespace %s: %w", current, err)
		}
		handles = append(handles, os.NewFile(uintptr(handle), current))
	}
	if err := validateClaudeExecutablePathComponents(full); err != nil {
		closeAll()
		return nil, err
	}
	if err := validateLockedClaudeExecutablePath(full, locked); err != nil {
		closeAll()
		return nil, err
	}
	return handles, nil
}

func validateClaudeExecutableLaunchNamespace(path string, locked *os.File, namespace []*os.File) error {
	full, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(full)
	current := volume + string(filepath.Separator)
	rel := strings.TrimPrefix(full, current)
	parts := strings.Split(rel, string(filepath.Separator))
	parents := parts[:len(parts)-1]
	if len(namespace) != len(parents) {
		return fmt.Errorf("trusted Claude executable namespace binding count changed: %s", full)
	}
	for i, part := range parents {
		current = filepath.Join(current, part)
		pathInfo, err := os.Lstat(current)
		if err != nil {
			return err
		}
		lockedInfo, err := namespace[i].Stat()
		if err != nil {
			return err
		}
		if !pathInfo.IsDir() || !lockedInfo.IsDir() || pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(pathInfo, lockedInfo) {
			return fmt.Errorf("trusted Claude executable parent namespace changed before launch: %s", current)
		}
	}
	if err := validateClaudeExecutablePathComponents(full); err != nil {
		return err
	}
	return validateLockedClaudeExecutablePath(full, locked)
}

func openClaudeExecutableReadLock(path string) (*os.File, error) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(
		pointer,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func verifyClaudeAuthenticodePublisher(locked *os.File) (publisher string, err error) {
	if locked == nil {
		return "", fmt.Errorf("trusted Claude executable handle is missing")
	}
	return verifyClaudeAuthenticodePublisherWithPath(locked, locked.Name())
}

func verifyClaudeAuthenticodePublisherWithPath(locked *os.File, displayPath string) (publisher string, err error) {
	if locked == nil {
		return "", fmt.Errorf("trusted Claude executable handle is missing")
	}
	pathPointer, err := syscall.UTF16PtrFromString(displayPath)
	if err != nil {
		return "", err
	}
	fileInfo := winTrustFileInfo{
		StructSize: uint32(unsafe.Sizeof(winTrustFileInfo{})),
		FilePath:   pathPointer,
		FileHandle: syscall.Handle(locked.Fd()),
	}
	trustData := winTrustData{
		StructSize:       uint32(unsafe.Sizeof(winTrustData{})),
		UIChoice:         2,
		RevocationChecks: 0,
		UnionChoice:      1,
		FileInfo:         &fileInfo,
		StateAction:      1,
		ProviderFlags:    0x00001000,
	}
	action := syscall.GUID{Data1: 0x00aac56b, Data2: 0xcd44, Data3: 0x11d0, Data4: [8]byte{0x8c, 0xc2, 0x00, 0xc0, 0x4f, 0xc2, 0x95, 0xee}}
	status, _, _ := winVerifyTrustEx.Call(^uintptr(0), uintptr(unsafe.Pointer(&action)), uintptr(unsafe.Pointer(&trustData)))
	defer func() {
		trustData.StateAction = 2
		closeStatus, _, _ := winVerifyTrustEx.Call(^uintptr(0), uintptr(unsafe.Pointer(&action)), uintptr(unsafe.Pointer(&trustData)))
		if int32(closeStatus) != 0 && err == nil {
			publisher = ""
			err = fmt.Errorf("close Claude Code Authenticode verification state: HRESULT 0x%s", strconv.FormatUint(uint64(uint32(closeStatus)), 16))
		}
	}()
	if int32(status) != 0 {
		return "", fmt.Errorf("verify Claude Code Authenticode signature: HRESULT 0x%s", strconv.FormatUint(uint64(uint32(status)), 16))
	}
	providerData, _, _ := wTHelperProvDataFromStateData.Call(uintptr(trustData.StateData))
	if providerData == 0 {
		return "", fmt.Errorf("Claude Code Authenticode provider data is unavailable")
	}
	signerPointer, _, _ := wTHelperGetProvSignerFromChain.Call(providerData, 0, 0, 0)
	if signerPointer == 0 {
		return "", fmt.Errorf("Claude Code Authenticode signer chain is unavailable")
	}
	providerCertPointer, _, _ := wTHelperGetProvCertFromChain.Call(signerPointer, 0)
	if providerCertPointer == 0 {
		return "", fmt.Errorf("Claude Code Authenticode signer certificate is unavailable")
	}
	var providerCert cryptProviderCert
	var copied uintptr
	currentProcess, _, _ := getCurrentProcess.Call()
	read, _, readErr := readProcessMemory.Call(
		currentProcess,
		providerCertPointer,
		uintptr(unsafe.Pointer(&providerCert)),
		unsafe.Sizeof(providerCert),
		uintptr(unsafe.Pointer(&copied)),
	)
	if read == 0 || copied != unsafe.Sizeof(providerCert) {
		return "", fmt.Errorf("read Claude Code Authenticode signer certificate: %v", readErr)
	}
	if providerCert.Cert == nil {
		return "", fmt.Errorf("Claude Code Authenticode signer certificate context is unavailable")
	}
	commonNameOID := append([]byte("2.5.4.3"), 0)
	const certNameAttributeType = 3
	length, _, _ := certGetNameStringW.Call(
		uintptr(unsafe.Pointer(providerCert.Cert)),
		certNameAttributeType,
		0,
		uintptr(unsafe.Pointer(&commonNameOID[0])),
		0,
		0,
	)
	if length <= 1 || length > 4096 {
		return "", fmt.Errorf("Claude Code Authenticode signer display name is invalid")
	}
	buffer := make([]uint16, length)
	written, _, _ := certGetNameStringW.Call(
		uintptr(unsafe.Pointer(providerCert.Cert)),
		certNameAttributeType,
		0,
		uintptr(unsafe.Pointer(&commonNameOID[0])),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if written != length {
		return "", fmt.Errorf("read Claude Code Authenticode signer display name")
	}
	publisher = strings.TrimSpace(syscall.UTF16ToString(buffer))
	if publisher == "" {
		return "", fmt.Errorf("Claude Code Authenticode signer display name is empty")
	}
	runtime.KeepAlive(locked)
	runtime.KeepAlive(pathPointer)
	return publisher, nil
}
