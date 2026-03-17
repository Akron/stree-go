//go:build !goexperiment.simd || !amd64

package stree

// HasSSE2 returns true if SSE2 SIMD search is available.
func HasSSE2() bool { return false }

// HasAVX2 returns true if AVX2 SIMD search is available.
func HasAVX2() bool { return false }

// HasAVX512 returns true if AVX-512 SIMD search is available.
func HasAVX512() bool { return false }

// Exported search function variables for benchmarking.
var (
	SearchSSE2    = searchGeneric
	SearchAVX2    = searchGeneric
	SearchAVX512  = searchGeneric
	SearchGeneric = searchGeneric
)
