package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kero/internal/proto"
)

func TestHandoff(t *testing.T) {
	d := testDaemon(t)
	codex := t.TempDir()
	t.Setenv("CODEX_HOME", codex)
	transcript := filepath.Join(codex, "sessions", "2026", "11", "27", "rollout-2026-11-27T20-00-00-019abc.jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := d.projects[0].Dir
	ses, err := d.create(proto.CreateParams{Project: "pr", Name: "fix-login", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(ses.ID) })

	if _, err := d.handoff(proto.HandoffParams{ID: ses.ID, Agent: "claude"}); err == nil {
		t.Error("handed over a shell")
	}
	p := d.findPane(ses.Panes[0].ID)
	p.mu.Lock()
	p.info.Agent = "codex"
	p.info.AgentSession = &proto.AgentSession{Agent: "codex", ID: "019abc"}
	p.mu.Unlock()
	if _, err := d.handoff(proto.HandoffParams{ID: ses.ID, Agent: "codex"}); err == nil {
		t.Error("handed over to itself")
	}
	if _, err := d.handoff(proto.HandoffParams{ID: ses.ID, Agent: "rm"}); err == nil {
		t.Error("handed over to an unknown agent")
	}

	s, err := d.handoff(proto.HandoffParams{ID: ses.ID, Agent: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(s.ID) })
	if s.Project != "pr" || s.Name != "fix-login" || !s.AgentNamed || len(s.Panes) != 1 {
		t.Fatalf("handed over to %+v", s)
	}
	cmd := s.Panes[0].Command
	if len(cmd) != 2 || cmd[0] != "claude" || s.Panes[0].Dir != dir {
		t.Fatalf("pane %+v", s.Panes[0])
	}
	for _, want := range []string{"Codex", transcript, dir, "git status"} {
		if !strings.Contains(cmd[1], want) {
			t.Errorf("prompt %q lacks %q", cmd[1], want)
		}
	}
	if strings.Contains(cmd[1], "\n") {
		t.Errorf("prompt %q spans lines", cmd[1])
	}
	// After the session handed over, which goes on.
	d.mu.Lock()
	order := []string{}
	for _, x := range d.sessions {
		order = append(order, x.id)
	}
	d.mu.Unlock()
	if strings.Join(order, ",") != ses.ID+","+s.ID {
		t.Errorf("sessions %v", order)
	}
}

func TestHandoffCommand(t *testing.T) {
	for agent, want := range map[string]string{
		"claude":      "claude P",
		"codex":       "codex P",
		"antigravity": "agy -i P",
		"opencode":    "opencode --prompt P",
		"pi":          "pi P",
		"codebuddy":   "codebuddy P",
	} {
		if got := strings.Join(handoffCommand(agent, "P"), " "); got != want {
			t.Errorf("%s: %q, want %q", agent, got, want)
		}
	}
}

func TestHandoffPromptWithoutTranscript(t *testing.T) {
	got := handoffPrompt(handoffSource{agent: "opencode", id: "ses_1"})
	if !strings.Contains(got, "OpenCode") || !strings.Contains(got, "ses_1") || strings.Contains(got, "transcript") {
		t.Errorf("prompt %q", got)
	}
	if transcriptPath("codex", "../x") != "" || transcriptPath("opencode", "ses_1") != "" {
		t.Error("found a transcript")
	}
}

func TestHandoffMultiPaneAndTitlePriority(t *testing.T) {
	d := testDaemon(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := d.projects[0].Dir

	// Split session: focused pane is plain shell, second pane runs codex.
	ses, err := d.create(proto.CreateParams{Project: "pr", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(ses.ID) })
	splitSes, err := d.split(proto.CreateParams{Split: ses.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(splitSes.Panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(splitSes.Panes))
	}
	p1 := d.findPane(splitSes.Panes[0].ID)
	p2 := d.findPane(splitSes.Panes[1].ID)
	d.mu.Lock()
	ds := d.findSessionLocked(ses.ID)
	ds.focus = p1.info.ID
	d.mu.Unlock()

	p2.mu.Lock()
	p2.info.Agent = "codex"
	p2.info.AgentSession = &proto.AgentSession{Agent: "codex", ID: "019codex"}
	p2.mu.Unlock()

	hs1, err := d.handoff(proto.HandoffParams{ID: ses.ID, Agent: "pi"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(hs1.ID) })
	if !strings.Contains(hs1.Panes[0].Command[1], "Codex") || !strings.Contains(hs1.Panes[0].Command[1], "019codex") {
		t.Errorf("expected codex handoff when focused is shell, got %+v", hs1.Panes[0].Command)
	}

	// Focused pane runs antigravity, second pane runs codex: focused wins.
	p1.mu.Lock()
	p1.info.Agent = "antigravity"
	p1.info.AgentSession = &proto.AgentSession{Agent: "antigravity", ID: "agy-123"}
	p1.mu.Unlock()

	hs2, err := d.handoff(proto.HandoffParams{ID: ses.ID, Agent: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(hs2.ID) })
	if !strings.Contains(hs2.Panes[0].Command[1], "Antigravity") || !strings.Contains(hs2.Panes[0].Command[1], "agy-123") {
		t.Errorf("expected focused antigravity handoff, got %+v", hs2.Panes[0].Command)
	}

	// Explicit session name takes precedence over AgentTitle.
	namedSes, err := d.create(proto.CreateParams{Project: "pr", Name: "my-task", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(namedSes.ID) })
	np := d.findPane(namedSes.Panes[0].ID)
	np.mu.Lock()
	np.info.Agent = "codex"
	np.info.AgentTitle = "first prompt"
	np.mu.Unlock()

	hs3, err := d.handoff(proto.HandoffParams{ID: namedSes.ID, Agent: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(hs3.ID) })
	if hs3.Name != "my-task" || !strings.Contains(hs3.Panes[0].Command[1], `"my-task"`) {
		t.Errorf("expected explicit name my-task, got name=%q cmd=%+v", hs3.Name, hs3.Panes[0].Command)
	}

	// Empty session name and p.info.Agent=="" with AgentSession & AgentTitle resolves AgentTitle.
	agentOnlySes, err := d.create(proto.CreateParams{Project: "pr", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(agentOnlySes.ID) })
	ap := d.findPane(agentOnlySes.Panes[0].ID)
	ap.mu.Lock()
	ap.info.Agent = ""
	ap.info.AgentSession = &proto.AgentSession{Agent: "claude", ID: "abc123"}
	ap.info.AgentTitle = "refactor auth"
	ap.mu.Unlock()

	hs4, err := d.handoff(proto.HandoffParams{ID: agentOnlySes.ID, Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(hs4.ID) })
	if hs4.Name != "refactor auth" || !strings.Contains(hs4.Panes[0].Command[1], `"refactor auth"`) {
		t.Errorf("expected AgentTitle refactor auth, got name=%q cmd=%+v", hs4.Name, hs4.Panes[0].Command)
	}
}

func TestHandoffDirGone(t *testing.T) {
	d := testDaemon(t)
	dir := d.projects[0].Dir
	ses, err := d.create(proto.CreateParams{Project: "pr", Name: "fix-login", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(ses.ID) })

	gone := filepath.Join(t.TempDir(), "gone")
	p := d.findPane(ses.Panes[0].ID)
	p.mu.Lock()
	p.info.Agent = "codex"
	p.info.Dir = gone
	p.info.ProcessDir = ""
	p.mu.Unlock()

	if _, err := d.handoff(proto.HandoffParams{ID: ses.ID, Agent: "claude"}); err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("expected gone error, got %v", err)
	}
	d.mu.Lock()
	n := len(d.sessions)
	d.mu.Unlock()
	if n != 1 {
		t.Errorf("expected 1 session, got %d", n)
	}
}

func TestHandoffTranscriptPaths(t *testing.T) {
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	claudeFile := filepath.Join(claudeDir, "projects", "kero", "c123.jsonl")
	if err := os.MkdirAll(filepath.Dir(claudeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	piDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", piDir)
	piFile := filepath.Join(piDir, "sessions", "kero", "2026-11-27_pi123.jsonl")
	if err := os.MkdirAll(filepath.Dir(piFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(piFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	codexDir := t.TempDir()
	t.Setenv("CODEX_HOME", codexDir)
	codexFile := filepath.Join(codexDir, "sessions", "2026", "11", "27", "rollout-2026-11-27T20-00-00-codex123.jsonl")
	if err := os.MkdirAll(filepath.Dir(codexFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	agyFile := filepath.Join(homeDir, ".gemini", "antigravity-cli", "brain", "agy123", ".system_generated", "logs", "transcript.jsonl")
	if err := os.MkdirAll(filepath.Dir(agyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agyFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		agent, id, want string
	}{
		{"claude", "c123", claudeFile},
		{"pi", "pi123", piFile},
		{"codex", "codex123", codexFile},
		{"antigravity", "agy123", agyFile},
		{"claude", "../escape", ""},
		{"antigravity", "../escape", ""},
		{"pi", "", ""},
		{"opencode", "oc123", ""},
	} {
		if got := transcriptPath(tc.agent, tc.id); got != tc.want {
			t.Errorf("transcriptPath(%q, %q) = %q, want %q", tc.agent, tc.id, got, tc.want)
		}
	}
}
