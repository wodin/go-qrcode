// go-qrcode
// Copyright 2014 Tom Harwood

package reedsolomon

import (
	"fmt"
	"sync"
	"testing"

	bitset "github.com/skip2/go-qrcode/bitset"
)

func TestGeneratorPoly(t *testing.T) {
	var tests = []struct {
		degree    int
		generator gfPoly
	}{
		// x^2 + 3x^1 + 2x^0 (the shortest generator poly)
		{
			2,
			gfPoly{term: []gfElement{2, 3, 1}},
		},
		// x^5 + 31x^4 + 198x^3 + 63x^2 + 147x^1 + 116x^0
		{
			5,
			gfPoly{term: []gfElement{116, 147, 63, 198, 31, 1}},
		},
		// x^68 + 131x^67 + 115x^66 + 9x^65 + 39x^64 + 18x^63 + 182x^62 + 60x^61 +
		// 94x^60 + 223x^59 + 230x^58 + 157x^57 + 142x^56 + 119x^55 + 85x^54 +
		// 107x^53 + 34x^52 + 174x^51 + 167x^50 + 109x^49 + 20x^48 + 185x^47 +
		// 112x^46 + 145x^45 + 172x^44 + 224x^43 + 170x^42 + 182x^41 + 107x^40 +
		// 38x^39 + 107x^38 + 71x^37 + 246x^36 + 230x^35 + 225x^34 + 144x^33 +
		// 20x^32 + 14x^31 + 175x^30 + 226x^29 + 245x^28 + 20x^27 + 219x^26 +
		// 212x^25 + 51x^24 + 158x^23 + 88x^22 + 63x^21 + 36x^20 + 199x^19 + 4x^18 +
		// 80x^17 + 157x^16 + 211x^15 + 239x^14 + 255x^13 + 7x^12 + 119x^11 + 11x^10
		// + 235x^9 + 12x^8 + 34x^7 + 149x^6 + 204x^5 + 8x^4 + 32x^3 + 29x^2 + 99x^1
		// + 11x^0 (the longest generator poly)
		{
			68,
			gfPoly{term: []gfElement{11, 99, 29, 32, 8, 204, 149, 34, 12,
				235, 11, 119, 7, 255, 239, 211, 157, 80, 4, 199, 36, 63, 88, 158, 51, 212,
				219, 20, 245, 226, 175, 14, 20, 144, 225, 230, 246, 71, 107, 38, 107, 182,
				170, 224, 172, 145, 112, 185, 20, 109, 167, 174, 34, 107, 85, 119, 142,
				157, 230, 223, 94, 60, 182, 18, 39, 9, 115, 131, 1}},
		},
	}

	for _, test := range tests {
		generator := rsGeneratorPoly(test.degree)

		if !generator.equals(test.generator) {
			t.Errorf("degree=%d generator=%s, want %s", test.degree,
				generator.string(true), test.generator.string(true))
		}
	}
}

func TestEncode(t *testing.T) {
	var tests = []struct {
		numECBytes int
		data       string
		rsCode     string
	}{
		{
			5,
			"01000000 00011000 10101100 11000011 00000000",
			"01000000 00011000 10101100 11000011 00000000 10000110 00001101 00100010 10101110 00110000",
		},
		{
			10,
			"00010000 00100000 00001100 01010110 01100001 10000000 11101100 00010001 11101100 00010001 11101100 00010001 11101100 00010001 11101100 00010001",
			"00010000 00100000 00001100 01010110 01100001 10000000 11101100 00010001 11101100 00010001 11101100 00010001 11101100 00010001 11101100 00010001 10100101 00100100 11010100 11000001 11101101 00110110 11000111 10000111 00101100 01010101",
		},
	}

	for _, test := range tests {
		data := bitset.NewFromBase2String(test.data)
		rsCode := bitset.NewFromBase2String(test.rsCode)

		result := Encode(data, test.numECBytes)

		if !rsCode.Equals(result) {
			t.Errorf("data=%s, numECBytes=%d, encoded=%s, want %s",
				data.String(),
				test.numECBytes,
				result.String(),
				rsCode)
		}
	}
}

// generatorSink keeps the compiler from discarding a generator polynomial the
// test below builds for its allocations alone.
var generatorSink gfPoly

// TestGeneratorPolyNotRebuiltPerCall pins the memoisation, with allocations as
// the evidence: a cached degree costs one allocation per call — the copy handed
// to the caller — where building the polynomial from scratch costs ~1900.
func TestGeneratorPolyNotRebuiltPerCall(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		generatorSink = rsGeneratorPoly(maxCachedGeneratorDegree)
	})

	if allocs > 1 {
		t.Errorf("rsGeneratorPoly(%d) allocs=%v, want <= 1 (rebuilt per call?)",
			maxCachedGeneratorDegree, allocs)
	}
}

