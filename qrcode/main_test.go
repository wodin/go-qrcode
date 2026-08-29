// go-qrcode
// Copyright 2014 Tom Harwood

package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qrcode "github.com/skip2/go-qrcode"
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

// pixelAt returns the colour of one pixel of a rendered image, whatever
// colour model the image itself was decoded into.
func pixelAt(img image.Image, x, y int) color.RGBA {
	return color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
}

// cornerColour returns the top left pixel of a rendered image, which lies in
// the quiet zone and therefore carries the background colour.
func cornerColour(t *testing.T, stdout []byte) color.RGBA {
	t.Helper()

	img := decodedImage(t, stdout)
	b := img.Bounds()
	return pixelAt(img, b.Min.X, b.Min.Y)
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

// brandedContent is content long enough to carry a logo at the default
// scale. The tool always encodes at the Highest recovery level, where a fifth
// of the symbol's width first fits at version 6 — a version 1 symbol carrying
// "hello" has no room for one.
const brandedContent = "https://example.org/campaigns/spring-sale/branded-qr-code"

// logoColour fills every test logo. It is a colour neither the symbol nor
// the quiet zone carries, so finding it in the output proves the logo was
// drawn rather than a module.
var logoColour = color.RGBA{R: 255, A: 255}

// nearLogoColour reports whether c is close enough to logoColour to be it,
// allowing for what a lossy format or a palette does to a colour on the way
// through.
func nearLogoColour(c color.RGBA) bool {
	return c.R > 200 && c.G < 60 && c.B < 60
}

// writeLogo encodes a solid logoColour square in the format named by suffix
// and returns its path. The file lives in a per-test temporary directory, so
// no test logo is ever tracked.
func writeLogo(t *testing.T, suffix string) string {
	t.Helper()

	logo := image.NewRGBA(image.Rect(0, 0, 64, 64))
	draw.Draw(logo, logo.Bounds(), &image.Uniform{C: logoColour},
		image.Point{}, draw.Src)

	path := filepath.Join(t.TempDir(), "logo"+suffix)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating the test logo: %v", err)
	}
	defer file.Close()

	switch suffix {
	case ".png":
		err = png.Encode(file, logo)
	case ".jpg":
		err = jpeg.Encode(file, logo, nil)
	case ".gif":
		err = gif.Encode(file, logo, nil)
	default:
		t.Fatalf("no encoder for a %q logo", suffix)
	}
	if err != nil {
		t.Fatalf("encoding the test logo: %v", err)
	}

	return path
}

// centreColour returns the middle pixel of a rendered image, which an
// attached logo covers.
func centreColour(t *testing.T, stdout []byte) color.RGBA {
	t.Helper()

	img := decodedImage(t, stdout)
	b := img.Bounds()
	return pixelAt(img, b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2)
}

func TestLogoFlagPlacesTheLogoInTheCentre(t *testing.T) {
	logo := writeLogo(t, ".png")

	stdout, stderr, err := invoke(t, "-L", logo, brandedContent)
	if err != nil {
		t.Fatalf("run(-L %s <content>) error = %v, want nil", logo, err)
	}

	// The image goes to stdout whatever the tool has to say about the scale
	// it picked, so a note about the fit never lands in a piped PNG.
	if !strings.Contains(stderr, "logo scaled to") {
		t.Errorf("run(-L ...) stderr = %q, want the fitted scale reported", stderr)
	}

	if got := centreColour(t, stdout); !nearLogoColour(got) {
		t.Errorf("the centre of the image is %v, want the logo colour %v",
			got, logoColour)
	}
}

func TestTheLongLogoFlagNamesTheSameFileAsTheShortOne(t *testing.T) {
	logo := writeLogo(t, ".png")

	short, _, err := invoke(t, "-L", logo, brandedContent)
	if err != nil {
		t.Fatalf("run(-L %s <content>) error = %v, want nil", logo, err)
	}
	long, _, err := invoke(t, "-logo", logo, brandedContent)
	if err != nil {
		t.Fatalf("run(-logo %s <content>) error = %v, want nil", logo, err)
	}
	if !bytes.Equal(short, long) {
		t.Error("-logo produced a different image from -L")
	}
}

