// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

// blockSize is the shape of a single error correction block: how many
// codewords it holds, and how many of those carry data rather than error
// correction.
//
// Distinct from the version table's block, which describes a whole group of
// identically sized blocks at once. A version has one table row per group and
// one blockSize per block.
type blockSize struct {
	numCodewords     int
	numDataCodewords int
}

// numErrorCodewords returns the number of error correction codewords in the
// block. Half of them is the block's correction capacity: the number of
// damaged codewords it can recover at unknown positions.
func (b blockSize) numErrorCodewords() int {
	return b.numCodewords - b.numDataCodewords
}

// blockSizes expands v's grouped block table into one entry per block, in the
// order the encoder splits the data between them.
func blockSizes(v qrCodeVersion) []blockSize {
	sizes := make([]blockSize, 0, v.numBlocks())

	for _, group := range v.block {
		for i := 0; i < group.numBlocks; i++ {
			sizes = append(sizes, blockSize{
				numCodewords:     group.numCodewords,
				numDataCodewords: group.numDataCodewords,
			})
		}
	}

	return sizes
}

// codewordSource locates one codeword of a symbol's interleaved codeword
// sequence in the blocks it was interleaved from.
type codewordSource struct {
	// Index of the block the codeword belongs to, into blockSizes.
	block int

	// Index of the codeword within that block: data codewords first, then
	// the block's error correction codewords.
	codeword int
}

// interleaveOrder returns the source of every codeword of v's final codeword
// sequence, in the order the codewords are laid into the symbol.
//
// Blocks are visited round robin, contributing one codeword each per round
// and dropping out once exhausted: every block's data codewords are
// interleaved first, then every block's error correction codewords
// (ISO/IEC 18004:2006 8.6). Interleaving is what makes contiguous damage —
// a logo, a coffee ring — spread evenly across the blocks instead of
// destroying one of them outright.
//
// This is the only statement of the interleave in the package. encodeBlocks
// assembles the sequence with it and codewordLayout reads the sequence back
// with it, so the two cannot drift apart.
func interleaveOrder(v qrCodeVersion) []codewordSource {
	sizes := blockSizes(v)

	order := make([]codewordSource, 0, numCodewords(sizes))

	// Data codewords, then error correction codewords. The two passes differ
	// only in which codewords of a block they draw from.
	order = appendInterleaved(order, sizes, func(b blockSize) (int, int) {
		return 0, b.numDataCodewords
	})
	order = appendInterleaved(order, sizes, func(b blockSize) (int, int) {
		return b.numDataCodewords, b.numErrorCodewords()
	})

	return order
}

// appendInterleaved appends one round robin pass over sizes to order. For
// each block, extent reports the codeword the pass starts at and how many
// codewords it takes; a block contributes one per round until it runs out.
func appendInterleaved(order []codewordSource, sizes []blockSize,
	extent func(blockSize) (start int, count int)) []codewordSource {

	for i := 0; ; i++ {
		exhausted := true

		for b, size := range sizes {
			start, count := extent(size)

			if i >= count {
				continue
			}

			order = append(order, codewordSource{block: b, codeword: start + i})
			exhausted = false
		}

		if exhausted {
			return order
		}
	}
}

// numCodewords returns the total number of codewords held by blocks.
func numCodewords(blocks []blockSize) int {
	total := 0

	for _, b := range blocks {
		total += b.numCodewords
	}

	return total
}
