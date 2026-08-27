// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

// occlusion is what seating a knockout costs one symbol: the codewords a
// decoder will read wrongly, counted per block, and the first protected
// function pattern module the knockout covers, if it covers any.
//
// Damage is held per block because error correction is spent per block: a
// block that loses more codewords than it can correct stops the whole symbol
// decoding, however lightly the other blocks got off (ADR-0001).
type occlusion struct {
	// occludesFunctionPattern reports whether the knockout covers a function
	// pattern a logo may not cover, and functionPattern is the first such
	// module, reading left to right and top to bottom.
	occludesFunctionPattern bool
	functionPattern         modulePosition

	// damaged[b] is the number of distinct codewords of block b the knockout
	// covers. blocks[b] is that block's shape.
	damaged []int
	blocks  []blockShape
}

// survivable reports whether a symbol with this occlusion is still safe to
// put in front of a scanner.
//
// Every block must keep at least half its correction capacity: the half we
// spend seats the logo, and the half we withhold pays for print bleed, camera
// blur, glare and a folded page. Covering a protected function pattern is
// never survivable, because a decoder needs those before error correction can
// run at all.
func (o occlusion) survivable() bool {
	if o.occludesFunctionPattern {
		return false
	}

	for b, damaged := range o.damaged {
		if 2*damaged > o.blocks[b].correctionCapacity() {
			return false
		}
	}

	return true
}

// worstBlock returns the index of the block left with the least correction
// capacity to spare — the block that decides whether the occlusion is
// survivable, and so the one worth reporting to a caller.
//
// Blocks of a version can differ in size, so blocks are ranked by what they
// have left rather than by what they have lost.
func (o occlusion) worstBlock() int {
	worst := 0

	for b := range o.damaged {
		if o.spare(b) < o.spare(worst) {
			worst = b
		}
	}

	return worst
}

// spare returns how much of block b's half share of its correction capacity
// the occlusion leaves unspent, in half codewords.
func (o occlusion) spare(b int) int {
	return o.blocks[b].correctionCapacity() - 2*o.damaged[b]
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

// occlusionOf returns what seating k costs the symbol.
func (f *logoFit) occlusionOf(k knockout) occlusion {
	o := occlusion{
		damaged: make([]int, f.layout.numBlocks()),
		blocks:  f.layout.blocks,
	}

	// A codeword is damaged by the first of its modules the knockout covers
	// and no further by the other seven, so each one is counted once.
	counted := make([]bool, f.layout.numCodewords())

	clipped := k.clip(f.symbolSize)

	for y := clipped.min.y; y < clipped.max.y; y++ {
		for x := clipped.min.x; x < clipped.max.x; x++ {
			if !f.protected.empty(x, y) {
				if !o.occludesFunctionPattern {
					o.occludesFunctionPattern = true
					o.functionPattern = modulePosition{x: x, y: y}
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
			o.damaged[f.layout.blockOf(n)]++
		}
	}

	return o
}

// largestSurvivingWidth returns the width in modules of the widest knockout
// the symbol survives, given margin modules of clear space around the logo,
// or 0 if the symbol survives no logo at all.
//
// A wider knockout covers every module a narrower one does, so it can only
// damage more codewords and cover more function patterns. Survival is
// therefore monotonic in width, and the widest survivor is the last one
// before the first failure.
func (f *logoFit) largestSurvivingWidth(margin int) int {
	widest := 0

	// A logo of any size at all knocks out its centre module, so the
	// narrowest knockout there is is that module plus the margin.
	for width := 1 + 2*margin; width <= f.symbolSize; width += 2 {
		if !f.occlusionOf(knockoutOfWidth(f.symbolSize, width)).survivable() {
			break
		}

		widest = width
	}

	return widest
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
