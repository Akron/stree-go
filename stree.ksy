meta:
  id: stree
  title: S-Tree Static B-Tree Index
  file-extension: stree
  endian: le
  
doc: |
  S-Tree is a static B-tree data structure optimized for cache efficiency
  and SIMD operations. It uses Eytzinger layout with a branching factor of 17
  (16 keys per node + 1 child pointer).
  
  Reference: https://en.algorithmica.org/hpc/data-structures/s-tree/
  
  The structure is designed for:
  - Memory-mapped access (mmap)
  - SIMD-accelerated search (SSE4.2 / AVX2)
  - Cache-friendly traversal (64-byte blocks = cache line)

seq:
  - id: header
    type: header
  - id: blocks
    type: block
    repeat: expr
    repeat-expr: num_blocks

types:
  header:
    doc: |
      16-byte header containing metadata about the S-Tree.
      The header is designed to be cache-line friendly and includes
      all information needed to search the tree.
    seq:
      - id: magic
        contents: "STRE"
        doc: Magic bytes identifying this as an S-Tree file
      - id: version
        type: u2
        doc: Format version (currently 0x0001)
      - id: block_size
        type: u2
        doc: Number of uint32 elements per block (default 16)
      - id: count
        type: u4
        doc: Total number of unique elements stored
      - id: crc32
        type: u4
        doc: CRC-32 checksum of header + data blocks for integrity validation

  block:
    doc: |
      A single block/node in the S-Tree.
      Each block contains up to 16 uint32 keys in Eytzinger order.
      Unused slots are filled with sentinel value 0xFFFFFFFF.
      
      Tree navigation uses implicit indexing:
      - Child i of node k is at index: k * 17 + i + 1
      - This eliminates pointer storage overhead
    seq:
      - id: keys
        type: u4
        repeat: expr
        repeat-expr: _root.header.block_size
        doc: |
          Keys stored in this node.
          Values are sorted within the node.
          Sentinel value 0xFFFFFFFF marks empty slots.

instances:
  num_blocks:
    value: (header.count + header.block_size - 1) / header.block_size
    doc: Number of blocks needed to store all elements
    
  data_offset:
    value: 16
    doc: Offset in bytes where block data begins (after header)

  block_size_bytes:
    value: header.block_size * 4
    doc: Size of each block in bytes (default 64 = cache line)
