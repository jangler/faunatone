# Faunatone

A tracker-style microtonal MIDI sequencer. Since MIDI does not have any
widely-implemented native support for microtonality, Faunatone-exported MIDI
files use pitch bending to play non-12edo pitches. The tradeoff is that in
this model, you cannot generally have more than 15-voice melodic polyphony
without experiencing artifacts, although GM 1 only guarantees 16 melodic voices
anyway.

Management of individual output MIDI channels by the user is not required or
even possible; Faunatone operates in terms of virtual channels which it maps
dynamically.

## Status

Faunatone has not seen extensive testing and it is still under initial
development. Users may experience quirks, bugs, crashes, design changes, etc,
although backwards-incompatible changes to the save and config file formats are
less likely to happen.

## Installation

Prebuilt standalone binaries for Windows and Linux are available from
[the releases page](https://github.com/jangler/faunatone/releases).

## Differences from other trackers

If you are familiar with tracker interfaces (Renoise, OpenMPT, SunVox, etc),
you will probably not have trouble picking up Faunatone. If you are *not*
familiar with tracker interfaces, then maybe find a tutorial somewhere?
Faunatone does make a few significant departures from "conventional" trackers:

1. There are no "rows"; beats can be divided into arbitrarily many equal
   divisions, and events are placed with MIDI tick precision (in this case, 960
   ticks per beat). The beat division can be adjusted on the fly using menus
   or keyboard shortcuts; this only affects the cursor's behavior and does not
   change the timing of any events that were already placed.
2. There are no "columns", only "tracks" that can each contain any type of event.
   Multiple tracks can be associated with the same virtual "channel" such that
   ex. a controller change in a track labeled "channel 1" will affect all
   tracks with that label.
3. There are no "patterns"; a song is one continuous sequence of events.
4. As in most trackers, the mapping of keys to intervals/pitches defaults to
   12edo, but this is completely configurable and the mapping can be changed at
   any time. Pitches that don't have names in the current mapping are displayed
   numerically instead of symbolically.

Also, percussion notes are entered by holding the Shift key. The keymap for
percussion notes is separate from the keymap for melodic notes.

## File format

Faunatone save files (\*.fna) are zlib-compressed JSON.

## Further documentation

- [usage.md](https://github.com/jangler/faunatone/blob/master/docs/usage.md):
  general usage guide
- [commands.md](https://github.com/jangler/faunatone/blob/master/docs/commands.md):
  menu commands
- [keymaps.md](https://github.com/jangler/faunatone/blob/master/docs/keymaps.md):
  note input mapping and notation
- [config.md](https://github.com/jangler/faunatone/blob/master/docs/config.md):
  other configuration files
- [building.md](https://github.com/jangler/faunatone/blob/master/docs/config.md):
  building from source
