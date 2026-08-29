// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"runtime"
	"strings"
	"testing"

	bitset "github.com/skip2/go-qrcode/bitset"
)

func TestDataModulePathStartsAtBottomRight(t *testing.T) {
	v := getQRCodeVersion(Low, 1)
	path := newPlacementPath(*v).modules

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
			path := newPlacementPath(*v).modules

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

		for i, p := range newPlacementPath(*v).modules {
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
	path := newPlacementPath(*v)
	numBits := numDataRegionModules(*v)

	for mask := 0; mask <= 7; mask++ {
		zeros := bitset.New()
		zeros.AppendNumBools(numBits, false)

		baseline, err := buildRegularSymbol(path, mask, zeros, false)
		if err != nil {
			t.Fatalf("buildRegularSymbol(mask=%d): %s", mask, err)
		}

		for i := 0; i < numBits; i++ {
			data := bitset.New()
			data.AppendNumBools(i, false)
			data.AppendBools(true)
			data.AppendNumBools(numBits-i-1, false)

			s, err := buildRegularSymbol(path, mask, data, false)
			if err != nil {
				t.Fatalf("buildRegularSymbol(mask=%d, bit=%d): %s", mask, i, err)
			}

			for y := 0; y < v.symbolSize(); y++ {
				for x := 0; x < v.symbolSize(); x++ {
					differs := s.get(x, y) != baseline.get(x, y)
					isBitsModule := path.modules[i] == modulePosition{x, y}

					if differs != isBitsModule {
						t.Fatalf("mask=%d bit=%d: module (%d,%d) differs=%v, "+
							"want %v (path[%d] = %v)",
							mask, i, x, y, differs, isBitsModule, i, path.modules[i])
					}
				}
			}
		}
	}
}

// bytesAllocated returns the mean number of heap bytes f allocates per call.
//
// Bytes rather than allocation counts, because the placement path's weight is
// in the size of what it allocates rather than in the number of objects: at
// version 40 one path is 553 KB against an encode's 15,800 other allocations.
func bytesAllocated(runs int, f func()) uint64 {
	var before, after runtime.MemStats

	runtime.ReadMemStats(&before)

	for i := 0; i < runs; i++ {
		f()
	}

	runtime.ReadMemStats(&after)

	return (after.TotalAlloc - before.TotalAlloc) / uint64(runs)
}

// TestEncodeBuildsThePlacementPathOnce pins the lifetime the path is built
// for: one per encode, shared by all eight data masks, rather than one per
// mask.
//
// Expressed in allocated bytes so that it needs no figure of its own to go
// stale. An encode that built the path per mask would allocate eight paths,
// so it could not come in under eight paths' worth however cheap the rest of
// it was; an encode that builds one comes in far under, since the other
// allocations are a fraction of a single path at this version.
func TestEncodeBuildsThePlacementPathOnce(t *testing.T) {
	// Version 40 is where the path is largest, and so where the difference
	// between one and eight is least likely to be lost in the noise.
	v := getQRCodeVersion(Low, 40)

	const runs = 5

	pathBytes := bytesAllocated(runs, func() {
		newPlacementPath(*v)
	})

	// encode() adds the terminator and padding to q.data, so each run needs
	// its own QRCode. Building them outside the measurement keeps New's
	// allocations out of the figure.
	codes := make([]*QRCode, runs)
	for i := range codes {
		q, err := New(strings.Repeat("0", 7089), Low)
		if err != nil {
			t.Fatal(err)
		}

		codes[i] = q
	}

	i := 0
	encodeBytes := bytesAllocated(runs, func() {
		codes[i].encode()
		i++
	})

	if limit := 8 * pathBytes; encodeBytes >= limit {
		t.Errorf("a version 40 encode allocates %d bytes, want under %d "+
			"(8 x the %d bytes of one placement path): the path looks like "+
			"it is being built once per mask",
			encodeBytes, limit, pathBytes)
	}
}

// BenchmarkDataModulePath measures the walk on its own. An encode runs it
// once, whatever the mask, since the path depends only on the version.
func BenchmarkDataModulePath(b *testing.B) {
	v := getQRCodeVersion(Low, 40)

	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		newPlacementPath(*v)
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
