package daemon

import (
	"strings"
	"testing"
)

func TestPromptTitleTracker(t *testing.T) {
	t.Run("english and chinese first prompt", func(t *testing.T) {
		var tr promptTitleTracker
		if ev, _ := tr.feed([]byte("hello ")); ev != promptNone {
			t.Fatalf("partial input returned %v", ev)
		}
		ev, title := tr.feed([]byte("world\r"))
		if ev != promptTitle || title != "hello world" {
			t.Fatalf("got (%v, %q), want (promptTitle, %q)", ev, title, "hello world")
		}

		ev, title = tr.feed([]byte("那你帮我出一版新的视觉，并帮我做个侧边栏\n"))
		if ev != promptTitle || title != "那你帮我出一版新的视觉，并帮我做个侧边栏" {
			t.Fatalf("chinese got (%v, %q)", ev, title)
		}
	})

	t.Run("editing keys backspace ctrl-w ctrl-u esc", func(t *testing.T) {
		var tr promptTitleTracker
		tr.feed([]byte("你好a\x7f世界"))
		ev, title := tr.feed([]byte("\r"))
		if ev != promptTitle || title != "你好世界" {
			t.Fatalf("after UTF-8 backspace: (%v, %q)", ev, title)
		}

		tr.reset()
		tr.feed([]byte("fix old bug\x17\x17new feature"))
		ev, title = tr.feed([]byte("\r"))
		if ev != promptTitle || title != "fix new feature" {
			t.Fatalf("after Ctrl+W: (%v, %q)", ev, title)
		}

		tr.reset()
		ev, title = tr.feed([]byte("discard me\x15keep this\r"))
		if ev != promptTitle || title != "keep this" {
			t.Fatalf("after Ctrl+U: (%v, %q)", ev, title)
		}

		tr.reset()
		tr.feed([]byte("stale draft"))
		tr.feed([]byte{0x1b})
		ev, title = tr.feed([]byte("fresh draft\r"))
		if ev != promptTitle || title != "fresh draft" {
			t.Fatalf("after ESC: (%v, %q)", ev, title)
		}
	})

	t.Run("bracketed paste and modified newlines", func(t *testing.T) {
		var tr promptTitleTracker
		pasted := "\x1b[200~first line\nsecond line\r\nthird line\x1b[201~"
		if ev, _ := tr.feed([]byte(pasted)); ev != promptNone {
			t.Fatalf("paste alone should not submit: %v", ev)
		}
		ev, title := tr.feed([]byte("\r"))
		if ev != promptTitle || title != "first line second line third line" {
			t.Fatalf("paste submit got (%v, %q)", ev, title)
		}

		tr.reset()
		if ev, _ := tr.feed([]byte("line one\x1b[13;2uline two\x1b\rline three")); ev != promptNone {
			t.Fatalf("modified enter should not submit: %v", ev)
		}
		ev, title = tr.feed([]byte("\x1b[13u"))
		if ev != promptTitle || title != "line one line two line three" {
			t.Fatalf("kitty enter got (%v, %q)", ev, title)
		}

		tr.reset()
		if ev, _ := tr.feed([]byte("part1\\\r\npart2")); ev != promptNone {
			t.Fatalf("backslash continuation should not submit: %v", ev)
		}
		ev, title = tr.feed([]byte("\r"))
		if ev != promptTitle || title != "part1 part2" {
			t.Fatalf("backslash continuation submit got (%v, %q)", ev, title)
		}
	})

	t.Run("ignores focus mouse and filters noise", func(t *testing.T) {
		var tr promptTitleTracker
		for _, in := range []string{
			"\x1b[I\x1b[O\x1b[<0;12;8M\x1b[A\x1b[B\r",
			"/model\r",
			"/help\r",
			"y\r",
			"1\r",
			"12345\r",
			"---\r",
			"/Users/me/project\r",
			"~/workspace/kero\r",
			"   \r",
		} {
			if ev, title := tr.feed([]byte(in)); ev != promptNone {
				t.Errorf("input %q produced (%v, %q), want promptNone", in, ev, title)
			}
		}
	})

	t.Run("reset commands and truncation", func(t *testing.T) {
		var tr promptTitleTracker
		for _, cmd := range []string{"/clear\r", "/NEW\r", "  /reset  \r"} {
			if ev, _ := tr.feed([]byte(cmd)); ev != promptReset {
				t.Errorf("command %q gave %v, want promptReset", cmd, ev)
			}
		}

		long := strings.Repeat("a中", 60)
		ev, title := tr.feed([]byte(long + "\r"))
		if ev != promptTitle || len([]rune(title)) != maxPromptRunes {
			t.Fatalf("truncated title len=%d (%v): %q", len([]rune(title)), ev, title)
		}
	})
}
