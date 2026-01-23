package stree

import "encoding/binary"

// Reader provides read-only access to an S-Tree.
// It can work with any byte slice, including memory-mapped files.
type Reader struct {
	data      []byte // Raw data (header + blocks)
	blocks    []byte // Pointer to block data (data[HeaderSize:])
	count     int    // Number of elements
	numBlocks int    // Number of blocks
}

// NewReader creates a Reader from a byte slice.
// The slice must contain valid S-Tree data (header + blocks).
// The slice is NOT copied; the Reader references the original data.
//
// This is designed to work with any byte slice source, including:
//   - In-memory data from STree.Data()
//   - Memory-mapped files (using syscall.Mmap or mmap libraries)
//   - Data read from disk
func NewReader(data []byte) (*Reader, error) {
	header, err := ParseHeader(data)
	if err != nil {
		return nil, err
	}

	count := int(header.Count)
	numBlocks := NumBlocks(count)
	expectedSize := HeaderSize + numBlocks*BlockSizeBytes

	if len(data) < expectedSize {
		return nil, ErrDataTooShort
	}

	return &Reader{
		data:      data,
		blocks:    data[HeaderSize:],
		count:     count,
		numBlocks: numBlocks,
	}, nil
}

// Count returns the number of elements in the S-Tree.
func (r *Reader) Count() int {
	return r.count
}

// NumBlocks returns the number of blocks in the S-Tree.
func (r *Reader) NumBlocks() int {
	return r.numBlocks
}

// Data returns the underlying byte slice.
func (r *Reader) Data() []byte {
	return r.data
}

// Search searches for a key in the S-Tree using tree traversal.
// Returns the position in the data array where the key is found, or -1 if not found.
// This uses the optimized pure-Go implementation; SIMD-optimized versions are
// available on supported architectures via SearchSIMD.
func (r *Reader) Search(key uint32) int {
	return search(r.blocks, key, r.numBlocks)
}

// Contains returns true if the S-Tree contains the given key.
func (r *Reader) Contains(key uint32) bool {
	return r.Search(key) >= 0
}

// blockValue reads a uint32 value from block k at position i.
func (r *Reader) blockValue(k, i int) uint32 {
	offset := k*BlockSizeBytes + i*4
	return binary.LittleEndian.Uint32(r.blocks[offset:])
}

// All returns an iterator over all non-sentinel values in the tree.
// Values are returned in tree traversal order (not necessarily sorted).
// For sorted iteration, use Sorted().
func (r *Reader) All() func(yield func(uint32) bool) {
	return func(yield func(uint32) bool) {
		for blockIdx := range r.numBlocks {
			for i := range BlockSize {
				val := r.blockValue(blockIdx, i)
				if val == Sentinel {
					continue
				}
				if !yield(val) {
					return
				}
			}
		}
	}
}

// Sorted returns an iterator over all (value, index) pairs in sorted (ascending) order.
// The index is the position in the S-Tree data structure.
// This performs an in-order traversal of the Eytzinger tree structure.
// This is useful for merging trees while preserving index information.
func (r *Reader) Sorted() func(yield func(value uint32, index int) bool) {
	return func(yield func(value uint32, index int) bool) {
		if r.numBlocks == 0 {
			return
		}
		r.inOrderTraversal(0, 0, yield)
	}
}

// inOrderTraversal performs an in-order traversal yielding both value and index.
// Returns false if iteration should stop.
func (r *Reader) inOrderTraversal(k, i int, yield func(value uint32, index int) bool) bool {
	if k >= r.numBlocks {
		return true
	}

	for ; i < BlockSize; i++ {
		// Traverse left child
		childK := childIndex(k, i)
		if childK < r.numBlocks {
			if !r.inOrderTraversal(childK, 0, yield) {
				return false
			}
		}

		// Yield current value with its index
		val := r.blockValue(k, i)
		if val == Sentinel {
			return true
		}
		idx := k*BlockSize + i
		if !yield(val, idx) {
			return false
		}
	}

	// Traverse rightmost child
	childK := childIndex(k, BlockSize)
	if childK < r.numBlocks {
		if !r.inOrderTraversal(childK, 0, yield) {
			return false
		}
	}

	return true
}
