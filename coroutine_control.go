package carrot

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	bits "github.com/nvlled/carrot/atombits"
)

// An Control is used to direct the program flow of a coroutine.
//
//	Note: Methods are all concurrent-safe.
//
//	Note: Methods may block for one or more several frames,
//	except for those labeled with Async.
//
//	Note: Control methods should be only called within a coroutine
//	since yield methods will panic with ErrCancelled when cancelled.
//	This error will automatically be handled inside a coroutine,
//	no need to try to recover from this.
type Control struct {
	// ID of invoker. Mainly used for debugging.
	ID int64

	kanata *katana

	state  atomic.Uint32
	action atomic.Uint32

	coroutine Coroutine

	subControls   []*Control
	subControlsMu sync.RWMutex

	tempSubControls []*Control
}

// A SubControl is a limited Control
// that is returned from the StartAsync method.
//
//	Note: You can cast SubControl to *Control if you
//	need to use Yield*() methods from a parent
//	coroutine, but this can lead to deadlocks or
//	unexpected behaviour is misused.
type SubControl interface {
	Cancel()
	Restart()
	Transition(Coroutine)
	IsRunning() bool
	IsDone() bool
}

// A Coroutine is function that only takes an *Control argument.
type Coroutine = func(*Control)

type coState = uint32

const (
	stateUnknown  coState = 0b000
	stateRunning  coState = 0b001
	stateStopping coState = 0b010
	stateCancel   coState = 0b100
)

type coAction = uint32

const (
	actionNone    coAction = 0b00
	actionCancel  coAction = 0b01
	actionRestart coAction = 0b10
)

var idGen = atomic.Int64{}

func NewControl() *Control {
	ctrl := &Control{
		ID:     idGen.Add(1),
		kanata: newKatana(),
	}
	go ctrl.loopRunner()
	return ctrl
}

// Yield waits until the next call to Update().
// In other words, Yield() waits for one frame.
// Panics when cancelled.
func (ctrl *Control) Yield() {
	ctrl.kanata.YieldRight()
	if ctrl.isCanceled() {
		panic(ErrCancelled)
	}
}

// Delay waits for a number of calls to Update().
// Panics when cancelled.
func (ctrl *Control) Delay(count int) {
	for i := 0; i < count; i++ {
		ctrl.Yield()
	}
}

// Sleep blocks and waits for the given duration.
//
//	Note: Actual sleep duration might be off by several milliseconds,
//	depending on your update FPS. Minimum sleep duration will be
//	the frame duration.
func (ctrl *Control) Sleep(sleepDuration time.Duration) {
	// time.Sleep isn't used here to allow immediate cancellation
	startTime := time.Now()
	for {
		ctrl.Yield()
		elapsed := time.Since(startTime)
		if elapsed.Microseconds() >= sleepDuration.Microseconds() {
			break
		}
	}
}

// Repeatedly yields, and stops when *value is false or nil.
func (ctrl *Control) YieldWhileVar(value *bool) {
	for value != nil && *value {
		ctrl.Yield()
	}
}

// Repeatedly yields, and stops when fn returns false.
func (ctrl *Control) YieldWhile(fn func() bool) {
	for fn() {
		ctrl.Yield()
	}
}

// Repeatedly yields, and stops when *value is true.
// Similar to While(), but with the condition negated.
func (ctrl *Control) YieldUntilVar(value *bool) {
	for value == nil || !*value {
		ctrl.Yield()
	}
}

// Repeatedly yields, and stops when fn returns true.
// Similar to WhileFunc(), but with the condition negated.
func (ctrl *Control) YieldUntil(fn func() bool) {
	for !fn() {
		ctrl.Yield()
	}
}

// Causes the coroutine to block indefinitely and
// spiral downwards the endless depths of nothingness, never
// again to return from the utter blackness of empty void.
func (ctrl *Control) Abyss() {
	for {
		ctrl.Yield()
	}
}

// Returns true if the coroutine is still running,
// meaning the coroutine function hasn't returned.
func (ctrl *Control) IsRunning() bool {
	return bits.IsSet(&ctrl.state, stateRunning)
}

// Returns true it's not IsRunning() and is not
// flagged for Restart().
func (ctrl *Control) IsDone() bool {
	return !ctrl.IsRunning() && !ctrl.isRestarting()
}

// Cancels the coroutine. Also cancels all child coroutines created with
// StartAsync. This does not affect parent coroutines.
//
//	Note: Cancel() won't immediately take effect.
//	Actual cancellation will be done on next Update().
func (ctrl *Control) Cancel() {
	ctrl.action.Store(actionCancel)
}

// Restarts the coroutine. If the coroutine still running,
// it is cancelled first.
//
//	Note: Restart() won't immediately take effect.
//	Actual restart will be done on next Update().
func (ctrl *Control) Restart() {
	bits.Set(&ctrl.action, actionRestart)
}

