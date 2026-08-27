// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import "testing"

// knockoutWidths returns the width of the knockout of every scale from 0 to 1
// in steps small enough that no module width is stepped over.
func knockoutWidths(t *testing.T, symbolSize int, margin int) []int {
	t.Helper()

	widths := []int{}

	for i := 0; i <= 400; i++ {
		widths = append(widths,
			newKnockout(symbolSize, float64(i)/400, margin).width())
	}

	return widths
}

func TestKnockoutStaysCentred(t *testing.T) {
	for versionNumber := 1; versionNumber <= 40; versionNumber++ {
		symbolSize := getQRCodeVersion(Low, versionNumber).symbolSize()

		for _, margin := range []int{0, 1, 4} {
			for i := 0; i <= 40; i++ {
				scale := float64(i) / 40
				k := newKnockout(symbolSize, scale, margin)

				// A symbol is always an odd number of modules wide, so a
				// centred square is centred on the middle module: its low and
				// high edges are equally far from each end of the symbol.
				if k.min.x+k.max.x != symbolSize || k.min.y+k.max.y != symbolSize {
					t.Errorf("v%d margin=%d scale=%.3f: knockout %v is not centred in a %d module symbol",
						versionNumber, margin, scale, k, symbolSize)
				}

				if k.width()%2 != 1 {
					t.Errorf("v%d margin=%d scale=%.3f: knockout width = %d, want an odd number",
						versionNumber, margin, scale, k.width())
				}
			}
		}
	}
}

func TestKnockoutGrowsWithScale(t *testing.T) {
	widths := knockoutWidths(t, 57, 1)

	for i := 1; i < len(widths); i++ {
		if widths[i] < widths[i-1] {
			t.Fatalf("width shrank from %d to %d as scale grew", widths[i-1], widths[i])
		}
	}
}

func TestKnockoutOfAVanishinglySmallLogoIsItsMargin(t *testing.T) {
	for _, margin := range []int{0, 1, 4} {
		k := newKnockout(57, 0, margin)

		// The logo covers the centre module alone, and the margin surrounds
		// it on every side.
		if want := 1 + 2*margin; k.width() != want {
			t.Errorf("margin=%d: knockout width = %d, want %d", margin, k.width(), want)
		}
	}
}

func TestKnockoutCountsTheMarginOnBothSides(t *testing.T) {
	const symbolSize = 57

	for i := 1; i <= 20; i++ {
		scale := float64(i) / 20

		bare := newKnockout(symbolSize, scale, 0).width()
		margined := newKnockout(symbolSize, scale, 3).width()

		if want := bare + 6; margined != want {
			t.Errorf("scale=%.2f: knockout width = %d with a 3 module margin and %d without, want %d",
				scale, margined, bare, want)
		}
	}
}

func TestKnockoutSnapsOutwardsToWholeModules(t *testing.T) {
	const symbolSize = 57

	tests := []struct {
		name        string
		logoModules float64
		want        int
	}{
		{"exactly three modules", 3, 3},
		{"a hair over three modules", 3.02, 5},
		{"a hair under five modules", 4.98, 5},
		{"exactly five modules", 5, 5},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			k := newKnockout(symbolSize, test.logoModules/symbolSize, 0)

			if k.width() != test.want {
				t.Errorf("a %.2f module logo knocks out %d modules, want %d",
					test.logoModules, k.width(), test.want)
			}
		})
	}
}

func TestKnockoutClipsToTheSymbol(t *testing.T) {
	const symbolSize = 21

	k := newKnockout(symbolSize, 1, 4).clip(symbolSize)

	want := knockout{
		min: modulePosition{0, 0},
		max: modulePosition{symbolSize, symbolSize},
	}

	if k != want {
		t.Errorf("clipped knockout = %v, want %v", k, want)
	}
}
