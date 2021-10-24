# Faunatone menu commands

All filename input dialogs support tab completion.

## File

**New** - Replace the working song data with an empty template.

**Open...** & **Save as..** - Load/save a song from/to the `saves/` folder.

**Export MIDI...** - Export a Standard MIDI File (.mid) of the current song to
the `exports/` folder.

**Quit** - Stop the program.

## Play

**From start** - Play the song, starting at the first beat.

**From top of screen** - Play the song, starting at the top of the current
viewport.

**From cursor** - Play the song, starting at the beginning of the current
selection.

**Stop** - Stop playback, as well as silencing any currently playing notes.

## Select

**Previous division** & **Next division** - Move the selection up or down by
one beat division.

**Previous track** & **Next track** - Move the selection left or right by one
track.

## Insert

**Note...** - Insert a melodic note on event at a specified absolute MIDI
pitch. Keymaps are a more convenient way to insert notes, but this command
allows you to use notes outside the current keymap.

**Drum note...** - Insert a percussion note on event at a specified absolute
MIDI "pitch". Keymaps are a more convenient way to insert notes, but this
command lets you insert percussion notes without having to worry about your
root pitch. Wikipedia has a
[list of GM percussion keys](https://en.wikipedia.org/wiki/General_MIDI#Percussion).

**Note off** - Insert a note off event.

**Pitch bend...** - Insert a pitch bend event to an interval from the keymap,
relative to the current root pitch. To get gradual pitch transitions, use the
**Edit -> Interpolate** command from one pitch bend event to another, or from
one note event to one pitch bend event. The maximum range of a pitch bend is
two octaves from the initial pitch, plus or minus the amount of bending
required to produce the initial pitch in the first place.

**Program change...** - Insert a program (instrument/patch) change event, value
range 1 to 128. Wikipedia has a
[list of GM program numbers](https://en.wikipedia.org/wiki/General_MIDI#Program_change_events).

**Tempo change...** - Insert a tempo change event. Tempos are specified in
beats per minute. The default is 120.

**Controller change...** - Insert a control change event for the current
controller (set by **Status -> Set controller...**), value range 0 to 127. Most
controllers default to 0, with the exception of 7 (volume) to 100, 10 (pan) to
64, and 11 (expression) to 127.

**Aftertouch...** - Insert an aftertouch (channel pressure) event, value range
0 to 127. Aftertouch usually produces a vibrato effect.

## Edit

**Go to beat...** - Scroll to a given beat (integers not required) without
changing the selection.

**Delete events** - Delete all selected events.

**Undo** & **Redo** - Undo or redo changes to song data. The size of the undo
buffer is configurable via `config/settings.csv`.

**Cut**, **Copy**, & **Paste** - The usual.

**Mix paste** - A variant of **Paste** that does not delete selected events
before pasting.

**Insert division** - Move all events in selected tracks after the start of the
selection down by one division.

**Delete division** - Move all events in selected tracks after the start of the
selection up by one division. Delete any events that would be end up above the
starting point.

**Transpose...** - Transpose selected pitches by an interval in the keymap.

**Interpolate...** - Insert events that gradually transition between the values of
the events at the beginning and end of the selection. Events will only be
inserted at beat divisions.

**Multiply...** - Multiply the last value of each event in the selection by a
specified factor.

**Vary...** - Add random variation to the last value of each event in the
selection up to a specified magnitude.

## Status

**Toggle keyjazz** - Off by default. When turned on, disables note entry via
keymap. Useful for experimenting without modifying the song data or undo
buffer.

**Decrease octave** & **Increase octave** - Shift the root pitch up or down by
an octave.

**Capture root pitch** - Set the root pitch to the pitch of the selected note.
A key familiar to users of a certain tracker.

**Set velocity...** - Set the velocity that notes entered via the computer
keyboard will have. Notes entered via MIDI input retain their velocities.

**Set controller...** - Set the controller that inserted controller change
events affect, range 0 to 127. GM level 1 controllers are 1 (modulation), 7
(volume), 10 (pan), 11 (expression), 64 (sustain), 121 (reset all controllers),
and 123 (all notes off).

**Set division...**, **Decrease division**, **Increase division**, **Halve
division**, & **Double division** - Change the number of equal divisions each
beat is divided into for cursor/selection purposes.

**Toggle song follow** - Off by default. When turned on, the view scrolls to
center the play position of the song every time the play position changes.

## Keymap

**Load...** & **Save as...** - Load/save a keymap from/to the `config/keymaps/`
folder.

**Import Scala scale...** - Import a Scala .scl file from the `config/keymaps/`
folder as a keymap.

**Remap key...** - Add or change a mapping in the current keymap.

**Generate equal division...**, **Generate rank-2 scale...**, & **Generate
isomorphic layout...** - See
[keymaps.md](https://github.com/jangler/faunatone/blob/master/docs/keymaps.md)
for details.

## Track

**Set channel...** - Change the virtual channel that the selected tracks
control.

**Insert** - Add one new track per selected track.

**Delete** - Delete selected tracks.

**Move left** & **Move right** - Shift selected tracks left or right.

## MIDI

For export, see **File -> Export MIDI...**.

**Display available inputs** & **Display available outputs** - Display lists of
available MIDI inputs/outputs by index.

**Send pitch bend sensitivity RPN** - MIDI outputs connected to Faunatone after
startup will need this to interpret pitches correctly.

**Send GM system on** - Resets the state of compliant outputs to the GM
defaults. (And then sends the pitch bend sensitivity RPN.)
