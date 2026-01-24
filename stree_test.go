package stree

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Example demonstrates basic S-Tree usage: building a tree from values and searching.
func Example() {
	// Build a tree from uint32 values.
	// Note: The input slice will be sorted and deduplicated in-place.
	values := []uint32{42, 17, 100, 5, 73}
	tree, err := Build(values)
	if err != nil {
		panic(err)
	}

	// Create a reader from the tree's serialized data.
	reader, err := NewReader(tree.Data())
	if err != nil {
		panic(err)
	}

	// Search for a value. Returns the position if found, -1 if not.
	pos := reader.Search(42)
	fmt.Printf("Search(42): found=%v\n", pos >= 0)

	pos = reader.Search(99)
	fmt.Printf("Search(99): found=%v\n", pos >= 0)

	// Use Contains for simple membership testing.
	fmt.Printf("Contains(100): %v\n", reader.Contains(100))

	fmt.Printf("Count: %d\n", reader.Count())
	// Output:
	// Search(42): found=true
	// Search(99): found=false
	// Contains(100): true
	// Count: 5
}

// ExampleBuild demonstrates building an S-Tree from uint32 values.
func ExampleBuild() {
	// Input does not need to be sorted; duplicates are removed automatically.
	values := []uint32{30, 10, 20, 10, 40, 20}
	tree, err := Build(values)
	if err != nil {
		panic(err)
	}

	// Count reflects unique values only.
	fmt.Printf("Unique count: %d\n", tree.Count())

	// The tree data can be written to disk or memory-mapped later.
	fmt.Printf("Data size: %d bytes\n", len(tree.Data()))
	// Output:
	// Unique count: 4
	// Data size: 80 bytes
}

// Document is an example type implementing the Keyed interface.
// This allows associating additional data with each key in the tree.
type Document struct {
	ID       uint32
	Position uint32 // Set by BuildFromKeyed during tree construction
	Title    string
}

func (d *Document) Key() uint32         { return d.ID }
func (d *Document) Index() uint32       { return d.Position }
func (d *Document) SetIndex(idx uint32) { d.Position = idx }

// ExampleBuildFromKeyed demonstrates building an S-Tree while tracking
// the position of each item in the tree structure.
func ExampleBuildFromKeyed() {
	// Create documents with IDs (keys) and associated data.
	docs := []*Document{
		{ID: 300, Title: "Third"},
		{ID: 100, Title: "First"},
		{ID: 200, Title: "Second"},
	}

	// Build the tree. Each document's Position field will be set to its
	// index in the tree, enabling O(1) lookup of associated data.
	tree, err := BuildFromKeyed(docs)
	if err != nil {
		panic(err)
	}

	// After building, we can look up documents by their tree position.
	reader, err := NewReader(tree.Data())
	if err != nil {
		panic(err)
	}

	// Search returns the same position that was stored in the Document.
	pos := reader.Search(200)
	fmt.Printf("Position of ID 200: %d\n", pos)

	// Find the document with that position.
	for _, doc := range docs {
		if int(doc.Position) == pos {
			fmt.Printf("Found: %s\n", doc.Title)
		}
	}
	// Output:
	// Position of ID 200: 1
	// Found: Second
}

// ExampleReader_Sorted demonstrates iterating through all values in sorted order.
// This performs an in-order traversal of the Eytzinger tree structure,
// yielding both the value and its position in the tree.
func ExampleReader_Sorted() {
	values := []uint32{50, 10, 40, 20, 30}
	tree, _ := Build(values)
	reader, _ := NewReader(tree.Data())

	// Sorted() returns an iterator that yields (value, index) pairs
	// in ascending order by value.
	fmt.Println("Values in sorted order:")
	reader.Sorted()(func(value uint32, index int) bool {
		fmt.Printf("  value=%d, index=%d\n", value, index)
		return true // return false to stop iteration early
	})
	// Output:
	// Values in sorted order:
	//   value=10, index=0
	//   value=20, index=1
	//   value=30, index=2
	//   value=40, index=3
	//   value=50, index=4
}

// ExampleSTree_WriteTo demonstrates serializing an S-Tree to a writer.
func ExampleSTree_WriteTo() {
	values := []uint32{1, 2, 3, 4, 5}
	tree, _ := Build(values)

	// Write to any io.Writer (file, buffer, network, etc.)
	var buf bytes.Buffer
	n, err := tree.WriteTo(&buf)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Wrote %d bytes\n", n)

	// The serialized data can be read back with NewReader.
	reader, _ := NewReader(buf.Bytes())
	fmt.Printf("Reader count: %d\n", reader.Count())
	// Output:
	// Wrote 80 bytes
	// Reader count: 5
}

