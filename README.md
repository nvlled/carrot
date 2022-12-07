# carrot

A go "coroutine" library, designed to run inside game loturn.
Took some inspiration from https://github.com/wraikny/AwaitableCoroutine.

## Features

- arbitrarily compose any coroutines
- cancelable and awaitable sub-coroutines

## Definitions

`Coroutine` - a function that takes an `Insect`

`Insect` - an object used to control the coroutine, with operations
such yielding or cancelling. It is left to imagination what insect means here,
it could stand for Interfaced Expressed Commands Terminal, or
it means more bugs for your code.

`Script` - an instance of several coroutines running

`Frame` - equivalent to one call to Update(). Alternatively, it
is one iteration in the game loop.

## Installation

```
go get github.com/nvlled/carrot
```

## Example code

### Based on AwaitableCoroutine

```go
count := 0
script := carrot.Start(func(in carrot.Insect) {
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
func subCoroutine0(in carrot.Insect) { /* ... */ }
func subCoroutine1(in carrot.Insect) int { /* ... */ }
func subCoroutine2(in carrot.Insect) string { /* ... */ }
func subCoroutine3(in carrot.Insect x int) string {
    subCoroutine2(in)
    // ...
}

func mainCoroutine(in carrot.Insect) {
    n := subCoroutine1(in)
    str := subCoroutine2(in)
    _, anInt, aStr, aStr2 := carrot.Await4(
        in.StartAsync(subCoroutine0),
        carrot.StartAsyncR(in, subCoroutine1),
        carrot.StartAsyncR(in, subCoroutine2),
        carrot.StartAsyncAR(in, subCoroutine3, n),
    )
}

runner.Start(mainCoroutine)

for !runner.IsDone() {
    runner.Update()
}

```
