//go:build amd64 && !noasm

//go:generate go run ./internal/asm/search.go

package stree

import "golang.org/x/sys/cpu"

func init() {
	initSIMDSelection()
}

func initSIMDSelection() {
	if cpu.X86.HasAVX2 {
		search = searchAVX2
		simdAvailable = true
	} else if cpu.X86.HasSSE42 {
		search = searchSSE
		simdAvailable = true
	}
	// Otherwise, search remains as searchGeneric (set in search_generic.go)
}

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
