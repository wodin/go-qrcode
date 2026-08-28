// go-qrcode
// Copyright 2014 Tom Harwood
/*
	Amendments Thu, 2017-December-14:
	- test integration (go test -v)
	- idiomatic go code
*/
package qrcode

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"testing"
)

func TestExampleEncode(t *testing.T) {
	if png, err := Encode("https://example.org", Medium, 256); err != nil {
		t.Errorf("Error: %s", err.Error())
	} else {
		fmt.Printf("PNG is %d bytes long", len(png))
	}
}

func TestExampleWriteFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "example.png")
	if err := WriteFile("https://example.org", Medium, 256, filename); err != nil {
		t.Errorf("WriteFile: %s", err.Error())
	}
}

func TestExampleEncodeWithColourAndWithoutBorder(t *testing.T) {
	q, err := New("https://example.org", Medium)
	if err != nil {
		t.Errorf("Error: %s", err)
		return
	}

	// Optionally, disable the QR Code border.
	q.DisableBorder = true

	// Optionally, set the colours.
	q.ForegroundColor = color.RGBA{R: 0x33, G: 0x33, B: 0x66, A: 0xff}
	q.BackgroundColor = color.RGBA{R: 0xef, G: 0xef, B: 0xef, A: 0xff}

	err = q.WriteFile(256, filepath.Join(t.TempDir(), "example2.png"))
	if err != nil {
		t.Errorf("Error: %s", err)
		return
	}
}

// TestExampleLogo runs the logo examples in README.md, so that a snippet a
// reader copies is one the package has actually accepted. The content is long
// enough to make a version 5 symbol, which carries a scale 0.15 logo; a
// shorter URL makes a smaller symbol and would be refused.
func TestExampleLogo(t *testing.T) {
	logo := image.NewRGBA(image.Rect(0, 0, 64, 64))

	q, err := New("https://example.org/campaigns/spring-sale", Highest)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	// Attach the largest logo the QR Code safely carries, with a one module
	// margin around it.
	if err := q.FitLogo(logo, 1); err != nil {
		t.Errorf("FitLogo: %s", err)
	}

	// Or attach a logo of a size you choose.
	options := DefaultLogoOptions()
	options.Scale = 0.15

	if err := q.SetLogo(logo, options); err != nil {
		t.Errorf("SetLogo: %s", err)
	}

	// Or ask what fits before attaching anything.
	if scale := q.MaxLogoScale(1); scale < options.Scale {
		t.Errorf("MaxLogoScale = %v, want at least the %v the example attaches",
			scale, options.Scale)
	}
}
