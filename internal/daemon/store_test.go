package daemon

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kero/internal/proto"
)

// A history that could not be written is the pane's still: it is saved
// again the next time, and the scrollback is not lost.
func TestFailedHistorySaveKeepsIt(t *testing.T) {
	d := testDaemon(t)
	p, err := newPane(d, "s1", proto.Pane{ID: "a", Status: proto.Idle, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.term.Close() })
	d.panes["a"] = p
	p.show("hello")
	// The history cannot be written: the path is a directory.
	if err := os.MkdirAll(d.historyPath("a"), 0o755); err != nil {
		t.Fatal(err)
	}
	d.saveHistories(false)
	p.mu.Lock()
	dirty := p.dirty
	p.mu.Unlock()
	if !dirty {
		t.Fatal("the history is not the pane's any more")
	}
	// Written, it is saved and the pane is clean.
	os.Remove(d.historyPath("a"))
	d.saveHistories(false)
	if b, err := os.ReadFile(d.historyPath("a")); err != nil || len(b) == 0 {
		t.Errorf("the history written: %v", err)
	}
	p.mu.Lock()
	dirty = p.dirty
	p.mu.Unlock()
	if dirty {
		t.Error("the history was saved and the pane is still dirty")
	}
}

// A history that cannot be taken away when its pane closes is said: the
// pane is gone, nothing will try again, and the file would stay without
// a word.
func TestHistoryThatCannotBeRemovedIsSaid(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root takes a file from a folder that cannot be written")
	}
	var logs bytes.Buffer
	d := testDaemon(t)
	d.log = log.New(&logs, "", 0)
	p, err := newPane(d, "s1", proto.Pane{ID: "a", Status: proto.Idle, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.term.Close() })
	d.panes["a"] = p
	p.show("hello")
	if err := os.MkdirAll(filepath.Join(d.dir, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	d.saveHistories(false)
	if _, err := os.Stat(d.historyPath("a")); err != nil {
		t.Fatal(err)
	}
	// The folder the histories are kept in takes no file away.
	sessions := filepath.Join(d.dir, "sessions")
	if err := os.Chmod(sessions, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(sessions, 0o700) })
	raw, err := json.Marshal(proto.IDParams{ID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.call(nil, "pane.close", raw); err != nil {
		t.Fatalf("pane.close: %v", err)
	}
	if _, err := os.Stat(d.historyPath("a")); err != nil {
		t.Errorf("the history was taken away after all: %v", err)
	}
	if !strings.Contains(logs.String(), "the history of a") {
		t.Errorf("a history that could not be removed was not said: %q", logs.String())
	}
}

// A state that could not be written is said: the sessions it holds are
// the ones the next daemon starts again, and a lost write is lost for
// good.
func TestFailedStateSaveIsSaid(t *testing.T) {
	d := testDaemon(t)
	var buf bytes.Buffer
	d.log = log.New(&buf, "", 0)
	// The path of the state is a directory: the write cannot put it
	// there.
	if err := os.MkdirAll(d.statePath(), 0o755); err != nil {
		t.Fatal(err)
	}
	d.saveState()
	if !strings.Contains(buf.String(), "state") {
		t.Fatalf("the failed write was not said: %q", buf.String())
	}
}

// A pane's history that could not be read is said: the pane restores
// without its scrollback, and the log names it. A pane without a history
// at all — none was saved — is no news.
func TestUnreadableHistoryIsSaid(t *testing.T) {
	d := testDaemon(t)
	var buf bytes.Buffer
	d.log = log.New(&buf, "", 0)
	saved := savedState{Version: 2, Projects: d.projects, Sessions: []savedSession{{
		ID: "s1", Project: "pr", Name: "s1",
		Panes: []savedPane{{ID: "p1", Dir: d.dir, Cols: 80, Rows: 24}},
	}}}
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.statePath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	// The history cannot be read: the path is a directory.
	if err := os.MkdirAll(d.historyPath("p1"), 0o755); err != nil {
		t.Fatal(err)
	}
	d.restore()
	if !strings.Contains(buf.String(), "history") {
		t.Fatalf("the unreadable history was not said: %q", buf.String())
	}
	// And the pane restored either way, without its scrollback.
	if len(d.sessions) != 1 || len(d.sessions[0].panes) != 1 {
		t.Fatalf("restored %d sessions", len(d.sessions))
	}
}

// A state that could not be read is said: the sessions of the last
// daemon are gone, and nobody is told.
func TestUnreadableStateIsSaid(t *testing.T) {
	d := testDaemon(t)
	var buf bytes.Buffer
	d.log = log.New(&buf, "", 0)
	if err := os.WriteFile(d.statePath(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	d.restore()
	if !strings.Contains(buf.String(), "state") {
		t.Fatalf("the unreadable state was not said: %q", buf.String())
	}
}

// Two saves of the same file at once do not leave it half of each: the
// save loop's and a stop's, which both write state.json's .tmp, run one
// at a time.
func TestSavesDoNotOverlap(t *testing.T) {
	d := testDaemon(t)
	var inside sync.Mutex
	var running, overlapped int
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.saveSerialized(func() {
				inside.Lock()
				running++
				if running > 1 {
					overlapped++
				}
				inside.Unlock()
				time.Sleep(5 * time.Millisecond)
				inside.Lock()
				running--
				inside.Unlock()
			})
		}()
	}
	wg.Wait()
	if overlapped > 0 {
		t.Errorf("%d saves ran at once", overlapped)
	}
}

func TestRestoreAgentTitleKind(t *testing.T) {
	d := testDaemon(t)
	p, err := newPane(d, "s1", proto.Pane{
		ID:           "p1",
		Dir:          d.dir,
		Agent:        "codex",
		AgentSession: &proto.AgentSession{Agent: "codex", ID: "019a2b3c-4d5e"},
		AgentTitle:   "修复重启后标题",
		Status:       proto.Idle,
		Cols:         80,
		Rows:         24,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.term.Close() })
	p.agentTitleKind = agentTitlePrompt
	d.sessions = []*session{{id: "s1", project: "pr", panes: []*pane{p}}}
	d.panes["p1"] = p
	d.saveState()

	data, err := os.ReadFile(d.statePath())
	if err != nil {
		t.Fatal(err)
	}
	var st savedState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Sessions) != 1 || len(st.Sessions[0].Panes) != 1 {
		t.Fatalf("saved state: %+v", st)
	}
	sp := st.Sessions[0].Panes[0]
	if sp.AgentTitle != "修复重启后标题" || sp.AgentTitleKind != agentTitlePrompt {
		t.Fatalf("saved pane = (%q, %d), want (%q, %d)", sp.AgentTitle, sp.AgentTitleKind, "修复重启后标题", agentTitlePrompt)
	}

	// Restore with saved AgentTitleKind: /clear resets AgentTitle and a new prompt sets it.
	d2 := testDaemon(t)
	if err := os.WriteFile(d2.statePath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	d2.restore()
	t.Cleanup(func() { _ = d2.removeSession("s1") })
	rp := d2.findPane("p1")
	if rp == nil {
		t.Fatal("p1 was not restored")
	}
	rp.mu.Lock()
	rp.info.Agent = "codex"
	if rp.info.AgentTitle != "修复重启后标题" || rp.agentTitleKind != agentTitlePrompt {
		rp.mu.Unlock()
		t.Fatalf("restored = (%q, %d)", rp.info.AgentTitle, rp.agentTitleKind)
	}
	rp.mu.Unlock()

	rp.input([]byte("/clear\r"))
	rp.mu.Lock()
	if rp.info.AgentTitle != "" || rp.agentTitleKind != agentTitleNone {
		rp.mu.Unlock()
		t.Fatalf("after /clear on restored pane: (%q, %d)", rp.info.AgentTitle, rp.agentTitleKind)
	}
	rp.mu.Unlock()

	rp.input([]byte("新的首条提问\r"))
	rp.mu.Lock()
	if rp.info.AgentTitle != "新的首条提问" || rp.agentTitleKind != agentTitlePrompt {
		rp.mu.Unlock()
		t.Fatalf("after new prompt on restored pane: (%q, %d)", rp.info.AgentTitle, rp.agentTitleKind)
	}
	rp.mu.Unlock()

	// Legacy state without AgentTitleKind defaults to agentTitlePrompt so /clear still works.
	d3 := testDaemon(t)
	st.Sessions[0].Panes[0].AgentTitleKind = agentTitleNone
	legacyData, _ := json.Marshal(st)
	if err := os.WriteFile(d3.statePath(), legacyData, 0o600); err != nil {
		t.Fatal(err)
	}
	d3.restore()
	t.Cleanup(func() { _ = d3.removeSession("s1") })
	rp3 := d3.findPane("p1")
	if rp3 == nil {
		t.Fatal("legacy p1 was not restored")
	}
	rp3.mu.Lock()
	rp3.info.Agent = "codex"
	if rp3.info.AgentTitle != "修复重启后标题" || rp3.agentTitleKind != agentTitlePrompt {
		rp3.mu.Unlock()
		t.Fatalf("legacy restored = (%q, %d)", rp3.info.AgentTitle, rp3.agentTitleKind)
	}
	rp3.mu.Unlock()

	// Unsupported resume command or resumeAgents=false must not restore AgentTitle.
	d4 := testDaemon(t)
	rp4 := d4.restorePane("s2", savedPane{
		ID:             "p4",
		Dir:            d4.dir,
		Agent:          &proto.AgentSession{Agent: "antigravity", ID: "abc"},
		AgentTitle:     "should not restore",
		AgentTitleKind: agentTitlePrompt,
		Cols:           80,
		Rows:           24,
	}, "now", true)
	if rp4 == nil {
		t.Fatal("rp4 was nil")
	}
	t.Cleanup(rp4.close)
	rp4.mu.Lock()
	if rp4.info.AgentTitle != "" || rp4.agentTitleKind != agentTitleNone {
		rp4.mu.Unlock()
		t.Fatalf("unsupported resume kept AgentTitle = (%q, %d)", rp4.info.AgentTitle, rp4.agentTitleKind)
	}
	rp4.mu.Unlock()

	rp5 := d4.restorePane("s3", savedPane{
		ID:             "p5",
		Dir:            d4.dir,
		Agent:          &proto.AgentSession{Agent: "codex", ID: "019a2b3c-4d5e"},
		AgentTitle:     "disabled resume",
		AgentTitleKind: agentTitlePrompt,
		Cols:           80,
		Rows:           24,
	}, "now", false)
	if rp5 == nil {
		t.Fatal("rp5 was nil")
	}
	t.Cleanup(rp5.close)
	rp5.mu.Lock()
	if rp5.info.AgentTitle != "" || rp5.agentTitleKind != agentTitleNone {
		rp5.mu.Unlock()
		t.Fatalf("disabled resume kept AgentTitle = (%q, %d)", rp5.info.AgentTitle, rp5.agentTitleKind)
	}
	rp5.mu.Unlock()
}
