//go:build windows

package main

import (
	"fmt"
	goruntime "runtime"
	"syscall"
	"time"
	"unsafe"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW           = user32.NewProc("FindWindowW")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procShowWindow            = user32.NewProc("ShowWindow")
	procFlashWindowEx         = user32.NewProc("FlashWindowEx")
	singleInstanceMutexHandle windows.Handle
)

// ───── 窗口句柄（Wails v2 不暴露 HWND，按标题查找） ─────

func findWindowByTitle(title string) uintptr {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	return hwnd
}

func windowIsForeground(hwnd uintptr) bool {
	foreground, _, _ := procGetForegroundWindow.Call()
	return foreground != 0 && foreground == hwnd
}

// ───── 单实例 ─────

const singleInstanceMutexName = `Local\KugoMusicConverter.SingleInstance`

// enforceSingleInstance 返回释放函数和是否允许继续启动。
// 已存在实例时把已有窗口带到前台并返回 ok=false。
func enforceSingleInstance(windowTitle string) (release func(), ok bool) {
	namePtr, err := windows.UTF16PtrFromString(singleInstanceMutexName)
	if err != nil {
		return func() {}, true
	}
	handle, err := windows.CreateMutex(nil, true, namePtr)
	if err == windows.ERROR_ALREADY_EXISTS {
		if hwnd := findWindowByTitle(windowTitle); hwnd != 0 {
			const swRestore = 9
			procShowWindow.Call(hwnd, swRestore)
			procSetForegroundWindow.Call(hwnd)
		}
		return func() {}, false
	}
	if err != nil {
		// 互斥体创建失败不应阻止应用启动
		return func() {}, true
	}
	singleInstanceMutexHandle = handle
	return func() {
		_ = windows.ReleaseMutex(handle)
		_ = windows.CloseHandle(handle)
	}, true
}

// ───── 任务栏进度（ITaskbarList3，COM 调用全部在锁定线程的派发队列执行） ─────

const (
	tbpfNoProgress    = 0
	tbpfIndeterminate = 1
	tbpfNormal        = 2
	tbpfError         = 4
)

var (
	clsidTaskbarList = ole.NewGUID("{56FDF344-FD6D-11D0-958A-006097C9A090}")
	iidITaskbarList3 = ole.NewGUID("{EA1AFB91-9E28-4B86-90E9-9E9F8A5EEFAF}")
)

type taskbarList3Vtbl struct {
	QueryInterface       uintptr
	AddRef               uintptr
	Release              uintptr
	HrInit               uintptr
	AddTab               uintptr
	DeleteTab            uintptr
	ActivateTab          uintptr
	SetActiveAlt         uintptr
	MarkFullscreenWindow uintptr
	SetProgressValue     uintptr
	SetProgressState     uintptr
}

type taskbarList3 struct{ vtbl *taskbarList3Vtbl }

var (
	taskbarQueue = make(chan func(), 16)
	taskbarObj   *taskbarList3
)

func init() {
	go func() {
		goruntime.LockOSThread()
		_ = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_DISABLE_OLE1DDE)
		for fn := range taskbarQueue {
			fn()
		}
	}()
}

func withTaskbar(fn func(*taskbarList3)) {
	taskbarQueue <- func() {
		if taskbarObj == nil {
			unk, err := ole.CreateInstance(clsidTaskbarList, iidITaskbarList3)
			if err == nil && unk != nil {
				taskbarObj = (*taskbarList3)(unsafe.Pointer(unk))
			}
		}
		if taskbarObj != nil {
			fn(taskbarObj)
		}
	}
}

func taskbarSetProgress(hwnd uintptr, percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	withTaskbar(func(t *taskbarList3) {
		_, _, _ = syscall.SyscallN(t.vtbl.SetProgressValue, uintptr(unsafe.Pointer(t)), hwnd, uintptr(uint64(percent)), uintptr(100))
	})
}

func taskbarSetState(hwnd uintptr, state uint32) {
	withTaskbar(func(t *taskbarList3) {
		_, _, _ = syscall.SyscallN(t.vtbl.SetProgressState, uintptr(unsafe.Pointer(t)), hwnd, uintptr(state))
	})
}

// ───── 窗口闪烁 ─────

type flashWInfo struct {
	cbSize  uint32
	hwnd    uintptr
	flags   uint32
	count   uint32
	timeout uint32
}

func flashWindow(hwnd uintptr) {
	const (
		flashwAll       = 0x3
		flashwTimerNoFG = 0xC
	)
	info := flashWInfo{
		cbSize:  uint32(unsafe.Sizeof(flashWInfo{})),
		hwnd:    hwnd,
		flags:   flashwAll | flashwTimerNoFG,
		count:   3,
		timeout: 0,
	}
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&info)))
}

// ───── Toast 通知 ─────

func showToastNotification(title, body string) {
	notification := &toast.Notification{
		AppID: "Kugo Music Converter",
		Title: title,
		Body:  body,
	}
	_ = notification.Push()
}

// ───── 应用级原生集成门面 ─────

// initNativeIntegration 在窗口创建后解析 HWND（窗口创建可能晚于 startup 回调）。
func (a *App) initNativeIntegration() {
	go func() {
		for attempt := 0; attempt < 40; attempt++ {
			if hwnd := findWindowByTitle(windowTitle); hwnd != 0 {
				a.mu.Lock()
				a.nativeHwnd = hwnd
				a.mu.Unlock()
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()
}

func (a *App) hwnd() uintptr {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.nativeHwnd
}

func (a *App) nativeTaskStart() {
	hwnd := a.hwnd()
	if hwnd == 0 {
		return
	}
	a.mu.Lock()
	a.nativePercent = 0
	a.mu.Unlock()
	taskbarSetState(hwnd, tbpfNormal)
	taskbarSetProgress(hwnd, 0)
}

func (a *App) nativeTaskProgress(percent int) {
	hwnd := a.hwnd()
	if hwnd == 0 {
		return
	}
	a.mu.Lock()
	if percent == a.nativePercent {
		a.mu.Unlock()
		return
	}
	a.nativePercent = percent
	a.mu.Unlock()
	taskbarSetProgress(hwnd, percent)
}

func (a *App) nativeTaskComplete(cancelled bool, success int, failed int) {
	hwnd := a.hwnd()
	if hwnd == 0 {
		return
	}
	taskbarSetState(hwnd, tbpfNoProgress)
	if windowIsForeground(hwnd) {
		return
	}
	flashWindow(hwnd)
	title := "转换完成"
	if cancelled {
		title = "转换已取消"
	}
	showToastNotification(title, fmt.Sprintf("成功 %d 个，失败 %d 个。", success, failed))
}

func (a *App) nativeTaskFailed() {
	hwnd := a.hwnd()
	if hwnd == 0 {
		return
	}
	taskbarSetState(hwnd, tbpfError)
	if windowIsForeground(hwnd) {
		return
	}
	flashWindow(hwnd)
	showToastNotification("转换失败", "转换任务遇到错误，请在应用内查看详情。")
}
