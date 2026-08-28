// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests use zbarimg to decode generated QR Codes to ensure they are
// readable. sudo apt-get install zbar-tools, or download from
// http://zbar.sourceforge.net.
//
// By default these tests are disabled to avoid a dependency on zbarimg if
// you're not running the tests. Use the -test-decode flag (go test
// -test-decode) to enable.

var testDecode *bool = flag.Bool("test-decode",
	false,
	"Enable decode tests. Requires zbarimg installed.")

var testDecodeFuzz *bool = flag.Bool("test-decode-fuzz",
	false,
	"Enable decode fuzz tests. Requires zbarimg installed.")

func TestDecodeBasic(t *testing.T) {
	if !*testDecode {
		t.Skip("Decode tests not enabled")
	}

	tests := []struct {
		content        string
		numRepetitions int
		level          RecoveryLevel
	}{
		{
			"A",
			1,
			Low,
		},
		{
			"A",
			1,
			Medium,
		},
		{
			"A",
			1,
			High,
		},
		{
			"A",
			1,
			Highest,
		},
		{
			"01234567",
			1,
			Medium,
		},
	}

	for _, test := range tests {
		content := strings.Repeat(test.content, test.numRepetitions)

		q, err := New(content, test.level)
		if err != nil {
			t.Error(err.Error())
		}

		err = zbarimgCheck(t, q)

		if err != nil {
			t.Error(err.Error())
		}
	}
}

func TestDecodeAllVersionLevels(t *testing.T) {
	if !*testDecode {
		t.Skip("Decode tests not enabled")
	}

	for version := 1; version <= 40; version++ {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			t.Logf("Version=%d Level=%d",
				version,
				level)

			q, err := NewWithForcedVersion(
				fmt.Sprintf("v-%d l-%d", version, level), version, level)
			if err != nil {
				t.Fatal(err.Error())
				return
			}

			err = zbarimgCheck(t, q)

			if err != nil {
				t.Errorf("Version=%d Level=%d, err=%s, expected success",
					version,
					level,
					err.Error())
				continue
			}
		}
	}
}

func TestDecodeAllCharacters(t *testing.T) {
	if !*testDecode {
		t.Skip("Decode tests not enabled")
	}

	var content string

	// zbarimg has trouble with null bytes, hence start from ASCII 1.
	for i := 1; i < 256; i++ {
		content += string(rune(i))
	}

	q, err := New(content, Low)
	if err != nil {
		t.Error(err.Error())
	}

	err = zbarimgCheck(t, q)

	if err != nil {
		t.Error(err.Error())
	}
}

func TestDecodeFuzz(t *testing.T) {
	if !*testDecodeFuzz {
		t.Skip("Decode fuzz tests not enabled")
	}

	r := rand.New(rand.NewSource(0))

	const iterations int = 32
	const maxLength int = 128

	for i := 0; i < iterations; i++ {
		len := r.Intn(maxLength-1) + 1

		var content string
		for j := 0; j < len; j++ {
			// zbarimg seems to have trouble with special characters, test printable
			// characters only for now.
			content += string(rune(32 + r.Intn(94)))
		}

		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			q, err := New(content, level)
			if err != nil {
				t.Error(err.Error())
			}

			err = zbarimgCheck(t, q)

			if err != nil {
				t.Error(err.Error())
			}
		}
	}
}

// TestDecodeWithLogo puts the fit check in front of a real decoder: a logo
// the package accepts, at the largest scale it says it will accept, must
// leave a symbol that still reads back. Everything else about the guarantee
// is measured in codewords; this is the only test that measures it in
// scanners.
//
// The assertion runs one way only. Beyond the limit a code will usually still
// decode in a clean synthetic test — the half-capacity budget is deliberately
// conservative, and what the withheld half pays for is print and optics, not
// zbarimg — so asserting that an oversized logo fails would fail for the right
// reason while looking like a defect.
//
// The versions are chosen so that the centre of the symbol lands on an
// alignment pattern for some and not for others, which is the geometry that
// decides whether a centred knockout has to swallow one (ADR-0002).
//
// The logo is solid black, which is the worst case for damage: every module
// under the knockout is wrong and none of them is accidentally right. The
// adversarial case for the downscaler — a rendered QR Code used as the logo —
// is asserted where it belongs, at the rendering seam, because a scannable
// mark inside a scannable symbol tests a decoder's ability to tell two codes
// apart rather than anything this package decides.
func TestDecodeWithLogo(t *testing.T) {
	if !*testDecode {
		t.Skip("Decode tests not enabled")
	}

	onAnAlignmentPattern := []int{7, 10, 21, 35, 38}
	clearOfOne := []int{1, 5, 14, 20, 40}

	assertCentresAreOnAlignmentPatterns(t, onAnAlignmentPattern, true)
	assertCentresAreOnAlignmentPatterns(t, clearOfOne, false)

	logo := solidLogo(color.Black, 64, 64)

	for _, versionNumber := range append(onAnAlignmentPattern, clearOfOne...) {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			decodeWithLargestLogo(t, logo, versionNumber, level)
		}
	}
}

