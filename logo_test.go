// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"errors"
	"image"
	"testing"
)

// testLogo returns an image standing in for a caller's logo. Nothing is
// rendered yet, so only that it is a non-empty image matters.
func testLogo() image.Image {
	return image.NewRGBA(image.Rect(0, 0, 64, 64))
}

// forcedVersion returns a QR Code of the given version and recovery level,
// failing the test if one cannot be built.
//
// The content is a single digit so that it fits a version 1 symbol at every
// recovery level: what a logo may cover depends on the version and level
// alone, never on what is encoded.
func forcedVersion(t *testing.T, versionNumber int, level RecoveryLevel) *QRCode {
	t.Helper()

	q, err := NewWithForcedVersion("1", versionNumber, level)
	if err != nil {
		t.Fatalf("NewWithForcedVersion(v%d, level %d): %s", versionNumber, level, err)
	}

	return q
}

func TestDefaultLogoOptions(t *testing.T) {
	want := LogoOptions{Scale: 0.2, Margin: 1}

	if got := DefaultLogoOptions(); got != want {
		t.Errorf("DefaultLogoOptions() = %+v, want %+v", got, want)
	}
}

func TestSetLogoAcceptsAModestLogoAtTheHighestRecoveryLevel(t *testing.T) {
	q := forcedVersion(t, 10, Highest)

	if err := q.SetLogo(testLogo(), DefaultLogoOptions()); err != nil {
		t.Fatalf("SetLogo: %s", err)
	}

	if q.logo == nil {
		t.Error("the accepted logo was not kept")
	}

	if want := DefaultLogoOptions(); q.logoOptions != want {
		t.Errorf("kept options %+v, want %+v", q.logoOptions, want)
	}
}

func TestSetLogoRefusesAnOversizedLogo(t *testing.T) {
	q := forcedVersion(t, 10, Highest)

	options := DefaultLogoOptions()
	options.Scale = 0.6

	err := q.SetLogo(testLogo(), options)

	var tooLarge *LogoTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("SetLogo returned %v, want a *LogoTooLargeError", err)
	}

	if tooLarge.Scale != options.Scale || tooLarge.Margin != options.Margin {
		t.Errorf("error reports scale %v margin %d, want the requested %v and %d",
			tooLarge.Scale, tooLarge.Margin, options.Scale, options.Margin)
	}

	if tooLarge.MaxScale <= 0 || tooLarge.MaxScale >= options.Scale {
		t.Errorf("error reports a maximum scale of %v, want one between 0 and the requested %v",
			tooLarge.MaxScale, options.Scale)
	}

	blocks := blockShapes(q.version)

	if tooLarge.Block < 0 || tooLarge.Block >= len(blocks) {
		t.Fatalf("error blames block %d, want one of the %d blocks",
			tooLarge.Block, len(blocks))
	}

	if want := blocks[tooLarge.Block].correctionCapacity(); tooLarge.Capacity != want {
		t.Errorf("error reports capacity %d for block %d, want %d",
			tooLarge.Capacity, tooLarge.Block, want)
	}

	// The blamed block must be one that actually broke the rule, or the
	// numbers in the error do not explain the refusal.
	if 2*tooLarge.DamagedCodewords <= tooLarge.Capacity {
		t.Errorf("error blames block %d for losing %d of %d correctable codewords, which is within its half share",
			tooLarge.Block, tooLarge.DamagedCodewords, tooLarge.Capacity)
	}

	if q.logo != nil {
		t.Error("a refused logo was kept")
	}
}

func TestReportedMaximumScaleIsAccepted(t *testing.T) {
	for _, versionNumber := range someVersions {
		for _, level := range everyLevel {
			t.Run(versionLevelName(versionNumber, level), func(t *testing.T) {
				q := forcedVersion(t, versionNumber, level)

				options := DefaultLogoOptions()
				options.Scale = 1

				var tooLarge *LogoTooLargeError
				var occludes *LogoOccludesFunctionPatternError

				maxScale := 0.0

				switch err := q.SetLogo(testLogo(), options); {
				case errors.As(err, &tooLarge):
					maxScale = tooLarge.MaxScale
				case errors.As(err, &occludes):
					maxScale = occludes.MaxScale
				default:
					t.Fatalf("a full width logo was not refused: %v", err)
				}

				if maxScale == 0 {
					// Nothing fits: the advice is that there is none, and the
					// smallest logo there is must be refused too.
					options.Scale = 1 / float64(q.version.symbolSize())

					if err := q.SetLogo(testLogo(), options); err == nil {
						t.Fatal("no maximum scale was offered, but a one module logo was accepted")
					}

					return
				}

				options.Scale = maxScale

				if err := q.SetLogo(testLogo(), options); err != nil {
					t.Fatalf("the reported maximum scale %v was refused: %s", maxScale, err)
				}
			})
		}
	}
}

