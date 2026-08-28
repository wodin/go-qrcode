# go-qrcode #

<img src='https://skip.org/img/nyancat-youtube-qr.png' align='right'>

Package qrcode implements a QR Code encoder. [![Build Status](https://travis-ci.org/skip2/go-qrcode.svg?branch=master)](https://travis-ci.org/skip2/go-qrcode)

A QR Code is a matrix (two-dimensional) barcode. Arbitrary content may be encoded, with URLs being a popular choice :)

Each QR Code contains error recovery information to aid reading damaged or obscured codes. There are four levels of error recovery: Low, medium, high and highest. QR Codes with a higher recovery level are more robust to damage, at the cost of being physically larger.

## Install

    go get -u github.com/skip2/go-qrcode/...

A command-line tool `qrcode` will be built into `$GOPATH/bin/`.

## Usage

    import qrcode "github.com/skip2/go-qrcode"

- **Create a 256x256 PNG image:**

        var png []byte
        png, err := qrcode.Encode("https://example.org", qrcode.Medium, 256)

- **Create a 256x256 PNG image and write to a file:**

        err := qrcode.WriteFile("https://example.org", qrcode.Medium, 256, "qr.png")

- **Create a 256x256 PNG image with custom colors and write to file:**

        err := qrcode.WriteColorFile("https://example.org", qrcode.Medium, 256, color.Black, color.White, "qr.png")

All examples use the qrcode.Medium error Recovery Level and create a fixed 256x256px size QR Code. The last function creates a white on black instead of black on white QR Code.

## Logos

A logo may be placed in the centre of a QR Code. The logo, and the clear space kept around it, cover modules a decoder would otherwise read, so they are paid for out of the error recovery information — and only half of it, leaving the rest to absorb the damage print and camera do.

- **Attach the largest logo the QR Code safely carries**, with a one module margin around it:

        q, err := qrcode.New("https://example.org/campaigns/spring-sale", qrcode.Highest)
        err = q.FitLogo(logo, 1)

- **Attach a logo of a size you choose:**

        options := qrcode.DefaultLogoOptions()
        options.Scale = 0.15

        err := q.SetLogo(logo, options)

  A logo the QR Code could not survive is refused with a `*qrcode.LogoTooLargeError` or a `*qrcode.LogoOccludesFunctionPatternError`, each carrying the largest scale that would have been accepted. The content above is long enough to make a version 5 symbol, which carries a scale 0.15 logo; the shorter `https://example.org` makes a version 3 symbol, which does not.

- **Ask what fits before attaching anything:**

        scale := q.MaxLogoScale(1)

  `MaxLogoScale` returns 0 when the QR Code carries no logo at all at that margin, which only versions 1 and 2 at the Low recovery level do at a one module margin.

The default scale of 0.2 is not a size every QR Code carries. No version carries it at Low. At Medium it is first accepted at version 11, at High and Highest at version 6 — and refused again at larger versions above those, so the first accepting version is not a floor.

**A higher recovery level does not always accept a larger logo.** Error correction is spent per block, and a higher level splits the symbol into more, smaller blocks, so a single block's budget can fall even as the proportion of the symbol given to error correction rises. At a one module margin, version 15 accepts a logo of 0.2727 at High and only 0.1688 at Highest, and 11 of the 120 recovery level steps go backwards like this. Larger versions behave no more monotonically. Ask `MaxLogoScale`, or let `FitLogo` choose; do not reason it out from the percentages.

## Documentation

[![godoc](https://godoc.org/github.com/skip2/go-qrcode?status.png)](https://godoc.org/github.com/skip2/go-qrcode)

## Demoapp

[http://go-qrcode.appspot.com](http://go-qrcode.appspot.com)

## CLI

A command-line tool `qrcode` will be built into `$GOPATH/bin/`.

```
qrcode -- QR Code encoder in Go
https://github.com/skip2/go-qrcode

Flags:
  -L string
    	shorthand for -logo
  -d	disable QR Code border
  -i	invert black and white
  -logo string
    	logo image file (PNG, JPEG or GIF) to place in the centre, empty for none
  -logo-scale float
    	logo width as a fraction of the QR Code's width, excluding the border (default: the largest that fits)
  -o string
    	out PNG file prefix, empty for stdout
  -s int
    	image size (pixel) (default 256)
  -t	print as text-art on stdout

Usage:
  1. Arguments except for flags are joined by " " and used to generate QR code.
     Default output is STDOUT, pipe to imagemagick command "display" to display
     on any X server.

       qrcode hello word | display

  2. Save to file if "display" not available:

       qrcode "homepage: https://github.com/skip2/go-qrcode" > out.png

  3. Brand the QR Code with a logo in its centre. The logo and the clear
     space around it cost error correction, so without -logo-scale the
     largest logo the QR Code survives is used, and the scale chosen is
     reported on stderr:

       qrcode -L logo.png "https://example.org" > out.png

     A scale given explicitly is used exactly or refused, with advice on
     what would fit instead. Longer content makes a larger symbol, which
     carries a larger logo:

       qrcode -L logo.png -logo-scale 0.15 "https://example.org/spring-sale" > out.png

```
## Maximum capacity
The maximum capacity of a QR Code varies according to the content encoded and the error recovery level. The maximum capacity is 2,953 bytes, 4,296 alphanumeric characters, 7,089 numeric digits, or a combination of these.

## Borderless QR Codes

To aid QR Code reading software, QR codes have a built in whitespace border.

If you know what you're doing, and don't want a border, see https://gist.github.com/skip2/7e3d8a82f5317df9be437f8ec8ec0b7d for how to do it. It's still recommended you include a border manually.

## Links

- [http://en.wikipedia.org/wiki/QR_code](http://en.wikipedia.org/wiki/QR_code)
- [ISO/IEC 18004:2006](http://www.iso.org/iso/catalogue_detail.htm?csnumber=43655) - Main QR Code specification (approx CHF 198,00)<br>
- [https://github.com/qpliu/qrencode-go/](https://github.com/qpliu/qrencode-go/) - alternative Go QR encoding library based on [ZXing](https://github.com/zxing/zxing)
