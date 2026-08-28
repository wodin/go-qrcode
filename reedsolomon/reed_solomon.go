// go-qrcode
// Copyright 2014 Tom Harwood

// Package reedsolomon provides error correction encoding for QR Code 2005.
//
// QR Code 2005 uses a Reed-Solomon error correcting code to detect and correct
// errors encountered during decoding.
//
// The generated RS codes are systematic, and consist of the input data with
// error correction bytes appended.
package reedsolomon

import (
	"log"
	"sync"

	bitset "github.com/skip2/go-qrcode/bitset"
)

// Encode data for QR Code 2005 using the appropriate Reed-Solomon code.
//
// numECBytes is the number of error correction bytes to append, and is
// determined by the target QR Code's version and error correction level.
//
// ISO/IEC 18004 table 9 specifies the numECBytes required. e.g. a 1-L code has
// numECBytes=7.
func Encode(data *bitset.Bitset, numECBytes int) *bitset.Bitset {
	// Create a polynomial representing |data|.
	//
	// The bytes are interpreted as the sequence of coefficients of a polynomial.
	// The last byte's value becomes the x^0 coefficient, the second to last
	// becomes the x^1 coefficient and so on.
	ecpoly := newGFPolyFromData(data)
	ecpoly = gfPolyMultiply(ecpoly, newGFPolyMonomial(gfOne, numECBytes))

	// Pick the generator polynomial.
	generator := rsGeneratorPoly(numECBytes)

	// Generate the error correction bytes.
	remainder := gfPolyRemainder(ecpoly, generator)

	// Combine the data & error correcting bytes.
	// The mathematically correct answer is:
	//
	//	result := gfPolyAdd(ecpoly, remainder).
	//
	// The encoding used by QR Code 2005 is slightly different this result: To
	// preserve the original |data| bit sequence exactly, the data and remainder
	// are combined manually below. This ensures any most significant zero bits
	// are preserved (and not optimised away).
	result := bitset.Clone(data)
	result.AppendBytes(remainder.data(numECBytes))

	return result
}

// maxCachedGeneratorDegree is the largest number of error correction bytes
// ISO/IEC 18004 table 9 asks of a single block: 30, for version 40 Highest
// among others. Caching up to it covers all 13 distinct degrees a QR Code
// symbol can call for, which run from 7 up.
//
// The bound is deliberate rather than incidental. Encode is exported and takes
// numECBytes from its caller, so a cache without a bound would grow on a key
// the package does not control. Reed-Solomon itself is not limited to degree 30
// — the tests below build degree 68 — and a degree past the table still
// encodes correctly; it is simply rebuilt per call, as every degree was before.
const maxCachedGeneratorDegree = 30

// generatorPolyCache memoises rsGeneratorPoly by degree.
//
// Every block of a symbol has the same numECBytes, so without this a version 40
// Highest encode rebuilds one identical degree-30 polynomial 81 times, at
// nearly 1900 allocations a time.
//
// The table fills lazily and per degree: a program that encodes a single symbol
// pays for the one polynomial it uses, and importers that never encode pay
// nothing. Each entry's sync.Once is what keeps Encode safe to call from
// several goroutines at once, as it has always been.
var generatorPolyCache [maxCachedGeneratorDegree + 1]cachedGeneratorPoly

// cachedGeneratorPoly is one entry of generatorPolyCache: a generator
// polynomial and the sync.Once that builds it on first use.
type cachedGeneratorPoly struct {
	once sync.Once
	poly gfPoly
}

// rsGeneratorPoly returns the Reed-Solomon generator polynomial with |degree|.
func rsGeneratorPoly(degree int) gfPoly {
	if degree < 2 {
		log.Panic("degree < 2")
	}

	if degree > maxCachedGeneratorDegree {
		return buildRSGeneratorPoly(degree)
	}

	cached := &generatorPolyCache[degree]
	cached.once.Do(func() {
		cached.poly = buildRSGeneratorPoly(degree)
	})

	// A cached polynomial outlives every call that receives it, so callers get a
	// copy. gfPoly is a value type over a slice: handing out the cached instance
	// would let one caller's write reach every later caller.
	//
	// No caller writes to one today — gfPolyRemainder, gfPolyMultiply and
	// gfPolyAdd each build a fresh polynomial, and normalised only reslices its
	// value receiver. The copy costs one allocation per call against the ~1900 it
	// saves, which is worth paying to keep that a local property of those three
	// functions rather than a standing obligation on the whole package.
	return cached.poly.clone()
}

// buildRSGeneratorPoly builds the Reed-Solomon generator polynomial with |degree|.
//
// The generator polynomial is calculated as:
// (x + a^0)(x + a^1)...(x + a^degree-1)
func buildRSGeneratorPoly(degree int) gfPoly {
	generator := gfPoly{term: []gfElement{1}}

	for i := 0; i < degree; i++ {
		nextPoly := gfPoly{term: []gfElement{gfExpTable[i], 1}}
		generator = gfPolyMultiply(generator, nextPoly)
	}

	return generator
}
