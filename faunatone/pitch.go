package main

import (
	"fmt"
	"math"
)

// represents an interval in terms of source components
type pitchSrc struct {
	Float float64 `json:",omitempty"` // raw interval or edx divided interval
	Ints  [2]int  `json:",omitempty"` // rational or edxstep, see IsEdx
	IsEdx bool    `json:",omitempty"`
}

// return a new semitone interval
func newSemiPitch(f float64) *pitchSrc {
	return &pitchSrc{Float: f}
}

// return a new rational interval
func newRatPitch(num, den int) *pitchSrc {
	return &pitchSrc{Ints: [2]int{num, den}}
}

// return a new edxstep interval
func newEdxPitch(f float64, steps, edo int) *pitchSrc {
	return &pitchSrc{Float: f, Ints: [2]int{steps, edo}, IsEdx: true}
}

// return a semitone representation of the interval
func (ps *pitchSrc) semitones() float64 {
	if ps.Ints[1] == 0 {
		return ps.Float
	} else if ps.IsEdx {
		return ps.Float * float64(ps.Ints[0]) / float64(ps.Ints[1])
	}
	return 12 * math.Log(float64(ps.Ints[0])/float64(ps.Ints[1])) / math.Log(2)
}

// return the semitone-based pitch class of the interval
func (ps *pitchSrc) class(octave float64) float64 {
	return posMod(ps.semitones(), octave)
}

// return the addition of this and another interval
func (ps *pitchSrc) add(other *pitchSrc) *pitchSrc {
	// special case: octaves and zero values are compatible with any type
	otherSemi := other.semitones()
	if otherSemi == 0 || math.Abs(otherSemi) == 12 {
		sign := 0
		if otherSemi < 0 {
			sign = -1
		} else if otherSemi > 0 {
			sign = 1
		}
		if ps.IsEdx {
			return newEdxPitch(ps.Float, ps.Ints[0]+ps.Ints[1]*sign, ps.Ints[1])
		} else if ps.Ints[1] != 0 {
			var num, den int
			if sign > 0 {
				num, den = reduce(ps.Ints[0]*2, ps.Ints[1])
			} else if sign < 0 {
				num, den = reduce(ps.Ints[0], ps.Ints[1]*2)
			} else {
				num, den = reduce(ps.Ints[0], ps.Ints[1])
			}
			return newRatPitch(num, den)
		}
	}
	// normal case
	if ps.IsEdx && other.IsEdx && ps.Float == other.Float {
		if ps.Ints[1] == other.Ints[1] {
			return newEdxPitch(ps.Float, ps.Ints[0]+other.Ints[0], ps.Ints[1])
		}
		// TODO allow adding steps of different edos, e.g. 1\3 + 1\4 = 7\12
	} else if !ps.IsEdx && !other.IsEdx && ps.Ints[1] != 0 && other.Ints[1] != 0 {
		num, den := reduce(ps.Ints[0]*other.Ints[0], ps.Ints[1]*other.Ints[1])
		return newRatPitch(num, den)
	}
	return newSemiPitch(ps.semitones() + other.semitones())
}

// return a negative version of the interval (not an octave inversion)
func (ps *pitchSrc) invert() *pitchSrc {
	return ps.multiply(-1)
}

// return an interval n times the size of this one
func (ps *pitchSrc) multiply(n int) *pitchSrc {
	if ps.IsEdx {
		return newEdxPitch(ps.Float, ps.Ints[0]*n, ps.Ints[1])
	} else if ps.Ints[1] != 0 {
		num, den := ps.Ints[0], ps.Ints[1]
		if n < 0 {
			num, den, n = den, num, -n
		}
		finalNum, finalDen := 1, 1
		for i := 0; i < n; i++ {
			finalNum, finalDen = finalNum*num, finalDen*den
		}
		return newRatPitch(finalNum, finalDen)
	}
	return newSemiPitch(ps.Float * float64(n))
}

// return an interval equal to this one modulo other
func (ps *pitchSrc) modulo(other *pitchSrc) *pitchSrc {
	result := ps.add(newSemiPitch(0)) // clone
	if other.semitones() == 0 {
		return result // avoid infinite loops
	}
	for result.semitones() > other.semitones() {
		result = result.add(other.invert())
	}
	for result.semitones() < 0 {
		result = result.add(other)
	}
	return result
}

// return a parseable string representation of the interval
func (ps *pitchSrc) String() string {
	if ps.Ints[1] == 0 {
		return fmt.Sprintf("%f", ps.Float)
	} else if ps.IsEdx {
		if ps.Float == 12 {
			return fmt.Sprintf("%d\\%d", ps.Ints[0], ps.Ints[1])
		}
		return fmt.Sprintf("%f", ps.semitones())
	}
	return fmt.Sprintf("%d/%d", ps.Ints[0], ps.Ints[1])
}

// reduce a fraction
func reduce(num, den int) (int, int) {
	for i := 2; i <= num && i <= den; i++ {
		if num%i == 0 && den%i == 0 {
			num, den = num/i, den/i
			i--
		}
	}
	return num, den
}
