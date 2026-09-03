//go:build windows

package steamai

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	createNewConsole             = 0x00000010
	createUnicodeEnv             = 0x00000400
	fileAttributeReparse         = 0x00000400
	regOptionNonVolatile         = 0
	regCreatedNewKey             = 1
	regOpenExistingKey           = 2
	defaultInstallRegistrySubkey = `Software\STeamAI`
	userEnvironmentSubkey        = `Environment`
	canonicalSourceValue         = `CanonicalSource`
	installedVersionValue        = `InstalledVersion`
	pathAddedValue               = `PathAdded`
	userPathValue                = `Path`
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	advapi32               = syscall.NewLazyDLL("advapi32.dll")
	procCreateMutexW       = kernel32.NewProc("CreateMutexW")
	procRegCreateKeyExW    = advapi32.NewProc("RegCreateKeyExW")
	procRegSetValueExW     = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW    = advapi32.NewProc("RegDeleteValueW")
	procRegDeleteKeyW      = advapi32.NewProc("RegDeleteKeyW")
	procSendMessageTimeout = syscall.NewLazyDLL("user32.dll").NewProc("SendMessageTimeoutW")
)

var installRegistrySubkey = defaultInstallRegistrySubkey

type nativePlatform struct{}

func (nativePlatform) Supported() bool { return true }

func (nativePlatform) CanonicalSource() (string, error) {
	key, err := openRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey, syscall.KEY_READ)
	if err != nil {
		return "", err
	}
	defer syscall.RegCloseKey(key)
	return queryRegistryString(key, canonicalSourceValue)
}

func (nativePlatform) ActiveExecutable() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return "", errors.New("LOCALAPPDATA 不可用")
	}
	return filepath.Join(localAppData, "STeamAI", "bin", "steamai.exe"), nil
}

func (nativePlatform) Install(executable, source, version string) error {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return errors.New("LOCALAPPDATA 不可用")
	}
	binDir := filepath.Join(localAppData, "STeamAI", "bin")
	installed := filepath.Join(binDir, "steamai.exe")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	productKey, err := createRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		return err
	}
	defer syscall.RegCloseKey(productKey)
	envKey, err := openRegistryKey(syscall.HKEY_CURRENT_USER, userEnvironmentSubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		return err
	}
	defer syscall.RegCloseKey(envKey)
	currentPath, err := queryRegistryString(envKey, userPathValue)
	if err != nil && !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return err
	}
	oldSource, sourceErr := queryRegistryString(productKey, canonicalSourceValue)
	oldSourceExists := sourceErr == nil
	if sourceErr != nil && !errors.Is(sourceErr, syscall.ERROR_FILE_NOT_FOUND) {
		return sourceErr
	}
	oldVersion, versionErr := queryRegistryString(productKey, installedVersionValue)
	oldVersionExists := versionErr == nil
	if versionErr != nil && !errors.Is(versionErr, syscall.ERROR_FILE_NOT_FOUND) {
		return versionErr
	}
	oldPathAdded, pathAddedErr := queryRegistryString(productKey, pathAddedValue)
	oldPathAddedExists := pathAddedErr == nil
	if pathAddedErr != nil && !errors.Is(pathAddedErr, syscall.ERROR_FILE_NOT_FOUND) {
		return pathAddedErr
	}
	if err := ensureInstallTarget(executable, installed); err != nil {
		return err
	}
	installedExists, err := pathExists(installed)
	if err != nil {
		return err
	}
	if err := installExecutable(executable, installed); err != nil {
		return err
	}
	rollbackExe := !installedExists
	rollbackRegistry := false
	defer func() {
		if rollbackExe {
			_ = os.Remove(installed)
		}
		if rollbackRegistry {
			if oldSourceExists {
				_ = setRegistryString(productKey, canonicalSourceValue, oldSource, syscall.REG_SZ)
			} else {
				_ = deleteRegistryValue(productKey, canonicalSourceValue)
			}
			if oldVersionExists {
				_ = setRegistryString(productKey, installedVersionValue, oldVersion, syscall.REG_SZ)
			} else {
				_ = deleteRegistryValue(productKey, installedVersionValue)
			}
			if oldPathAddedExists {
				_ = setRegistryString(productKey, pathAddedValue, oldPathAdded, syscall.REG_SZ)
			} else {
				_ = deleteRegistryValue(productKey, pathAddedValue)
			}
		}
	}()
	if err := setRegistryString(productKey, canonicalSourceValue, source, syscall.REG_SZ); err != nil {
		return err
	}
	rollbackRegistry = true
	if err := setRegistryString(productKey, installedVersionValue, version, syscall.REG_SZ); err != nil {
		return err
	}
	pathAdded, addPath := setupPathDecision(currentPath, binDir, oldPathAdded, oldPathAddedExists)
	if err := setRegistryString(productKey, pathAddedValue, fmt.Sprint(pathAdded), syscall.REG_SZ); err != nil {
		return err
	}
	if addPath {
		newPath := currentPath
		if newPath != "" && !strings.HasSuffix(newPath, ";") {
			newPath += ";"
		}
		newPath += binDir
		if err := setRegistryString(envKey, userPathValue, newPath, syscall.REG_EXPAND_SZ); err != nil {
			return err
		}
		if err := broadcastEnvironmentChange(); err != nil {
			_ = setRegistryString(envKey, userPathValue, currentPath, syscall.REG_EXPAND_SZ)
			return err
		}
	}
	rollbackExe = false
	rollbackRegistry = false
	return nil
}

