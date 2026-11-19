package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"kero/internal/proto"

	"kero/locales"
)

// The sidebar opens with the reference's brand, and counts the sessions
// under it, each row naming its program and session ($KERO_SHOTS).
func TestSidebarBrandAndTasks(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	shot(t, tt, "sidebar")
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Xview", "All In One 终端工作台", locales.SidebarProjects(1), locales.SidebarTasks(1), "Shell", locales.EndedTitle()} {
		if !strings.Contains(texts, want) {
			t.Errorf("the sidebar says none of %q; texts %q", want, tt.Texts())
		}
	}
}

// The reference's two levels keep the real session name and creation
// time visible. Long names stay in the sidebar at its minimum width.
func TestSidebarReferenceLayout(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a := testApp(t)
	a.local().conn = connReady
	a.set.SessionHistory = true
	a.st.Projects = []proto.Project{{ID: "p1", Name: "workspace", Dir: "/tmp/workspace"}, {ID: "p2", Name: "categraf", Dir: "/tmp/categraf"}}
	created := time.Date(2026, 11, 19, 15, 55, 0, 0, time.Local).UnixMilli()
	a.st.Sessions = []proto.Session{
		{ID: "s1", Project: "p1", Name: "新会话", Pane: proto.Pane{Shell: "zsh", Status: proto.Idle, Created: created}},
		{ID: "s2", Project: "p2", Name: "新会话", Pane: proto.Pane{Shell: "zsh", Status: proto.Idle, Created: created, Git: &proto.GitInfo{Branch: "main", Default: "main"}}},
	}
	a.selected = "s2"
	tt := ui.NewTester(a.view, 1100, 700)
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		for _, want := range []string{locales.SidebarProjects(2), locales.SidebarTasks(2), "Shell", "新会话", "· 15:55", "main", locales.EndedTitle()} {
			if !tt.HasText(want) {
				t.Errorf("reference layout missing %q; texts %q", want, tt.Texts())
			}
		}
		name := "sidebar-reference-light"
		if dark {
			name = "sidebar-reference-dark"
		}
		shot(t, tt, name)
	}
	a.prefs.SidebarWidth = minSidebar
	a.st.Sessions[1].Name = "a very long session name that must stay inside the sidebar"
	tt.Frame()
	if r, ok := tt.Find(a.st.Sessions[1].Name); !ok || r.X+r.W > minSidebar {
		t.Fatalf("long session name escaped the sidebar: %v, found %t", r, ok)
	}
	shot(t, tt, "sidebar-reference-narrow")
}

// Switching a theme in the same window updates the sidebar's wash and
// selected edge along with the rest of the window, including Ghostty themes.
func TestSidebarFollowsTheme(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	tt := ui.NewTester(a.view, 1100, 700)
	reference := ui.NewTester(func(c *ui.Context) {
		a.looks()
		backdrop(c, ui.Row(c).Fill())
		ui.Box(c).Absolute().Left(800).Top(500).Size(10, 10).Background(paletteFor(c).accent)
	}, 1100, 700)
	for _, tc := range []struct {
		name, theme string
		dark        bool
	}{
		{"paper-light", "paper", false},
		{"paper-dark", "paper", true},
		{"tokyonight-moon", "TokyoNight Moon", true},
		{"catppuccin-mocha", "Catppuccin Mocha", true},
		{"tokyonight-day", "TokyoNight Day", false},
		{"plain-light", "plain", false},
		{"plain-dark", "plain", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a.set.LightTheme, a.set.DarkTheme = tc.theme, tc.theme
			tt.SetDark(tc.dark)
			tt.Frame()
			tt.Advance(time.Second)
			tt.Frame()
			reference.SetDark(tc.dark)
			reference.Frame()
			got, want := tt.Image(), reference.Image()
			if got.RGBAAt(150, 500) != want.RGBAAt(150, 500) {
				t.Errorf("sidebar background %v differs from theme background %v", got.RGBAAt(150, 500), want.RGBAAt(150, 500))
			}
			program, ok := tt.Find("Shell")
			if !ok {
				t.Fatal("sidebar program missing")
			}
			if col := got.RGBAAt(4, int(program.Y+program.H/2)); col != want.RGBAAt(805, 505) {
				t.Errorf("selected edge %v differs from theme accent %v", col, want.RGBAAt(805, 505))
			}
			shot(t, tt, "sidebar-theme-"+tc.name)
		})
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
