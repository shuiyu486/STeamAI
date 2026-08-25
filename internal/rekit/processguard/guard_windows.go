//go:build windows

package processguard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	getFinalPathNameByHandleW  = kernel32.NewProc("GetFinalPathNameByHandleW")
	queryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	createJobObjectW           = kernel32.NewProc("CreateJobObjectW")
	setInformationJobObject    = kernel32.NewProc("SetInformationJobObject")
	assignProcessToJobObject   = kernel32.NewProc("AssignProcessToJobObject")
	ntdll                      = syscall.NewLazyDLL("ntdll.dll")
	ntOpenFile                 = ntdll.NewProc("NtOpenFile")
	ntResumeProcess            = ntdll.NewProc("NtResumeProcess")
)

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

type ExecutableBinding struct {
	path       string
	nativePath string
	sha256     string
	file       *os.File
}

func LockExecutable(path string, maxBytes int64) (*ExecutableBinding, error) {
	full, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	if maxBytes < 1 {
		return nil, fmt.Errorf("executable size limit must be positive")
	}
	if err := rejectReparseComponents(full); err != nil {
		return nil, err
	}
	pointer, err := syscall.UTF16PtrFromString(full)
	if err != nil {
		return nil, err
	}
	const fileShareDelete = 0x00000004
	handle, err := syscall.CreateFile(pointer, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|fileShareDelete, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("lock executable for launch: %w", err)
	}
	file := os.NewFile(uintptr(handle), full)
	binding := &ExecutableBinding{path: full, file: file}
	fail := func(err error) (*ExecutableBinding, error) {
		_ = binding.Close()
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxBytes {
		return fail(fmt.Errorf("executable must be a bounded regular file: %s", full))
	}
	var nativeInfo syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &nativeInfo); err != nil {
		return fail(err)
	}
	if nativeInfo.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || nativeInfo.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return fail(fmt.Errorf("executable must not be a reparse point or directory: %s", full))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	hash := sha256.New()
	written, err := io.CopyN(hash, file, maxBytes+1)
	if err != nil && err != io.EOF {
		return fail(err)
	}
	if written != info.Size() || written > maxBytes {
		return fail(fmt.Errorf("executable changed or exceeded its size limit while hashing: %s", full))
	}
	binding.sha256 = hex.EncodeToString(hash.Sum(nil))
	binding.nativePath, err = nativePath(file)
	if err != nil {
		return fail(err)
	}
	if err := binding.Validate(); err != nil {
		return fail(err)
	}
	return binding, nil
}

func (binding *ExecutableBinding) Path() string {
	if binding == nil {
		return ""
	}
	return binding.path
}

func (binding *ExecutableBinding) SHA256() string {
	if binding == nil {
		return ""
	}
	return binding.sha256
}

func (binding *ExecutableBinding) Validate() error {
	if binding == nil || binding.file == nil || binding.nativePath == "" {
		return fmt.Errorf("executable launch binding is missing")
	}
	if err := rejectReparseComponents(binding.path); err != nil {
		return err
	}
	pathInfo, err := os.Lstat(binding.path)
	if err != nil || !pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("executable path is no longer a regular file: %s", binding.path)
	}
	lockedInfo, err := binding.file.Stat()
	if err != nil || !os.SameFile(pathInfo, lockedInfo) {
		return fmt.Errorf("executable path changed after launch binding: %s", binding.path)
	}
	return nil
}

func (binding *ExecutableBinding) Close() error {
	if binding == nil || binding.file == nil {
		return nil
	}
	file := binding.file
	binding.file = nil
	return file.Close()
}

type Containment struct {
	handle syscall.Handle
}

func (containment *Containment) Close() error {
	if containment == nil || containment.handle == 0 {
		return nil
	}
	handle := containment.handle
	containment.handle = 0
	return syscall.CloseHandle(handle)
}