// ExampleNewReaderWithValidation demonstrates creating a reader with CRC-32 validation.
func ExampleNewReaderWithValidation() {
	tree, _ := Build([]uint32{10, 20, 30})

	// NewReaderWithValidation checks the CRC-32 checksum during construction.
	// Use this when loading from untrusted sources (network, disk).
	reader, err := NewReaderWithValidation(tree.Data())
	if err != nil {
		fmt.Println("Data integrity check failed!")
		return
	}

	fmt.Printf("Valid data, count: %d\n", reader.Count())
	// Output:
	// Valid data, count: 3
}

// TestBuild tests S-Tree construction with various inputs.
func TestBuild(t *testing.T) {
	t.Run("single element", func(t *testing.T) {
		st, err := Build([]uint32{42})
		require.NoError(t, err)
		assert.Equal(t, 1, st.Count())
		assert.Equal(t, 1, st.NumBlocks())
	})

	t.Run("multiple elements", func(t *testing.T) {
		st, err := Build([]uint32{10, 20, 30, 40, 50})
		require.NoError(t, err)
		assert.Equal(t, 5, st.Count())
	})

	t.Run("with duplicates", func(t *testing.T) {
		st, err := Build([]uint32{5, 3, 5, 1, 3, 5, 7})
		require.NoError(t, err)
		assert.Equal(t, 4, st.Count()) // Unique: 1, 3, 5, 7
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := Build([]uint32{})
		assert.ErrorIs(t, err, ErrEmptyInput)
	})

	t.Run("nil input", func(t *testing.T) {
		_, err := Build(nil)
		assert.ErrorIs(t, err, ErrEmptyInput)
	})

	t.Run("exact block size boundary", func(t *testing.T) {
		// Test boundary: exactly 16 elements = 1 block
		input := make([]uint32, blockSize)
		for i := range input {
			input[i] = uint32(i + 1)
		}
		st, err := Build(input)
		require.NoError(t, err)
		assert.Equal(t, 1, st.NumBlocks())

		// Test boundary: 17 elements = 2 blocks
		input2 := make([]uint32, blockSize+1)
		for i := range input2 {
			input2[i] = uint32(i + 1)
		}
		st2, err := Build(input2)
		require.NoError(t, err)
		assert.Equal(t, 2, st2.NumBlocks())
	})

	t.Run("value too large", func(t *testing.T) {
		_, err := Build([]uint32{1, 0x80000000})
		assert.ErrorIs(t, err, ErrValueTooLarge)

		_, err = Build([]uint32{^uint32(0)})
		assert.ErrorIs(t, err, ErrValueTooLarge)
	})
}

// TestSentinelInitialization verifies that empty block slots are correctly
// initialized with sentinel values (0xFFFFFFFF) using 8-byte writes.
func TestSentinelInitialization(t *testing.T) {
	t.Run("partial block has sentinels", func(t *testing.T) {
		// Create tree with fewer elements than a full block
		st, err := Build([]uint32{1, 2, 3})
		require.NoError(t, err)

		data := st.Data()
		blocks := data[headerSize:]

		// Count sentinels in the block (should be blockSize - 3 = 13)
		sentinelCount := 0
		for i := 0; i < blockSizeBytes; i += 4 {
			val := be.Uint32(blocks[i:])
			if val == sentinel {
				sentinelCount++
			}
		}
		assert.Equal(t, blockSize-3, sentinelCount, "should have 13 sentinel values")
	})

	t.Run("multiple blocks have sentinels", func(t *testing.T) {
		// 20 elements = 2 blocks, second block has 4 elements + 12 sentinels
		input := make([]uint32, 20)
		for i := range input {
			input[i] = uint32(i + 1)
		}
		st, err := Build(input)
		require.NoError(t, err)

		data := st.Data()
		blocks := data[headerSize:]

		// Check that all bytes not occupied by values are 0xFF
		// This verifies the 8-byte sentinel initialization works correctly
		expectedSentinels := (st.NumBlocks() * blockSize) - st.Count()
		sentinelCount := 0
		for i := 0; i < len(blocks); i += 4 {
			val := be.Uint32(blocks[i:])
			if val == sentinel {
				sentinelCount++
			}
		}
		assert.Equal(t, expectedSentinels, sentinelCount)
	})

	t.Run("sentinel bytes are 0xFF", func(t *testing.T) {
		st, err := Build([]uint32{1})
		require.NoError(t, err)

		data := st.Data()
		blocks := data[headerSize:]

		// Count 0xFF bytes - should be at least (blockSize-1)*4 bytes
		ffCount := 0
		for i := range blocks {
			if blocks[i] == 0xFF {
				ffCount++
			}
		}
		// Assert that there are at least (blockSize-1)*4 bytes of 0xFF
		assert.GreaterOrEqual(t, ffCount, (blockSize-1)*4)

		// Optionally: Assert that at least some bytes after the first value are 0xFF
		assert.Greater(t, ffCount, 0)
	})
}

