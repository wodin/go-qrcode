// go-qrcode
// Copyright 2014 Tom Harwood

package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// invoke runs the command line tool as the process would, capturing both
// streams instead of letting them reach the terminal.
func invoke(t *testing.T, args ...string) (stdout []byte, stderr string, err error) {
	t.Helper()

	var out, diagnostics bytes.Buffer
	err = run(args, &out, &diagnostics)
	return out.Bytes(), diagnostics.String(), err
}

func TestNoContentIsAnErrorAndPrintsUsage(t *testing.T) {
	stdout, stderr, err := invoke(t)

	if err == nil {
		t.Fatal("run() with no arguments returned no error")
	}
	if err.Error() != "Error: no content given" {
		t.Errorf("run() error = %q, want %q", err, "Error: no content given")
	}
	if len(stdout) != 0 {
		t.Errorf("run() wrote %d bytes to stdout, want none", len(stdout))
	}
	if !strings.Contains(stderr, "qrcode -- QR Code encoder in Go") {
		t.Errorf("run() stderr = %q, want the usage text", stderr)
	}
}

func TestHelpPrintsUsageAndSucceeds(t *testing.T) {
	stdout, stderr, err := invoke(t, "-h")

	if err != nil {
		t.Errorf("run(-h) error = %v, want nil: asking for help is not a failure", err)
	}
	if len(stdout) != 0 {
		t.Errorf("run(-h) wrote %d bytes to stdout, want none", len(stdout))
	}
	if !strings.Contains(stderr, "Flags:") {
		t.Errorf("run(-h) stderr = %q, want the usage text", stderr)
	}
}

func TestUnknownFlagIsAnErrorAndPrintsUsage(t *testing.T) {
	stdout, stderr, err := invoke(t, "-x")

	if err == nil {
		t.Fatal("run(-x) returned no error for an undefined flag")
	}
	if len(stdout) != 0 {
		t.Errorf("run(-x) wrote %d bytes to stdout, want none", len(stdout))
	}
	if !strings.Contains(stderr, "flag provided but not defined: -x") {
		t.Errorf("run(-x) stderr = %q, want the undefined flag message", stderr)
	}
	if !strings.Contains(stderr, "Flags:") {
		t.Errorf("run(-x) stderr = %q, want the usage text", stderr)
	}
}

func TestAMalformedCommandLineExitsTwoWithoutRepeatingItself(t *testing.T) {
	_, _, err := invoke(t, "-x")
	if err == nil {
		t.Fatal("run(-x) returned no error for an undefined flag")
	}

	// The flag package has already written the message, so reporting it must
	// stay silent, and a malformed command line exits 2 rather than 1.
	var repeated bytes.Buffer
	if status := report(err, &repeated); status != 2 {
		t.Errorf("report(malformed command line) = %d, want 2", status)
	}
	if repeated.Len() != 0 {
		t.Errorf("report() repeated the message %q", repeated.String())
	}
}

func TestOrdinaryFailuresAreReportedAndExitOne(t *testing.T) {
	var reported bytes.Buffer
	if status := report(errors.New("no data to encode"), &reported); status != 1 {
		t.Errorf("report(failure) = %d, want 1", status)
	}
	if reported.String() != "no data to encode\n" {
		t.Errorf("report(failure) wrote %q, want %q", reported.String(), "no data to encode\n")
	}

	var quiet bytes.Buffer
	if status := report(nil, &quiet); status != 0 {
		t.Errorf("report(nil) = %d, want 0", status)
	}
	if quiet.Len() != 0 {
		t.Errorf("report(nil) wrote %q, want nothing", quiet.String())
	}
}

// decodedImage decodes the PNG the tool printed, failing the test if stdout
// carries something else.
func decodedImage(t *testing.T, stdout []byte) image.Image {
	t.Helper()

	img, err := png.Decode(bytes.NewReader(stdout))
	if err != nil {
		t.Fatalf("stdout is not a PNG: %v", err)
	}
	return img
}

