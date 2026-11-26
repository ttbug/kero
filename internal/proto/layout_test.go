package proto

import (
	"slices"
	"testing"
)

func TestLayoutSplitRemove(t *testing.T) {
	var l *Layout
	l = l.Split("a", "b", "right", 0)
	l = l.Split("b", "c", "down", 0)
	l = l.Split("a", "d", "left", 0)
	if got := l.Panes(); !slices.Equal(got, []string{"d", "a", "b", "c"}) {
		t.Fatalf("panes %v", got)
	}
	if n := l.Neighbor("a", "right"); n != "b" && n != "c" {
		t.Errorf("right of a: %q", n)
	}
	if n := l.Neighbor("b", "down"); n != "c" {
		t.Errorf("below b: %q", n)
	}
	if n := l.Neighbor("d", "left"); n != "" {
		t.Errorf("left of d: %q", n)
	}
	l = l.Remove("b")
	if got := l.Panes(); !slices.Equal(got, []string{"d", "a", "c"}) {
		t.Fatalf("after removing b: %v", got)
	}
	l = l.Remove("d").Remove("c")
	if !l.Leaf() || l.Pane != "a" {
		t.Fatalf("last pane: %+v", l)
	}
	if l.Remove("a") != nil {
		t.Fatal("no pane left, a layout remains")
	}
}

func TestLead(t *testing.T) {
	panes := []Pane{{ID: "a", Status: Working}, {ID: "b", Status: Idle, Unread: true}, {ID: "c", Status: Waiting}}
	if i := Lead(panes, "a"); panes[i].ID != "c" {
		t.Fatalf("lead %s, want the one waiting", panes[i].ID)
	}
	panes[2].Status = Idle
	if i := Lead(panes, "a"); panes[i].ID != "b" {
		t.Fatalf("lead %s, want the unread one", panes[i].ID)
	}
}

func TestDisplayNameFollowsFocus(t *testing.T) {
	agent := Pane{ID: "a", Process: "claude", Agent: "claude", Title: "✳ Fix login", Status: Working}
	shell := Pane{ID: "b", Shell: "fish", Dir: "/src/web", Status: Idle}
	s := Session{Pane: agent, Panes: []Pane{agent, shell}, Lead: "a", Focus: "b"}
	if n := s.DisplayName(NameFocused); n != "Fix login" {
		t.Fatalf("name %q, want the agent's title over an idle shell", n)
	}
	s.Panes[1].Process, s.Panes[1].Title = "vim", "main.go"
	if n := s.DisplayName(NameFocused); n != "main.go" {
		t.Fatalf("name %q, want the focused program's title", n)
	}
	if n := s.DisplayName(NameLead); n != "Fix login" {
		t.Fatalf("name %q, want the lead's title", n)
	}
	s.Panes[0].Dir = "/src/api"
	if n := s.DisplayName(NameFolder); n != "api" {
		t.Fatalf("name %q, want the first pane's folder", n)
	}
	s.Focus = "a"
	if n := s.DisplayName(NameFocused); n != "Fix login" {
		t.Fatalf("name %q, want the agent's title", n)
	}
}

func TestIsGenericAgentTitle(t *testing.T) {
	for _, tc := range []struct {
		title, agent string
		want         bool
	}{
		{"✳ Claude Code", "claude", true},
		{"claude", "claude", true},
		{"Antigravity", "antigravity", true},
		{"agy", "antigravity", true},
		{"Codex", "codex", true},
		{"OpenAI Codex", "codex", true},
		{"codex|kero|Ready|01a215d6-a2...", "codex", true},
		{"codex | kero | Working | 1234", "codex", true},
		{"Fix login", "claude", false},
		{"配置按钮删除", "antigravity", false},
		{"Antigravity", "", false},
	} {
		if got := IsGenericAgentTitle(tc.title, tc.agent); got != tc.want {
			t.Errorf("IsGenericAgentTitle(%q, %q) = %v, want %v", tc.title, tc.agent, got, tc.want)
		}
	}
}