// TestEntry is a test type implementing Keyed interface.
type TestEntry struct {
	key   uint32
	index uint32
	data  string
}

func (e *TestEntry) Key() uint32         { return e.key }
func (e *TestEntry) Index() uint32       { return e.index }
func (e *TestEntry) SetIndex(idx uint32) { e.index = idx }

// TestBuildFromKeyed tests the Keyed interface building API.
func TestBuildFromKeyed(t *testing.T) {
	t.Run("index correlation", func(t *testing.T) {
		entries := []*TestEntry{
			{key: 10, data: "ten"},
			{key: 5, data: "five"},
			{key: 20, data: "twenty"},
		}

		st, err := BuildFromKeyed(entries)
		require.NoError(t, err)
		assert.Equal(t, 3, st.Count())

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		// Verify SetIndex was called and indices match search results
		for _, e := range entries {
			pos := reader.Search(e.key)
			assert.Equal(t, int(e.index), pos, "index for key %d should match", e.key)
		}
	})

	t.Run("duplicates keep first", func(t *testing.T) {
		entries := []*TestEntry{
			{key: 5, data: "first"},
			{key: 5, data: "second"}, // duplicate - should be ignored
			{key: 10, data: "ten"},
		}

		st, err := BuildFromKeyed(entries)
		require.NoError(t, err)
		assert.Equal(t, 2, st.Count())
	})

	t.Run("empty and nil input", func(t *testing.T) {
		var nilEntries []*TestEntry
		_, err := BuildFromKeyed(nilEntries)
		assert.ErrorIs(t, err, ErrEmptyInput)

		_, err = BuildFromKeyed([]*TestEntry{})
		assert.ErrorIs(t, err, ErrEmptyInput)
	})

	t.Run("value too large", func(t *testing.T) {
		entries := []*TestEntry{{key: 0x80000000}}
		_, err := BuildFromKeyed(entries)
		assert.ErrorIs(t, err, ErrValueTooLarge)
	})
}

// TestSearch tests search functionality.
func TestSearch(t *testing.T) {
	t.Run("found and not found", func(t *testing.T) {
		st, err := Build([]uint32{10, 20, 30, 40, 50})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		// Existing values
		assert.True(t, reader.Contains(10))
		assert.True(t, reader.Contains(30))
		assert.True(t, reader.Contains(50))

		// Non-existent values
		assert.False(t, reader.Contains(5))
		assert.False(t, reader.Contains(25))
		assert.False(t, reader.Contains(100))
	})

	t.Run("position maps to correct value", func(t *testing.T) {
		st, err := Build([]uint32{1, 3, 5, 7, 9})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		for _, key := range []uint32{1, 3, 5, 7, 9} {
			pos := reader.Search(key)
			require.GreaterOrEqual(t, pos, 0)
			blockIdx := pos / blockSize
			posInBlock := pos % blockSize
			assert.Equal(t, key, reader.blockValue(blockIdx, posInBlock))
		}
	})

	t.Run("empty blocks search", func(t *testing.T) {
		// searchGeneric should handle empty blocks gracefully
		result := searchGeneric(nil, 42, 0)
		assert.Equal(t, -1, result)

		result = searchGeneric([]byte{}, 42, 0)
		assert.Equal(t, -1, result)
	})
}

// TestWriteTo tests serialization.
func TestWriteTo(t *testing.T) {
	st, err := Build([]uint32{10, 20, 30, 40, 50})
	require.NoError(t, err)

	var buf bytes.Buffer
	n, err := st.WriteTo(&buf)
	require.NoError(t, err)
	assert.Equal(t, int64(len(st.Data())), n)
	assert.Equal(t, st.Data(), buf.Bytes())

	// Verify roundtrip
	reader, err := NewReader(buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, st.Count(), reader.Count())
	assert.True(t, reader.Contains(30))
}

