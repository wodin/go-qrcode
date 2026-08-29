// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

const (
	// inkTestSymbolSize is a version 1 symbol, the smallest there is: the
	// pictures a failure prints stay readable, and nothing here depends on
	// the version.
	inkTestSymbolSize = 21

	// inkTestPixelsPerModule is the render pitch every fixture is drawn at.
	// A fixture drawn at the same pitch as the modules it will be measured
	// against resamples one for one, so what a test asks for is what newInk
	// sees.
	inkTestPixelsPerModule = 8
)

// moduleLogo returns a width by width module logo, drawn at the pitch the ink
// tests render at, coloured by at.
func moduleLogo(width int, at func(x int, y int) color.NRGBA) image.Image {
	side := width * inkTestPixelsPerModule
	logo := image.NewNRGBA(image.Rect(0, 0, side, side))

	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			logo.Set(x, y,
				at(x/inkTestPixelsPerModule, y/inkTestPixelsPerModule))
		}
	}

	return logo
}

// opaqueWhere returns a colouring for moduleLogo that is opaque black on the
// modules inked reports and wholly transparent elsewhere.
func opaqueWhere(inked func(x int, y int) bool) func(int, int) color.NRGBA {
	return func(x int, y int) color.NRGBA {
		if inked(x, y) {
			return color.NRGBA{A: 255}
		}

		return color.NRGBA{}
	}
}

// seatedInk returns the ink of a knockoutWidth module logo filling the whole
// of the knockout of that width, in a version 1 symbol rendered without a
// quiet zone.
//
// The logo is resampled first, exactly as withLogo composites it, so a test
// measures the pixels that are actually drawn (ADR-0008).
func seatedInk(t *testing.T, logo image.Image, knockoutWidth int) ink {
	t.Helper()

	k := knockoutOfWidth(inkTestSymbolSize, knockoutWidth)
	scale := newModuleScale(inkTestSymbolSize*inkTestPixelsPerModule,
		inkTestSymbolSize, 0)

	seat := scale.pixelsOfSymbolModules(k.min, k.max)

	return newInk(resample(logo, seat.Dx(), seat.Dy()), seat, k, scale)
}

// inkPicture draws the modules of the symbol that covers reports, as rows of
// '#' and '.', so that a disagreement reads as one picture rather than as
// four hundred separate failures.
func inkPicture(covers func(p modulePosition) bool) string {
	var picture strings.Builder

	for y := 0; y < inkTestSymbolSize; y++ {
		for x := 0; x < inkTestSymbolSize; x++ {
			if covers(modulePosition{x: x, y: y}) {
				picture.WriteByte('#')
			} else {
				picture.WriteByte('.')
			}
		}

		picture.WriteByte('\n')
	}

	return picture.String()
}

// assertInk fails the test unless got inks exactly the modules of the symbol
// that want reports — including, because it scans the whole symbol, none
// outside the knockout.
func assertInk(t *testing.T, got ink, want func(p modulePosition) bool) {
	t.Helper()

	if inked, wanted := inkPicture(got.covers), inkPicture(want); inked != wanted {
		t.Errorf("inked modules:\n%swant:\n%s", inked, wanted)
	}
}

// onACross reports whether the module at (x, y) lies on the middle row or the
// middle column of a width module logo: a mark with four transparent
// quadrants and no transparent hole a dilation could close.
func onACross(width int) func(x int, y int) bool {
	return func(x int, y int) bool {
		return x == width/2 || y == width/2
	}
}

func TestInkCoversEveryModuleTheLogoTouches(t *testing.T) {
	const width = 9

	k := knockoutOfWidth(inkTestSymbolSize, width)
	got := seatedInk(t, moduleLogo(width, opaqueWhere(onACross(width))), width)

	assertInk(t, got, func(p modulePosition) bool {
		centre := modulePosition{
			x: (k.min.x + k.max.x - 1) / 2,
			y: (k.min.y + k.max.y - 1) / 2,
		}

		return k.contains(p) && (p.x == centre.x || p.y == centre.y)
	})
}

func TestInkLeavesAModuleNoPixelOfTheLogoTouches(t *testing.T) {
	const width = 5

	k := knockoutOfWidth(inkTestSymbolSize, width)
	corner := moduleLogo(width, opaqueWhere(func(x int, y int) bool {
		return x == 0 && y == 0
	}))

	assertInk(t, seatedInk(t, corner, width), func(p modulePosition) bool {
		return p == k.min
	})
}

func TestInkCountsAnyNonZeroAlphaAsInk(t *testing.T) {
	const width = 5

	// The faintest mark an eight bit alpha channel can carry. A module it
	// touches is damaged as thoroughly as one under solid ink (ADR-0001), so
	// there is no threshold below which the logo is not there.
	barelyThere := moduleLogo(width, func(x int, y int) color.NRGBA {
		return color.NRGBA{A: 1}
	})

	k := knockoutOfWidth(inkTestSymbolSize, width)

	assertInk(t, seatedInk(t, barelyThere, width), func(p modulePosition) bool {
		return k.contains(p)
	})
}

func TestInkDilatesByTheMargin(t *testing.T) {
	const width = 9

	k := knockoutOfWidth(inkTestSymbolSize, width)
	centre := modulePosition{
		x: (k.min.x + k.max.x - 1) / 2,
		y: (k.min.y + k.max.y - 1) / 2,
	}

	dot := moduleLogo(width, opaqueWhere(func(x int, y int) bool {
		return x == width/2 && y == width/2
	}))

	for _, margin := range []int{0, 1, 2} {
		t.Run("margin", func(t *testing.T) {
			got := seatedInk(t, dot, width).dilated(margin)

			// A square neighbourhood, as newKnockout's own margin is: it
			// adds margin modules to all four sides of the logo.
			assertInk(t, got, func(p modulePosition) bool {
				return abs(p.x-centre.x) <= margin && abs(p.y-centre.y) <= margin
			})
		})
	}
}

func TestClearingNeverReachesOutsideTheKnockout(t *testing.T) {
	const width = 5

	k := knockoutOfWidth(inkTestSymbolSize, width)
	solid := moduleLogo(width, opaqueWhere(func(x int, y int) bool { return true }))

	// A margin wider than the knockout itself: the dilation has nowhere to
	// go but out, and it must not. The budget charged the knockout and
	// nothing beyond it (ADR-0008).
	got := seatedInk(t, solid, width).dilated(width)

	assertInk(t, got, func(p modulePosition) bool {
		return k.contains(p)
	})
}

// abs returns the absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
