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
What the budget cannot see is locatability. ADR-0008 measured ink clearing
failing to locate on about 5% of combinations against about 1% for knockout;
`ClearNone` abuts live modules on every side and is expected to be at least as
bad. That was not re-measured here.

## Consequences

**The default changes.** A caller who set no `Clearing` now gets live modules
around their logo, where they got a cleared square. They ask for
`ClearKnockout` to restore it. `ClearingStyle` values were renumbered:
`ClearNone` is 0, `ClearKnockout` 1, `ClearInk` 2.
