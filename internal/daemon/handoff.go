package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kero/internal/proto"
)

// handoffSource is what an agent's task is handed over with: the agent,
// its conversation when known, where it worked and what it was about.
type handoffSource struct {
	agent, id, dir, title, transcript string
}

// handoff starts the agent p.Agent in a new session after p.ID's, in the
// directory the session's agent worked in, told to read what the other
// agent did and carry its task on. The session handed over goes on as it
// was: the user closes it when done with it.
func (d *Daemon) handoff(p proto.HandoffParams) (proto.Session, error) {
	if !slices.Contains(proto.HandoffAgents, p.Agent) {
		return proto.Session{}, fmt.Errorf("cannot hand over to %q", p.Agent)
	}
	d.mu.Lock()
	s := d.findSessionLocked(p.ID)
	if s == nil {
		d.mu.Unlock()
		return proto.Session{}, fmt.Errorf("no session %q", p.ID)
	}
	src, ok := handoffSourceOf(s)
	sid, project, name := s.id, s.project, s.name
	d.mu.Unlock()
	if !ok {
		return proto.Session{}, fmt.Errorf("no agent runs in session %q", p.ID)
	}
	if src.agent == p.Agent {
		return proto.Session{}, fmt.Errorf("%s already has the task", proto.AgentNames[p.Agent])
	}
	if name != "" {
		src.title = name
	}
	if src.dir != "" {
		st, err := os.Stat(src.dir)
		if err != nil {
			if os.IsNotExist(err) {
				return proto.Session{}, fmt.Errorf("the session's directory %s is gone", proto.ShortDir(src.dir))
			}
			return proto.Session{}, err
		}
		if !st.IsDir() {
			return proto.Session{}, fmt.Errorf("the session's directory %s is gone", proto.ShortDir(src.dir))
		}
	}
	src.transcript = transcriptPath(src.agent, src.id)
	info, err := d.create(proto.CreateParams{
		Project: project, Name: src.title, Dir: src.dir,
		Command: handoffCommand(p.Agent, handoffPrompt(src)), After: sid, Cols: p.Cols, Rows: p.Rows,
	})
	if err != nil {
		return info, err
	}
	if src.title != "" {
		// The task's name, for the new agent to rename as its own.
		d.mu.Lock()
		if ns := d.findSessionLocked(info.ID); ns != nil {
			ns.agentNamed = true
			info = d.sessionInfo(ns)
		}
		d.mu.Unlock()
		d.changed()
	}
	d.log.Printf("session %s hands %s's task in session %s over to %s", info.ID, src.agent, sid, p.Agent)
	return info, nil
}

// handoffSourceOf is the agent of a session's to hand over: the focused
// pane's, else the first pane's that has one. d.mu is held.
func handoffSourceOf(s *session) (handoffSource, bool) {
	var found handoffSource
	for _, pn := range s.panes {
		pn.mu.Lock()
		info := pn.info
		pn.mu.Unlock()
		agent := info.Agent
		id := ""
		if as := info.AgentSession; as != nil && (agent == "" || agent == as.Agent) {
			agent, id = as.Agent, as.ID
		}
		if agent == "" {
			continue
		}
		info.Agent = agent
		src := handoffSource{agent: agent, id: id, dir: info.WorkDir(), title: info.AgentConversationTitle()}
		if info.ID == s.focus {
			return src, true
		}
		if found.agent == "" {
			found = src
		}
	}
	return found, found.agent != ""
}

// transcriptPath is where an agent keeps a conversation's transcript, ""
// when unknown.
func transcriptPath(agent, id string) string {
	if id == "" || !agentSessionRe.MatchString(id) {
		return ""
	}
	var (
		path string
		err  error
	)
	switch agent {
	case "claude":
		path, _, err = claudeTranscript(id)
	case "pi":
		path, _, err = piTranscript(id)
	case "codex":
		dir := os.Getenv("CODEX_HOME")
		if dir == "" {
			dir = filepath.Join(proto.Home(), ".codex")
		}
		path, _, err = findTranscript(id, filepath.Join(dir, "sessions", "*", "*", "*", "rollout-*"+id+".jsonl"))
	case "antigravity":
		path, _, err = findTranscript(id, filepath.Join(proto.Home(), ".gemini", "antigravity-cli", "brain", id, ".system_generated", "logs", "transcript.jsonl"))
	default:
		return ""
	}
	if err != nil {
		return ""
	}
	return path
}

// handoffPrompt tells the agent taking a task over where it was left. One
// line: every shell, cmd.exe's too, passes it as one argument.
func handoffPrompt(src handoffSource) string {
	from := proto.AgentNames[src.agent]
	if from == "" {
		from = src.agent
	}
	var b strings.Builder
	fmt.Fprintf(&b, "You are taking over a task from another coding agent, %s.", from)
	if src.title != "" {
		fmt.Fprintf(&b, " The task: %q.", src.title)
	}
	if src.dir != "" {
		fmt.Fprintf(&b, " It worked in %s.", src.dir)
	}
	switch {
	case src.transcript != "":
		fmt.Fprintf(&b, " Its conversation transcript is at %s; read it, the latest part first, to learn what was asked, decided and done.", src.transcript)
	case src.id != "":
		fmt.Fprintf(&b, " Its conversation ID is %s.", src.id)
	}
	b.WriteString(" Check the working tree's state (git status, git diff) to see the changes made so far. Then summarize briefly where the task stands and carry it on; ask me if what is left is unclear.")
	return b.String()
}

// handoffCommand starts agent in its interactive mode with prompt as its
// first message.
func handoffCommand(agent, prompt string) []string {
	switch agent {
	case "antigravity":
		return []string{"agy", "-i", prompt}
	case "opencode":
		return []string{"opencode", "--prompt", prompt}
	}
	return []string{agent, prompt}
}
