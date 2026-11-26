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
	projLabel := strings.TrimSpace(strings.TrimSuffix(locales.SidebarProjects(1), "1"))
	taskLabel := strings.TrimSpace(strings.TrimPrefix(locales.SidebarTasks(1), "·"))
	for _, want := range []string{"Xview", "All In One 终端工作台", projLabel, taskLabel, "Shell", locales.EndedTitle()} {
		if !strings.Contains(texts, want) {
			t.Errorf("the sidebar says none of %q; texts %q", want, tt.Texts())
		}
	}
}

// The reference's card-and-tree layout keeps project cards, Files & Diff
// summaries, session titles, status/program subtitles, and relative times
// inside the sidebar even at its minimum width.
func TestSidebarReferenceLayout(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a := testApp(t)
	a.local().conn = connReady
	a.set.SessionHistory = true
	a.st.Projects = []proto.Project{{ID: "p1", Name: "workspace", Dir: "/tmp/workspace"}, {ID: "p2", Name: "categraf", Dir: "/tmp/categraf"}}
	created := time.Now().Add(-2 * time.Hour).UnixMilli()
	a.st.Sessions = []proto.Session{
		{ID: "s1", Project: "p1", Name: "新会话", Pane: proto.Pane{Shell: "zsh", Status: proto.Idle, Created: created}},
		{ID: "s2", Project: "p2", Name: "配置按钮删除", Pane: proto.Pane{Shell: "zsh", Agent: "claude", Status: proto.Idle, Created: created, Git: &proto.GitInfo{Branch: "main", Default: "main", Files: 3, Added: 649, Removed: 125}}},
		{ID: "s3", Project: "p2", Name: "多平台权限兼容", Pane: proto.Pane{Shell: "zsh", Agent: "codex", Status: proto.Idle, Created: created}},
		{ID: "s4", Project: "p2", Name: "优化侧边栏", Pane: proto.Pane{Shell: "zsh", Agent: "claude", Status: proto.Working, Created: created}},
	}
	a.selected = "s2"
	tt := ui.NewTester(a.view, 1100, 700)
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		for _, want := range []string{"当前项目", "会话 4", "Project Files", "Files & Diff", "+649", "−125", "3", "Shell", "Claude Code", "Codex", "新会话", "配置按钮删除", "多平台权限兼容", "优化侧边栏", "运行中", "就绪", "main", locales.EndedTitle()} {
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
// selected tree glow along with the rest of the window, including Ghostty themes.
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
			tag, ok := tt.Find("All In One 终端工作台")
			if !ok {
				t.Fatal("sidebar brand tagline missing")
			}
			wantAccent := want.RGBAAt(805, 505)
			foundAccent := false
			for y := int(tag.Y + tag.H + 4); y <= int(tag.Y+tag.H+24); y++ {
				for x := 144; x <= 158; x++ {
					if got.RGBAAt(x, y) == wantAccent {
						foundAccent = true
						break
					}
				}
				if foundAccent {
					break
				}
			}
			if !foundAccent {
				t.Errorf("brand accent %v not found below tagline", wantAccent)
			}
			shot(t, tt, "sidebar-theme-"+tc.name)
		})
	}
}

