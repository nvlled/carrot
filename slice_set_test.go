package carrot

import (
	"sync"
	"testing"
)

func TestSliceConcurrency(t *testing.T) {
	nums := newSliceSet[int]()
	var wg sync.WaitGroup
	n := 1000
	wg.Add(1)
	go func() {
		for i := 0; i < n; i++ {
			nums.Add(i)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < n; i++ {
			if i%5 == 0 {
				nums.Remove(i)

			}
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < n; i++ {
			if i%100 == 0 {
				nums.Clear()
			}
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < n; i++ {
			nums.Each(func(x int) {
			})
		}
		wg.Done()
	}()

	wg.Wait()
}
