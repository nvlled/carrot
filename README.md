# carrot

[![Go Reference](https://pkg.go.dev/badge/github.com/nvlled/carrot.svg)](https://pkg.go.dev/github.com/nvlled/carrot)

A go "coroutine" library, designed to run inside game loops.
It is library to be primarily used for creating asynchronous state machines.

## Features

- subjectively simple API
- arbitrarily compose any coroutines
- cancelablle and awaitable sub-coroutines

## Definitions

`Coroutine` - a function that takes a `Control` argument.

`Control` - an object used to control the coroutine, with operations
such yielding or cancelling.

`Script` - a root of several related coroutines running.

`Frame` - equivalent to one call to Update(). Alternatively, it
is one iteration in the game loop.

## Installation

```
go get github.com/nvlled/carrot
```

## Documentation

API reference can be found [here](https://pkg.go.dev/github.com/nvlled/carrot).
A working example program can be found [here:TODO](#TODO).

## Quick example code

```go
count := 0
script := carrot.Start(func(ctrl carrot.Control) {
    println("Started!")
    for i := 0; i < 10; i++ {
        ctrl.Yield()
        count++
    }
})

for !script.IsDone() {
    script.Update()
    println(count)
}
```

```go
script := carrot.Start(func(ctrl carrot.Control) {
  // synchronously call a coroutine
  subCoroutine(ctrl)

  // will not proceed here until subCoroutine is done

  // asynchronously start coroutines
  otherCtrl := ctrl.StartAsync(otherCoroutine)
  moreCtrl := ctrl.StartAsync(moreCoroutine)
  someCtrl := ctrl.StartAsync(someCoroutine)

  // proceed here immediately without waiting,
  // all sub-coroutines will run in the background

  done := false
  for !done {
    done = doSomething()
    ctrl.Yield()
  }

  // cancel moreCoroutine
  moreCtrl.Cancel()

  // waits until otherCoroutine is done
  ctrl.YieldUntil(otherCtrl.IsDone)

  // someCoroutine will be cancelled automatically
  // when this main coroutine ends.
})

// ... update script somewhere else

```

For more examples, see the coroutine test file.
For actual usage, see the [example platformer game](#), or the
[screenshot tool](#). **TODO: Fix link**

## Troubleshooting

- My program froze or hung up
  Check if you have any loops that doesn't have a yield in it.
  Consider the following:

  ```
  for {
    input := getInput()
    state := doSomething(input)
    updateState(state)
    ctrl.Yield() // forgetting this will freeze the program
  }
  ```

## Development

If you plan on making changes for yourself, regularly run
the tests at least 10 times after some changes, especially
anything inside loopRunner or Update methods. It's all
too easy to cause a deadlock, or a bug that occurs rarely
when you least expect it.

## Prior art

Took some inspiration from https://github.com/wraikny/AwaitableCoroutine.