func (nativePlatform) ActivateUpdate(update updateInstall) (updateResult, error) {
	active, err := (nativePlatform{}).ActiveExecutable()
	if err != nil {
		return updateResult{}, err
	}
	active, err = filepath.Abs(active)
	if err != nil {
		return updateResult{}, err
	}
	if err := requirePlainFile(active); err != nil {
		return updateResult{}, errors.New("已安装的 steamai.exe 缺失或无效")
	}
	if err := requirePlainFile(update.Executable); err != nil {
		return updateResult{}, err
	}
	if update.ReplaceSource {
		if err := requirePlainDirectory(update.StagedSource); err != nil {
			return updateResult{}, err
		}
	}
	productKey, err := openRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		return updateResult{}, err
	}
	defer syscall.RegCloseKey(productKey)
	boundSource, err := queryRegistryString(productKey, canonicalSourceValue)
	if err != nil {
		return updateResult{}, err
	}
	boundSource, err = filepath.Abs(boundSource)
	if err != nil || !sameExistingFile(boundSource, update.Source) {
		return updateResult{}, errors.New("canonical source binding 在更新期间发生变化")
	}
	if update.ExpectedHead == "" || update.ExpectedRefs == "" {
		return updateResult{}, errors.New("update source pre-state 未完整绑定")
	}
	git, err := exec.LookPath("git.exe")
	if err != nil {
		return updateResult{}, errors.New("找不到原生 Git，无法执行最终 source pre-state 校验")
	}
	state, err := captureCanonicalUpdateState(git, update.Source)
	if err != nil || state.Head != update.ExpectedHead || state.Status != update.ExpectedStatus || state.Refs != update.ExpectedRefs {
		return updateResult{}, errors.New("canonical checkout 在最终切换前发生变化")
	}
	backupSource := ""
	rollbackSource := false
	if update.ReplaceSource {
		if err := os.Chdir(filepath.Dir(update.Source)); err != nil {
			return updateResult{}, fmt.Errorf("切换到 canonical source 父目录: %w", err)
		}
		backupSource = update.Source + ".steamai-update-backup"
		if exists, pathErr := pathExists(backupSource); pathErr != nil || exists {
			return updateResult{}, errors.New("canonical source update backup 已存在")
		}
		if err := os.Rename(update.Source, backupSource); err != nil {
			return updateResult{}, fmt.Errorf("备份 canonical source: %w", err)
		}
		rollbackSource = true
		defer func() {
			if rollbackSource {
				_ = os.Rename(update.Source, update.StagedSource)
				_ = os.Rename(backupSource, update.Source)
			}
		}()
		if err := os.Rename(update.StagedSource, update.Source); err != nil {
			return updateResult{}, fmt.Errorf("发布 canonical source update: %w", err)
		}
	}
	activeDir := filepath.Dir(active)
	nextExe := filepath.Join(activeDir, "steamai.next.exe")
	if exists, pathErr := pathExists(nextExe); pathErr != nil || exists {
		return updateResult{}, errors.New("待切换 steamai.next.exe 已存在")
	}
	if err := installExecutable(update.Executable, nextExe); err != nil {
		return updateResult{}, err
	}
	oldVersion, oldVersionErr := queryRegistryString(productKey, installedVersionValue)
	oldVersionExists := oldVersionErr == nil
	if oldVersionErr != nil && !errors.Is(oldVersionErr, syscall.ERROR_FILE_NOT_FOUND) {
		_ = os.Remove(nextExe)
		return updateResult{}, oldVersionErr
	}
	previousExe := filepath.Join(activeDir, "steamai.previous.exe")
	if err := os.Remove(previousExe); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(nextExe)
		return updateResult{}, fmt.Errorf("清理旧 steamai.previous.exe: %w", err)
	}
	if err := os.Rename(active, previousExe); err != nil {
		_ = os.Remove(nextExe)
		return updateResult{}, fmt.Errorf("切换当前 steamai.exe: %w", err)
	}
	if err := os.Rename(nextExe, active); err != nil {
		_ = os.Rename(previousExe, active)
		return updateResult{}, fmt.Errorf("发布新版 steamai.exe: %w", err)
	}
	if err := setRegistryString(productKey, installedVersionValue, update.Version, syscall.REG_SZ); err != nil {
		_ = os.Remove(active)
		_ = os.Rename(previousExe, active)
		if oldVersionExists {
			_ = setRegistryString(productKey, installedVersionValue, oldVersion, syscall.REG_SZ)
		} else {
			_ = deleteRegistryValue(productKey, installedVersionValue)
		}
		return updateResult{}, err
	}
	rollbackSource = false
	return updateResult{CleanupPath: backupSource}, nil
}

