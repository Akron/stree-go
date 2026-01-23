//go:build amd64 && !noasm

//go:generate go run ./internal/asm/search.go

package stree

import "golang.org/x/sys/cpu"

var (
	hasSSE42 bool
	hasAVX2  bool
)

func init() {
	initSIMDSelection()
}

func initSIMDSelection() {
	hasSSE42 = cpu.X86.HasSSE42
	hasAVX2 = cpu.X86.HasAVX2

	if hasAVX2 {
		search = searchAVX2
		simdAvailable = true
	} else if hasSSE42 {
		search = searchSSE
		simdAvailable = true
	}
	// Otherwise, search remains as searchGeneric (set in search_generic.go)
}

// HasSSE42 returns true if SSE4.2 is available.
func HasSSE42() bool { return hasSSE42 }

// HasAVX2 returns true if AVX2 is available.
func HasAVX2() bool { return hasAVX2 }

// searchAVX2 searches for a key in S-Tree blocks using AVX2 SIMD.
// Returns the position (block*16 + offset) if found, -1 otherwise.
//
//go:noescape
func searchAVX2(blocks []byte, key uint32, numBlocks int) int

// searchSSE searches for a key in S-Tree blocks using SSE4.2 SIMD.
// Returns the position (block*16 + offset) if found, -1 otherwise.
//
//go:noescape
func searchSSE(blocks []byte, key uint32, numBlocks int) int

// Exported for benchmarking purposes
var (
	SearchAVX2    = searchAVX2
	SearchSSE     = searchSSE
	SearchGeneric = searchGeneric
)
