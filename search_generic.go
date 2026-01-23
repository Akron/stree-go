package stree

import "encoding/binary"

// search is the function variable that holds the current search implementation.
// It is set to searchGeneric by default, but can be replaced with SIMD
// implementations on supported architectures via initSIMDSelection().
var search = searchGeneric

// simdAvailable indicates if SIMD search is available on the current platform.
var simdAvailable = false

// searchGeneric implements S-Tree search using pure Go.
// This is the reference implementation that works on all platforms.
//
// The search follows the implicit B-tree structure using Eytzinger numbering:
// - Start at root node (k = 0)
// - For each node, find the first key >= search key
// - Descend to the appropriate child using: k * 17 + childIndex + 1
// - Repeat until key is found or we exceed the number of blocks
//
// Time complexity: O(log₁₇ n) where n is the number of elements.
func searchGeneric(blocks []byte, key uint32, numBlocks int) int {
	if len(blocks) == 0 || numBlocks == 0 {
		return -1
	}

	k := 0 // Current node index (start at root)

	for k < numBlocks {
		blockStart := k * BlockSizeBytes
		childIdx := BlockSize // Default to rightmost child

		// Unrolled search for 16-element blocks
		// This avoids loop overhead and allows better CPU branch prediction

		// Position 0
		v := binary.LittleEndian.Uint32(blocks[blockStart:])
		if v == Sentinel {
			childIdx = 0
			goto descend
		}
		if v == key {
			return k*BlockSize + 0
		}
		if v > key {
			childIdx = 0
			goto descend
		}

		// Position 1
		v = binary.LittleEndian.Uint32(blocks[blockStart+4:])
		if v == Sentinel {
			childIdx = 1
			goto descend
		}
		if v == key {
			return k*BlockSize + 1
		}
		if v > key {
			childIdx = 1
			goto descend
		}

		// Position 2
		v = binary.LittleEndian.Uint32(blocks[blockStart+8:])
		if v == Sentinel {
			childIdx = 2
			goto descend
		}
		if v == key {
			return k*BlockSize + 2
		}
		if v > key {
			childIdx = 2
			goto descend
		}

		// Position 3
		v = binary.LittleEndian.Uint32(blocks[blockStart+12:])
		if v == Sentinel {
			childIdx = 3
			goto descend
		}
		if v == key {
			return k*BlockSize + 3
		}
		if v > key {
			childIdx = 3
			goto descend
		}

		// Position 4
		v = binary.LittleEndian.Uint32(blocks[blockStart+16:])
		if v == Sentinel {
			childIdx = 4
			goto descend
		}
		if v == key {
			return k*BlockSize + 4
		}
		if v > key {
			childIdx = 4
			goto descend
		}

		// Position 5
		v = binary.LittleEndian.Uint32(blocks[blockStart+20:])
		if v == Sentinel {
			childIdx = 5
			goto descend
		}
		if v == key {
			return k*BlockSize + 5
		}
		if v > key {
			childIdx = 5
			goto descend
		}

		// Position 6
		v = binary.LittleEndian.Uint32(blocks[blockStart+24:])
		if v == Sentinel {
			childIdx = 6
			goto descend
		}
		if v == key {
			return k*BlockSize + 6
		}
		if v > key {
			childIdx = 6
			goto descend
		}

		// Position 7
		v = binary.LittleEndian.Uint32(blocks[blockStart+28:])
		if v == Sentinel {
			childIdx = 7
			goto descend
		}
		if v == key {
			return k*BlockSize + 7
		}
		if v > key {
			childIdx = 7
			goto descend
		}

		// Position 8
		v = binary.LittleEndian.Uint32(blocks[blockStart+32:])
		if v == Sentinel {
			childIdx = 8
			goto descend
		}
		if v == key {
			return k*BlockSize + 8
		}
		if v > key {
			childIdx = 8
			goto descend
		}

		// Position 9
		v = binary.LittleEndian.Uint32(blocks[blockStart+36:])
		if v == Sentinel {
			childIdx = 9
			goto descend
		}
		if v == key {
			return k*BlockSize + 9
		}
		if v > key {
			childIdx = 9
			goto descend
		}

		// Position 10
		v = binary.LittleEndian.Uint32(blocks[blockStart+40:])
		if v == Sentinel {
			childIdx = 10
			goto descend
		}
		if v == key {
			return k*BlockSize + 10
		}
		if v > key {
			childIdx = 10
			goto descend
		}

		// Position 11
		v = binary.LittleEndian.Uint32(blocks[blockStart+44:])
		if v == Sentinel {
			childIdx = 11
			goto descend
		}
		if v == key {
			return k*BlockSize + 11
		}
		if v > key {
			childIdx = 11
			goto descend
		}

		// Position 12
		v = binary.LittleEndian.Uint32(blocks[blockStart+48:])
		if v == Sentinel {
			childIdx = 12
			goto descend
		}
		if v == key {
			return k*BlockSize + 12
		}
		if v > key {
			childIdx = 12
			goto descend
		}

		// Position 13
		v = binary.LittleEndian.Uint32(blocks[blockStart+52:])
		if v == Sentinel {
			childIdx = 13
			goto descend
		}
		if v == key {
			return k*BlockSize + 13
		}
		if v > key {
			childIdx = 13
			goto descend
		}

		// Position 14
		v = binary.LittleEndian.Uint32(blocks[blockStart+56:])
		if v == Sentinel {
			childIdx = 14
			goto descend
		}
		if v == key {
			return k*BlockSize + 14
		}
		if v > key {
			childIdx = 14
			goto descend
		}

		// Position 15
		v = binary.LittleEndian.Uint32(blocks[blockStart+60:])
		if v == Sentinel {
			childIdx = 15
			goto descend
		}
		if v == key {
			return k*BlockSize + 15
		}
		if v > key {
			childIdx = 15
			goto descend
		}

		// All 16 values are less than key, go to rightmost child
		// childIdx is already 16

	descend:
		k = k*(BlockSize+1) + childIdx + 1
	}

	return -1 // Not found
}

// searchSimple is a simpler implementation used for comparison/fallback.
// It uses straightforward sequential search within each block.
func searchSimple(blocks []byte, key uint32, numBlocks int) int {
	if len(blocks) == 0 || numBlocks == 0 {
		return -1
	}

	k := 0

	for k < numBlocks {
		blockStart := k * BlockSizeBytes
		childIdx := BlockSize

		for i := range BlockSize {
			offset := blockStart + i*4
			nodeKey := binary.LittleEndian.Uint32(blocks[offset:])

			if nodeKey == Sentinel {
				childIdx = i
				break
			}

			if nodeKey == key {
				return k*BlockSize + i
			}

			if nodeKey > key {
				childIdx = i
				break
			}
		}

		k = k*(BlockSize+1) + childIdx + 1
	}

	return -1
}
