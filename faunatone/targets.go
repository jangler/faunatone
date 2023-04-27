package main

var (
	instrumentTargets = [][]*tabTarget{
		// GM
		{
			{display: "Acoustic Grand Piano", value: "1 0 0"},
			{display: "Bright Acoustic Piano", value: "2 0 0"},
			{display: "Electric Grand Piano", value: "3 0 0"},
			{display: "Honky-tonk Piano", value: "4 0 0"},
			{display: "Electric Piano 1", value: "5 0 0"},
			{display: "Electric Piano 2", value: "6 0 0"},
			{display: "Harpsichord", value: "7 0 0"},
			{display: "Clavinet", value: "8 0 0"},
			{display: "Celesta", value: "9 0 0"},
			{display: "Glockenspiel", value: "10 0 0"},
			{display: "Music Box", value: "11 0 0"},
			{display: "Vibraphone", value: "12 0 0"},
			{display: "Marimba", value: "13 0 0"},
			{display: "Xylophone", value: "14 0 0"},
			{display: "Tubular Bells", value: "15 0 0"},
			{display: "Dulcimer", value: "16 0 0"},
			{display: "Drawbar Organ", value: "17 0 0"},
			{display: "Percussive Organ", value: "18 0 0"},
			{display: "Rock Organ", value: "19 0 0"},
			{display: "Church Organ", value: "20 0 0"},
			{display: "Reed Organ", value: "21 0 0"},
			{display: "Accordion", value: "22 0 0"},
			{display: "Harmonica", value: "23 0 0"},
			{display: "Tango Accordion", value: "24 0 0"},
			{display: "Acoustic Guitar (nylon)", value: "25 0 0"},
			{display: "Acoustic Guitar (steel)", value: "26 0 0"},
			{display: "Electric Guitar (jazz)", value: "27 0 0"},
			{display: "Electric Guitar (clean)", value: "28 0 0"},
			{display: "Electric Guitar (muted)", value: "29 0 0"},
			{display: "Overdriven Guitar", value: "30 0 0"},
			{display: "Distortion Guitar", value: "31 0 0"},
			{display: "Guitar harmonics", value: "32 0 0"},
			{display: "Acoustic Bass", value: "33 0 0"},
			{display: "Electric Bass (finger)", value: "34 0 0"},
			{display: "Electric Bass (pick)", value: "35 0 0"},
			{display: "Fretless Bass", value: "36 0 0"},
			{display: "Slap Bass 1", value: "37 0 0"},
			{display: "Slap Bass 2", value: "38 0 0"},
			{display: "Synth Bass 1", value: "39 0 0"},
			{display: "Synth Bass 2", value: "40 0 0"},
			{display: "Violin", value: "41 0 0"},
			{display: "Viola", value: "42 0 0"},
			{display: "Cello", value: "43 0 0"},
			{display: "Contrabass", value: "44 0 0"},
			{display: "Tremolo Strings", value: "45 0 0"},
			{display: "Pizzicato Strings", value: "46 0 0"},
			{display: "Orchestral Harp", value: "47 0 0"},
			{display: "Timpani", value: "48 0 0"},
			{display: "String Ensemble 1", value: "49 0 0"},
			{display: "String Ensemble 2", value: "50 0 0"},
			{display: "SynthStrings 1", value: "51 0 0"},
			{display: "SynthStrings 2", value: "52 0 0"},
			{display: "Choir Aahs", value: "53 0 0"},
			{display: "Voice Oohs", value: "54 0 0"},
			{display: "Synth Voice", value: "55 0 0"},
			{display: "Orchestra Hit", value: "56 0 0"},
			{display: "Trumpet", value: "57 0 0"},
			{display: "Trombone", value: "58 0 0"},
			{display: "Tuba", value: "59 0 0"},
			{display: "Muted Trumpet", value: "60 0 0"},
			{display: "French Horn", value: "61 0 0"},
			{display: "Brass Section", value: "62 0 0"},
			{display: "SynthBrass 1", value: "63 0 0"},
			{display: "SynthBrass 2", value: "64 0 0"},
			{display: "Soprano Sax", value: "65 0 0"},
			{display: "Alto Sax", value: "66 0 0"},
			{display: "Tenor Sax", value: "67 0 0"},
			{display: "Baritone Sax", value: "68 0 0"},
			{display: "Oboe", value: "69 0 0"},
			{display: "English Horn", value: "70 0 0"},
			{display: "Bassoon", value: "71 0 0"},
			{display: "Clarinet", value: "72 0 0"},
			{display: "Piccolo", value: "73 0 0"},
			{display: "Flute", value: "74 0 0"},
			{display: "Recorder", value: "75 0 0"},
			{display: "Pan Flute", value: "76 0 0"},
			{display: "Blown Bottle", value: "77 0 0"},
			{display: "Shakuhachi", value: "78 0 0"},
			{display: "Whistle", value: "79 0 0"},
			{display: "Ocarina", value: "80 0 0"},
			{display: "Lead 1 (square)", value: "81 0 0"},
			{display: "Lead 2 (sawtooth)", value: "82 0 0"},
			{display: "Lead 3 (calliope)", value: "83 0 0"},
			{display: "Lead 4 (chiff)", value: "84 0 0"},
			{display: "Lead 5 (charang)", value: "85 0 0"},
			{display: "Lead 6 (voice)", value: "86 0 0"},
			{display: "Lead 7 (fifths)", value: "87 0 0"},
			{display: "Lead 8 (bass + lead)", value: "88 0 0"},
			{display: "Pad 1 (new age)", value: "89 0 0"},
			{display: "Pad 2 (warm)", value: "90 0 0"},
			{display: "Pad 3 (polysynth)", value: "91 0 0"},
			{display: "Pad 4 (choir)", value: "92 0 0"},
			{display: "Pad 5 (bowed)", value: "93 0 0"},
			{display: "Pad 6 (metallic)", value: "94 0 0"},
			{display: "Pad 7 (halo)", value: "95 0 0"},
			{display: "Pad 8 (sweep)", value: "96 0 0"},
			{display: "FX 1 (rain)", value: "97 0 0"},
			{display: "FX 2 (soundtrack)", value: "98 0 0"},
			{display: "FX 3 (crystal)", value: "99 0 0"},
			{display: "FX 4 (atmosphere)", value: "100 0 0"},
			{display: "FX 5 (brightness)", value: "101 0 0"},
			{display: "FX 6 (goblins)", value: "102 0 0"},
			{display: "FX 7 (echoes)", value: "103 0 0"},
			{display: "FX 8 (sci-fi)", value: "104 0 0"},
			{display: "Sitar", value: "105 0 0"},
			{display: "Banjo", value: "106 0 0"},
			{display: "Shamisen", value: "107 0 0"},
			{display: "Koto", value: "108 0 0"},
			{display: "Kalimba", value: "109 0 0"},
			{display: "Bagpipe", value: "110 0 0"},
			{display: "Fiddle", value: "111 0 0"},
			{display: "Shanai", value: "112 0 0"},
			{display: "Tinkle Bell", value: "113 0 0"},
			{display: "Agogo", value: "114 0 0"},
			{display: "Steel Drums", value: "115 0 0"},
			{display: "Woodblock", value: "116 0 0"},
			{display: "Taiko Drum", value: "117 0 0"},
			{display: "Melodic Tom", value: "118 0 0"},
			{display: "Synth Drum", value: "119 0 0"},
			{display: "Reverse Cymbal", value: "120 0 0"},
			{display: "Guitar Fret Noise", value: "121 0 0"},
			{display: "Breath Noise", value: "122 0 0"},
			{display: "Seashore", value: "123 0 0"},
			{display: "Bird Tweet", value: "124 0 0"},
			{display: "Telephone Ring", value: "125 0 0"},
			{display: "Helicopter", value: "126 0 0"},
			{display: "Applause", value: "127 0 0"},
			{display: "Gunshot", value: "128 0 0"},
		},

		// GS
		{
			{display: "Piano 1", value: "1 0 0"},
			{display: "Piano 1w", value: "1 8 0"},
			{display: "Piano 1d", value: "1 16 0"},
			{display: "Piano 2", value: "2 0 0"},
			{display: "Piano 2w", value: "2 8 0"},
			{display: "Piano 3", value: "3 0 0"},
			{display: "Piano 3w", value: "3 8 0"},
			{display: "Honky-tonk", value: "4 0 0"},
			{display: "Honky-tonk w", value: "4 8 0"},
			{display: "E.Piano 1", value: "5 0 0"},
			{display: "Detuned EP 1", value: "5 8 0"},
			{display: "E.Piano 1v", value: "5 16 0"},
			{display: "60's E.Piano", value: "5 24 0"},
			{display: "E.Piano 2", value: "6 0 0"},
			{display: "Detuned EP 2", value: "6 8 0"},
			{display: "E.Piano 2v", value: "6 16 0"},
			{display: "Harpsichord", value: "7 0 0"},
			{display: "Coupled Hps.", value: "7 8 0"},
			{display: "Harpsi.w", value: "7 16 0"},
			{display: "Harpsi.o", value: "7 24 0"},
			{display: "Vibraphone", value: "12 0 0"},
			{display: "Vib.w", value: "12 8 0"},
			{display: "Marimba", value: "13 0 0"},
			{display: "Marimba w", value: "13 8 0"},
			{display: "Tubular-bell", value: "15 0 0"},
			{display: "Church Bell", value: "15 8 0"},
			{display: "Carillon", value: "15 9 0"},
			{display: "Organ 1", value: "17 0 0"},
			{display: "Detuned Or.1", value: "17 8 0"},
			{display: "60's Organ 1", value: "17 16 0"},
			{display: "Organ 4", value: "17 32 0"},
			{display: "Organ 2", value: "18 0 0"},
			{display: "Detuned Or.2", value: "18 8 0"},
			{display: "Organ 5", value: "18 32 0"},
			{display: "Church Org.1", value: "20 0 0"},
			{display: "Church Org.2", value: "20 8 0"},
			{display: "Church Org.3", value: "20 16 0"},
			{display: "Accordion Fr", value: "22 0 0"},
			{display: "Accordion It", value: "22 8 0"},
			{display: "Nylon-str.Gt", value: "25 0 0"},
			{display: "Ukulele", value: "25 8 0"},
			{display: "Nylon Gt.o", value: "25 16 0"},
			{display: "Nylon Gt.2", value: "25 32 0"},
			{display: "Steel-str.Gt", value: "26 0 0"},
			{display: "12-str.Gt", value: "26 8 0"},
			{display: "Mandolin", value: "26 16 0"},
			{display: "Jazz Gt.", value: "27 0 0"},
			{display: "Hawaiian Gt.", value: "27 8 0"},
			{display: "Clean Gt.", value: "28 0 0"},
			{display: "Chorus Gt.", value: "28 8 0"},
			{display: "Muted Gt.", value: "29 0 0"},
			{display: "Funk Gt.", value: "29 8 0"},
			{display: "Funk Gt.2", value: "29 16 0"},
			{display: "DistortionGt", value: "31 0 0"},
			{display: "Feedback Gt.", value: "31 8 0"},
			{display: "Gt.Harmonics", value: "32 0 0"},
			{display: "Gt. Feedback", value: "32 8 0"},
			{display: "Synth Bass 1", value: "39 0 0"},
			{display: "SynthBass101", value: "39 1 0"},
			{display: "Synth Bass 3", value: "39 8 0"},
			{display: "Synth Bass 2", value: "40 0 0"},
			{display: "Synth Bass 4", value: "40 8 0"},
			{display: "Rubber Bass", value: "40 16 0"},
			{display: "Violin", value: "41 0 0"},
			{display: "Slow Violin", value: "41 8 0"},
			{display: "Strings", value: "49 0 0"},
			{display: "Orchestra", value: "49 8 0"},
			{display: "Syn.Strings1", value: "51 0 0"},
			{display: "Syn.Strings3", value: "51 8 0"},
			{display: "Choir Aahs", value: "53 0 0"},
			{display: "Choir Aahs 2", value: "53 32 0"},
			{display: "Trombone", value: "58 0 0"},
			{display: "Trombone 2", value: "58 1 0"},
			{display: "French Horns 2", value: "61 0 0"},
			{display: "Fr.Horn 2", value: "61 1 0"},
			{display: "Brass 1", value: "62 0 0"},
			{display: "Brass 2", value: "62 8 0"},
			{display: "Synth Brass1", value: "63 0 0"},
			{display: "Synth Brass3", value: "63 8 0"},
			{display: "AnalogBrass1", value: "63 16 0"},
			{display: "Synth Brass2", value: "64 0 0"},
			{display: "Synth Brass4", value: "64 8 0"},
			{display: "AnalogBrass2", value: "64 16 0"},
			{display: "Square Wave", value: "81 0 0"},
			{display: "Square", value: "81 1 0"},
			{display: "Sine Wave", value: "81 8 0"},
			{display: "Saw Wave", value: "82 0 0"},
			{display: "Saw", value: "82 1 0"},
			{display: "Doctor Solo", value: "82 8 0"},
			{display: "Crystal", value: "99 0 0"},
			{display: "Syn Mallet", value: "99 1 0"},
			{display: "Echo Drops", value: "103 0 0"},
			{display: "Echo Bell", value: "103 1 0"},
			{display: "Echo Pan", value: "103 2 0"},
			{display: "Sitar", value: "105 0 0"},
			{display: "Sitar 2", value: "105 1 0"},
			{display: "Koto", value: "108 0 0"},
			{display: "Taisho Koto", value: "108 8 0"},
			{display: "Woodblock", value: "116 0 0"},
			{display: "Castanets", value: "116 8 0"},
			{display: "Taiko", value: "117 0 0"},
			{display: "Concert BD", value: "117 8 0"},
			{display: "Melo. Tom 1", value: "118 0 0"},
			{display: "Melo. Tom 2", value: "118 8 0"},
			{display: "Synth Drum", value: "119 0 0"},
			{display: "808 Tom", value: "119 8 0"},
			{display: "Elec Perc.", value: "119 9 0"},
			{display: "Gt.FretNoise", value: "121 0 0"},
			{display: "Gt.Cut Noise", value: "121 1 0"},
			{display: "String Slap", value: "121 2 0"},
			{display: "Breath Noise", value: "122 0 0"},
			{display: "Fl.Key Click", value: "122 1 0"},
			{display: "Seashore", value: "123 0 0"},
			{display: "Rain", value: "123 1 0"},
			{display: "Thunder", value: "123 2 0"},
			{display: "Wind", value: "123 3 0"},
			{display: "Stream", value: "123 4 0"},
			{display: "Bubble", value: "123 5 0"},
			{display: "Bird", value: "124 0 0"},
			{display: "Dog", value: "124 1 0"},
			{display: "Horse-Gallop", value: "124 2 0"},
			{display: "Bird 2", value: "124 3 0"},
			{display: "Telephone 1", value: "125 0 0"},
			{display: "Telephone 2", value: "125 1 0"},
			{display: "DoorCreaking", value: "125 2 0"},
			{display: "Door", value: "125 3 0"},
			{display: "Scratch", value: "125 4 0"},
			{display: "Wind Chimes", value: "125 5 0"},
			{display: "Helicopter", value: "126 0 0"},
			{display: "Car-Engine", value: "126 1 0"},
			{display: "Car-Stop", value: "126 2 0"},
			{display: "Car-Pass", value: "126 3 0"},
			{display: "Car-Crash", value: "126 4 0"},
			{display: "Siren", value: "126 5 0"},
			{display: "Train", value: "126 6 0"},
			{display: "Jetplane", value: "126 7 0"},
			{display: "Starship", value: "126 8 0"},
			{display: "Burst Noise", value: "126 9 0"},
			{display: "Applause", value: "127 0 0"},
			{display: "Laughing", value: "127 1 0"},
			{display: "Screaming", value: "127 2 0"},
			{display: "Punch", value: "127 3 0"},
			{display: "Heart Beat", value: "127 4 0"},
			{display: "Footsteps", value: "127 5 0"},
			{display: "Gun Shot", value: "128 0 0"},
			{display: "Machine Gun", value: "128 1 0"},
			{display: "Lasergun", value: "128 2 0"},
			{display: "Explosion", value: "128 3 0"},
		},

		// XG
		{
			{display: "GrandPno", value: "1 0 0"},
			{display: "GrndPnoK", value: "1 0 1"},
			{display: "MelloGrP", value: "1 0 18"},
			{display: "PianoStr", value: "1 0 40"},
			{display: "Dream", value: "1 0 41"},
			{display: "BritePno", value: "2 0 0"},
			{display: "BritPnoK", value: "2 0 1"},
			{display: "E.Grand", value: "3 0 0"},
			{display: "ElGrPnoK", value: "3 0 1"},
			{display: "Det.CP80", value: "3 0 32"},
			{display: "ElGrPno1", value: "3 0 40"},
			{display: "ElGrPno2", value: "3 0 41"},
			{display: "HnkyTonk", value: "4 0 0"},
			{display: "HnkyTnkK", value: "4 0 1"},
			{display: "E.Piano1", value: "5 0 0"},
			{display: "El.Pno1K", value: "5 0 1"},
			{display: "MelloEP1", value: "5 0 18"},
			{display: "Chor.EP1", value: "5 0 32"},
			{display: "HardEl.P", value: "5 0 40"},
			{display: "VX El.P1", value: "5 0 45"},
			{display: "60sEl.P", value: "5 0 64"},
			{display: "E.Piano2", value: "6 0 0"},
			{display: "El.Pno2K", value: "6 0 1"},
			{display: "Chor.EP2", value: "6 0 32"},
			{display: "DX Hard", value: "6 0 33"},
			{display: "DXLegend", value: "6 0 34"},
			{display: "DX Phase", value: "6 0 40"},
			{display: "DX+Analg", value: "6 0 41"},
			{display: "DXKotoEP", value: "6 0 42"},
			{display: "VX El.P2", value: "6 0 45"},
			{display: "Harpsi.", value: "7 0 0"},
			{display: "Harpsi.K", value: "7 0 1"},
			{display: "Harpsi.2", value: "7 0 25"},
			{display: "Harpsi.3", value: "7 0 35"},
			{display: "Clavi.", value: "8 0 0"},
			{display: "Clavi. K", value: "8 0 1"},
			{display: "ClaviWah", value: "8 0 27"},
			{display: "PulseClv", value: "8 0 64"},
			{display: "PierceCl", value: "8 0 65"},
			{display: "Celesta", value: "9 0 0"},
			{display: "Glocken", value: "10 0 0"},
			{display: "MusicBox", value: "11 0 0"},
			{display: "Orgel", value: "11 0 64"},
			{display: "Vibes", value: "12 0 0"},
			{display: "VibesK", value: "12 0 1"},
			{display: "HardVibe", value: "12 0 45"},
			{display: "Marimba", value: "13 0 0"},
			{display: "MarimbaK", value: "13 0 1"},
			{display: "SineMrmb", value: "13 0 65"},
			{display: "Balafon2", value: "13 0 97"},
			{display: "Log Drum", value: "13 0 98"},
			{display: "Xylophon", value: "14 0 0"},
			{display: "TubulBel", value: "15 0 0"},
			{display: "ChrchBel", value: "15 0 96"},
			{display: "Carillon", value: "15 0 97"},
			{display: "Dulcimer", value: "16 0 0"},
			{display: "Dulcimr2", value: "16 0 35"},
			{display: "Cimbalom", value: "16 0 96"},
			{display: "Santur", value: "16 0 97"},
			{display: "DrawOrgn", value: "17 0 0"},
			{display: "DetDrwOr", value: "17 0 32"},
			{display: "60sDrOr1", value: "17 0 33"},
			{display: "60sDrOr2", value: "17 0 34"},
			{display: "70sDrOr1", value: "17 0 35"},
			{display: "DrawOrg2", value: "17 0 36"},
			{display: "60sDrOr3", value: "17 0 37"},
			{display: "70sDrOr2", value: "17 0 65"},
			{display: "CheezOrg", value: "17 0 66"},
			{display: "DrawOrg3", value: "17 0 67"},
			{display: "EvenBar", value: "17 0 38"},
			{display: "16+2\"2/3", value: "17 0 40"},
			{display: "Organ Ba", value: "17 0 65"},
			{display: "PercOrgn", value: "18 0 0"},
			{display: "70sPcOr1", value: "18 0 24"},
			{display: "DetPrcOr", value: "18 0 32"},
			{display: "LiteOrg", value: "18 0 33"},
			{display: "PercOrg2", value: "18 0 37"},
			{display: "RockOrgn", value: "19 0 0"},
			{display: "RotaryOr", value: "19 0 65"},
			{display: "SloRotar", value: "19 0 65"},
			{display: "FstRotar", value: "19 0 66"},
			{display: "ChrchOrg", value: "20 0 0"},
			{display: "ChurOrg3", value: "20 0 32"},
			{display: "ChurOrg2", value: "20 0 35"},
			{display: "NotreDam", value: "20 0 40"},
			{display: "OrgFlute", value: "20 0 64"},
			{display: "TrmOrgFl", value: "20 0 65"},
			{display: "ReedOrgn", value: "21 0 0"},
			{display: "Acordion", value: "22 0 0"},
			{display: "AccordIt", value: "22 0 32"},
			{display: "Harmnica", value: "23 0 0"},
			{display: "Harmo 2", value: "23 0 32"},
			{display: "TangoAcd", value: "24 0 0"},
			{display: "TngoAcd2", value: "24 0 64"},
			{display: "NylonGtr", value: "25 0 0"},
			{display: "NylonGt2", value: "25 0 16"},
			{display: "NylonGt3", value: "25 0 25"},
			{display: "VelGtHrm", value: "25 0 43"},
			{display: "Ukulele", value: "25 0 96"},
			{display: "SteelGtr", value: "26 0 0"},
			{display: "SteelGt2", value: "26 0 16"},
			{display: "12StrGtr", value: "26 0 35"},
			{display: "Nyln&Stl", value: "26 0 40"},
			{display: "Stl&Body", value: "26 0 41"},
			{display: "Mandolin", value: "26 0 96"},
			{display: "Jazz Gtr", value: "27 0 0"},
			{display: "MelloGtr", value: "27 0 18"},
			{display: "JazzAmp", value: "27 0 32"},
			{display: "CleanGtr", value: "28 0 0"},
			{display: "ChorusGt", value: "28 0 32"},
			{display: "Mute.Gtr", value: "29 0 0"},
			{display: "FunkGtr1", value: "29 0 40"},
			{display: "MuteStlG", value: "29 0 41"},
			{display: "FunkGtr2", value: "29 0 43"},
			{display: "Jazz Man", value: "29 0 45"},
			{display: "Ovrdrive", value: "30 0 0"},
			{display: "Gt.Pinch", value: "30 0 43"},
			{display: "Dist.Gtr", value: "31 0 0"},
			{display: "FeedbkGt", value: "31 0 40"},
			{display: "FeedbGt2", value: "31 0 41"},
			{display: "GtrHarmo", value: "32 0 0"},
			{display: "GtFeedbk", value: "32 0 65"},
			{display: "GtrHrmo2", value: "32 0 66"},
			{display: "Aco.Bass", value: "33 0 0"},
			{display: "JazzRthm", value: "33 0 40"},
			{display: "VXUprght", value: "33 0 45"},
			{display: "FngrBass", value: "34 0 0"},
			{display: "FingrDrk", value: "34 0 18"},
			{display: "FlangeBa", value: "34 0 27"},
			{display: "Ba&DstEG", value: "34 0 40"},
			{display: "FngrSlap", value: "34 0 43"},
			{display: "FngBass2", value: "34 0 45"},
			{display: "ModAlem", value: "34 0 65"},
			{display: "PickBass", value: "35 0 0"},
			{display: "MutePkBa", value: "35 0 28"},
			{display: "Fretless", value: "36 0 0"},
			{display: "Fretles2", value: "36 0 32"},
			{display: "Fretles3", value: "36 0 33"},
			{display: "Fretles4", value: "36 0 34"},
			{display: "SynFretl", value: "36 0 96"},
			{display: "Smooth", value: "36 0 97"},
			{display: "SlapBas1", value: "37 0 0"},
			{display: "ResoSlap", value: "37 0 27"},
			{display: "PunchThm", value: "37 0 32"},
			{display: "SlapBas2", value: "38 0 0"},
			{display: "VeloSlap", value: "38 0 43"},
			{display: "SynBass1", value: "39 0 0"},
			{display: "SynBa1Dk", value: "39 0 18"},
			{display: "FastResB", value: "39 0 20"},
			{display: "AcidBass", value: "39 0 24"},
			{display: "Clv Bass", value: "39 0 35"},
			{display: "TeknoBa", value: "39 0 40"},
			{display: "Oscar", value: "39 0 64"},
			{display: "SqrBass.39", value: "0 65 "},
			{display: "RubberBa", value: "39 0 66"},
			{display: "Hammer", value: "39 0 96"},
			{display: "SynBass2", value: "40 0 0"},
			{display: "MelloSB1", value: "40 0 6"},
			{display: "Seq Bass", value: "40 0 12"},
			{display: "ClkSynBa", value: "40 0 18"},
			{display: "SynBa2Dk", value: "40 0 19"},
			{display: "SmthBa 2", value: "40 0 32"},
			{display: "ModulrBa", value: "40 0 40"},
			{display: "DX Bass", value: "40 0 41"},
			{display: "X WireBa", value: "40 0 64"},
			{display: "Violin", value: "41 0 0"},
			{display: "SlowVln", value: "41 0 8"},
			{display: "Viola", value: "42 0 0"},
			{display: "Cello", value: "43 0 0"},
			{display: "Contrabs", value: "44 0 0"},
			{display: "Trem.Str", value: "45 0 0"},
			{display: "SlowTrStr", value: "45 0 8"},
			{display: "Susp Str", value: "45 0 40"},
			{display: "Pizz.Str", value: "46 0 0"},
			{display: "Harp", value: "47 0 0"},
			{display: "YangChin", value: "47 0 40"},
			{display: "Timpani", value: "48 0 0"},
			{display: "Strings1", value: "49 0 0"},
			{display: "S.Strngs", value: "49 0 3"},
			{display: "SlowStr", value: "49 0 8"},
			{display: "ArcoStr", value: "49 0 24"},
			{display: "60sStrng", value: "49 0 35"},
			{display: "Orchestr", value: "49 0 40"},
			{display: "Orchstr2", value: "49 0 41"},
			{display: "TremOrch", value: "49 0 42"},
			{display: "VeloStr", value: "49 0 45"},
			{display: "Strings2", value: "50 0 0"},
			{display: "S.SlwStr", value: "50 0 3"},
			{display: "LegatoSt", value: "50 0 8"},
			{display: "Warm Str", value: "50 0 40"},
			{display: "Kingdom", value: "50 0 41"},
			{display: "70s Str", value: "50 0 64"},
			{display: "Str Ens3", value: "50 0 65"},
			{display: "Syn.Str1", value: "51 0 0"},
			{display: "ResoStr", value: "51 0 27"},
			{display: "Syn Str4", value: "51 0 64"},
			{display: "SS St", value: "51 0 65"},
			{display: "Syn.Str2", value: "52 0 0"},
			{display: "ChoirAah", value: "53 0 0"},
			{display: "S.Choir", value: "53 0 3"},
			{display: "Ch.Aahs2", value: "53 0 16"},
			{display: "MelChoir", value: "53 0 32"},
			{display: "ChoirStr", value: "53 0 40"},
			{display: "VoiceOoh", value: "54 0 0"},
			{display: "SynVoice", value: "55 0 0"},
			{display: "SynVox2", value: "55 0 40"},
			{display: "Choral", value: "55 0 41"},
			{display: "AnaVoice", value: "55 0 64"},
			{display: "Orch.Hit", value: "56 0 0"},
			{display: "OrchHit2", value: "56 0 35"},
			{display: "Impact", value: "56 0 64"},
			{display: "Trumpet", value: "57 0 0"},
			{display: "Trumpet2", value: "57 0 16"},
			{display: "BriteTrp", value: "57 0 17"},
			{display: "WarmTrp", value: "57 0 32"},
			{display: "FluglHrn", value: "57 0 96"},
			{display: "Trombone", value: "58 0 0"},
			{display: "Trmbone2", value: "58 0 17"},
			{display: "Tuba", value: "59 0 0"},
			{display: "Tuba 2", value: "59 0 16"},
			{display: "Mute.Trp", value: "60 0 0"},
			{display: "Fr.Horn", value: "61 0 0"},
			{display: "FrHrSolo", value: "61 0 6"},
			{display: "FrHorn2", value: "61 0 32"},
			{display: "HornOrch", value: "61 0 37"},
			{display: "BrasSect", value: "62 0 0"},
			{display: "Tp&TbSec", value: "62 0 35"},
			{display: "BrssSec2", value: "62 0 40"},
			{display: "HiBrass", value: "62 0 41"},
			{display: "MelloBrs", value: "62 0 42"},
			{display: "SynBras1", value: "63 0 0"},
			{display: "QuackBr", value: "63 0 12"},
			{display: "RezSynBr", value: "63 0 20"},
			{display: "PolyBrss", value: "63 0 24"},
			{display: "SynBras3", value: "63 0 27"},
			{display: "JumpBrss", value: "63 0 32"},
			{display: "AnaVelBr", value: "63 0 45"},
			{display: "AnaBrss1", value: "63 0 64"},
			{display: "SynBras2", value: "64 0 0"},
			{display: "Soft Brs", value: "64 0 18"},
			{display: "SynBrss4", value: "64 0 40"},
			{display: "ChoirBrs", value: "64 0 41"},
			{display: "VelBrss2", value: "64 0 45"},
			{display: "AnaBrss2", value: "64 0 64"},
			{display: "SprnoSax", value: "65 0 0"},
			{display: "Alto Sax", value: "66 0 0"},
			{display: "Sax Sect", value: "66 0 40"},
			{display: "HyprAlto", value: "66 0 43"},
			{display: "TenorSax", value: "67 0 0"},
			{display: "BrthTnSx", value: "67 0 40"},
			{display: "SoftTenr", value: "67 0 41"},
			{display: "TnrSax 2", value: "67 0 64"},
			{display: "Bari.Sax", value: "68 0 0"},
			{display: "Oboe", value: "69 0 0"},
			{display: "Eng.Horn", value: "70 0 0"},
			{display: "Bassoon", value: "71 0 0"},
			{display: "Clarinet", value: "72 0 0"},
			{display: "Piccolo", value: "73 0 0"},
			{display: "Flute", value: "74 0 0"},
			{display: "Recorder", value: "75 0 0"},
			{display: "PanFlute", value: "76 0 0"},
			{display: "Bottle", value: "77 0 0"},
			{display: "Shakhchi", value: "78 0 0"},
			{display: "Whistle", value: "79 0 0"},
			{display: "Ocarina", value: "80 0 0"},
			{display: "SquareLd", value: "81 0 0"},
			{display: "Square 2", value: "81 0 6"},
			{display: "LMSquare", value: "81 0 8"},
			{display: "Hollow", value: "81 0 18"},
			{display: "Shmoog", value: "81 0 19"},
			{display: "Mellow", value: "81 0 64"},
			{display: "SoloSine", value: "81 0 65"},
			{display: "SineLead", value: "81 0 66"},
			{display: "Saw.Lead", value: "82 0 0"},
			{display: "Saw 2", value: "82 0 6"},
			{display: "ThickSaw", value: "82 0 8"},
			{display: "DynaSaw", value: "82 0 18"},
			{display: "DigiSaw", value: "82 0 19"},
			{display: "Big Lead", value: "82 0 20"},
			{display: "HeavySyn", value: "82 0 24"},
			{display: "WaspySyn", value: "82 0 25"},
			{display: "PulseSaw", value: "82 0 40"},
			{display: "Dr. Lead", value: "82 0 41"},
			{display: "VeloLead", value: "82 0 45"},
			{display: "Seq Ana", value: "82 0 96"},
			{display: "CaliopLd", value: "83 0 0"},
			{display: "Pure Pad", value: "83 0 65"},
			{display: "Chiff Ld", value: "84 0 0"},
			{display: "Rubby", value: "84 0 64"},
			{display: "CharanLd", value: "85 0 0"},
			{display: "DistLead", value: "85 0 64"},
			{display: "WireLead", value: "85 0 65"},
			{display: "Voice Ld", value: "86 0 0"},
			{display: "SynthAah", value: "86 0 0"},
			{display: "VoxLead", value: "86 0 64"},
			{display: "Fifth Ld", value: "87 0 0"},
			{display: "Big Five", value: "87 0 35"},
			{display: "Bass &Ld", value: "88 0 0"},
			{display: "Big&Low", value: "88 0 16"},
			{display: "Fat&Prky", value: "88 0 64"},
			{display: "SoftWurl", value: "88 0 65"},
			{display: "NewAgePd", value: "89 0 0"},
			{display: "Fantasy2", value: "89 0 64"},
			{display: "Warm Pad", value: "90 0 0"},
			{display: "ThickPad", value: "90 0 16"},
			{display: "Soft Pad", value: "90 0 17"},
			{display: "SinePad", value: "90 0 18"},
			{display: "Horn Pad", value: "90 0 64"},
			{display: "RotarStr", value: "90 0 65"},
			{display: "PolySyPd", value: "91 0 0"},
			{display: "PolyPd80", value: "91 0 64"},
			{display: "ClickPad", value: "91 0 65"},
			{display: "Ana Pad", value: "91 0 66"},
			{display: "SquarPad", value: "91 0 67"},
			{display: "ChoirPad", value: "92 0 0"},
			{display: "Heaven2", value: "91 0 64"},
			{display: "Itopia", value: "91 0 66"},
			{display: "CC Pad", value: "91 0 67"},
			{display: "BowedPad", value: "93 0 0"},
			{display: "Glacier", value: "93 0 64"},
			{display: "GlassPad", value: "93 0 65"},
			{display: "MetalPad", value: "94 0 0"},
			{display: "Tine Pad", value: "94 0 64"},
			{display: "Pan Pad", value: "94 0 65"},
			{display: "Halo Pad", value: "95 0 0"},
			{display: "SweepPad", value: "96 0 0"},
			{display: "Shwimmer", value: "96 0 20"},
			{display: "Converge", value: "96 0 27"},
			{display: "PolarPad", value: "96 0 64"},
			{display: "Celstial", value: "96 0 66"},
			{display: "Rain", value: "97 0 0"},
			{display: "ClaviPad", value: "97 0 45"},
			{display: "HrmoRain", value: "97 0 64"},
			{display: "AfrcnWnd", value: "97 0 65"},
			{display: "Caribean", value: "97 0 66"},
			{display: "SoundTrk", value: "98 0 0"},
			{display: "Prologue", value: "98 0 27"},
			{display: "Ancestrl", value: "98 0 64"},
			{display: "Rave", value: "98 0 65"},
			{display: "Crystal", value: "99 0 0"},
			{display: "SynDrCmp", value: "99 0 12"},
			{display: "Popcorn", value: "99 0 14"},
			{display: "TinyBell", value: "99 0 18"},
			{display: "RndGlock", value: "99 0 35"},
			{display: "GlockChi", value: "99 0 40"},
			{display: "ClearBel", value: "99 0 41"},
			{display: "ChorBell", value: "99 0 42"},
			{display: "SynMalet", value: "99 0 64"},
			{display: "SftCryst", value: "99 0 65"},
			{display: "LoudGlok", value: "99 0 66"},
			{display: "XmasBell", value: "99 0 67"},
			{display: "VibeBell", value: "99 0 68"},
			{display: "DigiBell", value: "99 0 69"},
			{display: "AirBells", value: "99 0 70"},
			{display: "BellHarp", value: "99 0 71"},
			{display: "Gamelmba", value: "99 0 72"},
			{display: "Atmosphr", value: "100 0 0"},
			{display: "WarmAtms", value: "100 0 18"},
			{display: "HollwRls", value: "100 0 19"},
			{display: "NylonEP", value: "100 0 40"},
			{display: "NylnHarp", value: "100 0 64"},
			{display: "Harp Vox", value: "100 0 65"},
			{display: "AtmosPad", value: "100 0 66"},
			{display: "Planet", value: "100 0 67"},
			{display: "Bright", value: "101 0 0"},
			{display: "FantaBel", value: "101 0 64"},
			{display: "Smokey", value: "101 0 96"},
			{display: "Goblins", value: "102 0 0"},
			{display: "GobSyn", value: "102 0 64"},
			{display: "50sSciFi", value: "102 0 65"},
			{display: "Ring Pad", value: "102 0 66"},
			{display: "Ritual", value: "102 0 67"},
			{display: "ToHeaven", value: "102 0 68"},
			{display: "Night", value: "102 0 70"},
			{display: "Glisten", value: "102 0 71"},
			{display: "BelChoir", value: "102 0 96"},
			{display: "Echoes", value: "103 0 0"},
			{display: "EchoPad2", value: "103 0 8"},
			{display: "Echo Pan", value: "103 0 14"},
			{display: "EchoBell", value: "103 0 64"},
			{display: "Big Pan", value: "103 0 65"},
			{display: "SynPiano", value: "103 0 66"},
			{display: "Creation", value: "103 0 67"},
			{display: "Stardust", value: "103 0 68"},
			{display: "Reso Pan", value: "103 0 69"},
			{display: "Sci-Fi", value: "104 0 0"},
			{display: "Starz", value: "104 0 64"},
			{display: "Sitar", value: "105 0 0"},
			{display: "DetSitar", value: "105 0 32"},
			{display: "Sitar 2", value: "105 0 35"},
			{display: "Tambra", value: "105 0 96"},
			{display: "Tamboura", value: "105 0 97"},
			{display: "Banjo", value: "106 0 0"},
			{display: "MuteBnjo", value: "106 0 28"},
			{display: "Rabab", value: "106 0 96"},
			{display: "Gopichnt", value: "106 0 97"},
			{display: "Oud", value: "106 0 98"},
			{display: "Shamisen", value: "107 0 0"},
			{display: "Koto", value: "108 0 0"},
			{display: "T. Koto", value: "108 0 96"},
			{display: "Kanoon", value: "108 0 97"},
			{display: "Kalimba", value: "109 0 0"},
			{display: "Bagpipe", value: "110 0 0"},
			{display: "Fiddle", value: "111 0 0"},
			{display: "Shanai", value: "112 0 0"},
			{display: "Shanai2", value: "112 0 64"},
			{display: "Pungi", value: "112 0 96"},
			{display: "Hichriki", value: "112 0 97"},
			{display: "TnklBell", value: "113 0 0"},
			{display: "Bonang", value: "113 0 96"},
			{display: "Gender", value: "113 0 97"},
			{display: "Gamelan", value: "113 0 98"},
			{display: "S.Gamlan", value: "113 0 99"},
			{display: "Rama Cym", value: "113 0 100"},
			{display: "AsianBel", value: "113 0 101"},
			{display: "Agogo", value: "114 0 0"},
			{display: "SteelDrm", value: "115 0 0"},
			{display: "GlasPerc", value: "115 0 97"},
			{display: "ThaiBel", value: "115 0 98"},
			{display: "WoodBlok", value: "116 0 0"},
			{display: "Castanet", value: "116 0 96"},
			{display: "TaikoDrm", value: "117 0 0"},
			{display: "Gr.Cassa", value: "117 0 96"},
			{display: "MelodTom", value: "118 0 0"},
			{display: "Mel Tom2", value: "118 0 64"},
			{display: "Real Tom", value: "118 0 65"},
			{display: "Rock Tom", value: "118 0 66"},
			{display: "Syn.Drum", value: "119 0 0"},
			{display: "Ana Tom", value: "119 0 64"},
			{display: "ElecPerc", value: "119 0 65"},
			{display: "RevCymbl", value: "120 0 0"},
			{display: "FretNoiz", value: "121 0 0"},
			{display: "BrthNoiz", value: "122 0 0"},
			{display: "Seashore", value: "123 0 0"},
			{display: "Tweet", value: "124 0 0"},
			{display: "Telphone", value: "125 0 0"},
			{display: "Helicptr", value: "126 0 0"},
			{display: "Applause", value: "127 0 0"},
			{display: "Gunshot", value: "128 0 0"},
		},
	}

	ccTargets = [][]*tabTarget{
		// GM
		{
			{display: "Modulation", value: "1"},
			{display: "Volume", value: "7"},
			{display: "Pan", value: "10"},
			{display: "Expression", value: "11"},
			{display: "Sustain", value: "64"},
			{display: "Reset All Controllers", value: "121"},
			{display: "All Notes Off", value: "123"},
		},
		// GS
		{
			{display: "Bank Select MSB", value: "0"},
			{display: "Modulation", value: "1"},
			{display: "Portamento Time", value: "5"},
			{display: "Volume", value: "7"},
			{display: "Pan", value: "10"},
			{display: "Expression", value: "11"},
			{display: "Bank Select LSB", value: "32"},
			{display: "Sustain", value: "64"},
			{display: "Portamento", value: "65"},
			{display: "Sostenuto", value: "66"},
			{display: "Soft Pedal", value: "67"},
			{display: "Portamento Control", value: "84"},
			{display: "Reverb Send Level", value: "91"},
			{display: "Chorus Send Level", value: "93"},
			{display: "Delay Send Level", value: "94"},
			{display: "NRPN MSB", value: "98"},
			{display: "NRPN LSB", value: "99"},
			{display: "All Sounds Off", value: "120"},
			{display: "Reset All Controllers", value: "121"},
			{display: "All Notes Off", value: "123"},
		},
		// XG
		{
			{display: "Bank Select MSB", value: "0"},
			{display: "Modulation", value: "1"},
			{display: "Portamento Time", value: "5"},
			{display: "Data Entry MSB", value: "6"},
			{display: "Volume", value: "7"},
			{display: "Pan", value: "10"},
			{display: "Expression", value: "11"},
			{display: "Bank Select LSB", value: "32"},
			{display: "Data Entry LSB", value: "38"},
			{display: "Sustain", value: "64"},
			{display: "Portamento", value: "65"},
			{display: "Sostenuto", value: "66"},
			{display: "Soft Pedal", value: "67"},
			{display: "Harmonic Content", value: "71"},
			{display: "Release Time", value: "72"},
			{display: "Attack Time", value: "73"},
			{display: "Brightness", value: "74"},
			{display: "Portamento Control", value: "84"},
			{display: "Reverb Send Level", value: "91"},
			{display: "Chorus Send Level", value: "93"},
			{display: "Variation Send Level", value: "94"},
			{display: "Data Increment", value: "96"},
			{display: "Data Decrement", value: "97"},
			{display: "NRPN MSB", value: "98"},
			{display: "NRPN LSB", value: "99"},
			{display: "RPN MSB", value: "100"},
			{display: "RPN LSB", value: "101"},
			{display: "All Sounds Off", value: "120"},
			{display: "Reset All Controllers", value: "121"},
			{display: "All Notes Off", value: "123"},
			{display: "OMNI Off", value: "124"},
			{display: "OMNI On", value: "125"},
			{display: "MONO", value: "126"},
			{display: "POLY", value: "127"},
		},
	}

	metaEvents = []*tabTarget{
		{display: "Text", value: "1"},
		{display: "Copyright", value: "2"},
		{display: "Track Name", value: "3"},
		{display: "Instrument Name", value: "4"},
		{display: "Lyric", value: "5"},
		{display: "Marker", value: "6"},
		{display: "Cue Point", value: "7"},
		{display: "Program Name", value: "8"},
		{display: "Device Name", value: "9"},
	}
)