func ConfigureSuspended(cmd *exec.Cmd, binding *ExecutableBinding) error {
	if cmd == nil || binding == nil {
		return fmt.Errorf("suspended executable command or binding is missing")
	}
	if err := binding.Validate(); err != nil {
		return err
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	const createSuspended = 0x00000004
	cmd.SysProcAttr.CreationFlags |= createSuspended
	return nil
}

func ConfigureInheritedFiles(cmd *exec.Cmd, files []*os.File) error {
	if cmd == nil {
		return fmt.Errorf("inherited file command is missing")
	}
	if len(files) == 0 {
		return nil
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	for _, file := range files {
		if file == nil {
			return fmt.Errorf("inherited child file handle is missing")
		}
		if _, err := file.Stat(); err != nil {
			return fmt.Errorf("validate inherited child file handle: %w", err)
		}
		cmd.SysProcAttr.AdditionalInheritedHandles = append(
			cmd.SysProcAttr.AdditionalInheritedHandles,
			syscall.Handle(file.Fd()),
		)
	}
	return nil
}

func ValidateContainAndResume(process *os.Process, binding *ExecutableBinding) (*Containment, error) {
	return validateContainAndResume(process, binding, false, nil)
}

func ValidateContainAndResumeObserved(
	process *os.Process,
	binding *ExecutableBinding,
	beforeResume func() error,
) (*Containment, error) {
	return validateContainAndResume(process, binding, false, beforeResume)
}

func ValidateContainAndResumeAllowBreakaway(process *os.Process, binding *ExecutableBinding) (*Containment, error) {
	return validateContainAndResume(process, binding, true, nil)
}

func validateContainAndResume(
	process *os.Process,
	binding *ExecutableBinding,
	allowBreakaway bool,
	beforeResume func() error,
) (*Containment, error) {
	if process == nil || binding == nil {
		return nil, fmt.Errorf("suspended executable process or binding is missing")
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
		return nil, fmt.Errorf("open suspended process for image validation: %w", err)
	}
	defer syscall.CloseHandle(handle)
	buffer := make([]uint16, 32768)
	length := uint32(len(buffer))
	result, _, callErr := queryFullProcessImageNameW.Call(uintptr(handle), processNameNative, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&length)))
	if result == 0 {
		return nil, fmt.Errorf("query suspended process image: %v", callErr)
	}
	actualNativePath := syscall.UTF16ToString(buffer[:length])
	if !validNativePath(actualNativePath) || !strings.EqualFold(actualNativePath, binding.nativePath) {
		return nil, fmt.Errorf("suspended process image path does not match the verified executable")
	}
	actual, err := openNativeExecutable(actualNativePath)
	if err != nil {
		return nil, fmt.Errorf("open suspended process image by native path: %w", err)
	}
	defer actual.Close()
	expectedInfo, err := binding.file.Stat()
	if err != nil {
		return nil, err
	}
	actualInfo, err := actual.Stat()
	if err != nil || !os.SameFile(expectedInfo, actualInfo) {
		return nil, fmt.Errorf("suspended process image identity does not match the verified executable")
	}
	job, _, callErr := createJobObjectW.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("create process containment job: %v", callErr)
	}
	containment := &Containment{handle: syscall.Handle(job)}
	limits := jobObjectExtendedLimitInformation{}
	const (
		jobObjectLimitBreakawayOK    = 0x00000800
		jobObjectLimitKillOnJobClose = 0x00002000
	)
	const jobObjectExtendedLimitInformationClass = 9
	limits.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	if allowBreakaway {
		limits.BasicLimitInformation.LimitFlags |= jobObjectLimitBreakawayOK
	}
	set, _, callErr := setInformationJobObject.Call(job, jobObjectExtendedLimitInformationClass, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if set == 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("set process containment job limits: %v", callErr)
	}
	assigned, _, callErr := assignProcessToJobObject.Call(job, uintptr(handle))
	if assigned == 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("assign verified process to containment job: %v", callErr)
	}
	if beforeResume != nil {
		if err := beforeResume(); err != nil {
			_ = containment.Close()
			return nil, fmt.Errorf("record verified contained process before resume: %w", err)
		}
	}
	status, _, callErr := ntResumeProcess.Call(uintptr(handle))
	if int32(status) < 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("resume verified process: NTSTATUS 0x%s: %v", strconv.FormatUint(uint64(uint32(status)), 16), callErr)
	}
	return containment, nil
}

func nativePath(file *os.File) (string, error) {
	const volumeNameNT = 0x2
	buffer := make([]uint16, 32768)
	length, _, callErr := getFinalPathNameByHandleW.Call(file.Fd(), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), volumeNameNT)
	if length == 0 || length >= uintptr(len(buffer)) {
		return "", fmt.Errorf("resolve executable native path: %v", callErr)
	}
	path := syscall.UTF16ToString(buffer[:length])
	if !validNativePath(path) {
		return "", fmt.Errorf("executable did not resolve to a native NT path: %s", path)
	}
	return path, nil
}

func validNativePath(path string) bool {
	return strings.HasPrefix(path, `\Device\`) && !strings.ContainsAny(path, "\x00\r\n")
}

func openNativeExecutable(path string) (*os.File, error) {
	if !validNativePath(path) {
		return nil, fmt.Errorf("invalid native executable path: %s", path)
	}
	utf16Path, err := syscall.UTF16FromString(path)
	if err != nil {
		return nil, err
	}
	name := unicodeString{Length: uint16((len(utf16Path) - 1) * 2), MaximumLength: uint16(len(utf16Path) * 2), Buffer: &utf16Path[0]}
	const (
		objectCaseInsensitive     = 0x00000040
		objectDontReparse         = 0x00001000
		fileGenericRead           = 0x00120089
		fileShareRead             = 0x00000001
		fileShareDelete           = 0x00000004
		fileSynchronousIONonAlert = 0x00000020
		fileNonDirectoryFile      = 0x00000040
		fileOpenReparsePoint      = 0x00200000
	)
	attributes := objectAttributes{Length: uint32(unsafe.Sizeof(objectAttributes{})), ObjectName: &name, Attributes: objectCaseInsensitive | objectDontReparse}
	var handle syscall.Handle
	var statusBlock ioStatusBlock
	status, _, _ := ntOpenFile.Call(uintptr(unsafe.Pointer(&handle)), fileGenericRead, uintptr(unsafe.Pointer(&attributes)), uintptr(unsafe.Pointer(&statusBlock)), fileShareRead|fileShareDelete, fileSynchronousIONonAlert|fileNonDirectoryFile|fileOpenReparsePoint)
	if int32(uint32(status)) < 0 {
		return nil, fmt.Errorf("NtOpenFile %s: NTSTATUS 0x%s", path, strconv.FormatUint(uint64(uint32(status)), 16))
	}
	if handle == 0 || handle == syscall.InvalidHandle {
		return nil, fmt.Errorf("NtOpenFile returned an invalid handle for %s", path)
	}
	return os.NewFile(uintptr(handle), path), nil
}

func rejectReparseComponents(path string) error {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	rel := strings.TrimPrefix(path, current)
	for component := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		pointer, err := syscall.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attributes, err := syscall.GetFileAttributes(pointer)
		if err != nil {
			return err
		}
		if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("executable path contains a reparse point: %s", current)
		}
	}
	return nil
}