func TestContentIsWrittenToStdoutAsAPNG(t *testing.T) {
	stdout, stderr, err := invoke(t, "hello")

	if err != nil {
		t.Fatalf("run(hello) error = %v, want nil", err)
	}
	if stderr != "" {
		t.Errorf("run(hello) stderr = %q, want nothing", stderr)
	}

	img := decodedImage(t, stdout)
	// -s defaults to 256 pixels.
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Errorf("image is %dx%d, want 256x256", b.Dx(), b.Dy())
	}
}

func TestSizeFlagSetsTheImageSizeInPixels(t *testing.T) {
	stdout, _, err := invoke(t, "-s", "128", "hello")

	if err != nil {
		t.Fatalf("run(-s 128 hello) error = %v, want nil", err)
	}
	img := decodedImage(t, stdout)
	if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 128 {
		t.Errorf("image is %dx%d, want 128x128", b.Dx(), b.Dy())
	}
}

// textArtLines returns the rows of a text-art symbol, discarding the blank
// line the tool prints after it.
func textArtLines(t *testing.T, stdout []byte) []string {
	t.Helper()

	art := strings.TrimRight(string(stdout), "\n")
	if art == "" {
		t.Fatal("stdout carries no text art")
	}
	return strings.Split(art, "\n")
}

func TestTextArtFlagWritesASymbolToStdout(t *testing.T) {
	stdout, stderr, err := invoke(t, "-t", "hello")

	if err != nil {
		t.Fatalf("run(-t hello) error = %v, want nil", err)
	}
	if stderr != "" {
		t.Errorf("run(-t hello) stderr = %q, want nothing", stderr)
	}

	lines := textArtLines(t, stdout)
	// "hello" is five bytes, so it fits a version 1 symbol even at the
	// Highest recovery level: 21 modules, plus a four module quiet zone on
	// each side.
	if len(lines) != 29 {
		t.Errorf("text art has %d rows, want 29", len(lines))
	}
}

// cornerColour returns the top left pixel of a rendered image, which lies in
// the quiet zone and therefore carries the background colour.
func cornerColour(t *testing.T, stdout []byte) color.RGBA {
	t.Helper()

	img := decodedImage(t, stdout)
	b := img.Bounds()
	return color.RGBAModel.Convert(img.At(b.Min.X, b.Min.Y)).(color.RGBA)
}

func TestInvertFlagSwapsTheTextArtColours(t *testing.T) {
	plain, _, err := invoke(t, "-t", "hello")
	if err != nil {
		t.Fatalf("run(-t hello) error = %v, want nil", err)
	}
	inverted, _, err := invoke(t, "-t", "-i", "hello")
	if err != nil {
		t.Fatalf("run(-t -i hello) error = %v, want nil", err)
	}

	// The first row is all quiet zone. Text art draws a light module as a
	// filled block, so inverting must leave that row blank.
	if got := []rune(textArtLines(t, plain)[0])[0]; got != '█' {
		t.Errorf("text art starts with %q, want a filled block", got)
	}
	if got := []rune(textArtLines(t, inverted)[0])[0]; got != ' ' {
		t.Errorf("inverted text art starts with %q, want a blank", got)
	}
}

func TestInvertFlagSwapsTheImageColours(t *testing.T) {
	plain, _, err := invoke(t, "hello")
	if err != nil {
		t.Fatalf("run(hello) error = %v, want nil", err)
	}
	inverted, _, err := invoke(t, "-i", "hello")
	if err != nil {
		t.Fatalf("run(-i hello) error = %v, want nil", err)
	}

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	if got := cornerColour(t, plain); got != white {
		t.Errorf("quiet zone is %v, want white", got)
	}
	if got := cornerColour(t, inverted); got != black {
		t.Errorf("inverted quiet zone is %v, want black", got)
	}
}

func TestDisableBorderFlagDropsTheQuietZone(t *testing.T) {
	stdout, _, err := invoke(t, "-t", "-d", "hello")
	if err != nil {
		t.Fatalf("run(-t -d hello) error = %v, want nil", err)
	}

	// The version 1 symbol alone, with no four module quiet zone around it.
	if lines := textArtLines(t, stdout); len(lines) != 21 {
		t.Errorf("text art has %d rows, want 21", len(lines))
	}
}

