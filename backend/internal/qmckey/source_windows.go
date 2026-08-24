//go:build windows && amd64

package qmckey

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	th32csSnapProcess       = 0x00000002
	processVMRead           = 0x0010
	processQueryInformation = 0x0400
	memCommit               = 0x1000
	memPrivate              = 0x20000
	memImage                = 0x1000000
	pageNoAccess            = 0x01
	pageGuard               = 0x100
	processScanChunkSize    = 1 << 20
	processScanOverlap      = 8 << 10
	maxProcessScanBytes     = 768 << 20
	processScanTimeout      = 8 * time.Second
	invalidHandleValue      = ^uintptr(0)
)

var targetProcessNames = map[string]struct{}{
	"qqmusic.exe":   {},
	"qmbrowser.exe": {},
}

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot   = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW            = kernel32.NewProc("Process32FirstW")
	procProcess32NextW             = kernel32.NewProc("Process32NextW")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procVirtualQueryEx             = kernel32.NewProc("VirtualQueryEx")
	procReadProcessMemory          = kernel32.NewProc("ReadProcessMemory")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
)

type localCredentialSource struct{}

type processEntry32 struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [syscall.MAX_PATH]uint16
}

type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	_                 uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
	_                 uint32
}

func newLocalCredentialSource() credentialSource { return localCredentialSource{} }

func (localCredentialSource) Load(ctx context.Context) (credentials, error) {
	if err := validateWindowsLayouts(); err != nil {
		return credentials{}, ErrUnavailable
	}
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return credentials{}, ErrUnavailable
	}
	root := filepath.Join(appData, "Tencent", "QQMusic")
	configData, err := readBoundedFile(filepath.Join(root, "QQMusicServiceConfig.ini"), maxCredentialFileSize)
	if err != nil {
		return credentials{}, ErrNotLoggedIn
	}
	uin := parseUIN(configData)
	if uin == "" {
		return credentials{}, ErrNotLoggedIn
	}

	tokens := make([]string, 0, maxAuthTokenCandidates)
	seen := make(map[string]struct{}, maxAuthTokenCandidates)
	for _, name := range []string{"SetCookie.dat", "_SetCookie.dat"} {
		data, readErr := readBoundedFile(filepath.Join(root, name), maxCredentialFileSize)
		if readErr != nil {
			continue
		}
		for _, candidate := range authTokenCandidates(data) {
			appendAuthCandidate(&tokens, seen, candidate)
			if len(tokens) >= maxAuthTokenCandidates {
				break
			}
		}
	}
	if len(tokens) == 0 {
		tokens, err = scanQQMusicProcesses(ctx)
		if err != nil && !errors.Is(err, ErrNotLoggedIn) {
			return credentials{}, err
		}
	}
	if len(tokens) == 0 {
		return credentials{}, ErrNotLoggedIn
	}
	return credentials{uin: uin, authTokens: tokens}, nil
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrUnavailable
	}
	return data, nil
}

func scanQQMusicProcesses(parent context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(parent, processScanTimeout)
	defer cancel()
	currentSID, err := currentUserSID()
	if err != nil {
		return nil, ErrUnavailable
	}

	pids, err := qqMusicProcessIDs()
	if err != nil || len(pids) == 0 {
		return nil, ErrNotLoggedIn
	}
	remaining := int64(maxProcessScanBytes)
	for _, pid := range pids {
		if ctx.Err() != nil || remaining <= 0 {
			break
		}
		tokens, scanned := scanProcess(ctx, pid, currentSID, remaining)
		remaining -= scanned
		if len(tokens) > 0 {
			return tokens, nil
		}
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, ErrUnavailable
	}
	return nil, ErrNotLoggedIn
}

func qqMusicProcessIDs() ([]uint32, error) {
	snapshot, _, callErr := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == invalidHandleValue {
		return nil, callErr
	}
	defer closeHandle(snapshot)

	entry := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	ok, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ok == 0 {
		return nil, ErrNotLoggedIn
	}
	var pids []uint32
	for {
		name := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if _, wanted := targetProcessNames[name]; wanted {
			pids = append(pids, entry.ProcessID)
		}
		entry.Size = uint32(unsafe.Sizeof(processEntry32{}))
		next, _, _ := procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if next == 0 {
			break
		}
	}
	return pids, nil
}

