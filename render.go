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
// It holds the quiet zone because the two coordinate systems it bridges
// disagree about it: an image covers the symbol and its quiet zone together,
// while a knockout counts from the corner of the symbol. Keeping the offset
// here is what stops every caller remembering to add it.
type moduleScale struct {
	// pixelWidth is the image's width, and modulesPerPixel the width of one
	// pixel in modules. The ratio is held rather than the module count
	// because moduleAt multiplies by it once per pixel.
	pixelWidth      int
	modulesPerPixel float64

	quietZoneSize int
}

// newModuleScale returns the scale drawing modules modules — symbol and quiet
// zone together — across pixelWidth pixels.
func newModuleScale(pixelWidth int, modules int, quietZoneSize int) moduleScale {
	return moduleScale{
		pixelWidth:      pixelWidth,
		modulesPerPixel: float64(modules) / float64(pixelWidth),
		quietZoneSize:   quietZoneSize,
	}
}

// moduleAt returns the module drawn at pixel, counting from the corner of the
// quiet zone.
func (s moduleScale) moduleAt(pixel int) int {
	return int(float64(pixel) * s.modulesPerPixel)
}

// pixelsOfSymbolModules returns the pixels drawing the half-open square of
// symbol modules [first, last), clamped to the image.
func (s moduleScale) pixelsOfSymbolModules(first modulePosition,
	last modulePosition) image.Rectangle {

	left, right := s.pixelsOf(first.x+s.quietZoneSize, last.x+s.quietZoneSize)
	top, bottom := s.pixelsOf(first.y+s.quietZoneSize, last.y+s.quietZoneSize)

	return image.Rect(left, top, right, bottom)
}

// pixelsOfModule returns the pixels drawing the single symbol module at p.
func (s moduleScale) pixelsOfModule(p modulePosition) image.Rectangle {
	return s.pixelsOfSymbolModules(p, modulePosition{x: p.x + 1, y: p.y + 1})
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
	return sort.Search(s.pixelWidth, func(pixel int) bool {
		return s.moduleAt(pixel) >= module
	})
}

// withLogo returns rendered with the attached logo seated in it: the modules
// LogoOptions.Clearing names cleared to the background colour, and the logo
// drawn over them.
//
// What is cleared is not what was charged. The whole knockout is charged
// against the error correction budget whichever clearing is asked for, so
// clearing only the ink leaves the symbol less damaged than the fit paid for
// — the one direction of that asymmetry which is safe (ADR-0008).
//
// The result is full colour. A logo does not fit the two colour palette that
// keeps a plain QR Code's PNG small, so a symbol carrying one gives that
// palette up — which is why nothing is converted unless a logo was attached.
func (q *QRCode) withLogo(rendered *image.Paletted,
	scale moduleScale) image.Image {

	img := image.NewRGBA(rendered.Bounds())
	draw.Draw(img, img.Bounds(), rendered, rendered.Bounds().Min, draw.Src)

	k := newKnockout(q.symbol.symbolSize, q.logoOptions.Scale,
		q.logoOptions.Margin)

	knockedOut := scale.pixelsOfSymbolModules(k.min, k.max)

	seat := q.logoSeat(k, knockedOut, scale)
	logo := resample(q.logo, seat.Dx(), seat.Dy())

	background := image.NewUniform(q.BackgroundColor)

	// The resampled logo, rather than the caller's own image, so that what is
	// cleared and what is drawn cannot disagree (ADR-0008).
	if q.logoOptions.Clearing == ClearInk {
		newInk(logo, seat, k, scale).
			dilated(q.logoOptions.Margin).
			fill(img, background, scale)
	} else {
		draw.Draw(img, knockedOut, background, image.Point{}, draw.Src)
	}

	draw.Draw(img, seat, logo, image.Point{}, draw.Over)

	return img
}

// logoSeat returns the pixels the logo itself is drawn into, given the
// knockout k charged for it and the pixels knockedOut covers.
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
func (q *QRCode) logoSeat(k knockout, knockedOut image.Rectangle,
	scale moduleScale) image.Rectangle {

	margin := q.logoOptions.Margin

	clearArea := scale.pixelsOfSymbolModules(
		modulePosition{x: k.min.x + margin, y: k.min.y + margin},
		modulePosition{x: k.max.x - margin, y: k.max.y - margin})

	wanted := q.logoOptions.Scale * float64(q.symbol.symbolSize)
	available := float64(k.width() - 2*margin)

	side := int(float64(min(clearArea.Dx(), clearArea.Dy())) * wanted / available)
	if side < 1 {
		side = 1
	}

	return fitted(centredIn(knockedOut, side, side), q.logo.Bounds())
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

	return centredIn(box, width, height)
}

// centredIn returns a width by height rectangle centred in box, to within the
// half pixel a centre between two pixels costs.
func centredIn(box image.Rectangle, width int, height int) image.Rectangle {
	return image.Rect(0, 0, width, height).Add(image.Point{
		X: box.Min.X + (box.Dx()-width)/2,
		Y: box.Min.Y + (box.Dy()-height)/2,
	})
}
