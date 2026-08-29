# Logo fit is measured in damaged codewords per block, not in area

A logo occluding the centre of a symbol is safe only if every error correction
block can still recover. Error correction operates on codewords, not modules —
a codeword with one wrong module costs a block exactly as much as one with
eight wrong — so any check phrased as "the logo covers less than N% of the
area" is measuring the wrong quantity, and the N is folklore. Instead we
count *distinct damaged codewords per block* and require that count to stay
within that block's correction capacity. `codewordLayout` supplies both
mappings: the placement path that `newPlacementPath` returns takes an occluded
module to its codeword, and the interleave that `interleaveOrder` states — and
`encodeBlocks` applies — takes that codeword to its block.

Two consequences that look arbitrary in the code and are not:

**We spend at most half of each block's capacity.** The check requires
`damaged <= t/2`, where `t` is half the block's error correction codewords.
The other half is reserved for the physical world — print bleed, camera blur,
glare, a folded page. A logo that consumes the entire budget produces a symbol
that decodes perfectly in a unit test and fails on paper. The margin is the
one tunable number in the design and it is deliberately conservative.

**We count the knockout, not the logo.** The region charged against the budget
is the whole cleared square including its one-module margin, snapped to module
boundaries — not the logo's opaque pixels. The knockout is what a decoder
actually reads as wrong, and snapping to modules is what makes "occluded" a
crisp per-module yes/no rather than a threshold on partial coverage. Counting
the logo's silhouette instead would leave modules half-covered, damaging their
codewords just as thoroughly while appearing not to.
