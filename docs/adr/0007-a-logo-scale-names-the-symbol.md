# The caller names the logo's scale, and the symbol grows to carry it

`-grow-symbol` takes no version number. The caller names the size they want
their logo — `-logo-scale` — and the tool encodes at the smallest version, at
or above the one the content itself needs, whose symbol carries that scale.
`SmallestVersionCarryingLogo` measures which version that is.

## Why not a version number

The logo budget follows from the symbol's version, recovery level and margin,
never from the content (ADR-0001). The CLI fixes the level at `Highest` and
the margin at one module, so the version is the only lever left — and, with no
flag for it, the content's length was pulling it. A 22 character URL makes a
version 3 symbol, which carries 0.1034, so the default 0.2 was unreachable
from the command line at any content that short.

Handing that lever over directly, as `-qr-version N`, is the smaller change
and maps straight onto the exported `NewWithForcedVersion`. It hands the
caller the wrong variable. A brander knows the size they want the logo to be;
nobody wants version 6 as such. Turning one into the other is not a
calculation a caller can do, because the table is not a ramp — at `Highest`
with a one module margin:

| version | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 13 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| max scale | 0.1200 | 0.1034 | 0.1515 | 0.1892 | 0.2683 | 0.2000 | 0.2245 | 0.1884 |

Version 3 carries less than version 2, 7 carries less than 6 and 13 drops back
under a fifth. So a caller given `-qr-version` would search this table by hand,
one re-run per guess, to answer a question the tool can answer by measuring.
That is the same reasoning as ADR-0004: name what has been measured, and do
the measuring rather than making the caller infer it.

A floor — `-qr-version` meaning "at least N" — was the third option and
answers no question. It is still expressed in versions, and "at least" is not
the constraint that matters: a floor of 6 gives version 6 whether or not
version 6 carries the logo.

## Consequences

The version can only rise. The content has to fit the symbol it is encoded in,
so the version the content chose is the floor, however much better a smaller
symbol carries a logo. Where that version already carries the scale, nothing
grows and nothing is reported: the note on stderr belongs to a choice the tool
made.

Where no version carries the scale, what the refusal offers is a *scale* —
the largest any of them carries — and not a version, because the scale is the
part left for the caller to change. `SmallestVersionCarryingLogo` returns it
for that message and returns nothing when nothing was measured, mirroring the
zero value of a refusal's remedy.

The scan builds a codeword layout per candidate version, up to 40 of them, on
the success path as well as the failure path. It runs once per encode, only
when `-grow-symbol` is given, and never from the library's own default path.

Exact-version control is not lost, and no flag name is spent: the library's
`NewWithForcedVersion` remains the way to demand a version, and a `-qr-version`
flag could still be added for a caller who wants one for a reason other than
the logo.
