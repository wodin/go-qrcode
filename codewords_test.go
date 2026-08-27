// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"testing"
)

// numTotalCodewords returns the number of codewords a v symbol carries, data
// and error correction together.
//
// Deliberately derived from the version tables rather than from the
// production expansion of them, so the tests check that expansion rather than
// restate it.
func numTotalCodewords(v qrCodeVersion) int {
	codewords := 0

	for _, b := range v.block {
		codewords += b.numBlocks * b.numCodewords
	}

	return codewords
}

func TestBlockSizesMatchVersionTable(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)
			sizes := blockSizes(*v)

			if len(sizes) != v.numBlocks() {
				t.Fatalf("v=%d level=%d: %d block sizes, want %d",
					versionNumber, level, len(sizes), v.numBlocks())
			}

			// The table groups identically sized blocks; the expansion must
			// list them in the same order, one entry per block.
			i := 0
			for _, group := range v.block {
				for j := 0; j < group.numBlocks; j++ {
					want := blockSize{
						numCodewords:     group.numCodewords,
						numDataCodewords: group.numDataCodewords,
					}

					if sizes[i] != want {
						t.Errorf("v=%d level=%d: block %d = %+v, want %+v",
							versionNumber, level, i, sizes[i], want)
					}

					i++
				}
			}
		}
	}
}

// TestInterleaveOrderMatchesISOWorkedExample checks the interleave against the
// worked example of ISO/IEC 18004:2006 8.6, which interleaves a version 5-H
// symbol: four blocks holding 11, 11, 12 and 12 data codewords and 22 error
// correction codewords each.
//
// The standard labels that symbol's data codewords D1-D46 and its error
// correction codewords E1-E88, block by block, and prints the interleaved
// result as D1 D12 D23 D35 D2 D13 D24 D36 ... D11 D22 D33 D45 D34 D46
// E1 E23 E45 E67 ... The expectations below are those labels translated into
// the block and codeword indices interleaveOrder reports.
func TestInterleaveOrderMatchesISOWorkedExample(t *testing.T) {
	v := getQRCodeVersion(Highest, 5)
	order := interleaveOrder(*v)

	if len(order) != 134 {
		t.Fatalf("interleaved %d codewords, want 134", len(order))
	}

	tests := []struct {
		label string
		at    int
		want  codewordSource
	}{
		// The first two rounds of data codewords.
		{"D1", 0, codewordSource{block: 0, codeword: 0}},
		{"D12", 1, codewordSource{block: 1, codeword: 0}},
		{"D23", 2, codewordSource{block: 2, codeword: 0}},
		{"D35", 3, codewordSource{block: 3, codeword: 0}},
		{"D2", 4, codewordSource{block: 0, codeword: 1}},
		{"D13", 5, codewordSource{block: 1, codeword: 1}},
		{"D24", 6, codewordSource{block: 2, codeword: 1}},
		{"D36", 7, codewordSource{block: 3, codeword: 1}},

		// The last full round, then the two longer blocks alone: blocks 0 and
		// 1 have run out of data codewords and are skipped.
		{"D11", 40, codewordSource{block: 0, codeword: 10}},
		{"D22", 41, codewordSource{block: 1, codeword: 10}},
		{"D33", 42, codewordSource{block: 2, codeword: 10}},
		{"D45", 43, codewordSource{block: 3, codeword: 10}},
		{"D34", 44, codewordSource{block: 2, codeword: 11}},
		{"D46", 45, codewordSource{block: 3, codeword: 11}},

		// Error correction codewords follow every data codeword, and are
		// interleaved the same way. Block 0 and 1 hold 11 data codewords, so
		// their first error correction codeword is codeword 11; blocks 2 and
		// 3 hold 12, so theirs is codeword 12.
		{"E1", 46, codewordSource{block: 0, codeword: 11}},
		{"E23", 47, codewordSource{block: 1, codeword: 11}},
		{"E45", 48, codewordSource{block: 2, codeword: 12}},
		{"E67", 49, codewordSource{block: 3, codeword: 12}},

		// The last round of error correction codewords.
		{"E22", 130, codewordSource{block: 0, codeword: 32}},
		{"E44", 131, codewordSource{block: 1, codeword: 32}},
		{"E66", 132, codewordSource{block: 2, codeword: 33}},
		{"E88", 133, codewordSource{block: 3, codeword: 33}},
	}

	for _, test := range tests {
		if got := order[test.at]; got != test.want {
			t.Errorf("codeword %d (%s) = %+v, want %+v",
				test.at, test.label, got, test.want)
		}
	}
}

