# STree-Go

A static B-tree implementation in Go based on the algorithm described
at [Algorithmica - S-Tree](https://en.algorithmica.org/hpc/data-structures/s-tree/)
with SIMD acceleration (SSE4.2 and AVX2).

**This is early work and may change without warning!**

STree-Go is designed for high-performance, cache-efficient lookups. It uses Eytzinger (B-tree) numeration with 16-element blocks that align with typical 64-byte CPU cache lines, maximizing cache utilization during tree traversal. On amd64 platforms, the library automatically detects and uses SSE4.2 or AVX2 SIMD instructions at runtime for accelerated search operations, while providing a pure Go fallback that works on all platforms without SIMD support.

The data structure is designed to work directly with memory-mapped byte slices, making it ideal for persistent, disk-backed indices. Search operations allocate no memory, ensuring predictable performance without GC pressure. Once built, the tree structure is immutable, making it safe for concurrent read access.

The data structure is documented in `stree.ksy` (Kaitai Struct format).

## Usage

```go
package main

import (
    "fmt"
    "github.com/Akron/stree-go"
)

func main() {
    // Build a tree from uint32 values
    // WARNING: The input slice will be sorted and deduplicated in-place
    values := []uint32{42, 17, 100, 5, 73, 88}
    tree, err := stree.Build(values)
    if err != nil {
        panic(err)
    }

    // Create a reader from the tree data
    reader, err := stree.NewReader(tree.Data())
    if err != nil {
        panic(err)
    }

    // Search for values
    pos := reader.Search(42)
    if pos >= 0 {
        fmt.Printf("Found 42 at position %d\n", pos)
    }

    // Check if value exists
    if reader.Contains(100) {
        fmt.Println("100 exists in the tree")
    }

    // Get count of unique values
    fmt.Printf("Tree contains %d unique values\n", reader.Count())
}
```

### Using the Keyed Interface

For more advanced use cases where you need to associate data with keys:

```go
package main

import (
    "fmt"
    "github.com/Akron/stree-go"
)

// Your struct must implement the Keyed interface
type Document struct {
    ID       uint32
    Position uint32  // Will be set by BuildFromKeyed
    Title    string
}

func (d *Document) Key() uint32         { return d.ID }
func (d *Document) SetIndex(idx uint32) { d.Position = idx }
func (d *Document) Index() uint32       { return d.Position }

func main() {
    docs := []*Document{
        {ID: 1001, Title: "First Document"},
        {ID: 500, Title: "Second Document"},
        {ID: 2000, Title: "Third Document"},
    }

    // Build tree - WARNING: slice will be sorted in-place
    tree, err := stree.BuildFromKeyed(docs)
    if err != nil {
        panic(err)
    }

    // After building, each document's Position field contains its index
    for _, doc := range docs {
        fmt.Printf("Document %d (%s) is at position %d\n", 
            doc.ID, doc.Title, doc.Position)
    }

    // The tree can be serialized and used later
    data := tree.Data()
    fmt.Printf("Tree size: %d bytes\n", len(data))
}
```

### Data Integrity (CRC-32)

STree automatically computes and stores a CRC-32 checksum for data integrity validation. The checksum covers the entire file contents and is computed during tree construction.

```go
// Reader creation with automatic integrity validation
reader, err := stree.NewReaderWithValidation(data)

// Standard reader creation (no validation)
reader, err := stree.NewReader(data)

// Manual integrity check
if reader.ValidateCRC32() {
    fmt.Println("Data integrity verified")
} else {
    fmt.Println("Data corruption detected")
}
```

### Serialization and Memory Mapping

```go
package main

import (
    "os"
    "syscall"
    "github.com/Akron/stree-go"
)

func main() {
    // Build and save to file
    values := []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
    tree, _ := stree.Build(values)
    
    f, _ := os.Create("tree.bin")
    f.Write(tree.Data())
    f.Close()

    // Load via mmap for efficient read-only access
    f, _ = os.Open("tree.bin")
    stat, _ := f.Stat()
    data, _ := syscall.Mmap(int(f.Fd()), 0, int(stat.Size()),
        syscall.PROT_READ, syscall.MAP_SHARED)
    
    reader, _ := stree.NewReader(data)
    fmt.Printf("Loaded tree with %d values\n", reader.Count())
    
    syscall.Munmap(data)
    f.Close()
}
```

### Sorted Iteration

```go
// Iterate through all keys in sorted order
reader.Sorted()(func(value uint32, index int) bool {
    fmt.Printf("Value: %d, Index: %d\n", value, index)
    return true // return false to stop iteration
})
```

## Performance

Typical benchmark results on Intel Core Ultra 5 125H:

| Operation | Size | Generic | SSE4.2 | AVX2 |
|-----------|------|---------|--------|------|
| Search | 1,000 | 25 ns | 11 ns | 8 ns |
| Search | 10,000 | 16 ns | 13 ns | 11 ns |
| Search | 100,000 | 59 ns | 20 ns | 13 ns |

SIMD provides 2-4x speedup compared to pure Go implementation.

## Limitations

- **Values must be < 0x80000000 (uint31)**: All keys must be in the range [0, 2^31 - 1]. This restriction is enforced at build time and search time. `Build()` and `BuildFromKeyed()` will return `ErrValueTooLarge` if any value exceeds this limit. `Search()` returns -1 immediately for keys >= 0x80000000. This constraint ensures consistent behavior between pure Go and SIMD implementations, as SIMD comparison uses signed arithmetic.
- **In-place modification**: `Build()` and `BuildFromKeyed()` sort the input slice in place.