package daemon

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"kero/internal/proto"
)

const (
	maxPromptBuf   = 512
	maxPromptRunes = 80
)

type promptEvent uint8

const (
	promptNone promptEvent = iota
	promptTitle
	promptReset
)

const (
	escNone uint8 = iota
	escSaw
	escCSI
	escOSC
	escOSCEsc
	escSS3
)

// promptTitleTracker extracts a cleaned single-line title from the first
// non-command user prompt submitted to an interactive coding agent in a pane.
// It tracks single-line cursor movement and aborts inference when history
// navigation or completion keys make the reconstructed buffer unreliable.
type promptTitleTracker struct {
	runes      []rune
	pos        int
	utf8Buf    []byte
	escState   uint8
	csiBuf     []byte
	inPaste    bool
	unreliable bool
}

func (t *promptTitleTracker) reset() {
	t.runes = t.runes[:0]
	t.pos = 0
	t.utf8Buf = t.utf8Buf[:0]
	t.escState = escNone
	t.csiBuf = t.csiBuf[:0]
	t.inPaste = false
	t.unreliable = false
}

func (t *promptTitleTracker) clearLine() {
	t.runes = t.runes[:0]
	t.pos = 0
	t.utf8Buf = t.utf8Buf[:0]
	t.unreliable = false
}

func (t *promptTitleTracker) feed(p []byte) (promptEvent, string) {
	if len(p) == 1 && p[0] == 0x1b && t.escState == escNone {
		t.clearLine()
		return promptNone, ""
	}
	for i := 0; i < len(p); i++ {
		b := p[i]
		switch t.escState {
		case escSaw:
			switch b {
			case '[':
				t.escState = escCSI
				t.csiBuf = t.csiBuf[:0]
			case ']':
				t.escState = escOSC
			case 'O':
				t.escState = escSS3
			case '\r', '\n':
				t.escState = escNone
				t.insertRune(' ')
				if b == '\r' && i+1 < len(p) && p[i+1] == '\n' {
					i++
				}
			case 0x08, 0x7f:
				t.escState = escNone
				t.backspaceWord()
			case 0x1b:
				t.clearLine()
			default:
				// Alt+key sequences (such as Alt+b/f/d) mutate the line in
				// readline/TUI-dependent ways we don't replicate.
				t.escState = escNone
				t.unreliable = true
			}
			continue
		case escCSI:
			if b >= 0x40 && b <= 0x7e {
				seq := string(t.csiBuf)
				t.escState = escNone
				t.csiBuf = t.csiBuf[:0]
				if ev, title := t.handleCSI(seq, b); ev != promptNone {
					return ev, title
				}
			} else if len(t.csiBuf) < 64 {
				t.csiBuf = append(t.csiBuf, b)
			}
			continue
		case escOSC:
			if b == 0x07 || b == 0x9c {
				t.escState = escNone
			} else if b == 0x1b {
				t.escState = escOSCEsc
			}
			continue
		case escOSCEsc:
			if b == '\\' {
				t.escState = escNone
			} else if b != 0x1b {
				t.escState = escOSC
			}
			continue
		case escSS3:
			t.escState = escNone
			switch b {
			case 'D':
				t.moveLeft()
			case 'C':
				t.moveRight()
			case 'H':
				t.moveHome()
			case 'F':
				t.moveEnd()
			case 'A', 'B':
				t.unreliable = true
			}
			continue
		}

		switch b {
		case 0x1b:
			t.escState = escSaw
		case '\r', '\n':
			if b == '\r' && i+1 < len(p) && p[i+1] == '\n' {
				i++
			}
			if t.inPaste {
				t.insertRune(' ')
				continue
			}
			if len(t.runes) > 0 && t.pos == len(t.runes) && t.runes[len(t.runes)-1] == '\\' {
				t.utf8Buf = t.utf8Buf[:0]
				t.runes[len(t.runes)-1] = ' '
				continue
			}
			if ev, title := t.submit(); ev != promptNone {
				return ev, title
			}
		case '\t':
			if t.inPaste {
				t.insertRune(' ')
			} else {
				t.unreliable = true
			}
		case 0x01: // Ctrl+A
			t.moveHome()
		case 0x02: // Ctrl+B
			t.moveLeft()
		case 0x03: // Ctrl+C
			t.clearLine()
		case 0x04: // Ctrl+D
			t.deleteRune()
		case 0x05: // Ctrl+E
			t.moveEnd()
		case 0x06: // Ctrl+F
			t.moveRight()
		case 0x08, 0x7f: // BS / DEL
			t.backspaceRune()
		case 0x0b: // Ctrl+K
			t.killToEnd()
		case 0x15: // Ctrl+U
			t.killToStart()
		case 0x17: // Ctrl+W
			t.backspaceWord()
		default:
			if b >= 0x20 {
				t.appendByte(b)
			}
		}
	}
	return promptNone, ""
}

func csiUnmodified(seq string) bool {
	return seq == "" || seq == "1" || seq == "1;1"
}

func tildeSplit(seq string) (key string, unmodified bool) {
	key, mod, hasMod := strings.Cut(seq, ";")
	return key, !hasMod || mod == "" || mod == "1"
}