func (nativePlatform) Uninstall(currentExecutable string) (uninstallResult, error) {
	active, err := (nativePlatform{}).ActiveExecutable()
	if err != nil {
		return uninstallResult{}, err
	}
	active, err = filepath.Abs(active)
	if err != nil {
		return uninstallResult{}, err
	}
	currentExecutable, err = filepath.Abs(currentExecutable)
	if err != nil {
		return uninstallResult{}, err
	}
	if !sameExecutablePath(active, currentExecutable) || !sameExistingFile(active, currentExecutable) {
		return uninstallResult{}, errors.New("uninstall 必须由 setup 安装的 steamai.exe 执行")
	}
	productKey, err := openRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		return uninstallResult{}, err
	}
	source, sourceErr := queryRegistryString(productKey, canonicalSourceValue)
	if sourceErr != nil && !errors.Is(sourceErr, syscall.ERROR_FILE_NOT_FOUND) {
		syscall.RegCloseKey(productKey)
		return uninstallResult{}, sourceErr
	}
	envKey, err := openRegistryKey(syscall.HKEY_CURRENT_USER, userEnvironmentSubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		syscall.RegCloseKey(productKey)
		return uninstallResult{}, err
	}
	pathValue, pathErr := queryRegistryString(envKey, userPathValue)
	if pathErr != nil && !errors.Is(pathErr, syscall.ERROR_FILE_NOT_FOUND) {
		syscall.RegCloseKey(envKey)
		syscall.RegCloseKey(productKey)
		return uninstallResult{}, pathErr
	}
	installedVersion, versionErr := queryRegistryString(productKey, installedVersionValue)
	if versionErr != nil && !errors.Is(versionErr, syscall.ERROR_FILE_NOT_FOUND) {
		syscall.RegCloseKey(envKey)
		syscall.RegCloseKey(productKey)
		return uninstallResult{}, versionErr
	}
	pathAdded, pathAddedErr := queryRegistryString(productKey, pathAddedValue)
	if pathAddedErr != nil && !errors.Is(pathAddedErr, syscall.ERROR_FILE_NOT_FOUND) {
		syscall.RegCloseKey(envKey)
		syscall.RegCloseKey(productKey)
		return uninstallResult{}, pathAddedErr
	}
	newPath, removedPath := removePathListEntry(pathValue, filepath.Dir(active))
	removedPath = removedPath && pathAdded == "true"
	if removedPath {
		if err := setRegistryString(envKey, userPathValue, newPath, syscall.REG_EXPAND_SZ); err != nil {
			syscall.RegCloseKey(envKey)
			syscall.RegCloseKey(productKey)
			return uninstallResult{}, err
		}
		if err := broadcastEnvironmentChange(); err != nil {
			_ = setRegistryString(envKey, userPathValue, pathValue, syscall.REG_EXPAND_SZ)
			syscall.RegCloseKey(envKey)
			syscall.RegCloseKey(productKey)
			return uninstallResult{}, err
		}
	}
	syscall.RegCloseKey(envKey)
	activeDir := filepath.Dir(active)
	retiredExe := filepath.Join(activeDir, "steamai.uninstalling.exe")
	helperExe := filepath.Join(activeDir, ".steamai-uninstall-helper-"+strconv.Itoa(os.Getpid())+".exe")
	for _, path := range []string{retiredExe, helperExe} {
		if exists, pathErr := pathExists(path); pathErr != nil || exists {
			syscall.RegCloseKey(productKey)
			if removedPath {
				_ = restoreUserPath(pathValue)
			}
			return uninstallResult{}, errors.New("待清理 uninstall 文件已存在")
		}
	}
	if err := installExecutable(active, helperExe); err != nil {
		syscall.RegCloseKey(productKey)
		if removedPath {
			_ = restoreUserPath(pathValue)
		}
		return uninstallResult{}, fmt.Errorf("准备原生 uninstall helper: %w", err)
	}
	if err := os.Rename(active, retiredExe); err != nil {
		_ = os.Remove(helperExe)
		syscall.RegCloseKey(productKey)
		if removedPath {
			_ = restoreUserPath(pathValue)
		}
		return uninstallResult{}, fmt.Errorf("准备卸载当前 steamai.exe: %w", err)
	}
	for _, name := range []string{canonicalSourceValue, installedVersionValue, pathAddedValue} {
		if err := deleteRegistryValue(productKey, name); err != nil {
			_ = os.Rename(retiredExe, active)
			_ = os.Remove(helperExe)
			if removedPath {
				_ = restoreUserPath(pathValue)
			}
			syscall.RegCloseKey(productKey)
			return uninstallResult{}, fmt.Errorf("删除安装定位: %w", err)
		}
	}
	cleanup, err := startUninstallCleanup(helperExe, os.Getpid(), []string{
		retiredExe,
		filepath.Join(activeDir, "steamai.previous.exe"),
		filepath.Join(activeDir, "steamai.next.exe"),
	})
	if err != nil {
		_ = setRegistryString(productKey, canonicalSourceValue, source, syscall.REG_SZ)
		if versionErr == nil {
			_ = setRegistryString(productKey, installedVersionValue, installedVersion, syscall.REG_SZ)
		}
		_ = setRegistryString(productKey, pathAddedValue, pathAdded, syscall.REG_SZ)
		_ = os.Rename(retiredExe, active)
		_ = os.Remove(helperExe)
		if removedPath {
			_ = restoreUserPath(pathValue)
		}
		syscall.RegCloseKey(productKey)
		return uninstallResult{}, err
	}
	syscall.RegCloseKey(productKey)
	_ = deleteRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey)
	_ = cleanup.Release()
	return uninstallResult{Source: source, CleanupDeferred: true, CleanupHelper: helperExe}, nil
}

