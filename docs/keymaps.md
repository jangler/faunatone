# Faunatone keymaps documentation

Faunatone aims to assume as little as possible about what tuning system the
musician is using (one caveat to this is that it currently assumes pure
octaves in some contexts). To this end, the intervals used, their mapping to
keyboard and MIDI input, and the way they are notated are all defined by files
called keymaps. If you have no interest in alternative tunings, you can
disregard keymaps entirely; the default keymap is 12edo with a standard
keyboard mapping and standard notation.

Microtonal music theory is an expansive field, and its details are beyond the
scope of this documentation. However, this document does aim to provide cursory
explanations of some relevant microtonal concepts in terms comprehensible to
musicians without backgrounds in microtonality.

## Interval syntax

Faunatone supports four kinds of syntax for defining intervals:

1. A number of 12edo semitones, ex. `7.02` or `-3.86`
2. A ratio of two numbers, ex. `3/2` or `4/5`
3. A number of edosteps, ex. `7\12` or `-10\31`
4. A reference to a previous entry in the current keymap, ex. `@T` or `@H`

None of the numbers are required to be integers.

Prefixing an interval with an `*` marks the mapping as an accidental. Instead
of entering notes, accidentals modify selected notes by the indicated amount.
Holding Shift while inputting an accidental modifies the root pitch instead of
the selection.

## Keymap file format

Keymaps are comma-separated value (CSV) files with three data per line: input,
notation, and interval. For example, the line `R, F-, 4/3` binds the R key to
an interval of 4/3 (an ascending just fourth), and that interval is to be
displayed as `F-5` when in octave 5. Lines that start with `#` are ignored by
the loader. The equivalent MIDI input mapping would be written as `m65, F-,
4/3`.

The keymap loader is able to "fill in the blanks" in a few ways:

1. The notation datum can be left blank if notation for the given pitch class
   was already provided by another mapping, or if you want the notation to be
   determined by the interval datum (for non-accidentals) or the key datum (for
   accidentals).
2. If the Z-/ and A-; keyboard rows are unmapped, the Q-P and 2-0 rows are
   automatically copied onto those keys, transposed an octave down.
3. Only one octave (or other period) of MIDI input mappings needs to be
   defined; the rest of the values will be extrapolated using the same pattern.
   The defined octave must include both the unison mapping and the
   octave/period mapping.

The first field of a line can be left blank to specify notation for an interval
interval without mapping it.

