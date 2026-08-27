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
