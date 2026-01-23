// Package stree provides an S-Tree (Static B-Tree) implementation optimized
// for cache efficiency and SIMD operations.
//
// The S-Tree uses Eytzinger layout with a branching factor of 17 (16 keys per node),
// enabling efficient cache-line-aligned traversal and SIMD-accelerated search.
//
// Reference: https://en.algorithmica.org/hpc/data-structures/s-tree/
package stree

import (
	"encoding/binary"
	"errors"
)

const (
	// BlockSize is the number of uint32 elements per block.
	// 16 elements = 64 bytes = typical CPU cache line size.
	// This enables efficient SIMD operations (SSE: 4 elements, AVX2: 8 elements).
	BlockSize = 16

	// BlockSizeBytes is the size of each block in bytes.
	BlockSizeBytes = BlockSize * 4 // 64 bytes

	// HeaderSize is the size of the file header in bytes.
	HeaderSize = 16

	// Magic bytes identifying an S-Tree file.
	Magic = "STRE"

	// Version is the current format version.
	Version uint16 = 0x0001

	// Sentinel is the value used to mark empty slots in a block.
	// Using max uint32 ensures it sorts last and is easily detectable.
	Sentinel = ^uint32(0) // 0xFFFFFFFF

	// MaxValue is the maximum allowed value for keys (2^31 - 1).
	// Values must be < 0x80000000 because SIMD comparison uses signed arithmetic.
	// This ensures consistent behavior between pure Go and SIMD implementations.
	MaxValue = uint32(0x7FFFFFFF)
)

// Errors returned by S-Tree operations.
var (
	ErrEmptyInput      = errors.New("stree: empty input")
	ErrInvalidMagic    = errors.New("stree: invalid magic bytes")
	ErrInvalidVersion  = errors.New("stree: unsupported version")
	ErrInvalidData     = errors.New("stree: invalid data")
	ErrDataTooShort    = errors.New("stree: data too short")
	ErrInvalidBlockSz  = errors.New("stree: invalid block size")
	ErrValueTooLarge   = errors.New("stree: value exceeds maximum (must be < 0x80000000)")
)

// Header represents the S-Tree file header.
type Header struct {
	Magic     [4]byte // "STRE"
	Version   uint16  // Format version
	BlockSize uint16  // Elements per block (default 16)
	Count     uint32  // Number of unique elements
	Reserved  uint32  // Reserved for future use
}

// ParseHeader parses an S-Tree header from a byte slice.
// The slice must be at least HeaderSize (16) bytes.
func ParseHeader(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, ErrDataTooShort
	}

	h := &Header{
		Version:   binary.LittleEndian.Uint16(data[4:6]),
		BlockSize: binary.LittleEndian.Uint16(data[6:8]),
		Count:     binary.LittleEndian.Uint32(data[8:12]),
		Reserved:  binary.LittleEndian.Uint32(data[12:16]),
	}
	copy(h.Magic[:], data[0:4])

	// Validate magic
	if string(h.Magic[:]) != Magic {
		return nil, ErrInvalidMagic
	}

	// Validate version
	if h.Version != Version {
		return nil, ErrInvalidVersion
	}

	// Validate block size
	if h.BlockSize == 0 || h.BlockSize != BlockSize {
		return nil, ErrInvalidBlockSz
	}

	return h, nil
}

// Bytes serializes the header to a byte slice.
func (h *Header) Bytes() []byte {
	buf := make([]byte, HeaderSize)
	copy(buf[0:4], h.Magic[:])
	binary.LittleEndian.PutUint16(buf[4:6], h.Version)
	binary.LittleEndian.PutUint16(buf[6:8], h.BlockSize)
	binary.LittleEndian.PutUint32(buf[8:12], h.Count)
	binary.LittleEndian.PutUint32(buf[12:16], h.Reserved)
	return buf
}

// NumBlocks returns the number of blocks needed to store count elements.
func NumBlocks(count int) int {
	if count <= 0 {
		return 0
	}
	return (count + BlockSize - 1) / BlockSize
}

// DataSize returns the total size in bytes needed to store count elements
// (header + data blocks).
func DataSize(count int) int {
	if count <= 0 {
		return HeaderSize
	}
	return HeaderSize + NumBlocks(count)*BlockSizeBytes
}

// childIndex calculates the index of child i for node k in Eytzinger layout.
// Formula: k * (BlockSize + 1) + i + 1 = k * 17 + i + 1
func childIndex(k, i int) int {
	return k*(BlockSize+1) + i + 1
}
