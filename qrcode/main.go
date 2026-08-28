// go-qrcode
// Copyright 2014 Tom Harwood

package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

func main() {
	os.Exit(report(run(os.Args[1:], os.Stdout, os.Stderr), os.Stderr))
}

// usageError is a malformed command line. By the time run returns one the
// flag package has already written both the message and the usage text to
// stderr, so reporting it again would print it twice.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

// report writes err to w unless it has already been reported, and returns the
// process exit status.
func report(err error, w io.Writer) int {
	var malformed usageError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &malformed):
		// The status the flag package itself uses for a bad command line.
		return 2
	default:
		fmt.Fprintf(w, "%s\n", err)
		return 1
	}
}

// printUsage writes the tool's flags and usage examples to the flag set's
// own output, so that both halves land on the same stream.
func printUsage(flags *flag.FlagSet) {
	w := flags.Output()

	fmt.Fprint(w, `qrcode -- QR Code encoder in Go
https://github.com/skip2/go-qrcode

Flags:
`)
	flags.PrintDefaults()
	fmt.Fprint(w, `
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
     what would fit instead:

       qrcode -L logo.png -logo-scale 0.15 "https://example.org" > out.png

`)
}

// misuse reports a command line that parsed but asks for something the tool
// cannot do. The usage text comes first, as it does for a command line the
// flag package itself rejects, so that the message is the last thing printed.
func misuse(flags *flag.FlagSet, message string) error {
	flags.Usage()

	return errors.New(message)
}

// isSet reports whether the named flag was given on the command line, which
// is what tells a flag left at its default apart from one deliberately set to
// that value.
func isSet(flags *flag.FlagSet, name string) bool {
	given := false

	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			given = true
		}
	})

	return given
}

// attachLogo reads the image in the file named by path and places it in the
// centre of q, scale of the symbol's width wide.
func attachLogo(q *qrcode.QRCode, path string, scale float64) error {
	logo, err := readImage(path)
	if err != nil {
		return err
	}

	options := qrcode.DefaultLogoOptions()
	options.Scale = scale

	return q.SetLogo(logo, options)
}

// fitLogo reads the image in the file named by path and places it in the
// centre of q at the largest scale the symbol carries, noting the choice on
// stderr.
//
// This is what -L alone does, because the default scale of 0.2 is refused by
// every symbol below version 6 at the recovery level the tool encodes at, and
// a caller who named no scale asked for a branded QR Code rather than for one
// particular size. The note is not a warning: it is how a caller learns the
// scale and version they would name to ask for the same image explicitly.
func fitLogo(q *qrcode.QRCode, path string, stderr io.Writer) error {
	logo, err := readImage(path)
	if err != nil {
		return err
	}

	margin := qrcode.DefaultLogoOptions().Margin

	if err := q.FitLogo(logo, margin); err != nil {
		return err
	}

	_, err = fmt.Fprintf(stderr, "logo scaled to %.4f of the QR Code's width, "+
		"the largest a version %d symbol accepts with a %d module margin\n",
		q.MaxLogoScale(margin), q.VersionNumber, margin)

	return err
}

// readImage decodes the image file named by path, which the standard library
// must have a decoder for.
//
// Three things go wrong here and each calls for a different fix, so each is
// reported differently: a file that is not there, a file in a format the
// standard library cannot read, and a file in a format it can read but which
// is damaged — which the decoder itself describes better than we could.
func readImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if errors.Is(err, image.ErrFormat) {
		return nil, fmt.Errorf("%s: not a PNG, JPEG or GIF", path)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return img, nil
}

// run is the command line tool: it encodes the content named by args and
// writes the result to stdout, or to the file named by -o, with diagnostics on
// stderr. It returns its failures rather than exiting, so that a test can
// drive it in-process.
func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("qrcode", flag.ContinueOnError)
	flags.SetOutput(stderr)

	outFile := flags.String("o", "", "out PNG file prefix, empty for stdout")
	size := flags.Int("s", 256, "image size (pixel)")
	textArt := flags.Bool("t", false, "print as text-art on stdout")
	negative := flags.Bool("i", false, "invert black and white")
	disableBorder := flags.Bool("d", false, "disable QR Code border")
	logoFile := flags.String("logo", "", "logo image file (PNG, JPEG or GIF) to place in the centre, empty for none")
	flags.StringVar(logoFile, "L", "", "shorthand for -logo")
	// The zero default is not a scale the tool ever uses — isSet decides
	// whether the flag was given at all — and it keeps the flag package from
	// printing a default that contradicts the description.
	logoScale := flags.Float64("logo-scale", 0,
		"logo width as a fraction of the QR Code's width, excluding the border (default: the largest that fits)")
	flags.Usage = func() { printUsage(flags) }

	if err := flags.Parse(args); err != nil {
		// Parse has already written the message, if any, and the usage text.
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return usageError{err}
	}

	if len(flags.Args()) == 0 {
		return misuse(flags, "Error: no content given")
	}

	if *logoFile == "" && isSet(flags, "logo-scale") {
		return misuse(flags, "-logo-scale needs a logo: pass -L <file>")
	}

	if *logoFile != "" && *textArt {
		return misuse(flags, "text art cannot show a logo: drop -t or -L")
	}

	q, err := qrcode.New(strings.Join(flags.Args(), " "), qrcode.Highest)
	if err != nil {
		return err
	}

	q.DisableBorder = *disableBorder

	if *logoFile != "" {
		// A scale the caller named is seated exactly or refused; one they did
		// not is the tool's to choose, and it chooses the largest that fits.
		place := func() error { return fitLogo(q, *logoFile, stderr) }
		if isSet(flags, "logo-scale") {
			place = func() error { return attachLogo(q, *logoFile, *logoScale) }
		}

		if err := place(); err != nil {
			return err
		}
	}

	if *textArt {
		_, err := fmt.Fprintln(stdout, q.ToString(*negative))
		return err
	}

	if *negative {
		q.ForegroundColor, q.BackgroundColor = q.BackgroundColor, q.ForegroundColor
	}

	png, err := q.PNG(*size)
	if err != nil {
		return err
	}
	if *outFile == "" {
		_, err = stdout.Write(png)
		return err
	}

	file, err := os.Create(*outFile + ".png")
	if err != nil {
		return err
	}
	if _, err := file.Write(png); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