func TestMaxLogoScaleAnswersWhatARefusalWouldReport(t *testing.T) {
	for _, versionNumber := range someVersions {
		for _, level := range everyLevel {
			t.Run(versionLevelName(versionNumber, level), func(t *testing.T) {
				q := forcedVersion(t, versionNumber, level)

				for _, margin := range []int{0, 1, 2, 5} {
					// The query is the same answer the refusal carries, or
					// the two drift and a caller who asked first is told
					// something else when they attach.
					options := LogoOptions{Scale: 1, Margin: margin}

					var tooLarge *LogoTooLargeError
					var occludes *LogoOccludesFunctionPatternError

					refused := 0.0

					switch err := q.SetLogo(testLogo(), options); {
					case errors.As(err, &tooLarge):
						refused = tooLarge.MaxScale
					case errors.As(err, &occludes):
						refused = occludes.MaxScale
					default:
						t.Fatalf("margin %d: a full width logo was not refused: %v", margin, err)
					}

					if got := q.MaxLogoScale(margin); got != refused {
						t.Errorf("margin %d: MaxLogoScale reports %v, the refusal %v",
							margin, got, refused)
					}
				}
			})
		}
	}
}

func TestMaxLogoScaleIsAskedWithoutAttachingALogo(t *testing.T) {
	q := forcedVersion(t, 10, Highest)

	scale := q.MaxLogoScale(DefaultLogoOptions().Margin)

	if scale <= 0 || scale > 1 {
		t.Fatalf("MaxLogoScale reports %v, want a fraction of the symbol's width", scale)
	}

	if q.logo != nil {
		t.Error("asking what fits attached a logo")
	}

	options := LogoOptions{Scale: scale, Margin: DefaultLogoOptions().Margin}

	if err := q.SetLogo(testLogo(), options); err != nil {
		t.Errorf("the scale MaxLogoScale reported was refused: %s", err)
	}
}

func TestMaxLogoScaleFitsNothingIntoTheSmallestLowSymbols(t *testing.T) {
	// Only these two fit no logo at all at the default margin, so they are
	// where a caller first meets a zero, and where the fitted seat has to
	// refuse rather than seat something.
	for _, versionNumber := range []int{1, 2} {
		q := forcedVersion(t, versionNumber, Low)

		if got := q.MaxLogoScale(DefaultLogoOptions().Margin); got != 0 {
			t.Errorf("version %d at Low reports a maximum scale of %v, want 0",
				versionNumber, got)
		}
	}
}

func TestMaxLogoScaleFitsNothingIntoAMarginWiderThanAnySymbol(t *testing.T) {
	q := forcedVersion(t, 40, Highest)

	// A margin is clear space paid for out of the same budget as the logo, so
	// a wide enough one leaves nothing for a logo of any size.
	if got := q.MaxLogoScale(32); got != 0 {
		t.Errorf("MaxLogoScale at a 32 module margin reports %v, want 0", got)
	}

	// A negative margin is not a narrower margin, it is not a margin: the
	// answer is that nothing fits, not a larger logo than margin 0 allows.
	if got := q.MaxLogoScale(-1); got != 0 {
		t.Errorf("MaxLogoScale at a negative margin reports %v, want 0", got)
	}
}

func TestSameLogoPassesAtAHigherRecoveryLevelAndFailsAtALower(t *testing.T) {
	options := DefaultLogoOptions()

	// A fifth of a version 10 symbol leaves every block of a Highest symbol
	// over half its correction capacity, and overruns a Low symbol's.
	if err := forcedVersion(t, 10, Highest).SetLogo(testLogo(), options); err != nil {
		t.Errorf("a scale %v logo was refused at the Highest recovery level: %s",
			options.Scale, err)
	}

	err := forcedVersion(t, 10, Low).SetLogo(testLogo(), options)

	var tooLarge *LogoTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Errorf("a scale %v logo returned %v at the Low recovery level, want a *LogoTooLargeError",
			options.Scale, err)
	}
}

