// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

// blockDamage is what a knockout costs one error correction block.
type blockDamage struct {
	shape blockShape

	// damaged is the number of distinct codewords of the block the knockout
	// covers.
	damaged int
}

// budget returns the number of damaged codewords the block tolerates: half
// its correction capacity.
//
// The other half is deliberately withheld. It pays for print bleed, camera
// blur, glare and a folded page — damage a unit test never sees and a
// scanner cannot avoid — and it is the one tunable number in the design
// (ADR-0001).
func (d blockDamage) budget() int {
	return d.shape.correctionCapacity() / 2
}

// spare returns how many further codewords the block could lose and stay
// within budget, negative once the budget is overspent.
func (d blockDamage) spare() int {
	return d.budget() - d.damaged
}

// knockoutDamage is what seating a knockout costs one symbol: the codewords a
// decoder will read wrongly, counted per block, and the first protected
// function pattern the knockout occludes, if it occludes any.
//
// Damage is held per block because error correction is spent per block. A
// block that loses more codewords than it can correct stops the whole symbol
// decoding, however lightly the other blocks got off — so a total, or a mean,
// would hide exactly the failure worth catching (ADR-0001).
type knockoutDamage struct {
	// occludesFunctionPattern reports whether the knockout covers a function
	// pattern a logo may not cover, and functionPattern is the first such
	// module, reading left to right and top to bottom.
	occludesFunctionPattern bool
	functionPattern         modulePosition

	// blocks[b] is what the knockout costs block b. Never empty: every
	// version splits into at least one block.
	blocks []blockDamage
}

// survivable reports whether a symbol damaged like this is still safe to put
// in front of a scanner.
//
// Every block must be within its budget, and no protected function pattern
// may be occluded: a decoder reads those before error correction can run at
// all, so no budget can pay for them.
func (d knockoutDamage) survivable() bool {
	return !d.occludesFunctionPattern && d.blocks[d.worstBlock()].spare() >= 0
}

// worstBlock returns the index of the block left with the least to spare —
// the block that decides whether the damage is survivable, and so the one
// worth reporting to a caller.
//
// Blocks of a version can differ in size, so blocks are ranked by what they
// have left rather than by what they have lost.
func (d knockoutDamage) worstBlock() int {
	worst := 0

	for b := range d.blocks {
		if d.blocks[b].spare() < d.blocks[worst].spare() {
			worst = b
		}
	}

	return worst
}

// logoFit judges knockouts against the error correction budget of a single
// version and recovery level.
//
// Both mappings it needs — module to codeword and codeword to block — depend
// on the version and level alone, not on the content encoded, so one logoFit
// answers for every symbol of that version and level.
type logoFit struct {
	symbolSize int

	layout    *codewordLayout
	protected *symbol
}

// newLogoFit returns the fit judge for version v.
func newLogoFit(v qrCodeVersion) *logoFit {
	return &logoFit{
		symbolSize: v.symbolSize(),
		layout:     newCodewordLayout(v),
		protected:  protectedFunctionPatternSymbol(v),
	}
}

// damageFrom returns what seating k costs the symbol.
func (f *logoFit) damageFrom(k knockout) knockoutDamage {
	d := knockoutDamage{blocks: make([]blockDamage, f.layout.numBlocks())}

	for b := range d.blocks {
		d.blocks[b].shape = f.layout.block(b)
	}

	// A codeword is damaged by the first of its modules the knockout covers
	// and no further by the other seven, so each one is counted once.
	counted := make([]bool, f.layout.numCodewords())

	clipped := k.clip(f.symbolSize)

	for y := clipped.min.y; y < clipped.max.y; y++ {
		for x := clipped.min.x; x < clipped.max.x; x++ {
			if !f.protected.empty(x, y) {
				if !d.occludesFunctionPattern {
					d.occludesFunctionPattern = true
					d.functionPattern = modulePosition{x: x, y: y}
				}

				continue
			}

			// An alignment pattern module or a remainder bit carries no
			// codeword, and costs the budget nothing.
			n := f.layout.codewordAt(x, y)
			if n == noCodeword || counted[n] {
				continue
			}

			counted[n] = true
			d.blocks[f.layout.blockOf(n)].damaged++
		}
	}

	return d
}

// largestSurvivingWidth returns the width in modules of the widest knockout
// the symbol survives, given margin modules of clear space around the logo,
// or 0 if the symbol survives no logo at all.
//
// A wider knockout covers every module a narrower one does, so it can only
// damage more codewords and occlude more function patterns. Survival is
// therefore monotonic in width, and the widest survivor is the last one
// before the first failure.
func (f *logoFit) largestSurvivingWidth(margin int) int {
	widest := 0

	// A negative margin is not clear space and no knockout has one, so there
	// is no width to search: nothing fits, as it does not at a margin wider
	// than the symbol.
	if margin < 0 {
		return 0
	}

	// A logo of any size at all knocks out its centre module, so the
	// narrowest knockout there is is that module plus the margin.
	for width := 1 + 2*margin; width <= f.symbolSize; width += 2 {
		if !f.damageFrom(knockoutOfWidth(f.symbolSize, width)).survivable() {
			break
		}

		widest = width
	}

	return widest
}

// smallestScale returns the scale of the narrowest logo there is: one module
// wide, whatever the margin around it.
//
// It is the scale a symbol that fits nothing is judged at, so that "nothing
// fits" is reported as the ordinary refusal of a real logo — which block ran
// out of capacity, or which function pattern was buried — rather than as a
// bare sentence.
func (f *logoFit) smallestScale() float64 {
	return 1 / float64(f.symbolSize)
}

// maxScale returns the largest logo scale, as a fraction of the symbol's
// width, the symbol survives at margin modules of clear space, or 0 if it
// survives no logo at all.
//
// Because the knockout snaps to whole modules, acceptance is a staircase in
// scale rather than a slope, and the maximum is the top of the last surviving
// step: the scale whose logo is exactly the widest surviving knockout less
// its two margins.
func (f *logoFit) maxScale(margin int) float64 {
	width := f.largestSurvivingWidth(margin)

	if width == 0 {
		return 0
	}

	return float64(width-2*margin) / float64(f.symbolSize)
}
