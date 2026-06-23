// Package win wraps the handful of Win32 calls wigglewiggle needs. Every
// call here operates within the current user session and requires no
// administrator or SYSTEM privileges.
package win

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")

	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")
	procGetTickCount            = kernel32.NewProc("GetTickCount")
	procSendInput               = user32.NewProc("SendInput")
	procGetLastInputInfo        = user32.NewProc("GetLastInputInfo")
)

// EXECUTION_STATE flags for SetThreadExecutionState.
const (
	esContinuous      = 0x80000000
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002
)

// KeepAwake tells Windows that the system and display are in use, resetting
// the idle timers that would otherwise sleep the machine or trigger the
// lock screen. The assertion is tied to the calling OS thread and stays in
// effect until ReleaseKeepAwake (or the thread exits), so the caller must
// keep invoking this from the same locked thread.
func KeepAwake() {
	procSetThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired | esDisplayRequired))
}

// ReleaseKeepAwake clears any standing keep-awake assertion on the calling
// thread, letting the normal idle timers resume.
func ReleaseKeepAwake() {
	procSetThreadExecutionState.Call(uintptr(esContinuous))
}

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// IdleMillis reports how long, in milliseconds, since the last user input
// (real or synthetic) anywhere on the desktop.
func IdleMillis() uint32 {
	lii := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii))); r == 0 {
		return 0
	}
	tick, _, _ := procGetTickCount.Call()
	return uint32(tick) - lii.dwTime // uint32 wraparound matches GetTickCount
}

// Virtual-key codes and SendInput constants.
const (
	VKF15 = 0x7E // F15: a real key that virtually no software acts on

	inputMouse     = 0
	inputKeyboard  = 1
	mouseEventMove = 0x0001
	keyEventKeyUp  = 0x0002
)

type mouseInput struct {
	dx, dy    int32
	mouseData uint32
	dwFlags   uint32
	time      uint32
	dwExtra   uintptr
}

type keybdInput struct {
	wVk     uint16
	wScan   uint16
	dwFlags uint32
	time    uint32
	dwExtra uintptr
}

// input mirrors the Win32 INPUT struct. The union is represented by its
// largest member (mouseInput); keyboard events overlay keybdInput onto it.
// The explicit padding keeps the union 8-byte aligned on amd64.
type input struct {
	typ uint32
	_   uint32
	mi  mouseInput
}

func send(events []input) {
	if len(events) == 0 {
		return
	}
	procSendInput.Call(
		uintptr(len(events)),
		uintptr(unsafe.Pointer(&events[0])),
		unsafe.Sizeof(events[0]),
	)
}

// TapKey presses and releases a single virtual key.
func TapKey(vk uint16) {
	key := func(flags uint32) input {
		in := input{typ: inputKeyboard}
		k := (*keybdInput)(unsafe.Pointer(&in.mi))
		k.wVk = vk
		k.dwFlags = flags
		return in
	}
	send([]input{key(0), key(keyEventKeyUp)})
}

// Wiggle nudges the mouse one pixel and immediately back, leaving the
// cursor where it started.
func Wiggle() {
	move := func(dx int32) input {
		in := input{typ: inputMouse}
		in.mi.dx = dx
		in.mi.dwFlags = mouseEventMove
		return in
	}
	send([]input{move(1), move(-1)})
}
