// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	bitset "github.com/skip2/go-qrcode/bitset"
)

func TestBuildRegularSymbol(t *testing.T) {
	for k := 0; k <= 7; k++ {
		v := getQRCodeVersion(Low, 1)

		data := bitset.New()
		for i := 0; i < 26; i++ {
			data.AppendNumBools(8, false)
		}

		s, err := buildRegularSymbol(*v, k, data, false)

		if err != nil {
			fmt.Println(err.Error())
		} else {
			_ = s
			//fmt.Print(m.string())
		}
	}
}

// numDataRegionModules returns the number of modules in v's data region,
// which is also the number of bits buildRegularSymbol expects to be given.
//
// Deliberately derived from the version tables rather than by asking a symbol
// how many modules it has left: the tests use it as an independent check on
// the path, so sharing the production derivation would make them tautologies.
// Not to be confused with numDataBits, which counts data codewords only.
func numDataRegionModules(v qrCodeVersion) int {
	bits := v.numRemainderBits

	for _, b := range v.block {
		bits += 8 * b.numBlocks * b.numCodewords
	}

	return bits
}

// pseudoRandomBits returns n deterministic bits, using xorshift32 so the
// stream is fixed by this function alone and not by the standard library's.
//
// A *varying* stream is what makes a symbol digest sensitive to placement
// order. Given a constant stream every module's value collapses to
// dataMask(x, y), which depends only on where the module is and not on which
// bit landed there, so any permutation of the path would digest identically.
func pseudoRandomBits(n int) *bitset.Bitset {
	b := bitset.New()
	state := uint32(1)

	for i := 0; i < n; i++ {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5

		b.AppendBools(state&1 == 1)
	}

	return b
}

// TestRegularSymbolPlacementGolden pins the module placement of every
// version/level/mask combination, so that refactoring the placement path
// cannot silently move a codeword. The digest covers the symbol bitmaps
// produced from a fixed pseudo-random codeword stream, which makes it
// sensitive to the order of the placement path and not merely to the set of
// modules the path visits.
func TestRegularSymbolPlacementGolden(t *testing.T) {
	const expectedDigest = "6d2b28a6ae54ee16824054110ae91627a4f5fce8efc1d61db419cbb55c7d40cb"

	digest := sha256.New()

	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)

			data := pseudoRandomBits(numDataRegionModules(*v))

			for mask := 0; mask <= 7; mask++ {
				s, err := buildRegularSymbol(*v, mask, data, false)
				if err != nil {
					t.Fatalf("buildRegularSymbol(v=%d, level=%d, mask=%d): %s",
						versionNumber, level, mask, err)
				}

				if n := s.numEmptyModules(); n != 0 {
					t.Fatalf("v=%d level=%d mask=%d: %d modules left empty",
						versionNumber, level, mask, n)
				}

				for _, row := range s.bitmap() {
					for _, module := range row {
						if module {
							digest.Write([]byte{1})
						} else {
							digest.Write([]byte{0})
						}
					}
				}
			}
		}
	}

	if got := hex.EncodeToString(digest.Sum(nil)); got != expectedDigest {
		t.Errorf("placement digest = %s, want %s (module placement changed)",
			got, expectedDigest)
	}
}

func TestBuildRegularSymbolRejectsOversizedData(t *testing.T) {
	v := getQRCodeVersion(Low, 1)

	data := bitset.New()
	data.AppendNumBools(numDataRegionModules(*v)+1, false)

	if _, err := buildRegularSymbol(*v, 0, data, false); err == nil {
		t.Error("buildRegularSymbol with one bit too much data: got nil error")
	}
}
