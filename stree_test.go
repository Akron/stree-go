package stree

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildBasic tests basic S-Tree construction.
func TestBuildBasic(t *testing.T) {
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
		// Unique values: 1, 3, 5, 7
		assert.Equal(t, 4, st.Count())
	})

	t.Run("unsorted input", func(t *testing.T) {
		st, err := Build([]uint32{10, 5, 15, 3, 8, 12, 6, 9})
		require.NoError(t, err)
		assert.Equal(t, 8, st.Count())
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := Build([]uint32{})
		assert.ErrorIs(t, err, ErrEmptyInput)
	})

	t.Run("exact block size", func(t *testing.T) {
		input := make([]uint32, BlockSize)
		for i := range input {
			input[i] = uint32(i + 1)
		}
		st, err := Build(input)
		require.NoError(t, err)
		assert.Equal(t, BlockSize, st.Count())
		assert.Equal(t, 1, st.NumBlocks())
	})

	t.Run("multiple blocks", func(t *testing.T) {
		input := make([]uint32, BlockSize+5)
		for i := range input {
			input[i] = uint32(i + 1)
		}
		st, err := Build(input)
		require.NoError(t, err)
		assert.Equal(t, BlockSize+5, st.Count())
		assert.Equal(t, 2, st.NumBlocks())
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

// TestBuildFromKeyed tests the interface-based building API.
func TestBuildFromKeyed(t *testing.T) {
	t.Run("basic usage", func(t *testing.T) {
		entries := []*TestEntry{
			{key: 10, data: "ten"},
			{key: 5, data: "five"},
			{key: 20, data: "twenty"},
		}

		st, err := BuildFromKeyed(entries)
		require.NoError(t, err)
		assert.Equal(t, 3, st.Count())

		// Verify indices point to correct values
		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		for _, e := range entries {
			// Search should return the same index that was set
			pos := reader.Search(e.key)
			assert.GreaterOrEqual(t, pos, 0, "key %d should be found", e.key)
			assert.Equal(t, int(e.index), pos, "index for key %d should match search result", e.key)
		}
	})

	t.Run("with duplicates", func(t *testing.T) {
		entries := []*TestEntry{
			{key: 5, data: "first-five"},
			{key: 5, data: "second-five"}, // duplicate
			{key: 10, data: "ten"},
		}

		st, err := BuildFromKeyed(entries)
		require.NoError(t, err)
		assert.Equal(t, 2, st.Count()) // Only 2 unique keys

		// First occurrence should have a valid index
		reader, err := NewReader(st.Data())
		require.NoError(t, err)
		pos := reader.Search(entries[0].key)
		assert.Equal(t, int(entries[0].index), pos)
	})

	t.Run("empty input", func(t *testing.T) {
		var entries []*TestEntry
		_, err := BuildFromKeyed(entries)
		assert.ErrorIs(t, err, ErrEmptyInput)
	})

	t.Run("single entry", func(t *testing.T) {
		entries := []*TestEntry{{key: 42, data: "answer"}}
		st, err := BuildFromKeyed(entries)
		require.NoError(t, err)
		assert.Equal(t, 1, st.Count())

		reader, err := NewReader(st.Data())
		require.NoError(t, err)
		pos := reader.Search(42)
		assert.Equal(t, int(entries[0].index), pos)
	})

	t.Run("large dataset", func(t *testing.T) {
		entries := make([]*TestEntry, 1000)
		for i := range entries {
			entries[i] = &TestEntry{key: uint32(i * 2), data: "data"}
		}

		st, err := BuildFromKeyed(entries)
		require.NoError(t, err)
		assert.Equal(t, 1000, st.Count())

		// Verify random samples
		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		for _, i := range []int{0, 100, 500, 999} {
			pos := reader.Search(entries[i].key)
			assert.Equal(t, int(entries[i].index), pos)
		}
	})
}

// TestSearchBasic tests basic search functionality.
func TestSearchBasic(t *testing.T) {
	t.Run("find single element", func(t *testing.T) {
		st, err := Build([]uint32{42})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		assert.GreaterOrEqual(t, reader.Search(42), 0, "should find 42")
		assert.True(t, reader.Contains(42))
		assert.Equal(t, -1, reader.Search(41), "should not find 41")
		assert.False(t, reader.Contains(41))
	})

	t.Run("find in multiple elements", func(t *testing.T) {
		input := []uint32{10, 20, 30, 40, 50}
		st, err := Build(input)
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		for _, v := range input {
			pos := reader.Search(v)
			assert.GreaterOrEqual(t, pos, 0, "should find %d", v)
		}

		// Test non-existent values
		assert.Equal(t, -1, reader.Search(5))
		assert.Equal(t, -1, reader.Search(15))
		assert.Equal(t, -1, reader.Search(100))
	})

	t.Run("search returns correct value", func(t *testing.T) {
		st, err := Build([]uint32{1, 3, 5, 7, 9})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		for _, key := range []uint32{1, 3, 5, 7, 9} {
			pos := reader.Search(key)
			require.GreaterOrEqual(t, pos, 0)
			// Verify the position contains the correct value
			blockIdx := pos / BlockSize
			posInBlock := pos % BlockSize
			val := reader.blockValue(blockIdx, posInBlock)
			assert.Equal(t, key, val)
		}
	})
}

// TestSearchLarge tests search with larger datasets.
func TestSearchLarge(t *testing.T) {
	t.Run("100 elements", func(t *testing.T) {
		input := make([]uint32, 100)
		for i := range input {
			input[i] = uint32(i * 2) // Even numbers 0-198
		}

		st, err := Build(input)
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		// Find all even numbers
		for _, v := range input {
			assert.True(t, reader.Contains(v), "should contain %d", v)
		}

		// Don't find odd numbers
		for i := uint32(1); i < 200; i += 2 {
			assert.False(t, reader.Contains(i), "should not contain %d", i)
		}
	})

	t.Run("1000 elements", func(t *testing.T) {
		input := make([]uint32, 1000)
		for i := range input {
			input[i] = uint32(i * 3)
		}

		st, err := Build(input)
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		assert.Equal(t, 1000, reader.Count())

		// Spot check some values
		assert.True(t, reader.Contains(0))
		assert.True(t, reader.Contains(1500))
		assert.True(t, reader.Contains(2997))
		assert.False(t, reader.Contains(1))
		assert.False(t, reader.Contains(3000))
	})
}

// TestWriteToAndReadFrom tests serialization and deserialization.
func TestWriteToAndReadFrom(t *testing.T) {
	input := []uint32{10, 20, 5, 30, 15, 25, 8, 12, 18, 22}

	t.Run("write and read via buffer", func(t *testing.T) {
		inputCopy := make([]uint32, len(input))
		copy(inputCopy, input)
		st, err := Build(inputCopy)
		require.NoError(t, err)

		// Write to buffer
		var buf bytes.Buffer
		n, err := st.WriteTo(&buf)
		require.NoError(t, err)
		assert.Equal(t, int64(len(st.Data())), n)

		// Read back using NewReader
		reader, err := NewReader(buf.Bytes())
		require.NoError(t, err)
		assert.Equal(t, st.Count(), reader.Count())

		// Verify all values can be found
		for _, v := range input {
			assert.True(t, reader.Contains(v), "should contain %d", v)
		}
	})

	t.Run("data is identical", func(t *testing.T) {
		inputCopy := make([]uint32, len(input))
		copy(inputCopy, input)
		st, err := Build(inputCopy)
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = st.WriteTo(&buf)
		require.NoError(t, err)

		// Data should match exactly
		assert.Equal(t, st.Data(), buf.Bytes())
	})
}

// TestHeader tests header parsing and serialization.
func TestHeader(t *testing.T) {
	t.Run("parse valid header", func(t *testing.T) {
		st, err := Build([]uint32{1, 2, 3})
		require.NoError(t, err)

		header, err := ParseHeader(st.Data())
		require.NoError(t, err)

		assert.Equal(t, "STRE", string(header.Magic[:]))
		assert.Equal(t, Version, header.Version)
		assert.Equal(t, uint16(BlockSize), header.BlockSize)
		assert.Equal(t, uint32(3), header.Count)
	})

	t.Run("invalid magic", func(t *testing.T) {
		data := []byte("XXXX\x01\x00\x10\x00\x03\x00\x00\x00\x00\x00\x00\x00")
		_, err := ParseHeader(data)
		assert.ErrorIs(t, err, ErrInvalidMagic)
	})

	t.Run("data too short", func(t *testing.T) {
		_, err := ParseHeader([]byte("STRE"))
		assert.ErrorIs(t, err, ErrDataTooShort)
	})
}

// TestNewReader tests creating readers from byte slices.
func TestNewReader(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		st, err := Build([]uint32{1, 2, 3, 4, 5})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		assert.Equal(t, st.Count(), reader.Count())
		assert.Equal(t, st.NumBlocks(), reader.NumBlocks())
	})

	t.Run("truncated data", func(t *testing.T) {
		st, err := Build([]uint32{1, 2, 3, 4, 5})
		require.NoError(t, err)

		// Truncate the data
		truncated := st.Data()[:HeaderSize+10]
		_, err = NewReader(truncated)
		assert.ErrorIs(t, err, ErrDataTooShort)
	})
}

