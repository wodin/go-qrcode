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

// numDataModules returns the number of modules in v's data region, which is
// also the number of bits buildRegularSymbol expects to be given.
func numDataModules(v qrCodeVersion) int {
	bits := v.numRemainderBits

	for _, b := range v.block {
		bits += 8 * b.numBlocks * b.numCodewords
	}

	return bits
}

// TestRegularSymbolPlacementGolden pins the module placement of every
// version/level/mask combination, so that refactoring the placement path
// cannot silently move a codeword. The digest covers the symbol bitmaps
// produced from an all-zero codeword stream: every module is then either a
// function pattern or the data mask, which is exactly what placement decides.
func TestRegularSymbolPlacementGolden(t *testing.T) {
	const expectedDigest = "2b4cc1735908e8467e48e8ac47c302a660e90e713570e46e078fa2e5bce93b67"

	digest := sha256.New()

	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)

			data := bitset.New()
			data.AppendNumBools(numDataModules(*v), false)

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
					for _, v := range row {
						if v {
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
