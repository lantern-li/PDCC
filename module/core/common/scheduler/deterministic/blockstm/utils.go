package blockstm

import "sync/atomic"

// DecrAtomic decreases the atomic value by 1
func DecrAtomic(a *atomic.Uint64) {
	a.Add(^uint64(0))
}

// IncrAtomic increases the atomic value by 1
func IncrAtomic(a *atomic.Uint64) {
	a.Add(1)
}
