package carrot_test

import (
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nvlled/carrot"
	"github.com/nvlled/quest"
)

func init() {
	rand.Seed(time.Now().UnixMilli())
}

func randomSleep() {
	ms := 10 + rand.Int31n(100)
	time.Sleep(time.Duration(ms * int32(time.Microsecond)))
}

func TestBlocking(t *testing.T) {
	startTime := time.Now()
	msPerLoop := 10
	numToTurn := 12

	in := carrot.Start(func(in carrot.Invoker) {
		for i := 0; i < numToTurn; i++ {
			// will wait 10ms before continuing
			in.Yield()
		}
	})

	// each loop iteration (roughly) takes 10ms
	for !in.IsDone() {
		in.Update()
		time.Sleep(time.Duration(msPerLoop) * time.Millisecond)
	}

	// 12 loop iterations, and 10ms per iteration == 120ms
	// elapsed time must at least 120ms or greater
	// if not, then fail
	elapsed := time.Since(startTime).Milliseconds()
	if int(elapsed) < msPerLoop*numToTurn {
		t.Errorf("finished too early, elapsed=%vms, expected=%vms", elapsed, msPerLoop*numToTurn)
	}
}

func TestQueue(t *testing.T) {
	// source:
	// https://gist.github.com/metafeather/3615b23097836bc36579100dac376906#file-main-go-L12
	goroutineID := func() int {
		var buf [64]byte
		n := runtime.Stack(buf[:], false)
		idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
		id, err := strconv.Atoi(idField)
		if err != nil {
			panic(fmt.Sprintf("cannot get goroutine id: %v", err))
		}
		return id
	}

	mainID := goroutineID()

	numInvoke := 0
	runner := carrot.Start(func(in carrot.Invoker) {
		outerID := goroutineID()
		if mainID == outerID {
			t.Errorf("it's broke: main=%v, outerID=%v\n", mainID, outerID)
		}

		// this should execute in the main thread,
		// on the next call Update()
		in.RunOnUpdate(func() {
			innerID := goroutineID()
			if mainID != innerID {
				t.Errorf("it's broke: main=%v, innerID=%v\n", mainID, innerID)
			}
			numInvoke++
		})
		// will wait for the queued function to finish
		// before proceeding

		for i := 0; i < 10; i++ {
			in.Yield()
		}
	})

	// main thread
	for !runner.IsDone() {
		runner.Update()
		randomSleep()
	}

	if numInvoke != 1 {
		t.Error("queued functions should be invoked once")
	}
}

func TestStartAsync1(t *testing.T) {
	counter := atomic.Int32{}
	sum := atomic.Int32{}

	adder := func(in carrot.Invoker) {
		for i := 0; i < 30; i++ {
			in.Yield()
			sum.Add(int32(i))
		}
	}

	loopCounter := func(in carrot.Invoker, n int) {
		for i := 0; i < n; i++ {
			in.Yield()
			counter.Add(1)
		}
	}

	runner := carrot.Start(func(in carrot.Invoker) {
		// asynchronous, will proceed immediately
		// without waiting for them to finish
		task1 := carrot.StartAsyncA(in, loopCounter, 10)

		task1.Await()

		if counter.Load() != 10 {
			t.Fatal("wrong counter value", counter.Load())
			return
		}

		task2 := carrot.StartAsyncA(in, loopCounter, 50)
		task3 := in.StartAsync(adder)

		// wait for the other loopCounter and adder to finish
		quest.AwaitAll[carrot.Void](task2, task3)

		if counter.Load() != 60 {
			t.Fatal("wrong counter value", counter.Load())
			return
		}

		// run another loopCounter, this time
		// synchronously
		loopCounter(in, 50)

		// Cancel all invokers, no effect if they are already done.
		// Note: cancelling the main invoker will
		// cancel all invokers created in this coroutine.
		in.Cancel()
	})

	for !runner.IsDone() {
		randomSleep()
		runner.Update()
	}

	if sum.Load() != 435 {
		t.Errorf("adder wasn't able to from 0 to 100, sum=%v", sum)
	}
	if counter.Load() != 110 {
		t.Errorf("loopCounter failed: counter=%v\n", counter.Load())
	}
}

