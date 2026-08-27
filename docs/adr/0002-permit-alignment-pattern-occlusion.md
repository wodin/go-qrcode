# A logo may cover alignment patterns, but never other function patterns

Function patterns carry no error correction, so the obvious rule is to refuse
any logo that touches one. That rule would delete the feature. The centre of a
symbol is always module `2v+8`, and cross-referencing `alignmentPatternCenter`
shows an alignment pattern's 5x5 body covers that exact centre module on 18 of
the 40 versions — 7 through 13, 21, 22, 23, 24, 25, 26, 27, 35, 37, 38 and 40 —
with several more within a module or two. Above version 6, a centred logo of
any useful size collides with one on nearly every version.

So we permit alignment pattern occlusion and refuse occlusion of finder,
timing, format info and version info modules. The latter four are unreachable
by any sensible centred logo; the check is free insurance against a
pathological scale value rather than a constraint anyone will meet.

The cost is real but bounded: decoders fall back to estimating perspective
from the three finder patterns alone when alignment patterns are missing,
which loses tolerance to curvature and steep viewing angles, not the ability
to decode a flat scan. Because this is an empirical claim about decoders
rather than a property of the specification, it is pinned by round-trip
`zbarimg` tests that deliberately cover versions with a centred alignment
pattern (7, 21, 38) alongside versions without (1, 5, 14).
