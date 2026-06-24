//go:build windows

// Package keepawake contains the actual "don't let the screen lock" logic,
// kept separate from the tray menu (which lives in the top-level main
// package).
//
// # FOR NON-PROGRAMMERS
//
// Think of this package as a little engine with a control panel. The tray
// menu flips the switches (which method to use, how often, pause/resume) and
// this engine does the work quietly in the background. It offers two methods:
//
//   - "OS keep-awake": politely asks Windows itself to stay awake. Nothing
//     moves on screen; Windows simply won't sleep or lock while we ask.
//   - "Input simulation": pretends you pressed a key or nudged the mouse, but
//     only after you've genuinely been idle for a while, so it never gets in
//     your way while you're actually using the computer.
package keepawake

import (
	"runtime" // lets us pin our work to a single Windows "thread" (explained in run)
	"strconv" // turns numbers into text for the status line
	"sync"    // provides a "lock" so two background tasks don't clash
	"time"    // time lengths and a repeating timer

	"github.com/rom/wigglewiggle/internal/win" // our thin wrappers around Windows features
)

// Mechanism is simply a label for which of the two methods is active. Using a
// named type instead of a bare number lets the rest of the code read clearly
// (OSKeepAwake vs InputSimulation).
type Mechanism int

const (
	// OSKeepAwake asks Windows to stay awake. It never touches the mouse or
	// keyboard, and keeps the machine up even while you are away from it.
	OSKeepAwake Mechanism = iota
	// InputSimulation fakes a keypress or a tiny mouse movement, but only
	// after the chosen idle period, so it never fights your real typing or
	// mouse use. It also keeps chat apps (Teams/Slack) showing you as active.
	InputSimulation
)

// String gives each Mechanism a readable name, used in messages.
func (m Mechanism) String() string {
	if m == InputSimulation {
		return "input simulation"
	}
	return "OS keep-awake"
}

// pollEvery is how often the engine "wakes up" on its own to check things:
// re-confirm the stay-awake request and, in input mode, see whether you've
// been idle long enough to need a nudge. Five seconds is frequent enough to
// feel instant yet light enough to be unnoticeable.
const pollEvery = 5 * time.Second

// Engine is the keep-awake engine itself. It holds the current settings and
// runs a background worker that keeps reality matching them.
//
// Several fields exist to do this safely in the background:
//   - mu is a "mutex" (a lock). Because the menu (one task) and the worker
//     (another task) both touch these settings, the lock ensures only one of
//     them reads or changes them at any instant, preventing corruption.
//   - wake, stop and done are "channels": think of them as little one-way
//     pipes for sending signals between tasks (e.g. "settings changed, wake
//     up" or "please shut down").
type Engine struct {
	mu       sync.Mutex    // protects the four settings below
	mech     Mechanism     // which method is active
	interval time.Duration // idle time before input simulation acts
	paused   bool          // true while everything is paused
	tick     uint64        // counts nudges, used to alternate key vs mouse

	wake chan struct{} // "settings changed, re-check now"
	stop chan struct{} // "shut down the worker"
	done chan struct{} // worker closes this to confirm it has stopped
}