func sameExistingFile(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func restoreUserPath(value string) error {
	key, err := openRegistryKey(syscall.HKEY_CURRENT_USER, userEnvironmentSubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		return err
	}
	defer syscall.RegCloseKey(key)
	if err := setRegistryString(key, userPathValue, value, syscall.REG_EXPAND_SZ); err != nil {
		return err
	}
	return broadcastEnvironmentChange()
}

func startUninstallCleanup(executable string, parentPID int, paths []string) (*os.Process, error) {
	args := []string{"__uninstall-cleanup", "--parent-pid", strconv.Itoa(parentPID)}
	for _, path := range paths {
		args = append(args, "--path", path)
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir = filepath.Dir(executable)
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createUnicodeEnv}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}

func deleteRegistryKey(root syscall.Handle, subkey string) error {
	name, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return err
	}
	result, _, _ := procRegDeleteKeyW.Call(uintptr(root), uintptr(unsafe.Pointer(name)))
	if result != 0 && syscall.Errno(result) != syscall.ERROR_FILE_NOT_FOUND {
		return syscall.Errno(result)
	}
	return nil
}

func (nativePlatform) CaseIdentity(path string) (string, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := syscall.CreateFile(
		name,
		0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(handle)
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x:%08x%08x", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow), nil
}

func (nativePlatform) AcquireCommander(name string) (commanderLease, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return commanderLease{}, err
	}
	security := syscall.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(syscall.SecurityAttributes{})),
		InheritHandle: 1,
	}
	handle, _, callErr := procCreateMutexW.Call(uintptr(unsafe.Pointer(&security)), 0, uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		return commanderLease{}, callErr
	}
	if errors.Is(callErr, syscall.ERROR_ALREADY_EXISTS) {
		syscall.CloseHandle(syscall.Handle(handle))
		return commanderLease{}, errCommanderRunning
	}
	return commanderLease{
		handle:  handle,
		release: func() { _ = syscall.CloseHandle(syscall.Handle(handle)) },
	}, nil
}

