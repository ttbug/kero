package main

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/proto"
	"kero/locales"
)

func TestHandoffFrom(t *testing.T) {
	tests := []struct {
		name string
		s    *proto.Session
		want string
	}{
		{
			name: "nil session",
			s:    nil,
			want: "",
		},
		{
			name: "plain shell",
			s: &proto.Session{
				ID:    "s1",
				Focus: "p1",
				Panes: []proto.Pane{{ID: "p1", Shell: "zsh"}},
			},
			want: "",
		},
		{
			name: "single agent pane",
			s: &proto.Session{
				ID:    "s1",
				Focus: "p1",
				Panes: []proto.Pane{{
					ID:           "p1",
					Agent:        "codex",
					AgentSession: &proto.AgentSession{Agent: "codex", ID: "019abc"},
				}},
			},
			want: "codex",
		},
		{
			name: "fallback to session Pane when Panes empty",
			s: &proto.Session{
				ID:   "s1",
				Pane: proto.Pane{ID: "p1", Agent: "claude"},
			},
			want: "claude",
		},
		{
			name: "focused shell and unfocused agent pane",
			s: &proto.Session{
				ID:    "s1",
				Focus: "p1",
				Panes: []proto.Pane{
					{ID: "p1", Shell: "zsh"},
					{ID: "p2", Agent: "antigravity", AgentSession: &proto.AgentSession{Agent: "antigravity", ID: "agy-1"}},
				},
			},
			want: "antigravity",
		},
		{
			name: "focused agent wins over unfocused agent",
			s: &proto.Session{
				ID:    "s1",
				Focus: "p1",
				Panes: []proto.Pane{
					{ID: "p1", Agent: "claude"},
					{ID: "p2", Agent: "codex"},
				},
			},
			want: "claude",
		},
		{
			name: "mismatched AgentSession ignored when Agent set",
			s: &proto.Session{
				ID:    "s1",
				Focus: "p1",
				Panes: []proto.Pane{{
					ID:           "p1",
					Agent:        "codex",
					AgentSession: &proto.AgentSession{Agent: "claude", ID: "stale"},
				}},
			},
			want: "codex",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := handoffFrom(tc.s); got != tc.want {
				t.Errorf("handoffFrom() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSessionMenuHandoff(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	var l *proto.Layout
	s1 := proto.Session{
		ID:      "s1",
		Project: "p1",
		Name:    "shell-task",
		Focus:   "a",
		Panes:   []proto.Pane{{ID: "a", Shell: "zsh", Dir: "/tmp/kero", Status: proto.Idle, Progress: -1}},
	}
	s1.Pane = s1.Panes[0]

	s2 := proto.Session{
		ID:      "s2",
		Project: "p1",
		Name:    "codex-task",
		Focus:   "b",
		Panes: []proto.Pane{{
			ID:           "b",
			Shell:        "zsh",
			Dir:          "/tmp/kero",
			Agent:        "codex",
			AgentSession: &proto.AgentSession{Agent: "codex", ID: "019abc"},
			Cols:         120,
			Rows:         40,
			Status:       proto.Idle,
			Progress:     -1,
		}},
	}
	s2.Pane = s2.Panes[0]

	s3 := proto.Session{
		ID:      "s3",
		Project: "p1",
		Name:    "split-task",
		Focus:   "c1",
		Layout:  l.Split("c1", "c2", "right", 0),
		Panes: []proto.Pane{
			{ID: "c1", Shell: "zsh", Dir: "/tmp/kero", Cols: 100, Rows: 30, Status: proto.Idle, Progress: -1},
			{ID: "c2", Shell: "zsh", Dir: "/tmp/kero", Agent: "antigravity", AgentSession: &proto.AgentSession{Agent: "antigravity", ID: "conv-1"}, Status: proto.Idle, Progress: -1},
		},
	}
	s3.Pane = s3.Panes[1]

	a.st = proto.State{
		Projects: []proto.Project{{ID: "p1", Name: "kero", Dir: "/tmp/kero"}},
		Sessions: []proto.Session{s1, s2, s3},
	}
	a.selected = "s1"
	a.terms["a"], a.terms["b"], a.terms["c1"], a.terms["c2"] = &termEntry{}, &termEntry{}, &termEntry{}, &termEntry{}

	called := make(chan proto.HandoffParams, 4)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		if method == "handoff" {
			var p proto.HandoffParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			called <- p
			return proto.Session{ID: "new-1", Project: "p1", Panes: []proto.Pane{{ID: "np1"}}}, nil
		}
		return true, nil
	})
	a.local().cl = cl
	defer stop()

	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()

	// Plain shell session should not have a Hand Off To menu item.
	if err := tt.RightClick("shell-task"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Menu(); slices.Contains(got, locales.SessionHandOff()) {
		t.Fatalf("shell session menu unexpectedly had %q: %v", locales.SessionHandOff(), got)
	}
	tt.CloseMenu()
	tt.Frame()

	// Codex session: Hand Off To excludes Codex and sends Claude Code handoff RPC.
	if err := tt.RightClick("codex-task"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Menu(); !slices.Contains(got, locales.SessionHandOff()) {
		t.Fatalf("codex session menu missing %q: %v", locales.SessionHandOff(), got)
	}
	shot(t, tt, "sidebar-handoff-light")
	if err := tt.ChooseMenuItem(locales.SessionHandOff(), proto.AgentNames["codex"]); err == nil {
		t.Error("Codex should be excluded from its own handoff menu")
	}
	tt.CloseMenu()
	tt.Frame()

	if err := tt.RightClick("codex-task"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.SessionHandOff(), proto.AgentNames["claude"]); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	select {
	case got := <-called:
		want := proto.HandoffParams{ID: "s2", Agent: "claude", Cols: 120, Rows: 40}
		if got != want {
			t.Errorf("handoff RPC = %+v, want %+v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("kerod was not asked to hand off codex-task")
	}

	// Dark mode: split session with focused shell and unfocused Antigravity pane.
	tt.SetDark(true)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.RightClick("split-task"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Menu(); !slices.Contains(got, locales.SessionHandOff()) {
		t.Fatalf("split session menu missing %q: %v", locales.SessionHandOff(), got)
	}
	shot(t, tt, "sidebar-handoff-dark")
	if err := tt.ChooseMenuItem(locales.SessionHandOff(), proto.AgentNames["antigravity"]); err == nil {
		t.Error("Antigravity should be excluded from its own handoff menu")
	}
	tt.CloseMenu()
	tt.Frame()

	if err := tt.RightClick("split-task"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.SessionHandOff(), proto.AgentNames["codex"]); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	select {
	case got := <-called:
		want := proto.HandoffParams{ID: "s3", Agent: "codex", Cols: s3.Cols, Rows: s3.Rows}
		if got != want {
			t.Errorf("handoff RPC = %+v, want %+v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("kerod was not asked to hand off split-task")
	}

	// Extra frames after closing menus ensure no stale element issues.
	tt.CloseMenu()
	tt.Frame()
	tt.Frame()

	// Disconnected host shows toast rather than calling nil client.
	a.local().cl = nil
	a.pendingToast = ""
	a.handoff("s2", "claude")
	wantToast := locales.HostNotConnected(a.local().label())
	if a.pendingToast != wantToast {
		t.Errorf("pendingToast = %q, want %q", a.pendingToast, wantToast)
	}
}
