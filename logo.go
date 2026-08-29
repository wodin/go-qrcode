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

// ClearingStyle says which modules of the knockout a render blanks to the
// background colour before drawing the logo over them.
//
// It never changes what the logo costs. The knockout is charged against the
// error correction budget whichever style is chosen, so clearing less than
// all of it can only leave a symbol less damaged than the fit paid for
// (ADR-0008), and MaxLogoScale answers the same either way.
type ClearingStyle int

const (
	// ClearKnockout blanks the whole knockout square. It is the zero value,
	// and so what a caller who says nothing gets: leaving a transparent
	// logo's holes filled with background is a design decision, and changing
	// it under someone already placing one would redesign their mark.
	ClearKnockout ClearingStyle = iota

	// ClearInk blanks only the modules the logo's ink covers, dilated by
	// Margin, so that the modules under a logo's transparent regions survive
	// and a decoder reads them. Any alpha above zero counts as ink.
	//
	// It buys nothing for an opaque logo, which inks every module of its
	// knockout, and little for a compact mark: a codeword threads eight
	// modules through the region and is damaged in full by any one of them,
	// so a hole only pays where it leaves whole codewords untouched. A thin
	// mark with space between its strokes is what this is for.
	ClearInk
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

	// Clearing says which modules of the knockout are blanked to the
	// background colour before the logo is drawn over them. The zero value,
	// ClearKnockout, blanks all of them.
	Clearing ClearingStyle
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
//
// A fifth is not a size every symbol carries. No symbol carries it at the Low
// recovery level, at any version 1 to 40. At Medium it is first accepted at
// version 11, at High and Highest at version 6 — and refused again at larger
// versions above those, because a version resplits the symbol into blocks of
// a different size rather than simply adding room. Do not read the first
// accepting version as a floor: ask MaxLogoScale.
//
// These are the options to start from when the size matters more than the
// attempt succeeding. Where the QR Code has to work and the logo is yours to
// scale, use FitLogo, which asks the symbol instead of assuming.
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
	// recovery level and margin, and is itself accepted if used. It is what
	// MaxLogoScale reports. It is 0 when no logo of any size fits, and the
	// message then says whether a larger version fits one.
	MaxScale float64

	// Block is the error correction block the logo left with the least
	// capacity to spare — the one that decided the refusal. DamagedCodewords
	// is how many of its codewords the logo would have covered, and Capacity
	// how many it can correct in total.
	Block            int
	DamagedCodewords int
	Capacity         int

	logoRemedy
}

func (e *LogoTooLargeError) Error() string {
	return fmt.Sprintf("logo of scale %.4f with a %d module margin damages %d "+
		"of the %d correctable codewords of block %d, more than the half it "+
		"may spend: %s",
		e.Scale, e.Margin, e.DamagedCodewords, e.Capacity, e.Block,
		logoScaleAdvice(e.MaxScale, e.logoRemedy))
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
	// recovery level and margin, and is itself accepted if used. It is what
	// MaxLogoScale reports. It is 0 when no logo of any size fits, and the
	// message then says whether a larger version fits one.
	MaxScale float64

	// X and Y locate a covered function pattern module, in modules from the
	// top left corner of the symbol. The quiet zone is not counted.
	X int
	Y int

	logoRemedy
}

func (e *LogoOccludesFunctionPatternError) Error() string {
	return fmt.Sprintf("logo of scale %.4f with a %d module margin covers the "+
		"function pattern module at (%d,%d), which carries no error "+
		"correction: %s",
		e.Scale, e.Margin, e.X, e.Y, logoScaleAdvice(e.MaxScale, e.logoRemedy))
}

// logoRemedy is a symbol the package has checked and found does carry a logo:
// a larger version at the same recovery level and margin, and the largest
// scale that version accepts.
//
// Its zero value means no such symbol was found, or that the scan never ran
// because a logo fits here and the question does not arise. Both mean the
// same thing to a refusal: it has nothing measured to offer, so it offers
// nothing.
type logoRemedy struct {
	version int
	scale   float64
}

