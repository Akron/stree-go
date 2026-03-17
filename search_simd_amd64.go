//go:build goexperiment.simd && amd64

package stree

import (
	"math"
	"math/bits"
	"simd/archsimd"
	"unsafe"
)

func init() {
	if archsimd.X86.AVX512() {
		search = searchAVX512
	} else if archsimd.X86.AVX2() {
		search = searchAVX2
	} else {
		search = searchSSE2
	}
	simdAvailable = true
}

// HasAVX512 returns true if AVX-512 is available.
func HasAVX512() bool { return archsimd.X86.AVX512() }

// HasAVX2 returns true if AVX2 is available.
func HasAVX2() bool { return archsimd.X86.AVX2() }

// HasSSE2 returns true if SSE2 is available (always true on amd64).
func HasSSE2() bool { return true }

// Exported search function variables for benchmarking.
var (
	SearchAVX512  = searchAVX512
	SearchAVX2    = searchAVX2
	SearchSSE2    = searchSSE2
	SearchGeneric = searchGeneric
)

// searchSSE2 searches the S-Tree using 128-bit SIMD (Uint32x4).
// Manually unrolled into 4 explicit chunks per block. Uses pre-computed
// sign-bit bias (XOR 0x80000000) with signed PCMPGTD instead of the
// per-call bias trick in Uint32x4.Greater(), reducing from 5 to 2
// SIMD instructions per unsigned comparison.
func searchSSE2(blocks []byte, key uint32, numBlocks int) int {
	if len(blocks) == 0 || numBlocks == 0 {
		return -1
	}

	keyVec := archsimd.BroadcastUint32x4(key)
	biasVec := archsimd.BroadcastInt32x4(math.MinInt32)
	biasedKeyVec := archsimd.BroadcastInt32x4(int32(key ^ 0x80000000))
	base := unsafe.Pointer(unsafe.SliceData(blocks))
	k := 0

	for k < numBlocks {
		ptr := unsafe.Add(base, uintptr(k*blockSizeBytes))
		childIdx := blockSize
		var eq, gt uint8
		var c archsimd.Uint32x4

		c = archsimd.LoadUint32x4((*[4]uint32)(ptr))
		eq = c.Equal(keyVec).ToBits()
		if eq != 0 {
			return k*blockSize + bits.TrailingZeros8(eq)
		}
		gt = c.AsInt32x4().Xor(biasVec).Greater(biasedKeyVec).ToBits()
		if gt != 0 {
			childIdx = bits.TrailingZeros8(gt)
			goto descend
		}

		c = archsimd.LoadUint32x4((*[4]uint32)(unsafe.Add(ptr, 16)))
		eq = c.Equal(keyVec).ToBits()
		if eq != 0 {
			return k*blockSize + 4 + bits.TrailingZeros8(eq)
		}
		gt = c.AsInt32x4().Xor(biasVec).Greater(biasedKeyVec).ToBits()
		if gt != 0 {
			childIdx = 4 + bits.TrailingZeros8(gt)
			goto descend
		}

		c = archsimd.LoadUint32x4((*[4]uint32)(unsafe.Add(ptr, 32)))
		eq = c.Equal(keyVec).ToBits()
		if eq != 0 {
			return k*blockSize + 8 + bits.TrailingZeros8(eq)
		}
		gt = c.AsInt32x4().Xor(biasVec).Greater(biasedKeyVec).ToBits()
		if gt != 0 {
			childIdx = 8 + bits.TrailingZeros8(gt)
			goto descend
		}

		c = archsimd.LoadUint32x4((*[4]uint32)(unsafe.Add(ptr, 48)))
		eq = c.Equal(keyVec).ToBits()
		if eq != 0 {
			return k*blockSize + 12 + bits.TrailingZeros8(eq)
		}
		gt = c.AsInt32x4().Xor(biasVec).Greater(biasedKeyVec).ToBits()
		if gt != 0 {
			childIdx = 12 + bits.TrailingZeros8(gt)
			goto descend
		}

	descend:
		k = k*(blockSize+1) + childIdx + 1
	}

	return -1
}

// searchAVX2 searches the S-Tree using AVX2 (Uint32x8).
func searchAVX2(blocks []byte, key uint32, numBlocks int) int {
	if len(blocks) == 0 || numBlocks == 0 {
		return -1
	}

	keyVec := archsimd.BroadcastUint32x8(key)
	biasVec := archsimd.BroadcastInt32x8(math.MinInt32)
	biasedKeyVec := archsimd.BroadcastInt32x8(int32(key ^ 0x80000000))
	base := unsafe.Pointer(unsafe.SliceData(blocks))
	k := 0

	for k < numBlocks {
		ptr := unsafe.Add(base, uintptr(k*blockSizeBytes))
		lo := archsimd.LoadUint32x8((*[8]uint32)(ptr))
		hi := archsimd.LoadUint32x8((*[8]uint32)(unsafe.Add(ptr, 32)))

		eqLo := lo.Equal(keyVec).ToBits()
		if eqLo != 0 {
			archsimd.ClearAVXUpperBits()
			return k*blockSize + bits.TrailingZeros8(eqLo)
		}

		eqHi := hi.Equal(keyVec).ToBits()
		if eqHi != 0 {
			archsimd.ClearAVXUpperBits()
			return k*blockSize + 8 + bits.TrailingZeros8(eqHi)
		}

		gtLo := lo.AsInt32x8().Xor(biasVec).Greater(biasedKeyVec).ToBits()
		childIdx := blockSize
		if gtLo != 0 {
			childIdx = bits.TrailingZeros8(gtLo)
		} else {
			gtHi := hi.AsInt32x8().Xor(biasVec).Greater(biasedKeyVec).ToBits()
			if gtHi != 0 {
				childIdx = 8 + bits.TrailingZeros8(gtHi)
			}
		}

		k = k*(blockSize+1) + childIdx + 1
	}

	archsimd.ClearAVXUpperBits()
	return -1
}

// searchAVX512 searches the S-Tree using AVX-512 (Uint32x16).
// Processes the entire 16-element block in a single load. Uses native
// unsigned VPCMPUD (no bias trick needed on AVX-512).
func searchAVX512(blocks []byte, key uint32, numBlocks int) int {
	if len(blocks) == 0 || numBlocks == 0 {
		return -1
	}

	keyVec := archsimd.BroadcastUint32x16(key)
	base := unsafe.Pointer(unsafe.SliceData(blocks))
	k := 0

	for k < numBlocks {
		ptr := unsafe.Add(base, uintptr(k*blockSizeBytes))
		block := archsimd.LoadUint32x16((*[16]uint32)(ptr))

		eqMask := block.Equal(keyVec).ToBits()
		if eqMask != 0 {
			archsimd.ClearAVXUpperBits()
			return k*blockSize + bits.TrailingZeros16(eqMask)
		}

		gtMask := block.Greater(keyVec).ToBits()
		childIdx := blockSize
		if gtMask != 0 {
			childIdx = bits.TrailingZeros16(gtMask)
		}

		k = k*(blockSize+1) + childIdx + 1
	}

	archsimd.ClearAVXUpperBits()
	return -1
}
