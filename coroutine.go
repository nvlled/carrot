package carrot

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nvlled/quest"
)

// A Coroutine is function that only takes a In argument.
type Coroutine = func(Invoker)

// A Coroutine that takes one additional argument.
type CoroutineA[Arg any] func(Invoker, Arg)

// A Coroutine that returns one result.
type CoroutineR[Result any] func(Invoker) Result

// A Coroutine that both takes one additional argument
// and returns one result.
type CoroutineAR[Arg any, Result any] func(Invoker, Arg) Result

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
	// Create a new invoker, for purposes of starting
	// a new sub coroutine. Use only for a custom
	// coroutine coordination. It is recommended to use StartAsync
	// instead.
	Create() Invoker

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

	// Delay waits for a number of calls to Update().
	// Panics when cancelled while Await()'ing the task.
	// Uses StartAsync.
	DelayAsync(count int) PlainTask[Void]

	// Sleep blocks and waits for the given duration.
	Sleep(duration time.Duration)

	// Sleep blocks and waits for the given duration.
	// Panics when cancelled while Await()'ing the task.
	// Uses StartAsync.
	SleepAsync(duration time.Duration) PlainTask[Void]

	// Cancels the coroutine. Cancels all child coroutines created with
	// StartAsync. This does not affect parent coroutines.
	Cancel()

	// Returns true if current coroutine has been canceled.
	IsCanceled() bool

	// Starts a sub-coroutine asychronously.
	// Use this method when you need to run a coroutine
	// in the background, or you need to run several coroutines
	// at the same time.
	// Otherwise, you can just call another coroutine normally as a function.
	// Similar to the other function StartAsync(in, coroutine)
	//
	// Example:
	//  in.StartAsync(func(in Invoker) { ... })
	//
	// Note: the argument in is a new and different in.
	// Use the same in name "in" in any coroutine, to
	// avoid accidentally using the parent ins.
	StartAsync(coroutine Coroutine) PlainTask[Void]
}

type invoker struct {
	// ID of invoker. Mainly used for debugging.
	ID int64

	updateTask voidTask
	yieldTask  voidTask

	subInvokers *sliceSet[*invoker]

	queued   action
	canceled bool

	hasYield bool
}

var idGen = atomic.Int64{}

func NewInvoker() Invoker {
	return newInvoker()
}
func newInvoker() *invoker {
	return &invoker{
		ID:          idGen.Add(1),
		updateTask:  quest.NewVoidTask(),
		yieldTask:   quest.NewVoidTask(),
		subInvokers: newSliceSet[*invoker](),
	}
}

func (in *invoker) Create() Invoker {
	return in.create()
}

func (in *invoker) create() *invoker {
	subInvoker := summonInvoker()
	subInvoker.reset()
	in.subInvokers.Add(subInvoker)
	return subInvoker
}

func (in *invoker) release(subInvoker *invoker) {
	disperseInvoker(subInvoker)
	in.subInvokers.Remove(subInvoker)
}

func (in *invoker) RunOnUpdate(fn func()) {
	if in.canceled {
		return
	}
	in.queued = fn
	in.Yield()
}

func (in *invoker) Yield() {
	in.hasYield = true
	in.yieldTask.Resolve(None)
	in.tryTerminate(in.updateTask.AwaitAndReset())
	in.hasYield = false
}

func (in *invoker) Delay(count int) {
	for i := 0; i < count; i++ {
		in.Yield()
	}
}

func (in *invoker) DelayAsync(count int) PlainTask[Void] {
	return StartAsync(in, func(in Invoker) {
		for i := 0; i < count; i++ {
			in.Yield()
		}
	})
}

func (in *invoker) Sleep(t time.Duration) {
	in.SleepAsync(t).Await()
}

func (in *invoker) SleepAsync(t time.Duration) PlainTask[Void] {
	return StartAsync(in, func(Invoker) {
		time.Sleep(t)
	})
}

