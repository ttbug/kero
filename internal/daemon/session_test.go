package daemon

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"kero/internal/proto"
)

// testPane is a pane with no program, whose output the test writes.
func testPane(t *testing.T) *pane {
	t.Helper()
	t.Setenv("KERO_CONFIG", filepath.Join(t.TempDir(), "config"))
	d := &Daemon{changeCh: make(chan struct{}, 1), saveCh: make(chan struct{}, 1)}
	d.viewed.Store(new(string))
	s, err := newPane(d, "s", proto.Pane{ID: "p", Status: proto.Idle, Cols: 96, Rows: 28})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.term.Close() })
	return s
}

// show writes a program's redraw of the whole screen.
func (s *pane) show(text string) {
	s.output([]byte("\x1b[H\x1b[2J" + strings.ReplaceAll(strings.TrimRight(text, "\n"), "\n", "\r\n")))
}

func claudeScreen(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "detect", "testdata", "claude-"+name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (s *pane) status(now time.Time) proto.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.look(now)
	return s.info.Status
}

// Claude Code at its prompt repaints its screen as the sidebar hides and
// the pane resizes, over and over: still idle. Its turn is work, and the
// prompt back is idle again.
func TestClaudeRedrawIsNoWork(t *testing.T) {
	s := testPane(t)
	s.info.Agent, s.info.Title = "claude", "✳ Claude Code"
	now := time.Now()
	idle := claudeScreen(t, "idle")
	s.show(idle)
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("at the prompt: %s", st)
	}
	for i := range 20 {
		s.resize(96-i, 28, 0, 0)
		s.show(idle)
		now = now.Add(16 * time.Millisecond)
		if st := s.status(now); st != proto.Idle {
			t.Fatalf("redraw %d after a resize: %s", i, st)
		}
	}
	s.show(claudeScreen(t, "working"))
	if st := s.status(now); st != proto.Working {
		t.Fatalf("in a turn: %s", st)
	}
	// Redraws of the turn, as a resize makes, are the same turn.
	s.resize(80, 28, 0, 0)
	s.show(claudeScreen(t, "working"))
	if st := s.status(now); st != proto.Working {
		t.Fatalf("in a turn, resized: %s", st)
	}
	s.show(idle)
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("after the turn: %s", st)
	}
	if !s.info.Unread {
		t.Error("a turn that ended unseen is not unread")
	}
	s.show(claudeScreen(t, "blocked"))
	if st := s.status(now); st != proto.Waiting {
		t.Fatalf("asking to run a command: %s", st)
	}
	// Keys that move in the dialog leave it asking.
	s.input(nil)
	if st := s.status(now); st != proto.Waiting {
		t.Fatalf("asking, after a key: %s", st)
	}
	s.show(idle)
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("back at the prompt: %s", st)
	}
}

// Claude Code at its prompt for a while notifies "waiting for your
// input": its screen, at the prompt, tells it is idle, not waiting. Its
// notification at a permission dialog words the wait. "kero notify" from
// it needs the user all the same.
func TestAgentNotifyFollowsScreen(t *testing.T) {
	s := testPane(t)
	s.info.Agent = "claude"
	now := time.Now()
	s.show(claudeScreen(t, "idle"))
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("at the prompt: %s", st)
	}
	s.output([]byte("\x1b]9;Claude is waiting for your input\x07"))
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("notified at the prompt: %s", st)
	}
	s.output([]byte("\x07"))
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("bell at the prompt: %s", st)
	}
	s.show(claudeScreen(t, "blocked"))
	if st := s.status(now); st != proto.Waiting {
		t.Fatalf("asking to run a command: %s", st)
	}
	s.output([]byte("\x1b]9;Claude needs your permission to use Bash\x07"))
	if st := s.status(now); st != proto.Waiting || s.info.Message != "Claude needs your permission to use Bash" {
		t.Fatalf("notified while asking: %s %q", st, s.info.Message)
	}
	s.show(claudeScreen(t, "idle"))
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("back at the prompt: %s", st)
	}
	s.mu.Lock()
	s.flag("", "Need your API key to continue")
	s.mu.Unlock()
	if st := s.status(now); st != proto.Waiting || s.info.Message != "Need your API key to continue" {
		t.Fatalf("kero notify: %s %q", st, s.info.Message)
	}
}