// decodeWithLargestLogo attaches logo to a symbol of the given version and
// recovery level at the largest scale the package will accept, and reads the
// result back through zbarimg.
func decodeWithLargestLogo(t *testing.T, logo image.Image, versionNumber int,
	level RecoveryLevel) {

	t.Helper()

	// A single digit fits every version at every level, so what the logo may
	// cover is decided by the version and level alone.
	q, err := NewWithForcedVersion("1", versionNumber, level)
	if err != nil {
		t.Fatalf("NewWithForcedVersion(v%d, level %d): %s",
			versionNumber, level, err)
	}

	scale := largestAcceptedScale(t, q, logo)
	if scale == 0 {
		return
	}

	options := DefaultLogoOptions()
	options.Scale = scale

	if err := q.SetLogo(logo, options); err != nil {
		t.Errorf("v%d level %d: the largest accepted scale %v was refused: %s",
			versionNumber, level, scale, err)
		return
	}

	if err := zbarimgCheck(t, q); err != nil {
		t.Errorf("v%d level %d at scale %v: %s",
			versionNumber, level, scale, err)
	}
}

// assertCentresAreOnAlignmentPatterns fails the test unless every one of
// versionNumbers does — or does not — put an alignment pattern's centre on
// the centre module of the symbol, so that the matrix cannot quietly lose the
// distinction it was chosen to cover.
func assertCentresAreOnAlignmentPatterns(t *testing.T, versionNumbers []int,
	want bool) {

	t.Helper()

	for _, versionNumber := range versionNumbers {
		// A symbol is 4*version + 17 modules wide (CONTEXT.md).
		centre := (4*versionNumber + 17 - 1) / 2

		on := false
		for _, c := range alignmentPatternCenter[versionNumber] {
			if c == centre {
				on = true
			}
		}

		if on != want {
			t.Fatalf("v%d has its centre on an alignment pattern: %t, want %t",
				versionNumber, on, want)
		}
	}
}

// largestAcceptedScale asks q for the largest logo it would accept, by
// offering one far too large and reading the answer off the refusal.
func largestAcceptedScale(t *testing.T, q *QRCode, logo image.Image) float64 {
	t.Helper()

	options := DefaultLogoOptions()
	options.Scale = 1

	err := q.SetLogo(logo, options)

	var tooLarge *LogoTooLargeError
	var occludes *LogoOccludesFunctionPatternError

	switch {
	case errors.As(err, &tooLarge):
		return tooLarge.MaxScale
	case errors.As(err, &occludes):
		return occludes.MaxScale
	}

	t.Fatalf("a logo covering the whole symbol was not refused: %v", err)

	return 0
}

// TestFailedSymbolIsWrittenOutsideTheWorkingTree pins the one guarantee a
// round-trip failure owes the repository: the symbol it could not read back is
// left somewhere a person can open and read, and nowhere `git status` will
// report.
//
// It is deliberately not gated behind -test-decode. The litter it guards
// against appears only when zbarimg is installed and a decode fails, which is
// precisely when nobody is watching for it, so the guarantee is checked on
// every ordinary `go test ./...` instead.
func TestFailedSymbolIsWrittenOutsideTheWorkingTree(t *testing.T) {
	q, err := New("a symbol that did not read back", Medium)
	if err != nil {
		t.Fatalf("New: %s", err)
	}

	path, err := writeFailedSymbol(t, q)
	if err != nil {
		t.Fatalf("writeFailedSymbol: %s", err)
	}

	// This symbol is a sample, not evidence of a real failure, so it does not
	// get the reprieve the directory grants the ones that are.
	t.Cleanup(func() { os.Remove(path) })

	workingTree, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %s", err)
	}

	if strings.HasPrefix(path, workingTree+string(os.PathSeparator)) {
		t.Errorf("the symbol was written to %s, inside the working tree %s",
			path, workingTree)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("the symbol was reported at %s: %s", path, err)
	}
	defer file.Close()

	written, err := png.Decode(file)
	if err != nil {
		t.Fatalf("the symbol at %s is not a PNG: %s", path, err)
	}

	// At least, rather than exactly: Write silently enlarges an image too
	// small to hold the symbol, and what this asserts is that the evidence is
	// legible, not how Write scales.
	if got := written.Bounds().Dx(); got < failedImageSize {
		t.Errorf("the symbol at %s is %d pixels wide, want at least %d",
			path, got, failedImageSize)
	}
}

