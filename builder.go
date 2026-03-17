package stree

import (
	"io"
	"slices"
)

// Keyed is the interface for types that can be indexed in an S-Tree.
// Implementations must provide a key for sorting/searching and accept
// an index position after the tree is built.
type Keyed interface {
	// Key returns the uint32 key used for building and searching the S-Tree.
	Key() uint32
	// Index returns the stored index position (set by SetIndex during building).
	Index() uint32
	// SetIndex is called during building with the position of this key in the S-Tree.
	// This allows correlating the key with additional data stored elsewhere.
	SetIndex(idx uint32)
}

// STree represents an S-Tree (Static B-Tree) in memory.
// It can be created from a slice of uint32 values and written to disk.
type STree struct {
	data  []byte // Complete serialized data (header + blocks)
	count int    // Number of unique elements
}

// Build creates a new S-Tree from the given slice of uint32 values.
// The input slice does not need to be sorted; duplicates will be removed.
// Returns ErrEmptyInput if the input is empty.
// Returns ErrValueTooLarge if any value equals the sentinel (0xFFFFFFFF).
//
// WARNING: The input slice will be sorted in-place. If you need to preserve
// the original order, make a copy before calling Build.
func Build(values []uint32) (*STree, error) {
	if len(values) == 0 {
		return nil, ErrEmptyInput
	}

	// Sort in-place and remove duplicates using slices.Compact
	slices.Sort(values)
	unique := slices.Compact(values)

	if len(unique) == 0 {
		return nil, ErrEmptyInput
	}

	// Since the slice is sorted, we only need to check the last (largest) value
	if unique[len(unique)-1] > MaxValue {
		return nil, ErrValueTooLarge
	}

	return buildFromUnique(unique)
}

// BuildFromKeyed creates a new S-Tree from a slice of Keyed items.
// The input does not need to be sorted; duplicates will be removed.
// During building, each unique item's SetIndex method is called with its position in the tree.
// This is the most efficient way to build a tree when you need index correlation.
// Returns ErrEmptyInput if the input is empty.
// Returns ErrValueTooLarge if any key equals the sentinel (0xFFFFFFFF).
//
// WARNING: The input slice will be reordered in-place (sorted by key).
// If you need to preserve the original order, make a copy before calling BuildFromKeyed.
//
// This is useful when you need to correlate keys with additional data:
//
//	type Entry struct {
//	    key   uint32
//	    index uint32
//	    data  []byte
//	}
//	func (e *Entry) Key() uint32            { return e.key }
//	func (e *Entry) Index() uint32          { return e.index }
//	func (e *Entry) SetIndex(idx uint32)    { e.index = idx }
//
//	entries := []*Entry{{key: 10}, {key: 5}, {key: 20}}
//	tree, _ := stree.BuildFromKeyed(entries)
//	// Now entries[i].Index() contains its position in the tree
func BuildFromKeyed[T Keyed](items []T) (*STree, error) {
	if len(items) == 0 {
		return nil, ErrEmptyInput
	}

	// Sort items by key in-place
	slices.SortFunc(items, func(a, b T) int {
		ak, bk := a.Key(), b.Key()
		if ak < bk {
			return -1
		}
		if ak > bk {
			return 1
		}
		return 0
	})

	// Remove duplicates and extract unique keys and items
	unique := make([]uint32, 0, len(items))
	uniqueItems := make([]T, 0, len(items))

	var prevKey uint32
	for i, item := range items {
		key := item.Key()
		if i == 0 || key != prevKey {
			unique = append(unique, key)
			uniqueItems = append(uniqueItems, item)
			prevKey = key
		}
	}

	if len(unique) == 0 {
		return nil, ErrEmptyInput
	}

	// Since the slice is sorted, we only need to check the last (largest) key
	if unique[len(unique)-1] > MaxValue {
		return nil, ErrValueTooLarge
	}

	// Build the tree, setting indices during construction
	return buildFromUniqueKeyed(unique, uniqueItems)
}

// buildFromUnique creates an S-Tree from a sorted, deduplicated slice (no index tracking).
func buildFromUnique(unique []uint32) (*STree, error) {
	// Calculate required space
	count := len(unique)
	numBlocks := numBlocks(count)
	totalSize := headerSize + numBlocks*blockSizeBytes

	// Allocate buffer
	data := make([]byte, totalSize)

	// Write header initially (CRC32 will be computed after data)
	header := &header{
		version:   Version,
		blockSize: blockSize,
		count:     uint32(count),
		crc32:     0, // Will be set after data construction
	}
	copy(header.magic[:], Magic)
	copy(data[0:headerSize], header.bytes())

	// Build tree data using Eytzinger layout
	blocks := data[headerSize:]
	buildEytzinger(unique, blocks, numBlocks)

	// Compute and store CRC-32
	header.crc32 = computeCRC32(data)
	copy(data[0:headerSize], header.bytes())

	return &STree{
		data:  data,
		count: count,
	}, nil
}