// A program's bell and notification tell nothing of its status: vim
// beeps at a wrong key, a build notifies it is done.
func TestProgramNotifyIsNoWait(t *testing.T) {
	s := testPane(t)
	s.fgPGID, s.shellPID = 2, 1
	now := time.Now()
	s.output([]byte("\x07\x1b]9;Build finished\x07\x1b]777;notify;make;Build finished\x07"))
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("after a bell and notifications: %s", st)
	}
}

// A program that is no agent works while its title turns as a spinner:
// a clock it draws, however long it ticks, is not work.
func TestProgramWorksByTitle(t *testing.T) {
	s := testPane(t)
	now := time.Now()
	for i := range 30 {
		s.output([]byte("\x1b[32;1H" + now.Format("05")))
		now = now.Add(time.Second)
		if st := s.status(now); st != proto.Idle {
			t.Fatalf("clock tick %d: %s", i, st)
		}
	}
	s.output([]byte("\x1b]0;⠋ building\x07"))
	if st := s.status(now); st != proto.Working {
		t.Fatalf("title turning: %s", st)
	}
	s.output([]byte("\x1b]0;done\x07"))
	if st := s.status(now); st != proto.Idle {
		t.Fatalf("title still: %s", st)
	}
}

// An agent's screen that stops showing work without showing its prompt is
// idle only once it has stayed so (idleHold).
func TestWorkingHeldBeforeIdle(t *testing.T) {
	s := testPane(t)
	s.info.Agent = "claude"
	now := time.Now()
	s.show(claudeScreen(t, "working"))
	if st := s.status(now); st != proto.Working {
		t.Fatalf("in a turn: %s", st)
	}
	s.show("⏺ Some answer")
	if st := s.status(now); st != proto.Working {
		t.Fatalf("between redraws: %s", st)
	}
	if st := s.status(now.Add(idleHold)); st != proto.Idle {
		t.Fatalf("after %s: %s", idleHold, st)
	}
}

// A restore resumes Claude Code's and Codex's sessions, and nothing a
// state file could slip in.
func TestResumeCommand(t *testing.T) {
	for _, c := range []struct {
		agent, id, want string
	}{
		{"claude", "13b997c3-f8b3-4b3e-bb8a-ca1b5d1b9787", "claude --resume 13b997c3-f8b3-4b3e-bb8a-ca1b5d1b9787"},
		{"codebuddy", "13b997c3-f8b3-4b3e-bb8a-ca1b5d1b9787", "codebuddy --resume 13b997c3-f8b3-4b3e-bb8a-ca1b5d1b9787"},
		{"codex", "019a2b3c-4d5e", "codex resume 019a2b3c-4d5e"},
		{"antigravity", "ec33ebf9-0cba-4100-8142-c61503f6c587", "agy --resume ec33ebf9-0cba-4100-8142-c61503f6c587"},
		{"pi", "0f3c2a1b-8e9d", "pi --session 0f3c2a1b-8e9d"},
		{"omp", "01a1a1cc-d32d", "omp --session 01a1a1cc-d32d"},
		{"pig", "02b2b2dd-e43e", "pig --session 02b2b2dd-e43e"},
		{"opencode", "ses_ea7b9b964ffemfrVYxK9yw55Gz", "opencode --session ses_ea7b9b964ffemfrVYxK9yw55Gz"},
		{"claude", "x; rm -rf ~", ""},
		{"claude", "", ""},
		{"vim", "abc", ""},
	} {
		got := strings.Join(resumeCommand(&proto.AgentSession{Agent: c.agent, ID: c.id}), " ")
		if got != c.want {
			t.Errorf("%s %q: got %q, want %q", c.agent, c.id, got, c.want)
		}
	}
	if resumeCommand(nil) != nil {
		t.Error("resumed nothing")
	}
}

