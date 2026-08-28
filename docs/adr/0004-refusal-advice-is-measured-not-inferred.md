# Refusal advice is measured, not inferred

When `SetLogo` refuses a logo it may name a remedy only if the package has
checked that the remedy works for *that* symbol. Every plausible rule of thumb
is false somewhere, so a refusal either cites a measured result or names no
lever at all.

## Why

The obvious advice — "raise the recovery level" — is wrong often enough to
matter. A higher level carries more error correction as a fraction of the
symbol but splits it into more, smaller blocks, and correction capacity is
held per block, so a block's budget can fall as the percentage rises. This is
a **fit inversion** (see `CONTEXT.md`), and it is not a rounding artefact:
across versions 1 to 40 at a one module margin, 11 of the 120 recovery level
steps accept a *smaller* logo than the level below. Of the 119 cases where no
logo fits at all at margins 0 to 6, raising the level fails to help in 76.

The version axis is no safer. Roughly half of all version steps go backwards
at each level, so "use a larger version" is not a matter of going one higher:
at Highest, version 2 accepts 0.1200, version 3 only 0.1034, version 6 accepts
0.2683 and version 7 drops back to 0.2000.

"Use a larger version" is nevertheless the one lever worth checking, because
it is the only one that is *usually* true and is cheap to verify. Holding the
recovery level fixed, scanning versions upward rescues every nothing-fits case
at margins 0 to 14. It first fails at margin 15, collapses at margin 16 and
above (where `Low` is defeated at all 40 versions), and at margin 32 and above
nothing fits at any version or level whatsoever.

## Consequences

A refusal that cannot fit the requested logo scans versions at the caller's
own recovery level and margin, and names a version only when it found one.
That scan is real work on an error path — one `codewordLayout` per candidate —
and it is deliberate. It is bounded by a single level's 40 versions rather
than the full 160 combinations, and it usually stops within a version or two.

The alternative, kept until this decision, was to name the three levers
without checking any of them. It cost nothing and was misleading in most of
the cases where a caller most needed it.

Do not "optimise" the scan away on the assumption that a higher recovery level
or the next version up would have served. The fit inversion table in the tests
exists to make that assumption fail loudly.
