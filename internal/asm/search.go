//go:build avogen
// +build avogen

package main

import (
	"fmt"

	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/operand"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
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
	k := GP64() // Current block index
	XORQ(k, k)  // k = 0 (start at root)

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
	// In Go asm (Plan 9): VPCMPGTD src2, src1, dest computes dest = (src1 > src2)
	// In avo: VPCMPGTD(a, b, c) generates "VPCMPGTD a, b, c" → c = (b > a)
	// We want: gtMask = (first8 > keyBroadcast) = (values > key)
	// So we call: VPCMPGTD(keyBroadcast, first8, gtMask)
	gtMask := YMM()
	gtBits := GP32()
	VPCMPGTD(keyBroadcast, first8, gtMask) // gtMask = (first8 > keyBroadcast) = (values > key)
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
	VPCMPGTD(keyBroadcast, second8, gtMask) // gtMask = (second8 > keyBroadcast) = (values > key)
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

	// Load all 4 chunks and check for matches using a loop
	chunks := make([]reg.VecVirtual, 4)
	matchXMM := XMM()
	matchBits := GP32()

	for i := range 4 {
		chunks[i] = XMM()
		MOVOU(Mem{Base: blockAddr, Disp: i * 16}, chunks[i])
		MOVO(keyXMM, matchXMM)
		PCMPEQL(chunks[i], matchXMM)
		PMOVMSKB(matchXMM, matchBits)
		TESTL(matchBits, matchBits)
		JNZ(LabelRef(fmt.Sprintf("sseFoundInChunk%d", i)))
	}

	// No match found, determine child index
	childIdx := GP64()
	MOVQ(U64(16), childIdx) // Default: rightmost

	// Check for greater values and sentinels in each chunk
	// PCMPGTL computes dest = (dest > src), so after MOVO(chunks[i], gtXMM),
	// PCMPGTL(keyXMM, gtXMM) computes gtXMM = (gtXMM > keyXMM) = (values > key)
	gtXMM := XMM()
	gtBits := GP32()
	sentXMM := XMM()
	sentBits := GP32()

	for i := range 4 {
		// Check for value > key: gtXMM = chunks[i], then gtXMM = (gtXMM > keyXMM)
		MOVO(chunks[i], gtXMM)
		PCMPGTL(keyXMM, gtXMM) // gtXMM = (chunks[i] > keyXMM) = (values > key)
		PMOVMSKB(gtXMM, gtBits)
		TESTL(gtBits, gtBits)
		JNZ(LabelRef(fmt.Sprintf("sseGtChunk%d", i)))

		// Check for sentinel
		MOVO(sentinelXMM, sentXMM)
		PCMPEQL(chunks[i], sentXMM)
		PMOVMSKB(sentXMM, sentBits)
		TESTL(sentBits, sentBits)
		JNZ(LabelRef(fmt.Sprintf("sseSentChunk%d", i)))
	}

	JMP(LabelRef("sseCalculateNext"))

	// Generate handlers for each chunk
	for i := range 4 {
		offset := i * 4

		// Greater than handler
		Label(fmt.Sprintf("sseGtChunk%d", i))
		BSFL(gtBits, childIdx.As32())
		SHRQ(Imm(2), childIdx) // Divide by 4 to get value index
		if offset > 0 {
			ADDQ(Imm(uint64(offset)), childIdx)
		}
		JMP(LabelRef("sseCalculateNext"))

		// Sentinel handler
		Label(fmt.Sprintf("sseSentChunk%d", i))
		BSFL(sentBits, childIdx.As32())
		SHRQ(Imm(2), childIdx)
		if offset > 0 {
			ADDQ(Imm(uint64(offset)), childIdx)
		}
		if i < 3 {
			JMP(LabelRef("sseCalculateNext"))
		}
	}

	Label("sseCalculateNext")
	// k = k * 17 + childIdx + 1
	temp := GP64()
	IMUL3Q(Imm(17), k, temp)
	ADDQ(childIdx, temp)
	INCQ(temp)
	MOVQ(temp, k)
	JMP(LabelRef("sseTraversalLoop"))

	// Generate found handlers for each chunk
	lane := GP32()
	result := GP64()
	temp2 := GP64()

	for i := range 4 {
		offset := i * 4

		Label(fmt.Sprintf("sseFoundInChunk%d", i))
		BSFL(matchBits, lane)
		SHRQ(Imm(2), lane.As64()) // Convert byte index to value index
		MOVQ(k, result)
		SHLQ(Imm(4), result)
		MOVL(lane, temp2.As32())
		ADDQ(temp2, result)
		if offset > 0 {
			ADDQ(Imm(uint64(offset)), result)
		}
		Store(result, ReturnIndex(0))
		RET()
	}

	Label("sseNotFound")
	result = GP64()
	MOVQ(operand.I64(-1), result)
	Store(result, ReturnIndex(0))
	RET()
}