func (nativePlatform) RunAttached(spec processSpec, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if len(spec.InheritedHandles) > 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		for _, handle := range spec.InheritedHandles {
			cmd.SysProcAttr.AdditionalInheritedHandles = append(cmd.SysProcAttr.AdditionalInheritedHandles, syscall.Handle(handle))
		}
	}
	return cmd.Run()
}

func (nativePlatform) OpenVisible(spec processSpec) error {
	app, err := syscall.UTF16PtrFromString(spec.Path)
	if err != nil {
		return err
	}
	arguments := make([]string, 0, len(spec.Args)+1)
	arguments = append(arguments, syscall.EscapeArg(spec.Path))
	for _, arg := range spec.Args {
		arguments = append(arguments, syscall.EscapeArg(arg))
	}
	commandLine := strings.Join(arguments, " ")
	command, err := syscall.UTF16PtrFromString(commandLine)
	if err != nil {
		return err
	}
	dir, err := syscall.UTF16PtrFromString(spec.Dir)
	if err != nil {
		return err
	}
	env, err := windowsEnvironmentBlock(spec.Env)
	if err != nil {
		return err
	}
	startup := syscall.StartupInfo{Cb: uint32(unsafe.Sizeof(syscall.StartupInfo{}))}
	var process syscall.ProcessInformation
	if err := syscall.CreateProcess(app, command, nil, nil, false, createNewConsole|createUnicodeEnv, &env[0], dir, &startup, &process); err != nil {
		return fmt.Errorf("打开成员窗口: %w", err)
	}
	_ = syscall.CloseHandle(process.Thread)
	_ = syscall.CloseHandle(process.Process)
	return nil
}

func rejectReparse(path string) error {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return err
	}
	if attributes&fileAttributeReparse != 0 {
		return errors.New("拒绝 symlink/junction/reparse 路径")
	}
	return nil
}