func zbarimgCheck(t testing.TB, q *QRCode) error {
	t.Helper()

	s, err := zbarimgDecode(q)
	if err != nil {
		return err
	}

	if s == q.Content {
		return nil
	}

	// The path is logged here, rather than folded into the returned error,
	// so that the criterion belongs to the code that writes the file instead
	// of to every caller remembering to report the error it gets back.
	if path, err := writeFailedSymbol(t, q); err != nil {
		t.Logf("the symbol that did not read back could not be written out: %s",
			err)
	} else {
		t.Logf("the symbol that did not read back is at %s", path)
	}

	return fmt.Errorf("got '%s' (%x) expected '%s' (%x)", s, s, q.Content, q.Content)
}

// failedImageSize is the width in pixels of the image written out after a
// failed round trip — large enough to read modules off by eye. It is an image
// width, so it spans the quiet zone as well as the symbol (CONTEXT.md).
const failedImageSize = 256

// writeFailedSymbol writes q to a PNG in t's own directory under the system
// temporary directory and returns the path, so that a symbol which would not
// read back can be looked at afterwards without leaving a file in the working
// tree.
//
// The directory is derived from t.Name() rather than remembered, so every
// failure within one test collects in one place with no shared state to guard
// — a test that fails all one hundred and sixty of its round trips leaves one
// directory, not one hundred and sixty.
//
// It also outlives the run, on purpose. t.TempDir is the idiomatic home for a
// file a test creates, and is what the rest of this package uses, but it is
// removed once the test completes — which would delete the symbol before
// anyone could open the path that was logged. The working tree must not
// collect litter; the developer still needs the evidence. A directory under
// the system temporary directory serves both, and the operating system
// reclaims it in its own time.
func writeFailedSymbol(t testing.TB, q *QRCode) (string, error) {
	directory := filepath.Join(os.TempDir(), "go-qrcode-failed-symbols", t.Name())

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("creating %s for it: %w", directory, err)
	}

	// Naming the file after q.Content is not an option: the two hundred and
	// fifty-five characters TestDecodeAllCharacters encodes hex-encode to a
	// five hundred and ten character name, longer than any mainstream
	// filesystem accepts.
	file, err := os.CreateTemp(directory, "symbol-*.png")
	if err != nil {
		return "", fmt.Errorf("creating a file for it in %s: %w", directory, err)
	}

	if err := q.Write(failedImageSize, file); err != nil {
		file.Close()
		return "", fmt.Errorf("writing it to %s: %w", file.Name(), err)
	}

	if err := file.Close(); err != nil {
		return "", fmt.Errorf("closing %s: %w", file.Name(), err)
	}

	return file.Name(), nil
}

func zbarimgDecode(q *QRCode) (string, error) {
	// 512x512px
	encoded, err := q.PNG(512)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("zbarimg", "--quiet", "-Sdisable",
		"-Sqrcode.enable", "-")

	var out bytes.Buffer

	cmd.Stdin = bytes.NewBuffer(encoded)
	cmd.Stdout = &out

	err = cmd.Run()

	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(strings.TrimPrefix(out.String(), "QR-Code:"), "\n"), nil
}

func BenchmarkDecodeTest(b *testing.B) {
	if !*testDecode {
		b.Skip("Decode benchmarks not enabled")
	}

	for n := 0; n < b.N; n++ {
		q, err := New("content", Medium)
		if err != nil {
			b.Error(err.Error())
		}

		err = zbarimgCheck(b, q)

		if err != nil {
			b.Error(err.Error())
		}
	}
}
