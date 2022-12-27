# carrot

[![Go Reference](https://pkg.go.dev/badge/github.com/nvlled/carrot.svg)](https://pkg.go.dev/github.com/nvlled/carrot)

A go "coroutine" library, designed to run inside game loops.
It is an async library helpful for creating asynchronous state machines.
Took some inspiration from https://github.com/wraikny/AwaitableCoroutine.

## Features

- subjectively simple API
- arbitrarily compose any coroutines
- cancelablle and awaitable sub-coroutines

## Definitions

`Coroutine` - a function that takes an `Invoker`

`Invoker` - an object used to control the coroutine, with operations
such yielding or cancelling.

`Script` - an instance of several coroutines running

`Frame` - equivalent to one call to Update(). Alternatively, it
is one iteration in the game loop.

## Installation

```
go get github.com/nvlled/carrot
```

## Documentation

API reference can be found [here](https://pkg.go.dev/github.com/nvlled/carrot).
A working example program can be found [here:TODO](#TODO).

## Example code

### Based on AwaitableCoroutine

```go
count := 0
script := carrot.Start(func(in carrot.Invoker) {
    println("Started!")
    for i := 0; i < 10; i++ {
        in.Yield()
        count++
    }
})

for !script.IsDone() {
    script.Update()
    println(count)
}
```

### Other example

```go
func subCoroutine0(in carrot.Invoker) { /* ... */ }
func subCoroutine1(in carrot.Invoker) int { /* ... */ }
func subCoroutine2(in carrot.Invoker) string { /* ... */ }
func subCoroutine3(in carrot.Invoker x int) string {
    subCoroutine2(in)
    // ...
}

func mainCoroutine(in carrot.Invoker) {
    n := subCoroutine1(in)
    str := subCoroutine2(in)
    _, anInt, aStr, aStr2 := carrot.Await4(
        in.StartAsync(subCoroutine0),
        carrot.StartAsyncR(in, subCoroutine1),
        carrot.StartAsyncR(in, subCoroutine2),
        carrot.StartAsyncAR(in, subCoroutine3, n),
    )
}

script := carrot.Start(mainCoroutine)

for !script.IsDone() {
    script.Update()
}

```

For more examples, see the coroutine test file.