// An agent's hook tells of the pane whose shell runs it; one that runs
// under none, as in Codex's app server that panes share, tells of the
// pane that can only be the agent's, and of none when several could be.
func TestAgentPane(t *testing.T) {
	t.Setenv("KERO_CONFIG", filepath.Join(t.TempDir(), "config"))
	d := &Daemon{changeCh: make(chan struct{}, 1), saveCh: make(chan struct{}, 1), panes: map[string]*pane{}}
	d.viewed.Store(new(string))
	add := func(id, agent, dir string, session bool) *pane {
		info := proto.Pane{ID: id, Agent: agent, Dir: dir, Status: proto.Idle, Cols: 80, Rows: 24}
		if session {
			info.AgentSession = &proto.AgentSession{Agent: agent, ID: "old-" + id}
		}
		p, err := newPane(d, "s", info)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.term.Close() })
		d.panes[id] = p
		return p
	}
	name := func(p *pane) string {
		if p == nil {
			return "none"
		}
		return p.info.ID
	}
	check := func(what string, pid int, session, dir, want string, wantAway bool) {
		t.Helper()
		p, away := d.agentPane(pid, "codex", session, dir)
		if name(p) != want || away != wantAway {
			t.Errorf("%s: %s away=%v, want %s away=%v", what, name(p), away, want, wantAway)
		}
	}

	a := add("a", "codex", "/repo", true)
	check("its only pane, telling a new session", 0, "", "", "a", true)
	check("external codex in another dir must not hijack a", 0, "ext-1", "/elsewhere", "none", true)
	b := add("b", "codex", "/repo", false)
	check("its only pane yet to tell", 0, "", "", "b", true)
	check("external codex in another dir must not claim pending b", 0, "ext-2", "/elsewhere", "none", true)
	add("c", "codex", "/other", false)
	check("of two yet to tell, the one in its dir", 0, "", "/other", "c", true)
	check("two yet to tell, in a dir neither is in", 0, "", "/worktree", "none", true)
	add("d", "claude", "/repo", false)
	check("two yet to tell, no dir", 0, "", "", "none", true)

	// Run under a pane's shell: that pane, whatever the others.
	a.shellPID = os.Getppid()
	check("under a's shell", os.Getpid(), "", "", "a", false)

	// Under Codex's app server that panes share, which a's Codex started
	// under a's shell: not a, which has its session, but the one pane yet
	// to tell its own (kero#237).
	if runtime.GOOS == "windows" {
		return
	}
	d.panes["c"].mu.Lock()
	d.panes["c"].info.Status = proto.Exited
	d.panes["c"].mu.Unlock()
	server := exec.Command("sh", "-c", "sleep 30; :", "codex", "app-server", "--listen", "unix://", "--managed-daemon")
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Process.Kill(); server.Wait() })
	// Its command line shows once it has exec'd, which Start may return
	// before.
	for i := 0; !sharedService(server.Process.Pid); i++ {
		if i == 5000 {
			t.Fatalf("the app server shows %q", procArgs(server.Process.Pid))
		}
		time.Sleep(time.Millisecond)
	}
	a.shellPID = os.Getpid()
	check("under the app server a's Codex started", server.Process.Pid, "", "", "b", true)

	// With two running Codex panes in different dirs, matching by session ID
	// (resume/compact) and by dir (/new) both disambiguate cleanly.
	b.mu.Lock()
	b.info.Dir = "/other"
	b.info.AgentSession = &proto.AgentSession{Agent: "codex", ID: "old-b"}
	b.mu.Unlock()
	check("two running panes, resume matches same session", server.Process.Pid, "old-b", "/elsewhere", "b", true)
	check("two running panes, /new matches by dir", server.Process.Pid, "new-b", "/other", "b", true)
	check("two running panes, external dir matches none", server.Process.Pid, "ext-3", "/elsewhere", "none", true)
}