// largerVersionFittingALogo scans versions above v, at v's own recovery level
// and the given margin, for the first that fits a logo of any size.
//
// The recovery level is deliberately held fixed. Raising it is the advice
// everyone expects and it is wrong often enough to be worthless: 11 of the
// 120 level steps accept a smaller logo than the level below, and where
// nothing fits at all it fails to help in most cases (ADR-0004). A larger
// version is the one lever usually worth pulling, and this measures it rather
// than assuming it.
//
// The scan builds a codeword layout per candidate, which is real work on an
// error path. It is bounded by one level's 40 versions rather than all 160
// combinations, it usually stops within a version or two, and it runs only
// when nothing fits here at all. That is the price of saying only what has
// been checked.
func largerVersionFittingALogo(v qrCodeVersion, margin int) logoRemedy {
	for versionNumber := v.version + 1; versionNumber <= maxVersionNumber; versionNumber++ {
		if scale := scaleCarriedBy(versionNumber, v.level, margin); scale > 0 {
			return logoRemedy{version: versionNumber, scale: scale}
		}
	}

	return logoRemedy{}
}

// scaleCarriedBy returns the largest logo scale a symbol of this version and
// recovery level carries at margin modules of clear space, or 0 when it
// carries none — as a version there is no symbol for carries none.
//
// It is what a scan over versions measures at each candidate, and the only
// place a version number becomes a measurement, so that the two scans cannot
// answer the same question differently.
func scaleCarriedBy(versionNumber int, level RecoveryLevel, margin int) float64 {
	v := getQRCodeVersion(level, versionNumber)
	if v == nil {
		return 0
	}

	return newLogoFit(*v).maxScale(margin)
}

// logoScaleAdvice tells a caller what the fit check found: the scale to ask
// for instead, a larger version that would carry a logo when this symbol
// carries none, or that there is nothing to offer.
//
// Everything named here has been measured on the symbol it is offered for
// (ADR-0004). No message suggests raising the recovery level, however
// plausible that sounds: correction capacity is held per block, a higher
// level splits the symbol into more, smaller blocks, and a block's budget can
// fall as the proportion of the symbol given to error correction rises.
func logoScaleAdvice(maxScale float64, remedy logoRemedy) string {
	switch {
	case maxScale > 0:
		return fmt.Sprintf("largest accepted scale is %.4f", maxScale)

	case remedy.version == 0:
		return "no logo fits this symbol at this margin, and no larger " +
			"version at the same recovery level fits one either"

	default:
		return fmt.Sprintf("no logo fits this symbol at this margin: version "+
			"%d at the same recovery level accepts a scale of up to %.4f",
			remedy.version, remedy.scale)
	}
}

// MaxLogoScale returns the largest LogoOptions.Scale SetLogo accepts for this
// QR Code with margin modules of clear space around the logo, or 0 when no
// logo of any size fits.
//
// It answers the question a refusal otherwise has to be provoked to answer,
// and it is the same answer: the MaxScale carried by a refusal is this value.
// The scale it reports is accepted if it is used.
//
// The answer depends on the symbol's version and recovery level alone, never
// on the content encoded, because what a logo damages is which codewords it
// covers and how those are grouped into error correction blocks. A margin
// below 0 is not a margin and reports 0, as a margin wider than the symbol
// does.
//
// A higher recovery level does not always report a larger scale. Correction
// capacity is held per block, and a higher level splits the symbol into more,
// smaller blocks, so a block's budget can fall as the proportion of the
// symbol given to error correction rises: version 15 accepts 0.2727 at High
// and only 0.1688 at Highest. Ask, rather than assume, which combination
// carries the logo you want.
func (q *QRCode) MaxLogoScale(margin int) float64 {
	return newLogoFit(q.version).maxScale(margin)
}