func (in *invoker) update() {
	if in.canceled {
		return
	}

	if in.queued != nil {
		in.queued()
		in.queued = nil
	}

	in.updateTask.Resolve(None)
	if in.hasYield {
		in.yieldTask.AwaitAndReset()
	}

	in.subInvokers.Each(func(sub *invoker) {
		sub.update()
	})

}

func (in *invoker) IsCanceled() bool {
	return in.canceled
}

func (in *invoker) Cancel() {
	if in.canceled {
		return
	}
	in.canceled = true
	in.queued = nil
	in.hasYield = false

	in.subInvokers.Each(func(sub *invoker) {
		sub.Cancel()
	})
	in.subInvokers.Clear()

	in.updateTask.Cancel()
}

func (in *invoker) reset() {
	in.queued = nil
	in.canceled = false
	in.hasYield = false
	in.subInvokers.Clear()
	in.updateTask.Reset()
	in.yieldTask.Reset()
	in.updateTask.SetPanic(true)
}

// See docs on the interface Invoker.StartAsync
func (in *invoker) StartAsync(coroutine Coroutine) PlainTask[Void] {
	return StartAsync(in, coroutine)
}

// Similar to in.StartAsync(coroutine)
func StartAsync(
	in Invoker,
	coroutine Coroutine,
) PlainTask[Void] {
	return StartAsyncAR(in, func(in Invoker, _ Void) Void {
		coroutine(in)
		return None
	}, None)
}

// Similar to StartAsync, but the coroutine takes on addition argument.
// Example:
//
//	StartAsyncA(in, func(in Invoker, arg int) { ... })
func StartAsyncA[Arg any](
	in Invoker,
	coroutine CoroutineA[Arg],
	arg Arg,
) PlainTask[Void] {
	return StartAsyncAR(in, func(in Invoker, arg Arg) Void {
		coroutine(in, arg)
		return None
	}, arg)
}

// Similar to StartAsync, but the coroutine returns one result.
// Example:
//
//	StartAsyncA(in, func(in Invoker) int { ... })
func StartAsyncR[Result any](
	in Invoker,
	coroutine CoroutineR[Result],
) PlainTask[Result] {
	return StartAsyncAR(in, func(in Invoker, _ Void) Result {
		return coroutine(in)
	}, None)
}

// Similar to StartAsync, but the coroutine takes one
// additional argument and returns one result.
// Example:
//
//	StartAsyncA(in, func(in Invoker, arg string) int { ... })
//	StartAsyncA(in, func(in Invoker, arg float32) string { ... })
//	StartAsyncA(in, func(in Invoker, arg int) Unit { ... })
//
// Note: if you are going to use Void in either Arg or Result,
// consider using the other variations of StartAsync.
func StartAsyncAR[Arg any, Result any](
	in Invoker,
	coroutine CoroutineAR[Arg, Result],
	arg Arg,
) PlainTask[Result] {
	self := in.(*invoker)
	subInvoker := self.create()

	completion := quest.AllocTask[Void]()
	blocker := quest.AllocTask[Result]()
	blocker.SetPanic(true)

	go func() {
		// ensure coroutine is done before cleaning up
		completion.Anticipate()
		if _, ok := blocker.Anticipate(); !ok {
			subInvoker.Cancel()
			quest.FreeTask(blocker)
			quest.FreeTask(completion)
			disperseInvoker(subInvoker)
		}
	}()

	go func() {
		defer completion.Resolve(None)
		defer CatchCancellation()
		result := coroutine(subInvoker, arg)
		if !subInvoker.canceled && !in.IsCanceled() {
			blocker.Resolve(result)
		}
	}()

	return blocker
}

func (in *invoker) String() string {
	return fmt.Sprintf("co-%v", in.ID)
}

func (in *invoker) tryTerminate(none Void, ok bool) (Void, bool) {
	if !ok {
		in.hasYield = false
		panic(ErrCancelled)
	}
	return none, ok
}