func (t *promptTitleTracker) handleCSI(seq string, final byte) (promptEvent, string) {
	switch final {
	case 'D':
		if csiUnmodified(seq) {
			t.moveLeft()
		} else {
			t.unreliable = true
		}
	case 'C':
		if csiUnmodified(seq) {
			t.moveRight()
		} else {
			t.unreliable = true
		}
	case 'H':
		if csiUnmodified(seq) {
			t.moveHome()
		} else {
			t.unreliable = true
		}
	case 'F':
		if csiUnmodified(seq) {
			t.moveEnd()
		} else {
			t.unreliable = true
		}
	case 'A', 'B':
		t.unreliable = true
	case '~':
		key, noMod := tildeSplit(seq)
		switch key {
		case "200":
			t.inPaste = true
		case "201":
			t.inPaste = false
		case "1", "7":
			if noMod {
				t.moveHome()
			} else {
				t.unreliable = true
			}
		case "4", "8":
			if noMod {
				t.moveEnd()
			} else {
				t.unreliable = true
			}
		case "3":
			if noMod {
				t.deleteRune()
			} else {
				t.unreliable = true
			}
		}
	case 'u':
		key, mod, hasMod := strings.Cut(seq, ";")
		key, _, _ = strings.Cut(key, ":")
		mod, _, _ = strings.Cut(mod, ":")
		noMod := !hasMod || mod == "" || mod == "1"
		switch key {
		case "13":
			if t.inPaste || !noMod {
				t.insertRune(' ')
				return promptNone, ""
			}
			return t.submit()
		case "127":
			if noMod {
				t.backspaceRune()
			} else {
				t.unreliable = true
			}
		case "27":
			if noMod {
				t.clearLine()
			}
		case "9":
			t.unreliable = true
		}
	}
	return promptNone, ""
}

func (t *promptTitleTracker) submit() (promptEvent, string) {
	raw := string(t.runes)
	unreliable := t.unreliable
	t.runes = t.runes[:0]
	t.pos = 0
	t.utf8Buf = t.utf8Buf[:0]
	t.unreliable = false
	if unreliable {
		return promptNone, ""
	}
	if isPromptResetCommand(raw) {
		return promptReset, ""
	}
	if title := cleanPromptTitle(raw); title != "" {
		return promptTitle, title
	}
	return promptNone, ""
}

func (t *promptTitleTracker) appendByte(b byte) {
	if b < 0x80 {
		t.utf8Buf = t.utf8Buf[:0]
		t.insertRune(rune(b))
		return
	}
	if b >= 0xc0 {
		t.utf8Buf = t.utf8Buf[:0]
	}
	if len(t.utf8Buf) >= 4 {
		t.utf8Buf = t.utf8Buf[:0]
	}
	t.utf8Buf = append(t.utf8Buf, b)
	if utf8.FullRune(t.utf8Buf) {
		r, _ := utf8.DecodeRune(t.utf8Buf)
		t.utf8Buf = t.utf8Buf[:0]
		if r != utf8.RuneError {
			t.insertRune(r)
		}
	}
}

func (t *promptTitleTracker) insertRune(r rune) {
	t.utf8Buf = t.utf8Buf[:0]
	if len(t.runes) >= maxPromptBuf {
		return
	}
	t.runes = slices.Insert(t.runes, t.pos, r)
	t.pos++
}

func (t *promptTitleTracker) moveLeft() {
	t.utf8Buf = t.utf8Buf[:0]
	if t.pos > 0 {
		t.pos--
	}
}

func (t *promptTitleTracker) moveRight() {
	t.utf8Buf = t.utf8Buf[:0]
	if t.pos < len(t.runes) {
		t.pos++
	}
}

func (t *promptTitleTracker) moveHome() {
	t.utf8Buf = t.utf8Buf[:0]
	t.pos = 0
}

func (t *promptTitleTracker) moveEnd() {
	t.utf8Buf = t.utf8Buf[:0]
	t.pos = len(t.runes)
}

func (t *promptTitleTracker) backspaceRune() {
	t.utf8Buf = t.utf8Buf[:0]
	if t.pos > 0 {
		t.runes = slices.Delete(t.runes, t.pos-1, t.pos)
		t.pos--
	}
}

func (t *promptTitleTracker) deleteRune() {
	t.utf8Buf = t.utf8Buf[:0]
	if t.pos < len(t.runes) {
		t.runes = slices.Delete(t.runes, t.pos, t.pos+1)
	}
}

func (t *promptTitleTracker) killToEnd() {
	t.utf8Buf = t.utf8Buf[:0]
	t.runes = t.runes[:t.pos]
}

func (t *promptTitleTracker) killToStart() {
	t.utf8Buf = t.utf8Buf[:0]
	if t.pos > 0 {
		t.runes = slices.Delete(t.runes, 0, t.pos)
		t.pos = 0
	}
	if len(t.runes) == 0 {
		t.unreliable = false
	}
}

func (t *promptTitleTracker) backspaceWord() {
	t.utf8Buf = t.utf8Buf[:0]
	end := t.pos
	for t.pos > 0 && t.runes[t.pos-1] == ' ' {
		t.pos--
	}
	for t.pos > 0 && t.runes[t.pos-1] != ' ' {
		t.pos--
	}
	if t.pos < end {
		t.runes = slices.Delete(t.runes, t.pos, end)
	}
}

func isPromptResetCommand(raw string) bool {
	fields := strings.Fields(strings.ToValidUTF8(raw, ""))
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "/clear", "/new", "/reset":
		return true
	}
	return false
}

func cleanPromptTitle(raw string) string {
	s := strings.Join(strings.Fields(strings.ToValidUTF8(raw, "")), " ")
	for {
		r, size := utf8.DecodeRuneInString(s)
		if size == 0 || !strings.ContainsRune(">»•·❯", r) {
			break
		}
		s = strings.TrimSpace(s[size:])
	}
	if s == "" || strings.HasPrefix(s, "/") {
		return ""
	}
	if proto.LooksLikePath(s) {
		return ""
	}
	if !strings.Contains(s, " ") && (strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../")) {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= 1 {
		return ""
	}
	hasLetter := false
	for _, r := range runes {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return ""
	}
	if len(runes) > maxPromptRunes {
		return string(runes[:maxPromptRunes])
	}
	return s
}
