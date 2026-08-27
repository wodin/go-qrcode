// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"errors"
	"fmt"
	"image"
)

// Defaults for the fields of LogoOptions. See DefaultLogoOptions.
const (
	defaultLogoScale  = 0.2
	defaultLogoMargin = 1
)

// LogoOptions says how large a logo is and how much clear space surrounds it.
type LogoOptions struct {
	// Scale is the logo's width as a fraction of the symbol's width. The
	// symbol excludes the quiet zone, so a logo's size does not change when
	// the border is disabled.
	Scale float64

	// Margin is the clear space kept between the logo and the modules around
	// it, in modules. It exists to stop a scanner's binarizer smearing the
	// boundary and corrupting modules the logo does not cover, and it is
	// charged against the error correction budget along with the logo itself.
	Margin int
}

// DefaultLogoOptions returns the options a logo is attached with unless the
// caller wants otherwise: a fifth of the symbol's width, with a one module
// margin.
//
// Amend a copy of these rather than filling in a LogoOptions from scratch, so
// that a field you have no opinion about keeps its default:
//
//	options := qrcode.DefaultLogoOptions()
//	options.Scale = 0.15
//
//	err := q.SetLogo(logo, options)
func DefaultLogoOptions() LogoOptions {
	return LogoOptions{Scale: defaultLogoScale, Margin: defaultLogoMargin}
}

// LogoTooLargeError is returned by SetLogo when a logo would leave some error
// correction block with less than half its correction capacity to spare.
//
// The remaining half is deliberately withheld: it pays for print bleed,
// camera blur, glare and creased paper, none of which a unit test can see.
type LogoTooLargeError struct {
	// LogoOptions are the options the logo was offered with, so that Scale
	// and Margin read straight off the error.
	LogoOptions

	// MaxScale is the largest scale that would have been accepted at this
	// recovery level and margin, and is itself accepted if used. It is 0 when
	// no logo of any size fits, which a higher recovery level or a larger
	// symbol may fix.
	MaxScale float64

	// Block is the error correction block the logo left with the least
	// capacity to spare — the one that decided the refusal. DamagedCodewords
	// is how many of its codewords the logo would have covered, and Capacity
	// how many it can correct in total.
	Block            int
	DamagedCodewords int
	Capacity         int
}

func (e *LogoTooLargeError) Error() string {
	return fmt.Sprintf("logo of scale %.4f with a %d module margin damages %d "+
		"of the %d correctable codewords of block %d, more than the half it "+
		"may spend: %s",
		e.Scale, e.Margin, e.DamagedCodewords, e.Capacity, e.Block,
		logoScaleAdvice(e.MaxScale))
}

// LogoOccludesFunctionPatternError is returned by SetLogo when a logo would
// cover a function pattern that carries no error correction, so that no
// recovery level could save the symbol.
//
// Alignment patterns are exempt: a centred logo collides with one on most
// versions above 6, and decoders fall back to the finder patterns without
// them (ADR-0002). The finder, timing, format info and version info patterns
// are out of a sensible logo's reach, so this refusal means the scale is
// wildly too large rather than slightly.
type LogoOccludesFunctionPatternError struct {
	// LogoOptions are the options the logo was offered with, so that Scale
	// and Margin read straight off the error.
	LogoOptions

	// MaxScale is the largest scale that would have been accepted at this
	// recovery level and margin, and is itself accepted if used. It is 0 when
	// no logo of any size fits.
	MaxScale float64

	// X and Y locate a covered function pattern module, in modules from the
	// top left corner of the symbol. The quiet zone is not counted.
	X int
	Y int
}

func (e *LogoOccludesFunctionPatternError) Error() string {
	return fmt.Sprintf("logo of scale %.4f with a %d module margin covers the "+
		"function pattern module at (%d,%d), which carries no error "+
		"correction: %s",
		e.Scale, e.Margin, e.X, e.Y, logoScaleAdvice(e.MaxScale))
}

// logoScaleAdvice tells a caller what to do about a refusal: the scale to ask
// for instead, or where else to look when no scale would have worked.
//
// A larger symbol is a real remedy and a lower recovery level is not: a
// higher version carries more codewords per block, whereas a lower level
// carries fewer error correction codewords to spend.
func logoScaleAdvice(maxScale float64) string {
	if maxScale == 0 {
		return "no logo fits this symbol: raise the recovery level or force a larger version"
	}

	return fmt.Sprintf("largest accepted scale is %.4f", maxScale)
}

// SetLogo attaches logo to the centre of the QR Code, refusing it if seating
// it would put decoding at risk.
//
// The region cleared to seat the logo — the logo plus its margin, snapped out
// to whole modules — damages every codeword it touches. A logo is accepted
// only if every error correction block keeps at least half its correction
// capacity afterwards, so that the other half remains to absorb the damage
// the physical world does. Covering an alignment pattern is permitted;
// covering a finder, timing, format info or version info module is not.
//
// A refusal is a *LogoTooLargeError or a *LogoOccludesFunctionPatternError,
// both of which carry the largest scale that would have been accepted. That
// scale is accepted if it is used. An unusable argument — no image, an empty
// one, a scale outside (0, 1] or a negative margin — is a plain error rather
// than one of these: it is a mistake in the call, not a verdict on the logo.
// A refused logo changes nothing, so any logo already attached stays attached.
//
// The judgement is made here, against the version and recovery level the QR
// Code was built with, and is never repeated. Nothing settable afterwards can
// invalidate it — the colours, the border and the exported Level and
// VersionNumber fields describe the symbol rather than change it — so no
// later assignment re-runs the check or reports a new error.
func (q *QRCode) SetLogo(logo image.Image, options LogoOptions) error {
	if logo == nil {
		return errors.New("no logo image to attach")
	}

	if logo.Bounds().Empty() {
		return errors.New("logo image is empty")
	}

	if options.Scale <= 0 || options.Scale > 1 {
		return fmt.Errorf("logo scale is %v (expected a fraction of the "+
			"symbol's width, greater than 0 and at most 1)", options.Scale)
	}

	if options.Margin < 0 {
		return fmt.Errorf("logo margin is %d modules (expected 0 or more)",
			options.Margin)
	}

	fit := newLogoFit(q.version)
	damage := fit.damageFrom(
		newKnockout(fit.symbolSize, options.Scale, options.Margin))

	if !damage.survivable() {
		return logoRefusal(fit, options, damage)
	}

	q.logo = logo
	q.logoOptions = options

	return nil
}

// logoRefusal returns the error explaining why damage is not survivable,
// including the largest scale that would have been.
func logoRefusal(fit *logoFit, options LogoOptions,
	damage knockoutDamage) error {

	maxScale := fit.maxScale(options.Margin)

	// Occluding a function pattern is reported ahead of the budget, which
	// such a logo also overruns: it is the more fundamental refusal, and
	// unlike the budget no recovery level relaxes it.
	if damage.occludesFunctionPattern {
		return &LogoOccludesFunctionPatternError{
			LogoOptions: options,
			MaxScale:    maxScale,
			X:           damage.functionPattern.x,
			Y:           damage.functionPattern.y,
		}
	}

	worst := damage.worstBlock()

	return &LogoTooLargeError{
		LogoOptions:      options,
		MaxScale:         maxScale,
		Block:            worst,
		DamagedCodewords: damage.blocks[worst].damaged,
		Capacity:         damage.blocks[worst].shape.correctionCapacity(),
	}
}
