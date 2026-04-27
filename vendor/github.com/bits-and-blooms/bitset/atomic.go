//go:build !go1.23
// +build !go1.23

package bitset

import _ "unsafe"

// Or8 atomically sets *ptr |= val and returns the previous *ptr value.
//
//go:linkname Or8 runtime/internal/atomic.Or8
func Or8(ptr *uint8, val uint8)