func TestInterleaveOrderVisitsEveryCodewordOnce(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)
			order := interleaveOrder(*v)

			if want := numTotalCodewords(*v); len(order) != want {
				t.Fatalf("v=%d level=%d: interleaved %d codewords, want %d",
					versionNumber, level, len(order), want)
			}

			seen := map[codewordSource]bool{}
			sizes := blockSizes(*v)

			for i, src := range order {
				if src.block < 0 || src.block >= len(sizes) {
					t.Fatalf("v=%d level=%d: codeword %d comes from block %d, "+
						"which does not exist",
						versionNumber, level, i, src.block)
				}

				if src.codeword < 0 || src.codeword >= sizes[src.block].numCodewords {
					t.Fatalf("v=%d level=%d: codeword %d is codeword %d of a "+
						"block holding %d",
						versionNumber, level, i, src.codeword,
						sizes[src.block].numCodewords)
				}

				if seen[src] {
					t.Fatalf("v=%d level=%d: codeword %d (%+v) interleaved twice",
						versionNumber, level, i, src)
				}

				seen[src] = true
			}
		}
	}
}

// TestInterleaveOrderPlacesDataBeforeErrorCorrection checks the property the
// interleave exists for: contiguous damage spreads across every block rather
// than destroying one (ISO/IEC 18004:2006 8.6).
func TestInterleaveOrderPlacesDataBeforeErrorCorrection(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)
			sizes := blockSizes(*v)

			seenErrorCodeword := false

			for i, src := range interleaveOrder(*v) {
				isData := src.codeword < sizes[src.block].numDataCodewords

				if isData && seenErrorCodeword {
					t.Fatalf("v=%d level=%d: data codeword %d (%+v) follows an "+
						"error correction codeword",
						versionNumber, level, i, src)
				}

				if !isData {
					seenErrorCodeword = true
				}
			}
		}
	}
}

func TestCodewordLayoutGivesEveryCodewordEightModules(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)
			l := newCodewordLayout(*v)
			size := v.symbolSize()

			if want := numTotalCodewords(*v); l.numCodewords() != want {
				t.Fatalf("v=%d level=%d: layout holds %d codewords, want %d",
					versionNumber, level, l.numCodewords(), want)
			}

			modules := make([]int, l.numCodewords())

			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					n := l.codewordAt(x, y)
					if n == noCodeword {
						continue
					}

					if n < 0 || n >= l.numCodewords() {
						t.Fatalf("v=%d level=%d: module (%d,%d) holds codeword "+
							"%d, which does not exist",
							versionNumber, level, x, y, n)
					}

					modules[n]++
				}
			}

			for n, count := range modules {
				if count != 8 {
					t.Fatalf("v=%d level=%d: codeword %d occupies %d modules, want 8",
						versionNumber, level, n, count)
				}
			}
		}
	}
}

// TestCodewordLayoutLeavesFunctionPatternsUnassigned checks the two kinds of
// module that carry no codeword: function pattern modules, which a logo must
// never occlude, and the version's remainder bits, which a logo may occlude
// for free because no codeword depends on them.
func TestCodewordLayoutLeavesFunctionPatternsUnassigned(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)
			l := newCodewordLayout(*v)
			fps := functionPatternSymbol(*v)
			size := v.symbolSize()

			unassignedDataModules := 0

			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					assigned := l.codewordAt(x, y) != noCodeword

					if !fps.empty(x, y) {
						if assigned {
							t.Fatalf("v=%d level=%d: function pattern module "+
								"(%d,%d) holds codeword %d",
								versionNumber, level, x, y, l.codewordAt(x, y))
						}

						continue
					}

					if !assigned {
						unassignedDataModules++
					}
				}
			}

			if unassignedDataModules != v.numRemainderBits {
				t.Errorf("v=%d level=%d: %d data region modules hold no "+
					"codeword, want %d (the version's remainder bits)",
					versionNumber, level, unassignedDataModules,
					v.numRemainderBits)
			}
		}
	}
}

