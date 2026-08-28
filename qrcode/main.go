// go-qrcode
// Copyright 2014 Tom Harwood

package main

import (
	"errors"
	"flag"
	"fmt"
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

`)
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
	flags.Usage = func() { printUsage(flags) }

	if err := flags.Parse(args); err != nil {
		// Parse has already written the message, if any, and the usage text.
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return usageError{err}
	}

	if len(flags.Args()) == 0 {
		flags.Usage()
		return errors.New("Error: no content given")
	}

	q, err := qrcode.New(strings.Join(flags.Args(), " "), qrcode.Highest)
	if err != nil {
		return err
	}

	q.DisableBorder = *disableBorder

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
