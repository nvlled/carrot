package carrot

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nvlled/quest"
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

	// RunOnUpdate invokes fn in the same thread as Update().
	// Useful when calling functions that are only useable in the main thread.
	// RunOnUpdate will block until fn is run and returns.
	//
	// Note: Avoid doing long-running synchronous operations inside fn.
	//
	// Note: Avoid calling Invoker methods inside fn, particularly the ones that panic.
	RunOnUpdate(fn func())

	// Yield waits until the next call to Update().
	// In other words, Yield() waits for one frame.
	// Panics when cancelled.
	Yield()

	// Delay waits for a number of calls to Update().
	// Panics when cancelled.
	Delay(count int)

	// Sleep blocks and waits for the given duration.
	Sleep(duration time.Duration)

	// Cancels the coroutine. Cancels all child coroutines created with
	// StartAsync. This does not affect parent coroutines.
	Cancel()

	// Returns true if current coroutine has been canceled.
	IsCanceled() bool

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
}

type invoker struct {
	// ID of invoker. Mainly used for debugging.
	ID int64

	updateTask voidTask
	yieldTask  voidTask

	queued   action
	canceled atomic.Bool
	hasYield atomic.Bool

	script *Script

	mu sync.Mutex
}

var idGen = atomic.Int64{}

func NewInvoker() Invoker {
	return newInvoker()
}
func newInvoker() *invoker {
	return &invoker{
		ID:         idGen.Add(1),
		updateTask: quest.NewVoidTask(),
		yieldTask:  quest.NewVoidTask(),
	}
}

func (in *invoker) RunOnUpdate(fn func()) {
	if in.canceled.Load() {
		return
	}
	in.queued = fn
	in.Yield()
}

func (in *invoker) Yield() {
	in.hasYield.Store(true)
	defer in.hasYield.Store(false)
	in.yieldTask.Resolve(None)
	ok := in.updateTask.Yield()
	in.tryTerminate(None, ok)
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

func (in *invoker) update() {
	if in.canceled.Load() {
		return
	}

	if in.queued != nil {
		in.queued()
		in.queued = nil
	}

	in.updateTask.Resolve(None)
	if in.hasYield.Load() {
		in.yieldTask.Yield()
	}
}

func (in *invoker) IsCanceled() bool {
	return in.canceled.Load()
}

func (in *invoker) Cancel() {
	in.script.Cancel()
}

func (in *invoker) applyCancel() {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.canceled.Load() {
		return
	}
	in.canceled.Store(true)
	in.queued = nil

	in.updateTask.Cancel()
	in.yieldTask.Cancel()
}

func (in *invoker) reset() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.queued = nil
	in.canceled.Store(false)
	in.updateTask.Reset()
	in.yieldTask.Reset()
	in.updateTask.SetPanic(true)
}

func (in *invoker) Transition(coroutine Coroutine) {
	if in.script != nil {
		in.script.Transition(coroutine)
	}
}

func (in *invoker) Abyss() {
	for {
		in.Yield()
	}
}

func (in *invoker) String() string {
	return fmt.Sprintf("co-%v", in.ID)
}

func (in *invoker) tryTerminate(none Void, ok bool) (Void, bool) {
	if !ok {
		panic(ErrCancelled)
	}
	return none, ok

}
