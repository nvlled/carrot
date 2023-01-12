package atombits

import "sync/atomic"

type T = atomic.Uint32

func IsSet(bits *T, flag uint32) bool {
	value := bits.Load()
	return value&flag != 0
}

func Set(bits *T, flag uint32) {
	value := bits.Load()
	bits.CompareAndSwap(value, value|flag)
}

func Unset(bits *T, flag uint32) {
	value := bits.Load()
	bits.CompareAndSwap(value, value&^flag)
}