func ensureInstallTarget(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if current, err := os.ReadFile(target); err == nil {
		if bytes.Equal(current, data) {
			return nil
		}
		return errors.New("已安装的 steamai.exe 内容不同；setup 不会静默覆盖，请运行 steamai update")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func installExecutable(source, target string) error {
	if err := ensureInstallTarget(source, target); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(target), ".steamai-install-*.new")
	if err != nil {
		return err
	}
	temporary := temporaryFile.Name()
	defer os.Remove(temporary)
	if _, err := temporaryFile.Write(data); err != nil {
		temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0o755); err != nil {
		return err
	}
	if err := os.Link(temporary, target); err != nil {
		return err
	}
	return nil
}

func broadcastEnvironmentChange() error {
	parameter, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return err
	}
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001a
		smtoAbortIfHung = 0x0002
	)
	var result uintptr
	ok, _, callErr := procSendMessageTimeout.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(parameter)),
		smtoAbortIfHung,
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
	if ok == 0 {
		return fmt.Errorf("广播 PATH 更新: %w", callErr)
	}
	return nil
}

func openRegistryKey(root syscall.Handle, subkey string, access uint32) (syscall.Handle, error) {
	name, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return 0, err
	}
	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(root, name, 0, access, &key); err != nil {
		return 0, err
	}
	return key, nil
}

func createRegistryKey(root syscall.Handle, subkey string, access uint32) (syscall.Handle, error) {
	name, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return 0, err
	}
	var key syscall.Handle
	var disposition uint32
	result, _, callErr := procRegCreateKeyExW.Call(
		uintptr(root), uintptr(unsafe.Pointer(name)), 0, 0, regOptionNonVolatile,
		uintptr(access), 0, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&disposition)),
	)
	if result != 0 {
		return 0, syscall.Errno(result)
	}
	if disposition != regCreatedNewKey && disposition != regOpenExistingKey {
		syscall.RegCloseKey(key)
		return 0, errors.New("注册表返回未知创建状态")
	}
	_ = callErr
	return key, nil
}

func queryRegistryString(key syscall.Handle, name string) (string, error) {
	valueName, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", err
	}
	var valueType uint32
	var size uint32
	if err := syscall.RegQueryValueEx(key, valueName, nil, &valueType, nil, &size); err != nil {
		return "", err
	}
	if valueType != syscall.REG_SZ && valueType != syscall.REG_EXPAND_SZ {
		return "", errors.New("注册表字符串类型无效")
	}
	if size == 0 {
		return "", nil
	}
	buffer := make([]byte, size)
	if err := syscall.RegQueryValueEx(key, valueName, nil, &valueType, &buffer[0], &size); err != nil {
		return "", err
	}
	values := make([]uint16, len(buffer)/2)
	for i := range values {
		values[i] = binary.LittleEndian.Uint16(buffer[i*2:])
	}
	return syscall.UTF16ToString(values), nil
}

func deleteRegistryValue(key syscall.Handle, name string) error {
	valueName, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	result, _, _ := procRegDeleteValueW.Call(uintptr(key), uintptr(unsafe.Pointer(valueName)))
	if result != 0 && syscall.Errno(result) != syscall.ERROR_FILE_NOT_FOUND {
		return syscall.Errno(result)
	}
	return nil
}

func setRegistryString(key syscall.Handle, name, value string, valueType uint32) error {
	valueName, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	result, _, _ := procRegSetValueExW.Call(
		uintptr(key), uintptr(unsafe.Pointer(valueName)), 0, uintptr(valueType),
		uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)*2),
	)
	if result != 0 {
		return syscall.Errno(result)
	}
	return nil
}

func windowsEnvironmentBlock(env []string) ([]uint16, error) {
	env = append([]string(nil), env...)
	sort.Slice(env, func(i, j int) bool {
		leftKey, _, _ := strings.Cut(env[i], "=")
		rightKey, _, _ := strings.Cut(env[j], "=")
		return strings.ToUpper(leftKey) < strings.ToUpper(rightKey)
	})
	block := make([]uint16, 0)
	for _, entry := range env {
		encoded, err := syscall.UTF16FromString(entry)
		if err != nil {
			return nil, err
		}
		block = append(block, encoded...)
	}
	return append(block, 0), nil
}
