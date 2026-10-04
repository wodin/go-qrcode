#!/usr/bin/env python3
# /// script
# requires-python = ">=3.9"
# dependencies = ["numpy", "pillow"]
# ///
"""Give a logo a real alpha channel, ready for `qrcode -L`.

    prepare_logo.py IN OUT [--outline FRACTION] [--outline-color COLOR]
                           [--background COLOR] [--scope connected|everywhere]
                           [--tolerance N] [--preview FILE]

Whatever comes in -- a logo that is already transparent, one on a flat
background, or one on a painted "transparency" chequerboard -- what goes out
is an RGBA PNG whose background is alpha 0 and whose ink is opaque, cropped to
the ink. It refuses, with exit status 1, when it cannot tell background from
logo, which is better than handing the QR Code a rectangle.
"""

import argparse
import sys

try:
    import numpy as np
    from PIL import Image, ImageColor, ImageDraw, ImageFilter
except ImportError as err:
    sys.exit("prepare_logo.py needs numpy and Pillow (%s): install them with "
             "`pip install numpy pillow`, or run this script with `uv run`, "
             "which fetches them itself" % err)

# A logo is drawn far smaller than this, so larger sources are shrunk first.
MAX_SIDE = 1024

# Alpha at or below this is nothing, and is cleared to exactly zero.
ALPHA_NOISE = 8 / 255

# How far into the logo an anti-aliased edge may reach, in pixels.
EDGE = 3

# The share of the image's border a background has to cover to be one.
FLAT = 0.9

# How far apart a colour's channels may be for it to count as a grey.
GREY = 24

# What a preview lays the logo over: a colour neither a logo nor its outline
# is likely to be, so that what is transparent cannot be mistaken for white.
PREVIEW = (230, 60, 160, 255)

# A logo with less transparency than this is treated as having none.
TRANSPARENT = 0.01


class Refusal(Exception):
    """The image is not one a logo can be safely cut out of."""


def distance(pixels, colour):
    return np.sqrt(((pixels - colour) ** 2).sum(axis=-1))


def neighbours(a):
    """a moved one pixel up, down, left and right, zero filled."""
    padded = np.pad(a, [(1, 1), (1, 1)] + [(0, 0)] * (a.ndim - 2))
    height, width = a.shape[:2]

    return [padded[y:y + height, x:x + width]
            for y, x in ((0, 1), (2, 1), (1, 0), (1, 2))]


def grown(mask, radius):
    """mask grown by radius pixels in every direction: dilated by a disc.

    A disc is a stack of horizontal runs, one for each row of it, so the mask
    is widened by each run's half width and the result laid that many rows
    above and below. Running sums make each widening one subtraction.
    """
    height, width = mask.shape
    sums = np.cumsum(np.pad(mask, [(0, 0), (radius + 1, radius)]), axis=1,
                     dtype=np.int32)
    out = np.zeros_like(mask)

    for rows in range(radius + 1):
        half = int((radius ** 2 - rows ** 2) ** 0.5)
        run = (sums[:, radius + 1 + half:radius + 1 + half + width]
               > sums[:, radius - half:radius - half + width])

        out[rows:] |= run[:height - rows]
        out[:height - rows] |= run[rows:]

    return out


def touching_border(mask):
    """The part of mask connected to the image's border."""
    padded = np.pad(mask, 1, constant_values=True)

    # The copy is what makes the image writable: one made from an array
    # shares the array's memory read-only, and the fill is silently lost.
    image = Image.fromarray(padded.astype(np.uint8) * 255).copy()
    ImageDraw.floodfill(image, (0, 0), 128)

    return np.asarray(image)[1:-1, 1:-1] == 128


