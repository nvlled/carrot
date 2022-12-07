package carrot

import (
	"github.com/nvlled/quest"
)

// The error that is thrown while waiting on
// methods like Yield(), Sleep() and Delay().
// This is used to prevent a coroutine from continuing
// when cancelled.
// No need to explicitly handle and recover from
// this error inside a coroutine.
//
// Note: if insect methods are called an another thread outside a coroutine,
// then this error needs to be handled. You may use the helper function:
//
//	...
//	defer CatchCancelled()
//	insect.Yield()
var ErrCancelled = quest.ErrCancelled

type Void = quest.Void

var None = quest.None

// A PlainTask is a cancellable Awaitable.
// Similar to Task, but simpler to avoid
// unwanted state changes.
type PlainTask[T any] interface {
	quest.Awaitable[T]
	Cancel()
}

type action = func()

type voidTask = quest.Task[Void]

func CatchCancellation(optionalOnCancel ...action) {
	if err := recover(); err == ErrCancelled {
		for _, fn := range optionalOnCancel {
			fn()
		}
	} else if err != nil {
		panic(err)
	}
}