// TestAllIterator tests the All() iterator.
func TestAllIterator(t *testing.T) {
	input := []uint32{5, 3, 8, 1, 9, 2, 7, 4, 6}
	inputCopy := make([]uint32, len(input))
	copy(inputCopy, input)
	st, err := Build(inputCopy)
	require.NoError(t, err)

	reader, err := NewReader(st.Data())
	require.NoError(t, err)

	// Collect all values using the iterator
	var values []uint32
	reader.All()(func(v uint32) bool {
		values = append(values, v)
		return true // continue iteration
	})

	// Should have all unique input values
	assert.Len(t, values, len(input))

	// All input values should be present
	valueSet := make(map[uint32]bool)
	for _, v := range values {
		valueSet[v] = true
	}
	for _, v := range input {
		assert.True(t, valueSet[v], "should have value %d", v)
	}
}

// TestSortedIterator tests the Sorted() iterator for in-order traversal.
func TestSortedIterator(t *testing.T) {
	t.Run("basic sorted iteration", func(t *testing.T) {
		input := []uint32{5, 3, 8, 1, 9, 2, 7, 4, 6}
		inputCopy := make([]uint32, len(input))
		copy(inputCopy, input)

		st, err := Build(inputCopy)
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		// Collect values in sorted order
		var values []uint32
		reader.Sorted()(func(v uint32, _ int) bool {
			values = append(values, v)
			return true
		})

		// Should have all values
		assert.Len(t, values, len(input))

		// Values should be in ascending order
		for i := 1; i < len(values); i++ {
			assert.Less(t, values[i-1], values[i], "values should be in ascending order")
		}

		// Should match sorted input
		expected := []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9}
		assert.Equal(t, expected, values)
	})

	t.Run("large dataset", func(t *testing.T) {
		input := make([]uint32, 1000)
		for i := range input {
			// Reverse order to make it interesting
			input[i] = uint32(1000 - i)
		}

		st, err := Build(input)
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		var values []uint32
		reader.Sorted()(func(v uint32, _ int) bool {
			values = append(values, v)
			return true
		})

		assert.Len(t, values, 1000)

		// Verify ascending order
		for i := 1; i < len(values); i++ {
			assert.Less(t, values[i-1], values[i])
		}

		// First and last values should be correct
		assert.Equal(t, uint32(1), values[0])
		assert.Equal(t, uint32(1000), values[len(values)-1])
	})

	t.Run("early termination", func(t *testing.T) {
		input := make([]uint32, 100)
		for i := range input {
			input[i] = uint32(i + 1)
		}

		st, err := Build(input)
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		// Stop after 10 values
		var values []uint32
		reader.Sorted()(func(v uint32, _ int) bool {
			values = append(values, v)
			return len(values) < 10
		})

		assert.Len(t, values, 10)
		// First 10 sorted values
		for i, v := range values {
			assert.Equal(t, uint32(i+1), v)
		}
	})
}

