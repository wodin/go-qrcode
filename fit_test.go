// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import "testing"

// someVersions is a spread of versions wide enough to exercise every block
// layout shape without running the slower fit tests 160 times: version 1 has
// a single block, 7 is the first with version info and a centred alignment
// pattern, 14 and 26 split into many blocks of two different sizes, and 40 is
// the largest symbol there is.
var someVersions = []int{1, 7, 14, 26, 40}

// forSomeSymbols runs test over someVersions at everyLevel.
func forSomeSymbols(t *testing.T, test func(t *testing.T, v qrCodeVersion)) {
	t.Helper()

	for _, versionNumber := range someVersions {
		for _, level := range everyLevel {
			v := getQRCodeVersion(level, versionNumber)

			t.Run(versionLevelName(versionNumber, level), func(t *testing.T) {
				test(t, *v)
			})
		}
	}
}

func TestOcclusionCountsCodewordsNotModules(t *testing.T) {
	forSomeSymbols(t, func(t *testing.T, v qrCodeVersion) {
		fit := newLogoFit(v)

		for width := 1; width <= 25; width += 2 {
			d := fit.damageFrom(knockoutOfWidth(v.symbolSize(), width))

			damaged := 0
			for _, block := range d.blocks {
				damaged += block.damaged
			}

			// A codeword owns eight modules, so covering m data region
			// modules damages between m/8 and m codewords: fewer than m/8 is
			// impossible, and more than m would mean counting a codeword
			// twice.
			modules := occludedDataRegionModules(v, width)

			if damaged > modules {
				t.Errorf("width %d: %d codewords damaged by %d data region modules, want no more than one each",
					width, damaged, modules)
			}

			if damaged < (modules+7)/8 {
				t.Errorf("width %d: %d codewords damaged by %d data region modules, want at least %d",
					width, damaged, modules, (modules+7)/8)
			}
		}
	})
}

// occludedDataRegionModules counts the modules of v's data region a width
// module wide centred knockout covers, counting them straight off the symbol
// rather than through the layout the production code uses.
func occludedDataRegionModules(v qrCodeVersion, width int) int {
	function := functionPatternSymbol(v)
	k := knockoutOfWidth(v.symbolSize(), width).clip(v.symbolSize())

	modules := 0

	for y := k.min.y; y < k.max.y; y++ {
		for x := k.min.x; x < k.max.x; x++ {
			if function.empty(x, y) {
				modules++
			}
		}
	}

	return modules
}

func TestOcclusionGrowsWithTheKnockout(t *testing.T) {
	forSomeSymbols(t, func(t *testing.T, v qrCodeVersion) {
		fit := newLogoFit(v)
		previous := make([]int, len(blockShapes(v)))

		for width := 1; width <= v.symbolSize(); width += 2 {
			d := fit.damageFrom(knockoutOfWidth(v.symbolSize(), width))

			for b, block := range d.blocks {
				if block.damaged < previous[b] {
					t.Fatalf("width %d: block %d lost %d codewords, having lost %d at the width below",
						width, b, block.damaged, previous[b])
				}

				previous[b] = block.damaged
			}
		}
	})
}

func TestKnockoutCoveringTheWholeSymbolOccludesAFunctionPattern(t *testing.T) {
	forSomeSymbols(t, func(t *testing.T, v qrCodeVersion) {
		fit := newLogoFit(v)
		d := fit.damageFrom(knockoutOfWidth(v.symbolSize(), v.symbolSize()))

		if !d.occludesFunctionPattern {
			t.Fatal("a knockout covering the whole symbol occludes no function pattern")
		}

		if d.survivable() {
			t.Error("a knockout covering the whole symbol is survivable")
		}

		p := d.functionPattern
		if protectedFunctionPatternSymbol(v).empty(p.x, p.y) {
			t.Errorf("reported module (%d,%d) is not a protected function pattern", p.x, p.y)
		}
	})
}

func TestSurvivableKnockoutLeavesHalfTheCorrectionCapacity(t *testing.T) {
	forSomeSymbols(t, func(t *testing.T, v qrCodeVersion) {
		fit := newLogoFit(v)

		for width := 1; width <= v.symbolSize(); width += 2 {
			d := fit.damageFrom(knockoutOfWidth(v.symbolSize(), width))

			if !d.survivable() {
				break
			}

			for b, block := range d.blocks {
				capacity := blockShapes(v)[b].correctionCapacity()

				if 2*block.damaged > capacity {
					t.Fatalf("width %d: block %d loses %d of its %d correctable codewords, want at most half",
						width, b, block.damaged, capacity)
				}
			}
		}
	})
}