func scanProcess(ctx context.Context, pid uint32, currentSID string, budget int64) ([]string, int64) {
	handle, _, _ := procOpenProcess.Call(processQueryInformation|processVMRead, 0, uintptr(pid))
	if handle == 0 {
		return nil, 0
	}
	defer closeHandle(handle)
	if !processOwnedBySID(syscall.Handle(handle), currentSID) || !verifiedQQMusicImage(handle) {
		return nil, 0
	}

	var (
		address uintptr
		total   int64
	)
	for total < budget && ctx.Err() == nil {
		var info memoryBasicInformation
		result, _, _ := procVirtualQueryEx.Call(
			handle,
			address,
			uintptr(unsafe.Pointer(&info)),
			unsafe.Sizeof(info),
		)
		if result == 0 || info.RegionSize == 0 {
			break
		}
		next := info.BaseAddress + info.RegionSize
		if next <= address {
			break
		}
		if readableMemory(info) {
			remaining := budget - total
			if regionBytes := int64(info.RegionSize); regionBytes < remaining {
				remaining = regionBytes
			}
			tokens, scanned := scanMemoryRegion(ctx, handle, info.BaseAddress, remaining)
			total += scanned
			if len(tokens) > 0 {
				return tokens, total
			}
		}
		address = next
	}
	return nil, total
}

func readableMemory(info memoryBasicInformation) bool {
	if info.State != memCommit || info.Protect&pageGuard != 0 || info.Protect&pageNoAccess != 0 {
		return false
	}
	return info.Type == memPrivate || info.Type == memImage
}

func scanMemoryRegion(ctx context.Context, handle, base uintptr, size int64) ([]string, int64) {
	var (
		offset  int64
		total   int64
		overlap []byte
	)
	for offset < size && ctx.Err() == nil {
		chunkSize := int64(processScanChunkSize)
		if size-offset < chunkSize {
			chunkSize = size - offset
		}
		buffer := make([]byte, int(chunkSize))
		var bytesRead uintptr
		_, _, _ = procReadProcessMemory.Call(
			handle,
			base+uintptr(offset),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(len(buffer)),
			uintptr(unsafe.Pointer(&bytesRead)),
		)
		if bytesRead > uintptr(len(buffer)) {
			bytesRead = uintptr(len(buffer))
		}
		if bytesRead > 0 {
			buffer = buffer[:bytesRead]
			window := make([]byte, 0, len(overlap)+len(buffer))
			window = append(window, overlap...)
			window = append(window, buffer...)
			if tokens := jsonAuthTokenCandidates(window); len(tokens) > 0 {
				return tokens, total + int64(bytesRead)
			}
			keep := processScanOverlap
			if len(window) < keep {
				keep = len(window)
			}
			overlap = append(overlap[:0], window[len(window)-keep:]...)
			total += int64(bytesRead)
			offset += int64(bytesRead)
			continue
		}
		offset += chunkSize
	}
	return nil, total
}

func verifiedQQMusicImage(handle uintptr) bool {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	ok, _, _ := procQueryFullProcessImageNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 || size == 0 || int(size) > len(buffer) {
		return false
	}
	imagePath := filepath.Clean(syscall.UTF16ToString(buffer[:size]))
	base := strings.ToLower(filepath.Base(imagePath))
	if _, wanted := targetProcessNames[base]; !wanted {
		return false
	}
	for _, part := range strings.Split(strings.ToLower(imagePath), string(os.PathSeparator)) {
		if part == "qqmusic" {
			return true
		}
	}
	return false
}

func currentUserSID() (string, error) {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return "", ErrUnavailable
	}
	return user.User.Sid.String()
}

func processOwnedBySID(process syscall.Handle, expectedSID string) bool {
	var token syscall.Token
	if err := syscall.OpenProcessToken(process, syscall.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return false
	}
	actualSID, err := user.User.Sid.String()
	return err == nil && actualSID == expectedSID
}

func closeHandle(handle uintptr) {
	if handle != 0 && handle != invalidHandleValue {
		_, _, _ = procCloseHandle.Call(handle)
	}
}

func validateWindowsLayouts() error {
	if unsafe.Sizeof(processEntry32{}) != 568 {
		return fmt.Errorf("unexpected PROCESSENTRY32W size: %d", unsafe.Sizeof(processEntry32{}))
	}
	if unsafe.Sizeof(memoryBasicInformation{}) != 48 {
		return fmt.Errorf("unexpected MEMORY_BASIC_INFORMATION size: %d", unsafe.Sizeof(memoryBasicInformation{}))
	}
	return nil
}
