package daemon

import (
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
type promptTitleTracker struct {
	buf      []byte
	escState uint8
	csiBuf   []byte
	inPaste  bool
}

func (t *promptTitleTracker) reset() {
	t.buf = t.buf[:0]
	t.escState = escNone
	t.csiBuf = t.csiBuf[:0]
	t.inPaste = false
}

func (t *promptTitleTracker) feed(p []byte) (promptEvent, string) {
	if len(p) == 1 && p[0] == 0x1b && t.escState == escNone {
		t.reset()
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
				continue
			case ']':
				t.escState = escOSC
				continue
			case 'O':
				t.escState = escSS3
				continue
			case '\r', '\n':
				t.escState = escNone
				t.appendByte(' ')
				if b == '\r' && i+1 < len(p) && p[i+1] == '\n' {
					i++
				}
				continue
			case 0x08, 0x7f:
				t.escState = escNone
				t.backspaceWord()
				continue
			default:
				t.escState = escNone
				t.buf = t.buf[:0]
				if b == 0x1b {
					t.escState = escSaw
					continue
				}
				// Re-process non-sequence byte as normal input after clearing.
			}
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
				t.appendByte(' ')
				continue
			}
			if len(t.buf) > 0 && t.buf[len(t.buf)-1] == '\\' {
				t.buf[len(t.buf)-1] = ' '
				continue
			}
			if ev, title := t.submit(); ev != promptNone {
				return ev, title
			}
		case 0x08, 0x7f:
			t.backspaceRune()
		case 0x17:
			t.backspaceWord()
		case 0x03, 0x15:
			t.buf = t.buf[:0]
		default:
			if b >= 0x20 {
				t.appendByte(b)
			}
		}
	}
	return promptNone, ""
}

func (t *promptTitleTracker) handleCSI(seq string, final byte) (promptEvent, string) {
	switch final {
	case '~':
		switch seq {
		case "200":
			t.inPaste = true
		case "201":
			t.inPaste = false
		}
	case 'u':
		key, mod, hasMod := strings.Cut(seq, ";")
		key, _, _ = strings.Cut(key, ":")
		mod, _, _ = strings.Cut(mod, ":")
		switch key {
		case "13":
			if t.inPaste || (hasMod && mod != "" && mod != "1") {
				t.appendByte(' ')
				return promptNone, ""
			}
			return t.submit()
		case "127":
			t.backspaceRune()
		case "27":
			if !hasMod || mod == "" || mod == "1" {
				t.reset()
			}
		}
	}
	return promptNone, ""
}

func (t *promptTitleTracker) submit() (promptEvent, string) {
	raw := string(t.buf)
	t.buf = t.buf[:0]
	if isPromptResetCommand(raw) {
		return promptReset, ""
	}
	if title := cleanPromptTitle(raw); title != "" {
		return promptTitle, title
	}
	return promptNone, ""
}

func (t *promptTitleTracker) appendByte(b byte) {
	if len(t.buf) < maxPromptBuf {
		t.buf = append(t.buf, b)
	}
}

func (t *promptTitleTracker) backspaceRune() {
	if len(t.buf) == 0 {
		return
	}
	_, size := utf8.DecodeLastRune(t.buf)
	if size <= 0 {
		size = 1
	}
	t.buf = t.buf[:len(t.buf)-size]
}

func (t *promptTitleTracker) backspaceWord() {
	for len(t.buf) > 0 && t.buf[len(t.buf)-1] == ' ' {
		t.buf = t.buf[:len(t.buf)-1]
	}
	for len(t.buf) > 0 && t.buf[len(t.buf)-1] != ' ' {
		t.backspaceRune()
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
