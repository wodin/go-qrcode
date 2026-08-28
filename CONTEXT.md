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
same. `codewordLayout` says which codeword occupies a given module, and which
block that codeword belongs to.

**Remainder bits**:
The handful of data region modules left over past the last codeword, because
a version's data region is not always a whole number of codewords wide.
Always zero, ignored by decoders, and free for a logo to occlude: no codeword
depends on them. Counted by `numRemainderBits`, and the reason a data region
module can carry no codeword without being a function pattern.

**Placement path**:
The order in which the encoded bit stream is laid into the data region:
upwards from the bottom right corner in two-module-wide columns, reversing
direction at each edge, stepping over every function pattern — and over the
vertical timing pattern a whole column at a time, so that the columns either
side of it stay correctly paired. Bit *i* goes to the *i*th module of the
path, so codeword *n* occupies path modules 8*n* to 8*n*+7. It is what maps a
module back to the codeword it carries. `dataModulePath` returns it.

**Data mask**:
One of eight fixed patterns exclusive-ORed with the encoded bit stream as it
is placed, chosen per symbol to avoid module arrangements that confuse
decoders. A property of a module's position, not of the bit placed there — so
two symbols of the same version and mask differ only where their data differs.
_Avoid_: mask (ambiguous — the spec also calls the chosen pattern's number the
mask pattern reference)

**Block**:
A group of codewords with its own error correction codewords, correctable
independently of the other blocks. Codewords are interleaved across blocks
before being laid into the symbol — blocks visited round robin, all data
codewords before any error correction codewords — which is why contiguous
damage spreads evenly across blocks rather than destroying one.
`interleaveOrder` returns that order, and is the only statement of it.

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

**Seat**:
The pixels of the rendered image the logo itself is drawn into: a fraction
`LogoOptions.Scale` of the symbol's width, centred on the knockout, and
narrowed or shortened to the logo's own aspect ratio. Always inside the
knockout and generally smaller than it, because the knockout snaps outwards
to an odd number of whole modules; the difference over and above the margin
shows as extra background around the logo. Distinguished from the knockout
because only the knockout counts as damage — enlarging the seat within a
knockout already paid for costs the correction capacity nothing.

**Occlusion**:
A module covered by the knockout, and therefore read wrongly by a decoder.
Occlusion of a data region module is counted per codeword, never per module,
and charged against the correction capacity. Occlusion of a function pattern
carries no such charge because no capacity can pay for it: a logo may occlude
alignment patterns and no other function pattern (ADR-0002).

**Damage**:
What a knockout costs a symbol: the occluded codewords of every block, and
any function pattern occlusion. `knockoutDamage` holds it, and a block is
within its **budget** while it has lost no more than half its correction
capacity.

**Fit inversion**:
A pair of symbols where the one carrying *more* error correction — a higher
recovery level, or a larger version — accepts a *smaller* logo than the other.
Correction capacity is held per block, so splitting a symbol into more, smaller
blocks can lower a single block's budget even as the proportion of the symbol
given to error correction rises.
_Avoid_: regression, anomaly, non-monotonicity