// TestSortedWithIndex tests sorted iteration with index information.
func TestSortedWithIndex(t *testing.T) {
	input := []uint32{50, 30, 70, 20, 40, 60, 80}
	inputCopy := make([]uint32, len(input))
	copy(inputCopy, input)

	st, err := Build(inputCopy)
	require.NoError(t, err)

	reader, err := NewReader(st.Data())
	require.NoError(t, err)

	type pair struct {
		value uint32
		index int
	}

	var pairs []pair
	reader.Sorted()(func(value uint32, index int) bool {
		pairs = append(pairs, pair{value, index})
		return true
	})

	// Verify sorted order
	for i := 1; i < len(pairs); i++ {
		assert.Less(t, pairs[i-1].value, pairs[i].value)
	}

	// Verify each index maps back to the correct value
	for _, p := range pairs {
		// Read the value at the stored index
		blockIdx := p.index / BlockSize
		posInBlock := p.index % BlockSize
		val := reader.blockValue(blockIdx, posInBlock)
		assert.Equal(t, p.value, val)
	}
}

// TestSearchEdgeCases tests edge cases in search.
func TestSearchEdgeCases(t *testing.T) {
	t.Run("search for 0", func(t *testing.T) {
		st, err := Build([]uint32{0, 5, 10})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		assert.True(t, reader.Contains(0))
	})

	t.Run("search for max uint32 minus 1", func(t *testing.T) {
		// Sentinel is ^uint32(0), so max-1 should be valid
		maxVal := ^uint32(0) - 1
		st, err := Build([]uint32{1, maxVal})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		assert.True(t, reader.Contains(maxVal))
		assert.True(t, reader.Contains(1))
	})

	t.Run("search in sparse data", func(t *testing.T) {
		st, err := Build([]uint32{1, 1000000, 2000000000})
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		assert.True(t, reader.Contains(1))
		assert.True(t, reader.Contains(1000000))
		assert.True(t, reader.Contains(2000000000))
		assert.False(t, reader.Contains(500000))
	})
}

