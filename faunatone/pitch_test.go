package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPitchSrcSemitones(t *testing.T) {
	assert.Equal(t, 12.0, newSemiPitch(12).semitones())
	assert.Equal(t, 12.0, newRatPitch(2, 1).semitones())
	assert.Equal(t, 12.0, newEdxPitch(12, 12, 12).semitones())
}

func TestPitchSrcClass(t *testing.T) {
	assert.Equal(t, 7.0, newSemiPitch(7).class(12))
	assert.Equal(t, 7.0, newSemiPitch(19).class(12))
	assert.Equal(t, 7.0, newSemiPitch(-5).class(12))
	assert.Equal(t, 1.0, newSemiPitch(7).class(6))
}

func TestPitchSrcAdd(t *testing.T) {
	assert.Equal(t, *newSemiPitch(2), *(newSemiPitch(1).add(newSemiPitch(1))))
	assert.Equal(t, *newRatPitch(15, 8), *(newRatPitch(3, 2).add(newRatPitch(5, 4))))
	assert.Equal(t, *newEdxPitch(12, 7, 12), *(newEdxPitch(12, 4, 12).add(newEdxPitch(12, 3, 12))))
	assert.Equal(t, *newRatPitch(3, 1), *(newRatPitch(3, 2).add(newSemiPitch(12))))
	assert.Equal(t, *newEdxPitch(12, 19, 12), *(newEdxPitch(12, 7, 12).add(newSemiPitch(12))))
	assert.Equal(t, *newRatPitch(3, 4), *(newRatPitch(3, 2).add(newEdxPitch(12, -12, 12))))
	assert.Equal(t, *newEdxPitch(12, -5, 12), *(newEdxPitch(12, 7, 12).add(newRatPitch(1, 2))))
}

func TestPitchSrcInvert(t *testing.T) {
	assert.Equal(t, *newSemiPitch(-1), *newSemiPitch(1).invert())
	assert.Equal(t, *newSemiPitch(1), *newSemiPitch(-1).invert())
	assert.Equal(t, *newRatPitch(2, 3), *newRatPitch(3, 2).invert())
	assert.Equal(t, *newRatPitch(3, 2), *newRatPitch(2, 3).invert())
	assert.Equal(t, *newEdxPitch(12, -4, 7), *newEdxPitch(12, 4, 7).invert())
	assert.Equal(t, *newEdxPitch(12, 4, 7), *newEdxPitch(12, -4, 7).invert())
}

func TestPitchSrcMultiply(t *testing.T) {
	assert.Equal(t, *newSemiPitch(3), *newSemiPitch(1).multiply(3))
	assert.Equal(t, *newRatPitch(27, 8), *newRatPitch(3, 2).multiply(3))
	assert.Equal(t, *newEdxPitch(12, 6, 12), *newEdxPitch(12, 2, 12).multiply(3))
}

func TestPitchSrcModulo(t *testing.T) {
	assert.Equal(t, *newSemiPitch(7), *newSemiPitch(7).modulo(newSemiPitch(12)))
	assert.Equal(t, *newSemiPitch(7), *newSemiPitch(19).modulo(newSemiPitch(12)))
	assert.Equal(t, *newSemiPitch(7), *newSemiPitch(-5).modulo(newSemiPitch(12)))
	assert.Equal(t, *newRatPitch(3, 2), *newRatPitch(3, 1).modulo(newSemiPitch(12)))
	assert.Equal(t, *newRatPitch(3, 2), *newRatPitch(3, 4).modulo(newSemiPitch(12)))
	assert.Equal(t, *newEdxPitch(12, 12, 41), *newEdxPitch(12, 53, 41).modulo(newSemiPitch(12)))
	assert.Equal(t, *newEdxPitch(12, 12, 41), *newEdxPitch(12, -29, 41).modulo(newSemiPitch(12)))
}

func TestPitchSrcString(t *testing.T) {
	assert.Equal(t, "7.020000", newSemiPitch(7.02).String())
	assert.Equal(t, "3/2", newRatPitch(3, 2).String())
	assert.Equal(t, "4\\7", newEdxPitch(12, 4, 7).String())
	assert.Equal(t, "7.020000", newEdxPitch(7.02, 9, 9).String())
}

func TestReduce(t *testing.T) {
	// adding to a pitch forces reduction
	assert.Equal(t, *newRatPitch(3, 2), *newRatPitch(3, 2).add(newSemiPitch(0)))
	assert.Equal(t, *newRatPitch(3, 2), *newRatPitch(6, 4).add(newSemiPitch(0)))
	assert.Equal(t, *newRatPitch(3, 2), *newRatPitch(36, 24).add(newSemiPitch(0)))
}
