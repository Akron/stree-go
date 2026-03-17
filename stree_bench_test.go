package stree

import (
	"fmt"
	"testing"
)

var (
	benchPos int
)

// benchmarkTree holds prebuilt benchmark fixtures for search tests.
type benchmarkTree struct {
	blocks    []byte
	numBlocks int
}

// buildBenchmarkTree constructs a reader fixture for benchmark size.
func buildBenchmarkTree(b *testing.B, size int) benchmarkTree {
	b.Helper()
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
	return benchmarkTree{
		blocks:    reader.Data()[headerSize:],
		numBlocks: reader.NumBlocks(),
	}
}

// runSearchBench runs one search implementation on deterministic keys.
func runSearchBench(
	b *testing.B,
	blocks []byte,
	numBlocks int,
	keys []uint32,
	searchFn func([]byte, uint32, int) int,
) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchPos = searchFn(blocks, keys[i%len(keys)], numBlocks)
	}
}

type keyPattern struct {
	name string
	keys []uint32
}

// buildKeyPatterns creates deterministic key sets for stable benchmarks.
func buildKeyPatterns(size int) []keyPattern {
	patterns := []keyPattern{
		{name: "Hit", keys: []uint32{uint32(size)}},
		{name: "Miss", keys: []uint32{uint32(size + 1)}},
	}

	sequentialHits := make([]uint32, 512)
	alternating := make([]uint32, 512)
	for i := range sequentialHits {
		v := uint32((i % size) * 2)
		sequentialHits[i] = v
		if i%2 == 0 {
			alternating[i] = v
		} else {
			alternating[i] = v + 1
		}
	}
	patterns = append(patterns,
		keyPattern{name: "SequentialHit", keys: sequentialHits},
		keyPattern{name: "AlternatingHitMiss", keys: alternating},
	)

	return patterns
}

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
			blocks := reader.Data()[headerSize:]
			numBlocks := reader.NumBlocks()
			for i := 0; i < b.N; i++ {
				searchGeneric(blocks, searchKey, numBlocks)
			}
		})

		b.Run(fmt.Sprintf("Simple/n=%d", size), func(b *testing.B) {
			blocks := reader.Data()[headerSize:]
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

		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				count := 0
				reader.Sorted()(func(v uint32, _ int) bool {
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

// BenchmarkSearchImplementations compares Generic, SSE2, AVX2, and AVX-512 search.
func BenchmarkSearchImplementations(b *testing.B) {
	sizes := []int{1000, 4000, 8000, 10000, 12000, 16000, 32000, 100000, 500000, 1000000}

	type impl struct {
		name  string
		avail func() bool
		fn    func([]byte, uint32, int) int
	}
	implementations := []impl{
		{name: "Generic", avail: func() bool { return true }, fn: SearchGeneric},
		{name: "SSE2", avail: HasSSE2, fn: SearchSSE2},
		{name: "AVX2", avail: HasAVX2, fn: SearchAVX2},
		{name: "AVX512", avail: HasAVX512, fn: SearchAVX512},
	}

	for _, size := range sizes {
		tree := buildBenchmarkTree(b, size)
		patterns := buildKeyPatterns(size)

		for _, implementation := range implementations {
			if !implementation.avail() {
				continue
			}
			for _, pattern := range patterns {
				benchName := fmt.Sprintf("%s/%s/n=%d", implementation.name, pattern.name, size)
				b.Run(benchName, func(b *testing.B) {
					runSearchBench(b, tree.blocks, tree.numBlocks, pattern.keys, implementation.fn)
				})
			}
		}
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

// BenchmarkCRC32 benchmarks CRC-32 computation and validation performance.
func BenchmarkCRC32(b *testing.B) {
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

		data := st.Data()

		b.Run(fmt.Sprintf("Compute/n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				computeCRC32(data)
			}
		})

		reader, err := NewReader(data)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("Validate/n=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				reader.ValidateCRC32()
			}
		})
	}
}
