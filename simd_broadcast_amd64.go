//go:build goexperiment.simd && amd64

package stree

import "simd/archsimd"

// archsimd.BroadcastUint32x4 and BroadcastInt32x4 are documented as
// "CPU Feature: AVX2" (VPBROADCASTD) in some toolchains. On a CPU with AVX
// but without AVX2 (Sandy Bridge, Ivy Bridge, Bulldozer ... Piledriver)
// they raise SIGILL. The helpers build the same vector from a four-element
// array with a plain (AVX) vector load instead.

// broadcastUint32x4 fills a 128-bit vector with x using AVX instructions only.
func broadcastUint32x4(x uint32) archsimd.Uint32x4 {
	v := [4]uint32{x, x, x, x}
	return archsimd.LoadUint32x4Array(&v)
}

// broadcastInt32x4 fills a 128-bit vector with x using AVX instructions only.
func broadcastInt32x4(x int32) archsimd.Int32x4 {
	v := [4]int32{x, x, x, x}
	return archsimd.LoadInt32x4Array(&v)
}