// The lowercase letter is held back for a flag the tool has yet to want, so
// -l must stay undefined rather than quietly becoming a second spelling.
func TestTheLowercaseLogoLetterIsUnused(t *testing.T) {
	logo := writeLogo(t, ".png")

	_, stderr, err := invoke(t, "-l", logo, brandedContent)
	if err == nil {
		t.Fatal("run(-l ...) returned no error, want -l to be undefined")
	}
	if !strings.Contains(stderr, "flag provided but not defined: -l") {
		t.Errorf("run(-l ...) stderr = %q, want -l to be undefined", stderr)
	}
}

// logoWidth returns the number of logo-coloured pixels across the middle row
// of a rendered image, which is how wide the logo was drawn.
func logoWidth(t *testing.T, stdout []byte) int {
	t.Helper()

	img := decodedImage(t, stdout)
	b := img.Bounds()
	y := b.Min.Y + b.Dy()/2

	width := 0
	for x := b.Min.X; x < b.Max.X; x++ {
		if nearLogoColour(pixelAt(img, x, y)) {
			width++
		}
	}
	return width
}

// shortContent is content a version 1 symbol carries at the Highest recovery
// level, where the default scale of 0.2 has never fitted. Fitting the logo is
// what lets the tool brand it at all.
const shortContent = "hello"

func TestALogoIsFittedWhenNoScaleIsGiven(t *testing.T) {
	logo := writeLogo(t, ".png")

	stdout, stderr, err := invoke(t, "-L", logo, shortContent)
	if err != nil {
		t.Fatalf("run(-L %s hello) error = %v, want nil: the tool picks a scale that fits", logo, err)
	}

	if got := centreColour(t, stdout); !nearLogoColour(got) {
		t.Errorf("the centre of the image is %v, want the logo colour %v", got, logoColour)
	}

	// The size was the tool's to choose, so it says what it chose and which
	// symbol decided it — the two things a caller needs to ask for the same
	// result explicitly.
	q, err := qrcode.New(shortContent, qrcode.Highest)
	if err != nil {
		t.Fatalf("qrcode.New: %v", err)
	}

	scale := fmt.Sprintf("%.4f", q.MaxLogoScale(qrcode.DefaultLogoOptions().Margin))
	version := fmt.Sprintf("version %d", q.VersionNumber)

	for _, want := range []string{scale, version} {
		if !strings.Contains(stderr, want) {
			t.Errorf("run(-L ...) stderr = %q, want it to report %q", stderr, want)
		}
	}
}

func TestAnExplicitLogoScaleIsSeatedExactlyOrRefused(t *testing.T) {
	logo := writeLogo(t, ".png")

	// Exactly what was asked for, and no note about a choice the tool did not
	// make.
	stated, stderr, err := invoke(t, "-L", logo, "-logo-scale", "0.2", brandedContent)
	if err != nil {
		t.Fatalf("run(-logo-scale 0.2 ...) error = %v, want nil", err)
	}
	if stderr != "" {
		t.Errorf("run(-logo-scale 0.2 ...) stderr = %q, want nothing: the caller chose the scale", stderr)
	}

	fitted, _, err := invoke(t, "-L", logo, brandedContent)
	if err != nil {
		t.Fatalf("run(-L ...) error = %v, want nil", err)
	}
	if bytes.Equal(stated, fitted) {
		t.Error("an explicit 0.2 produced the fitted logo, so the flag chose nothing")
	}

	// And a scale the symbol cannot carry is still refused rather than
	// quietly shrunk to one it can.
	_, _, err = invoke(t, "-L", logo, "-logo-scale", "0.6", brandedContent)
	if err == nil {
		t.Fatal("run(-logo-scale 0.6 ...) returned no error, want the logo refused")
	}
	if !strings.Contains(err.Error(), "largest accepted scale") {
		t.Errorf("run(-logo-scale 0.6 ...) error = %q, want the scale that would fit", err)
	}
}

