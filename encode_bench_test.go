// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"strings"
	"testing"
)

// The existing BenchmarkQRCode* benchmarks call New only, which chooses a
// version and encodes the data but stops there: encode(), and with it the
// placement path and the block interleave, runs lazily on the first Bitmap or
// Image call. These two drive a whole encode, which is what #12 measures.

func BenchmarkEncodeURL(b *testing.B) {
	const content = "http://www.example.org"

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		q, err := New(content, Medium)
		if err != nil {
			b.Fatal(err)
		}

		q.Bitmap()
	}
}

func BenchmarkEncodeMaximumSize(b *testing.B) {
	// 7089 is the maximum encodable number of numeric digits: version 40.
	content := strings.Repeat("0", 7089)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		q, err := New(content, Low)
		if err != nil {
			b.Fatal(err)
		}

		q.Bitmap()
	}
}

// BenchmarkEncodeBlocksOnly isolates the block split, error correction and
// interleave from the eight-mask placement loop around it, so the two halves
// of an encode can be weighed against each other. encodeBlocks reads q.data
// but does not mutate it, so one setup serves every iteration.
func BenchmarkEncodeBlocksOnly(b *testing.B) {
	q, err := New(strings.Repeat("0", 7089), Low)
	if err != nil {
		b.Fatal(err)
	}

	q.addTerminatorBits(q.version.numTerminatorBitsRequired(q.data.Len()))
	q.addPadding()

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		q.encodeBlocks()
	}
}
