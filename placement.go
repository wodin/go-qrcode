// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

// modulePosition is the location of one module within a symbol, in module
// coordinates measured from the symbol's top left corner. The quiet zone is
// not counted, matching symbol's get/set.
type modulePosition struct {
	x int
	y int
}

// verticalTimingPatternColumn is the column the vertical timing pattern runs
// down, between the left hand finder patterns.
const verticalTimingPatternColumn = 6

// direction of travel of the placement path along a two module wide column.
type direction uint8

const (
	up direction = iota
	down
)

// functionPatternSymbol returns a symbol of version containing every function
// pattern and no data. It carries no quiet zone: it exists to say which
// modules the data region is made of, not to be rendered.
//
// The format info of mask 0 is used. Which modules hold the format info is
// fixed by the version alone, and only their positions matter here.
func functionPatternSymbol(version qrCodeVersion) *symbol {
	m := newRegularSymbol(version, 0, nil, 0)
	m.addFunctionPatterns()

	return m.symbol
}

// protectedFunctionPatternSymbol returns a symbol of version containing every
// function pattern a logo may not cover — the finder patterns and their
// separators, the timing patterns, the format info and the version info — and
// nothing else. Like functionPatternSymbol it carries no quiet zone and no
// data: only which modules are used matters.
//
// The alignment patterns are deliberately absent. A centred logo of any
// useful size collides with one on most versions above 6, so refusing that
// collision would delete the logo feature; the other function patterns are
// out of a centred logo's reach and refusing them costs nothing (ADR-0002).
func protectedFunctionPatternSymbol(version qrCodeVersion) *symbol {
	m := newRegularSymbol(version, 0, nil, 0)

	m.addFinderPatterns()
	m.addTimingPatterns()
	m.addFormatInfo()
	m.addVersionInfo()

	return m.symbol
}

// dataModulePath returns the modules of version's data region, in the order
// the encoder fills them. See symbol.dataModulePath for what the order is.
func dataModulePath(version qrCodeVersion) []modulePosition {
	return functionPatternSymbol(version).dataModulePath()
}

// dataModulePath returns the modules of s's data region, in the order the
// encoder fills them: bit i of the encoded bit stream is placed in the module
// at path[i], so codeword n occupies path[8n:8n+8]. Any remainder bits trail
// the last codeword.
//
// s must hold the function patterns and no data — it is s's empty modules
// that define the data region. Callers without such a symbol already in hand
// should use the dataModulePath function, which builds one.
//
// The path walks upwards from the bottom right corner in two module wide
// columns, right module of each pair first, reversing direction at the top
// and bottom of the symbol, and stepping over every function pattern
// (ISO/IEC 18004:2006 8.7.3). The vertical timing pattern is skipped a column
// at a time rather than module by module, so that the two module wide columns
// either side of it stay paired as the spec requires.
func (s *symbol) dataModulePath() []modulePosition {
	// The data region is what the function patterns leave empty, so its size
	// is the exact capacity the path needs.
	path := make([]modulePosition, 0, s.numEmptyModules())

	// x is the left module of the current two module wide column, xOffset
	// selects the module within it.
	xOffset := 1
	dir := up

	x := s.symbolSize - 2
	y := s.symbolSize - 1

	for {
		path = append(path, modulePosition{x: x + xOffset, y: y})

		// Find the next free module in the symbol.
		for {
			if xOffset == 1 {
				xOffset = 0
			} else {
				xOffset = 1

				if dir == up {
					if y > 0 {
						y--
					} else {
						dir = down
						x -= 2
					}
				} else {
					if y < s.symbolSize-1 {
						y++
					} else {
						dir = up
						x -= 2
					}
				}
			}

			// Shift the whole column pair left rather than let its right
			// module fall in the vertical timing pattern.
			if x == verticalTimingPatternColumn-1 {
				x--
			}

			// Walking off the left hand edge is how the path ends: every
			// column of the symbol has been offered.
			if x+xOffset < 0 {
				return path
			}

			if s.empty(x+xOffset, y) {
				break
			}
		}
	}
}
