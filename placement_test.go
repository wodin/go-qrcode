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

// withinAnAlignmentPattern reports whether (x, y) lies in the 5x5 body of one
// of v's alignment patterns.
//
// Derived from the alignment pattern centre table rather than from the symbol
// the encoder builds, so that it can say independently what the difference
// between the two function pattern symbols is allowed to contain.
func withinAnAlignmentPattern(v qrCodeVersion, x int, y int) bool {
	centres := alignmentPatternCenter[v.version]

	near := func(centre int, i int) bool {
		return i >= centre-2 && i <= centre+2
	}

	for _, cx := range centres {
		for _, cy := range centres {
			if near(cx, x) && near(cy, y) {
				return true
			}
		}
	}

	return false
}

func TestProtectedFunctionPatternsCoverEveryKindButAlignment(t *testing.T) {
	// Version 7 is the smallest version carrying version info, so a single
	// symbol holds all four protected kinds at once.
	v := getQRCodeVersion(Medium, 7)
	protected := protectedFunctionPatternSymbol(*v)

	tests := []struct {
		kind string
		x, y int
	}{
		{"top left finder pattern", 0, 0},
		{"top right finder pattern", 44, 0},
		{"bottom left finder pattern", 0, 44},
		{"finder pattern separator", 7, 0},
		{"horizontal timing pattern", 8, 6},
		{"vertical timing pattern", 6, 8},
		{"format info", 8, 0},
		{"format info, second copy", 8, 44},
		{"always dark module", 8, 37},
		{"version info", 0, 34},
		{"version info, second copy", 34, 0},
	}

	for _, test := range tests {
		if protected.empty(test.x, test.y) {
			t.Errorf("module (%d,%d) is unprotected, want the %s protected",
				test.x, test.y, test.kind)
		}
	}
}

func TestProtectedFunctionPatternsExcludeAlignmentPatterns(t *testing.T) {
	// The centre of a version 7 symbol is the centre of an alignment pattern,
	// which is exactly the collision ADR-0002 permits.
	v := getQRCodeVersion(Medium, 7)
	protected := protectedFunctionPatternSymbol(*v)

	for _, p := range []modulePosition{{22, 22}, {20, 20}, {24, 24}, {20, 24}} {
		if !protected.empty(p.x, p.y) {
			t.Errorf("alignment pattern module (%d,%d) is protected, want it occludable",
				p.x, p.y)
		}
	}
}

func TestProtectedFunctionPatternsAreTheFunctionPatternsLessAlignment(t *testing.T) {
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		function := functionPatternSymbol(v)
		protected := protectedFunctionPatternSymbol(v)

		for y := 0; y < v.symbolSize(); y++ {
			for x := 0; x < v.symbolSize(); x++ {
				switch {
				case !protected.empty(x, y) && function.empty(x, y):
					t.Fatalf("module (%d,%d) is protected but is not a function pattern",
						x, y)

				case protected.empty(x, y) && !function.empty(x, y) &&
					!withinAnAlignmentPattern(v, x, y):
					t.Fatalf("function pattern module (%d,%d) is unprotected but is not part of an alignment pattern",
						x, y)
				}
			}
		}
	})
}
