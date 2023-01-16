package carrot_test

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nvlled/carrot"
)

var updateDelay = 100 * time.Microsecond

func init() {
	rand.Seed(time.Now().UnixMilli())
}

func randomSleep() {
	ms := 100 + rand.Int31n(500)
	time.Sleep(time.Duration(ms * int32(time.Microsecond)))
}

func TestBlocking(t *testing.T) {
	startTime := time.Now()
	msPerLoop := 10
	numToTurn := 12

	script := carrot.Start(func(in *carrot.Invoker) {
		for i := 0; i < numToTurn; i++ {
			// will wait 10ms before continuing
			in.Yield()
		}
	})

	// each loop iteration (roughly) takes 10ms
	for !script.IsDone() {
		script.Update()
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

func TestLoop(t *testing.T) {
	count := 0
	script := carrot.Start(func(in *carrot.Invoker) {
		for i := 0; i < 100; i++ {
			if i >= 50 {
				count = i
				in.Cancel()
			}
			in.Yield()
		}
	})

	for !script.IsDone() {
		script.Update()
	}
	if count != 50 {
		t.Error("wrong count", count)
	}
}

func TestTransition1(t *testing.T) {
	var script *carrot.Script
	count := atomic.Int32{}
	done := atomic.Bool{}
	go func() {
		script = carrot.Create()

		// TODO: I should probably add locks
		// on Cancel and Restart just in case,
		// seems to fail when frame time < 1ms
		time.Sleep(10 * time.Millisecond)

		script.Transition(func(in *carrot.Invoker) {
			for {
				count.Add(1)
				in.Yield()
				in.Logf("co a %v", count.Load())
			}
		})

		for count.Load() < 10 {
			time.Sleep(1 * time.Millisecond)
		}

		script.Transition(func(in *carrot.Invoker) {
			for count.Load() < 30 {
				count.Add(1)
				in.Yield()
				in.Logf("co b %v", count.Load())
			}
			in.Cancel()
			done.Store(true)
		})
	}()

	for {
		if script != nil {
			script.Update()
			if done.Load() {
				break
			}
		}
		time.Sleep(1 * time.Millisecond)
	}

	if count.Load() < 30 {
		t.Error("failed to count up to 30:", count)
	}

}
func TestTransition2(t *testing.T) {
	coroutine := func(in *carrot.Invoker) {
		for {
			in.Yield()
		}
	}
	script := carrot.Start(coroutine)
	go func() {
		for {
			script.Restart()
			time.Sleep(time.Duration(rand.Intn(10)) * time.Microsecond)
		}
	}()
	go func() {
		for {
			script.Cancel()
			time.Sleep(time.Duration(rand.Intn(10)) * time.Microsecond)
		}
	}()
	go func() {
		for {
			script.Transition(coroutine)
			time.Sleep(time.Duration(rand.Intn(10)) * time.Microsecond)
		}
	}()

	for i := 0; i < 100000; i++ {
		script.Update()
	}
}

func TestTransition3(t *testing.T) {
	coroutine1 := func(in *carrot.Invoker) {
		for {
			in.Yield()
			for i := 0; i < 1000; i++ {
			}
			in.Yield()
		}
	}
	coroutine2 := func(in *carrot.Invoker) {
		for {
			in.Yield()
			for i := 0; i < 1000; i++ {
				in.Yield()
			}
			in.Yield()
		}
	}
	coroutine3 := func(in *carrot.Invoker) {
	}
	script1 := carrot.Start(coroutine1)
	script2 := carrot.Start(func(in *carrot.Invoker) {
		i := 0
		for n := 0; n < 1000; n++ {
			i++
			if i%2 == 0 {
				script1.Transition(coroutine1)
				in.Yield()
				script1.Transition(coroutine2)
			} else {
				script1.Transition(coroutine2)
				in.Yield()
				script1.Transition(coroutine1)
				in.Yield()
				script1.Transition(coroutine3)
			}
			in.Yield()
			randomSleep()
			in.Yield()
			if rand.Float32() < 0.8 {
				script1.Cancel()
			}
			in.Yield()
			randomSleep()
			in.Yield()

			if rand.Float32() < 0.9 {
				in.Yield()
			}
			randomSleep()
			script1.Transition(coroutine1)
			randomSleep()
		}
	})

	go func() {
		for {
			script1.Cancel()
			time.Sleep(time.Duration(rand.Intn(10)) * time.Microsecond)
		}
	}()
	go func() {
		for {
			script1.Transition(coroutine1)
			time.Sleep(time.Duration(rand.Intn(10)) * time.Microsecond)
		}
	}()

	for i := 0; i < 1000; i++ {
		script1.Update()
		script2.Update()
	}
}

func TestCoroutineWithoutYield(t *testing.T) {
	count := atomic.Int32{}
	script := carrot.Start(func(in *carrot.Invoker) {
		in.Logf("in coroutine")
		count.Add(1)
	})

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	if !script.IsDone() {
		t.Error("script should be done", script.IsDone())
	}
	if count.Load() != 1 {
		t.Error("coroutine should have run once", count.Load())
	}
}

func TestCoroutineWithYield(t *testing.T) {
	count := atomic.Int32{}
	script := carrot.Start(func(in *carrot.Invoker) {
		in.Logf("before yield")
		count.Add(1)
		for i := 0; i < 10; i++ {
			in.Yield()
		}
		in.Logf("after yield")
	})

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	if !script.IsDone() {
		t.Error("script should be done", script.IsDone())
	}
	if count.Load() != 1 {
		t.Error("wrong count", count.Load())
	}
}

func TestCoroutineCancel(t *testing.T) {
	count := atomic.Int32{}
	script := carrot.Start(func(in *carrot.Invoker) {
		for i := 0; i < 10; i++ {
			in.Yield()
			if i == 4 {
				in.Cancel()
			}
			count.Add(1)
			in.Logf("count=%v", i)
		}
	})

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	if !script.IsDone() {
		t.Error("script should be done:", script.IsDone())
	}

	if count.Load() != 5 {
		t.Error("wrong count", count.Load())
	}
}

func TestCoroutineCancel2(t *testing.T) {
	count := atomic.Int32{}
	_ = count

	script0 := carrot.Start(func(in *carrot.Invoker) {
		for {
			in.Yield()
		}
	})

	script := carrot.Start(func(in *carrot.Invoker) {
		script0.Cancel()
		in.Yield()
		in.Logf("script0 %v", script0.IsDone())
		in.UntilFunc(script0.IsDone)
	})

	for !script.IsDone() {
		script0.Update()
		script.Update()
		script0.Cancel()
		time.Sleep(updateDelay)
	}

}

func TestCoroutineRestart(t *testing.T) {
	count := atomic.Int32{}
	script := carrot.Start(func(in *carrot.Invoker) {
		count.Add(1)
		for i := 0; i < 100; i++ {
			if i == 10 {
				in.Cancel()
			}
		}
	})

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	script.Restart()

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	if !script.IsDone() {
		t.Error("script should be done:", script.IsDone())
	}
	if count.Load() != 2 {
		t.Error("wrong count", count.Load())
	}
}

func TestCoroutineTransition(t *testing.T) {
	count := atomic.Int32{}
	script := carrot.Start(func(in *carrot.Invoker) {
		count.Add(1)
		for i := 0; i < 100; i++ {
			if i == 10 {
				in.Cancel()
			}
		}
	})

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	script.Restart()

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	if !script.IsDone() {
		t.Error("script should be done:", script.IsDone())
	}
	if count.Load() != 2 {
		t.Error("wrong count", count.Load())
	}
}
func TestAsyncSimple(t *testing.T) {
	counter := atomic.Int32{}
	done := atomic.Bool{}

	script := carrot.Start(func(in *carrot.Invoker) {
		subIn1 := in.StartAsync(func(in *carrot.Invoker) {
			for i := 0; i < 10; i++ {
				counter.Add(1)
				in.Yield()
			}
		})

		subIn2 := in.StartAsync(func(in *carrot.Invoker) {
			for {
				if done.Load() {
					t.Error("sub-coroutine should stop running when parent coroutine stops.")
				}

				counter.Add(100)
				in.Yield()
			}
		})

		// Note: avoid using subIn* outside the scope of this function,
		// as they be disposed and freed for subsequent use.

		// wait for subIn1 to finish
		in.UntilFunc(subIn1.IsDone)

		// don't wait for subIn2 to finish, it will be automatically cancelled
		_ = subIn2
	})

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	done.Store(true)

	result := counter.Load()
	if result != 1110 {
		t.Errorf("hmm, actual counter value is %v", result)
	}

}

func TestAsyncNested(t *testing.T) {
	resultList := []string{}
	var mu sync.Mutex
	add := func(s string) {
		mu.Lock()
		resultList = append(resultList, s)
		mu.Unlock()
	}

	script := carrot.Start(func(in *carrot.Invoker) {
		subA := in.StartAsync(func(in *carrot.Invoker) {
			subB := in.StartAsync(func(in *carrot.Invoker) {
				for i := 0; i < 3; i++ {
					add(fmt.Sprintf("%v%v", "b", i))
					in.Yield()
				}
			})
			subC := in.StartAsync(func(in *carrot.Invoker) {
				for i := 0; i < 5; i++ {
					add(fmt.Sprintf("%v%v", "c", i))
					in.Yield()
				}
			})
			_ = subC
			for i := 0; i < 10; i++ {
				if i == 4 {
					subB.Cancel()
				}
				add(fmt.Sprintf("%v%v", "a", i))
				in.Yield()
			}

			in.UntilFunc(subB.IsDone)
		})

		subD := in.StartAsync(func(in *carrot.Invoker) {
			for i := 0; i < 100; i++ {
				add(fmt.Sprintf("%v%v", "d", i))
				in.Yield()
			}
		})
		_ = subD

		for i := 0; i < 10; i++ {
			add(fmt.Sprintf("%v%v", "x", i))
			in.Yield()
		}

		in.UntilFunc(subA.IsDone)

	})

	for !script.IsDone() {
		script.Update()
		time.Sleep(updateDelay)
	}

	sort.Strings(resultList)
	result := strings.Join(resultList, " ")

	// Note: this particular output isn't important here, as long as it consistently
	// produces the same output every time the same test is ran. If this test fails, try commenting
	// out the t.Error() below and observe the results.
	expected := "a0 a1 a2 a3 a4 a5 a6 a7 a8 a9 b0 b1 b2 c0 c1 c2 c3 c4 d0 d1 d10 d2 d3 d4 d5 d6 d7 d8 d9 x0 x1 x2 x3 x4 x5 x6 x7 x8 x9"

	if result != expected {
		println(result)
		println(expected)
		t.Error("coroutine execution must be deterministic")
	}
}

func BenchmarkYield(b *testing.B) {
	script := carrot.Start(func(in *carrot.Invoker) {
		for {
			in.Yield()
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		script.Update()
	}
}