func TestOutputFileFlagWritesThePNGToDiskInsteadOfStdout(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "branded")

	stdout, stderr, err := invoke(t, "-o", prefix, "hello")
	if err != nil {
		t.Fatalf("run(-o %s hello) error = %v, want nil", prefix, err)
	}
	if len(stdout) != 0 {
		t.Errorf("run(-o ...) wrote %d bytes to stdout, want none", len(stdout))
	}
	if stderr != "" {
		t.Errorf("run(-o ...) stderr = %q, want nothing", stderr)
	}

	// The flag names a prefix, not a filename: the tool appends the suffix.
	written, err := os.ReadFile(prefix + ".png")
	if err != nil {
		t.Fatalf("reading the written image: %v", err)
	}

	piped, _, err := invoke(t, "hello")
	if err != nil {
		t.Fatalf("run(hello) error = %v, want nil", err)
	}
	if !bytes.Equal(written, piped) {
		t.Error("the written image differs from the one the tool prints")
	}
}

func TestAnUnwritableOutputFileIsAnError(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "no-such-directory", "branded")

	stdout, _, err := invoke(t, "-o", prefix, "hello")

	if err == nil {
		t.Fatal("run() with an unwritable output file returned no error")
	}
	if !strings.Contains(err.Error(), prefix+".png") {
		t.Errorf("run() error = %q, want it to name %s.png", err, prefix)
	}
	if len(stdout) != 0 {
		t.Errorf("run() wrote %d bytes to stdout, want none", len(stdout))
	}
}

func TestArgumentsAreJoinedWithSpaces(t *testing.T) {
	separate, _, err := invoke(t, "-t", "hello", "world")
	if err != nil {
		t.Fatalf("run(-t hello world) error = %v, want nil", err)
	}
	quoted, _, err := invoke(t, "-t", "hello world")
	if err != nil {
		t.Fatalf("run(-t 'hello world') error = %v, want nil", err)
	}
	unseparated, _, err := invoke(t, "-t", "helloworld")
	if err != nil {
		t.Fatalf("run(-t helloworld) error = %v, want nil", err)
	}

	if !bytes.Equal(separate, quoted) {
		t.Error("separate arguments encode something other than the quoted string")
	}
	if bytes.Equal(separate, unseparated) {
		t.Error("separate arguments encode the same as the unseparated string")
	}
}

func TestFlagsAfterTheContentAreContent(t *testing.T) {
	trailing, _, err := invoke(t, "-t", "hello", "-i")
	if err != nil {
		t.Fatalf("run(-t hello -i) error = %v, want nil", err)
	}
	literal, _, err := invoke(t, "-t", "hello -i")
	if err != nil {
		t.Fatalf("run(-t 'hello -i') error = %v, want nil", err)
	}

	if !bytes.Equal(trailing, literal) {
		t.Error("a flag after the content was parsed as a flag, not encoded as content")
	}
}

func TestUnencodableContentIsAnErrorWithoutUsage(t *testing.T) {
	stdout, stderr, err := invoke(t, "-t", "")

	if err == nil {
		t.Fatal("run() with empty content returned no error")
	}
	if err.Error() != "no data to encode" {
		t.Errorf("run() error = %q, want %q", err, "no data to encode")
	}
	if len(stdout) != 0 {
		t.Errorf("run() wrote %d bytes to stdout, want none", len(stdout))
	}
	// Content that cannot be encoded is not a misuse of the command line, so
	// the usage text would be noise.
	if stderr != "" {
		t.Errorf("run() stderr = %q, want nothing", stderr)
	}
}

func TestTextArtIgnoresTheOutputFileFlag(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "branded")

	stdout, _, err := invoke(t, "-t", "-o", prefix, "hello")
	if err != nil {
		t.Fatalf("run(-t -o %s hello) error = %v, want nil", prefix, err)
	}

	// Text art goes to stdout whatever -o says, and writes no image.
	if lines := textArtLines(t, stdout); len(lines) != 29 {
		t.Errorf("text art has %d rows, want 29", len(lines))
	}
	if _, err := os.Stat(prefix + ".png"); !os.IsNotExist(err) {
		t.Errorf("run(-t -o ...) wrote %s.png, want no image", prefix)
	}
}
