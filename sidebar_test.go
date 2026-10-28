package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"kero/internal/proto"

	"kero/locales"
)

// The sidebar opens with the reference's brand, and counts the sessions
// under it, each row naming its status ($KERO_SHOTS).
func TestSidebarBrandAndTasks(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	shot(t, tt, "sidebar")
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Xview", "All In One 终端工作台", locales.SidebarTasks(1), locales.StatusReady()} {
		if !strings.Contains(texts, want) {
			t.Errorf("the sidebar says none of %q; texts %q", want, tt.Texts())
		}
	}
}

// A project folds all its sessions away, the one shown included, which
// stays shown in the window.
func TestFoldHidesSelected(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	a.st.Sessions[0].Name = "wombat"
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText("wombat") {
		t.Fatalf("the row is missing before the fold: %q", tt.Texts())
	}
	a.toggleProject("p1")
	tt.Frame()
	if tt.HasText("wombat") {
		t.Errorf("the row stays after the fold: %q", tt.Texts())
	}
	if a.selected != "s1" {
		t.Errorf("the shown session changed to %q", a.selected)
	}
	if ids := a.visibleOrder(); len(ids) != 0 {
		t.Errorf("visible order %q", ids)
	}
	a.toggleProject("p1")
	tt.Frame()
	if !tt.HasText("wombat") {
		t.Errorf("the row is missing after the unfold: %q", tt.Texts())
	}
}

// Closing a session hands the window to the next one the sidebar shows,
// else the one before, including when the closed one was folded away.
func TestNeighbor(t *testing.T) {
	a := testApp(t)
	a.st.Projects = append(a.st.Projects, proto.Project{ID: "p2", Name: "next", Dir: "/tmp"})
	a.st.Sessions = append(a.st.Sessions, proto.Session{ID: "s2", Project: "p2", Name: "next"})
	if got := a.neighbor("s1"); got != "s2" {
		t.Errorf("next %q, want s2", got)
	}
	if got := a.neighbor("s2"); got != "s1" {
		t.Errorf("before %q, want s1", got)
	}
	a.prefs.Collapsed = map[string]bool{"p1": true}
	if got := a.neighbor("s1"); got != "s2" {
		t.Errorf("folded, next %q, want s2", got)
	}
	a.prefs.Collapsed["p2"] = true
	if got := a.neighbor("s1"); got != "" {
		t.Errorf("all folded, %q, want none", got)
	}
}

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
