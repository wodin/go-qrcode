// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"image"
	"image/draw"
	"sort"
)

// moduleScale maps between the pixels of a rendered image and the modules
// drawn in it.
//
// Both spans cover the symbol and its quiet zone together, so a module
// coordinate here counts from the corner of the quiet zone rather than the
// corner of the symbol. A knockout counts from the corner of the symbol, and
// the difference is quietZoneSize.
type moduleScale struct {
	// pixels is the image's width, and modulesPerPixel the width of one
	// pixel in modules. The ratio is held rather than the module count
	// because moduleAt multiplies by it once per pixel.
	pixels          int
	modulesPerPixel float64
}

// newModuleScale returns the scale drawing modules modules across pixels
// pixels.
func newModuleScale(pixels int, modules int) moduleScale {
	return moduleScale{
		pixels:          pixels,
		modulesPerPixel: float64(modules) / float64(pixels),
	}
}

// moduleAt returns the module drawn at pixel.
func (s moduleScale) moduleAt(pixel int) int {
	return int(float64(pixel) * s.modulesPerPixel)
}

// pixelsOf returns the half-open range of pixels drawing the half-open range
// of modules [first, last), clamped to the image.
//
// It is the inverse of moduleAt, and is found by searching moduleAt rather
// than by dividing the other way about. The two must agree exactly — a pixel
// the search leaves out of a knockout is a pixel the drawing loop paints as a
// module — and a search of the mapping itself cannot disagree with it,
// whereas arithmetic that rounds differently in the last bit can.
func (s moduleScale) pixelsOf(first int, last int) (int, int) {
	return s.firstPixelOf(first), s.firstPixelOf(last)
}

// firstPixelOf returns the first pixel drawing module, or the image's width
// if module is past its far edge.
func (s moduleScale) firstPixelOf(module int) int {
	return sort.Search(s.pixels, func(pixel int) bool {
		return s.moduleAt(pixel) >= module
	})
}

// withLogo returns symbol with the attached logo seated in it: the knockout
// cleared to the background colour, and the logo drawn over that.
//
// The result is full colour. A logo does not fit the two colour palette that
// keeps a plain QR Code's PNG small, so a symbol carrying one gives that
// palette up — which is why nothing is converted unless a logo was attached.
func (q *QRCode) withLogo(symbol *image.Paletted, scale moduleScale) image.Image {
	img := image.NewRGBA(symbol.Bounds())
	draw.Draw(img, img.Bounds(), symbol, symbol.Bounds().Min, draw.Src)

	k := newKnockout(q.symbol.symbolSize, q.logoOptions.Scale,
		q.logoOptions.Margin)

	cleared := q.pixelsOfModules(k.min, k.max, scale)
	draw.Draw(img, cleared, image.NewUniform(q.BackgroundColor),
		image.Point{}, draw.Src)

	seat := q.logoSeat(k, cleared, scale)
	draw.Draw(img, seat, resample(q.logo, seat.Dx(), seat.Dy()),
		image.Point{}, draw.Over)

	return img
}

// pixelsOfModules returns the pixels drawing the half-open square of symbol
// modules [first, last).
func (q *QRCode) pixelsOfModules(first modulePosition, last modulePosition,
	scale moduleScale) image.Rectangle {

	quiet := q.symbol.quietZoneSize

	left, right := scale.pixelsOf(first.x+quiet, last.x+quiet)
	top, bottom := scale.pixelsOf(first.y+quiet, last.y+quiet)

	return image.Rect(left, top, right, bottom)
}

// logoSeat returns the pixels the logo itself is drawn into, given the
// knockout k cleared for it and the pixels cleared covers.
//
// The logo is drawn the fraction of the symbol's width that
// LogoOptions.Scale promises rather than stretched to fill the knockout. The
// knockout snaps outwards to an odd number of whole modules, so it is
// generally the wider of the two, and the difference shows as more
// background around the logo than the margin alone asked for.
//
// That fraction is taken of the clear area — the knockout less its margin —
// rather than of the symbol's own width in pixels. The two are the same
// length, because the clear area is by construction at least as wide as the
// logo asked for, but taking it this way means a seat that rounds outwards
// still rounds inside the area cleared for it rather than into the modules
// beyond.
func (q *QRCode) logoSeat(k knockout, cleared image.Rectangle,
	scale moduleScale) image.Rectangle {

	margin := q.logoOptions.Margin

	clear := q.pixelsOfModules(
		modulePosition{x: k.min.x + margin, y: k.min.y + margin},
		modulePosition{x: k.max.x - margin, y: k.max.y - margin},
		scale)

	wanted := q.logoOptions.Scale * float64(q.symbol.symbolSize)
	available := float64(k.width() - 2*margin)

	side := int(float64(min(clear.Dx(), clear.Dy())) * wanted / available)
	if side < 1 {
		side = 1
	}

	square := image.Rect(0, 0, side, side).Add(image.Point{
		X: cleared.Min.X + (cleared.Dx()-side)/2,
		Y: cleared.Min.Y + (cleared.Dy()-side)/2,
	})

	return fitted(square, q.logo.Bounds())
}

// fitted returns the largest rectangle of source's aspect ratio that fits
// inside box, centred in it.
//
// A logo is never stretched to the shape of its seat: a mark whose
// proportions are wrong is more obviously broken than one that leaves clear
// space on two sides.
func fitted(box image.Rectangle, source image.Rectangle) image.Rectangle {
	width, height := box.Dx(), box.Dy()

	if source.Dx()*height > source.Dy()*width {
		height = max(1, width*source.Dy()/source.Dx())
	} else {
		width = max(1, height*source.Dx()/source.Dy())
	}

	return image.Rect(0, 0, width, height).Add(image.Point{
		X: box.Min.X + (box.Dx()-width)/2,
		Y: box.Min.Y + (box.Dy()-height)/2,
	})
}
