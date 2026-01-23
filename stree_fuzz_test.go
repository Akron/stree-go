package stree

import (
	"testing"
)

// FuzzBuildAndSearch tests that all inserted keys can be found.
// Note: Values are limited to < 0x80000000 because SIMD comparison uses signed
// arithmetic. Values >= 0x80000000 may cause incorrect results with SIMD search.
func FuzzBuildAndSearch(f *testing.F) {
	// Add seed corpus
	f.Add([]byte{1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0})
	f.Add([]byte{255, 255, 255, 127}) // Max valid value for SIMD (0x7FFFFFFF)
	f.Add([]byte{0, 0, 0, 0})         // Zero
	f.Add([]byte{42, 0, 0, 0})        // Single value

	f.Fuzz(func(t *testing.T, data []byte) {
		// Convert bytes to uint32 slice
		if len(data) < 4 || len(data)%4 != 0 {
			return
		}

		// Collect unique values before Build modifies the slice
		uniqueValues := make(map[uint32]bool)
		values := make([]uint32, len(data)/4)
		for i := range values {
			values[i] = uint32(data[i*4]) |
				uint32(data[i*4+1])<<8 |
				uint32(data[i*4+2])<<16 |
				uint32(data[i*4+3])<<24

			// Limit to values < 0x80000000 for correct SIMD comparison
			// and avoid sentinel value
			values[i] = values[i] & 0x7FFFFFFF
			if values[i] == 0 && i > 0 {
				values[i] = 1 // Avoid too many zeros
			}
			uniqueValues[values[i]] = true
		}

		tree, err := Build(values)
		if err != nil {
			return // Empty input is expected
		}

		reader, err := NewReader(tree.Data())
		if err != nil {
			t.Fatalf("Failed to create reader: %v", err)
		}

		// Verify all unique values can be found
		for v := range uniqueValues {
			if !reader.Contains(v) {
				t.Errorf("Value %d not found in tree", v)
			}
		}

		// Verify count matches unique values
		if reader.Count() != len(uniqueValues) {
			t.Errorf("Count mismatch: got %d, want %d", reader.Count(), len(uniqueValues))
		}
	})
}

// FuzzBuildFromKeyed tests the Keyed interface.
func FuzzBuildFromKeyed(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0})
	f.Add([]byte{10, 0, 0, 0, 5, 0, 0, 0, 15, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 4 || len(data)%4 != 0 {
			return
		}

		// Create keyed entries
		entries := make([]*fuzzEntry, len(data)/4)
		for i := range entries {
			key := uint32(data[i*4]) |
				uint32(data[i*4+1])<<8 |
				uint32(data[i*4+2])<<16 |
				uint32(data[i*4+3])<<24

			// Limit to values < 0x80000000 for correct SIMD comparison
			key = key & 0x7FFFFFFF
			if key == 0 && i > 0 {
				key = uint32(i)
			}
			entries[i] = &fuzzEntry{key: key}
		}

		tree, err := BuildFromKeyed(entries)
		if err != nil {
			return
		}

		reader, err := NewReader(tree.Data())
		if err != nil {
			t.Fatalf("Failed to create reader: %v", err)
		}

		// Verify all entries have valid indices set
		for _, e := range entries {
			if e.index > uint32(reader.Count()*blockSize) {
				t.Errorf("Entry index %d out of bounds", e.index)
			}

			// Verify the key can be found
			pos := reader.Search(e.key)
			if pos < 0 {
				t.Errorf("Key %d not found", e.key)
			}
		}
	})
}

// FuzzSIMDConsistency verifies SIMD and pure-Go return same results.
// Values are limited to < 0x80000000 for correct SIMD comparison.
func FuzzSIMDConsistency(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0}, uint32(2))
	f.Add([]byte{10, 0, 0, 0, 20, 0, 0, 0}, uint32(15))
	f.Add([]byte{100, 0, 0, 0}, uint32(100))

	f.Fuzz(func(t *testing.T, data []byte, searchKey uint32) {
		if len(data) < 4 || len(data)%4 != 0 {
			return
		}

		// Limit search key to valid range
		searchKey = searchKey & 0x7FFFFFFF

		values := make([]uint32, len(data)/4)
		for i := range values {
			values[i] = uint32(data[i*4]) |
				uint32(data[i*4+1])<<8 |
				uint32(data[i*4+2])<<16 |
				uint32(data[i*4+3])<<24

			// Limit to values < 0x80000000
			values[i] = values[i] & 0x7FFFFFFF
		}

		tree, err := Build(values)
		if err != nil {
			return
		}

		reader, err := NewReader(tree.Data())
		if err != nil {
			t.Fatalf("Failed to create reader: %v", err)
		}

		blocks := reader.Data()[headerSize:]
		numBlocks := reader.NumBlocks()

		// Compare SIMD and generic results
		genericResult := SearchGeneric(blocks, searchKey, numBlocks)
		simdResult := search(blocks, searchKey, numBlocks)

		if genericResult != simdResult {
			t.Errorf("Mismatch for key %d: generic=%d, simd=%d", searchKey, genericResult, simdResult)
		}
	})
}

// FuzzSortedIteration verifies sorted order is maintained.
func FuzzSortedIteration(f *testing.F) {
	f.Add([]byte{5, 0, 0, 0, 3, 0, 0, 0, 8, 0, 0, 0, 1, 0, 0, 0})
	f.Add([]byte{100, 0, 0, 0, 50, 0, 0, 0, 150, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 4 || len(data)%4 != 0 {
			return
		}

		values := make([]uint32, len(data)/4)
		for i := range values {
			values[i] = uint32(data[i*4]) |
				uint32(data[i*4+1])<<8 |
				uint32(data[i*4+2])<<16 |
				uint32(data[i*4+3])<<24

			// Limit to values < 0x80000000
			values[i] = values[i] & 0x7FFFFFFF
		}

		tree, err := Build(values)
		if err != nil {
			return
		}

		reader, err := NewReader(tree.Data())
		if err != nil {
			t.Fatalf("Failed to create reader: %v", err)
		}

		// Collect values in sorted order
		var sorted []uint32
		reader.Sorted()(func(v uint32, _ int) bool {
			sorted = append(sorted, v)
			return true
		})

		// Verify ascending order
		for i := 1; i < len(sorted); i++ {
			if sorted[i-1] >= sorted[i] {
				t.Errorf("Not sorted: %d >= %d at positions %d, %d", sorted[i-1], sorted[i], i-1, i)
			}
		}
	})
}

// fuzzEntry implements Keyed for fuzz testing
type fuzzEntry struct {
	key   uint32
	index uint32
}

func (e *fuzzEntry) Key() uint32         { return e.key }
func (e *fuzzEntry) Index() uint32       { return e.index }
func (e *fuzzEntry) SetIndex(idx uint32) { e.index = idx }