// SmallestVersionCarryingLogo returns the smallest version, at or above
// from, whose symbol at this recovery level accepts a logo of scale with
// margin modules of clear space around it, and the scale to ask for instead
// when none of them accepts that one.
//
// It is MaxLogoScale asked before there is a symbol to ask about. MaxLogoScale
// answers what a QR Code already built carries, which leaves the caller who
// wants a particular size nothing to do but shorten their content until the
// symbol grows: this answers which symbol to build instead. from is the
// smallest version it may name, because the content still has to fit — a
// version below the one the content needs is not on offer however well it
// carries a logo.
//
// The version is 0 when no version from there up carries the scale, and
// largestCarried is then the largest scale they do carry: a scale rather than
// a version, because the scale is what the caller chooses and this is what
// turns it into a version. Otherwise largestCarried is 0, as the zero value
// of a refusal's remedy means: there is nothing measured to offer instead.
//
// Both are 0 where there is no measurement to report at all — a scale outside
// (0, 1] is not a fraction of a symbol's width and so is a mistake in the
// call rather than a symbol too small to carry it, exactly as SetLogo treats
// it, and a margin can be wide enough that no version carries a logo of any
// size. Ask SetLogo, which says which of the two it is.
//
// Every candidate is measured and none is inferred from its neighbour. Around
// half of all version steps carry a smaller logo than the version below them
// — a fit inversion (see CONTEXT.md) — so there is no first version above
// which every version fits, and no arithmetic on the symbol's width that
// would find one (ADR-0004).
func SmallestVersionCarryingLogo(from int, level RecoveryLevel, scale float64,
	margin int) (version int, largestCarried float64) {

	if from < 1 || from > maxVersionNumber || scale <= 0 || scale > 1 {
		return 0, 0
	}

	for candidate := from; candidate <= maxVersionNumber; candidate++ {
		carried := scaleCarriedBy(candidate, level, margin)

		if carried >= scale {
			return candidate, 0
		}

		if carried > largestCarried {
			largestCarried = carried
		}
	}

	return 0, largestCarried
}

// FitLogo attaches logo to the centre of the QR Code at the largest scale the
// symbol safely carries, with margin modules of clear space around it.
//
// It is SetLogo asked the other way round. SetLogo takes the size a caller
// wants and answers whether the symbol survives it; FitLogo takes the symbol
// and picks the size, which is what a caller wants when the logo is theirs to
// scale and the QR Code has to work. The scale chosen is the one MaxLogoScale
// reports, and it is judged by SetLogo like any other.
//
// A symbol that carries no logo at all — only versions 1 and 2 at the Low
// recovery level, at the default margin — is refused with the ordinary
// refusal SetLogo would have given the smallest logo there is, describing
// what that logo would have cost. There is no separate error to handle:
// nothing fits is the same answer whether the caller named a size or left it
// to the package.
func (q *QRCode) FitLogo(logo image.Image, margin int) error {
	fit := newLogoFit(q.version)

	scale := fit.maxScale(margin)
	if scale == 0 {
		scale = fit.smallestScale()
	}

	return q.SetLogo(logo, LogoOptions{Scale: scale, Margin: margin})
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
		return q.logoRefusal(fit, options, damage)
	}

	q.logo = logo
	q.logoOptions = options

	return nil
}

// logoRefusal returns the error explaining why damage is not survivable,
// including the largest scale that would have been survivable and, when no
// scale would have been, a larger version that carries a logo.
func (q *QRCode) logoRefusal(fit *logoFit, options LogoOptions,
	damage knockoutDamage) error {

	maxScale := fit.maxScale(options.Margin)

	// Only a symbol that carries no logo at all needs somewhere else to look,
	// and only then is the scan worth what it costs.
	remedy := logoRemedy{}
	if maxScale == 0 {
		remedy = largerVersionFittingALogo(q.version, options.Margin)
	}

	// Occluding a function pattern is reported ahead of the budget, which
	// such a logo also overruns: it is the more fundamental refusal. No
	// budget can pay for it, at any version or recovery level, whereas an
	// overspent budget is at least a question of which symbol you ask.
	if damage.occludesFunctionPattern {
		return &LogoOccludesFunctionPatternError{
			LogoOptions: options,
			MaxScale:    maxScale,
			X:           damage.functionPattern.x,
			Y:           damage.functionPattern.y,
			logoRemedy:  remedy,
		}
	}

	worst := damage.worstBlock()

	return &LogoTooLargeError{
		LogoOptions:      options,
		MaxScale:         maxScale,
		Block:            worst,
		DamagedCodewords: damage.blocks[worst].damaged,
		Capacity:         damage.blocks[worst].shape.correctionCapacity(),
		logoRemedy:       remedy,
	}
}
