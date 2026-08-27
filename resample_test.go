// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"image"
	"image/color"
	"testing"
)

func TestResampleAveragesRatherThanSamples(t *testing.T) {
	// Reducing a checkerboard to a single pixel is the case that separates
	// the two algorithms: sampling returns one of the two colours it lands
	// on, averaging returns the tone between them.
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.Set(0, 0, color.White)
	src.Set(1, 1, color.White)
	src.Set(1, 0, color.Black)
	src.Set(0, 1, color.Black)

	// Two black and two white samples, meaned and truncated to eight bits.
	want := color.RGBA{R: 127, G: 127, B: 127, A: 255}

	if got := resample(src, 1, 1).RGBAAt(0, 0); got != want {
		t.Errorf("a checkerboard reduced to one pixel is %+v, want %+v", got, want)
	}
}

func TestResampleKeepsAThinStroke(t *testing.T) {
	// A single black column in a white image, ten times wider than the
	// destination. A filter that samples the source lands on the stroke for
	// at most one destination pixel and misses it entirely for most offsets;
	// averaging cannot miss it, because every source pixel is covered by some
	// destination pixel.
	const (
		size   = 100
		stroke = 43
	)

	src := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			src.Set(x, y, color.White)
		}

		src.Set(stroke, y, color.Black)
	}

	got := resample(src, size/10, size/10)

	darkened := 0
	for x := 0; x < got.Bounds().Dx(); x++ {
		if got.RGBAAt(x, 0).R < 255 {
			darkened++
		}
	}

	if darkened != 1 {
		t.Errorf("%d of %d columns show the stroke, want exactly 1",
			darkened, got.Bounds().Dx())
	}
}

func TestResampleAveragesTransparency(t *testing.T) {
	// Opaque red beside fully transparent blue. Averaging the channels as
	// stored would carry the invisible blue into the result at half
	// strength; averaging them alpha-premultiplied, as image/color reports
	// them, weights each pixel's colour by its coverage, so the transparent
	// pixel contributes nothing but its transparency.
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.NRGBA{R: 255, A: 255})
	src.Set(1, 0, color.NRGBA{B: 255})

	want := color.RGBA{R: 127, A: 127}

	if got := resample(src, 1, 1).RGBAAt(0, 0); got != want {
		t.Errorf("half a transparent pair is %+v, want %+v", got, want)
	}
}
