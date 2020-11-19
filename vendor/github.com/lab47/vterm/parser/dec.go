package parser

type DecModeCommand int

const (
	DECCKM        DecModeCommand = 1  // Application Cursor Keys (DECCKM)
	DECANM        DecModeCommand = 2  // Designate USASCII for character sets G0-G3 (DECANM), and set VT100 mode.
	DECCOLM       DecModeCommand = 3  // 132 Column Mode (DECCOLM)
	DECSCLM       DecModeCommand = 4  // Smooth (Slow) Scroll (DECSCLM)
	DECSCNM       DecModeCommand = 5  // Reverse Video (DECSCNM)
	DECOM         DecModeCommand = 6  // Origin Mode (DECOM)
	DECAWM        DecModeCommand = 7  // Wraparound Mode (DECAWM)
	DECARM        DecModeCommand = 8  // Auto-repeat Keys (DECARM)
	DECSNDM       DecModeCommand = 9  // Send Mouse X & Y on button press. See the section Mouse Tracking.
	DECTOOL       DecModeCommand = 10 // Show toolbar (rxvt)
	DECBLINK      DecModeCommand = 12 // Start Blinking Cursor (att610)
	DECPFF        DecModeCommand = 18 // Print form feed (DECPFF)
	DECPEX        DecModeCommand = 19 // Set print extent to full screen (DECPEX)
	DECTCEM       DecModeCommand = 25 // Show Cursor (DECTCEM)
	DECSHOWSCROLL DecModeCommand = 30 // Show scrollbar (rxvt).
	DECFONTSHIFT  DecModeCommand = 35 // Enable font-shifting functions (rxvt).
	DECTEK        DecModeCommand = 38 // Enter Tektronix Mode (DECTEK)
	DEC132        DecModeCommand = 40 // Allow 80 → 132 Mode
	DECMORE       DecModeCommand = 41 // more(1) fix (see curses resource)
	DECNRCM       DecModeCommand = 42 // Enable Nation Replacement Character sets (DECNRCM)
	DECMBELL      DecModeCommand = 44 // Turn On Margin Bell
	DECRWRAP      DecModeCommand = 45 // Reverse-wraparound Mode
	DECLOG        DecModeCommand = 46 // Start Logging (normally disabled by a compile-time option)
	DECALT        DecModeCommand = 47 // Use Alternate Screen Buffer (unless disabled by the titeInhibit resource)
	DECNKM        DecModeCommand = 66 // Application keypad (DECNKM)
	DECBKM        DecModeCommand = 67 // Backarrow key sends backspace (DECBKM)
	DECVSSM       DecModeCommand = 69
	DECMCLICK     DecModeCommand = 1000 // Send Mouse X & Y on button press and release. See the section Mouse Tracking.
	DECMDRAG      DecModeCommand = 1002 // Use Cell Motion Mouse Tracking.
	DECMMOVE      DecModeCommand = 1003 // Use All Motion Mouse Tracking.
	DECRPTFOC     DecModeCommand = 1004
	DECMPROT1     DecModeCommand = 1005
	DECMPROT2     DecModeCommand = 1006
	DECMPROT3     DecModeCommand = 1015
	DECALTSCRN    DecModeCommand = 1047 // Use Alternate Screen Buffer (unless disabled by the titeInhibit resource)
	DECSAVECUR    DecModeCommand = 1048 // Save cursor as in DECSC (unless disabled by the titeInhibit resource)
	DECALTSCRN2   DecModeCommand = 1049 // Save cursor as in DECSC and use Alternate Screen Buffer, clearing it first (unless disabled by the titeInhibit resource). This combines the effects of the 1 0 4 7 and 1 0 4 8 modes. Use this with terminfo-based applications rather than the 4 7 mode.
	DECBRACKET    DecModeCommand = 2004 // Set bracketed paste mode.
)

//go:generate stringer -type=DecModeCommand
