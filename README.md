# carrot

[![Go Reference](https://pkg.go.dev/badge/github.com/nvlled/carrot.svg)](https://pkg.go.dev/github.com/nvlled/carrot)

A go coroutine library, designed to run inside game loops.
It is library to be primarily used for creating asynchronous state machines.

## Features

- subjectively simple API
- arbitrarily compose any coroutines
- cancelablle and awaitable sub-coroutines
- concurrent-safe without any further explicit locking,
  no coroutines will be running at the same time

## Definitions

`Coroutine` - a function that takes a `Control` argument

`Control` - an object used to control the coroutine, with operations
such yielding or cancelling

`Script` - a group of running related coroutines

`Frame` - equivalent to one call to Update(). Alternatively, it
is one iteration in the game loop

## Installation

```
go get github.com/nvlled/carrot
```

## Documentation

API reference can be found [here](https://pkg.go.dev/github.com/nvlled/carrot).

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
For actual usage, see the [example platformer game](https://github.com/nvlled/dinojump), or the
[screenshot tool](https://github.com/nvlled/screencage).

## Troubleshooting

- **My program froze or hung up**

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

  If not, then it might be a bug.

## Development

If you plan on making changes for yourself, regularly run
the tests at least 10 times after some changes, especially
anything inside loopRunner or Update methods. It's all
too easy to cause a deadlock, or a bug that occurs rarely
when you least expect it.

## Prior art

- Took some inspiration from a C# library I have used before: [AwaitableCoroutine](https://github.com/wraikny/AwaitableCoroutine). Notable difference is that carrot doesn't use shared global state, and async sub-coroutines can be cancelled without affecting parent coroutines. Also, I think AwaitableCoroutine has a bit confusing API, something I kept in mind while designing carrot.

- I haven't looked much into it, but it seems [coro](https://github.com/tcard/coro) also uses channels for coroutines.
  The implementation and API design is different though, and I didn't refer to it while writing carrot, but it roughly had
  the same idea before I did, so it's worth a mention at least.