func TestStartAsync2(t *testing.T) {
	coroutine := func(in carrot.Invoker) {
		for i := 0; i < 10; i++ {
			in.Yield()
		}
	}
	coroutineWithArg := func(in carrot.Invoker, n int) {
		for i := 0; i < n; i++ {
			in.Yield()
		}
	}
	coroutineWithResult := func(in carrot.Invoker) int {
		sum := 0
		for i := 0; i < 20; i++ {
			sum += i
			in.Yield()
		}
		return sum
	}

	coroutineWithArgsResult := func(in carrot.Invoker, n int) int {
		sum := 0
		for i := 0; i < n; i++ {
			sum += i
			in.Yield()
		}
		return sum
	}

	runner := carrot.Start(func(in carrot.Invoker) {
		// run coroutines synchronously, one by one
		coroutine(in)
		coroutineWithArg(in, 20)
		sum1 := coroutineWithResult(in)
		sum2 := coroutineWithArgsResult(in, 20)

		// run coroutines asynchronously, all at the same time
		task1 := carrot.StartAsync(in, coroutine)
		task2 := carrot.StartAsyncAR(in, coroutineWithArgsResult, 20)
		task3 := carrot.StartAsyncA(in, coroutineWithArg, 20)
		task4 := carrot.StartAsyncR(in, coroutineWithResult)

		// wait for all coroutines to finish
		// only task2 and task4 returns a result
		_, asyncSum1, _, asyncSum2 := quest.Await4[carrot.Void, int, carrot.Void, int](task1, task2, task3, task4)

		if sum1 != *asyncSum1 {
			t.FailNow()
		}
		if sum2 != *asyncSum2 {
			t.FailNow()
		}
	})

	for !runner.IsDone() {
		randomSleep()
		runner.Update()
	}
}

func TestStartAsyncWithResult(t *testing.T) {
	doSomething := func(in carrot.Invoker, n int) carrot.Void {
		for i := 0; i < n; i++ {
			in.Yield()
		}
		return carrot.None
	}
	computeResult := func(in carrot.Invoker, n int) int {
		sum := 0
		for i := 0; i < n; i++ {
			sum += i
			in.Yield()
		}
		return sum
	}

	runner := carrot.Start(func(in carrot.Invoker) {
		task1 := carrot.StartAsyncAR(in, computeResult, 10)
		task2 := carrot.StartAsyncAR(in, computeResult, 20)
		task3 := carrot.StartAsyncAR(in, doSomething, 30)

		x, y, _ := quest.Await3[int, int, carrot.Void](task1, task2, task3)
		if x == nil || y == nil {
			t.FailNow()
		}

		if *x+*y != 235 {
			t.FailNow()
		}
	})

	for !runner.IsDone() {
		randomSleep()
		runner.Update()
	}
}

func TestDelays(t *testing.T) {
	startTime := time.Now()
	x := 0
	expectedMs := 0
	notCancelled := false
	runner := carrot.Start(func(in carrot.Invoker) {
		for i := 0; i < 100; i++ {
			in.StartAsync(func(in carrot.Invoker) {
				in.Delay((i + 1) * 10000)
			})
		}

		expectedMs += 1

		in.SleepAsync(2 * time.Millisecond).Await()
		expectedMs += 2

		in.SleepAsync(10000000 * time.Millisecond).Cancel()

		in.SleepAsync(2 * time.Millisecond).Await()
		expectedMs += 2

		in.Delay(2)
		expectedMs += 2

		in.DelayAsync(2).Await()
		expectedMs += 2

		in.DelayAsync(1000000).Cancel()

		task := carrot.StartAsync(in, func(in carrot.Invoker) {
			in.DelayAsync(1000000).Await()
			x++
		})

		notCancelled = true
		in.Delay(100)
		expectedMs += 100

		task.Cancel()
		in.Delay(50)
	})

	for !runner.IsDone() {
		time.Sleep(1 * time.Millisecond)
		runner.Update()
	}

	actualMs := time.Since(startTime).Milliseconds()
	println(expectedMs, actualMs)

	if !notCancelled {
		t.Error("... coroutine should not be cancelled")
	}
	if expectedMs-1 > int(actualMs) {
		t.Errorf("delays were too short, expected=%v, actual%v", expectedMs, actualMs)
	}

	if x != 0 {
		t.Errorf("cancelled coroutines should not continue")
	}
}

func TestCancelSubCoroutines(t *testing.T) {
	wg := sync.WaitGroup{}
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner := carrot.Start(func(in carrot.Invoker) {
				in.StartAsync(func(in carrot.Invoker) {
					in.StartAsync(func(in carrot.Invoker) {
						in.Sleep(5000 * time.Millisecond)
						t.Error("should not be called")
					}).Await()
					t.Error("should not be called")
				})

				in.Delay(1)
				in.Cancel()
			})
			for !runner.IsDone() {
				time.Sleep(1 * time.Millisecond)
				runner.Update()
			}
			time.Sleep(50 * time.Millisecond)
		}()
	}
	wg.Wait()
}

func TestEndSubroutine(t *testing.T) {
	runner := carrot.Start(func(in carrot.Invoker) {
		task1 := in.StartAsync(func(in carrot.Invoker) {
			for i := 0; i < 10; i++ {
				in.Yield()
			}
		})
		task2 := in.StartAsync(func(in carrot.Invoker) {
			for i := 0; i < 10; i++ {
				in.Yield()
			}
		})
		task2.Await()
		task1.Await()
	})
	for !runner.IsDone() {
		time.Sleep(1 * time.Millisecond)
		runner.Update()
	}
}
