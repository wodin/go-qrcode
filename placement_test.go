// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"testing"

	bitset "github.com/skip2/go-qrcode/bitset"
)

func TestDataModulePathStartsAtBottomRight(t *testing.T) {
	v := getQRCodeVersion(Low, 1)
	path := dataModulePath(*v)

	// ISO/IEC 18004:2006 8.7.3: placement begins in the bottom right corner
	// and works upwards in two module wide columns, right module first.
	expected := []modulePosition{
		{20, 20}, {19, 20},
		{20, 19}, {19, 19},
		{20, 18}, {19, 18},
		{20, 17}, {19, 17},
	}

	for i, want := range expected {
		if path[i] != want {
			t.Errorf("path[%d] = %v, want %v", i, path[i], want)
		}
	}
}

func TestDataModulePathCoversDataRegionExactlyOnce(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			v := getQRCodeVersion(level, versionNumber)
			path := dataModulePath(*v)

			// The version tables say how many bits the data region holds.
			// The path must offer a module for each, and no more.
			if want := numDataRegionModules(*v); len(path) != want {
				t.Errorf("v=%d level=%d: path length = %d, want %d",
					versionNumber, level, len(path), want)
			}

			seen := map[modulePosition]bool{}
			size := v.symbolSize()

			for i, p := range path {
				if p.x < 0 || p.x >= size || p.y < 0 || p.y >= size {
					t.Fatalf("v=%d level=%d: path[%d] = %v is outside the symbol",
						versionNumber, level, i, p)
				}

				if seen[p] {
					t.Fatalf("v=%d level=%d: path[%d] = %v visited twice",
						versionNumber, level, i, p)
				}

				seen[p] = true
			}
		}
	}
}

func TestDataModulePathSkipsFunctionPatterns(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		v := getQRCodeVersion(Low, versionNumber)
		fps := functionPatternSymbol(*v)

		for i, p := range dataModulePath(*v) {
			if !fps.empty(p.x, p.y) {
				t.Fatalf("v=%d: path[%d] = %v is a function pattern module",
					versionNumber, i, p)
			}

			// The vertical timing pattern occupies a whole column.
			if p.x == verticalTimingPatternColumn {
				t.Fatalf("v=%d: path[%d] = %v is in the vertical timing pattern",
					versionNumber, i, p)
			}
		}
	}
}

// TestDataModulePathMatchesEncoderPlacement checks the contract the fit check
// depends on: bit i of the encoded stream lands in the module at path[i], so
// codeword n occupies path[8n:8n+8].
func TestDataModulePathMatchesEncoderPlacement(t *testing.T) {
	v := getQRCodeVersion(Low, 1)
	path := dataModulePath(*v)
	numBits := numDataRegionModules(*v)

	for mask := 0; mask <= 7; mask++ {
		zeros := bitset.New()
		zeros.AppendNumBools(numBits, false)

		baseline, err := buildRegularSymbol(*v, mask, zeros, false)
		if err != nil {
			t.Fatalf("buildRegularSymbol(mask=%d): %s", mask, err)
		}

		for i := 0; i < numBits; i++ {
			data := bitset.New()
			data.AppendNumBools(i, false)
			data.AppendBools(true)
			data.AppendNumBools(numBits-i-1, false)

			s, err := buildRegularSymbol(*v, mask, data, false)
			if err != nil {
				t.Fatalf("buildRegularSymbol(mask=%d, bit=%d): %s", mask, i, err)
			}

			for y := 0; y < v.symbolSize(); y++ {
				for x := 0; x < v.symbolSize(); x++ {
					differs := s.get(x, y) != baseline.get(x, y)
					isBitsModule := path[i] == modulePosition{x, y}

					if differs != isBitsModule {
						t.Fatalf("mask=%d bit=%d: module (%d,%d) differs=%v, "+
							"want %v (path[%d] = %v)",
							mask, i, x, y, differs, isBitsModule, i, path[i])
					}
				}
			}
		}
	}
}

// BenchmarkDataModulePath measures the walk on its own. encode() calls it
// once per mask, eight times per encode, though it depends only on the
// version — see issue #12.
func BenchmarkDataModulePath(b *testing.B) {
	v := getQRCodeVersion(Low, 40)

	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		dataModulePath(*v)
	}
}
