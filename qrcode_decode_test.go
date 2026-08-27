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
	"math/rand"
	"os/exec"
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

		err = zbarimgCheck(q)

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

			err = zbarimgCheck(q)

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

	err = zbarimgCheck(q)

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

			err = zbarimgCheck(q)

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
// The logo is solid black, which is the worst case: every module under the
// knockout is wrong, and none of them is accidentally right.
func TestDecodeWithLogo(t *testing.T) {
	if !*testDecode {
		t.Skip("Decode tests not enabled")
	}

	logo := solidLogo(color.Black, 64, 64)

	contents := []string{
		"A",
		"https://example.org/a/moderately/long/path",
		strings.Repeat("0123456789", 60),
	}

	for _, content := range contents {
		for _, level := range []RecoveryLevel{Low, Medium, High, Highest} {
			q, err := New(content, level)
			if err != nil {
				t.Fatalf("New(level %d): %s", level, err)
			}

			scale := largestAcceptedScale(t, q, logo)
			if scale == 0 {
				continue
			}

			options := DefaultLogoOptions()
			options.Scale = scale

			if err := q.SetLogo(logo, options); err != nil {
				t.Errorf("v%d level %d: the largest accepted scale %v was "+
					"refused: %s", q.VersionNumber, level, scale, err)
				continue
			}

			if err := zbarimgCheck(q); err != nil {
				t.Errorf("v%d level %d with a logo of scale %v: %s",
					q.VersionNumber, level, scale, err)
			}
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

func zbarimgCheck(q *QRCode) error {
	s, err := zbarimgDecode(q)
	if err != nil {
		return err
	}

	if s != q.Content {
		q.WriteFile(256, fmt.Sprintf("%x.png", q.Content))
		return fmt.Errorf("got '%s' (%x) expected '%s' (%x)", s, s, q.Content, q.Content)
	}

	return nil
}

func zbarimgDecode(q *QRCode) (string, error) {
	var png []byte

	// 512x512px
	png, err := q.PNG(512)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("zbarimg", "--quiet", "-Sdisable",
		"-Sqrcode.enable", "-")

	var out bytes.Buffer

	cmd.Stdin = bytes.NewBuffer(png)
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

		err = zbarimgCheck(q)

		if err != nil {
			b.Error(err.Error())
		}
	}
}
