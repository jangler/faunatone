# Faunatone general usage guide

This guide assumes some basic familiarity with tracker interfaces and MIDI. It
attempts to cover the main points of interest but is not a complete tutorial.
If you haven't read
[README.md](https://github.com/jangler/faunatone/blob/master/README.md), read
that first. If you wish to use Faunatone's microtonal capabilities, you should
also read 
[keymaps.md](https://github.com/jangler/faunatone/blob/master/docs/keymaps.md),
but read this first.

## Setup

Download and extract a build of Faunatone for your platform from
[the releases page](https://github.com/jangler/faunatone/releases) if you
haven't already. For macOS, try Wine.

Since Faunatone does not include any kind of sound generation capabilities, you
will also need a MIDI-based synthesizer (software or hardware) for it to
interface with. Windows has one built-in. One option for Linux users is
FluidSynth. Regardless of platform, anything that supports GM 1 should work.
The default index of the ports Faunatone connects to for MIDI output and input
can be changed in `config/settings.csv`.

## Sequencing and time control

On the left side of the window are the beat numbers, counting upwards from 1 at
the beginning of the song. There are no "rows" or "patterns", just one
continuous sequence of events. The song ends when the last event is reached. To
leave time for instruments to "fade out" as a song ends, place an additional
note off some distance after the last functional note off in the song.

Most trackers use a fixed number of rows per beat; Faunatone allows you to
freely change how many units the interface divides beats into. Events that do
not align with the current division are drawn as transparent; the degree of
transparency is configurable via `config/settings.csv`.

## Tracks and channels

In Faunatone terminology, a "track" is a vertical sequence of events, and a
"channel" is equivalent to the MIDI definition of the term. Any number of
tracks can be mapped to the same channel. Channels do not have individual
polyphony limits, but only one note can be "on" at a time in each track.

Note that although channels are MIDI-equivalent, they are not mapped one-to-one
onto output MIDI channels. The mapping is dynamic and determined at
playback/export time; each new note is assigned to the MIDI channel that has
least recently had an active note, cutting an existing note off if there are no
other options. This design facilitates arbitrary note pitches via pitch
bending, which operates on a per-MIDI-channel basis.

## Selection and song data input

As a general rule, commands that affect tracks, events, or the selection itself
operate on the entire selection. When inserting most types of events, an
identical event is inserted into each track in the selection. For note on
events (melodic and percussion), the pitches of an input chord are distributed
across the selected tracks. You will need to select multiple tracks in order to
play chords, even in keyjazz mode.

Percussion notes using the GM percussion map can be input by holding Shift. The
percussion keymap does not change with the melodic keymap, since they serve
different functions.

Because there are no "columns" in Faunatone, event parameters are not directly
addressable with the cursor like they are in most trackers.
