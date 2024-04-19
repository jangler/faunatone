# Faunatone menu commands

In search-style dialogs (file dialogs, program change, set controller, drum
note, and text event), results are filtered by the input string. Results can
either match exactly, by prefix, by substring, and by word prefixes, in that
order of precedence. Pressing enter with incomplete input will automatically
expand the input to the top search result, except in save and export dialogs,
where input must be explicitly expanded with the tab key.

Keyboard shortcuts are configurable in `config/shortcuts.csv`.

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

**Note...** - Insert a melodic note on event at an interval relative to the
root pitch. Keymaps are a more convenient way to insert notes, but this command
allows you to use notes outside the current keymap.

**Drum note...** - Insert a percussion note on event at a specified absolute
MIDI "pitch". You can also insert percussion notes without using this dialog,
by holding shift and pressing keys from the percussion keymap.

**Note off** - Insert a note off event.

**Pitch bend...** - Insert a pitch bend event to an interval from the keymap,
relative to the current root pitch. To get gradual pitch transitions, use the
**Edit -> Interpolate** command from one note or pitch bend event to another.
The maximum range of a pitch bend is two octaves from the initial pitch, plus
or minus the amount of bending required to produce the initial pitch in the
first place.

**Program change...** - Insert a program (instrument/patch) change event, value
range 1 to 128. Also sets bank MSB and LSB in GS and XG modes.

**Tempo change...** - Insert a tempo change meta-event. Tempos are specified in
beats per minute. The default is 120. Tempos can also be specified as ratios,
in which case they multiply the previous tempo.

**Controller change...** - Insert a control change event for the current
controller (set by **Status -> Set controller...**), value range 0 to 127. Most
controllers default to 0, with the exception of 7 (volume) to 100, 10 (pan) to
64, and 11 (expression) to 127. GS and XG controllers may have other defaults.

**Aftertouch...** - Insert an aftertouch (channel pressure) event, value range
0 to 127. Aftertouch usually produces a vibrato effect.

**Text...** - Insert a text meta-event.

**Release length...** - Insert a release length directive. This tells the
channel allocator not to steal MIDI channels from future off notes in this
channel until a number of beats after the note off. This directive does not
affect notes that have already been turned off.

**MIDI channel range...** - Insert a MIDI channel range directive. This
specifies the minimum and maximum MIDI channel numbers that this virtual
channel will use, value range 1 to 16. This will force multiple notes to play
on the same MIDI channel if necessary.

**MIDI output index...** - Insert a MIDI output index directive. This
specifies the zero-based index of the MIDI output device that this virtual
channel will use. Note that this is the index of the device in the list
provided for `MidiOutPortNumber` in `config.settings.csv`, *not* the port
number itself.

**MIDI mode...** - Insert a directive to change the MIDI mode used by this
track's output.

## Edit

**Go to beat...** - Scroll to a given beat (integers not required) without
changing the selection.

**Delete events** - Delete all selected events.

**Undo** & **Redo** - Undo or redo changes to song data. The size of the undo
buffer is configurable via `config/settings.csv`.

**Cut**, **Copy**, & **Paste** - The usual.

**Mix paste** - A variant of **Paste** that does not delete or overwrite
events.

**Insert division** - Move all events in selected tracks after the start of the
selection down by the size of the selected block (minimum one division).

**Delete division** - Move all events in selected tracks after the start of the
selection up by the size of the selected block (minimum one division). Delete
any events that would be end up above the starting point.

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
events affect, range 0 to 127.

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

**Display as CSV** - Display the current keymap as it would be written to a CSV
file.

**Change key signature...** - Set which accidentals are automatically applied
to which input pitch classes (before transposition by the root pitch). This
does not change the keymap itself. The key signature is lost when loading a new
keymap.

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

**Send system on** - Resets the state of compliant outputs to the GM, GS, XG,
or MT-32 defaults. (And then sends the pitch bend sensitivity RPN.) This also
resets the virtual channel states.

**Cycle mode** - Cycle between GM, GS, XG, MT-32, and MPE modes.
