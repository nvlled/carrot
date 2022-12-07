package carrot

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nvlled/quest"
)

// A Coroutine is function that only takes a In argument.
type Coroutine = func(Insect)

// A Coroutine that takes one additional argument.
type CoroutineA[Arg any] func(Insect, Arg)

// A Coroutine that returns one result.
type CoroutineR[Result any] func(Insect) Result

// A Coroutine that both takes one additional argument
// and returns one result.
type CoroutineAR[Arg any, Result any] func(Insect, Arg) Result

// An Insect is used to direct the program flow of a coroutine.
//
// Note: Methods may block for one or more several frames,
// except for those labeled with Async.
//
// Note: Methods are all concurrent-safe.
//
// Note: Blocking methods should be called from a coroutine, directly
// or indirectly. Blocking methods will panic() with a ErrCancelled,
// and coroutines will automatically catch this.
type Insect interface {
	// Create a new Insect, for purposes of starting
	// a new sub coroutine. Use only for a custom
	// coroutine coordination. It is recommended to use StartAsync
	// instead.
	Create() Insect

	// RunOnUpdate invokes fn in the same thread as Update().
	// Useful when calling functions that are only useable in the main thread.
	// RunOnUpdate will block until fn is run and returns.
	//
	// Note: Avoid doing long-running synchronous operations inside fn.
	//
	// Note: Avoid calling Insect methods inside fn, particularly the ones that panic.
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
	// Similar to the other function StartAsync(insect, coroutine)
	//
	// Example:
	//  insect.StartAsync(func(insect Insect) { ... })
	//
	// Note: the argument insect is a new and different insect.
	// Use the same insect name "in" in any coroutine, to
	// avoid accidentally using the parent insects.
	StartAsync(coroutine Coroutine) PlainTask[Void]
}

type Cancellable interface {
	Cancel()
}

type insectoid struct {
	// ID of coroutine insect. Mainly used for debugging.
	ID int64

	updateTask voidTask

	subInsects *sliceSet[*insectoid]

	queued   action
	canceled bool
}

var idGen = atomic.Int64{}

func newInsect() *insectoid {
	return &insectoid{
		ID:         idGen.Add(1),
		updateTask: quest.NewVoidTask(),
		subInsects: newSliceSet[*insectoid](),
	}
}

func (insect *insectoid) Create() Insect {
	return insect.create()
}

func (insect *insectoid) create() *insectoid {
	subInsect := spawnInsectoid()
	subInsect.reset()
	insect.subInsects.Add(subInsect)
	return subInsect
}

func (insect *insectoid) RunOnUpdate(fn func()) {
	if insect.canceled {
		return
	}
	insect.queued = fn
	insect.tryTerminate(insect.updateTask.AwaitAndReset())
}

func (insect *insectoid) Yield() {
	insect.tryTerminate(insect.updateTask.AwaitAndReset())
}

func (insect *insectoid) Delay(count int) {
	for i := 0; i < count; i++ {
		insect.Yield()
	}
}

func (insect *insectoid) DelayAsync(count int) PlainTask[Void] {
	return StartAsync(insect, func(in Insect) {
		for i := 0; i < count; i++ {
			insect.Yield()
		}
	})
}

func (insect *insectoid) Sleep(t time.Duration) {
	insect.SleepAsync(t).Await()
}

func (insect *insectoid) SleepAsync(t time.Duration) PlainTask[Void] {
	return StartAsync(insect, func(Insect) {
		time.Sleep(t)
	})
}

func (insect *insectoid) update() {
	if insect.canceled {
		return
	}

	if insect.queued != nil {
		insect.queued()
		insect.queued = nil
	}

	insect.updateTask.Resolve(None)

	insect.subInsects.Each(func(sub *insectoid) {
		sub.update()
	})
}

func (insect *insectoid) IsCanceled() bool {
	return insect.canceled
}

func (insect *insectoid) Cancel() {
	if insect.canceled {
		return
	}
	insect.canceled = true
	insect.queued = nil

	insect.subInsects.Each(func(sub *insectoid) {
		sub.Cancel()
	})
	insect.subInsects.Clear()

	insect.updateTask.Cancel()
}

func (insect *insectoid) reset() {
	insect.queued = nil
	insect.updateTask.Reset()
	insect.canceled = false
	insect.subInsects.Clear()
}

// See docs on the interface Insect.StartAsync
func (insect *insectoid) StartAsync(coroutine Coroutine) PlainTask[Void] {
	return StartAsync(insect, coroutine)
}

// Similar to insect.StartAsync(coroutine)
func StartAsync(
	insect Insect,
	coroutine Coroutine,
) PlainTask[Void] {
	return StartAsyncAR(insect, func(insect Insect, _ Void) Void {
		coroutine(insect)
		return None
	}, None)
}

// Similar to StartAsync, but the coroutine takes on addition argument.
// Example:
//
//	StartAsyncA(insect, func(insect Insect, arg int) { ... })
func StartAsyncA[Arg any](
	insect Insect,
	coroutine CoroutineA[Arg],
	arg Arg,
) PlainTask[Void] {
	return StartAsyncAR(insect, func(insect Insect, arg Arg) Void {
		coroutine(insect, arg)
		return None
	}, arg)
}

// Similar to StartAsync, but the coroutine returns one result.
// Example:
//
//	StartAsyncA(insect, func(insect Insect) int { ... })
func StartAsyncR[Result any](
	insect Insect,
	coroutine CoroutineR[Result],
) PlainTask[Result] {
	return StartAsyncAR(insect, func(insect Insect, _ Void) Result {
		return coroutine(insect)
	}, None)
}

// Similar to StartAsync, but the coroutine takes one
// additional argument and returns one result.
// Example:
//
//	StartAsyncA(insect, func(insect Insect, arg string) int { ... })
//	StartAsyncA(insect, func(insect Insect, arg float32) string { ... })
//	StartAsyncA(insect, func(insect Insect, arg int) Unit { ... })
//
// Note: if you are going to use Void in either Arg or Result,
// consider using the other variations of StartAsync.
func StartAsyncAR[Arg any, Result any](
	in Insect,
	coroutine CoroutineAR[Arg, Result],
	arg Arg,
) PlainTask[Result] {
	insect := in.(*insectoid)
	subInsect := insect.create()

	completion := quest.AllocTask[Void]()
	blocker := quest.AllocTask[Result]()
	blocker.SetPanic(true)

	go func() {
		// ensure coroutine is done before cleaning up
		completion.Anticipate()
		if _, ok := blocker.Anticipate(); !ok {
			subInsect.Cancel()
			quest.FreeTask(blocker)
			quest.FreeTask(completion)
			releaseInsectoid(subInsect)
		}
	}()

	go func() {
		defer completion.Resolve(None)
		defer CatchCancellation()
		result := coroutine(subInsect, arg)
		if !subInsect.canceled && !insect.IsCanceled() {
			blocker.Resolve(result)
		}
	}()

	return blocker
}

func (insect *insectoid) String() string {
	return fmt.Sprintf("co-%v", insect.ID)
}

func (insect *insectoid) tryTerminate(none Void, ok bool) (Void, bool) {
	if !ok {
		panic(ErrCancelled)
	}
	return none, ok
}
