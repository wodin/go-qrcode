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
     space around it cost error correction, so a logo the QR Code could not
     survive is refused, with advice on what would fit instead:

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
	logoScale := flags.Float64("logo-scale", qrcode.DefaultLogoOptions().Scale,
		"logo width as a fraction of the QR Code's width, excluding the border")
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
		if err := attachLogo(q, *logoFile, *logoScale); err != nil {
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