// TestSearchConsistency verifies search returns valid positions.
func TestSearchConsistency(t *testing.T) {
	testCases := [][]uint32{
		{42},
		{1, 2, 3, 4, 5},
		{5, 4, 3, 2, 1},
		{3, 1, 4, 1, 5, 9, 2, 6, 5, 3},
		{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
	}

	for _, input := range testCases {
		// Get unique values before Build modifies the slice
		seen := make(map[uint32]bool)
		var unique []uint32
		for _, v := range input {
			if !seen[v] {
				seen[v] = true
				unique = append(unique, v)
			}
		}

		// Make a copy since Build sorts in-place
		inputCopy := make([]uint32, len(input))
		copy(inputCopy, input)
		st, err := Build(inputCopy)
		require.NoError(t, err)

		reader, err := NewReader(st.Data())
		require.NoError(t, err)

		// Verify each unique value can be found
		for _, key := range unique {
			pos := reader.Search(key)
			require.GreaterOrEqual(t, pos, 0, "should find %d", key)

			// Verify position contains correct value
			blockIdx := pos / BlockSize
			posInBlock := pos % BlockSize
			val := reader.blockValue(blockIdx, posInBlock)
			assert.Equal(t, key, val, "position %d should contain %d", pos, key)
		}
	}
}

// TestNumBlocks tests block count calculations.
func TestNumBlocks(t *testing.T) {
	tests := []struct {
		count    int
		expected int
	}{
		{0, 0},
		{1, 1},
		{BlockSize, 1},
		{BlockSize + 1, 2},
		{BlockSize * 2, 2},
		{BlockSize*2 + 1, 3},
		{100, 7}, // ceil(100/16) = 7
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, NumBlocks(tt.count),
			"NumBlocks(%d) should be %d", tt.count, tt.expected)
	}
}

// TestDataSize tests total data size calculations.
func TestDataSize(t *testing.T) {
	tests := []struct {
		count    int
		expected int
	}{
		{0, HeaderSize},
		{1, HeaderSize + BlockSizeBytes},
		{BlockSize, HeaderSize + BlockSizeBytes},
		{BlockSize + 1, HeaderSize + 2*BlockSizeBytes},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, DataSize(tt.count),
			"DataSize(%d) should be %d", tt.count, tt.expected)
	}
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

			blocks := reader.Data()[HeaderSize:]
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
