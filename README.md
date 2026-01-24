# S-Tree

A static B-tree implementation in Go based on the algorithm described
at [Algorithmica - S-Tree](https://en.algorithmica.org/hpc/data-structures/s-tree/)
with SIMD acceleration (SSE4.2 and AVX2).

**This is early work and may change without warning!**

STree-Go is designed for high-performance, cache-efficient lookups. It uses Eytzinger (B-tree) numeration with 16-element blocks that align with typical 64-byte CPU cache lines, maximizing cache utilization during tree traversal. On amd64 platforms, the library automatically detects and uses SSE4.2 or AVX2 SIMD instructions at runtime for accelerated search operations, while providing a pure Go fallback that works on all platforms without SIMD support.

The data structure is designed to work directly with memory-mapped byte slices, making it ideal for persistent, disk-backed indices. Search operations allocate no memory, ensuring predictable performance without GC pressure. Once built, the tree structure is immutable, making it safe for concurrent read access.

The data structure is documented in `stree.ksy` as a [Kaitai Struct format](https://kaitai.io/).

## Usage

```go
package main

import (
	"fmt"
	"os"

	"github.com/Akron/stree-go"
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

## Development

Assembly is generated using [avo](https://github.com/mmcloughlin/avo). Regeneration is done using

```shell
$ go generate ./internal/asm
```

## Performance

Typical benchmark results on Intel Core Ultra 5 125H:

| Operation | Size    | Generic | SSE4.2 | AVX2  |
|-----------|---------|---------|--------|-------|
| Search    | 1,000   | 25 ns   | 11 ns  | 8 ns  |
| Search    | 10,000  | 16 ns   | 13 ns  | 11 ns |
| Search    | 100,000 | 59 ns   | 20 ns  | 13 ns |

## Limitations

- Values must be (uint31): All keys must be in the range [0, 2^31 - 1]. This restriction is enforced at build time and search time. `Build()` and `BuildFromKeyed()` will return `ErrValueTooLarge` if any value exceeds this limit. `Search()` returns -1 immediately for keys >= 0x80000000. This constraint ensures consistent behavior between pure Go and SIMD implementations, as SIMD comparison uses signed arithmetic.
- In-place modification: `Build()` and `BuildFromKeyed()` sort the input slice in place.

## Disclaimer

This library was developed with AI assistance (Claude Opus 4.5).

## Copyright

Copyright (c) 2025-2026 Nils Diewald
