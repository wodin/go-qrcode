---
name: qrcode-logo
description: Make QR code PNGs, plain or with a logo in the centre. Use when the user asks for a QR code (URL, Wi-Fi, contact, any text), wants a logo on one, or needs a logo made or cleaned to real transparency for one.
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
working directory, so that outputs land there and not inside the skill.

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

If a logo is wanted, read [LOGO.md](LOGO.md) before anything else: it covers
obtaining the logo, preparing it with `prepare_logo.py`, and the flags that
size and seat it. Every logo goes through `prepare_logo.py`, wherever it came
from.

## 3. Generate

`scripts/qrcode.py` runs the tool, installing it first if need be, and passes
every argument through unchanged:

```
python3 scripts/qrcode.py -s 1024 -o qr "https://example.org"
```

This writes `qr.png`. Things that go wrong otherwise:

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
| `-s N` | Image size in pixels (default 256). A negative N is pixels per module: `-s -12`. |
| `-i` | Invert: white on black. |
| `-d` | Drop the quiet zone. Keep it unless the code is placed on white anyway. |
| `-t` | Text art on stdout instead of a PNG. Cannot show a logo. |

## 4. Check and deliver

Decode the finished PNG and compare it with the content, character for
character. Use whichever decoder is at hand:

```
zbarimg -q --raw -Sbinary qr.png
python3 -c "import cv2,sys; print(cv2.QRCodeDetector().detectAndDecode(cv2.imread(sys.argv[1]))[0])" qr.png
```

Without `-Sbinary` zbar misreads non-ASCII text such as "Café" as Shift JIS.
OpenCV's detector is weaker than a phone's, so a failure there is a prompt to
look closer, not proof. If a logo code does not decode, see LOGO.md's
fallbacks. If no decoder is available, say that the code has not been
test-scanned and ask the user to scan it.

Give the user the PNG, the exact content it encodes, and how it was checked.

## Installing the tool

`scripts/qrcode.py` handles this, trying in order: `$QRCODE_BIN` (used or
fails), a `qrcode` on PATH, a binary in this skill's `bin/` (Linux, packaged
skill only), its cache, and the GitHub release for this platform, verified
against `SHA256SUMS`. `python3 scripts/qrcode.py --locate` prints the path it
settles on.

If it exits with status 127 nothing could be installed: follow the steps it
prints. A macOS binary downloaded with a browser is quarantined:
`xattr -d com.apple.quarantine qrcode` clears it.
