package carrot

import (
	"sync"

	"golang.org/x/exp/slices"
)

type sliceSet[T comparable] struct {
	items []T
	mu    sync.RWMutex
}

func newSliceSet[T comparable]() *sliceSet[T] {
	return new(sliceSet[T])
}

func (slice *sliceSet[T]) Add(x T) {
	slice.mu.Lock()
	defer slice.mu.Unlock()
	index := slices.Index(slice.items, x)
	if index >= 0 {
		return
	}
	slice.items = append(slice.items, x)
}

func (slice *sliceSet[T]) Remove(x T) {
	slice.mu.Lock()
	defer slice.mu.Unlock()
	index := slices.Index(slice.items, x)
	if index >= 0 {
		slices.Delete(slice.items, index, index+1)
	}
}

func (slice *sliceSet[T]) Clear() {
	slice.mu.Lock()
	defer slice.mu.Unlock()
	slice.items = slice.items[:0]
}

func (slice *sliceSet[T]) Each(fn func(x T)) {
	if len(slice.items) == 0 {
		return
	}
	for _, x := range slice.items {
		fn(x)

	}
}