func TestLogoScaleFlagSetsTheLogoWidth(t *testing.T) {
	logo := writeLogo(t, ".png")

	byDefault, _, err := invoke(t, "-L", logo, brandedContent)
	if err != nil {
		t.Fatalf("run(-L ...) error = %v, want nil", err)
	}

	narrow, _, err := invoke(t, "-L", logo, "-logo-scale", "0.1", brandedContent)
	if err != nil {
		t.Fatalf("run(-logo-scale 0.1 ...) error = %v, want nil", err)
	}
	if logoWidth(t, narrow) >= logoWidth(t, byDefault) {
		t.Errorf("a 0.1 logo is %d pixels wide, no narrower than the %d of a "+
			"fitted one", logoWidth(t, narrow), logoWidth(t, byDefault))
	}
}

func TestAScaleWithoutALogoIsAnError(t *testing.T) {
	stdout, _, err := invoke(t, "-logo-scale", "0.1", brandedContent)

	if err == nil {
		t.Fatal("run(-logo-scale ...) without a logo returned no error")
	}
	if !strings.Contains(err.Error(), "-logo") {
		t.Errorf("run(-logo-scale ...) error = %q, want it to name -logo", err)
	}
	if len(stdout) != 0 {
		t.Errorf("run(-logo-scale ...) wrote %d bytes to stdout, want none",
			len(stdout))
	}
}

func TestJPEGAndGIFLogosAreAcceptedToo(t *testing.T) {
	for _, suffix := range []string{".jpg", ".gif"} {
		t.Run(suffix, func(t *testing.T) {
			logo := writeLogo(t, suffix)

			stdout, _, err := invoke(t, "-L", logo, brandedContent)
			if err != nil {
				t.Fatalf("run(-L %s ...) error = %v, want nil", logo, err)
			}
			if got := centreColour(t, stdout); !nearLogoColour(got) {
				t.Errorf("the centre of the image is %v, want the logo colour",
					got)
			}
		})
	}
}

func TestALogoCannotBeShownAsTextArt(t *testing.T) {
	logo := writeLogo(t, ".png")

	stdout, _, err := invoke(t, "-t", "-L", logo, brandedContent)

	if err == nil {
		t.Fatal("run(-t -L ...) returned no error, want text art to refuse a logo")
	}
	if !strings.Contains(err.Error(), "-t") {
		t.Errorf("run(-t -L ...) error = %q, want it to name -t", err)
	}
	if len(stdout) != 0 {
		t.Errorf("run(-t -L ...) wrote %d bytes to stdout, want none", len(stdout))
	}
}

// logoError returns the failure of a run given the logo file named by path,
// which the test expects to fail.
func logoError(t *testing.T, path string) string {
	t.Helper()

	_, _, err := invoke(t, "-L", path, brandedContent)
	if err == nil {
		t.Fatalf("run(-L %s ...) returned no error", path)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("run(-L %s ...) error = %q, want it to name the file", path, err)
	}

	return err.Error()
}

func TestEachWayALogoFileCanBeUnusableIsReportedDifferently(t *testing.T) {
	dir := t.TempDir()
	intact := writeLogo(t, ".png")

	// A file that is there but is not an image, whatever its name promises.
	prose := filepath.Join(dir, "prose.png")
	if err := os.WriteFile(prose, []byte("not an image"), 0o600); err != nil {
		t.Fatalf("writing the file that is not an image: %v", err)
	}

	// A PNG that is a PNG, and damaged. The format is not the problem here,
	// so advice about formats would send the reader the wrong way.
	whole, err := os.ReadFile(intact)
	if err != nil {
		t.Fatalf("reading the test logo back: %v", err)
	}
	truncated := filepath.Join(dir, "truncated.png")
	if err := os.WriteFile(truncated, whole[:len(whole)/2], 0o600); err != nil {
		t.Fatalf("writing the truncated PNG: %v", err)
	}

	missingErr := logoError(t, filepath.Join(dir, "absent.png"))
	proseErr := logoError(t, prose)
	truncatedErr := logoError(t, truncated)

	if !strings.Contains(missingErr, "no such file") {
		t.Errorf("a missing file reports %q, want it to say the file is not there",
			missingErr)
	}
	if !strings.Contains(proseErr, "not a PNG, JPEG or GIF") {
		t.Errorf("a file that is not an image reports %q, want the formats that "+
			"would have worked", proseErr)
	}
	if strings.Contains(truncatedErr, "not a PNG") {
		t.Errorf("a damaged PNG reports %q, blaming its format", truncatedErr)
	}
	if !strings.Contains(truncatedErr, "png:") {
		t.Errorf("a damaged PNG reports %q, want the decoder's own complaint",
			truncatedErr)
	}
}