// New creates a ready-to-use Engine with a starting method and idle interval.
// It does not begin working yet — call Start for that.
func New(mech Mechanism, interval time.Duration) *Engine {
	return &Engine{
		mech:     mech,
		interval: interval,
		wake:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the background worker. "go e.run()" runs e.run() as a
// separate, concurrent task, so Start returns immediately.
func (e *Engine) Start() { go e.run() }

// Stop shuts the worker down and waits for it to finish. Closing the stop
// channel signals the worker to quit; "<-e.done" then blocks until the worker
// confirms it has cleaned up (including cancelling the stay-awake request).
func (e *Engine) Stop() {
	close(e.stop)
	<-e.done
}

// SetMechanism switches between the two methods. It locks, updates the
// setting, unlocks, then signals the worker to apply the change right away.
func (e *Engine) SetMechanism(m Mechanism) {
	e.mu.Lock()
	e.mech = m
	e.mu.Unlock()
	e.signal()
}

// SetInterval changes how long you must be idle before input simulation acts.
// Same lock-update-signal pattern as SetMechanism.
func (e *Engine) SetInterval(d time.Duration) {
	e.mu.Lock()
	e.interval = d
	e.mu.Unlock()
	e.signal()
}

// SetPaused pauses (true) or resumes (false) all keep-awake activity.
func (e *Engine) SetPaused(p bool) {
	e.mu.Lock()
	e.paused = p
	e.mu.Unlock()
	e.signal()
}

// Status returns a short sentence describing what the engine is doing right
// now, e.g. "Active — OS keep-awake" or "Paused". The tray menu shows this.
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

// snapshot safely reads all three settings at once under the lock and returns
// copies, so the worker can act on a consistent set of values without holding
// the lock while it works.
func (e *Engine) snapshot() (Mechanism, time.Duration, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mech, e.interval, e.paused
}

// signal pokes the worker awake after a settings change. The "select" with a
// "default" makes this non-blocking: if a poke is already waiting in the
// pipe, we don't add another (one is enough to trigger a re-check).
func (e *Engine) signal() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// run is the heart of the engine: a loop that runs in the background for the
// whole life of the program, keeping things in line with the settings.
func (e *Engine) run() {
	// Why "lock to one OS thread"? Windows remembers our stay-awake request
	// per "thread" (a single line of execution), and forgets it if that
	// thread ends. Normally Go is free to move our code between threads,
	// which could silently drop the request. LockOSThread pins this loop to
	// one thread for its entire life, so the request always sticks.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// "defer" schedules cleanup to run when this function returns:
	//   - close(e.done) tells Stop() we've finished;
	//   - win.ReleaseKeepAwake() cancels any stay-awake request so the PC can
	//     sleep/lock normally again.
	// Defers run in reverse order, so ReleaseKeepAwake happens first.
	defer close(e.done)
	defer win.ReleaseKeepAwake()

	e.apply() // do the right thing immediately on startup

	// A ticker fires on the channel t.C every pollEvery seconds.
	t := time.NewTicker(pollEvery)
	defer t.Stop()

	// Loop forever, waiting for one of three things to happen and reacting:
	for {
		select {
		case <-e.stop: // asked to shut down -> leave the loop (cleanup runs)
			return
		case <-e.wake: // a setting changed -> re-apply now
			e.apply()
		case <-t.C: // the regular timer ticked -> re-apply / re-check
			e.apply()
		}
	}
}

// apply makes reality match the current settings. It is called on startup,
// whenever a setting changes, and on every timer tick.
func (e *Engine) apply() {
	m, interval, paused := e.snapshot()

	// Part 1: the OS stay-awake request. We want it ON only when we're not
	// paused and we're in OS keep-awake mode; otherwise make sure it's OFF.
	if paused || m != OSKeepAwake {
		win.ReleaseKeepAwake()
	} else {
		win.KeepAwake()
	}

	// Part 2: input simulation. Only relevant when not paused and in input
	// mode; otherwise there's nothing more to do.
	if paused || m != InputSimulation {
		return
	}
	// win.IdleMillis() is how long since you last touched the computer, in
	// milliseconds. If that's reached the chosen interval, send a nudge.
	if win.IdleMillis() >= uint32(interval/time.Millisecond) {
		e.nudge()
	}
}

// nudge sends one piece of fake activity. It alternates between a keypress and
// a mouse wiggle so that software watching only one of the two still sees you
// as active. Sending input also resets the system's idle timer, which neatly
// spaces the next nudge a full interval later.
func (e *Engine) nudge() {
	// Read and increment the counter under the lock, then act outside it.
	e.mu.Lock()
	n := e.tick
	e.tick++
	e.mu.Unlock()

	if n%2 == 0 { // even counts -> tap a key
		win.TapKey(win.VKF15)
	} else { // odd counts -> wiggle the mouse
		win.Wiggle()
	}
}

// human turns a length of time into a short label for the status line, e.g.
// 30 seconds -> "30s" and 2 minutes -> "2m".
func human(d time.Duration) string {
	if d < time.Minute {
		return strconv.Itoa(int(d/time.Second)) + "s"
	}
	return strconv.Itoa(int(d/time.Minute)) + "m"
}
