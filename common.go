package carrot

import (
	"errors"
)

// The error that is thrown while waiting on
// methods like Yield(), Sleep() and Delay().
// This is used to prevent a coroutine from continuing
// when cancelled.
// No need to explicitly handle and recover from
// this error inside a coroutine.
var ErrCancelled = errors.New("coroutine has been cancelled")

// A type representing none.
// Used on tasks that doesn't return
// value: Task[void]
type void struct{}

// That value that represents nothing.
// Similar to nil, but safer.
var none = void{}

func catchCancellation() {
	if err := recover(); err != nil && err != ErrCancelled {
		panic(err)
	}
}
