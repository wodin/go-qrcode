// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"errors"
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"
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

func TestTheDefaultScaleIsNeverAcceptedAtLow(t *testing.T) {
	// What DefaultLogoOptions' godoc and the README tell a caller, measured
	// rather than repeated: no version at Low carries a fifth of its width,
	// and the first version that does at each other level is the one named.
	firstAccepting := map[RecoveryLevel]int{Medium: 11, High: 6, Highest: 6}

	for _, level := range everyLevel {
		first := 0

		for versionNumber := 1; versionNumber <= maxVersionNumber; versionNumber++ {
			q := forcedVersion(t, versionNumber, level)

			if q.MaxLogoScale(defaultLogoMargin) >= defaultLogoScale && first == 0 {
				first = versionNumber
			}
		}

		if got, want := first, firstAccepting[level]; got != want {
			if want == 0 {
				t.Errorf("level %d first accepts the default scale at version %d, want no version to accept it",
					level, got)
			} else {
				t.Errorf("level %d first accepts the default scale at version %d, want version %d",
					level, got, want)
			}
		}
	}
}

func TestTheDefaultScaleIsRefusedAgainAboveTheFirstVersionAcceptingIt(t *testing.T) {
	// The first accepting version is not a floor, which is why the godoc says
	// so: version 11 at Medium accepts the default scale and version 12 does
	// not. A caller who read the first version as "this and everything above"
	// would be refused.
	if got := forcedVersion(t, 11, Medium).MaxLogoScale(defaultLogoMargin); got < defaultLogoScale {
		t.Errorf("version 11 at Medium accepts %v, want at least the default scale %v",
			got, defaultLogoScale)
	}

	if got := forcedVersion(t, 12, Medium).MaxLogoScale(defaultLogoMargin); got >= defaultLogoScale {
		t.Errorf("version 12 at Medium accepts %v, want less than the default scale %v",
			got, defaultLogoScale)
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

func TestMaxLogoScaleReportsNothingFitsTheSmallestLowSymbols(t *testing.T) {
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

func TestMaxLogoScaleReportsNothingFitsAMarginWiderThanAnySymbol(t *testing.T) {
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

// namedVersion matches the version a refusal offers as a remedy, so that a
// test can check the package really measured it rather than guessed.
var namedVersion = regexp.MustCompile(
	`version (\d+) at the same recovery level accepts a scale of up to ([0-9.]+)`)

// refusalMessage returns the message SetLogo refuses a full width logo with,
// which every symbol does at every margin.
func refusalMessage(t *testing.T, q *QRCode, margin int) string {
	t.Helper()

	err := q.SetLogo(testLogo(), LogoOptions{Scale: 1, Margin: margin})
	if err == nil {
		t.Fatalf("margin %d: a full width logo was accepted", margin)
	}

	return err.Error()
}

func TestARefusalNamesOnlyAVersionItHasMeasured(t *testing.T) {
	// Version 1 at Low fits no logo at all, so the refusal has to look
	// elsewhere, and what it finds must hold up when the caller acts on it.
	q := forcedVersion(t, 1, Low)

	message := refusalMessage(t, q, DefaultLogoOptions().Margin)

	named := namedVersion.FindStringSubmatch(message)
	if named == nil {
		t.Fatalf("no version named in %q, want one: a larger version does fit a logo here", message)
	}

	versionNumber, err := strconv.Atoi(named[1])
	if err != nil {
		t.Fatalf("version %q in %q: %s", named[1], message, err)
	}

	scale, err := strconv.ParseFloat(named[2], 64)
	if err != nil {
		t.Fatalf("scale %q in %q: %s", named[2], message, err)
	}

	if versionNumber <= q.VersionNumber {
		t.Errorf("refusal offers version %d, which is not larger than the %d refusing",
			versionNumber, q.VersionNumber)
	}

	larger := forcedVersion(t, versionNumber, q.Level)

	if got := larger.MaxLogoScale(DefaultLogoOptions().Margin); !closeEnough(got, scale) {
		t.Errorf("refusal offers version %d a scale of %v, which accepts %v",
			versionNumber, scale, got)
	}
}

func TestARefusalNamesNoVersionWhenTheScanFoundNone(t *testing.T) {
	// A margin this wide is paid for out of the correction budget before any
	// logo is, and defeats every version at every level, so there is nothing
	// truthful to offer.
	q := forcedVersion(t, 1, Low)

	message := refusalMessage(t, q, 32)

	if named := namedVersion.FindStringSubmatch(message); named != nil {
		t.Errorf("refusal offers version %s at a 32 module margin, where no version fits a logo: %q",
			named[1], message)
	}
}

func TestNoRefusalAdvisesRaisingTheRecoveryLevel(t *testing.T) {
	// The advice everyone expects, and the one the package must never give:
	// 11 of the 120 level steps accept a smaller logo at the higher level
	// (ADR-0004).
	unmeasured := []string{
		"higher recovery level",
		"different recovery level",
		"another recovery level",
		"raise the recovery level",
	}

	for _, versionNumber := range someVersions {
		for _, level := range everyLevel {
			t.Run(versionLevelName(versionNumber, level), func(t *testing.T) {
				q := forcedVersion(t, versionNumber, level)

				for _, margin := range []int{0, 1, 2, 16, 32} {
					message := refusalMessage(t, q, margin)

					for _, phrase := range unmeasured {
						if strings.Contains(message, phrase) {
							t.Errorf("margin %d: refusal advises a %q: %q",
								margin, phrase, message)
						}
					}
				}
			})
		}
	}
}

func TestARefusalReportsTheScaleThatWouldFit(t *testing.T) {
	q := forcedVersion(t, 10, Highest)

	message := refusalMessage(t, q, DefaultLogoOptions().Margin)

	want := fmt.Sprintf("largest accepted scale is %.4f",
		q.MaxLogoScale(DefaultLogoOptions().Margin))

	if !strings.Contains(message, want) {
		t.Errorf("refusal says %q, want it to report %q", message, want)
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

func TestFitLogoSeatsTheLargestLogoTheSymbolCarries(t *testing.T) {
	forSomeSymbolsAndMargins(t, func(t *testing.T, q *QRCode, margin int) {
		scale := q.MaxLogoScale(margin)

		err := q.FitLogo(testLogo(), margin)

		if scale == 0 {
			// Nothing fits, so there is nothing to seat: the caller gets one
			// of the two ordinary refusals, describing what the smallest logo
			// there is would have cost. Which of the two depends on the
			// symbol — a wide enough margin buries a finder pattern on a
			// small version before it overruns any budget.
			var tooLarge *LogoTooLargeError
			var occludes *LogoOccludesFunctionPatternError

			if !errors.As(err, &tooLarge) && !errors.As(err, &occludes) {
				t.Fatalf("margin %d: FitLogo returned %v where no logo fits, want an ordinary refusal",
					margin, err)
			}

			if q.logo != nil {
				t.Errorf("margin %d: a logo was seated where none fits", margin)
			}

			return
		}

		if err != nil {
			t.Fatalf("margin %d: FitLogo refused the scale %v it reports fitting: %s",
				margin, scale, err)
		}

		if q.logo == nil {
			t.Fatalf("margin %d: FitLogo kept no logo", margin)
		}

		if want := (LogoOptions{Scale: scale, Margin: margin}); q.logoOptions != want {
			t.Errorf("margin %d: logo seated with %+v, want %+v", margin, q.logoOptions, want)
		}
	})
}

// forSomeSymbolsAndMargins runs test over someVersions, every recovery level
// and a spread of margins wide enough to reach the symbols that fit nothing.
func forSomeSymbolsAndMargins(t *testing.T, test func(t *testing.T, q *QRCode, margin int)) {
	t.Helper()

	for _, versionNumber := range someVersions {
		for _, level := range everyLevel {
			for _, margin := range []int{0, 1, 2, 8} {
				t.Run(fmt.Sprintf("%s-margin%d", versionLevelName(versionNumber, level), margin),
					func(t *testing.T) {
						test(t, forcedVersion(t, versionNumber, level), margin)
					})
			}
		}
	}
}

func TestFitLogoSucceedsWhereverAScaleFitsAtAll(t *testing.T) {
	// The promise the fitted seat makes, over every symbol there is rather
	// than a spread of them: if the query says something fits, the seat takes
	// it. Fit inversion means the answer cannot be extrapolated from a
	// neighbouring version or level, so each is asked.
	for versionNumber := 1; versionNumber <= maxVersionNumber; versionNumber++ {
		for _, level := range everyLevel {
			q := forcedVersion(t, versionNumber, level)

			margin := DefaultLogoOptions().Margin
			if q.MaxLogoScale(margin) == 0 {
				continue
			}

			if err := q.FitLogo(testLogo(), margin); err != nil {
				t.Errorf("%s: FitLogo: %s", versionLevelName(versionNumber, level), err)
			}
		}
	}
}

func TestFitLogoRefusesTheSmallestLowSymbols(t *testing.T) {
	// The only symbols that carry no logo at all at the default margin. The
	// refusal is the ordinary one, describing what the smallest logo there is
	// would have cost, not a new kind of error a caller has to learn.
	for _, versionNumber := range []int{1, 2} {
		q := forcedVersion(t, versionNumber, Low)

		err := q.FitLogo(testLogo(), DefaultLogoOptions().Margin)

		var tooLarge *LogoTooLargeError
		if !errors.As(err, &tooLarge) {
			t.Errorf("version %d at Low: FitLogo returned %v, want a *LogoTooLargeError",
				versionNumber, err)
		}
	}
}

func TestFitLogoRejectsUnusableArguments(t *testing.T) {
	tests := []struct {
		name   string
		logo   image.Image
		margin int
	}{
		{"no image", nil, 1},
		{"an empty image", image.NewRGBA(image.Rect(0, 0, 0, 0)), 1},
		{"a negative margin", testLogo(), -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := forcedVersion(t, 10, Highest)

			err := q.FitLogo(test.logo, test.margin)
			if err == nil {
				t.Fatal("FitLogo accepted it")
			}

			// A mistake in the call is not a verdict on the logo, exactly as
			// it is not for SetLogo.
			var tooLarge *LogoTooLargeError
			var occludes *LogoOccludesFunctionPatternError

			if errors.As(err, &tooLarge) || errors.As(err, &occludes) {
				t.Errorf("FitLogo returned %v, want a plain error", err)
			}
		})
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
