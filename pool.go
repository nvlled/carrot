package carrot

import (
	"github.com/nvlled/mud"
)

// TODO: use arena
var coroutinePool = mud.NewPool()

func init() {
	PreAllocCoroutines(5)
}

// Pre-allocate a number of coroutine.
func PreAllocCoroutines(count int) {
	mud.PreAlloc(coroutinePool, NewControl, count)
}

func allocCoroutine() *Control {
	co := mud.Alloc(coroutinePool, NewControl)
	return co
}

func freeCoroutine(co *Control) {
	mud.Free(coroutinePool, co)
}
