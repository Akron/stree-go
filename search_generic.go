package stree

// search is the function variable that holds the current search implementation.
// It is set to searchGeneric by default, but can be replaced with SIMD
// implementations on supported architectures via init().
var search = searchGeneric

// simdAvailable indicates if SIMD search is available on the current platform.
var simdAvailable = false

// SIMDAvailable returns true if SIMD-accelerated search is available.
func SIMDAvailable() bool {
	return simdAvailable
}

// searchGeneric implements S-Tree search using pure Go with manually unrolled
// block traversal. Each block position requires only 2 branches (equality +
// greater-than) instead of 3, because the sentinel value (0xFFFFFFFF) is
// always greater than any valid key under unsigned comparison.
// Time complexity: O(log_17 n) where n is the number of elements.
func searchGeneric(blocks []byte, key uint32, numBlocks int) int {
	if len(blocks) == 0 || numBlocks == 0 {
		return -1
	}

	k := 0

	for k < numBlocks {
		blockStart := k * blockSizeBytes
		childIdx := blockSize

		v := be.Uint32(blocks[blockStart:])
		if v == key {
			return k*blockSize + 0
		}
		if v > key {
			childIdx = 0
			goto descend
		}

		v = be.Uint32(blocks[blockStart+4:])
		if v == key {
			return k*blockSize + 1
		}
		if v > key {
			childIdx = 1
			goto descend
		}

		v = be.Uint32(blocks[blockStart+8:])
		if v == key {
			return k*blockSize + 2
		}
		if v > key {
			childIdx = 2
			goto descend
		}

		v = be.Uint32(blocks[blockStart+12:])
		if v == key {
			return k*blockSize + 3
		}
		if v > key {
			childIdx = 3
			goto descend
		}

		v = be.Uint32(blocks[blockStart+16:])
		if v == key {
			return k*blockSize + 4
		}
		if v > key {
			childIdx = 4
			goto descend
		}

		v = be.Uint32(blocks[blockStart+20:])
		if v == key {
			return k*blockSize + 5
		}
		if v > key {
			childIdx = 5
			goto descend
		}

		v = be.Uint32(blocks[blockStart+24:])
		if v == key {
			return k*blockSize + 6
		}
		if v > key {
			childIdx = 6
			goto descend
		}

		v = be.Uint32(blocks[blockStart+28:])
		if v == key {
			return k*blockSize + 7
		}
		if v > key {
			childIdx = 7
			goto descend
		}

		v = be.Uint32(blocks[blockStart+32:])
		if v == key {
			return k*blockSize + 8
		}
		if v > key {
			childIdx = 8
			goto descend
		}

		v = be.Uint32(blocks[blockStart+36:])
		if v == key {
			return k*blockSize + 9
		}
		if v > key {
			childIdx = 9
			goto descend
		}

		v = be.Uint32(blocks[blockStart+40:])
		if v == key {
			return k*blockSize + 10
		}
		if v > key {
			childIdx = 10
			goto descend
		}

		v = be.Uint32(blocks[blockStart+44:])
		if v == key {
			return k*blockSize + 11
		}
		if v > key {
			childIdx = 11
			goto descend
		}

		v = be.Uint32(blocks[blockStart+48:])
		if v == key {
			return k*blockSize + 12
		}
		if v > key {
			childIdx = 12
			goto descend
		}

		v = be.Uint32(blocks[blockStart+52:])
		if v == key {
			return k*blockSize + 13
		}
		if v > key {
			childIdx = 13
			goto descend
		}

		v = be.Uint32(blocks[blockStart+56:])
		if v == key {
			return k*blockSize + 14
		}
		if v > key {
			childIdx = 14
			goto descend
		}

		v = be.Uint32(blocks[blockStart+60:])
		if v == key {
			return k*blockSize + 15
		}
		if v > key {
			childIdx = 15
			goto descend
		}

	descend:
		k = k*(blockSize+1) + childIdx + 1
	}

	return -1
}

// searchSimple is a straightforward loop-based implementation used for
// comparison and fallback. Same 2-branch optimization as searchGeneric.
func searchSimple(blocks []byte, key uint32, numBlocks int) int {
	if len(blocks) == 0 || numBlocks == 0 {
		return -1
	}

	k := 0

	for k < numBlocks {
		blockStart := k * blockSizeBytes
		childIdx := blockSize

		for i := range blockSize {
			offset := blockStart + i*4
			nodeKey := be.Uint32(blocks[offset:])

			if nodeKey == key {
				return k*blockSize + i
			}

			if nodeKey > key {
				childIdx = i
				break
			}
		}

		k = k*(blockSize+1) + childIdx + 1
	}

	return -1
}
