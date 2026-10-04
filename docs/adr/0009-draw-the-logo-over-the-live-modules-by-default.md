# Draw the logo over the live modules by default

`ClearNone` blanks nothing: the logo is composited straight onto the symbol.
It is the zero value, so it is the default for `LogoOptions` and for the CLI's
`-logo-clearing`. `ClearKnockout` and `ClearInk` are opt-in.

## Why

A transparent or soft-edged mark looks as designed: modules show through its
holes, and an edge slices a module diagonally instead of the module being
wholly shown or omitted. Clearing is a choice about how the mark looks; the
default should not make it.

## What it is not

It is not "knockout for opaque, ink for transparent". An opaque logo under
`ClearNone` differs from `ClearKnockout` by the margin ring and the snap-out
slack, which stay live. A transparent logo differs from `ClearInk` by the
dilation, which also stays live.

## Safety

The budget is unchanged (ADR-0008): the whole knockout is charged, and the
logo is seated inside it, so nothing drawn can damage more than was paid for.
What the budget cannot see
is locatability, so it was measured as ADR-0008 did: zbarimg over all 158
version and recovery level combinations that accept a logo, each at the
largest accepted scale, with the right half transparent:

| clearing | 512px | 4px per module | 6px per module |
| --- | --- | --- | --- |
| none | 157 of 158 | 157 | 157 |
| knockout | 156 | 156 | 156 |
| ink | 150 | 148 | 148 |

Expectation was that `none` would be worst, since it abuts live modules on
every side. It is not: it reads as well as `knockout`. The ink clearing's
cost comes from clearing around each stroke, leaving a ragged blank boundary
for a locator; with nothing cleared there is none. With an opaque logo all
three read 156 to 157 of 158 at 512px (153 to 152 of 158 at 4px, which is
noise-level: the failing combinations differ between styles and none is a
pattern).

Failures at this size are single combinations that differ between styles
(none: v17/High; knockout: v7/Low, v19/High), so the one-in-158 differences
are not evidence for one style over another.

## Consequences

**The default changes.** A caller who set no `Clearing` now gets live modules
around their logo, where they got a cleared square. They ask for
`ClearKnockout` to restore it. No margin is added automatically: a caller who
wants a border around a mark puts an opaque one in the image. `ClearingStyle` values were renumbered:
`ClearNone` is 0, `ClearKnockout` 1, `ClearInk` 2.