// Changes the current coroutine to a new one. If there is
// a current coroutine running, it is cancelled first.
// This is conceptually equivalent to transitions in
// finite state machines.
func (ctrl *Control) Transition(newCoroutine Coroutine) {
	ctrl.coroutine = newCoroutine
	ctrl.Cancel()
	ctrl.Restart()
}

// Starts a new child coroutine asynchronously. The child
// coroutine will be automatically cancelled when the current
// coroutine ends and is no longer IsRunning().
// To explicitly wait for the child coroutine to finish, use
// any preferred synchronization method, or do somethinIsRunning
// like
//
//	ctrl.YieldUntil(childIn.IsDone)
//
// See also the test functions TestAsync* for a more thorough
// example.
func (ctrl *Control) StartAsync(coroutine Coroutine) SubControl {
	subIn := allocCoroutine()
	subIn.initialize(coroutine)
	ctrl.subControlsMu.Lock()
	ctrl.subControls = append(ctrl.subControls, subIn)
	ctrl.subControlsMu.Unlock()

	return subIn
}

// Use for debugging. Call SetLogging(true) to enable.
func (ctrl *Control) Logf(format string, args ...any) {
	logFn(ctrl, format, args...)
}

func (ctrl *Control) String() string {
	return fmt.Sprintf("coroutine-%v", ctrl.ID)
}

func (ctrl *Control) setRunning(yes bool) {
	if yes {
		bits.Set(&ctrl.state, stateRunning)
	} else {
		bits.Unset(&ctrl.state, stateRunning)
	}
}

func (ctrl *Control) applyRestart() {
	bits.Unset(&ctrl.state, stateCancel)
	bits.Unset(&ctrl.action, actionRestart|actionCancel)
}
func (ctrl *Control) applyCancel() {
	bits.Set(&ctrl.state, stateCancel)
	bits.Unset(&ctrl.action, actionCancel)
}

func (ctrl *Control) isRestarting() bool { return bits.IsSet(&ctrl.action, actionRestart) }
func (ctrl *Control) isCancelling() bool { return bits.IsSet(&ctrl.action, actionCancel) }
func (ctrl *Control) isCanceled() bool   { return bits.IsSet(&ctrl.state, stateCancel) }

func (ctrl *Control) loopRunner() {
	ctrl.setRunning(true)
	for {
		ctrl.Logf("loop start")
		ctrl.kanata.YieldRight()

		ctrl.Logf("coroutine start")
		ctrl.setRunning(true)
		ctrl.startCoroutine()

		ctrl.waitForSubsToEnd()

		ctrl.Logf("coroutine end")
		ctrl.setRunning(false)
	}
}

func (ctrl *Control) startCoroutine() {
	defer catchCancellation()
	ctrl.coroutine(ctrl)
}

func (ctrl *Control) waitForSubsToEnd() {
	bits.Set(&ctrl.state, stateStopping)
	defer bits.Unset(&ctrl.state, stateStopping)

	ctrl.subControlsMu.RLock()
	subs := ctrl.subControls
	ctrl.subControlsMu.RUnlock()

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
			ctrl.kanata.YieldRight()
		}
	}

	ctrl.subControlsMu.Lock()
	ctrl.subControls = ctrl.subControls[:0]
	ctrl.subControlsMu.Unlock()

	for _, s := range subs {
		freeCoroutine(s)
	}

}

func (ctrl *Control) update() {
	restartNow := ctrl.isRestarting()
	if ctrl.isCancelling() {
		ctrl.applyCancel()
		restartNow = false
	} else if restartNow {
		bits.Unset(&ctrl.action, actionRestart)
		ctrl.applyRestart()
	}

	if ctrl.coroutine != nil && (ctrl.IsRunning() || restartNow) {
		ctrl.kanata.YieldLeft()
	}

	{
		// update and remove finished subs
		ctrl.subControlsMu.RLock()
		subs := ctrl.subControls
		ctrl.subControlsMu.RUnlock()
		if len(subs) > 0 {
			// if it's stopping already, don't bother
			// filtering out finished subs here, since they will
			// be removed soon anyway on the loopRunner thread.
			if bits.IsSet(&ctrl.state, stateStopping) {
				for _, sub := range subs {
					sub.update()
				}
			} else {
				hasRemoved := false
				for _, sub := range subs {
					sub.update()
					if sub.IsDone() {
						freeCoroutine(sub)
						hasRemoved = true
					} else {
						ctrl.tempSubControls = append(ctrl.tempSubControls, sub)
					}
				}
				if hasRemoved {
					ctrl.subControlsMu.Lock()
					ctrl.subControls = ctrl.tempSubControls
					ctrl.subControlsMu.Unlock()
				}
				ctrl.tempSubControls = ctrl.tempSubControls[:0]
			}
		}
	}
}

func (ctrl *Control) initialize(coroutine Coroutine) {
	ctrl.coroutine = coroutine
	ctrl.Logf("created")
	ctrl.Restart()

}
