//go:build !amd64 || noasm

package stree

func init() {
	initSIMDSelection()
}

func initSIMDSelection() {
	// No SIMD available on this platform
	// search remains as searchGeneric (set in search_generic.go)
}