// A restart that failed leaves the pane as it was: it does not say it
// restarted, it does not go idle with no shell to run, and it keeps the
// exit code it had — a wait for it would say it is idle, and it is not.
func TestRestartThatFailedLeavesThePaneAsItWas(t *testing.T) {
	dir := t.TempDir()
	// A shell that cannot be run: the file is there, and it may not be
	// run.
	shell := filepath.Join(dir, "shell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := testPane(t)
	t.Setenv("KERO_CONFIG", filepath.Join(dir, "config"))
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("command = "+shell+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.info.Status = proto.Exited
	s.info.ExitCode = 3
	s.mu.Unlock()
	if err := s.restart(); err == nil {
		t.Fatal("the restart succeeded with a shell that cannot be run")
	}
	s.mu.Lock()
	status, code := s.info.Status, s.info.ExitCode
	s.mu.Unlock()
	if status != proto.Exited {
		t.Errorf("the pane is %s, want it to stay exited", status)
	}
	if code != 3 {
		t.Errorf("the exit code = %d, want 3", code)
	}
	text := s.term.Text()
	if !strings.Contains(text, "[Restarted]") {
		t.Errorf("the scrollback does not say the restart was asked for:\n%s", text)
	}
	if !strings.Contains(text, "[Restart failed: ") {
		t.Errorf("the scrollback does not say the restart failed:\n%s", text)
	}
}

func TestDetectAgentAntigravity(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"agy", nil, "antigravity"},
		{"agy-dev", nil, "antigravity"},
		{"antigravity", nil, "antigravity"},
		{"antigravity-cli", nil, "antigravity"},
		{"node", []string{"node", "/Users/dev/.npm/antigravity-cli/bin/cli.js"}, "antigravity"},
		{"bash", []string{"bash", "-l"}, ""},
	} {
		if got := detectAgent(tc.name, tc.args); got != tc.want {
			t.Errorf("detectAgent(%q, %q) = %q, want %q", tc.name, tc.args, got, tc.want)
		}
	}
}

