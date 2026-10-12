package platform

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                               = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassExW                 = user32.NewProc("RegisterClassExW")
	procCreateWindowExW                  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                   = user32.NewProc("DefWindowProcW")
	procGetMessageW                      = user32.NewProc("GetMessageW")
	procTranslateMessage                 = user32.NewProc("TranslateMessage")
	procDispatchMessageW                 = user32.NewProc("DispatchMessageW")
	procRegisterPowerSettingNotification = user32.NewProc("RegisterPowerSettingNotification")
	procRegisterSuspendResumeNotif       = user32.NewProc("RegisterSuspendResumeNotification")
)

const (
	wmPowerBroadcast      = 0x0218
	pbtAPMSuspend         = 0x0004
	pbtAPMResumeSuspend   = 0x0007
	pbtAPMResumeAutomatic = 0x0012
	pbtPowerSettingChange = 0x8013
)

// GUID_CONSOLE_DISPLAY_STATE: 0 = off, 1 = on, 2 = dimmed.
var guidConsoleDisplayState = windows.GUID{
	Data1: 0x6fe69556, Data2: 0x704a, Data3: 0x47a0,
	Data4: [8]byte{0x8f, 0x24, 0xc2, 0x8d, 0x93, 0x6f, 0xda, 0x47},
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type msg struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type powerBroadcastSetting struct {
	PowerSetting windows.GUID
	DataLength   uint32
	Data         [1]byte
}

// WatchPower starts a hidden window on its own OS thread and reports display
// and sleep transitions. The initial display state that Windows sends right
// after registration is not reported.
func WatchPower(h PowerHandler) error {
	errc := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		hwnd, err := createPowerWindow(h)
		if err != nil {
			errc <- err
			return
		}
		r, _, e := procRegisterPowerSettingNotification.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&guidConsoleDisplayState)), 0)
		if r == 0 {
			errc <- fmt.Errorf("RegisterPowerSettingNotification: %w", e)
			return
		}
		// Needed on Modern Standby systems; harmless elsewhere.
		procRegisterSuspendResumeNotif.Call(uintptr(hwnd), 0)
		errc <- nil

		var m msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	return <-errc
}

func createPowerWindow(h PowerHandler) (windows.HWND, error) {
	className, _ := windows.UTF16PtrFromString("TvTrayPowerWatcher")
	var inst windows.Handle
	windows.GetModuleHandleEx(0, nil, &inst)

	initial := true
	lastDisplay := -1
	// lParam is unsafe.Pointer (pointer-sized, allowed by NewCallback) so the
	// PBT_POWERSETTINGCHANGE payload can be read without uintptr conversion.
	wndProc := func(hwnd windows.HWND, message uint32, wParam uintptr, lParam unsafe.Pointer) uintptr {
		if message == wmPowerBroadcast {
			switch wParam {
			case pbtAPMSuspend:
				h(Suspend)
			case pbtAPMResumeSuspend, pbtAPMResumeAutomatic:
				// Both arrive on a user-initiated wake; the controller dedups.
				h(Resume)
			case pbtPowerSettingChange:
				s := (*powerBroadcastSetting)(lParam)
				if s.PowerSetting == guidConsoleDisplayState && s.DataLength >= 1 {
					state := int(s.Data[0])
					if state == 2 { // dimmed counts as on
						state = 1
					}
					if initial {
						initial = false
					} else if state != lastDisplay {
						if state == 0 {
							h(DisplayOff)
						} else {
							h(DisplayOn)
						}
					}
					lastDisplay = state
				}
			}
			return 1
		}
		r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, uintptr(lParam))
		return r
	}

	wc := wndClassEx{
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     inst,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return 0, fmt.Errorf("RegisterClassEx: %w", e)
	}
	// A top-level (never shown) window: message-only windows don't get
	// WM_POWERBROADCAST suspend/resume broadcasts.
	hwnd, _, e := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0, 0, 0, uintptr(inst), 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowEx: %w", e)
	}
	return windows.HWND(hwnd), nil
}
