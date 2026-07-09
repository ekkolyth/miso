package tui

import tea "charm.land/bubbletea/v2"

// encode a key event as the bytes a child expects on stdin. printable chars
// (incl. shifted) arrive via Text; special keys map to control sequences; nil
// for keys with no byte form
func keyToBytes(k tea.Key) []byte {
	if k.Text != "" {
		return []byte(k.Text)
	}
	switch k.Code {
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
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	}
	if k.Mod&tea.ModCtrl != 0 && k.Code >= 'a' && k.Code <= 'z' {
		return []byte{byte(k.Code-'a') + 1}
	}
	return nil
}