func TestPaneAgentTitleTracking(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	dir := filepath.Join(cfg, "projects", "-src-app")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(dir, "13b997c3.jsonl")

	s := testPane(t)
	s.mu.Lock()
	s.info.Process, s.info.Agent = "claude", "claude"
	s.info.Status = proto.Waiting
	s.mu.Unlock()

	// Approval input while Waiting must not become AgentTitle.
	s.input([]byte("allow once\r"))
	s.mu.Lock()
	if s.info.AgentTitle != "" {
		t.Fatalf("waiting input leaked into AgentTitle: %q", s.info.AgentTitle)
	}
	s.mu.Unlock()

	// First real prompt while Idle/Working sets AgentTitle.
	s.input([]byte("修复侧边栏会话标题展示\r"))
	s.mu.Lock()
	if s.info.AgentTitle != "修复侧边栏会话标题展示" || s.agentTitleKind != agentTitlePrompt {
		t.Fatalf("first prompt AgentTitle = (%q, %d)", s.info.AgentTitle, s.agentTitleKind)
	}
	s.mu.Unlock()

	// Subsequent prompts do not overwrite the first prompt title.
	s.input([]byte("second follow-up prompt\r"))
	s.mu.Lock()
	if s.info.AgentTitle != "修复侧边栏会话标题展示" {
		t.Fatalf("follow-up overwrote AgentTitle: %q", s.info.AgentTitle)
	}
	s.mu.Unlock()

	// /clear resets the prompt-derived title, allowing a new first prompt.
	s.input([]byte("/clear\r"))
	s.mu.Lock()
	if s.info.AgentTitle != "" || s.agentTitleKind != agentTitleNone {
		t.Fatalf("/clear did not reset AgentTitle: (%q, %d)", s.info.AgentTitle, s.agentTitleKind)
	}
	s.mu.Unlock()

	s.input([]byte("新的对话主题\r"))
	s.mu.Lock()
	if s.info.AgentTitle != "新的对话主题" {
		t.Fatalf("after /clear new prompt = %q", s.info.AgentTitle)
	}
	s.info.AgentSession = &proto.AgentSession{Agent: "claude", ID: "13b997c3"}
	s.mu.Unlock()

	// Claude transcript title overrides the prompt-derived title.
	if err := os.WriteFile(transcript, []byte(`{"type":"ai-title","aiTitle":"Refine sidebar title","sessionId":"13b997c3"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !s.syncClaudeTitle("13b997c3") {
		t.Fatal("syncClaudeTitle reported no change")
	}
	s.mu.Lock()
	if s.info.AgentTitle != "Refine sidebar title" || s.agentTitleKind != agentTitleTranscript {
		t.Fatalf("transcript AgentTitle = (%q, %d)", s.info.AgentTitle, s.agentTitleKind)
	}
	// Reset clears AgentTitle when agent ends.
	if !s.resetAgentTitleLocked() || s.info.AgentTitle != "" || s.agentTitleKind != agentTitleNone {
		t.Fatalf("resetAgentTitleLocked failed: (%q, %d)", s.info.AgentTitle, s.agentTitleKind)
	}
	s.mu.Unlock()
}

func TestTickClearsStaleRestoredAgentTitle(t *testing.T) {
	// Case 1: Restored command ended and fell back to shell before any tick saw proc != "".
	s := testPane(t)
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not installed")
	}
	proc, err := startPTY(ptyOptions{
		Path: shell, Args: []string{"sh", "-c", "sleep 30"},
		Dir: t.TempDir(), Env: []string{"TERM=xterm-256color"},
		Cols: s.info.Cols, Rows: s.info.Rows,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { proc.Kill() })
	s.mu.Lock()
	s.proc = proc
	s.shellPID = proc.PID()
	s.info.AgentTitle = "restored task"
	s.agentTitleKind = agentTitlePrompt
	s.info.AgentSession = &proto.AgentSession{Agent: "claude", ID: "13b997c3"}
	s.awaiting = false
	s.ended = true
	s.mu.Unlock()
	if changed := s.tick(time.Now()); !changed {
		t.Fatal("tick did not report change when clearing ended restored title")
	}
	s.mu.Lock()
	if s.info.AgentTitle != "" || s.agentTitleKind != agentTitleNone {
		t.Fatalf("ended restored pane kept AgentTitle = (%q, %d)", s.info.AgentTitle, s.agentTitleKind)
	}
	s.mu.Unlock()

	// Case 2: No AgentSession or mismatched AgentSession when an agent starts on a shell with stale title.
	s2 := testPane(t)
	s2.mu.Lock()
	s2.info.AgentTitle = "stale claude title"
	s2.agentTitleKind = agentTitlePrompt
	s2.info.AgentSession = &proto.AgentSession{Agent: "claude", ID: "13b997c3"}
	// Simulate tick transition where agent="antigravity" is detected while s2.info.Agent=="":
	// Here s2.proc is nil, so test the helper logic directly via resetAgentTitleLocked guard condition.
	agent := "antigravity"
	if agent != "" && agent != s2.info.Agent && s2.info.AgentTitle != "" && (s2.info.AgentSession == nil || s2.info.AgentSession.Agent != agent) {
		s2.resetAgentTitleLocked()
	}
	if s2.info.AgentTitle != "" || s2.agentTitleKind != agentTitleNone {
		t.Fatalf("mismatched agent kept stale AgentTitle = (%q, %d)", s2.info.AgentTitle, s2.agentTitleKind)
	}
	s2.mu.Unlock()
}

func TestPaneCodexTitleTracking(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("CODEX_HOME", cfg)
	indexPath := filepath.Join(cfg, "session_index.jsonl")
	id := "01a21ab6-f1da-7953-9036-9d5f0ff60e3a"

	s := testPane(t)
	s.mu.Lock()
	s.info.Process, s.info.Agent = "codex", "codex"
	s.info.AgentSession = &proto.AgentSession{Agent: "codex", ID: id}
	s.mu.Unlock()

	// First prompt sets prompt-derived AgentTitle while session_index has no title.
	s.input([]byte("帮我排查侧边栏 codex 会话名称\r"))
	s.mu.Lock()
	if s.info.AgentTitle != "帮我排查侧边栏 codex 会话名称" || s.agentTitleKind != agentTitlePrompt {
		s.mu.Unlock()
		t.Fatalf("first prompt AgentTitle = (%q, %d)", s.info.AgentTitle, s.agentTitleKind)
	}
	s.mu.Unlock()

	// Generic status string or path in session_index must not overwrite existing valid title.
	if err := os.WriteFile(indexPath, []byte(`{"id":"`+id+`","thread_name":"codex|kero|Ready|`+id+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s.syncAgentTitle("codex", id) {
		t.Fatal("generic codex status string overwrote AgentTitle")
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"`+id+`","thread_name":"/tmp/work"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s.syncAgentTitle("codex", id) {
		t.Fatal("path thread_name overwrote AgentTitle")
	}

	// Meaningful thread_name overrides the prompt-derived title.
	if err := os.WriteFile(indexPath, []byte(`{"id":"`+id+`","thread_name":"排查 Codex 会话标题"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !s.syncAgentTitle("codex", id) {
		t.Fatal("syncAgentTitle(codex) reported no change")
	}
	s.mu.Lock()
	if s.info.AgentTitle != "排查 Codex 会话标题" || s.agentTitleKind != agentTitleTranscript {
		s.mu.Unlock()
		t.Fatalf("synced AgentTitle = (%q, %d)", s.info.AgentTitle, s.agentTitleKind)
	}
	s.mu.Unlock()

	// Appending a newer thread_name updates the pane title on next sync.
	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"id":"` + id + `","thread_name":"Codex 标题已更新"}` + "\n"); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if !s.syncAgentTitle("codex", id) {
		t.Fatal("syncAgentTitle(codex) did not pick up appended thread_name")
	}
	s.mu.Lock()
	if s.info.AgentTitle != "Codex 标题已更新" || s.agentTitleKind != agentTitleTranscript {
		s.mu.Unlock()
		t.Fatalf("updated AgentTitle = (%q, %d)", s.info.AgentTitle, s.agentTitleKind)
	}
	s.mu.Unlock()

	// Verify tick() triggers Codex title sync automatically.
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not installed")
	}
	tp := testPane(t)
	proc, err := startPTY(ptyOptions{
		Path: shell, Args: []string{"sh", "-c", "sleep 30"},
		Dir: t.TempDir(), Env: []string{"TERM=xterm-256color"},
		Cols: tp.info.Cols, Rows: tp.info.Rows,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { proc.Kill() })
	tp.mu.Lock()
	tp.proc = proc
	tp.shellPID = proc.PID()
	tp.info.Agent = "codex"
	tp.info.AgentSession = &proto.AgentSession{Agent: "codex", ID: id}
	tp.mu.Unlock()
	if changed := tp.tick(time.Now()); !changed {
		t.Fatal("tick() did not report change when syncing codex title")
	}
	tp.mu.Lock()
	if tp.info.AgentTitle != "Codex 标题已更新" || tp.agentTitleKind != agentTitleTranscript {
		tp.mu.Unlock()
		t.Fatalf("tick synced AgentTitle = (%q, %d)", tp.info.AgentTitle, tp.agentTitleKind)
	}
	tp.mu.Unlock()
}

func TestPaneAgentSessionTitleRetention(t *testing.T) {
	d := testDaemon(t)
	p, err := newPane(d, "s1", proto.Pane{
		ID:     "p1",
		Dir:    d.dir,
		Agent:  "antigravity",
		Status: proto.Idle,
		Cols:   80,
		Rows:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.term.Close() })
	p.mu.Lock()
	p.info.AgentTitle = "首条提问标题"
	p.agentTitleKind = agentTitlePrompt
	p.mu.Unlock()
	d.sessions = []*session{{id: "s1", project: "pr", panes: []*pane{p}}}
	d.panes["p1"] = p

	call := func(agent, sid string) {
		t.Helper()
		raw, _ := json.Marshal(proto.AgentSessionParams{PID: 0, Agent: agent, Session: sid, Dir: d.dir})
		if _, err := d.call(nil, "pane.agentSession", raw); err != nil {
			t.Fatalf("pane.agentSession(%s, %s): %v", agent, sid, err)
		}
	}

	// First SessionStart for the same agent preserves the prompt-derived title.
	call("antigravity", "conv-1")
	p.mu.Lock()
	if p.info.AgentTitle != "首条提问标题" || p.agentTitleKind != agentTitlePrompt {
		p.mu.Unlock()
		t.Fatalf("first SessionStart cleared prompt title: (%q, %d)", p.info.AgentTitle, p.agentTitleKind)
	}
	p.mu.Unlock()

	// Switching to a different conversation resets the old title.
	call("antigravity", "conv-2")
	p.mu.Lock()
	if p.info.AgentTitle != "" || p.agentTitleKind != agentTitleNone {
		p.mu.Unlock()
		t.Fatalf("new conversation kept old title: (%q, %d)", p.info.AgentTitle, p.agentTitleKind)
	}
	p.mu.Unlock()
}

func TestProbeAgentLockedClearsTickRace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sh")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not installed")
	}
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	codexBin := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexBin, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := testDaemon(t)
	p, err := newPane(d, "s1", proto.Pane{ID: "p1", Dir: dir, Status: proto.Idle, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.close)
	proc, err := startPTY(ptyOptions{
		Path: shell,
		Args: []string{"sh", "-c", `set -m; "$1"`, "sh", codexBin},
		Dir:  dir,
		Env:  []string{"TERM=xterm-256color"},
		Cols: 80,
		Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { proc.Kill() })
	go p.pump(proc)
	p.mu.Lock()
	p.proc = proc
	p.shellPID = proc.PID()
	p.mu.Unlock()
	d.panes["p1"] = p

	// Before any tick() runs, wait until codex is foreground in the PTY, then
	// verify agentPane probes it immediately.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		matched, away := d.agentPane(0, "codex", "sess-fast", dir)
		if matched == p && away {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("agentPane did not probe foreground codex before tick")
}

func TestResumeSessionFromArgs(t *testing.T) {
	cases := []struct {
		name  string
		agent string
		args  []string
		want  string
	}{
		{"codex plain", "codex", []string{"codex", "resume", "019a-codex"}, "019a-codex"},
		{"codex node wrapper", "codex", []string{"node", "/usr/lib/node_modules/@openai/codex/bin/codex.js", "resume", "019a-codex"}, "019a-codex"},
		{"codex leading flags", "codex", []string{"codex", "-m", "gpt-5", "--cd", "/repo", "resume", "--all", "019a-codex"}, "019a-codex"},
		{"codex last with prompt", "codex", []string{"codex", "resume", "--last", "fix"}, ""},
		{"codex new session", "codex", []string{"codex", "--cd", "/repo"}, ""},
		{"antigravity resume", "antigravity", []string{"agy", "--resume", "conv-1"}, "conv-1"},
		{"antigravity short r", "antigravity", []string{"agy", "-r", "conv-2"}, "conv-2"},
		{"antigravity conversation eq", "antigravity", []string{"agy", "--conversation=conv-3"}, "conv-3"},
		{"claude resume", "claude", []string{"claude", "--resume", "claude-1"}, "claude-1"},
		{"claude session-id", "claude", []string{"claude", "--session-id", "claude-2"}, "claude-2"},
		{"codebuddy short r", "codebuddy", []string{"codebuddy", "-r", "cb-1"}, "cb-1"},
		{"pi session", "pi", []string{"pi", "--session", "pi-1"}, "pi-1"},
		{"opencode session", "opencode", []string{"opencode", "--session=oc-1"}, "oc-1"},
		{"invalid id", "codex", []string{"codex", "resume", "../bad"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resumeSessionFromArgs(tc.agent, tc.args)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			if got == nil || got.Agent != tc.agent || got.ID != tc.want {
				t.Fatalf("got %+v, want (%s, %s)", got, tc.agent, tc.want)
			}
		})
	}
}

func TestTickInfersResumeSessionAndPreservesHookUpdate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sh")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not installed")
	}
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	codexBin := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexBin, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := testDaemon(t)
	p, err := newPane(d, "s1", proto.Pane{ID: "p1", Dir: dir, Status: proto.Idle, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.close)
	proc, err := startPTY(ptyOptions{
		Path: shell,
		Args: []string{"sh", "-c", `set -m; "$1" resume 019a-resumed`, "sh", codexBin},
		Dir:  dir,
		Env:  []string{"TERM=xterm-256color"},
		Cols: 80,
		Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { proc.Kill() })
	go p.pump(proc)
	p.mu.Lock()
	p.proc = proc
	p.shellPID = proc.PID()
	p.mu.Unlock()
	d.sessions = []*session{{id: "s1", project: "pr", panes: []*pane{p}}}
	d.panes["p1"] = p

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		p.tick(time.Now())
		p.mu.Lock()
		as := p.info.AgentSession
		p.mu.Unlock()
		if as != nil {
			if as.Agent != "codex" || as.ID != "019a-resumed" {
				t.Fatalf("inferred AgentSession = %+v", as)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	p.mu.Lock()
	if p.info.AgentSession == nil {
		p.mu.Unlock()
		t.Fatal("tick did not infer AgentSession from codex resume args")
	}
	p.mu.Unlock()

	raw, _ := json.Marshal(proto.AgentSessionParams{Agent: "codex", Session: "019a-cleared", Dir: dir})
	if _, err := d.call(nil, "pane.agentSession", raw); err != nil {
		t.Fatalf("pane.agentSession: %v", err)
	}
	p.tick(time.Now())
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.info.AgentSession == nil || p.info.AgentSession.ID != "019a-cleared" {
		t.Fatalf("tick overwrote hook-reported AgentSession: %+v", p.info.AgentSession)
	}
}

func TestPaneAgentSessionLogsUnmatched(t *testing.T) {
	var logs bytes.Buffer
	d := testDaemon(t)
	d.log = log.New(&logs, "", 0)

	raw, _ := json.Marshal(proto.AgentSessionParams{PID: 1234, Agent: "codex", Session: "secret-conv-id", Dir: "/tmp/nope"})
	if _, err := d.call(nil, "pane.agentSession", raw); err == nil {
		t.Fatal("expected unmatched error")
	}
	out := logs.String()
	if !strings.Contains(out, "pane.agentSession unmatched") || !strings.Contains(out, `agent="codex"`) {
		t.Fatalf("unexpected log output: %q", out)
	}
	if strings.Contains(out, "secret-conv-id") || strings.Contains(out, "/tmp/nope") {
		t.Fatalf("log leaked session ID or path: %q", out)
	}
}