def dominant(pixels, tolerance):
    """The most common colour among pixels, and which of them are near it."""
    bins = (pixels // 16).astype(int)
    keys = bins[:, 0] * 256 + bins[:, 1] * 16 + bins[:, 2]
    values, counts = np.unique(keys, return_counts=True)

    # The fullest bin only seeds the colour: noise can split one flat colour
    # across two bins, and the mean of everything near the seed heals that.
    seed = pixels[keys == values[counts.argmax()]].mean(axis=0)
    near = distance(pixels, seed) <= tolerance

    return pixels[near].mean(axis=0), near


def find_background(rgb, tolerance):
    """The one or two flat colours the image's border is made of.

    Two is a chequerboard: what an image generator paints when asked for
    transparency it cannot produce.
    """
    strip = max(2, min(rgb.shape[:2]) // 50)
    border = np.concatenate([
        rgb[:strip].reshape(-1, 3),
        rgb[-strip:].reshape(-1, 3),
        rgb[strip:-strip, :strip].reshape(-1, 3),
        rgb[strip:-strip, -strip:].reshape(-1, 3),
    ])

    first, covered = dominant(border, tolerance)
    colours = [first]

    if covered.mean() < FLAT:
        second, _ = dominant(border[~covered], tolerance)
        colours.append(second)
        covered = covered | (distance(border, second) <= tolerance)

    # A chequerboard is two greys. Two colours of any other kind are a logo
    # reaching the edge of its image, and the second of them is its ink.
    grey = all(colour.max() - colour.min() <= GREY for colour in colours)

    if covered.mean() < FLAT or not grey and len(colours) == 2:
        raise Refusal(
            "no flat background along the image's edges, so background cannot "
            "be told from logo. Regenerate the logo on one flat colour it does "
            "not contain, with clear space on every side, or name the colour "
            "with --background.")

    return colours


def keyed_scope(colours):
    """Which background-coloured pixels to remove when the caller did not say.

    A saturated colour is a chroma key: it was chosen because the logo does
    not contain it, so every pixel of it is background, the holes the logo
    encloses included. White, black and grey may just as well be the logo's
    own ink -- a highlight, white lettering -- so only what is connected to
    the border goes, and whatever the logo encloses stays opaque.
    """
    saturated = all(colour.max() - colour.min() > 96 for colour in colours)

    return "everywhere" if saturated else "connected"


def spread(colour, known, steps):
    """Carries the known colours into their unknown neighbours, a pixel a step."""
    colour = np.where(known[..., None], colour, 0.0)

    for _ in range(steps):
        count = sum(neighbours(known.astype(float)))
        fresh = ~known & (count > 0)
        colour[fresh] = sum(neighbours(colour))[fresh] / count[fresh, None]
        known = known | fresh

    return colour, known


def matte(rgb, colours, tolerance, scope):
    """Cuts the logo out of its background: the unmixed colours and the alpha."""
    distances = np.stack([distance(rgb, colour) for colour in colours])
    behind = np.array(colours)[distances.argmin(axis=0)]

    background = distances.min(axis=0) <= tolerance
    if scope == "connected":
        background = touching_border(background)

    if background.all():
        raise Refusal("the whole image matches its background: there is no "
                      "logo here to cut out")

    # An edge pixel is the logo's colour mixed with the background's, and how
    # far it sits between the two is its alpha. The logo's colour there is
    # not known, so it is carried out from the solid ink beside it.
    edge = grown(background, EDGE) & ~background
    ink, known = spread(rgb, ~background & ~edge, EDGE)
    mixed = edge & known

    span = ink - behind
    coverage = ((rgb - behind) * span).sum(axis=-1) / np.maximum(
        (span ** 2).sum(axis=-1), 1)

    alpha = np.where(background, 0.0, 1.0)
    alpha[mixed] = np.clip(coverage[mixed], 0, 1)

    # Unmixing takes the background's share back out of the colour, which is
    # what stops a fringe of it following the logo onto the QR Code.
    share = np.maximum(alpha, ALPHA_NOISE)[..., None]
    unmixed = np.clip(behind + (rgb - behind) / share, 0, 255)

    return np.where(mixed[..., None], unmixed, rgb), alpha


def outlined(rgb, alpha, width, colour):
    """Backs the logo with an opaque shape width pixels larger all round.

    The holes the logo encloses are filled too: an outline is asked for so
    that the mark reads over live modules, and modules showing through its
    middle would undo that.
    """
    room = width + 2
    rgb = np.pad(rgb, [(room, room), (room, room), (0, 0)])
    alpha = np.pad(alpha, room)

    backing = ~touching_border(~grown(alpha > 0.5, width))

    # The shape is cut out of whole pixels, and a slight blur is what gives
    # its edge the anti-aliasing the logo's own edge has.
    soft = Image.fromarray(backing.astype(np.uint8) * 255).filter(
        ImageFilter.GaussianBlur(0.7))
    backing = np.maximum(np.asarray(soft) / 255, alpha)

    cover = alpha[..., None]

    return rgb * cover + np.array(colour, dtype=float) * (1 - cover), backing


def trimmed(rgb, alpha):
    """The logo cropped to its ink: the tool scales the image, not the ink.

    Alpha too faint to see is cleared first. The tool counts any alpha above
    zero as ink, so a stray trace of it would be cropped to and cleared for.
    """
    alpha = np.where(alpha <= ALPHA_NOISE, 0.0, alpha)
    rows = np.flatnonzero(alpha.any(axis=1))
    columns = np.flatnonzero(alpha.any(axis=0))
    if rows.size == 0:
        raise Refusal("nothing is left of the logo once its background is "
                      "removed")

    crop = (slice(rows[0], rows[-1] + 1), slice(columns[0], columns[-1] + 1))

    return rgb[crop], alpha[crop]


def hexadecimal(colour):
    return "#%02x%02x%02x" % tuple(int(round(channel)) for channel in colour)


def prepare(image, args):
    """The prepared logo as an RGBA image, and a line on what it was made from."""
    image = image.convert("RGBA")
    image.thumbnail((MAX_SIDE, MAX_SIDE), Image.LANCZOS)

    pixels = np.asarray(image, dtype=float)
    rgb, alpha = pixels[..., :3], pixels[..., 3] / 255

    if args.background is None and (alpha <= ALPHA_NOISE).mean() >= TRANSPARENT:
        source = "a real alpha channel"
    else:
        if args.background is None:
            colours = find_background(rgb, args.tolerance)
        else:
            colours = [np.array(ImageColor.getrgb(args.background)[:3], dtype=float)]

        scope = args.scope or keyed_scope(colours)
        rgb, alpha = matte(rgb, colours, args.tolerance, scope)

        kind = "a chequerboard" if len(colours) == 2 else "a flat background"
        source = "%s of %s, removed %s" % (
            kind, " and ".join(hexadecimal(colour) for colour in colours),
            "everywhere" if scope == "everywhere" else "where connected to the border")

    rgb, alpha = trimmed(rgb, alpha)

    if args.outline > 0:
        width = max(1, round(args.outline * max(alpha.shape)))
        colour = ImageColor.getrgb(args.outline_color)[:3]
        rgb, alpha = trimmed(*outlined(rgb, alpha, width, colour))

    rgba = np.dstack([rgb, alpha * 255]).round().astype(np.uint8)

    return Image.fromarray(rgba, "RGBA"), source


def main():
    parser = argparse.ArgumentParser(
        description="Give a logo a real alpha channel, ready for `qrcode -L`.")
    parser.add_argument("source", help="the logo: PNG, JPEG, GIF or WebP")
    parser.add_argument("output", help="the PNG to write")
    parser.add_argument(
        "--outline", type=float, default=0, metavar="FRACTION",
        help="back the logo with an opaque outline this fraction of its "
             "larger side wide, enclosed holes filled (try 0.06; default none)")
    parser.add_argument(
        "--outline-color", default="white", metavar="COLOR",
        help="the outline's colour: the QR Code's background (default white)")
    parser.add_argument(
        "--background", metavar="COLOR",
        help="the colour to remove (default: found along the image's edges)")
    parser.add_argument(
        "--scope", choices=["connected", "everywhere"],
        help="remove the background colour only where connected to the "
             "border, or everywhere (default: everywhere for a saturated "
             "key colour, connected for white, black or grey)")
    parser.add_argument(
        "--tolerance", type=float, default=48, metavar="N",
        help="how far a pixel may be from the background colour and still be "
             "background, as a distance in RGB (default 48)")
    parser.add_argument(
        "--preview", metavar="FILE",
        help="also write the logo laid over a strong pink, to look at: an "
             "image viewer shows transparency as white or as a chequerboard, "
             "which is no evidence either way")
    args = parser.parse_args()

    try:
        with Image.open(args.source) as image:
            logo, source = prepare(image, args)
    except (OSError, ValueError, Refusal) as err:
        print("%s: %s" % (args.source, err), file=sys.stderr)
        return 1

    logo.save(args.output, "PNG")

    if args.preview:
        backdrop = Image.new("RGBA", logo.size, PREVIEW)
        backdrop.alpha_composite(logo)
        backdrop.convert("RGB").save(args.preview, "PNG")

    alpha = np.asarray(logo)[..., 3]
    transparent, opaque = (alpha == 0).mean(), (alpha == 255).mean()

    print("source: %s" % source)
    print("wrote %s: %dx%d, %.0f%% transparent, %.0f%% opaque, %.0f%% soft edge"
          % (args.output, logo.width, logo.height, transparent * 100,
             opaque * 100, (1 - transparent - opaque) * 100))

    if opaque < (1 - transparent) / 2:
        print("warning: most of the logo is semi-transparent, so it will look "
              "washed out over the QR Code's modules; a glow or shadow in the "
              "source is the usual cause", file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
