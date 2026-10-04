---
name: qrcode-logo
description: Generate QR codes as PNG images, plain or branded with a logo in the centre, using the wodin/go-qrcode `qrcode` command-line tool (installed on demand). Use this whenever the user asks for a QR code - for a URL, Wi-Fi login, contact card or any other text - and especially when they want a logo, icon or brand mark on it, or need a logo generated or cleaned up so that its background is genuinely transparent (a real alpha channel, not a painted chequerboard) before it goes on the code.
license: MIT
compatibility: Needs Python 3.9+ for the bundled scripts, plus numpy and Pillow (from PyPI) when a logo is involved, and either network access to github.com or an already installed qrcode binary.
---

# QR codes, with or without a logo

The `qrcode` tool from [wodin/go-qrcode](https://github.com/wodin/go-qrcode)
encodes text as a QR code PNG and can seat a logo in its centre. It pays for
the logo out of the code's error correction and refuses a logo the code would
not survive, so what it produces scans. Your part is the three things it
cannot do: get it installed, hand it a logo whose background is really
transparent, and check the result.

The scripts live in this skill's `scripts/` directory: wherever a command
below says `scripts/`, give the path to that directory. Stay in the user's
working directory, so that outputs land there and not inside the skill. Use
`python` where there is no `python3`.

1. Settle the content.
2. If a logo is wanted: obtain it, then prepare it.
3. Generate the QR code.
4. Decode it to check it, and deliver it.

## 1. Content

The content is encoded exactly as given, so settle the exact string first. A
URL needs its scheme (`https://`). Common non-URL forms:

- Wi-Fi: `WIFI:T:WPA;S:<network>;P:<password>;;` (`T:WEP` for WEP; for an
  open network `T:nopass` and no `P:`. Backslash-escape `\ ; , : "` inside
  the values.)
- Email, phone, SMS: `mailto:a@b.org`, `tel:+27215550100`, `smsto:+27215550100:text`
- Contact: a vCard (`BEGIN:VCARD` ... `END:VCARD`)

Shorter content makes a coarser code that scans from further away, so do not
pad it. A QR code cannot be edited after it is printed: say so if the content
looks temporary.

## 2. The logo

### Obtain it

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

### Prepare it: always

Run every logo through `prepare_logo.py`, wherever it came from:

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

`--outline 0.06` backs the logo with an opaque white border, 6% of its size
wide, and fills the holes it encloses. By default the tool draws the logo
straight over the code's live modules, and without a border a mark dissolves
into them. The outline is not needed with `-logo-clearing knockout` below,
which clears a square for the logo anyway, and is wrong where modules showing
through the mark is the look that is wanted. On an
inverted code (`-i`) the background is black, so add `--outline-color black`.

A white, black or grey background is removed only where it connects to the
image's border, so white or black inside the logo survives. A saturated key
colour is removed everywhere, enclosed holes included. `--scope connected` or
`--scope everywhere` overrides either.

## 3. Generate

`scripts/qrcode.py` runs the tool, installing it first if need be, and passes
every argument through unchanged:

```
python3 scripts/qrcode.py -s 1024 -o qr "https://example.org"

python3 scripts/qrcode.py -L logo.png -logo-scale 0.25 -grow-symbol \
    -s 1024 -o qr "https://example.org"
```

Both write `qr.png`. Things that go wrong otherwise:

- `-o` takes a file name *without* `.png`; the tool adds it. Without `-o` the
  PNG goes to stdout.
- Flags go before the content. If the content starts with `-`, put `--`
  before it.
- Put the content in single quotes. Inside double quotes the shell rewrites
  `\"`, `$` and backticks, and the tool then faithfully encodes the wrong
  string. Wi-Fi content, with its backslashes, is where this bites.
- Notes on stderr with exit status 0 ("logo scaled to ...", "symbol grown to
  ...") are information, not errors.

| Flag | Meaning |
| --- | --- |
| `-s N` | Image size in pixels (default 256). Use 1024 or more with a logo, so the logo has pixels to be drawn in. A negative N is pixels per module: `-s -12`. |
| `-L FILE` | The logo: PNG, JPEG or GIF. |
| `-logo-scale F` | Logo width as a fraction of the code's width. Used exactly or refused. Without it the largest logo the code carries is used, which for short content is small, about 0.10. |
| `-grow-symbol` | With `-logo-scale`: make the code as dense as that scale needs, instead of as sparse as the content allows. |
| `-logo-clearing STYLE` | `none` (default): draw the logo over the live modules. `knockout`: blank a white square under it. `ink`: blank only what the logo covers. |
| `-i` | Invert: white on black. |
| `-d` | Drop the white border. Scanners need that border, so leave it unless the code is placed on white anyway. |
| `-t` | Text art on stdout instead of a PNG. Cannot show a logo. |

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

**Clearing.** `none` with an outlined logo and `knockout` with a bare one
both scan reliably. `ink` reads less reliably than either; keep it for thin
marks whose negative space is the point.

## 4. Check and deliver

Decode the finished PNG and compare it with the content, character for
character. Use whichever decoder is at hand:

```
zbarimg -q --raw qr.png
python3 -c "import cv2,sys; print(cv2.QRCodeDetector().detectAndDecode(cv2.imread(sys.argv[1]))[0])" qr.png
```

OpenCV's detector is weaker than a phone's, so a failure there is a prompt to
look closer, not proof. If the code does not decode, try `-logo-clearing
knockout`, then a smaller `-logo-scale`. If no decoder is available, say that
the code has not been test-scanned and ask the user to scan it. Do not
describe a code as verified unless it was decoded.

Give the user the PNG, the exact content it encodes, and how it was checked.

## Installing the tool

`scripts/qrcode.py` handles this. If `$QRCODE_BIN` is set it uses that binary
or fails. Otherwise it uses, in order: a `qrcode` on PATH, a binary shipped in
this skill's `bin/` (Linux only, and only in the packaged skill), one it
cached earlier, and finally the release for this platform, downloaded from
GitHub and verified against the release's `SHA256SUMS`.
`python3 scripts/qrcode.py --locate` does the same, installing if need be, and
prints the path it settled on.

If it exits with status 127 nothing could be installed, usually for want of
network access. It prints these steps, with the exact file to fetch, for you
or the user to follow:

1. Download the archive for the platform from
   <https://github.com/wodin/go-qrcode/releases>
   (`qrcode-<version>-<os>-<arch>.tar.gz`, or `.zip` for Windows), unpack it,
   and set `QRCODE_BIN` to the `qrcode` binary inside, or put it on PATH. In a
   sandbox without network access, ask the user to download the archive and
   attach it to the conversation.
2. Or build it, with Go installed:
   `git clone https://github.com/wodin/go-qrcode && cd go-qrcode && go build -o qrcode ./qrcode`

Two traps. `go install github.com/skip2/go-qrcode/...` installs upstream,
which has the same command name and no logo support; `qrcode.py` passes such
a binary over. And a macOS binary downloaded with a browser is quarantined:
`xattr -d com.apple.quarantine qrcode` clears it.
