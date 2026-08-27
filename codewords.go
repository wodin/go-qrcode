// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import "log"

// blockShape is the shape of a single error correction block: how many
// codewords it holds, and how many of those carry data rather than error
// correction.
//
// Distinct from the version table's block, which describes a whole group of
// identically sized blocks at once. A version has one table row per group and
// one blockShape per block.
type blockShape struct {
	numCodewords     int
	numDataCodewords int
}

// numErrorCodewords returns the number of error correction codewords in the
// block. Half of them is the block's correction capacity: the number of
// damaged codewords it can recover at unknown positions.
func (b blockShape) numErrorCodewords() int {
	return b.numCodewords - b.numDataCodewords
}

// correctionCapacity returns the number of damaged codewords the block can
// recover when their positions are unknown: half its error correction
// codewords (ISO/IEC 18004:2006 6.5.1).
func (b blockShape) correctionCapacity() int {
	return b.numErrorCodewords() / 2
}

// blockShapes expands v's grouped block table into one entry per block, in the
// order the encoder splits the data between them.
func blockShapes(v qrCodeVersion) []blockShape {
	shapes := make([]blockShape, 0, v.numBlocks())

	for _, group := range v.block {
		for i := 0; i < group.numBlocks; i++ {
			shapes = append(shapes, blockShape{
				numCodewords:     group.numCodewords,
				numDataCodewords: group.numDataCodewords,
			})
		}
	}

	return shapes
}

// codewordSource locates one codeword of a symbol's interleaved codeword
// sequence in the blocks it was interleaved from.
type codewordSource struct {
	// Index of the block the codeword belongs to, into blockShapes.
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
	shapes := blockShapes(v)

	capacity := 0
	for _, shape := range shapes {
		capacity += shape.numCodewords
	}

	order := make([]codewordSource, 0, capacity)

	// Data codewords, then error correction codewords. The two passes differ
	// only in which codewords of a block they draw from.
	order = appendInterleaved(order, shapes, func(b blockShape) (int, int) {
		return 0, b.numDataCodewords
	})
	order = appendInterleaved(order, shapes, func(b blockShape) (int, int) {
		return b.numDataCodewords, b.numErrorCodewords()
	})

	return order
}

// appendInterleaved appends one round robin pass over shapes to order. For
// each block, extent reports the codeword the pass starts at and how many
// codewords it takes; a block contributes one per round until it runs out.
func appendInterleaved(order []codewordSource, shapes []blockShape,
	extent func(blockShape) (start int, count int)) []codewordSource {

	for i := 0; ; i++ {
		exhausted := true

		for b, shape := range shapes {
			start, count := extent(shape)

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

// noCodeword is what codewordAt reports for a module carrying no codeword.
const noCodeword = -1

// codewordLayout says which codeword occupies each module of a symbol, and
// which block each codeword belongs to.
//
// Together those two mappings are what let occlusion be judged: a logo covers
// modules, error correction is spent on codewords, and capacity is held by
// blocks. Both hops are needed to get from one to the other.
//
// A layout depends on the version and recovery level only, not on the content
// encoded, so one is valid for every symbol of that version and level.
type codewordLayout struct {
	version    qrCodeVersion
	symbolSize int

	// codeword[y*symbolSize+x] is the index into the interleaved codeword
	// sequence of the codeword occupying the module at (x, y), or noCodeword.
	codeword []int

	// source[n] is where codeword n of the interleaved sequence came from.
	source []codewordSource

	// blocks[b] is the shape of block b.
	blocks []blockShape
}

// newCodewordLayout builds the layout of a version v symbol.
func newCodewordLayout(v qrCodeVersion) *codewordLayout {
	l := &codewordLayout{
		version:    v,
		symbolSize: v.symbolSize(),
		source:     interleaveOrder(v),
		blocks:     blockShapes(v),
	}

	l.codeword = make([]int, l.symbolSize*l.symbolSize)

	for i := range l.codeword {
		l.codeword[i] = noCodeword
	}

	// Bit i of the placed bit stream lands in the module at path[i], and the
	// stream is the interleaved codewords in order, so bits 8n to 8n+7 are
	// codeword n. Any bits beyond the last codeword are the version's
	// remainder bits, which pad the data region out to a whole number of
	// modules and belong to no codeword.
	numPlacedBits := 8 * len(l.source)

	for i, p := range dataModulePath(v) {
		if i >= numPlacedBits {
			break
		}

		l.codeword[p.y*l.symbolSize+p.x] = i / 8
	}

	return l
}

// codewordAt returns the index of the codeword occupying the module at
// (x, y), or noCodeword if the module carries none.
//
// Two kinds of module carry none: every function pattern module, and the
// version's remainder bits. The distinction matters to a caller judging
// occlusion — covering a function pattern is unrecoverable and covering a
// remainder bit costs nothing — so a caller that needs to tell them apart
// must ask functionPatternSymbol, not this.
func (l *codewordLayout) codewordAt(x int, y int) int {
	if x < 0 || x >= l.symbolSize || y < 0 || y >= l.symbolSize {
		log.Panicf("bug: module (%d,%d) is outside a version %d symbol, which "+
			"is %d modules wide",
			x, y, l.version.version, l.symbolSize)
	}

	return l.codeword[y*l.symbolSize+x]
}

// numCodewords returns the number of codewords the symbol carries, data and
// error correction together.
func (l *codewordLayout) numCodewords() int {
	return len(l.source)
}

// blockOf returns the index of the block codeword n belongs to.
func (l *codewordLayout) blockOf(n int) int {
	return l.source[n].block
}

// block returns the shape of block b.
func (l *codewordLayout) block(b int) blockShape {
	return l.blocks[b]
}

// numBlocks returns the number of error correction blocks the symbol is split
// into.
func (l *codewordLayout) numBlocks() int {
	return len(l.blocks)
}
