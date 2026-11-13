//go:build goexperiment.simd && amd64

package stree

import (
	"reflect"
	"runtime"
	"testing"
)

// TestSearchDispatch checks that init() picked the kernel that matches the
// CPU features (or the GODEBUG=cpu.<x>=off simulation). Run with -v to see
// which kernel was dispatched, e.g. on a real AVX-only machine:
//
//	./stree.test -test.v -test.run TestSearchDispatch
func TestSearchDispatch(t *testing.T) {
	hasAVX, hasAVX2, hasAVX512 := simdLevel()

	var want string
	switch {
	case hasAVX512:
		want = "searchAVX512"
	case hasAVX2:
		want = "searchAVX2"
	case hasAVX:
		want = "searchSSE2"
	default:
		want = "searchGeneric"
	}

	got := runtime.FuncForPC(reflect.ValueOf(search).Pointer()).Name()
	t.Logf("cpu: avx=%v avx2=%v avx512=%v; SIMDAvailable=%v; dispatched kernel: %s",
		hasAVX, hasAVX2, hasAVX512, SIMDAvailable(), got)

	if reflect.ValueOf(search).Pointer() != kernelPointer(want) {
		t.Fatalf("dispatched kernel is %s, want %s", got, want)
	}
	if SIMDAvailable() != hasAVX {
		t.Fatalf("SIMDAvailable() = %v, want %v", SIMDAvailable(), hasAVX)
	}
}

func kernelPointer(name string) uintptr {
	switch name {
	case "searchAVX512":
		return reflect.ValueOf(searchAVX512).Pointer()
	case "searchAVX2":
		return reflect.ValueOf(searchAVX2).Pointer()
	case "searchSSE2":
		return reflect.ValueOf(searchSSE2).Pointer()
	default:
		return reflect.ValueOf(searchGeneric).Pointer()
	}
}
