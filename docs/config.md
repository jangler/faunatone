# Faunatone configuration

## config/keymaps/

See
[keymaps.md](https://github.com/jangler/faunatone/blob/master/docs/keymaps.md).

## config/shortcuts.csv

See [the SDL_Keycode documentation](https://wiki.libsdl.org/SDL_Keycode) for
key names. Multiple shortcuts (or none) can exist for each menu item. If you
want to explicitly disable a shortcut that exists in the default config, keep
the line but leave the key field blank.

## config/settings.csv

**ColorBeat** - The color of beat lines, in RGBA.

**ColorBg1** - The primary background color, in RGBA.

**ColorBg2** - The secondary background color, in RGBA.

**ColorFg** - The foreground (text) color, in RGBA.

**ColorPlayPos** - The color of the play position highlight, in RGBA.

**ColorSelect** - The color of the selection, in RGBA.

**DefaultKeymap** - The filename of the default melodic keymap. Must be in the
`config/keymaps/` folder.

**PercussionKeymap** - The filename of the percussion keymap. Must be in the
`config/keymaps/` folder.

**Font** - The filename of the font used for drawing. Must be in the `assets/`
folder.

**FontSize** - The point size of the font used for drawing.

**MessageDuration** - How long to display status messages for, in seconds.

**MidiInPortNumber** - The index of the MIDI input port used. -1 means none.

**MidiInputChannels** - How to interpret input from different MIDI channels.
`ignore` means that all input channels are identical. `octaves` means that
channel 1 is mapped to the base octave, channel 2 is mapped an octave higher,
and so on.

**MidiOutPortNumber** - The index of the MIDI output port used. -1 means none.
Can use multiple port numbers, separated by spaces; in this case, the first
port is the default.

**OffDivisionAlpha** - The alpha value to use for drawing events that don't
fall on a current division of the beat, range 0 to 255.

**PitchBendSemitones** - The maximum depth of pitch bends, in semitones. Change
this to match your playback synth if it doesn't support the default range of
two octaves.

**ShiftScrollMult** - Multiplier for scroll wheel distance when a Shift key is held.

**UndoBufferSize** - The approximate limit on the size of the undo buffer, in bytes.

**WindowHeight** - The initial height of the window, in pixels.

**WindowWidth** - The initial width of the window, in pixels.
