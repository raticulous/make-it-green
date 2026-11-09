package engine

import (
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	procSendInput          = user32.NewProc("SendInput")
	procPostMessage        = user32.NewProc("PostMessageW")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procSetThreadExecState = kernel32.NewProc("SetThreadExecutionState")
)

// Field layout matches the Win32 INPUT struct (40 bytes on 64-bit, 28 on 32-bit).
type mouseInput struct {
	Type uint32
	Mi   struct {
		Dx, Dy                   int32
		MouseData, DwFlags, Time uint32
		DwExtraInfo              uintptr
	}
}

func nudge() error {
	var in [2]mouseInput
	for i, dx := range []int32{1, -1} {
		in[i].Type = 0          // INPUT_MOUSE
		in[i].Mi.DwFlags = 0x01 // MOUSEEVENTF_MOVE
		in[i].Mi.Dx = dx
	}
	n, _, err := procSendInput.Call(2, uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
	if n != 2 {
		return err
	}
	return nil
}

const (
	esContinuous      = 0x80000000
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002
)

// SetThreadExecutionState is per OS thread, so one locked goroutine holds it.
func inhibitSleep(display bool) (func(), error) {
	flags := uintptr(esContinuous | esSystemRequired)
	if display {
		flags |= esDisplayRequired
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		procSetThreadExecState.Call(flags)
		<-stop
		procSetThreadExecState.Call(esContinuous)
		close(done)
	}()
	return func() { close(stop); <-done }, nil
}

func displayOff() error {
	// HWND_BROADCAST, WM_SYSCOMMAND, SC_MONITORPOWER, 2 = off
	r, _, err := procPostMessage.Call(0xFFFF, 0x0112, 0xF170, 2)
	if r == 0 {
		return err
	}
	return nil
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func setAutostart(enable bool, exe string, args []string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enable {
		if err := k.DeleteValue("make-it-green"); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetStringValue("make-it-green", `"`+exe+`" `+strings.Join(args, " "))
}

func AutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue("make-it-green")
	return err == nil
}
