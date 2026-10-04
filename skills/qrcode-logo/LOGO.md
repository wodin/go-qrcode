# The logo

## Obtain it

In order of preference:

- **The user's own file.** A brand's logo is whatever the brand says it is.
  Ask for the file rather than generating a likeness of it.
- **Drawn with code**, for a simple mark such as a monogram or a geometric
  shape: draw it with Pillow on a transparent RGBA canvas, or render an SVG.
  It has real transparency by construction. Make it 1024 pixels or so, and
  draw at four times that and scale down if the library does not anti-alias.
- **An image generator**, for anything more pictorial. See below.

A logo ends up about a quarter of the code's width, so it wants to be simple:
bold shapes, a few flat colours, no small text, no fine detail.

### Generating one: use a key colour, not "transparent"

Asking an image generator for a "transparent background" in the prompt is
unreliable. Many cannot produce an alpha channel, and paint a grey and white
chequerboard instead, which looks transparent in a preview and is in fact
opaque pixels. So unless your image tool has a real transparency *setting*
(such as an API's `background: "transparent"`), do not ask for transparency
at all. Ask for a flat key colour and let `prepare_logo.py` remove it:

> A flat vector logo of <subject>, centred, with clear space on every side
> and nothing touching the edges of the image. Solid, uniform, pure magenta
> (#FF00FF) background. No shadow, no glow, no gradient, no texture, no
> border, no text.

Magenta is the default because few logos contain it. If this one has pink or
purple in it, use pure green (#00FF00) or whichever saturated colour the logo
lacks. Parts of the logo that are meant to be white should be asked for as
white: they stay opaque, and only the key colour is removed.

The image has to reach the filesystem before a script can open it. If your
image tool only shows the picture and gives you no file, ask the user to
download it and attach it to the conversation.

## Prepare it

```
python3 scripts/prepare_logo.py generated.png logo.png --outline 0.06
```

It needs numpy and Pillow: `pip install numpy pillow`, or run it as
`uv run scripts/prepare_logo.py ...`, which fetches them itself.

It accepts a logo that already has real transparency, one on a flat
background, or one on a painted chequerboard, and writes an RGBA PNG whose
background is alpha 0 and whose ink is opaque, cropped to the ink. Cropping
matters because the tool sizes the image, not the ink: transparent padding
makes the logo smaller for the same cost. It prints what it found and what it
wrote:

```
source: a flat background of #fa08f5, removed everywhere
wrote logo.png: 786x624, 38% transparent, 60% opaque, 2% soft edge
```

That report is the evidence that the transparency is real: a logo with a
background has a substantial transparent share, and one reported as 0%
transparent is still a rectangle. (The opaque share includes the outline.)
Looking at `logo.png` itself proves nothing, since viewers show transparency
as white or as a chequerboard. To see it, add `--preview preview.png`, which
also writes the logo laid over a strong pink.

If it refuses with "no flat background", the picture has a gradient, a scene
or a logo touching its edges: regenerate it with the prompt above. For a
tightly cropped logo on a known colour, name the colour instead:
`--background white`.

A white, black or grey background is removed only where it connects to the
image's border, so white or black inside the logo survives. A saturated key
colour is removed everywhere, enclosed holes included. `--scope connected` or
`--scope everywhere` overrides either.

## Outline and clearing

`--outline 0.06` (to `prepare_logo.py`) backs the logo with an opaque white
border, 6% of its size wide, and fills the holes it encloses. On an inverted
code (`-i`) add `--outline-color black`.

`-logo-clearing` (to the tool) decides what happens to the modules under the
logo:

| Style | Effect | Pair with |
| --- | --- | --- |
| `none` (default) | Logo drawn over the live modules. | An outlined logo: without the border the mark dissolves into the modules. |
| `knockout` | A white square blanked under the logo. | A bare logo; the outline is not needed. |
| `ink` | Only what the logo covers is blanked. | Thin marks whose negative space is the point. Reads less reliably than the other two. |

Both recommended pairings scan reliably. Leave the outline off only where
modules showing through the mark is the look that is wanted.

## Generate

```
python3 scripts/qrcode.py -L logo.png -logo-scale 0.25 -grow-symbol \
    -s 1024 -o qr "https://example.org"
```

| Flag | Meaning |
| --- | --- |
| `-L FILE` | The logo: PNG, JPEG or GIF. |
| `-logo-scale F` | Logo width as a fraction of the code's width. Used exactly or refused. Without it the largest logo the code carries is used, which for short content is small, about 0.10. |
| `-grow-symbol` | With `-logo-scale`: make the code as dense as that scale needs, instead of as sparse as the content allows. |
| `-s 1024` | Or more, so the logo has pixels to be drawn in. |

**Size.** `-logo-scale 0.25 -grow-symbol` is a good default, and as big as
"make the logo big" should usually mean. Past it the code gets denser fast.
For short content:

| `-logo-scale` | Code it grows to | Modules across |
| --- | --- | --- |
| up to 0.25 | version 6 | 41 |
| 0.30 | version 14 | 73 |
| 0.33 | version 40 | 177 |

Nothing above 0.3333 is carried by any code. Long content starts at a higher
version and its limits differ, so read the version the tool reports. A denser
code needs more pixels and a larger print: past version 6, size the image by
module instead, `-s -12`, so that every module stays 12 pixels wide. Do not
shrink the logo "to be safe": the tool has already refused anything unsafe.
The "1 module margin" in its messages is the clear space it keeps around the
logo; it is fixed.

**Refusals.** A logo too large for the code is refused with exit status 1 and
a message naming the largest scale that fits. Either use that scale, or add
`-grow-symbol` to keep the scale and grow the code.

**Fallbacks.** If the code does not decode, try `-logo-clearing knockout`,
then a smaller `-logo-scale`.
