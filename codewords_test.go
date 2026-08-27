// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"bytes"
	"fmt"
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

// everyLevel is the four recovery levels, least protective first.
var everyLevel = []RecoveryLevel{Low, Medium, High, Highest}

// versionLevelName names a subtest for one version and recovery level.
func versionLevelName(versionNumber int, level RecoveryLevel) string {
	return fmt.Sprintf("v%d-level%d", versionNumber, level)
}

// forEveryVersion runs test once for each of the 160 version and recovery
// level combinations, as a subtest named for the pair. Naming the subtest is
// what lets the tests themselves report a failure without repeating which
// version and level it happened at.
func forEveryVersion(t *testing.T, test func(t *testing.T, v qrCodeVersion)) {
	t.Helper()

	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range everyLevel {
			v := getQRCodeVersion(level, versionNumber)

			t.Run(versionLevelName(versionNumber, level), func(t *testing.T) {
				test(t, *v)
			})
		}
	}
}

func TestBlockShapesMatchVersionTable(t *testing.T) {
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		shapes := blockShapes(v)

		if len(shapes) != v.numBlocks() {
			t.Fatalf("%d block shapes, want %d", len(shapes), v.numBlocks())
		}

		// The table groups identically shaped blocks; the expansion must list
		// them in the same order, one entry per block.
		i := 0

		for _, group := range v.block {
			for j := 0; j < group.numBlocks; j++ {
				want := blockShape{
					numCodewords:     group.numCodewords,
					numDataCodewords: group.numDataCodewords,
				}

				if shapes[i] != want {
					t.Errorf("block %d = %+v, want %+v", i, shapes[i], want)
				}

				i++
			}
		}
	})
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
		// interleaved the same way. Blocks 0 and 1 hold 11 data codewords, so
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
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		order := interleaveOrder(v)

		if want := numTotalCodewords(v); len(order) != want {
			t.Fatalf("interleaved %d codewords, want %d", len(order), want)
		}

		seen := map[codewordSource]bool{}
		shapes := blockShapes(v)

		for i, src := range order {
			if src.block < 0 || src.block >= len(shapes) {
				t.Fatalf("codeword %d comes from block %d, which does not exist",
					i, src.block)
			}

			if src.codeword < 0 || src.codeword >= shapes[src.block].numCodewords {
				t.Fatalf("codeword %d is codeword %d of a block holding %d",
					i, src.codeword, shapes[src.block].numCodewords)
			}

			if seen[src] {
				t.Fatalf("codeword %d (%+v) interleaved twice", i, src)
			}

			seen[src] = true
		}
	})
}

// TestInterleaveOrderPlacesDataBeforeErrorCorrection checks the property the
// interleave exists for: contiguous damage spreads across every block rather
// than destroying one (ISO/IEC 18004:2006 8.6).
func TestInterleaveOrderPlacesDataBeforeErrorCorrection(t *testing.T) {
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		shapes := blockShapes(v)
		seenErrorCodeword := false

		for i, src := range interleaveOrder(v) {
			isData := src.codeword < shapes[src.block].numDataCodewords

			if isData && seenErrorCodeword {
				t.Fatalf("data codeword %d (%+v) follows an error correction "+
					"codeword", i, src)
			}

			if !isData {
				seenErrorCodeword = true
			}
		}
	})
}

func TestCodewordLayoutGivesEveryCodewordEightModules(t *testing.T) {
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		l := newCodewordLayout(v)
		size := v.symbolSize()

		if want := numTotalCodewords(v); l.numCodewords() != want {
			t.Fatalf("layout holds %d codewords, want %d",
				l.numCodewords(), want)
		}

		modules := make([]int, l.numCodewords())

		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				n := l.codewordAt(x, y)
				if n == noCodeword {
					continue
				}

				if n < 0 || n >= l.numCodewords() {
					t.Fatalf("module (%d,%d) holds codeword %d, which does "+
						"not exist", x, y, n)
				}

				modules[n]++
			}
		}

		for n, count := range modules {
			if count != 8 {
				t.Fatalf("codeword %d occupies %d modules, want 8", n, count)
			}
		}
	})
}

// TestCodewordLayoutLeavesFunctionPatternsAndRemainderBitsUnassigned checks
// the two kinds of module that carry no codeword: function pattern modules,
// which a logo must never occlude, and the version's remainder bits, which a
// logo may occlude for free because no codeword depends on them.
func TestCodewordLayoutLeavesFunctionPatternsAndRemainderBitsUnassigned(t *testing.T) {
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		l := newCodewordLayout(v)
		fps := functionPatternSymbol(v)
		size := v.symbolSize()

		unassignedDataModules := 0

		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				assigned := l.codewordAt(x, y) != noCodeword

				if !fps.empty(x, y) {
					if assigned {
						t.Fatalf("function pattern module (%d,%d) holds "+
							"codeword %d", x, y, l.codewordAt(x, y))
					}

					continue
				}

				if !assigned {
					unassignedDataModules++
				}
			}
		}

		if unassignedDataModules != v.numRemainderBits {
			t.Errorf("%d data region modules hold no codeword, want %d (the "+
				"version's remainder bits)",
				unassignedDataModules, v.numRemainderBits)
		}
	})
}

func TestCodewordLayoutBlocksMatchVersionTable(t *testing.T) {
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		l := newCodewordLayout(v)

		if l.numBlocks() != v.numBlocks() {
			t.Fatalf("layout holds %d blocks, want %d",
				l.numBlocks(), v.numBlocks())
		}

		codewordsInBlock := make([]int, v.numBlocks())

		for n := 0; n < l.numCodewords(); n++ {
			b := l.blockOf(n)

			if b < 0 || b >= len(codewordsInBlock) {
				t.Fatalf("codeword %d belongs to block %d, which does not exist",
					n, b)
			}

			codewordsInBlock[b]++
		}

		for b, count := range codewordsInBlock {
			if want := l.block(b).numCodewords; count != want {
				t.Errorf("block %d owns %d codewords, want %d", b, count, want)
			}
		}
	})
}

