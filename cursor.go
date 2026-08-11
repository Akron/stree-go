package stree

// maxCursorDepth is the maximum tree depth supported by Cursor.
// With branching factor 17, depth 8 covers trees with up to 17^8-1 ≈ 6.97
// billion values, which exceeds the full uint32 value space (~4.29 billion).
const maxCursorDepth = 8

// cursorFrame represents one level of the in-order traversal stack.
type cursorFrame struct {
	k     uint32 // block index
	i     uint8  // position within block (0..blockSize)
	phase uint8  // 0 = descend to child, 1 = yield element
}

// Cursor provides allocation-free, pull-style sorted iteration over an S-Tree.
// It maintains an explicit traversal stack.
//
// Two cursors can be used concurrently, as they
// only read the shared immutable block data.
//
// Usage:
//
//	c := reader.Cursor()
//	for {
//	    val, idx, ok := c.Next()
//	    if !ok { break }
//	    // process (val, idx)
//	}
//	c.Reset() // reuse for another pass
type Cursor struct {
	blocks    []byte
	numBlocks int
	stack     [maxCursorDepth]cursorFrame
	depth     int
	val       uint32
	idx       int
	ok        bool
}

// Cursor returns a pull-style cursor for sorted (ascending) iteration.
// The cursor is returned by value with zero heap allocations.
// The cursor is primed with the first element; call Next() or Peek()
// to access it.
func (r *Reader) Cursor() Cursor {
	c := Cursor{
		blocks:    r.blocks,
		numBlocks: r.numBlocks,
	}
	if r.numBlocks > 0 {
		c.stack[0] = cursorFrame{k: 0}
		c.depth = 1
		c.advance()
	}
	return c
}

// Next returns the next (value, index) pair in sorted order.
// Returns ok=false when iteration is exhausted.
// The index is the position in the S-Tree data structure (Eytzinger index).
func (c *Cursor) Next() (value uint32, index int, ok bool) {
	value, index, ok = c.val, c.idx, c.ok
	if ok {
		c.advance()
	}
	return
}

// Peek returns the next (value, index) pair without advancing the cursor.
// Subsequent calls to Peek return the same result until Next is called.
// Returns ok=false when iteration is exhausted.
func (c *Cursor) Peek() (value uint32, index int, ok bool) {
	return c.val, c.idx, c.ok
}

// Reset restarts iteration from the beginning of the tree.
// This allows reuse of the cursor without creating a new one.
func (c *Cursor) Reset() {
	c.depth = 0
	c.ok = false
	if c.numBlocks > 0 {
		c.stack[0] = cursorFrame{k: 0}
		c.depth = 1
		c.advance()
	}
}

// advance moves the cursor to the next value in sorted order.
// It implements an iterative in-order traversal using the explicit stack,
// with a fast path for leaf blocks (which comprise ~94% of all blocks
// in a tree with branching factor 17).
func (c *Cursor) advance() {
	for c.depth > 0 {
		f := &c.stack[c.depth-1]
		k := int(f.k)

		// Fast path: leaf blocks have no children, so skip the
		// phase-based state machine and iterate sequentially.
		if childIndex(k, 0) >= c.numBlocks {
			i := int(f.i)
			if i < blockSize {
				offset := k*blockSizeBytes + i*4
				val := be.Uint32(c.blocks[offset:])
				if val == sentinel {
					c.depth--
					continue
				}
				c.val = val
				c.idx = k*blockSize + i
				c.ok = true
				f.i = uint8(i + 1)
				return
			}
			c.depth--
			continue
		}

		// Internal block: alternate between descending to children
		// (phase 0) and yielding keys (phase 1).
		if f.phase == 0 {
			i := int(f.i)
			f.phase = 1
			child := childIndex(k, i)
			if child < c.numBlocks {
				c.stack[c.depth] = cursorFrame{k: uint32(child)}
				c.depth++
			}
			continue
		}

		// Phase 1: yield the key at the current position, or pop if done.
		i := int(f.i)
		if i < blockSize {
			offset := k*blockSizeBytes + i*4
			val := be.Uint32(c.blocks[offset:])
			if val == sentinel {
				c.depth--
				continue
			}
			c.val = val
			c.idx = k*blockSize + i
			c.ok = true
			f.i = uint8(i + 1)
			f.phase = 0
			return
		}

		c.depth--
	}
	c.ok = false
}
