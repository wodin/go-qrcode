// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"testing"
)

func TestImageWithoutALogoIsTwoColourPaletted(t *testing.T) {
	img, paletted := forcedVersion(t, 10, Highest).Image(-8).(*image.Paletted)

	if !paletted {
		t.Fatal("a QR Code carrying no logo no longer renders to a palette")
	}

	if len(img.Palette) != 2 {
		t.Errorf("the palette holds %d colours, want the background and the "+
			"foreground alone", len(img.Palette))
	}
}

// TestImageWithoutALogoIsUnchangedGolden pins what a QR Code carrying no logo
// renders to, across the size, border and colour options that reach the
// drawing loop. Attaching a logo may not disturb any of it.
func TestImageWithoutALogoIsUnchangedGolden(t *testing.T) {
	const expectedDigest = "ba6bb22daf2fa3b56429492fadd8de11ac24d402e07907a9f5d86d4997a7c25f"

	digest := sha256.New()

	for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
		for _, versionNumber := range []int{1, 7, 20, 40} {
			q := forcedVersion(t, versionNumber, level)

			for _, size := range []int{-1, -5, 100, 257} {
				for _, disableBorder := range []bool{false, true} {
					q.DisableBorder = disableBorder

					png, err := q.PNG(size)
					if err != nil {
						t.Fatalf("PNG(%d) at v%d level %d: %s",
							size, versionNumber, level, err)
					}

					digest.Write(png)
				}
			}
		}
	}

	if got := hex.EncodeToString(digest.Sum(nil)); got != expectedDigest {
		t.Errorf("logo-free render digest = %s, want %s (rendering changed)",
			got, expectedDigest)
	}
}

// solidLogo returns an opaque width by height image of a single colour,
// standing in for a caller's mark.
func solidLogo(c color.Color, width int, height int) image.Image {
	logo := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			logo.Set(x, y, c)
		}
	}

	return logo
}

// rgbaAt returns the colour of a rendered pixel, so that colours from
// different image models compare equal when they are the same colour.
func rgbaAt(img image.Image, x int, y int) color.RGBA {
	c := color.RGBAModel.Convert(img.At(x, y))

	return c.(color.RGBA)
}

// withLogo returns a QR Code of the given version and recovery level carrying
// logo, failing the test if the logo is refused.
func withLogo(t *testing.T, versionNumber int, level RecoveryLevel,
	logo image.Image, options LogoOptions) *QRCode {

	t.Helper()

	q := forcedVersion(t, versionNumber, level)

	if err := q.SetLogo(logo, options); err != nil {
		t.Fatalf("SetLogo: %s", err)
	}

	return q
}

func TestImageWithALogoIsFullColour(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	q := withLogo(t, 10, Highest, solidLogo(red, 64, 64), DefaultLogoOptions())

	img := q.Image(-8)

	if _, paletted := img.(*image.Paletted); paletted {
		t.Fatal("a QR Code carrying a logo rendered to a two colour palette")
	}

	centre := img.Bounds().Dx() / 2

	if got := rgbaAt(img, centre, centre); got != red {
		t.Errorf("the centre pixel is %+v, want the logo's %+v", got, red)
	}
}

// extentOf returns the bounding box of the pixels of img that are exactly c,
// so that a test can find where the logo landed without repeating the
// arithmetic that put it there.
func extentOf(img image.Image, c color.RGBA) image.Rectangle {
	extent := image.Rectangle{}

	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if rgbaAt(img, x, y) != c {
				continue
			}

			pixel := image.Rect(x, y, x+1, y+1)

			if extent.Empty() {
				extent = pixel
			} else {
				extent = extent.Union(pixel)
			}
		}
	}

	return extent
}

// assertCentred fails the test unless extent is centred in a width by width
// image, allowing the half pixel a centre between two pixels costs.
func assertCentred(t *testing.T, extent image.Rectangle, width int) {
	t.Helper()

	if off := extent.Min.X + extent.Max.X - width; off < -1 || off > 1 {
		t.Errorf("logo spans x %d..%d, want it centred on %d",
			extent.Min.X, extent.Max.X, width/2)
	}

	if off := extent.Min.Y + extent.Max.Y - width; off < -1 || off > 1 {
		t.Errorf("logo spans y %d..%d, want it centred on %d",
			extent.Min.Y, extent.Max.Y, width/2)
	}
}

