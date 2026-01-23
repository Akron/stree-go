//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/operand"
	. "github.com/mmcloughlin/avo/operand"
)

const (
	BlockSize      = 16 // Number of uint32 values per block
	BlockSizeBytes = 64 // 16 * 4 bytes
)

func main() {
	genSearchAVX2()
	genSearchSSE()
	Generate()
}

// genSearchAVX2 generates AVX2-optimized S-Tree search.
// This implements the full tree traversal to minimize Go-assembly call overhead.
func genSearchAVX2() {
	TEXT("searchAVX2", NOSPLIT, "func(blocks []byte, key uint32, numBlocks int) int")
	Doc("searchAVX2 searches for a key in S-Tree blocks using AVX2 SIMD.",
		"Returns the position (block*16 + offset) if found, -1 otherwise.")

	blocks := Mem{Base: Load(Param("blocks").Base(), GP64())}
	key := Load(Param("key"), GP32())
	numBlocks := Load(Param("numBlocks"), GP64())

	// Check for empty tree
	TESTQ(numBlocks, numBlocks)
	JZ(LabelRef("notFound"))

	// Broadcast search key to all 8 lanes of YMM register
	keyBroadcast := YMM()
	VMOVD(key, keyBroadcast.AsX())
	VPBROADCASTD(keyBroadcast.AsX(), keyBroadcast)

	// Sentinel value broadcast (0xFFFFFFFF)
	sentinel := GP64()
	MOVQ(U64(0xFFFFFFFF), sentinel)
	sentinelBroadcast := YMM()
	VMOVD(sentinel.As32(), sentinelBroadcast.AsX())
	VPBROADCASTD(sentinelBroadcast.AsX(), sentinelBroadcast)

	// Tree traversal state
	k := GP64()       // Current block index
	XORQ(k, k)        // k = 0 (start at root)

	Label("traversalLoop")
	// Check if k < numBlocks
	CMPQ(k, numBlocks)
	JAE(LabelRef("notFound"))

	// Calculate block address: blocks.Base + k * 64
	blockAddr := GP64()
	MOVQ(k, blockAddr)
	SHLQ(Imm(6), blockAddr) // k * 64
	ADDQ(blocks.Base, blockAddr)

	// Load first 8 uint32 values (32 bytes)
	first8 := YMM()
	VMOVDQU(Mem{Base: blockAddr}, first8)

	// Check for exact match in first half
	matchMask := YMM()
	VPCMPEQD(keyBroadcast, first8, matchMask)
	matchBits := GP32()
	VMOVMSKPS(matchMask, matchBits)
	TESTL(matchBits, matchBits)
	JNZ(LabelRef("foundInFirstHalf"))

	// Load second 8 uint32 values (next 32 bytes)
	second8 := YMM()
	VMOVDQU(Mem{Base: blockAddr, Disp: 32}, second8)

	// Check for exact match in second half
	VPCMPEQD(keyBroadcast, second8, matchMask)
	VMOVMSKPS(matchMask, matchBits)
	TESTL(matchBits, matchBits)
	JNZ(LabelRef("foundInSecondHalf"))

	// No exact match, find child index for tree traversal
	// childIndex = number of keys < search key (or first sentinel position)
	childIdx := GP64()
	MOVQ(U64(16), childIdx) // Default: rightmost child

	// Count values greater than key in first half
	gtMask := YMM()
	gtBits := GP32()
	VPCMPGTD(keyBroadcast, first8, gtMask) // gtMask = (values > key)
	VMOVMSKPS(gtMask, gtBits)
	TESTL(gtBits, gtBits)
	JNZ(LabelRef("greaterInFirstHalf"))

	// Check for sentinels in first half
	sentMask := YMM()
	sentBits := GP32()
	VPCMPEQD(sentinelBroadcast, first8, sentMask)
	VMOVMSKPS(sentMask, sentBits)
	TESTL(sentBits, sentBits)
	JNZ(LabelRef("sentinelInFirstHalf"))

	// Check second half for greater values
	VPCMPGTD(keyBroadcast, second8, gtMask)
	VMOVMSKPS(gtMask, gtBits)
	TESTL(gtBits, gtBits)
	JNZ(LabelRef("greaterInSecondHalf"))

	// Check for sentinels in second half
	VPCMPEQD(sentinelBroadcast, second8, sentMask)
	VMOVMSKPS(sentMask, sentBits)
	TESTL(sentBits, sentBits)
	JNZ(LabelRef("sentinelInSecondHalf"))

	// No greater value or sentinel, use rightmost child (16)
	JMP(LabelRef("calculateNext"))

	Label("greaterInFirstHalf")
	BSFL(gtBits, childIdx.As32())
	MOVL(childIdx.As32(), childIdx.As32()) // Zero-extend
	JMP(LabelRef("calculateNext"))

	Label("greaterInSecondHalf")
	BSFL(gtBits, childIdx.As32())
	ADDQ(Imm(8), childIdx)
	JMP(LabelRef("calculateNext"))

	Label("sentinelInFirstHalf")
	BSFL(sentBits, childIdx.As32())
	MOVL(childIdx.As32(), childIdx.As32()) // Zero-extend
	JMP(LabelRef("calculateNext"))

	Label("sentinelInSecondHalf")
	BSFL(sentBits, childIdx.As32())
	ADDQ(Imm(8), childIdx)

	Label("calculateNext")
	// k = k * 17 + childIdx + 1
	temp := GP64()
	IMUL3Q(Imm(17), k, temp)
	ADDQ(childIdx, temp)
	INCQ(temp)
	MOVQ(temp, k)
	JMP(LabelRef("traversalLoop"))

	Label("foundInFirstHalf")
	// Return k * 16 + (first set bit in matchBits)
	lane := GP32()
	BSFL(matchBits, lane)
	result := GP64()
	MOVQ(k, result)
	SHLQ(Imm(4), result) // k * 16
	MOVL(lane, lane)     // Zero-extend to 64-bit via implicit zeroing
	temp2 := GP64()
	MOVL(lane, temp2.As32())
	ADDQ(temp2, result)
	VZEROUPPER()
	Store(result, ReturnIndex(0))
	RET()

	Label("foundInSecondHalf")
	// Return k * 16 + 8 + (first set bit in matchBits)
	BSFL(matchBits, lane)
	MOVQ(k, result)
	SHLQ(Imm(4), result) // k * 16
	MOVL(lane, temp2.As32())
	ADDQ(temp2, result)
	ADDQ(Imm(8), result)
	VZEROUPPER()
	Store(result, ReturnIndex(0))
	RET()

	Label("notFound")
	VZEROUPPER()
	result = GP64()
	MOVQ(I64(-1), result)
	Store(result, ReturnIndex(0))
	RET()
}

