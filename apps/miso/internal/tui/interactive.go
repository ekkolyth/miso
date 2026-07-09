package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// encode a key event as the bytes a child expects on stdin. printable chars
// (incl. shifted) arrive via Text; special keys map to control/CSI sequences;
// nil for keys with no byte form
func keyToBytes(k tea.Key) []byte {
	// ctrl+space -> NUL (checked before the Text/space branches)
	if k.Code == tea.KeySpace && k.Mod&tea.ModCtrl != 0 {
		return []byte{0}
	}
	// alt+<printable> -> ESC-prefixed
	if k.Mod&tea.ModAlt != 0 && k.Text != "" {
		return append([]byte{0x1b}, []byte(k.Text)...)
	}
	if k.Text != "" {
		return []byte(k.Text)
	}
	switch k.Code {
	case tea.KeyUp, tea.KeyDown, tea.KeyRight, tea.KeyLeft:
		var letter byte
		switch k.Code {
		case tea.KeyUp:
			letter = 'A'
		case tea.KeyDown:
			letter = 'B'
		case tea.KeyRight:
			letter = 'C'
		case tea.KeyLeft:
			letter = 'D'
		}
		if mod := csiMod(k.Mod); mod > 1 {
			return []byte(fmt.Sprintf("\x1b[1;%d%c", mod, letter))
		}
		return []byte{0x1b, '[', letter}
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeyBackspace:
		return []byte{0x7f}
	case tea.KeyEscape:
		return []byte{0x1b}
	case tea.KeySpace:
		return []byte{' '}
	case tea.KeyHome:
		return []byte("\x1b[H")
	case tea.KeyEnd:
		return []byte("\x1b[F")
	case tea.KeyInsert:
		return []byte("\x1b[2~")
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	case tea.KeyPgUp:
		return []byte("\x1b[5~")
	case tea.KeyPgDown:
		return []byte("\x1b[6~")
	case tea.KeyF1:
		return []byte("\x1bOP")
	case tea.KeyF2:
		return []byte("\x1bOQ")
	case tea.KeyF3:
		return []byte("\x1bOR")
	case tea.KeyF4:
		return []byte("\x1bOS")
	case tea.KeyF5:
		return []byte("\x1b[15~")
	case tea.KeyF6:
		return []byte("\x1b[17~")
	case tea.KeyF7:
		return []byte("\x1b[18~")
	case tea.KeyF8:
		return []byte("\x1b[19~")
	case tea.KeyF9:
		return []byte("\x1b[20~")
	case tea.KeyF10:
		return []byte("\x1b[21~")
	case tea.KeyF11:
		return []byte("\x1b[23~")
	case tea.KeyF12:
		return []byte("\x1b[24~")
	}
	// ctrl+<letter> -> control byte
	if k.Mod&tea.ModCtrl != 0 && k.Code >= 'a' && k.Code <= 'z' {
		return []byte{byte(k.Code-'a') + 1}
	}
	return nil
}

// xterm modifier parameter: 1 (none) + shift + alt + ctrl bits
func csiMod(mod tea.KeyMod) int {
	n := 1
	if mod&tea.ModShift != 0 {
		n += 1
	}
	if mod&tea.ModAlt != 0 {
		n += 2
	}
	if mod&tea.ModCtrl != 0 {
		n += 4
	}
	return n
}