// The worked example of ISO/IEC 18004:2006 Annex I: the content "01234567" at
// version 1, recovery level Medium. It is the only place in this feature
// where a source outside this package says what the answer is.

// isoWorkedExampleCodewords is the worked example's 26 codewords. Sixteen data
// codewords — the encoded content, a terminator, and the alternating pad
// codewords 0xec 0x11 filling the remaining capacity — then ten error
// correction codewords.
var isoWorkedExampleCodewords = []byte{
	0x10, 0x20, 0x0c, 0x56, 0x61, 0x80, 0xec, 0x11,
	0xec, 0x11, 0xec, 0x11, 0xec, 0x11, 0xec, 0x11,
	0xa5, 0x24, 0xd4, 0xc1, 0xed, 0x36, 0xc7, 0x87,
	0x2c, 0x55,
}

// isoWorkedExampleMaskPattern is the data mask pattern the worked example's
// finished symbol uses. The standard's penalty rules select it, and this
// package's own penaltyScore independently selects it too.
const isoWorkedExampleMaskPattern = 2

// isoWorkedExampleSymbol is the worked example's finished symbol, one string
// per row, 'X' for a dark module. No quiet zone: these are symbol
// coordinates, so isoWorkedExampleSymbol[y][x] is the module at (x, y).
//
// Frozen deliberately, and that is the point of it. A test that builds a
// symbol and reads it back through the same placement path proves nothing
// about the path — write and read cancel out, and a systematically wrong path
// still recovers the right codewords. Reading a *fixed* symbol does not
// cancel: move the path and the layout picks different modules out of these
// rows, and the codewords come out wrong.
//
// Provenance: this matrix was checked against zbarimg, an outside decoder,
// which reads it as "01234567". TestDecodeBasic re-checks that content at
// this version and level whenever the decode tests are enabled.
var isoWorkedExampleSymbol = []string{
	"XXXXXXX..X.XX.XXXXXXX",
	"X.....X..XXXX.X.....X",
	"X.XXX.X.X.....X.XXX.X",
	"X.XXX.X.XX....X.XXX.X",
	"X.XXX.X.X.XXX.X.XXX.X",
	"X.....X.X...X.X.....X",
	"XXXXXXX.X.X.X.XXXXXXX",
	"........X..XX........",
	"X.XXXXX..X..X.XXXXX..",
	"...X.X.XX.X.X..X.XX..",
	"..X...XX.X.X.X..XXXXX",
	"....X....X.....XXXX..",
	"...XXXXXX..X.X..X....",
	"........X.XXXXX..XX..",
	"XXXXXXX..XX.X.XX.....",
	"X.....X.X.XXXXX...X.X",
	"X.XXX.X.X...X..X.XX..",
	"X.XXX.X.XX..X..X.....",
	"X.XXX.X.X.XX.X..X.X..",
	"X.....X........XX.XX.",
	"XXXXXXX.XXXX.X..X.X..",
}

// TestEncodedCodewordsMatchISOWorkedExample checks the codeword sequence the
// encoder produces — data, padding, error correction and interleave — against
// the standard's.
func TestEncodedCodewordsMatchISOWorkedExample(t *testing.T) {
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

	got := make([]byte, len(isoWorkedExampleCodewords))

	for i := 0; i < 8*len(got); i++ {
		got[i/8] <<= 1

		if encoded.At(i) {
			got[i/8] |= 1
		}
	}

	if !bytes.Equal(got, isoWorkedExampleCodewords) {
		t.Errorf("encoded codewords =\n\t%#x\nwant\n\t%#x",
			got, isoWorkedExampleCodewords)
	}
}

// TestCodewordLayoutMatchesISOWorkedExample reads the codewords back out of
// the standard's finished symbol using the layout, and checks them against
// the standard's codewords.
//
// This is what pins the mapping to an outside source: the symbol is a fixed
// matrix rather than one this package just built, so the layout has to pick
// the right eight modules out of it for each of the 26 codewords. Getting the
// placement path, the grouping into codewords, the bit order within a
// codeword or the mask wrong all yield different bytes.
func TestCodewordLayoutMatchesISOWorkedExample(t *testing.T) {
	v := getQRCodeVersion(Medium, 1)
	l := newCodewordLayout(*v)

	if l.numCodewords() != len(isoWorkedExampleCodewords) {
		t.Fatalf("layout holds %d codewords, want %d",
			l.numCodewords(), len(isoWorkedExampleCodewords))
	}

	got := make([]byte, l.numCodewords())

	// Each codeword is assembled from the eight modules the layout assigns to
	// it, taken in placement path order, most significant bit first.
	for _, p := range dataModulePath(*v) {
		n := l.codewordAt(p.x, p.y)
		if n == noCodeword {
			continue
		}

		dark := isoWorkedExampleSymbol[p.y][p.x] == 'X'

		got[n] <<= 1

		if dark != dataMask(isoWorkedExampleMaskPattern, p.x, p.y) {
			got[n] |= 1
		}
	}

	if !bytes.Equal(got, isoWorkedExampleCodewords) {
		t.Errorf("codewords read out of the worked example's symbol =\n"+
			"\t%#x\nwant\n\t%#x", got, isoWorkedExampleCodewords)
	}
}
