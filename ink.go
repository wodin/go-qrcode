// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"image"
	"image/draw"
)

// ink is the set of modules a logo's pixels cover, within the knockout
// seating it.
//
// A module is inked if any pixel of the logo over it has non-zero alpha. Any
// alpha at all counts: a module the logo partly covers is read just as
// wrongly as one it covers entirely (ADR-0001), and a translucent pixel has
// to composite over the background rather than over a live module.
//
// It is what a render clears, once dilated by the margin — never what the
// budget charges. The charge stays the whole knockout however little of it is
// inked (ADR-0008), which is why nothing here can reach the fit.
type ink struct {
	// within is the knockout the set is indexed inside. A module outside it
	// is never inked, so the clearing is always a subset of what was charged.
	within knockout

	// inked holds one flag per module of within, in row-major order from
	// within.min. A grid rather than a set of positions because the region is
	// small, bounded by the knockout, and dilation reads its neighbours.
	inked []bool
}

// newInk returns the modules of within that logo covers, where logo is drawn
// into the pixels seat covers with its own bounds' origin at seat.Min — the
// resampled image withLogo composites, not the caller's original.
//
// It is measured on the resampled image so that what is cleared and what is
// drawn cannot disagree: the box filter of ADR-0003 can round a hairline's
// alpha to zero, and a stroke drawn over a module that was charged for anyway
// is cosmetic, whereas a module cleared with nothing drawn into it is a hole.
func newInk(logo image.Image, seat image.Rectangle, within knockout,
	scale moduleScale) ink {

	inked := ink{
		within: within,
		inked:  make([]bool, within.width()*within.width()),
	}

	for y := within.min.y; y < within.max.y; y++ {
		for x := within.min.x; x < within.max.x; x++ {
			module := scale.pixelsOfSymbolModules(
				modulePosition{x: x, y: y},
				modulePosition{x: x + 1, y: y + 1})

			inked.set(modulePosition{x: x, y: y},
				anyPixelInked(logo, module.Intersect(seat), seat.Min))
		}
	}

	return inked
}

// anyPixelInked reports whether logo is anywhere but fully transparent over
// the pixels of covered, where the logo's own bounds begin at origin.
func anyPixelInked(logo image.Image, covered image.Rectangle,
	origin image.Point) bool {

	for y := covered.Min.Y; y < covered.Max.Y; y++ {
		for x := covered.Min.X; x < covered.Max.X; x++ {
			if _, _, _, alpha := logo.At(x-origin.X, y-origin.Y).RGBA(); alpha > 0 {
				return true
			}
		}
	}

	return false
}

// covers reports whether the logo inks the module at p. A module outside the
// knockout is never inked, so a caller need not clip before asking.
func (i ink) covers(p modulePosition) bool {
	offset, ok := i.offsetOf(p)

	return ok && i.inked[offset]
}

// set marks the module at p. A module outside the knockout is silently
// dropped, which is what clips a dilation to what the budget charged.
func (i *ink) set(p modulePosition, inked bool) {
	if offset, ok := i.offsetOf(p); ok {
		i.inked[offset] = inked
	}
}

// offsetOf returns p's index into inked, and whether p lies inside the
// knockout at all.
func (i ink) offsetOf(p modulePosition) (int, bool) {
	if p.x < i.within.min.x || p.x >= i.within.max.x ||
		p.y < i.within.min.y || p.y >= i.within.max.y {

		return 0, false
	}

	return (p.y-i.within.min.y)*i.within.width() + (p.x - i.within.min.x), true
}

// dilated returns the ink grown by margin modules in every direction, clipped
// to the knockout.
//
// The margin's job — keeping a scanner's binarizer from smearing the logo's
// edge into the modules beside it — is local to each stroke rather than to
// the bounding square, so it is grown here and not around the whole region
// (ADR-0008). The neighbourhood is square, as newKnockout's own margin is.
func (i ink) dilated(margin int) ink {
	grown := ink{
		within: i.within,
		inked:  make([]bool, len(i.inked)),
	}

	for y := i.within.min.y; y < i.within.max.y; y++ {
		for x := i.within.min.x; x < i.within.max.x; x++ {
			p := modulePosition{x: x, y: y}

			if i.nearInk(p, margin) {
				grown.set(p, true)
			}
		}
	}

	return grown
}

// nearInk reports whether any module within margin modules of p is inked.
func (i ink) nearInk(p modulePosition, margin int) bool {
	for dy := -margin; dy <= margin; dy++ {
		for dx := -margin; dx <= margin; dx++ {
			if i.covers(modulePosition{x: p.x + dx, y: p.y + dy}) {
				return true
			}
		}
	}

	return false
}

// fill paints every module of the ink into dst with src, a module at a time.
//
// Per module rather than per rectangle because that is the whole point: an
// inked set is not a rectangle, and the modules it leaves out are the ones a
// decoder gets to read.
func (i ink) fill(dst draw.Image, src image.Image, scale moduleScale) {
	for y := i.within.min.y; y < i.within.max.y; y++ {
		for x := i.within.min.x; x < i.within.max.x; x++ {
			p := modulePosition{x: x, y: y}

			if !i.covers(p) {
				continue
			}

			draw.Draw(dst,
				scale.pixelsOfSymbolModules(p,
					modulePosition{x: x + 1, y: y + 1}),
				src, image.Point{}, draw.Src)
		}
	}
}