func TestLargestSurvivingWidthIsTheLastThatSurvives(t *testing.T) {
	forSomeSymbols(t, func(t *testing.T, v qrCodeVersion) {
		fit := newLogoFit(v)

		for _, margin := range []int{0, 1, 3} {
			width := fit.largestSurvivingWidth(margin)
			smallest := 2*margin + 1

			if width == 0 {
				if fit.damageFrom(knockoutOfWidth(v.symbolSize(), smallest)).survivable() {
					t.Errorf("margin %d: no width survives, but the smallest knockout does", margin)
				}

				continue
			}

			if width < smallest || width%2 != 1 {
				t.Fatalf("margin %d: largest surviving width is %d, want an odd width of at least %d",
					margin, width, smallest)
			}

			if !fit.damageFrom(knockoutOfWidth(v.symbolSize(), width)).survivable() {
				t.Errorf("margin %d: the largest surviving width %d does not survive", margin, width)
			}

			if fit.damageFrom(knockoutOfWidth(v.symbolSize(), width+2)).survivable() {
				t.Errorf("margin %d: width %d survives, so %d is not the largest",
					margin, width+2, width)
			}
		}
	})
}

func TestMaxScaleReproducesTheLargestSurvivingKnockout(t *testing.T) {
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		fit := newLogoFit(v)

		for _, margin := range []int{0, 1, 2, 3, 10} {
			scale := fit.maxScale(margin)
			width := fit.largestSurvivingWidth(margin)

			if width == 0 {
				if scale != 0 {
					t.Errorf("margin %d: max scale is %v with no surviving knockout, want 0",
						margin, scale)
				}

				continue
			}

			// The whole point of the reported maximum: used as a scale, it
			// must knock out exactly the widest square that survives, not the
			// next one up.
			if got := newKnockout(v.symbolSize(), scale, margin); got.width() != width {
				t.Errorf("margin %d: max scale %v knocks out %d modules, want the largest surviving %d",
					margin, scale, got.width(), width)
			}
		}
	})
}

func TestWorstBlockIsTheBlockLeftWithLeastToSpare(t *testing.T) {
	forSomeSymbols(t, func(t *testing.T, v qrCodeVersion) {
		fit := newLogoFit(v)

		for width := 1; width <= v.symbolSize(); width += 2 {
			d := fit.damageFrom(knockoutOfWidth(v.symbolSize(), width))
			worst := d.worstBlock()

			for b, block := range d.blocks {
				if block.spare() < d.blocks[worst].spare() {
					t.Fatalf("width %d: block %d has %d codewords to spare, less than block %d, which was called the worst with %d",
						width, b, block.spare(), worst, d.blocks[worst].spare())
				}
			}

			// A symbol survives exactly while its worst block does, so the
			// block singled out for the caller is the one that decided it.
			if got, want := d.blocks[worst].spare() >= 0, d.survivable(); got != want &&
				!d.occludesFunctionPattern {

				t.Fatalf("width %d: the worst block is within budget = %v, but the symbol survives = %v",
					width, got, want)
			}
		}
	})
}

func TestBlockBudgetIsHalfTheCorrectionCapacity(t *testing.T) {
	forEveryVersion(t, func(t *testing.T, v qrCodeVersion) {
		for b, shape := range blockShapes(v) {
			damage := blockDamage{shape: shape}

			// ADR-0001 spends at most half of t, where t is half the block's
			// error correction codewords.
			if want := shape.numErrorCodewords() / 2 / 2; damage.budget() != want {
				t.Fatalf("block %d of %d error correction codewords budgets %d damaged codewords, want %d",
					b, shape.numErrorCodewords(), damage.budget(), want)
			}
		}
	})
}

func TestAHigherRecoveryLevelDoesNotAlwaysAcceptALargerLogo(t *testing.T) {
	// A higher recovery level carries more error correction as a fraction of
	// the symbol, but splits the symbol into more, smaller blocks — and
	// correction capacity is held per block, so a block's budget can fall as
	// the percentage rises.
	//
	// This is pinned because a refusal's advice depends on it: a message
	// telling a caller to raise the recovery level would be wrong here, and
	// the inversion is surprising enough that someone would otherwise
	// "simplify" the message back.
	high := newLogoFit(*getQRCodeVersion(High, 15)).maxScale(1)
	highest := newLogoFit(*getQRCodeVersion(Highest, 15)).maxScale(1)

	if !(highest < high) {
		t.Errorf("version 15 accepts a scale %v logo at High and %v at Highest, want Highest to accept less",
			high, highest)
	}
}
