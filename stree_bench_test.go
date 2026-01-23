package stree

import (
	"fmt"
	"testing"
)

// BenchmarkSearch benchmarks search performance across different tree sizes.
func BenchmarkSearch(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		input := make([]uint32, size)
		for i := range input {
			input[i] = uint32(i * 2) // Even numbers
		}

		st, err := Build(input)
		if err != nil {
			b.Fatal(err)
		}

		reader, err := NewReader(st.Data())
		if err != nil {
			b.Fatal(err)
		}

		// Search for middle element (exists)
		searchKey := uint32(size)

		b.Run(fmt.Sprintf("Found/n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				reader.Search(searchKey)
			}
		})

		// Search for non-existent element
		missingKey := uint32(size + 1) // Odd number, won't exist

		b.Run(fmt.Sprintf("NotFound/n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				reader.Search(missingKey)
			}
		})
	}
}

// BenchmarkSearchCompare compares optimized vs simple search.
func BenchmarkSearchCompare(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		input := make([]uint32, size)
		for i := range input {
			input[i] = uint32(i * 2)
		}

		st, err := Build(input)
		if err != nil {
			b.Fatal(err)
		}

		reader, err := NewReader(st.Data())
		if err != nil {
			b.Fatal(err)
		}

		searchKey := uint32(size) // Middle element

		b.Run(fmt.Sprintf("Optimized/n=%d", size), func(b *testing.B) {
			blocks := reader.Data()[HeaderSize:]
			numBlocks := reader.NumBlocks()
			for i := 0; i < b.N; i++ {
				searchGeneric(blocks, searchKey, numBlocks)
			}
		})

		b.Run(fmt.Sprintf("Simple/n=%d", size), func(b *testing.B) {
			blocks := reader.Data()[HeaderSize:]
			numBlocks := reader.NumBlocks()
			for i := 0; i < b.N; i++ {
				searchSimple(blocks, searchKey, numBlocks)
			}
		})
	}
}

// BenchmarkBuild benchmarks tree construction.
func BenchmarkBuild(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		input := make([]uint32, size)
		for i := range input {
			input[i] = uint32(i)
		}

		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = Build(input)
			}
		})
	}
}

// BenchEntry is a test type implementing Keyed interface.
type BenchEntry struct {
	key   uint32
	index uint32
}

func (e *BenchEntry) Key() uint32         { return e.key }
func (e *BenchEntry) Index() uint32       { return e.index }
func (e *BenchEntry) SetIndex(idx uint32) { e.index = idx }

// BenchmarkBuildFromKeyed benchmarks interface-based tree construction.
func BenchmarkBuildFromKeyed(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		entries := make([]*BenchEntry, size)
		for i := range entries {
			entries[i] = &BenchEntry{key: uint32(i)}
		}

		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Reset indices
				for _, e := range entries {
					e.index = 0
				}
				_, _ = BuildFromKeyed(entries)
			}
		})
	}
}

// BenchmarkSortedIteration benchmarks in-order traversal.
func BenchmarkSortedIteration(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		input := make([]uint32, size)
		for i := range input {
			input[i] = uint32(i * 2)
		}

		st, err := Build(input)
		if err != nil {
			b.Fatal(err)
		}

		reader, err := NewReader(st.Data())
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("Sorted/n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				count := 0
				reader.Sorted()(func(v uint32, _ int) bool {
					count++
					return true
				})
			}
		})

		b.Run(fmt.Sprintf("All/n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				count := 0
				reader.All()(func(v uint32) bool {
					count++
					return true
				})
			}
		})
	}
}

// BenchmarkSearchPatterns tests different search access patterns.
func BenchmarkSearchPatterns(b *testing.B) {
	size := 100000
	input := make([]uint32, size)
	for i := range input {
		input[i] = uint32(i * 2)
	}

	st, err := Build(input)
	if err != nil {
		b.Fatal(err)
	}

	reader, err := NewReader(st.Data())
	if err != nil {
		b.Fatal(err)
	}

	b.Run("First", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			reader.Search(0)
		}
	})

	b.Run("Middle", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			reader.Search(uint32(size))
		}
	})

	b.Run("Last", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			reader.Search(uint32((size - 1) * 2))
		}
	})

	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := uint32((i % size) * 2)
			reader.Search(key)
		}
	})

	// Pseudo-random pattern
	keys := make([]uint32, 1000)
	for i := range keys {
		keys[i] = uint32(((i*17 + 13) % size) * 2)
	}

	b.Run("Random", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := keys[i%len(keys)]
			reader.Search(key)
		}
	})
}

// BenchmarkContains benchmarks the Contains method.
func BenchmarkContains(b *testing.B) {
	size := 10000
	input := make([]uint32, size)
	for i := range input {
		input[i] = uint32(i * 2)
	}

	st, err := Build(input)
	if err != nil {
		b.Fatal(err)
	}

	reader, err := NewReader(st.Data())
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Found", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			reader.Contains(uint32((i % size) * 2))
		}
	})

	b.Run("NotFound", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			reader.Contains(uint32((i%size)*2 + 1))
		}
	})
}

// BenchmarkSIMDvsGeneric compares SIMD and pure-Go search implementations.
func BenchmarkSIMDvsGeneric(b *testing.B) {
	if !SIMDAvailable() {
		b.Skip("SIMD not available")
	}

	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		input := make([]uint32, size)
		for i := range input {
			input[i] = uint32(i * 2)
		}

		st, err := Build(input)
		if err != nil {
			b.Fatal(err)
		}

		reader, err := NewReader(st.Data())
		if err != nil {
			b.Fatal(err)
		}

		blocks := reader.Data()[HeaderSize:]
		numBlocks := reader.NumBlocks()
		searchKey := uint32(size)

		b.Run(fmt.Sprintf("SIMD/n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				search(blocks, searchKey, numBlocks)
			}
		})

		b.Run(fmt.Sprintf("Generic/n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				searchGeneric(blocks, searchKey, numBlocks)
			}
		})
	}
}

// BenchmarkMemoryEfficiency measures memory allocations.
func BenchmarkMemoryEfficiency(b *testing.B) {
	sizes := []int{1000, 10000}

	for _, size := range sizes {
		input := make([]uint32, size)
		for i := range input {
			input[i] = uint32(i)
		}

		b.Run(fmt.Sprintf("Build/n=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = Build(input)
			}
		})

		st, _ := Build(input)
		reader, _ := NewReader(st.Data())
		key := uint32(size / 2)

		b.Run(fmt.Sprintf("Search/n=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				reader.Search(key)
			}
		})
	}
}