// buildEytzinger constructs the S-Tree using Eytzinger numeration (no index tracking).
func buildEytzinger(unique []uint32, blocks []byte, numBlocks int) {
	// Initialize all blocks with sentinel values (0xFFFFFFFF)
	// Write 8 bytes at a time for efficiency; blocks are always multiples of 64 bytes
	const sentinel64 = uint64(0xFFFFFFFFFFFFFFFF)
	for i := 0; i < len(blocks); i += 8 {
		be.PutUint64(blocks[i:], sentinel64)
	}

	t := 0 // Current position in input array

	var build func(k int)
	build = func(k int) {
		if k < numBlocks {
			for i := range blockSize {
				build(childIndex(k, i))
				if t < len(unique) {
					offset := k*blockSizeBytes + i*4
					be.PutUint32(blocks[offset:], unique[t])
					t++
				}
			}
			build(childIndex(k, blockSize))
		}
	}

	build(0)
}

// buildFromUniqueKeyed creates an S-Tree from a sorted, deduplicated slice with index tracking.
// SetIndex is called on each item during construction.
func buildFromUniqueKeyed[T Keyed](unique []uint32, items []T) (*STree, error) {
	// Calculate required space
	count := len(unique)
	numBlocks := numBlocks(count)
	totalSize := headerSize + numBlocks*blockSizeBytes

	// Allocate buffer
	data := make([]byte, totalSize)

	// Write header initially (CRC32 will be computed after data)
	header := &header{
		version:   Version,
		blockSize: blockSize,
		count:     uint32(count),
		crc32:     0, // Will be set after data construction
	}
	copy(header.magic[:], Magic)
	copy(data[0:headerSize], header.bytes())

	// Build tree data using Eytzinger layout
	blocks := data[headerSize:]
	buildEytzingerWithIndex(unique, items, blocks, numBlocks)

	// Compute and store CRC-32
	header.crc32 = computeCRC32(data)
	copy(data[0:headerSize], header.bytes())

	return &STree{
		data:  data,
		count: count,
	}, nil
}

// buildEytzingerWithIndex constructs the S-Tree using Eytzinger numeration.
// If items is non-nil, SetIndex is called on each item with its position in the tree.
// This follows the algorithm from the algorithmica paper:
//
//	void build(int k = 0) {
//	    static int t = 0;
//	    if (k < nblocks) {
//	        for (int i = 0; i < B; i++) {
//	            build(go(k, i));
//	            btree[k][i] = (t < n ? a[t++] : INT_MAX);
//	        }
//	        build(go(k, B));
//	    }
//	}
func buildEytzingerWithIndex[T Keyed](unique []uint32, items []T, blocks []byte, numBlocks int) {
	// Initialize all blocks with sentinel values (0xFFFFFFFF)
	// Write 8 bytes at a time for efficiency; blocks are always multiples of 64 bytes
	const sentinel64 = uint64(0xFFFFFFFFFFFFFFFF)
	for i := 0; i < len(blocks); i += 8 {
		be.PutUint64(blocks[i:], sentinel64)
	}

	t := 0 // Current position in input array

	var build func(k int)
	build = func(k int) {
		if k < numBlocks {
			// For each position in the block
			for i := range blockSize {
				// Recursively build left child
				build(childIndex(k, i))

				// Place current element or sentinel
				if t < len(unique) {
					offset := k*blockSizeBytes + i*4
					be.PutUint32(blocks[offset:], unique[t])

					// Set index on the item if provided - this is the key optimization!
					// The position in the tree is: block * BlockSize + position in block
					if items != nil {
						items[t].SetIndex(uint32(k*blockSize + i))
					}

					t++
				}
			}
			// Recursively build rightmost child
			build(childIndex(k, blockSize))
		}
	}

	// Start building from root (node 0)
	build(0)
}

// Count returns the number of unique elements in the S-Tree.
func (st *STree) Count() int {
	return st.count
}

// NumBlocks returns the number of blocks in the S-Tree.
func (st *STree) NumBlocks() int {
	return numBlocks(st.count)
}

// Data returns the underlying byte slice containing the serialized S-Tree.
// This can be used directly with mmap or written to a file.
func (st *STree) Data() []byte {
	return st.data
}

// WriteTo writes the S-Tree to an io.Writer.
// Implements io.WriterTo interface.
func (st *STree) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(st.data)
	return int64(n), err
}
