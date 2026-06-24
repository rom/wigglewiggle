// Package win is a thin "translator" layer between our Go code and a handful
// of built-in Windows features. Each function here wraps a single Windows
// system call so the rest of the program can stay readable.
//
// # FOR NON-PROGRAMMERS
//
// Windows offers its features through libraries called DLLs (for example,
// "user32.dll" handles windows, the mouse and the keyboard; "kernel32.dll"
// handles core system things). Below we look up the specific functions we
// need inside those libraries and call them. Everything here works within
// your own signed-in session and needs no administrator rights.
//
// (This file is named win_windows.go. The "_windows" ending tells the Go
// build system to include it only when building for Windows, so no explicit
// build tag is needed.)
package win

import (
	"unsafe" // needed to hand Windows raw memory in the exact shape it expects

	"golang.org/x/sys/windows" // helper for loading DLLs and calling into them
)

// These lines locate the Windows libraries and the specific functions inside
// them that we call later. "Lazy" means each lookup happens the first time we
// actually use it, not up front.
var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")

	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState") // ask Windows to stay awake
	procGetTickCount            = kernel32.NewProc("GetTickCount")            // milliseconds since Windows started
	procSendInput               = user32.NewProc("SendInput")                 // inject fake key/mouse input
	procGetLastInputInfo        = user32.NewProc("GetLastInputInfo")          // when the user last did anything
)

// These flags tell SetThreadExecutionState what kind of "stay awake" we want.
// They are combined together (with "|") when we make the request.
const (
	esContinuous      = 0x80000000 // keep the request in effect until we change it
	esSystemRequired  = 0x00000001 // don't let the computer go to sleep
	esDisplayRequired = 0x00000002 // don't let the display turn off (this is what blocks the lock screen)
)

// KeepAwake asks Windows to keep both the system and the display on. While
// this request stands, Windows keeps resetting its idle timers, so it won't
// sleep, blank the screen, start the screensaver, or show the lock screen.
//
// Note: Windows ties this request to the calling thread, which is why the
// engine always calls it from its single locked thread (see keepawake.run).
func KeepAwake() {
	procSetThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired | esDisplayRequired))
}

// ReleaseKeepAwake cancels the request made by KeepAwake (by asking for
// "continuous, but nothing required"), letting normal sleep/lock resume.
func ReleaseKeepAwake() {
	procSetThreadExecutionState.Call(uintptr(esContinuous))
}

// lastInputInfo mirrors the small Windows structure that GetLastInputInfo
// fills in. cbSize is its size in bytes (Windows requires we tell it that);
// dwTime is the timestamp of the most recent input.
type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// IdleMillis reports how many milliseconds have passed since the last user
// input of any kind (real or faked). The engine uses this to decide when to
// nudge in input-simulation mode.
func IdleMillis() uint32 {
	// Prepare the structure (telling Windows its size) and ask Windows to
	// fill in the time of the last input. If the call fails, report 0 idle.
	lii := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii))); r == 0 {
		return 0
	}
	// "Now" minus "time of last input" = how long you've been idle. Both are
	// counters of milliseconds since Windows started; subtracting them as
	// 32-bit numbers handles the counter rolling over correctly.
	tick, _, _ := procGetTickCount.Call()
	return uint32(tick) - lii.dwTime // uint32 wraparound matches GetTickCount
}

// Codes and flags used when sending fake input via SendInput.
const (
	VKF15 = 0x7E // the F15 key: a real key that virtually no software acts on, so it's harmless

	inputMouse     = 0      // this INPUT describes a mouse event
	inputKeyboard  = 1      // this INPUT describes a keyboard event
	mouseEventMove = 0x0001 // the mouse event is a movement
	keyEventKeyUp  = 0x0002 // the keyboard event is a key being released
)

// The next three types must match, byte-for-byte, the structures Windows'
// SendInput expects in memory. We never read these fields ourselves; we just
// fill them in and hand the memory to Windows.

// mouseInput describes a single mouse event (a movement, in our case).
type mouseInput struct {
	dx, dy    int32  // how far to move (we use +1 then -1 pixel)
	mouseData uint32 // unused here
	dwFlags   uint32 // what kind of mouse event (we use "move")
	time      uint32 // let Windows fill in the timestamp
	dwExtra   uintptr
}

// keybdInput describes a single keyboard event (a key going down or up).
type keybdInput struct {
	wVk     uint16 // which key (a "virtual-key code" such as VKF15)
	wScan   uint16 // unused here
	dwFlags uint32 // e.g. key-up vs key-down
	time    uint32 // let Windows fill in the timestamp
	dwExtra uintptr
}

// input is Windows' INPUT structure. In Windows this is one fixed-size block
// that can hold either a mouse event or a keyboard event. We represent it
// using the larger (mouse) layout and, for keyboard events, write the
// keyboard fields into that same space. The blank "_" field is padding so the
// memory lines up exactly as Windows expects on 64-bit systems.
type input struct {
	typ uint32 // inputMouse or inputKeyboard: tells Windows how to read the rest
	_   uint32
	mi  mouseInput
}

// send hands one or more input events to Windows in a single call. The three
// arguments tell SendInput: how many events there are, where they sit in
// memory, and how big each one is.
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

// TapKey simulates pressing and then releasing a single key. Windows wants
// this as two separate events: key-down, then key-up.
func TapKey(vk uint16) {
	// This helper builds one keyboard event with the given flags. It writes
	// the keyboard fields into the shared memory slot via a pointer
	// conversion (that's what the (*keybdInput)(...) does).
	key := func(flags uint32) input {
		in := input{typ: inputKeyboard}
		k := (*keybdInput)(unsafe.Pointer(&in.mi))
		k.wVk = vk
		k.dwFlags = flags
		return in
	}
	send([]input{key(0), key(keyEventKeyUp)}) // 0 = key down, then key up
}

// Wiggle simulates the tiniest possible mouse movement: one pixel to the
// right and immediately one pixel back, leaving the pointer where it began.
func Wiggle() {
	// Helper to build one "move by dx pixels" mouse event.
	move := func(dx int32) input {
		in := input{typ: inputMouse}
		in.mi.dx = dx
		in.mi.dwFlags = mouseEventMove
		return in
	}
	send([]input{move(1), move(-1)}) // right one, then left one
}