For the purposes of keymaps, keyboard input is interpreted by its scancode (and
therefore its position on the physical keyboard) rather than its symbolic
value. This means that keymaps must always be written for QWERTY keyboards, but
should work identically in a different software-level keyboard layout. For key
names, see
[the SCL_Scancode documentation](https://wiki.libsdl.org/SDL_Scancode).

## Loading and saving keymaps

Faunatone includes a few example keymaps in the `config/keymaps/` folder. Other
keymaps must also be placed in this folder to be loaded via the Keymap menu.

Faunatone .faun save files also include the working keymap along with the song
data. You can extract a keymap from a save file by loading the save file and
then saving the keymap.

## Generating keymaps

Faunatone is able to automatically generate some specific types of parametric
keymaps using commands in the Keymaps menu. These are by no means the only
types of framework worth exploring, but facilities for them are included
because they are "proven" and simple to generate programmatically. For other
historical and experimental approaches, see well temperaments, tetrachords,
tonality diamonds, combination product sets, Euler-Fokker genera, rank-3
scales, and the traditional tonal frameworks of various non-Western cultures.
[Scala](https://www.huygens-fokker.org/scala/) may be able to help you with
these.

You may wish to generate a keymap, save it, and then edit it as a text file in
order to provide different notation, alter its layout, or add accidentals.

### Equal division keymaps

Equal division keymaps are formed by splitting an interval into a number of
equal steps. The modern standard for Western tuning fits this description, as
the octave is split into 12 identical steps. This is called 12edo (12-equal
division of the octave) or sometimes 12tet (12-tone equal temperament).

For generated equal division scales of 10 or fewer notes, Faunatone maps one
ascending scale to the Q-P row, and one to the Z-/ row an octave lower. For 11
to 19 notes, the Q-P and Z-/ rows alternate with the 2-0 and S-; rows, much
like the traditional tracker keyboard layout, but with no gaps between "black
keys". For 20 or more notes, pitches ascend on the keys 2W3E... and descend on
the keys /;.L.... In all cases, Q is the root, 1 is an ascending octave, and A
is a descending octave. In this way, complete scales of up to 38 notes can be
played on the computer keyboard. Keys outside the quadrilateral bounded by 1,
0, Z, and / are left unmapped in order to preserve the regularity of the layout
and leave room for added accidentals. MIDI pitches are mapped with 60 as the
root and no gaps. The generated notation is based on the scale degree.

The "zeta peaks" 5, 7, 12, 19, 22, 31, 41, and 53 are some edos that generally
provide close approximations of just intervals relative to their size, although
the last two won't fit in a generated layout. Other relatively well-known equal
scales include Wendy Carlos's alpha, beta, and gamma scales at approximately
9edf, 11edf, and 20edf (where f means 3/2 fifth), and the Bohlen-Pierce scale
at 13edt (where t means 3/1 "tritave").

### Rank-2 scale keymaps

Rank-2 scales are formed by two intervals called the period (usually an octave
or an equal division of one) and the generator. The scale is formed by sorting
the results of stacking the generator modulo the period. For example, a
diatonic scale in 12edo tuning can be formed by using a period of an octave and
a generator of 7\12, and sorting the resulting series [0\12, 7\12, 2\12, 9\12,
4\12, 11\12, 6\12]. The series could be continued infinitely, although in this
case it would repeat after 12 notes because of the 12edo tuning.

For every combination of period and generator, there are certain numbers of
notes at which the resulting scale is *distributionally even*, one potentially
useful consequence of which is that there are at most two different intervals
formed by each number of scale steps. This does not guarantee that the scale is
*proper*, which would mean that a larger number of scale steps never
corresponds to a smaller interval.

Rank-2 scales represent a middle ground between the conceptual simplicity of
equal divisions (which are rank-1 scales) and the harmonic precision of just
intonation. Using the right combination of period and generator, you can get
better-tuned intervals than in an edo with the same number of notes. The
tradeoff is that not every represented interval will be accessible from every
note in the scale.

Rank-2 scales are usually discussed in terms of temperaments, which effectively
reduce the number of dimensions in a JI space by eliminating the differences
between specific intervals. Some relatively well-known temperaments include
meantone, schismatic/Helmholtz, miracle, magic, porcupine, pajara, and
superpyth.

With regard to keyboard layout and notation, Faunatone uses the same rules for
rank-2 scales as it does for equal divisions.

### Isomorphic keymaps

Isomorphic keymaps have the property that every shape on the keyboard
corresponds to the same interval. For two intervals A and B, a generated
isomorphic keymap will fill the quadrilateral bounded by the 1, 0, Z, and /
keys such that the G key is the root, one step to the right of any key is
interval A, and one step up and to the right is interval B. The generated
notation is based on the vector from the root pitch.

One example of an isomorphic keymap is the Wicki-Hayden note layout for
concertinas, formed by the intervals 2\12 and 7\12. A 5-limit JI lattice can be
formed by the intervals 3/2 and 5/4 (among other pairs). Additional primes
require additional dimensions, which can be accessed via accidentals.

## Importing Scala scales

Faunatone can load files in
[.scl format](https://www.huygens-fokker.org/scala/scl_format.html) from the
`config/keymaps/` folder using the same layout and notation rules as for
generated equal divisions. You can then save the resulting keymap if you wish
to edit it as a Faunatone keymap rather than a Scala scale.

Scala .kbm files are not currently supported.
