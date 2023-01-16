package carrot

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	bits "github.com/nvlled/carrot/atombits"
)

// A Coroutine is function that only takes a In argument.
type Coroutine = func(Invoker)

// An Invoker is used to direct the program flow of a coroutine.
//
// Note: Methods may block for one or more several frames,
// except for those labeled with Async.
//
// Note: Methods are all concurrent-safe.
//
// Note: Blocking methods should be called from a coroutine, directly
// or indirectly. Blocking methods will panic() with a ErrCancelled,
// and coroutines will automatically catch this.
type Invoker interface {
	// Returns an id representing the coroutine invocation.
	ID() int64

	// Yield waits until the next call to Update().
	// In other words, Yield() waits for one frame.
	// Panics when cancelled.
	Yield()

	// Delay waits for a number of calls to Update().
	// Panics when cancelled.
	Delay(count int)

	// Sleep blocks and waits for the given duration.
	Sleep(duration time.Duration)

	// Repeatedly yields, and stops when *value is false or nil.
	While(value *bool)

	// Repeatedly yields, and stops when fn returns false.
	WhileFunc(fn func() bool)

	// Repeatedly yields, and stops when *value is true.
	// Similar to While(), but with the condition negated.
	Until(value *bool)

	// Repeatedly yields, and stops when fn returns true.
	// Similar to WhileFunc(), but with the condition negated.
	UntilFunc(func() bool)

	// Cancels the coroutine. Cancels all child coroutines created with
	// StartAsync. This does not affect parent coroutines.
	Cancel()

	Restart()

	// Returns true if current coroutine has been canceled.
	IsCanceled() bool

	IsDone() bool

	// Causes the coroutine to block indefinitely and
	// spiral downwards the endless depths of nothingness, never
	// again to return from the utter blackness of empty void.
	Abyss()

	// Changes the current coroutine to a new one. The old
	// one is cancelled first before running the new coroutine.
	// This is conceptually equivalent to transitions in
	// finite state machines.
	// Similar to script.Transition()
	Transition(Coroutine)

	Logf(string, ...any)

	StartAsync(Coroutine) Invoker
}

type State = uint32

const (
	StateUnknown State = 0b00
	StateRunning State = 0b01
	StateCancel  State = 0b10
)

type PendingAction = uint32

const (
	ActionNone    PendingAction = 0b00
	ActionCancel  PendingAction = 0b01
	ActionRestart PendingAction = 0b10
)

type invoker struct {
	// id of invoker. Mainly used for debugging.
	id int64

	kanata *katana
	script *Script

	state  atomic.Uint32
	action atomic.Uint32

	mainCoroutine Coroutine

	subs   []*invoker
	subsMu sync.RWMutex
}

var idGen = atomic.Int64{}

func NewInvoker() Invoker {
	return newInvoker()
}

func newInvoker() *invoker {
	return &invoker{
		id:     idGen.Add(1),
		kanata: newKatana(),
	}
}

func (in *invoker) initialize(coroutine Coroutine) {
	in.mainCoroutine = coroutine
	in.Logf("created")
	in.Restart()
	go in.loopRunner()

}

func (in *invoker) ID() int64 {
	return in.id
}

func (in *invoker) Yield() {
	in.kanata.YieldRight()
	if bits.IsSet(&in.state, StateCancel) {
		panic(ErrCancelled)
	}
}

func (in *invoker) UntilFunc(fn func() bool) {
	for !fn() {
		in.Yield()
	}
}

func (in *invoker) WhileFunc(fn func() bool) {
	for fn() {
		in.Yield()
	}
}

func (in *invoker) Until(value *bool) {
	for value == nil || !*value {
		in.Yield()
	}
}

func (in *invoker) While(value *bool) {
	for value != nil && *value {
		in.Yield()
	}
}

func (in *invoker) Delay(count int) {
	for i := 0; i < count; i++ {
		in.Yield()
	}
}

func (in *invoker) Sleep(sleepDuration time.Duration) {
	startTime := time.Now()
	for {
		in.Yield()
		elapsed := time.Since(startTime)
		if elapsed.Microseconds() >= sleepDuration.Microseconds() {
			break
		}
	}
}

func (in *invoker) IsRunning() bool {
	return bits.IsSet(&in.state, StateRunning)
}

func (in *invoker) IsCanceled() bool {
	return bits.IsSet(&in.state, StateCancel)
}

func (in *invoker) Cancel() {
	bits.Set(&in.action, ActionCancel)
}

func (in *invoker) Restart() {
	bits.Set(&in.action, ActionRestart)
}

func (in *invoker) setRunning(yes bool) {
	if yes {
		bits.Set(&in.state, StateRunning)
	} else {
		bits.Unset(&in.state, StateRunning)
	}
}

func (in *invoker) applyRestart() {
	bits.Unset(&in.state, StateCancel)
	bits.Unset(&in.action, ActionRestart|ActionCancel)
}
func (in *invoker) applyCancel() {
	bits.Set(&in.state, StateCancel)
	bits.Unset(&in.action, ActionCancel)
}

func (in *invoker) isRestarting() bool { return bits.IsSet(&in.action, ActionRestart) }
func (in *invoker) isCancelling() bool { return bits.IsSet(&in.action, ActionCancel) }

func (in *invoker) Abyss() {
	for {
		in.Yield()
	}
}

func (in *invoker) String() string {
	return fmt.Sprintf("coroutine-%v", in.id)
}

// Use for debugging. Call SetLogging(true) to enable.
func (in *invoker) Logf(format string, args ...any) {
	logFn(in.script, format, args...)
}

func (in *invoker) StartAsync(coroutine Coroutine) Invoker {
	subIn := newInvoker()
	subIn.initialize(coroutine)
	in.subsMu.Lock()
	in.subs = append(in.subs, subIn)
	in.subsMu.Unlock()

	return subIn
}

func (in *invoker) loopRunner() {
	in.setRunning(true)
	for {
		in.Logf("loop start")
		in.kanata.YieldRight()

		in.Logf("coroutine start")
		in.setRunning(true)
		in.startCoroutine()

		in.waitForSubsToEnd()

		in.Logf("coroutine end")
		in.setRunning(false)
	}
}

func (in *invoker) startCoroutine() {
	defer catchCancellation()
	in.mainCoroutine(in)
}

func (in *invoker) waitForSubsToEnd() {
	in.subsMu.RLock()
	subs := in.subs
	in.subsMu.RUnlock()

	for _, s := range subs {
		s.Cancel()
	}

	done := false
	for !done {
		done = true
		for _, s := range subs {
			if !s.IsDone() {
				done = false
				break
			}

		}
		if !done {
			in.kanata.YieldRight()
		}
	}

	in.subsMu.Lock()
	in.subs = in.subs[:0]
	in.subsMu.Unlock()

}

func (in *invoker) update() {
	restartNow := in.isRestarting()
	if in.isCancelling() {
		in.applyCancel()
		restartNow = false
	} else if restartNow {
		bits.Unset(&in.action, ActionRestart)
		in.applyRestart()
	}

	if in.mainCoroutine != nil && (in.IsRunning() || restartNow) {
		in.kanata.YieldLeft()
	}

	in.subsMu.RLock()
	subs := in.subs
	in.subsMu.RUnlock()
	for _, sub := range subs {
		sub.update()
	}
}

func (in *invoker) IsDone() bool {
	return !in.IsRunning() && !in.isRestarting()
}

func (in *invoker) Transition(newCoroutine Coroutine) {
	in.mainCoroutine = newCoroutine
	in.Restart()
	in.Cancel()
}
