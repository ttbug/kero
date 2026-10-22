package main

import (
	"testing"

	"kero/internal/proto"

	"kero/locales"
)

// A project's header shows the one branch its sessions share, and none
// when they tell of several.
func TestGroupBranch(t *testing.T) {
	session := func(branch, def string) *proto.Session {
		s := &proto.Session{}
		if branch != "" || def != "" {
			s.Git = &proto.GitInfo{Branch: branch, Default: def}
		}
		return s
	}
	for _, tt := range []struct {
		name string
		gs   []*proto.Session
		want string
	}{
		{"off default", []*proto.Session{session("feat/sidebar", "main")}, "feat/sidebar"},
		{"on default", []*proto.Session{session("main", "main")}, "main"},
		{"shared", []*proto.Session{session("feat/sidebar", ""), session("feat/sidebar", "")}, "feat/sidebar"},
		{"several", []*proto.Session{session("one", ""), session("two", "")}, ""},
		{"none", []*proto.Session{session("", "")}, ""},
	} {
		if got := groupBranch(group{sessions: tt.gs}); got != tt.want {
			t.Errorf("%s: branch %q, want %q", tt.name, got, tt.want)
		}
	}
}

// A collapsed project shows none of its sessions, the selected one too.
func TestVisibleOrderSkipsCollapsed(t *testing.T) {
	a := &App{
		st: proto.State{
			Projects: []proto.Project{{ID: "p1", Name: "one"}, {ID: "p2", Name: "two"}},
			Sessions: []proto.Session{{ID: "a", Project: "p1"}, {ID: "b", Project: "p2"}},
		},
		prefs: prefs{Selected: "a", Collapsed: map[string]bool{"p1": true}},
	}
	if got := a.visibleOrder(); len(got) != 1 || got[0] != "b" {
		t.Errorf("visible order %v, want [b]", got)
	}
}

// A session's row names its status, with Ready for an idle session the
// user has seen.
func TestSessionStatusLabel(t *testing.T) {
	for _, tt := range []struct {
		name string
		s    proto.Session
		want string
	}{
		{"working", proto.Session{Pane: proto.Pane{Status: proto.Working}}, locales.StatusWorking()},
		{"waiting", proto.Session{Pane: proto.Pane{Status: proto.Waiting}}, locales.StatusNeedsYou()},
		{"failed", proto.Session{Pane: proto.Pane{Status: proto.Failed}}, locales.StatusFailed()},
		{"exited", proto.Session{Pane: proto.Pane{Status: proto.Exited}}, locales.StatusExited()},
		{"done", proto.Session{Pane: proto.Pane{Status: proto.Idle, Unread: true}}, locales.StatusDone()},
		{"ready", proto.Session{Pane: proto.Pane{Status: proto.Idle}}, locales.StatusReady()},
	} {
		if got := sessionStatusLabel(&tt.s); got != tt.want {
			t.Errorf("%s: label %q, want %q", tt.name, got, tt.want)
		}
	}
}
