# The budget charges the knockout; the render clears the ink

`LogoOptions.Clearing` chooses between blanking the whole knockout square
(`ClearKnockout`, the default) and blanking only the modules the logo's ink
covers, dilated by the margin (`ClearInk`). Whichever is chosen, the fit
charges the whole square.

The two halves of the word "knockout" come apart here. Until now the region
charged against the correction capacity and the region cleared to background
colour were the same square, so one word carried both meanings — ADR-0001's
"we count the knockout, not the logo" was a statement about the charge, and
`withLogo`'s single `draw.Draw` was the same square again. `ClearInk` keeps
the charge and shrinks the clearing.

## The asymmetry has a direction, and only one is safe

A logo with transparent regions clears modules it never covers. Those modules
were carrying real codewords, and blanking them throws away data the decoder
could otherwise have read. On `ames-logo-transparent.png` — 80.7% transparent
— an ink clearing at version 6, scale 0.20 leaves 56 of the 121 knocked-out
modules standing, and the codewords a decoder actually reads as damaged fall
from 24 to 15; at version 10, scale 0.26 they fall from 43 to 23.

Charging pessimistically while clearing precisely is safe in exactly one
direction:

- **The budget already paid for it.** The fit charged 24 damaged codewords;
  the rendered symbol suffers 15. A decoder can only ever see less damage
  than was paid for, so no accepted logo becomes unreadable.
- **No fit signature moves.** `newKnockout` is still built from
  `(symbolSize, scale, margin)` with no image in sight, so `damageFrom`,
  `maxScale`, `MaxLogoScale`, `LogoTooLargeError.MaxScale`,
  `SmallestVersionCarryingLogo` and `FitLogo` are all untouched, and there is
  still one `logoFit` per version rather than one per version and image.

The reverse — charging the ink and clearing the square — would be dangerous
for the same reasons read backwards, and is not on offer. Charging the ink is
measurable and would buy something real (the worst block falls from 6 damaged
codewords to 5 at v6/0.20, and 6 to 4 at v10/0.26), but it makes
`MaxLogoScale` depend on the image, costs the fit its monotonicity in scale,
and turns the alpha threshold into a safety-critical number. That is a
separate decision, not this one.

## Any alpha above zero is ink

A module is inked if *any* pixel of the logo over it has alpha > 0. There is
no threshold to tune, for two reasons.

A partly covered module is fully damaged (ADR-0001), so a fractional test
would be measuring the wrong quantity — the same mistake as measuring a fit
in area. And a translucent pixel has to composite over the background rather
than over a live black module, or the mark's own edge colour shifts with
whatever the symbol happens to carry underneath.

This is the one place the alpha threshold escapes ADR-0004's
measured-not-inferred rule, and it escapes it *because* of the asymmetry
above: the threshold decides how the mark looks, not whether the symbol
scans. Move the charge onto the ink and it becomes a safety tunable, and
ADR-0004 applies again.

## The ink is dilated by the margin, per stroke

`LogoOptions.Margin` exists to stop a scanner's binarizer smearing the logo's
edge into the modules beside it. That job is local to each stroke, not to the
bounding square: a one module ring around the knockout does nothing for the
inside edge of a ring-shaped mark. So the inked set is dilated by `Margin`
modules in every direction and then clipped to the knockout, which is what
keeps the clearing a subset of what was charged.

Dilation is why the gain is not simply the transparent fraction. At v6/0.20
the mark spans about eight modules with roughly one-module strokes, so
dilation eats most of the gaps and the 46% restored is scattered speckle;
v10/0.26 is where contiguous negative space comes back.

## Consequences

**The default does not change.** `ClearKnockout` is the zero value, so a
caller who says nothing renders exactly what they rendered before. This is a
visible change to anyone already placing a transparent logo — negative space
where there was solid background — and that is a redesign of their mark, not
a bug fix, so they have to ask for it.

**The logo does not get bigger.** The fit still charges the square, so
`MaxLogoScale` reports what it always reported and a scale of 0.2 still wants
version 6. What is gained is a symbol that decodes with more real data than
it was budgeted for, at the same apparent size.

**Ink is measured on the resampled logo**, the same pixels `withLogo`
composites, so what is cleared and what is drawn cannot disagree. The box
filter of ADR-0003 can round a hairline's alpha to zero, leaving a faint
stroke drawn over a live module — cosmetic, and never a decode risk, since
the module was charged for anyway. Measuring the source image instead would
risk the opposite disagreement: a module cleared that nothing is drawn into.

**An opaque logo renders identically**, which is the regression that would
otherwise go unnoticed. Every module of the knockout is inked, so dilation
and clipping return the knockout itself. A test pins it.

## The cost the budget does not model, measured

Less codeword damage is not the same as a symbol that reads more easily, and
this is where the issue's reasoning stops short. The budget counts codewords;
a scanner has to *find* the symbol before error correction runs at all.
Clearing the ink replaces one boundary around a square with one around every
stroke, most of it now abutting live modules, and that costs a locator
something the budget cannot see.

Measured with zbarimg over all 158 version and recovery level combinations
that accept a logo, each at the largest accepted scale, with a mark whose
right half is transparent:

| clearing | 512px render | 4px per module | 6px per module |
| --- | --- | --- | --- |
| knockout | 156 of 158 read | 156 | 156 |
| ink | 148 of 158 | 148 | 148 |

Every failure is zbarimg reporting no symbol at all, never a wrong read: the
symbol is not located, so the correction capacity the fit reserved is never
spent. Three things follow from the shape of the numbers.

The scale does not matter. The same seven or so combinations fail at half the
largest accepted scale as at the whole of it, so this is not damage and no
amount of headroom in the budget buys it off. What matters is that the mark
abuts live modules at all.

The pixel pitch does not matter either, so this is not a resampling artefact.
A wider margin helps — 5 failures of 149 at two modules rather than 8 of 158
at one — without removing the effect, and widening the margin's default is a
separate decision (#17 puts it out of scope).

**This is the reason the clearing is opt-in that outlasts the compatibility
one.** A caller choosing `ClearInk` is trading a few percent of scanner
reliability for negative space in their mark, and that is a design decision
they have to make knowingly. `ClearKnockout` remains the default and remains
the one to reach for where the QR Code has to work more than it has to look a
particular way.
