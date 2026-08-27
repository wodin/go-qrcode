# go-qrcode

A QR Code encoder: it turns content into a matrix barcode and renders that
matrix as an image. This glossary fixes the vocabulary for the parts of a QR
Code that the encoder, the renderer and the logo feature all have to talk
about at once — and which English happily conflates.

## The matrix

**Module**:
One cell of the QR Code matrix — the atomic black-or-white unit. It is not a
pixel; at render time one module becomes many pixels.
_Avoid_: pixel, dot, cell, square

**Symbol**:
The square grid of modules that carries the code, excluding the quiet zone.
Its width is `symbolSize()` = `4×version + 17` modules.
_Avoid_: QR code (too vague — see Image)

**Quiet zone**:
The blank margin of modules surrounding the symbol, required by the spec so a
decoder can find the symbol's edges. Suppressed by `DisableBorder`.
_Avoid_: border, padding, margin

**Image**:
The rendered raster output, in pixels, covering symbol *and* quiet zone
together. `symbol.size` is measured in this combined span, not in symbol
modules — the single most common source of off-by-a-quiet-zone errors here.

**Version**:
The size class of a symbol, 1–40. Higher version means a wider symbol and more
capacity. Unrelated to software versioning.

## Module roles

**Function pattern**:
Any module whose value is fixed by the spec rather than by content: finder
patterns, separators, timing patterns, alignment patterns, format info and
version info. Function patterns carry no error correction — damage to them is
unrecoverable, because a decoder needs them before error correction can run.
_Avoid_: structural module, fixed module

**Alignment pattern**:
A 5×5 function pattern used by decoders to correct for perspective distortion.
Their centres are given by `alignmentPatternCenter`. Distinguished from the
other function patterns because a centred logo unavoidably collides with one
on most versions ≥ 7, whereas it can never reach a finder or timing pattern.

**Data region**:
Every module that is not a function pattern. These carry the interleaved data
and error correction codewords, and are the only modules a logo may occlude.

## Error correction

**Codeword**:
Eight consecutive bits of the encoded stream, laid into eight modules of the
data region. The unit error correction actually operates on: a codeword with
one wrong module and a codeword with eight wrong modules cost the decoder the
same.

**Block**:
A group of codewords with its own error correction codewords, correctable
independently of the other blocks. Codewords are interleaved across blocks
before being laid into the symbol, which is why contiguous damage spreads
evenly across blocks rather than destroying one.

**Recovery level**:
`Low`, `Medium`, `High`, `Highest` — how much of each block is error
correction. Roughly 7%, 15%, 25% and 30% of total codewords are correctable at
unknown positions.
_Avoid_: ECC level, error level, L/M/Q/H

**Correction capacity**:
The number of damaged codewords a single block can recover, `t` = half its
error correction codewords. The real ceiling on how much of the symbol a logo
may cover.

## The logo feature

**Logo**:
A caller-supplied image composited over the centre of the rendered image.

**Knockout**:
The region of the image cleared to background colour to seat the logo,
including its surrounding margin. Larger than the logo itself, and it — not
the logo's own extent — is what counts as damage against the correction
capacity.

**Occlusion**:
A data-region module covered by the knockout, and therefore read wrongly by a
decoder. Counted per codeword, never per module.