func TestLogoIsCentredInsideARingOfBackground(t *testing.T) {
	const pixelsPerModule = 8

	red := color.RGBA{R: 255, A: 255}
	options := DefaultLogoOptions()
	q := withLogo(t, 10, Highest, solidLogo(red, 64, 64), options)

	img := q.Image(-pixelsPerModule)
	logo := extentOf(img, red)

	if logo.Empty() {
		t.Fatal("no pixel of the rendered image is the logo's colour")
	}

	assertCentred(t, logo, img.Bounds().Dx())

	// Every pixel within the margin of the logo is background: that clear
	// ring is what stops a binarizer smearing the logo into the modules
	// around it.
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	ring := logo.Inset(-options.Margin * pixelsPerModule)

	for y := ring.Min.Y; y < ring.Max.Y; y++ {
		for x := ring.Min.X; x < ring.Max.X; x++ {
			if (image.Point{X: x, Y: y}).In(logo) {
				continue
			}

			if got := rgbaAt(img, x, y); got != white {
				t.Fatalf("the margin at (%d,%d) is %+v, want the background %+v",
					x, y, got, white)
			}
		}
	}
}

func TestLogoLeavesTheRestOfTheSymbolAlone(t *testing.T) {
	const pixelsPerModule = 8

	plain := forcedVersion(t, 10, Highest).Image(-pixelsPerModule)

	q := withLogo(t, 10, Highest, solidLogo(color.RGBA{R: 255, A: 255}, 64, 64),
		DefaultLogoOptions())
	img := q.Image(-pixelsPerModule)

	// A logo is refused long before its knockout reaches a quarter of the
	// way out, so everything outside the middle half of the image — the
	// finder patterns, the timing patterns, most of the data region — must
	// render exactly as it does without one.
	width := plain.Bounds().Dx()
	middle := image.Rect(width/4, width/4, 3*width/4, 3*width/4)

	for y := 0; y < width; y++ {
		for x := 0; x < width; x++ {
			if (image.Point{X: x, Y: y}).In(middle) {
				continue
			}

			if got, want := rgbaAt(img, x, y), rgbaAt(plain, x, y); got != want {
				t.Fatalf("(%d,%d) renders %+v with a logo and %+v without",
					x, y, got, want)
			}
		}
	}
}

func TestNonSquareLogoKeepsItsAspectRatio(t *testing.T) {
	const (
		pixelsPerModule = 8
		logoWidth       = 64
		logoHeight      = 32
	)

	red := color.RGBA{R: 255, A: 255}
	q := withLogo(t, 10, Highest, solidLogo(red, logoWidth, logoHeight),
		DefaultLogoOptions())

	img := q.Image(-pixelsPerModule)
	logo := extentOf(img, red)

	if logo.Empty() {
		t.Fatal("no pixel of the rendered image is the logo's colour")
	}

	// Half as tall as it is wide, as it was given, rather than stretched to
	// the square knockout.
	want := logo.Dx() * logoHeight / logoWidth

	if off := logo.Dy() - want; off < -1 || off > 1 {
		t.Errorf("a %dx%d logo rendered %dx%d, want %d tall",
			logoWidth, logoHeight, logo.Dx(), logo.Dy(), want)
	}

	assertCentred(t, logo, img.Bounds().Dx())
}

// halfTransparentLogo returns a width by height image whose left half is
// opaque c and whose right half is fully transparent.
func halfTransparentLogo(c color.NRGBA, width int, height int) image.Image {
	logo := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width/2; x++ {
			logo.Set(x, y, c)
		}
	}

	return logo
}