// TestHeaderParsing tests header validation errors.
func TestHeaderParsing(t *testing.T) {
	t.Run("valid header", func(t *testing.T) {
		st, err := Build([]uint32{1, 2, 3})
		require.NoError(t, err)

		header, err := parseHeader(st.Data())
		require.NoError(t, err)
		assert.Equal(t, "STRE", string(header.magic[:]))
		assert.Equal(t, Version, header.version)
		assert.Equal(t, uint32(3), header.count)
	})

	t.Run("invalid magic", func(t *testing.T) {
		data := []byte("XXXX\x01\x00\x10\x00\x03\x00\x00\x00\x00\x00\x00\x00")
		_, err := parseHeader(data)
		assert.ErrorIs(t, err, ErrInvalidMagic)
	})

	t.Run("invalid version", func(t *testing.T) {
		// Valid magic but wrong version (0x9999)
		data := []byte("STRE\x99\x99\x10\x00\x03\x00\x00\x00\x00\x00\x00\x00")
		_, err := parseHeader(data)
		assert.ErrorIs(t, err, ErrInvalidVersion)
	})

	t.Run("invalid block size", func(t *testing.T) {
		// Valid magic/version but wrong block size (8 instead of 16)
		data := []byte("STRE\x01\x00\x08\x00\x03\x00\x00\x00\x00\x00\x00\x00")
		_, err := parseHeader(data)
		assert.ErrorIs(t, err, ErrInvalidBlockSz)
	})

	t.Run("data too short", func(t *testing.T) {
		_, err := parseHeader([]byte("STRE"))
		assert.ErrorIs(t, err, ErrDataTooShort)

		_, err = parseHeader(nil)
		assert.ErrorIs(t, err, ErrDataTooShort)
	})
}

// TestNewReader tests reader creation edge cases.
func TestNewReader(t *testing.T) {
	t.Run("truncated block data", func(t *testing.T) {
		st, err := Build([]uint32{1, 2, 3, 4, 5})
		require.NoError(t, err)

		// Header is valid but block data is truncated
		truncated := st.Data()[:headerSize+10]
		_, err = NewReader(truncated)
		assert.ErrorIs(t, err, ErrDataTooShort)
	})

	t.Run("reader references original data", func(t *testing.T) {
		st, err := Build([]uint32{1, 2, 3})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		// Reader.Data() should return the same slice
		assert.Equal(t, st.Data(), reader.Data())
	})
}

// TestIterators tests All() and Sorted() iterators.
func TestIterators(t *testing.T) {
	input := []uint32{5, 3, 8, 1, 9, 2, 7, 4, 6}
	st, err := Build(append([]uint32{}, input...))
	require.NoError(t, err)

	reader, err := NewReader(st.Data())
	require.NoError(t, err)

	t.Run("All returns all values", func(t *testing.T) {
		var values []uint32
		reader.All()(func(v uint32) bool {
			values = append(values, v)
			return true
		})

		assert.Len(t, values, len(input))
		valueSet := make(map[uint32]bool)
		for _, v := range values {
			valueSet[v] = true
		}
		for _, v := range input {
			assert.True(t, valueSet[v])
		}
	})

	t.Run("Sorted returns ascending order", func(t *testing.T) {
		var values []uint32
		reader.Sorted()(func(v uint32, _ int) bool {
			values = append(values, v)
			return true
		})

		expected := []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9}
		assert.Equal(t, expected, values)
	})

	t.Run("Sorted index maps to correct value", func(t *testing.T) {
		reader.Sorted()(func(value uint32, index int) bool {
			blockIdx := index / blockSize
			posInBlock := index % blockSize
			assert.Equal(t, value, reader.blockValue(blockIdx, posInBlock))
			return true
		})
	})

	t.Run("early termination", func(t *testing.T) {
		count := 0
		reader.Sorted()(func(_ uint32, _ int) bool {
			count++
			return count < 3
		})
		assert.Equal(t, 3, count)
	})

	t.Run("empty tree iteration", func(t *testing.T) {
		// Create a reader with zero count (simulated)
		emptyReader := &Reader{numBlocks: 0}
		count := 0
		emptyReader.Sorted()(func(_ uint32, _ int) bool {
			count++
			return true
		})
		assert.Equal(t, 0, count)
	})
}

// TestSearchEdgeCases tests boundary conditions for search.
func TestSearchEdgeCases(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		st, err := Build([]uint32{0, 5, 10})
		require.NoError(t, err)
		reader, _ := NewReader(st.Data())
		assert.True(t, reader.Contains(0))
	})

	t.Run("MaxValue boundary", func(t *testing.T) {
		st, err := Build([]uint32{1, MaxValue})
		require.NoError(t, err)
		reader, _ := NewReader(st.Data())

		assert.True(t, reader.Contains(MaxValue))
		assert.Equal(t, -1, reader.Search(MaxValue+1)) // Just above max
		assert.Equal(t, -1, reader.Search(^uint32(0))) // Max uint32
	})

	t.Run("sparse values", func(t *testing.T) {
		st, err := Build([]uint32{1, 1000000, 2000000000})
		require.NoError(t, err)
		reader, _ := NewReader(st.Data())

		assert.True(t, reader.Contains(1))
		assert.True(t, reader.Contains(1000000))
		assert.True(t, reader.Contains(2000000000))
		assert.False(t, reader.Contains(500000))
	})
}

