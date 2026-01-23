//go:build !amd64 || noasm

package stree

func init() {
	initSIMDSelection()
}

func initSIMDSelection() {
	// No SIMD available on this platform
	// search remains as searchGeneric (set in search_generic.go)
}

// HasSSE42 returns true if SSE4.2 is available (always false on non-amd64).
func HasSSE42() bool { return false }

// HasAVX2 returns true if AVX2 is available (always false on non-amd64).
func HasAVX2() bool { return false }

// Stubs for non-SIMD platforms - fall back to generic
var (
	SearchAVX2    = searchGeneric
	SearchSSE     = searchGeneric
	SearchGeneric = searchGeneric
)