func TestSidebarTitle(t *testing.T) {
	shell := Session{ID: "s1", Pane: Pane{ID: "p1", Shell: "zsh", Dir: "/src/one-rust"}}
	if got := shell.SidebarTitle(NameFocused, "新会话"); got != "新会话" {
		t.Fatalf("empty shell title = %q, want 新会话", got)
	}
	if got := shell.SidebarTitle(NameFolder, "新会话"); got != "one-rust" {
		t.Fatalf("NameFolder title = %q, want one-rust", got)
	}

	codexOSC := Session{
		ID:   "s2",
		Pane: Pane{ID: "p2", Process: "codex", Agent: "codex", Title: "codex|kero|Ready|01a215d6-a2...", Dir: "/src/one-rust"},
	}
	if got := codexOSC.SidebarTitle(NameFocused, "新会话"); got != "新会话" {
		t.Fatalf("codex OSC title = %q, want 新会话", got)
	}

	agentOSC := Session{
		ID:   "s2b",
		Pane: Pane{ID: "p2b", Process: "antigravity", Agent: "antigravity", Title: "✳ 配置按钮删除", Dir: "/src/one-rust"},
	}
	if got := agentOSC.SidebarTitle(NameFocused, "新会话"); got != "配置按钮删除" {
		t.Fatalf("antigravity OSC title = %q, want 配置按钮删除", got)
	}

	agyOSCOverPrompt := Session{
		ID:   "s2c",
		Pane: Pane{ID: "p2c", Shell: "zsh", Process: "antigravity", Agent: "antigravity", Title: "⠋ 配置按钮删除", AgentTitle: "帮我把配置按钮删掉", Dir: "/src/one-rust"},
	}
	if got := agyOSCOverPrompt.SidebarTitle(NameFocused, "新会话"); got != "配置按钮删除" {
		t.Fatalf("antigravity OSC over prompt = %q, want 配置按钮删除", got)
	}
	if got := agyOSCOverPrompt.DisplayName(NameFocused); got != "配置按钮删除" {
		t.Fatalf("DisplayName with antigravity OSC = %q, want 配置按钮删除", got)
	}
	agyOSCOverPrompt.Name = "手动标题"
	if got := agyOSCOverPrompt.SidebarTitle(NameFocused, "新会话"); got != "手动标题" {
		t.Fatalf("explicit Name over antigravity OSC = %q, want 手动标题", got)
	}

	for _, ignored := range []string{"Antigravity", "agy", "/src/one-rust", "~/one-rust", "zsh", "one-rust", "Antigravity - one-rust"} {
		withPrompt := Session{
			ID:   "s2d",
			Pane: Pane{ID: "p2d", Shell: "zsh", Process: "antigravity", Agent: "antigravity", Title: ignored, AgentTitle: "帮我把配置按钮删掉", Dir: "/src/one-rust"},
		}
		if got := withPrompt.SidebarTitle(NameFocused, "新会话"); got != "帮我把配置按钮删掉" {
			t.Fatalf("antigravity ignored OSC %q with AgentTitle = %q, want 帮我把配置按钮删掉", ignored, got)
		}
		noPrompt := Session{
			ID:   "s2e",
			Pane: Pane{ID: "p2e", Shell: "zsh", Process: "antigravity", Agent: "antigravity", Title: ignored, Dir: "/src/one-rust"},
		}
		if got := noPrompt.SidebarTitle(NameFocused, "新会话"); got != "新会话" {
			t.Fatalf("antigravity ignored OSC %q without AgentTitle = %q, want 新会话", ignored, got)
		}
	}

	claudeOSC := Session{
		ID:   "s2f",
		Pane: Pane{ID: "p2f", Process: "claude", Agent: "claude", Title: "✳ 某个标题", Dir: "/src/one-rust"},
	}
	if got := claudeOSC.SidebarTitle(NameFocused, "新会话"); got != "新会话" {
		t.Fatalf("claude OSC without AgentTitle = %q, want 新会话", got)
	}

	prompted := Session{
		ID:   "s3",
		Pane: Pane{ID: "p3", Process: "codex", Agent: "codex", Title: "codex|kero|Ready|01a215d6-a2...", AgentTitle: "那你帮我出一版新的视觉", Dir: "/src/one-rust"},
	}
	if got := prompted.SidebarTitle(NameFocused, "新会话"); got != "那你帮我出一版新的视觉" {
		t.Fatalf("agentTitle = %q, want 那你帮我出一版新的视觉", got)
	}
	if got := prompted.DisplayName(NameFocused); got != "那你帮我出一版新的视觉" {
		t.Fatalf("DisplayName with agentTitle = %q, want 那你帮我出一版新的视觉", got)
	}

	prompted.Name = "自定义名称"
	if got := prompted.SidebarTitle(NameFocused, "新会话"); got != "自定义名称" {
		t.Fatalf("explicit Name = %q, want 自定义名称", got)
	}

	vim := Session{ID: "s4", Pane: Pane{ID: "p4", Process: "vim", Title: "main.go", Dir: "/src/one-rust"}}
	if got := vim.SidebarTitle(NameFocused, "新会话"); got != "main.go" {
		t.Fatalf("vim title = %q, want main.go", got)
	}

	browser := Session{ID: "s5", Pane: Pane{ID: "p5", URL: "https://example.com/docs", Title: "Docs"}}
	if got := browser.SidebarTitle(NameFocused, "新会话"); got != "Docs" {
		t.Fatalf("browser title = %q, want Docs", got)
	}
}

func TestLayoutMoveSwap(t *testing.T) {
	var l *Layout
	l = l.Split("a", "b", "right", 0).Split("b", "c", "down", 0) // a | (b / c)
	m := l.Move("c", "a", "left")                                // c | a | b
	if got := m.Panes(); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("move: %v", got)
	}
	if n := m.Neighbor("a", "left"); n != "c" {
		t.Errorf("left of a after the move: %q", n)
	}
	if got := l.Swap("a", "c").Panes(); !slices.Equal(got, []string{"c", "b", "a"}) {
		t.Fatalf("swap: %v", got)
	}
	if got := l.Panes(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("swap changed the original: %v", got)
	}
	if l.Move("a", "a", "up") != l {
		t.Error("a pane moved onto itself")
	}
}