func TestAnOversizedLogoReportsTheLargestScaleThatFits(t *testing.T) {
	logo := writeLogo(t, ".png")

	stdout, _, err := invoke(t, "-L", logo, "-logo-scale", "0.9", brandedContent)

	if err == nil {
		t.Fatal("run(-logo-scale 0.9 ...) returned no error, want a refusal")
	}
	if !strings.Contains(err.Error(), "largest accepted scale is") {
		t.Errorf("run(-logo-scale 0.9 ...) error = %q, want the largest "+
			"accepted scale", err)
	}
	if len(stdout) != 0 {
		t.Errorf("a refused logo wrote %d bytes to stdout, want none", len(stdout))
	}
}

func TestAnInvertedImageCarriesTheLogoToo(t *testing.T) {
	logo := writeLogo(t, ".png")

	stdout, _, err := invoke(t, "-i", "-L", logo, brandedContent)
	if err != nil {
		t.Fatalf("run(-i -L ...) error = %v, want nil", err)
	}

	black := color.RGBA{A: 255}
	if got := cornerColour(t, stdout); got != black {
		t.Errorf("inverted quiet zone is %v, want black", got)
	}
	if got := centreColour(t, stdout); !nearLogoColour(got) {
		t.Errorf("the centre of the inverted image is %v, want the logo colour",
			got)
	}
}

func TestUsageDocumentsTheLogoFlags(t *testing.T) {
	_, stderr, err := invoke(t, "-h")
	if err != nil {
		t.Fatalf("run(-h) error = %v, want nil", err)
	}

	// What -logo-scale defaults to is the fitted scale, not the package's
	// 0.2: a caller reading the usage text must not be told a number the
	// tool never uses.
	for _, want := range []string{"-L", "-logo ", "-logo-scale", "-grow-symbol",
		"(default: the largest that fits)"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the usage text does not mention %q:\n%s", want, stderr)
		}
	}
}

func TestGrowSymbolEncodesAtTheSmallestVersionCarryingTheScale(t *testing.T) {
	logo := writeLogo(t, ".png")

	// Five characters make a version 1 symbol, which carries a logo of
	// 0.0476: without a larger symbol the content's length caps the logo, and
	// asking for a fifth of the width is refused however the flags are
	// arranged.
	if _, _, err := invoke(t, "-L", logo, "-logo-scale", "0.2", shortContent); err == nil {
		t.Fatal("run(-logo-scale 0.2 hello) returned no error, so the symbol the content chose already carries the scale")
	}

	stdout, stderr, err := invoke(t, "-L", logo, "-logo-scale", "0.2",
		"-grow-symbol", shortContent)
	if err != nil {
		t.Fatalf("run(-logo-scale 0.2 -grow-symbol hello) error = %v, want nil", err)
	}

	if got := centreColour(t, stdout); !nearLogoColour(got) {
		t.Errorf("the centre of the image is %v, want the logo colour %v", got, logoColour)
	}

	// Version 6 is the smallest carrying a fifth of its width at the recovery
	// level the tool encodes at, and it is not reached by stepping: versions
	// 2 to 5 carry 0.1200, 0.1034, 0.1515 and 0.1892.
	if !strings.Contains(stderr, "version 6") {
		t.Errorf("run(-grow-symbol ...) stderr = %q, want the version it grew to", stderr)
	}
	if !strings.Contains(stderr, "0.2000") {
		t.Errorf("run(-grow-symbol ...) stderr = %q, want the scale it grew for", stderr)
	}

	// The point of the larger symbol is the larger logo, so the logo really
	// is drawn wider than the one the version the content chose would carry.
	fitted, _, err := invoke(t, "-L", logo, shortContent)
	if err != nil {
		t.Fatalf("run(-L ... hello) error = %v, want nil", err)
	}
	if logoWidth(t, stdout) <= logoWidth(t, fitted) {
		t.Errorf("the grown symbol's logo is %d pixels wide, no wider than the %d of the symbol the content chose",
			logoWidth(t, stdout), logoWidth(t, fitted))
	}
}