// TestGeneratorPolyNotSharedBetweenCallers guards the cache against its callers:
// gfPoly holds a slice, so handing out the cached instance would let one caller
// corrupt the polynomial every later caller receives.
func TestGeneratorPolyNotSharedBetweenCallers(t *testing.T) {
	want := gfPoly{term: []gfElement{116, 147, 63, 198, 31, 1}}

	scribbled := rsGeneratorPoly(5)
	for i := range scribbled.term {
		scribbled.term[i] = 0
	}

	if generator := rsGeneratorPoly(5); !generator.equals(want) {
		t.Errorf("degree=5 generator=%s after a caller overwrote an earlier "+
			"copy, want %s", generator.string(true), want.string(true))
	}
}

// TestGeneratorPolyCacheMatchesFreshBuild checks the cache against the builder
// it stands in for, over every degree the table holds and the first one past
// its end, calling repeatedly so a second call cannot differ from the first.
func TestGeneratorPolyCacheMatchesFreshBuild(t *testing.T) {
	for degree := 2; degree <= maxCachedGeneratorDegree+1; degree++ {
		want := buildRSGeneratorPoly(degree)

		for call := 0; call < 3; call++ {
			generator := rsGeneratorPoly(degree)

			if !generator.equals(want) {
				t.Errorf("degree=%d call=%d generator=%s, want %s", degree, call,
					generator.string(true), want.string(true))
			}
		}
	}
}

// TestGeneratorPolyConcurrent races the fill itself: several goroutines ask one
// cold cache for the same degrees at once. Under `go test -race` this is what
// shows the memoisation has not cost the package the concurrency safety it has
// always had.
//
// The cache is an instance of the test's own rather than the package's, so the
// fill is certainly cold however many earlier tests have warmed the shared one,
// and racing it writes no package state.
func TestGeneratorPolyConcurrent(t *testing.T) {
	// Every degree the table holds, and one past its end so the uncached path
	// is raced too.
	want := make([]gfPoly, maxCachedGeneratorDegree+2)

	for degree := 2; degree < len(want); degree++ {
		want[degree] = buildRSGeneratorPoly(degree)
	}

	var cache generatorPolyCache

	failures := runConcurrently(func() []string {
		var failures []string

		for degree := 2; degree < len(want); degree++ {
			generator := cache.get(degree)

			if !generator.equals(want[degree]) {
				failures = append(failures, fmt.Sprintf("degree=%d generator=%s, want %s",
					degree, generator.string(true), want[degree].string(true)))
			}
		}

		return failures
	})

	for _, failure := range failures {
		t.Error(failure)
	}
}

// TestEncodeConcurrent demonstrates the criterion the cache had to preserve at
// the level the package actually exposes: Encode, called from several
// goroutines at once, over one shared input it must not disturb. Run under
// `go test -race`.
//
// The cold fill is raced by TestGeneratorPolyConcurrent above; by the time this
// runs the shared cache may hold any degree already, which is the state a
// long-lived program calls Encode in.
func TestEncodeConcurrent(t *testing.T) {
	const numECBytes = 5

	data := bitset.NewFromBase2String(
		"01000000 00011000 10101100 11000011 00000000")
	want := bitset.NewFromBase2String(
		"01000000 00011000 10101100 11000011 00000000 10000110 00001101 " +
			"00100010 10101110 00110000")

	failures := runConcurrently(func() []string {
		result := Encode(data, numECBytes)

		if !want.Equals(result) {
			return []string{fmt.Sprintf("encoded=%s, want %s", result.String(), want)}
		}

		return nil
	})

	for _, failure := range failures {
		t.Error(failure)
	}
}

// runConcurrently runs work in several goroutines released together, and
// returns everything they reported. Failures are collected rather than reported
// from the goroutines because t.Errorf may only be called from the goroutine
// running the test.
func runConcurrently(work func() []string) []string {
	const numGoroutines = 8

	start := make(chan struct{})
	reported := make(chan []string, numGoroutines)

	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			reported <- work()
		}()
	}

	close(start)
	wg.Wait()
	close(reported)

	var failures []string
	for f := range reported {
		failures = append(failures, f...)
	}

	return failures
}

// BenchmarkEncode encodes one block at the largest error correction size the QR
// Code spec uses: numECBytes=30 over 15 data codewords, the shape of 20 of the
// 81 blocks in a version 40 Highest symbol. Every one of those blocks pays this
// cost, so it is the per-block price the generator polynomial cache changes.
func BenchmarkEncode(b *testing.B) {
	const numECBytes = maxCachedGeneratorDegree

	data := bitset.New()
	for i := 0; i < 15; i++ {
		data.AppendByte(byte(i), 8)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		Encode(data, numECBytes)
	}
}
