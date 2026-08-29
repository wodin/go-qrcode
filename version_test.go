// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"testing"

	bitset "github.com/skip2/go-qrcode/bitset"
)

func TestFormatInfo(t *testing.T) {
	tests := []struct {
		level       RecoveryLevel
		maskPattern int

		expected uint32
	}{
		{ // L=01 M=00 Q=11 H=10
			Low,
			1,
			0x72f3,
		},
		{
			Medium,
			2,
			0x5e7c,
		},
		{
			High,
			3,
			0x3a06,
		},
		{
			Highest,
			4,
			0x0762,
		},
		{
			Low,
			5,
			0x6318,
		},
		{
			Medium,
			6,
			0x4f97,
		},
		{
			High,
			7,
			0x2bed,
		},
	}

	for i, test := range tests {
		v := getQRCodeVersion(test.level, 1)

		result := v.formatInfo(test.maskPattern)

		expected := bitset.New()
		expected.AppendUint32(test.expected, formatInfoLengthBits)

		if !expected.Equals(result) {
			t.Errorf("formatInfo test #%d got %s, expected %s", i, result.String(),
				expected.String())
		}
	}
}

func TestVersionInfo(t *testing.T) {
	tests := []struct {
		version  int
		expected uint32
	}{
		{
			7,
			0x007c94,
		},
		{
			10,
			0x00a4d3,
		},
		{
			20,
			0x0149a6,
		},
		{
			30,
			0x01ed75,
		},
		{
			40,
			0x028c69,
		},
	}

	for i, test := range tests {
		var v *qrCodeVersion

		v = getQRCodeVersion(Low, test.version)

		result := v.versionInfo()

		expected := bitset.New()
		expected.AppendUint32(test.expected, versionInfoLengthBits)

		if !expected.Equals(result) {
			t.Errorf("versionInfo test #%d got %s, expected %s", i, result.String(),
				expected.String())
		}
	}
}

func TestNumBitsToPadToCodeoword(t *testing.T) {
	tests := []struct {
		level   RecoveryLevel
		version int

		numDataBits int
		expected    int
	}{
		{
			Low,
			1,
			0,
			0,
		}, {
			Low,
			1,
			1,
			7,
		}, {
			Low,
			1,
			7,
			1,
		}, {
			Low,
			1,
			8,
			0,
		},
	}

	for i, test := range tests {
		var v *qrCodeVersion

		v = getQRCodeVersion(test.level, test.version)

		result := v.numBitsToPadToCodeword(test.numDataBits)

		if result != test.expected {
			t.Errorf("numBitsToPadToCodeword test %d (version=%d numDataBits=%d), got %d, expected %d",
				i, test.version, test.numDataBits, result, test.expected)
		}
	}
}

// TestMaxErrorCodewordsPerBlock pins the bound the reedsolomon package sizes its
// generator polynomial cache to.
//
// reedsolomon.maxCachedGeneratorDegree is 30 on the authority of ISO/IEC 18004
// table 9, and nothing in that package can check the claim: qrcode imports
// reedsolomon, not the other way round, so this table is the only statement of
// the spec's error correction sizes either package can reach.
//
// A wrong bound is otherwise invisible. Every symbol would still encode
// identically — a degree past the end of the cache is built afresh and encodes
// correctly — the degrees past it would simply rebuild their polynomial once
// per block again, failing no test and changing no output.
func TestMaxErrorCodewordsPerBlock(t *testing.T) {
	// Kept in step by hand with reedsolomon.maxCachedGeneratorDegree, which is
	// unexported and should stay that way: it sizes a cache, and no caller of
	// the package has any business reading it.
	const maxCachedGeneratorDegree = 30

	for _, v := range versions {
		for _, shape := range blockShapes(v) {
			if shape.numErrorCodewords() > maxCachedGeneratorDegree {
				t.Errorf("version %d level %d: %d error correction codewords per "+
					"block, past the %d reedsolomon caches a generator polynomial for",
					v.version, v.level, shape.numErrorCodewords(),
					maxCachedGeneratorDegree)
			}
		}
	}
}
