# The logo is downscaled with a hand-rolled box filter

`image/draw` composites but cannot resample, and this package has no
third-party dependencies. Rather than add `golang.org/x/image/draw`, the
library implements area-averaging (box filter) downscaling in about
twenty-five lines.

This is not a not-invented-here compromise. Logos arrive far larger than the
hole they go into — a 512x512 source into a 50px knockout is a 10x reduction —
and `CatmullRom` and `ApproxBiLinear` are tuned for modest scale factors. At
10x they undersample, dropping thin strokes entirely. Area averaging is the
correct algorithm for large reductions, so the dependency would have bought a
worse result.

It also keeps the resampler where the sizes are known. The knockout is snapped
to module boundaries, so its pixel dimensions are not settled until render
time, inside the library — a caller cannot pre-size the logo without
`symbolSize()`, which is unexported and should stay that way.