// TestHelperFunctions tests numBlocks and DataSize calculations.
func TestHelperFunctions(t *testing.T) {
	t.Run("numBlocks", func(t *testing.T) {
		assert.Equal(t, 0, numBlocks(0))
		assert.Equal(t, 0, numBlocks(-1))
		assert.Equal(t, 1, numBlocks(1))
		assert.Equal(t, 1, numBlocks(blockSize))
		assert.Equal(t, 2, numBlocks(blockSize+1))
		assert.Equal(t, 7, numBlocks(100)) // ceil(100/16)
	})

	t.Run("DataSize", func(t *testing.T) {
		assert.Equal(t, headerSize, DataSize(0))
		assert.Equal(t, headerSize, DataSize(-1))
		assert.Equal(t, headerSize+blockSizeBytes, DataSize(1))
		assert.Equal(t, headerSize+blockSizeBytes, DataSize(blockSize))
		assert.Equal(t, headerSize+2*blockSizeBytes, DataSize(blockSize+1))
	})

	t.Run("childIndex", func(t *testing.T) {
		// k=0, i=0 -> child is 1
		assert.Equal(t, 1, childIndex(0, 0))
		// k=0, i=16 -> rightmost child of root is 17
		assert.Equal(t, 17, childIndex(0, blockSize))
		// k=1, i=0 -> child is 18
		assert.Equal(t, 18, childIndex(1, 0))
	})
}

// TestSIMDConsistency verifies SIMD and pure-Go implementations return same results.
func TestSIMDConsistency(t *testing.T) {
	sizes := []int{1, 10, 100, 1000, 10000}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("n=%d", size), func(t *testing.T) {
			input := make([]uint32, size)
			for i := range input {
				input[i] = uint32(i * 2)
			}

			st, err := Build(input)
			require.NoError(t, err)

			reader, err := NewReader(st.Data())
			require.NoError(t, err)

			blocks := reader.Data()[headerSize:]
			numBlocks := reader.NumBlocks()

			// Test keys that exist
			for i := 0; i < size; i += max(1, size/100) {
				key := uint32(i * 2)
				genericResult := searchGeneric(blocks, key, numBlocks)
				simdResult := search(blocks, key, numBlocks)
				assert.Equal(t, genericResult, simdResult,
					"mismatch for existing key %d: generic=%d, simd=%d", key, genericResult, simdResult)
			}

			// Test keys that don't exist
			for i := 0; i < size; i += max(1, size/100) {
				key := uint32(i*2 + 1) // Odd numbers don't exist
				genericResult := searchGeneric(blocks, key, numBlocks)
				simdResult := search(blocks, key, numBlocks)
				assert.Equal(t, genericResult, simdResult,
					"mismatch for non-existing key %d: generic=%d, simd=%d", key, genericResult, simdResult)
			}
		})
	}
}

// TestCRC32 tests CRC-32 checksum functionality.
func TestCRC32(t *testing.T) {
	t.Run("valid after build", func(t *testing.T) {
		st, err := Build([]uint32{10, 20, 30})
		require.NoError(t, err)

		reader, _ := NewReader(st.Data())
		assert.True(t, reader.ValidateCRC32())
	})

	t.Run("NewReaderWithValidation detects corruption", func(t *testing.T) {
		st, err := Build([]uint32{100, 200, 300})
		require.NoError(t, err)

		// Corrupt block data
		data := append([]byte{}, st.Data()...)
		data[headerSize+4] ^= 0xFF

		_, err = NewReaderWithValidation(data)
		assert.ErrorIs(t, err, ErrInvalidData)
	})

	t.Run("CRC field tampering detected", func(t *testing.T) {
		st, err := Build([]uint32{5, 10, 15})
		require.NoError(t, err)

		data := append([]byte{}, st.Data()...)
		data[12] ^= 0x01 // Flip bit in CRC field

		reader, _ := NewReader(data)
		assert.False(t, reader.ValidateCRC32())
	})

	t.Run("short data returns false", func(t *testing.T) {
		assert.False(t, validateCRC32([]byte("SHORT")))
		assert.False(t, validateCRC32(nil))
	})
}