// Idle and selected sessions keep neutral tree lines without animation;
// working sessions stream animated data pulses along the trunk into their
// rows ($KERO_SHOTS).
func TestSidebarWorkingDataFlow(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	a.st.Projects = []proto.Project{{ID: "p1", Name: "one-rust", Dir: "/tmp/one-rust"}}
	a.st.Sessions = []proto.Session{
		{ID: "s1", Project: "p1", Name: "配置按钮删除", Pane: proto.Pane{Shell: "zsh", Agent: "claude", Status: proto.Idle}},
		{ID: "s2", Project: "p1", Name: "全局性能优化", Pane: proto.Pane{Shell: "zsh", Agent: "claude", Status: proto.Idle}},
		{ID: "s3", Project: "p1", Name: "跨 Agent 交接", Pane: proto.Pane{Shell: "zsh", Agent: "codex", Status: proto.Idle}},
	}
	a.selected = "s1"
	tt := ui.NewTester(a.view, 1100, 700)
	tt.SetDark(true)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()

	firstRow, ok1 := tt.Find("配置按钮删除")
	lastRow, ok2 := tt.Find("跨 Agent 交接")
	if !ok1 || !ok2 {
		t.Fatalf("missing session rows; texts %q", tt.Texts())
	}
	yStart, yEnd := int(firstRow.Y), int(lastRow.Y+lastRow.H)
	img1 := tt.Image()
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	img2 := tt.Image()
	for y := yStart; y <= yEnd; y++ {
		if img1.RGBAAt(20, y) != img2.RGBAAt(20, y) {
			t.Fatalf("idle selected tree trunk changed at y=%d (%v -> %v)", y, img1.RGBAAt(20, y), img2.RGBAAt(20, y))
		}
	}

	// Turn the second session into Working: the trunk pixels must animate.
	a.st.Sessions[1].Status = proto.Working
	tt.Frame()
	imgA := tt.Image()
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	imgB := tt.Image()
	changed := 0
	for y := yStart; y <= yEnd; y++ {
		if imgA.RGBAAt(20, y) != imgB.RGBAAt(20, y) {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("working session tree trunk did not animate over 250ms")
	}
	shot(t, tt, "sidebar-dataflow-dark")
}

// Clicking a project card's Files & Diff or Files row opens the corresponding
// right sidebar tab, switching to that project's session when needed; folding
// the project hides the row.
func TestSidebarProjectFilesRow(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	a.st.Projects = []proto.Project{
		{ID: "p1", Name: "alpha", Dir: "/tmp/alpha"},
		{ID: "p2", Name: "beta", Dir: "/tmp/beta"},
	}
	a.st.Sessions = []proto.Session{
		{ID: "s1", Project: "p1", Name: "one", Pane: proto.Pane{ID: "a", Shell: "zsh", Status: proto.Idle, Progress: -1}},
		{ID: "s2", Project: "p2", Name: "two", Pane: proto.Pane{ID: "b", Shell: "zsh", Status: proto.Idle, Progress: -1, Git: &proto.GitInfo{Root: "/tmp/beta", Branch: "main", Default: "main", Files: 2, Added: 10, Removed: 3}}},
	}
	a.selected = "s1"
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if err := tt.Click("Files & Diff"); err != nil {
		t.Fatalf("could not click Files & Diff: %v", err)
	}
	tt.Frame()
	if a.selected != "s2" || a.prefs.Side != sideChanges {
		t.Fatalf("after clicking Files & Diff on p2: selected=%q side=%q, want s2 / changes", a.selected, a.prefs.Side)
	}
	a.toggleProject("p2")
	tt.Frame()
	if tt.HasText("Files & Diff") || tt.HasText("two") {
		t.Errorf("folding p2 left Files & Diff or session visible: %q", tt.Texts())
	}
}

// The bottom-left footer shows how many agents are running and waiting for
// approval, and lists the sessions needing approval above it even when their
// project is folded; clicking one unfolds and focuses it ($KERO_SHOTS).
func TestSidebarFooterAgentStatus(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a := testApp(t)
	a.local().conn = connReady
	a.st.Projects = []proto.Project{
		{ID: "p1", Name: "one-rust", Dir: "/tmp/one-rust"},
		{ID: "p2", Name: "js-motion", Dir: "/tmp/js-motion"},
	}
	a.st.Sessions = []proto.Session{
		{ID: "s1", Project: "p1", Name: "性能优化", Pane: proto.Pane{Shell: "zsh", Agent: "claude", Status: proto.Working}},
		{ID: "s2", Project: "p1", Name: "接口重构", Pane: proto.Pane{Shell: "zsh", Agent: "codex", Status: proto.Working}},
		{ID: "s3", Project: "p1", Name: "发布审批", Pane: proto.Pane{Shell: "zsh", Agent: "claude", Status: proto.Waiting, Kind: "permission", Message: "Allow running git push?"}},
		{ID: "s4", Project: "p2", Name: "依赖确认", Pane: proto.Pane{Shell: "zsh", Agent: "gemini", Status: proto.Waiting, Kind: "question"}},
	}
	a.selected = "s1"
	a.prefs.Collapsed = map[string]bool{"p2": true}
	tt := ui.NewTester(a.view, 1100, 700)
	for _, dark := range []bool{true, false} {
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		for _, want := range []string{"2 运行中", "2 需要你", "发布审批", "依赖确认", "gemini"} {
			if !tt.HasText(want) {
				t.Errorf("footer agent status missing %q; texts %q", want, tt.Texts())
			}
		}
		name := "sidebar-footer-agents-light"
		if dark {
			name = "sidebar-footer-agents-dark"
		}
		shot(t, tt, name)
	}

	if err := tt.Click("依赖确认 · gemini"); err != nil {
		t.Fatalf("could not click waiting agent item: %v", err)
	}
	tt.Frame()
	if a.selected != "s4" || a.prefs.Collapsed["p2"] {
		t.Fatalf("clicking waiting item left selected=%q collapsed=%v, want s4 and unfolded", a.selected, a.prefs.Collapsed["p2"])
	}

	a.prefs.SidebarWidth = minSidebar
	tt.Frame()
	if r, ok := tt.Find(a.tip(locales.SettingsTitle(), "settings.open")); !ok || r.X+r.W > minSidebar {
		t.Fatalf("settings button escaped narrow sidebar: %v, found %t", r, ok)
	}
}

// Antigravity sessions show their dedicated emblem, brand color, and
// "Antigravity" program name in both session rows and the attention card.
func TestSidebarAntigravity(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a := testApp(t)
	a.local().conn = connReady
	a.st.Projects = []proto.Project{{ID: "p1", Name: "kero", Dir: "/tmp/kero"}}
	a.st.Sessions = []proto.Session{
		{ID: "s1", Project: "p1", Name: "侧边栏适配", Pane: proto.Pane{Shell: "zsh", Process: "antigravity", Agent: "antigravity", Status: proto.Working}},
		{ID: "s2", Project: "p1", Name: "确认提交", Pane: proto.Pane{Shell: "zsh", Process: "antigravity", Agent: "antigravity", Status: proto.Waiting, Kind: "permission"}},
	}
	a.selected = "s1"

	p := &a.st.Sessions[0].Pane
	if got := programBitmap(p); got == nil {
		t.Fatal("programBitmap(antigravity) = nil, want decoded bitmap")
	}
	if got := programBitmap(&proto.Pane{Agent: "agy"}); got == nil {
		t.Fatal("programBitmap(agy) = nil, want decoded bitmap")
	}
	if got := programIcon(p); got != programIcons["antigravity"] || got == terminalIcon {
		t.Fatalf("programIcon(antigravity) = %p, want %p", got, programIcons["antigravity"])
	}
	if got := programColor(&darkPalette, p); got != agentColors["antigravity"] {
		t.Fatalf("programColor(dark, antigravity) = %v, want %v", got, agentColors["antigravity"])
	}
	if got := programColor(&lightPalette, p); got != agentColors["antigravity"] {
		t.Fatalf("programColor(light, antigravity) = %v, want %v", got, agentColors["antigravity"])
	}

	tt := ui.NewTester(a.view, 1100, 700)
	for _, dark := range []bool{true, false} {
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		for _, want := range []string{"侧边栏适配", "确认提交", "Antigravity", "1 运行中", "1 需要你"} {
			if !tt.HasText(want) {
				t.Errorf("antigravity sidebar missing %q; texts %q", want, tt.Texts())
			}
		}
		name := "sidebar-antigravity-light"
		if dark {
			name = "sidebar-antigravity-dark"
		}
		shot(t, tt, name)
	}
}

func TestSidebarSessionTitle(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })

	a := testApp(t)
	a.local().conn = connReady
	created := time.Now().Add(-2 * time.Hour).UnixMilli()
	a.st.Projects = []proto.Project{
		{ID: "p1", Name: "one-rust", Dir: "/tmp/one-rust"},
		{ID: "p2", Name: "project", Dir: "/tmp/project"},
	}
	a.st.Sessions = []proto.Session{
		{ID: "s1", Project: "p1", Pane: proto.Pane{ID: "a", Shell: "zsh", Process: "antigravity", Agent: "antigravity", Title: "Antigravity", AgentTitle: "配置按钮删除", Status: proto.Idle, Created: created}},
		{ID: "s1b", Project: "p1", Pane: proto.Pane{ID: "a2", Shell: "zsh", Process: "antigravity", Agent: "antigravity", Title: "✳ 侧栏标题优化", AgentTitle: "帮我排查侧边栏会话名", Status: proto.Idle, Created: created}},
		{ID: "s2", Project: "p1", Pane: proto.Pane{ID: "b", Shell: "zsh", Process: "claude", Agent: "claude", Title: "✳ Claude Code", AgentTitle: "那你帮我出一版新的视觉，并帮我做个侧边栏", Status: proto.Working, Created: created}},
		{ID: "s3", Project: "p1", Pane: proto.Pane{ID: "c", Shell: "zsh", Process: "codex", Agent: "codex", Title: "codex|kero|Ready|01a215d6-a2...", Status: proto.Idle, Created: created}},
		{ID: "s4", Project: "p1", Pane: proto.Pane{ID: "d", Shell: "zsh", Process: "vim", Title: "main.go", Status: proto.Idle, Created: created}},
		{ID: "s5", Project: "p1", Name: "手动命名会话", Pane: proto.Pane{ID: "e", Shell: "zsh", Process: "claude", Agent: "claude", Title: "✳ Other", Status: proto.Idle, Created: created}},
		{ID: "s6", Project: "p2", Pane: proto.Pane{ID: "f", Shell: "zsh", Dir: "/tmp/project", Status: proto.Idle, Created: created}},
	}
	a.selected = "s1"

	tt := ui.NewTester(a.view, 1100, 700)
	for _, dark := range []bool{true, false} {
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		for _, want := range []string{
			"配置按钮删除",
			"侧栏标题优化",
			"那你帮我出一版新的视觉，并帮我做个侧边栏",
			"新会话",
			"main.go",
			"手动命名会话",
			"Antigravity",
			"Claude Code",
			"Codex",
			"Shell",
		} {
			if !tt.HasText(want) {
				t.Errorf("dark=%t missing %q; texts %q", dark, want, tt.Texts())
			}
		}
		if tt.HasText("✳ Claude Code") || tt.HasText("✳ 侧栏标题优化") || tt.HasText("帮我排查侧边栏会话名") || tt.HasText("codex|kero|Ready|01a215d6-a2...") || tt.HasText("one-rust ·") {
			t.Errorf("dark=%t showed agent OSC title or wrong fallback: %q", dark, tt.Texts())
		}
		name := "sidebar-titles-light"
		if dark {
			name = "sidebar-titles-dark"
		}
		shot(t, tt, name)
	}

	// Switching locale to English updates empty/generic session titles to "New Session".
	i18n.SetLocaleFunc(func() string { return "en" })
	tt.Frame()
	if !tt.HasText("New Session") {
		t.Errorf("english locale missing New Session; texts %q", tt.Texts())
	}
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	tt.Frame()

	// Double-clicking an untitled session and committing unchanged does not persist "新会话",
	// even if a new conversation title arrives while the rename input is open.
	a.startRename("s6")
	if a.renameBuf != "新会话" {
		t.Fatalf("renameBuf = %q, want 新会话", a.renameBuf)
	}
	a.session("s6").Pane.Agent = "claude"
	a.session("s6").Pane.AgentTitle = "异步到达的标题"
	a.commitRename()
	if a.session("s6").Name != "" {
		t.Fatalf("commitRename persisted placeholder name %q", a.session("s6").Name)
	}

	// Explicit NameFolder setting still displays the folder name.
	a.set.SessionName = proto.NameFolder
	tt.Frame()
	if !tt.HasText("project") {
		t.Errorf("NameFolder missing folder name; texts %q", tt.Texts())
	}
	a.set.SessionName = proto.NameFocused

	// Narrow sidebar keeps long session titles inside bounds.
	a.prefs.SidebarWidth = minSidebar
	tt.Frame()
	if r, ok := tt.Find("那你帮我出一版新的视觉，并帮我做个侧边栏"); !ok || r.X+r.W > minSidebar {
		t.Fatalf("long title escaped narrow sidebar: %v, found %t", r, ok)
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
