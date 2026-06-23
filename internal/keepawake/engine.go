//go:build windows

// Package keepawake implements the logic that stops Windows from sleeping
// or locking, offering two switchable mechanisms.
package keepawake

import (
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/rom/wigglewiggle/internal/win"
)

// Mechanism selects how the screen is kept awake.
type Mechanism int

const (
	// OSKeepAwake holds a native Windows "stay awake" assertion. It never
	// touches the mouse or keyboard and keeps the machine up even while
	// you are away.
	OSKeepAwake Mechanism = iota
	// InputSimulation simulates a keypress or a tiny mouse movement, but
	// only after the configured idle period, so it never fights real
	// input. It also keeps chat-app presence "active".
	InputSimulation
)

func (m Mechanism) String() string {
	if m == InputSimulation {
		return "input simulation"
	}
	return "OS keep-awake"
}

// pollEvery is how often the worker re-checks idle time and re-asserts the
// keep-awake state. It bounds how late an idle nudge can be, and is small
// enough to be imperceptible yet cheap for a background app.
const pollEvery = 5 * time.Second

// Engine drives the keep-awake behaviour on a single locked OS thread.
// All exported methods are safe for concurrent use.
type Engine struct {
	mu       sync.Mutex
	mech     Mechanism
	interval time.Duration
	paused   bool
	tick     uint64 // alternation counter for input simulation

	wake chan struct{}
	stop chan struct{}
	done chan struct{}
}

// New creates an Engine with the given starting mechanism and idle interval.
// Call Start to begin and Stop to shut down.
func New(mech Mechanism, interval time.Duration) *Engine {
	return &Engine{
		mech:     mech,
		interval: interval,
		wake:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the worker goroutine.
func (e *Engine) Start() { go e.run() }

// Stop releases any keep-awake assertion and waits for the worker to exit.
func (e *Engine) Stop() {
	close(e.stop)
	<-e.done
}

// SetMechanism switches between OS keep-awake and input simulation.
func (e *Engine) SetMechanism(m Mechanism) {
	e.mu.Lock()
	e.mech = m
	e.mu.Unlock()
	e.signal()
}

// SetInterval changes the idle interval used by input simulation.
func (e *Engine) SetInterval(d time.Duration) {
	e.mu.Lock()
	e.interval = d
	e.mu.Unlock()
	e.signal()
}

// SetPaused suspends or resumes all keep-awake activity.
func (e *Engine) SetPaused(p bool) {
	e.mu.Lock()
	e.paused = p
	e.mu.Unlock()
	e.signal()
}

// Status returns a short human-readable description of the current state.
func (e *Engine) Status() string {
	m, interval, paused := e.snapshot()
	switch {
	case paused:
		return "Paused"
	case m == InputSimulation:
		return "Active — input every " + human(interval) + " when idle"
	default:
		return "Active — OS keep-awake"
	}
}

func (e *Engine) snapshot() (Mechanism, time.Duration, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mech, e.interval, e.paused
}

// signal nudges the worker to re-apply state immediately after a change.
func (e *Engine) signal() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *Engine) run() {
	// SetThreadExecutionState assertions are per-thread and only persist
	// while that thread lives, so pin this goroutine to one OS thread for
	// the whole run.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(e.done)
	defer win.ReleaseKeepAwake()

	e.apply()
	t := time.NewTicker(pollEvery)
	defer t.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-e.wake:
			e.apply()
		case <-t.C:
			e.apply()
		}
	}
}

// apply enforces the current state: it sets or releases the OS assertion and,
// in input-simulation mode, sends a nudge once the idle threshold is reached.
func (e *Engine) apply() {
	m, interval, paused := e.snapshot()

	if paused || m != OSKeepAwake {
		win.ReleaseKeepAwake()
	} else {
		win.KeepAwake()
	}

	if paused || m != InputSimulation {
		return
	}
	if win.IdleMillis() >= uint32(interval/time.Millisecond) {
		e.nudge()
	}
}

// nudge alternates between a keypress and a mouse wiggle so that software
// tracking only one kind of input still sees activity. Sending input also
// resets the system idle timer, which naturally spaces nudges one interval
// apart.
func (e *Engine) nudge() {
	e.mu.Lock()
	n := e.tick
	e.tick++
	e.mu.Unlock()

	if n%2 == 0 {
		win.TapKey(win.VKF15)
	} else {
		win.Wiggle()
	}
}

func human(d time.Duration) string {
	if d < time.Minute {
		return strconv.Itoa(int(d/time.Second)) + "s"
	}
	return strconv.Itoa(int(d/time.Minute)) + "m"
}
