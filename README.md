# S-Tree

[![GoDoc](https://pkg.go.dev/badge/github.com/Akron/stree-go/v2?utm_source=godoc)](https://godoc.org/github.com/Akron/stree-go/v2) [![Go Report Card](https://goreportcard.com/badge/Akron/stree-go/v2)](https://goreportcard.com/report/github.com/Akron/stree-go/v2) 

A static B-tree implementation in Go based on the algorithm described
at [Algorithmica - S-Tree](https://en.algorithmica.org/hpc/data-structures/s-tree/)
with SIMD acceleration (SSE2, AVX2, and AVX-512 via Go 1.26+ experimental `archsimd`).

**This is early work and may change without warning!**

S-Tree is designed for high-performance, cache-efficient lookups. It uses Eytzinger (B-tree) numeration with 16-element blocks that align with typical 64-byte CPU cache lines, maximizing cache utilization during tree traversal. On amd64 platforms with `GOEXPERIMENT=simd`, the library automatically detects and uses SSE2, AVX2, or AVX-512 SIMD instructions at runtime for accelerated search operations, while providing a pure Go fallback that works on all platforms without SIMD support.

Keys can be any uint32 value in the range `[0, 0xFFFFFFFE]` (the value `0xFFFFFFFF` is reserved as a sentinel).

The data structure is designed to work directly with memory-mapped byte slices, making it ideal for persistent, disk-backed indices. Search operations allocate no memory, ensuring predictable performance without GC pressure. Once built, the tree structure is immutable, making it safe for concurrent read access.

The data structure is documented in `stree.ksy` as a [Kaitai Struct format](https://kaitai.io/).

## Usage

```go
package main

import (
	"fmt"
	"os"

	stree "github.com/Akron/stree-go/v2"
)

func main() {
	// 1. Build the S-Tree
	
	// Input: unsorted uint32 values with duplicates.
	values := []uint32{42, 17, 100, 5, 73, 88, 42, 17}
	
	tree, err := stree.Build(values)
	if err != nil {
		panic(err)
	}
	
	// 2. Serialize to disk
	file, err := os.Create("index.stree")
	if err != nil {
		panic(err)
	}
	
	_, err = tree.WriteTo(file)
	if err != nil {
		file.Close()
		panic(err)
	}
	file.Close()
	
	// 3. Load the S-Tree
	data, err := os.ReadFile("index.stree")
	if err != nil {
		panic(err)
	}
	
	reader, err := stree.NewReaderWithValidation(data)
	if err != nil {
		panic(err)
	}

    // 4. Search for keys
	// returns the position in the tree, or -1 if not found.
	// The position can be used to correlate
	// with external data.
	keysToFind := []uint32{42, 100, 999}
	for _, key := range keysToFind {
		pos := reader.Search(key)
		if pos >= 0 {
			fmt.Printf("Found %d at position %d\n", key, pos)
		} else {
			fmt.Printf("Key %d not found\n", key)
		}
	}
}
```

## Building with SIMD

SIMD acceleration requires Go 1.26+ and (for the moment) `GOEXPERIMENT=simd`:

```shell
# Build with SIMD support
GOEXPERIMENT=simd go build ./...

# Test with SIMD
GOEXPERIMENT=simd go test -v ./...

# Test without SIMD (pure Go fallback)
go test -v ./...

# Benchmarks with SIMD
GOEXPERIMENT=simd go test -bench=BenchmarkSearchImplementations -benchmem ./...
```

## Performance

Benchmark results on Intel Core Ultra 5 125H (`GOEXPERIMENT=simd`):

| Operation | Size      | Generic | SSE2    | AVX2    |
|-----------|-----------|---------|---------|---------|
| Search    | 1,000     | 16.4 ns | 8.9 ns  | 7.2 ns  |
| Search    | 10,000    | 11.8 ns | 9.2 ns  | 10.3 ns |
| Search    | 100,000   | 36.1 ns | 16.2 ns | 12.4 ns |
| Search    | 1,000,000 | 36.4 ns | 17.5 ns | 13.4 ns |

## Limitations

- The value `0xFFFFFFFF` is reserved as a sentinel. `Build()` and `BuildFromKeyed()` return `ErrValueTooLarge` for sentinel values. `Search()` returns -1 for `0xFFFFFFFF`.
- In-place modification: `Build()` and `BuildFromKeyed()` sort the input slice in place.

## Disclaimer

This library was developed with AI assistance (Claude Opus 4.5 + 4.6).

## Copyright

Copyright (c) 2025-2026 Nils Diewald
