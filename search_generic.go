package stree

// search is the function variable that holds the current search implementation.
// It is set to searchGeneric by default, but can be replaced with SIMD
// implementations on supported architectures via initSIMDSelection().
var search = searchGeneric

// simdAvailable indicates if SIMD search is available on the current platform.
var simdAvailable = false

// SIMDAvailable returns true if SIMD-accelerated search is available.
func SIMDAvailable() bool {
	return simdAvailable
}

// searchGeneric implements S-Tree search using pure Go.
// This should work on all platforms.
// Time complexity: O(log₁₇ n) where n is the number of elements.
// This is an optimized implementation - for an easier to understand
// implementation of the algorithm, see searchSimple.
func searchGeneric(blocks []byte, key uint32, numBlocks int) int {
	if len(blocks) == 0 || numBlocks == 0 {
		return -1
	}

	k := 0 // Current node index (start at root)

	for k < numBlocks {
		blockStart := k * blockSizeBytes
		childIdx := blockSize // Default to rightmost child

		// Unrolled search for 16-element blocks
		// This avoids loop overhead and allows better CPU branch prediction

		// Position 0
		v := be.Uint32(blocks[blockStart:])
		if v == sentinel {
			childIdx = 0
			goto descend
		}
		if v == key {
			return k*blockSize + 0
		}
		if v > key {
			childIdx = 0
			goto descend
		}

		// Position 1
		v = be.Uint32(blocks[blockStart+4:])
		if v == sentinel {
			childIdx = 1
			goto descend
		}
		if v == key {
			return k*blockSize + 1
		}
		if v > key {
			childIdx = 1
			goto descend
		}

		// Position 2
		v = be.Uint32(blocks[blockStart+8:])
		if v == sentinel {
			childIdx = 2
			goto descend
		}
		if v == key {
			return k*blockSize + 2
		}
		if v > key {
			childIdx = 2
			goto descend
		}

		// Position 3
		v = be.Uint32(blocks[blockStart+12:])
		if v == sentinel {
			childIdx = 3
			goto descend
		}
		if v == key {
			return k*blockSize + 3
		}
		if v > key {
			childIdx = 3
			goto descend
		}

		// Position 4
		v = be.Uint32(blocks[blockStart+16:])
		if v == sentinel {
			childIdx = 4
			goto descend
		}
		if v == key {
			return k*blockSize + 4
		}
		if v > key {
			childIdx = 4
			goto descend
		}

		// Position 5
		v = be.Uint32(blocks[blockStart+20:])
		if v == sentinel {
			childIdx = 5
			goto descend
		}
		if v == key {
			return k*blockSize + 5
		}
		if v > key {
			childIdx = 5
			goto descend
		}

		// Position 6
		v = be.Uint32(blocks[blockStart+24:])
		if v == sentinel {
			childIdx = 6
			goto descend
		}
		if v == key {
			return k*blockSize + 6
		}
		if v > key {
			childIdx = 6
			goto descend
		}

		// Position 7
		v = be.Uint32(blocks[blockStart+28:])
		if v == sentinel {
			childIdx = 7
			goto descend
		}
		if v == key {
			return k*blockSize + 7
		}
		if v > key {
			childIdx = 7
			goto descend
		}

		// Position 8
		v = be.Uint32(blocks[blockStart+32:])
		if v == sentinel {
			childIdx = 8
			goto descend
		}
		if v == key {
			return k*blockSize + 8
		}
		if v > key {
			childIdx = 8
			goto descend
		}

		// Position 9
		v = be.Uint32(blocks[blockStart+36:])
		if v == sentinel {
			childIdx = 9
			goto descend
		}
		if v == key {
			return k*blockSize + 9
		}
		if v > key {
			childIdx = 9
			goto descend
		}

		// Position 10
		v = be.Uint32(blocks[blockStart+40:])
		if v == sentinel {
			childIdx = 10
			goto descend
		}
		if v == key {
			return k*blockSize + 10
		}
		if v > key {
			childIdx = 10
			goto descend
		}

		// Position 11
		v = be.Uint32(blocks[blockStart+44:])
		if v == sentinel {
			childIdx = 11
			goto descend
		}
		if v == key {
			return k*blockSize + 11
		}
		if v > key {
			childIdx = 11
			goto descend
		}

		// Position 12
		v = be.Uint32(blocks[blockStart+48:])
		if v == sentinel {
			childIdx = 12
			goto descend
		}
		if v == key {
			return k*blockSize + 12
		}
		if v > key {
			childIdx = 12
			goto descend
		}

		// Position 13
		v = be.Uint32(blocks[blockStart+52:])
		if v == sentinel {
			childIdx = 13
			goto descend
		}
		if v == key {
			return k*blockSize + 13
		}
		if v > key {
			childIdx = 13
			goto descend
		}

		// Position 14
		v = be.Uint32(blocks[blockStart+56:])
		if v == sentinel {
			childIdx = 14
			goto descend
		}
		if v == key {
			return k*blockSize + 14
		}
		if v > key {
			childIdx = 14
			goto descend
		}

		// Position 15
		v = be.Uint32(blocks[blockStart+60:])
		if v == sentinel {
			childIdx = 15
			goto descend
		}
		if v == key {
			return k*blockSize + 15
		}
		if v > key {
			childIdx = 15
			goto descend
		}

		// All 16 values are less than key, go to rightmost child
		// childIdx is already 16

	descend:
		k = k*(blockSize+1) + childIdx + 1
	}

	return -1 // Not found
}

// searchSimple is a naive implementation used for comparison/fallback.
// It uses straightforward sequential search within each block.
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

			if nodeKey == sentinel {
				childIdx = i
				break
			}

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