func TestTransparentLogoCompositesOverTheBackground(t *testing.T) {
	const pixelsPerModule = 8

	blue := color.NRGBA{B: 255, A: 255}
	cream := color.RGBA{R: 255, G: 240, B: 200, A: 255}

	q := withLogo(t, 10, Highest, halfTransparentLogo(blue, 64, 64),
		DefaultLogoOptions())
	q.BackgroundColor = cream

	img := q.Image(-pixelsPerModule)

	opaque := extentOf(img, color.RGBA{B: 255, A: 255})
	if opaque.Empty() {
		t.Fatal("no pixel of the rendered image is the logo's opaque colour")
	}

	// The transparent half shows the background through it rather than the
	// black an unset alpha channel would otherwise paint. Sampled clear of
	// the seam, where averaging blends the two halves.
	clear := opaque.Max.X + opaque.Dx()/2
	middle := (opaque.Min.Y + opaque.Max.Y) / 2

	if got := rgbaAt(img, clear, middle); got != cream {
		t.Errorf("the transparent half renders %+v at (%d,%d), want the "+
			"background %+v", got, clear, middle, cream)
	}
}

func TestInvertedColoursClearTheKnockoutToTheInvertedBackground(t *testing.T) {
	const pixelsPerModule = 8

	red := color.RGBA{R: 255, A: 255}
	options := DefaultLogoOptions()

	q := withLogo(t, 10, Highest, solidLogo(red, 64, 64), options)
	q.BackgroundColor = color.Black
	q.ForegroundColor = color.White

	img := q.Image(-pixelsPerModule)
	logo := extentOf(img, red)

	if logo.Empty() {
		t.Fatal("inverting the colours changed the logo's own")
	}

	black := color.RGBA{A: 255}

	for _, at := range []image.Point{
		{X: logo.Min.X - 1, Y: (logo.Min.Y + logo.Max.Y) / 2},
		{X: logo.Max.X, Y: (logo.Min.Y + logo.Max.Y) / 2},
		{X: (logo.Min.X + logo.Max.X) / 2, Y: logo.Min.Y - 1},
		{X: (logo.Min.X + logo.Max.X) / 2, Y: logo.Max.Y},
	} {
		if got := rgbaAt(img, at.X, at.Y); got != black {
			t.Errorf("the margin at (%d,%d) is %+v, want the inverted "+
				"background %+v", at.X, at.Y, got, black)
		}
	}
}

func TestLogoIsDrawnAtTheRequestedScale(t *testing.T) {
	const (
		pixelsPerModule = 8
		versionNumber   = 10
		symbolSize      = 4*versionNumber + 17
	)

	red := color.RGBA{R: 255, A: 255}

	for _, scale := range []float64{0.1, 0.15, 0.2} {
		options := DefaultLogoOptions()
		options.Scale = scale

		q := withLogo(t, versionNumber, Highest, solidLogo(red, 64, 64), options)

		img := q.Image(-pixelsPerModule)
		logo := extentOf(img, red)

		// Scale is the logo's width as a fraction of the symbol's, whatever
		// the knockout snapped out to around it.
		want := int(scale * symbolSize * pixelsPerModule)

		if off := logo.Dx() - want; off < -1 || off > 1 {
			t.Errorf("a logo of scale %v rendered %d pixels wide, want %d",
				scale, logo.Dx(), want)
		}
	}
}

func TestKnockoutReplacesTheModulesBeneathIt(t *testing.T) {
	const pixelsPerModule = 8

	// A background colour carrying an alpha channel is still exactly what
	// the knockout is cleared to: the modules under it are replaced, not
	// left showing through.
	translucent := color.RGBA{R: 128, A: 128}

	// A wholly transparent logo, so that the centre shows the cleared
	// knockout and nothing else.
	q := withLogo(t, 10, Highest, image.NewNRGBA(image.Rect(0, 0, 64, 64)),
		DefaultLogoOptions())
	q.BackgroundColor = translucent

	img := q.Image(-pixelsPerModule)
	centre := img.Bounds().Dx() / 2

	if got := rgbaAt(img, centre, centre); got != translucent {
		t.Errorf("the centre of the knockout is %+v, want the background %+v",
			got, translucent)
	}
}
