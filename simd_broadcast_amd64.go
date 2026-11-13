//go:build goexperiment.simd && amd64

package stree

import "simd/archsimd"

// archsimd.BroadcastUint32x4 and BroadcastInt32x4 are documented as
// "CPU Feature: AVX2" (VPBROADCASTD) in some toolchains. On a CPU with AVX
// but without AVX2 (Sandy Bridge, Ivy Bridge, Bulldozer ... Piledriver)
// they raise SIGILL. The helpers build the same vector with AVX-only
// instructions: VPINSRD (SetElem) puts x into element 0, VPSHUFD
// (PermuteScalars with constant indices) copies it to all four elements.
//
// Do NOT build the vector through a [4]uint32 array on the stack and load it
// with LoadUint32x4Array: that is four 4-byte stores followed by one 16-byte
// load, which the CPU cannot forward from the store buffer (store-forwarding
// stall, ~12-15 cycles per vector). With three vectors per Search call this
// made searchSSE2 slower than searchGeneric on small trees.

// broadcastUint32x4 fills a 128-bit vector with x using AVX instructions only.
func broadcastUint32x4(x uint32) archsimd.Uint32x4 {
	var z archsimd.Uint32x4
	return z.SetElem(0, x).PermuteScalars(0, 0, 0, 0)
}

// broadcastInt32x4 fills a 128-bit vector with x using AVX instructions only.
func broadcastInt32x4(x int32) archsimd.Int32x4 {
	var z archsimd.Int32x4
	return z.SetElem(0, x).PermuteScalars(0, 0, 0, 0)
}