func TestGrowSymbolLeavesASymbolThatAlreadyCarriesTheScaleAlone(t *testing.T) {
	logo := writeLogo(t, ".png")

	// This content makes a version 6 symbol, which carries 0.2683, so the
	// smallest version carrying 0.2 is the one the content already chose.
	// Growing is a floor, not a bump: there is nothing to grow to here.
	stated, _, err := invoke(t, "-L", logo, "-logo-scale", "0.2", brandedContent)
	if err != nil {
		t.Fatalf("run(-logo-scale 0.2 ...) error = %v, want nil", err)
	}

	grown, stderr, err := invoke(t, "-L", logo, "-logo-scale", "0.2",
		"-grow-symbol", brandedContent)
	if err != nil {
		t.Fatalf("run(-logo-scale 0.2 -grow-symbol ...) error = %v, want nil", err)
	}

	if !bytes.Equal(stated, grown) {
		t.Error("-grow-symbol changed a symbol that already carries the scale asked for")
	}

	// Nothing was chosen, so there is nothing to report: the note belongs to
	// a version the tool picked, not to one the content did.
	if stderr != "" {
		t.Errorf("run(-grow-symbol ...) stderr = %q, want nothing: the symbol did not grow", stderr)
	}
}

func TestGrowSymbolNeedsAScaleToGrowTo(t *testing.T) {
	logo := writeLogo(t, ".png")

	// Without a scale the tool already fits the logo to the symbol, so there
	// is no size to grow the symbol to and the flag asks for nothing.
	stdout, _, err := invoke(t, "-L", logo, "-grow-symbol", brandedContent)

	if err == nil {
		t.Fatal("run(-L ... -grow-symbol) returned no error, want the flag to need a scale")
	}
	if !strings.Contains(err.Error(), "-logo-scale") {
		t.Errorf("run(-grow-symbol) without a scale reports %q, want it to name -logo-scale", err)
	}
	if len(stdout) != 0 {
		t.Errorf("run(-grow-symbol) without a scale wrote %d bytes to stdout, want none", len(stdout))
	}
}

func TestAScaleNoVersionCarriesIsRefusedWithTheLargestThatIs(t *testing.T) {
	logo := writeLogo(t, ".png")

	stdout, _, err := invoke(t, "-L", logo, "-logo-scale", "0.9", "-grow-symbol",
		brandedContent)

	if err == nil {
		t.Fatal("run(-logo-scale 0.9 -grow-symbol ...) returned no error, want a refusal")
	}

	// Growing the symbol was the lever, and it has been pulled all the way:
	// what is left to ask for is a smaller scale, and the tool says which.
	largest := fmt.Sprintf("%.4f", 0.3333)
	if !strings.Contains(err.Error(), largest) {
		t.Errorf("run(-logo-scale 0.9 -grow-symbol ...) error = %q, want the largest scale any version carries, %s",
			err, largest)
	}
	if len(stdout) != 0 {
		t.Errorf("a refused logo wrote %d bytes to stdout, want none", len(stdout))
	}
}

func TestNoScaleIsOfferedWhenNoVersionCarriesALogoAtAll(t *testing.T) {
	// The tool fixes the margin at one module, where every version carries a
	// logo of some size, so it takes a direct call to reach the case where
	// there is no scale to offer. It is reachable through the library, whose
	// margin is the caller's, and a message naming a scale of 0.0000 would be
	// advice to attach nothing.
	message := noVersionCarries(3, 0.9, 0).Error()

	if strings.Contains(message, "0.0000") {
		t.Errorf("with nothing carried the refusal says %q, want no scale offered", message)
	}
	if !strings.Contains(message, "0.9000") {
		t.Errorf("the refusal says %q, want it to name the scale asked for", message)
	}
}
