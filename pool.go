package carrot

import (
	"github.com/nvlled/mud"
)

var gathering = mud.NewPool()

func init() {
	Populate(10)
}

// Pre-allocate a number of invokers of the given type.
func Populate(count int) {
	mud.PreAlloc(gathering, NewInvoker, count)
}

func summonInvoker() *Invoker {
	in := mud.Alloc(gathering, NewInvoker)
	return in
}
func disperseInvoker(in *Invoker) {
	mud.Free(gathering, in)
}
