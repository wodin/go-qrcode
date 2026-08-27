// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"image"
	"image/color"
)

// resample returns src scaled to width by height pixels by area averaging:
// each destination pixel is the mean of the source pixels it covers.
//
// This is the box filter of ADR-0003, and it is here rather than imported
// because a logo is reduced far more than a general purpose resampler is
// tuned for — a 512 pixel mark into a 50 pixel knockout is a ten times
// reduction, at which a bilinear or bicubic filter samples a fraction of the
// source and drops thin strokes entirely. Averaging every covered pixel
// cannot drop one.
//
// Colours are averaged alpha-premultiplied, as image/color reports them, so a
// transparent pixel contributes nothing but its transparency rather than
// bleeding its colour into its neighbours.
//
// Enlarging is not what this is for: with fewer source pixels than
// destination ones each destination pixel covers exactly one source pixel and
// the result is a blocky nearest neighbour copy — which is how the symbol's
// own modules are enlarged too. src must be non-empty and width and height at
// least one.
func resample(src image.Image, width int, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := src.Bounds()

	for y := 0; y < height; y++ {
		firstRow, lastRow := sourceSpan(y, height, bounds.Dy())

		for x := 0; x < width; x++ {
			firstCol, lastCol := sourceSpan(x, width, bounds.Dx())

			var sum [4]uint64

			for srcY := firstRow; srcY < lastRow; srcY++ {
				for srcX := firstCol; srcX < lastCol; srcX++ {
					r, g, b, a := src.At(bounds.Min.X+srcX,
						bounds.Min.Y+srcY).RGBA()

					sum[0] += uint64(r)
					sum[1] += uint64(g)
					sum[2] += uint64(b)
					sum[3] += uint64(a)
				}
			}

			covered := uint64((lastRow - firstRow) * (lastCol - firstCol))

			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(sum[0] / covered >> 8),
				G: uint8(sum[1] / covered >> 8),
				B: uint8(sum[2] / covered >> 8),
				A: uint8(sum[3] / covered >> 8),
			})
		}
	}

	return dst
}

// sourceSpan returns the half-open range of source pixels covered by
// destination pixel i, where a run of dstLength destination pixels spans
// srcLength source ones.
//
// The range is never empty: the first source pixel is rounded down and the
// last up, so a destination pixel covering part of a source pixel covers all
// of it. That is what stops a reduction dropping a stroke, and what makes an
// enlargement a nearest neighbour copy rather than a division by zero.
func sourceSpan(i int, dstLength int, srcLength int) (int, int) {
	first := i * srcLength / dstLength
	last := ((i+1)*srcLength + dstLength - 1) / dstLength

	return first, last
}