// genSearchSSE generates SSE4.2-optimized S-Tree search.
// Processes 4 uint32 values at a time instead of 8.
func genSearchSSE() {
	TEXT("searchSSE", NOSPLIT, "func(blocks []byte, key uint32, numBlocks int) int")
	Doc("searchSSE searches for a key in S-Tree blocks using SSE4.2 SIMD.",
		"Returns the position (block*16 + offset) if found, -1 otherwise.")

	blocks := Mem{Base: Load(Param("blocks").Base(), GP64())}
	key := Load(Param("key"), GP32())
	numBlocks := Load(Param("numBlocks"), GP64())

	// Check for empty tree
	TESTQ(numBlocks, numBlocks)
	JZ(LabelRef("sseNotFound"))

	// Broadcast search key to all 4 lanes of XMM register
	keyXMM := XMM()
	MOVD(key, keyXMM)
	PSHUFD(Imm(0), keyXMM, keyXMM) // Broadcast to all 4 lanes

	// Sentinel value broadcast
	sentinel := GP64()
	MOVQ(U64(0xFFFFFFFF), sentinel)
	sentinelXMM := XMM()
	MOVD(sentinel.As32(), sentinelXMM)
	PSHUFD(Imm(0), sentinelXMM, sentinelXMM)

	// Tree traversal state
	k := GP64()
	XORQ(k, k) // k = 0

	Label("sseTraversalLoop")
	CMPQ(k, numBlocks)
	JAE(LabelRef("sseNotFound"))

	// Calculate block address
	blockAddr := GP64()
	MOVQ(k, blockAddr)
	SHLQ(Imm(6), blockAddr)
	ADDQ(blocks.Base, blockAddr)

	// Process 16 values in 4 chunks of 4
	// Chunk 0: bytes 0-15 (values 0-3)
	chunk0 := XMM()
	MOVOU(Mem{Base: blockAddr}, chunk0)
	matchXMM := XMM()
	MOVO(keyXMM, matchXMM)
	PCMPEQL(chunk0, matchXMM)
	matchBits := GP32()
	PMOVMSKB(matchXMM, matchBits)
	TESTL(matchBits, matchBits)
	JNZ(LabelRef("sseFoundInChunk0"))

	// Chunk 1: bytes 16-31 (values 4-7)
	chunk1 := XMM()
	MOVOU(Mem{Base: blockAddr, Disp: 16}, chunk1)
	MOVO(keyXMM, matchXMM)
	PCMPEQL(chunk1, matchXMM)
	PMOVMSKB(matchXMM, matchBits)
	TESTL(matchBits, matchBits)
	JNZ(LabelRef("sseFoundInChunk1"))

	// Chunk 2: bytes 32-47 (values 8-11)
	chunk2 := XMM()
	MOVOU(Mem{Base: blockAddr, Disp: 32}, chunk2)
	MOVO(keyXMM, matchXMM)
	PCMPEQL(chunk2, matchXMM)
	PMOVMSKB(matchXMM, matchBits)
	TESTL(matchBits, matchBits)
	JNZ(LabelRef("sseFoundInChunk2"))

	// Chunk 3: bytes 48-63 (values 12-15)
	chunk3 := XMM()
	MOVOU(Mem{Base: blockAddr, Disp: 48}, chunk3)
	MOVO(keyXMM, matchXMM)
	PCMPEQL(chunk3, matchXMM)
	PMOVMSKB(matchXMM, matchBits)
	TESTL(matchBits, matchBits)
	JNZ(LabelRef("sseFoundInChunk3"))

	// No match found, determine child index
	childIdx := GP64()
	MOVQ(U64(16), childIdx) // Default: rightmost

	// Find first value > key in chunk 0
	gtXMM := XMM()
	MOVO(chunk0, gtXMM)
	PCMPGTL(keyXMM, gtXMM)
	gtBits := GP32()
	PMOVMSKB(gtXMM, gtBits)
	TESTL(gtBits, gtBits)
	JNZ(LabelRef("sseGtChunk0"))

	// Check sentinels in chunk 0
	sentXMM := XMM()
	MOVO(sentinelXMM, sentXMM)
	PCMPEQL(chunk0, sentXMM)
	sentBits := GP32()
	PMOVMSKB(sentXMM, sentBits)
	TESTL(sentBits, sentBits)
	JNZ(LabelRef("sseSentChunk0"))

	// Chunk 1
	MOVO(chunk1, gtXMM)
	PCMPGTL(keyXMM, gtXMM)
	PMOVMSKB(gtXMM, gtBits)
	TESTL(gtBits, gtBits)
	JNZ(LabelRef("sseGtChunk1"))

	MOVO(sentinelXMM, sentXMM)
	PCMPEQL(chunk1, sentXMM)
	PMOVMSKB(sentXMM, sentBits)
	TESTL(sentBits, sentBits)
	JNZ(LabelRef("sseSentChunk1"))

	// Chunk 2
	MOVO(chunk2, gtXMM)
	PCMPGTL(keyXMM, gtXMM)
	PMOVMSKB(gtXMM, gtBits)
	TESTL(gtBits, gtBits)
	JNZ(LabelRef("sseGtChunk2"))

	MOVO(sentinelXMM, sentXMM)
	PCMPEQL(chunk2, sentXMM)
	PMOVMSKB(sentXMM, sentBits)
	TESTL(sentBits, sentBits)
	JNZ(LabelRef("sseSentChunk2"))

	// Chunk 3
	MOVO(chunk3, gtXMM)
	PCMPGTL(keyXMM, gtXMM)
	PMOVMSKB(gtXMM, gtBits)
	TESTL(gtBits, gtBits)
	JNZ(LabelRef("sseGtChunk3"))

	MOVO(sentinelXMM, sentXMM)
	PCMPEQL(chunk3, sentXMM)
	PMOVMSKB(sentXMM, sentBits)
	TESTL(sentBits, sentBits)
	JNZ(LabelRef("sseSentChunk3"))

	JMP(LabelRef("sseCalculateNext"))

	// Greater than handlers - PMOVMSKB gives byte mask, need to convert to value index
	// Each uint32 is 4 bytes, so we BSF and divide by 4
	Label("sseGtChunk0")
	BSFL(gtBits, childIdx.As32())
	SHRQ(Imm(2), childIdx) // Divide by 4 to get value index
	JMP(LabelRef("sseCalculateNext"))

	Label("sseGtChunk1")
	BSFL(gtBits, childIdx.As32())
	SHRQ(Imm(2), childIdx)
	ADDQ(Imm(4), childIdx)
	JMP(LabelRef("sseCalculateNext"))

	Label("sseGtChunk2")
	BSFL(gtBits, childIdx.As32())
	SHRQ(Imm(2), childIdx)
	ADDQ(Imm(8), childIdx)
	JMP(LabelRef("sseCalculateNext"))

	Label("sseGtChunk3")
	BSFL(gtBits, childIdx.As32())
	SHRQ(Imm(2), childIdx)
	ADDQ(Imm(12), childIdx)
	JMP(LabelRef("sseCalculateNext"))

	// Sentinel handlers
	Label("sseSentChunk0")
	BSFL(sentBits, childIdx.As32())
	SHRQ(Imm(2), childIdx)
	JMP(LabelRef("sseCalculateNext"))

	Label("sseSentChunk1")
	BSFL(sentBits, childIdx.As32())
	SHRQ(Imm(2), childIdx)
	ADDQ(Imm(4), childIdx)
	JMP(LabelRef("sseCalculateNext"))

	Label("sseSentChunk2")
	BSFL(sentBits, childIdx.As32())
	SHRQ(Imm(2), childIdx)
	ADDQ(Imm(8), childIdx)
	JMP(LabelRef("sseCalculateNext"))

	Label("sseSentChunk3")
	BSFL(sentBits, childIdx.As32())
	SHRQ(Imm(2), childIdx)
	ADDQ(Imm(12), childIdx)

	Label("sseCalculateNext")
	// k = k * 17 + childIdx + 1
	temp := GP64()
	IMUL3Q(Imm(17), k, temp)
	ADDQ(childIdx, temp)
	INCQ(temp)
	MOVQ(temp, k)
	JMP(LabelRef("sseTraversalLoop"))

	// Found handlers - convert byte mask to value index
	Label("sseFoundInChunk0")
	lane := GP32()
	BSFL(matchBits, lane)
	SHRQ(Imm(2), lane.As64()) // Convert byte index to value index
	result := GP64()
	MOVQ(k, result)
	SHLQ(Imm(4), result)
	temp2 := GP64()
	MOVL(lane, temp2.As32())
	ADDQ(temp2, result)
	Store(result, ReturnIndex(0))
	RET()

	Label("sseFoundInChunk1")
	BSFL(matchBits, lane)
	SHRQ(Imm(2), lane.As64())
	MOVQ(k, result)
	SHLQ(Imm(4), result)
	MOVL(lane, temp2.As32())
	ADDQ(temp2, result)
	ADDQ(Imm(4), result)
	Store(result, ReturnIndex(0))
	RET()

	Label("sseFoundInChunk2")
	BSFL(matchBits, lane)
	SHRQ(Imm(2), lane.As64())
	MOVQ(k, result)
	SHLQ(Imm(4), result)
	MOVL(lane, temp2.As32())
	ADDQ(temp2, result)
	ADDQ(Imm(8), result)
	Store(result, ReturnIndex(0))
	RET()

	Label("sseFoundInChunk3")
	BSFL(matchBits, lane)
	SHRQ(Imm(2), lane.As64())
	MOVQ(k, result)
	SHLQ(Imm(4), result)
	MOVL(lane, temp2.As32())
	ADDQ(temp2, result)
	ADDQ(Imm(12), result)
	Store(result, ReturnIndex(0))
	RET()

	Label("sseNotFound")
	result = GP64()
	MOVQ(operand.I64(-1), result)
	Store(result, ReturnIndex(0))
	RET()
}