func TestCodewordLayoutBlocksMatchVersionTable(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)
			l := newCodewordLayout(*v)

			if l.numBlocks() != v.numBlocks() {
				t.Fatalf("v=%d level=%d: layout holds %d blocks, want %d",
					versionNumber, level, l.numBlocks(), v.numBlocks())
			}

			codewordsInBlock := make([]int, v.numBlocks())

			for n := 0; n < l.numCodewords(); n++ {
				b := l.blockOf(n)

				if b < 0 || b >= len(codewordsInBlock) {
					t.Fatalf("v=%d level=%d: codeword %d belongs to block %d, "+
						"which does not exist", versionNumber, level, n, b)
				}

				codewordsInBlock[b]++
			}

			for b, count := range codewordsInBlock {
				if want := l.block(b).numCodewords; count != want {
					t.Errorf("v=%d level=%d: block %d owns %d codewords, want %d",
						versionNumber, level, b, count, want)
				}
			}
		}
	}
}

// TestCodewordLayoutMatchesISOWorkedExample reads the codewords back out of a
// built symbol using the layout, and checks them against the worked example
// of ISO/IEC 18004:2006 Annex I: the content "01234567" at version 1,
// recovery level Medium.
//
// This is the one place in the feature where an external source says what the
// answer is. It pins the layout's grouping of modules into codewords, the bit
// order within a codeword, and the interleave — a mistake in any of them
// yields different bytes. It does not pin the geometry of the placement path
// itself, since the encoder writes and the layout reads through the same
// path; TestRegularSymbolPlacementGolden and the zbarimg decode tests cover
// that.
func TestCodewordLayoutMatchesISOWorkedExample(t *testing.T) {
	// ISO/IEC 18004:2006 Annex I. Sixteen data codewords: the encoded content,
	// a terminator, and the alternating pad codewords 0xec 0x11 filling the
	// remaining capacity. Then ten error correction codewords.
	want := []byte{
		0x10, 0x20, 0x0c, 0x56, 0x61, 0x80, 0xec, 0x11,
		0xec, 0x11, 0xec, 0x11, 0xec, 0x11, 0xec, 0x11,
		0xa5, 0x24, 0xd4, 0xc1, 0xed, 0x36, 0xc7, 0x87,
		0x2c, 0x55,
	}

	q, err := New("01234567", Medium)
	if err != nil {
		t.Fatalf("New: %s", err)
	}

	if q.VersionNumber != 1 {
		t.Fatalf("version = %d, want the worked example's version 1",
			q.VersionNumber)
	}

	// The steps New leaves to encode: complete the bit stream, then split,
	// error correct and interleave it.
	q.addTerminatorBits(q.version.numTerminatorBitsRequired(q.data.Len()))
	q.addPadding()

	encoded := q.encodeBlocks()
	l := newCodewordLayout(q.version)

	// The mask is a property of the module's position, not of the codeword,
	// so reading the codewords back must give the same answer under every one.
	for mask := 0; mask <= 7; mask++ {
		s, err := buildRegularSymbol(q.version, mask, encoded, false)
		if err != nil {
			t.Fatalf("buildRegularSymbol(mask=%d): %s", mask, err)
		}

		got := readCodewords(s, mask, l)

		for n, b := range want {
			if got[n] != b {
				t.Fatalf("mask=%d: codeword %d = %#02x, want %#02x "+
					"(got %#02x, want %#02x)",
					mask, n, got[n], b, got, want)
			}
		}
	}
}

// readCodewords reads s's codewords back out through l, undoing mask.
//
// Each codeword is assembled from the eight modules l assigns to it, taken in
// placement path order, most significant bit first.
func readCodewords(s *symbol, mask int, l *codewordLayout) []byte {
	codewords := make([]byte, l.numCodewords())

	for _, p := range dataModulePath(l.version) {
		n := l.codewordAt(p.x, p.y)
		if n == noCodeword {
			continue
		}

		codewords[n] <<= 1

		if s.get(p.x, p.y) != dataMask(mask, p.x, p.y) {
			codewords[n] |= 1
		}
	}

	return codewords
}
