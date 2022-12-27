package carrot

import (
	"github.com/nvlled/mud"
)

var gathering = mud.NewPool()

func init() {
	Populate(1000)
}

// Pre-allocate a number of invokers of the given type.
func Populate(count int) {
	mud.PreAlloc(gathering, newInvoker, count)
}

// Allocate a invoker using an object pool.
// Free the invoker afterwards with Free().
// Use only when gc is a concern.
func SummonInvoker() Invoker {
	in := mud.Alloc(gathering, newInvoker)
	return in
}

// Free a invoker that was previously Alloc()'d.
func DisperseInvoker(in Invoker) {
	object, ok := in.(*invoker)
	if !ok {
		return
	}
	mud.Free(gathering, object)
}

func summonInvoker() *invoker {
	in := mud.Alloc(gathering, newInvoker)
	return in
}
func disperseInvoker(in *invoker) {
	mud.Free(gathering, in)
}