func TestSetLogoPermitsAlignmentPatternOcclusion(t *testing.T) {
	// The centre of a version 7 symbol is the centre of an alignment pattern.
	q := forcedVersion(t, 7, Highest)
	options := DefaultLogoOptions()

	k := newKnockout(q.version.symbolSize(), options.Scale, options.Margin)
	function := functionPatternSymbol(q.version)
	protected := protectedFunctionPatternSymbol(q.version)

	covered := false

	for y := k.min.y; y < k.max.y; y++ {
		for x := k.min.x; x < k.max.x; x++ {
			if !function.empty(x, y) && protected.empty(x, y) {
				covered = true
			}
		}
	}

	if !covered {
		t.Fatal("the knockout covers no alignment pattern, so this proves nothing")
	}

	if err := q.SetLogo(testLogo(), options); err != nil {
		t.Errorf("SetLogo refused a logo covering only an alignment pattern: %s", err)
	}
}

func TestSetLogoRefusesFunctionPatternOcclusion(t *testing.T) {
	q := forcedVersion(t, 7, Highest)

	options := DefaultLogoOptions()
	options.Scale = 1

	err := q.SetLogo(testLogo(), options)

	var occludes *LogoOccludesFunctionPatternError
	if !errors.As(err, &occludes) {
		t.Fatalf("SetLogo returned %v, want a *LogoOccludesFunctionPatternError", err)
	}

	if protectedFunctionPatternSymbol(q.version).empty(occludes.X, occludes.Y) {
		t.Errorf("error blames module (%d,%d), which is not a protected function pattern",
			occludes.X, occludes.Y)
	}

	if q.logo != nil {
		t.Error("a refused logo was kept")
	}
}

func TestSetLogoRejectsUnusableArguments(t *testing.T) {
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))

	tests := []struct {
		name    string
		logo    image.Image
		options LogoOptions
	}{
		{"no image", nil, DefaultLogoOptions()},
		{"an empty image", empty, DefaultLogoOptions()},
		{"a zero scale", testLogo(), LogoOptions{Scale: 0, Margin: 1}},
		{"a negative scale", testLogo(), LogoOptions{Scale: -0.2, Margin: 1}},
		{"a scale past the symbol", testLogo(), LogoOptions{Scale: 1.5, Margin: 1}},
		{"a negative margin", testLogo(), LogoOptions{Scale: 0.2, Margin: -1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := forcedVersion(t, 10, Highest)

			if err := q.SetLogo(test.logo, test.options); err == nil {
				t.Error("SetLogo accepted it")
			}
		})
	}
}

func TestSetLogoLeavesAQRCodeWithoutALogoUnchanged(t *testing.T) {
	q := forcedVersion(t, 10, Highest)

	if q.logo != nil {
		t.Error("a new QR Code already carries a logo")
	}
}

func TestTheMarginIsChargedAgainstTheBudgetWithTheLogo(t *testing.T) {
	for _, versionNumber := range someVersions {
		for _, level := range everyLevel {
			t.Run(versionLevelName(versionNumber, level), func(t *testing.T) {
				q := forcedVersion(t, versionNumber, level)

				options := LogoOptions{Scale: newLogoFit(q.version).maxScale(1), Margin: 1}
				if options.Scale == 0 {
					t.Skip("no logo fits this symbol at all")
				}

				if err := q.SetLogo(testLogo(), options); err != nil {
					t.Fatalf("SetLogo at the largest accepted scale: %s", err)
				}

				// The knockout is the logo plus its margin, so widening the
				// margin by a module on each side must cost the same logo the
				// acceptance it just had. Were only the logo's own extent
				// counted, nothing would change.
				options.Margin = 2

				if err := q.SetLogo(testLogo(), options); err == nil {
					t.Errorf("widening the margin to %d modules cost a scale %v logo nothing",
						options.Margin, options.Scale)
				}
			})
		}
	}
}

func TestARefusedLogoLeavesAnAttachedOneAlone(t *testing.T) {
	q := forcedVersion(t, 10, Highest)

	accepted := testLogo()
	if err := q.SetLogo(accepted, DefaultLogoOptions()); err != nil {
		t.Fatalf("SetLogo: %s", err)
	}

	options := DefaultLogoOptions()
	options.Scale = 0.9

	if err := q.SetLogo(testLogo(), options); err == nil {
		t.Fatal("SetLogo accepted a scale 0.9 logo")
	}

	if q.logo != accepted {
		t.Error("a refused logo displaced the one already attached")
	}

	if want := DefaultLogoOptions(); q.logoOptions != want {
		t.Errorf("kept options %+v, want the accepted %+v", q.logoOptions, want)
	}
}
