// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"log"
	"math"
)

// moduleSnapTolerance is how close to a module boundary a knockout edge may
// fall and still be treated as lying exactly on it.
//
// It exists so that a scale derived from a module count survives the round
// trip back through newKnockout. maxLogoScale reports the scale of the widest
// knockout that fits, computed as a ratio of whole modules; without a
// tolerance a rounding error of one part in 2^52 would snap that scale
// outwards to the next module and make the reported maximum unusable.
const moduleSnapTolerance = 1e-9

// knockout is the square of modules cleared to background colour to seat a
// logo: the logo itself plus its surrounding margin, snapped outwards to
// whole modules.
//
// It is the knockout, not the logo's own extent, that counts as damage — a
// module the logo only partly covers is read just as wrongly as one it covers
// entirely (ADR-0001).
type knockout struct {
	// min is the top left module of the square; max is one past its bottom
	// right, in the same module coordinates as symbol's get and set.
	min modulePosition
	max modulePosition
}

// newKnockout returns the knockout of a logo scale symbol widths across,
// centred in a symbolSize module symbol, with margin modules of clear space
// on every side.
//
// A symbol is always an odd number of modules wide, so its centre is the
// centre of the middle module and the knockout is an odd number of modules
// wide about it. That is what keeps the knockout centred as scale varies: it
// grows a ring at a time rather than drifting half a module back and forth.
func newKnockout(symbolSize int, scale float64, margin int) knockout {
	// Half the knockout's width, in modules, before snapping. The middle
	// module contributes half a module to each side, so the ring of whole
	// modules around it is half a module narrower than this.
	half := scale*float64(symbolSize)/2 + float64(margin)

	rings := int(math.Ceil(half - 0.5 - moduleSnapTolerance))
	if rings < 0 {
		rings = 0
	}

	return knockoutOfWidth(symbolSize, 1+2*rings)
}

// knockoutOfWidth returns the width module wide knockout centred in a
// symbolSize module symbol. width must be odd, since only an odd width can be
// centred on the symbol's middle module.
func knockoutOfWidth(symbolSize int, width int) knockout {
	if width%2 != 1 {
		log.Panicf("bug: knockout width is %d (expected an odd number)", width)
	}

	centre := (symbolSize - 1) / 2
	rings := (width - 1) / 2

	return knockout{
		min: modulePosition{x: centre - rings, y: centre - rings},
		max: modulePosition{x: centre + rings + 1, y: centre + rings + 1},
	}
}

// width returns the knockout's width in modules.
func (k knockout) width() int {
	return k.max.x - k.min.x
}

// clip returns the part of k lying within a symbolSize module symbol.
//
// Only an absurd scale reaches past the symbol's edge, and such a knockout is
// refused for burying the finder patterns long before the clipping matters.
// Clipping is what stops the refusal being a panic.
func (k knockout) clip(symbolSize int) knockout {
	return knockout{
		min: modulePosition{x: max(k.min.x, 0), y: max(k.min.y, 0)},
		max: modulePosition{
			x: min(k.max.x, symbolSize),
			y: min(k.max.y, symbolSize),
		},
	}
}
