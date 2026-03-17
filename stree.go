// Package stree provides an S-Tree (Static B-Tree) implementation optimized
// for cache efficiency and SIMD operations.
//
// The S-Tree uses Eytzinger layout with a branching factor of 17 (16 keys per node),
// enabling efficient cache-line-aligned traversal and SIMD-accelerated search.
//
// Reference: https://en.algorithmica.org/hpc/data-structures/s-tree/
package stree

import (
	"errors"
	"hash/crc32"
)

const (
	// blockSize is the number of uint32 elements per block.
	// 16 elements = 64 bytes = typical CPU cache line size.
	// This enables efficient SIMD operations (SSE: 4 elements, AVX2: 8 elements).
	blockSize = 16

	// blockSizeBytes is the size of each block in bytes.
	blockSizeBytes = blockSize * 4 // 64 bytes

	// headerSize is the size of the file header in bytes.
	headerSize = 16

	// Magic bytes identifying an S-Tree file.
	Magic = "STRE"

	// Version is the current format version.
	Version uint16 = 0x0002

	// versionV1 is the legacy v1 format version, accepted for backward compatibility.
	versionV1 uint16 = 0x0001

	// sentinel is the value used to mark empty slots in a block.
	// Using max uint32 ensures it sorts last and is easily detectable.
	sentinel = ^uint32(0) // 0xFFFFFFFF

	// MaxValue is the maximum allowed value for keys.
	// All uint32 values except the sentinel (0xFFFFFFFF) are valid.
	MaxValue = uint32(0xFFFFFFFE)
)

// Errors returned by S-Tree operations.
var (
	ErrEmptyInput     = errors.New("stree: empty input")
	ErrInvalidMagic   = errors.New("stree: invalid magic bytes")
	ErrInvalidVersion = errors.New("stree: unsupported version")
	ErrInvalidData    = errors.New("stree: invalid data")
	ErrDataTooShort   = errors.New("stree: data too short")
	ErrInvalidBlockSz = errors.New("stree: invalid block size")
	ErrValueTooLarge  = errors.New("stree: value exceeds maximum (must be < 0xFFFFFFFF)")
)

// header represents the S-Tree file header.
type header struct {
	magic     [4]byte // "STRE"
	version   uint16  // Format version
	blockSize uint16  // Elements per block (default 16)
	count     uint32  // Number of unique elements
	crc32     uint32  // CRC-32 checksum of header + data blocks
}

// parseHeader parses an S-Tree header from a byte slice.
// The slice must be at least HeaderSize (16) bytes.
func parseHeader(data []byte) (*header, error) {
	if len(data) < headerSize {
		return nil, ErrDataTooShort
	}

	h := &header{
		version:   be.Uint16(data[4:6]),
		blockSize: be.Uint16(data[6:8]),
		count:     be.Uint32(data[8:12]),
		crc32:     be.Uint32(data[12:16]),
	}
	copy(h.magic[:], data[0:4])

	// Validate magic
	if string(h.magic[:]) != Magic {
		return nil, ErrInvalidMagic
	}

	// Validate version (accept both v1 and v2)
	if h.version != Version && h.version != versionV1 {
		return nil, ErrInvalidVersion
	}

	// Validate block size
	if h.blockSize == 0 || h.blockSize != blockSize {
		return nil, ErrInvalidBlockSz
	}

	return h, nil
}

// bytes serializes the header to a byte slice.
func (h *header) bytes() []byte {
	buf := make([]byte, headerSize)
	copy(buf[0:4], h.magic[:])
	be.PutUint16(buf[4:6], h.version)
	be.PutUint16(buf[6:8], h.blockSize)
	be.PutUint32(buf[8:12], h.count)
	be.PutUint32(buf[12:16], h.crc32)
	return buf
}

// numBlocks returns the number of blocks needed to store count elements.
func numBlocks(count int) int {
	if count <= 0 {
		return 0
	}
	return (count + blockSize - 1) / blockSize
}

// DataSize returns the total size in bytes needed to store count elements
// (header + data blocks).
func DataSize(count int) int {
	if count <= 0 {
		return headerSize
	}
	return headerSize + numBlocks(count)*blockSizeBytes
}

// childIndex calculates the index of child i for node k in Eytzinger layout.
// Formula: k * (BlockSize + 1) + i + 1 = k * 17 + i + 1
func childIndex(k, i int) int {
	return k*(blockSize+1) + i + 1
}

// computeCRC32 calculates CRC-32 checksum of the entire S-Tree data (header + blocks).
// The CRC is computed with the CRC32 field in the header temporarily set to 0.
func computeCRC32(data []byte) uint32 {
	// Create a copy of the header with CRC32 field zeroed for computation
	headerData := make([]byte, headerSize)
	copy(headerData, data[:headerSize])
	// Zero the CRC32 field (bytes 12-16) for consistent computation
	headerData[12] = 0
	headerData[13] = 0
	headerData[14] = 0
	headerData[15] = 0

	// Compute CRC-32 over header + data blocks
	hasher := crc32.NewIEEE()
	hasher.Write(headerData)
	hasher.Write(data[headerSize:])
	return hasher.Sum32()
}

// validateCRC32 checks if the stored CRC-32 matches the computed CRC-32.
func validateCRC32(data []byte) bool {
	if len(data) < headerSize {
		return false
	}

	storedCRC := be.Uint32(data[12:16])
	computedCRC := computeCRC32(data)

	return storedCRC == computedCRC
}
