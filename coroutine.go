package carrot

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	bits "github.com/nvlled/carrot/atombits"
)

// A Coroutine is function that only takes an *Invoker argument.
type Coroutine = func(*Invoker)

type coState = uint32

const (
	stateUnknown coState = 0b00
	stateRunning coState = 0b01
	stateCancel  coState = 0b10
)

type coAction = uint32

const (
	actionNone    coAction = 0b00
	actionCancel  coAction = 0b01
	actionRestart coAction = 0b10
)

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
type Invoker struct {
	// ID of invoker. Mainly used for debugging.
	ID int64

	kanata *katana

	state  atomic.Uint32
	action atomic.Uint32

	coroutine Coroutine

	subs   []*Invoker
	subsMu sync.RWMutex
}

var idGen = atomic.Int64{}

func NewInvoker() *Invoker {
	in := &Invoker{
		ID:     idGen.Add(1),
		kanata: newKatana(),
	}
	go in.loopRunner()
	return in
}

// Yield waits until the next call to Update().
// In other words, Yield() waits for one frame.
// Panics when cancelled.
func (in *Invoker) Yield() {
	in.kanata.YieldRight()
	if in.isCanceled() {
		panic(ErrCancelled)
	}
}

// Delay waits for a number of calls to Update().
// Panics when cancelled.
func (in *Invoker) Delay(count int) {
	for i := 0; i < count; i++ {
		in.Yield()
	}
}

// Sleep blocks and waits for the given duration.
func (in *Invoker) Sleep(sleepDuration time.Duration) {
	// time.Sleep isn't used here to allow immediate cancellation
	startTime := time.Now()
	for {
		in.Yield()
		elapsed := time.Since(startTime)
		if elapsed.Microseconds() >= sleepDuration.Microseconds() {
			break
		}
	}
}

// Repeatedly yields, and stops when *value is false or nil.
func (in *Invoker) While(value *bool) {
	for value != nil && *value {
		in.Yield()
	}
}

// Repeatedly yields, and stops when fn returns false.
func (in *Invoker) WhileFunc(fn func() bool) {
	for fn() {
		in.Yield()
	}
}

// Repeatedly yields, and stops when *value is true.
// Similar to While(), but with the condition negated.
func (in *Invoker) Until(value *bool) {
	for value == nil || !*value {
		in.Yield()
	}
}

// Repeatedly yields, and stops when fn returns true.
// Similar to WhileFunc(), but with the condition negated.
func (in *Invoker) UntilFunc(fn func() bool) {
	for !fn() {
		in.Yield()
	}
}

// Causes the coroutine to block indefinitely and
// spiral downwards the endless depths of nothingness, never
// again to return from the utter blackness of empty void.
func (in *Invoker) Abyss() {
	for {
		in.Yield()
	}
}

// Returns true if the coroutine is still running,
// meaning the coroutine function hasn't returned.
func (in *Invoker) IsRunning() bool {
	return bits.IsSet(&in.state, stateRunning)
}

// Returns true it's not IsRunning() and is not
// flagged for Restart().
func (in *Invoker) IsDone() bool {
	return !in.IsRunning() && !in.isRestarting()
}

// Cancels the coroutine. Also cancels all child coroutines created with
// StartAsync. This does not affect parent coroutines.
// Note: Cancel() won't immediately take effect.
// Actual cancellation will be done on next Update().
func (in *Invoker) Cancel() {
	bits.Set(&in.action, actionCancel)
}

// Restarts the coroutine. If the coroutine still running,
// it is cancelled first.
// Note: Restart() won't immediately take effect.
// Actual restart will be done on next Update().
func (in *Invoker) Restart() {
	bits.Set(&in.action, actionRestart)
}

// Changes the current coroutine to a new one. If there is
// a current coroutine running, it is cancelled first.
// This is conceptually equivalent to transitions in
// finite state machines.
func (in *Invoker) Transition(newCoroutine Coroutine) {
	in.coroutine = newCoroutine
	in.Restart()
	in.Cancel()
}

// Starts a new child coroutine asynchronously. The child
// coroutine will be automatically cancelled when the current
// coroutine ends and is no longer IsRunning().
// To explicitly wait for the child coroutine to finish, use
// any preferred synchronization method, or do something
// like
//
//	in.UntilFunc(childIn.IsDone)
//
// See also the test functions TestAsync* for a more thorough
// example.
func (in *Invoker) StartAsync(coroutine Coroutine) *Invoker {
	subIn := summonInvoker()
	subIn.initialize(coroutine)
	in.subsMu.Lock()
	in.subs = append(in.subs, subIn)
	in.subsMu.Unlock()

	return subIn
}

// Use for debugging. Call SetLogging(true) to enable.
func (in *Invoker) Logf(format string, args ...any) {
	logFn(in, format, args...)
}

func (in *Invoker) String() string {
	return fmt.Sprintf("coroutine-%v", in.ID)
}

func (in *Invoker) setRunning(yes bool) {
	if yes {
		bits.Set(&in.state, stateRunning)
	} else {
		bits.Unset(&in.state, stateRunning)
	}
}

func (in *Invoker) applyRestart() {
	bits.Unset(&in.state, stateCancel)
	bits.Unset(&in.action, actionRestart|actionCancel)
}
func (in *Invoker) applyCancel() {
	bits.Set(&in.state, stateCancel)
	bits.Unset(&in.action, actionCancel)
}

func (in *Invoker) isRestarting() bool { return bits.IsSet(&in.action, actionRestart) }
func (in *Invoker) isCancelling() bool { return bits.IsSet(&in.action, actionCancel) }

func (in *Invoker) loopRunner() {
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

func (in *Invoker) startCoroutine() {
	defer catchCancellation()
	in.coroutine(in)
}

func (in *Invoker) waitForSubsToEnd() {
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

	for _, s := range subs {
		disperseInvoker(s)
	}

}

func (in *Invoker) update() {
	restartNow := in.isRestarting()
	if in.isCancelling() {
		in.applyCancel()
		restartNow = false
	} else if restartNow {
		bits.Unset(&in.action, actionRestart)
		in.applyRestart()
	}

	if in.coroutine != nil && (in.IsRunning() || restartNow) {
		in.kanata.YieldLeft()
	}

	in.subsMu.RLock()
	subs := in.subs
	in.subsMu.RUnlock()
	for _, sub := range subs {
		sub.update()
	}
}

func (in *Invoker) initialize(coroutine Coroutine) {
	in.coroutine = coroutine
	in.Logf("created")
	in.Restart()

}

func (in *Invoker) isCanceled() bool {
	return bits.IsSet(&in.state, stateCancel)
}
