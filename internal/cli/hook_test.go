package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kero/internal/proto"
)

func testHook(t *testing.T, settings string) agentHook {
	t.Helper()
	t.Setenv("KERO_HOME", filepath.Join(t.TempDir(), "kero home"))
	h := agentHook{"claude", filepath.Join(t.TempDir(), "settings.json")}
	if settings != "" {
		if err := os.WriteFile(h.file, []byte(settings), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Installing keeps the rest of the settings, in their order, and the
// other hooks; installing again changes nothing; removing takes only
// kero's out.
func TestHookInstallKeepsSettings(t *testing.T) {
	h := testHook(t, `{
  "model": "opus",
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "say done"}]}],
    "SessionStart": [{"matcher": "startup", "hooks": [{"type": "command", "command": "echo hi"}]}]
  },
  "env": {"A": "1"}
}
`)
	if st := h.state(); st != "missing" {
		t.Fatalf("state %s before install", st)
	}
	if err := h.install(); err != nil {
		t.Fatal(err)
	}
	got := read(t, h.file)
	if h.state() != "current" {
		t.Fatalf("state %s after install:\n%s", h.state(), got)
	}
	if i, j, k := strings.Index(got, `"model"`), strings.Index(got, `"hooks"`), strings.Index(got, `"env"`); !(i < j && j < k) {
		t.Fatalf("keys out of order:\n%s", got)
	}
	for _, want := range []string{`"say done"`, `"echo hi"`, `"matcher": "startup"`, `"timeout": 10`, "kero home/bin/kero' hook claude"} {
		if !strings.Contains(got, want) {
			t.Fatalf("no %s in:\n%s", want, got)
		}
	}
	if err := h.install(); err != nil {
		t.Fatal(err)
	}
	if again := read(t, h.file); again != got {
		t.Fatalf("installing again changed it:\n%s", again)
	}
	if err := h.remove(); err != nil {
		t.Fatal(err)
	}
	removed := read(t, h.file)
	if strings.Contains(removed, "kero") || !strings.Contains(removed, `"echo hi"`) || !strings.Contains(removed, `"say done"`) {
		t.Fatalf("after remove:\n%s", removed)
	}
	if h.state() != "missing" {
		t.Fatalf("state %s after remove", h.state())
	}
}

// Another kero's hook, as of a moved KERO_HOME, is outdated, and setup
// replaces it.
func TestHookReplacesOutdated(t *testing.T) {
	h := testHook(t, `{"hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "'/old/kero/bin/kero' hook claude", "timeout": 10}]}]}}`)
	if st := h.state(); st != "outdated" {
		t.Fatalf("state %s, want outdated", st)
	}
	if err := h.install(); err != nil {
		t.Fatal(err)
	}
	got := read(t, h.file)
	if strings.Contains(got, "/old/kero") || strings.Count(got, "hook claude") != 1 {
		t.Fatalf("after install:\n%s", got)
	}
}

// Without a settings file, install makes one, and remove takes away what
// it made.
func TestHookNewFile(t *testing.T) {
	h := testHook(t, "")
	if err := h.install(); err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(read(t, h.file)), &v); err != nil {
		t.Fatal(err)
	}
	if err := h.remove(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(read(t, h.file)); got != "{}" {
		t.Fatalf("after remove: %s", got)
	}
}

// Settings that aren't JSON are left alone.
func TestHookRefusesBadSettings(t *testing.T) {
	h := testHook(t, "{ // a comment\n}")
	if err := h.install(); err == nil {
		t.Fatal("installed into settings it can't read")
	}
	if got := read(t, h.file); got != "{ // a comment\n}" {
		t.Fatalf("changed: %s", got)
	}
}

// CodeBuddy Code has a skill and a hook of its own: it is not one of
// the agents that share a directory with the others.
func TestCodeBuddyIsNotAmongTheAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KERO_HOME", filepath.Join(home, "kero"))
	for _, env := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "PI_CODING_AGENT_DIR", "XDG_CONFIG_HOME", "CODEBUDDY_CONFIG_DIR"} {
		t.Setenv(env, "")
	}
	if err := os.MkdirAll(filepath.Join(home, ".codebuddy"), 0o755); err != nil {
		t.Fatal(err)
	}
	names := func(agent string) string {
		var out []string
		for _, it := range agentHooks(agent) {
			out = append(out, it.name())
		}
		return strings.Join(out, ",")
	}
	if got := names("agents"); strings.Contains(got, "codebuddy") {
		t.Errorf("the agents that share a directory include codebuddy: %s", got)
	}
	if got := names(""); !strings.Contains(got, "codebuddy") {
		t.Errorf("every agent misses codebuddy: %s", got)
	}
	if got := names("codebuddy"); got != "codebuddy" {
		t.Errorf("codebuddy alone = %s", got)
	}
}

// Pi's and OpenCode's plugins go in when the agent is installed, run
// Kero's own kero command, and come out whole.
func TestAgentPlugins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KERO_HOME", filepath.Join(home, "kero"))
	for _, env := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "PI_CODING_AGENT_DIR", "XDG_CONFIG_HOME"} {
		t.Setenv(env, "")
	}
	names := func() (out []string) {
		for _, it := range agentHooks("") {
			out = append(out, it.name())
		}
		return
	}
	if got := strings.Join(names(), ","); got != "claude" {
		t.Fatalf("with no agents installed: %s", got)
	}
	os.MkdirAll(filepath.Join(home, ".pi", "agent"), 0o755)
	os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755)
	if got := strings.Join(names(), ","); got != "claude,pi,opencode" {
		t.Fatalf("with Pi and OpenCode: %s", got)
	}
	for _, it := range agentHooks("agents") {
		if it.state() != "missing" {
			t.Fatalf("%s: %s before install", it.name(), it.state())
		}
		if err := it.install(); err != nil {
			t.Fatal(err)
		}
		if it.state() != "current" {
			t.Fatalf("%s: %s after install", it.name(), it.state())
		}
		p := it.(agentPlugin)
		for name := range p.files {
			b := read(t, filepath.Join(p.dir, name))
			if strings.Contains(b, "__KERO__") || name != "package.json" && !strings.Contains(b, filepath.Join(home, "kero", "bin", "kero")) {
				t.Fatalf("%s/%s runs no kero:\n%s", it.name(), name, b)
			}
		}
		if err := it.remove(); err != nil {
			t.Fatal(err)
		}
		if it.state() != "missing" {
			t.Fatalf("%s: %s after remove", it.name(), it.state())
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode", "plugins", "kero")); !os.IsNotExist(err) {
		t.Fatal("OpenCode's plugin directory stayed")
	}
	if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "extensions")); err != nil {
		t.Fatal("Pi's extensions directory went with Kero's extension")
	}
}

// A settings file that is a symlink, as of a dotfiles repo, is written
// through: the symlink stays, and the file it points to changes.
func TestHookInstallThroughASymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("KERO_HOME", filepath.Join(home, "kero"))
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(filepath.Join("dotfiles", "settings.json"), link); err != nil {
		t.Skip(err) // Windows without the privilege
	}
	h := agentHook{"claude", link}
	if err := h.install(); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the symlink was taken away by a plain file")
	}
	if got := read(t, target); !strings.Contains(got, "hook claude") {
		t.Fatalf("the file it points to did not change:\n%s", got)
	}
}

// A hook of another program is not kero's, even when its name has
// "kero" in it: the user's own hook is not kero's to take away.
func TestHookOfAnotherProgram(t *testing.T) {
	h := agentHook{agent: "claude"}
	for _, c := range []string{
		"'/usr/local/bin/kero' hook claude",
		"/usr/local/bin/kero hook claude",
		"kero hook claude",
		`"C:\Program Files\Kero\bin\kero.exe" hook claude`,
		`cmd /d /c "C:\Program Files\Kero\bin\kero.exe" hook claude`,
	} {
		if !h.ours(c) {
			t.Errorf("%q is kero's hook", c)
		}
	}
	for _, c := range []string{
		"/usr/local/bin/kerouac hook claude",
		"/opt/kero-notes/hook claude",
		"my-kero hook claude",
		"kero hook codex",
	} {
		if h.ours(c) {
			t.Errorf("%q is not kero's hook", c)
		}
	}
}

func TestMain(m *testing.M) {
	// The kero command of TestCodexHookRunsInEveryShell: this test binary.
	if os.Getenv("KERO_TEST_HOOK") == "1" {
		fmt.Println("hook ran:", strings.Join(os.Args[1:], " "))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestAntigravityHookInstallKeepsSettings(t *testing.T) {
	t.Setenv("KERO_HOME", filepath.Join(t.TempDir(), "kero home"))
	h := antigravityHook{filepath.Join(t.TempDir(), "hooks.json")}
	initial := `{
  "tty7": {
    "PreInvocation": [
      {
        "type": "command",
        "command": "\"/Applications/tty7.app/Contents/MacOS/tty7-app\" agent-hook antigravity prompt-submit",
        "timeout": 5
      }
    ],
    "Stop": [
      {
        "type": "command",
        "command": "\"/Applications/tty7.app/Contents/MacOS/tty7-app\" agent-hook antigravity stop",
        "timeout": 5
      }
    ]
  }
}
`
	if err := os.WriteFile(h.file, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if st := h.state(); st != "missing" {
		t.Fatalf("state %s before install, want missing", st)
	}
	if err := h.install(); err != nil {
		t.Fatal(err)
	}
	got := read(t, h.file)
	if h.state() != "current" {
		t.Fatalf("state %s after install:\n%s", h.state(), got)
	}
	if i, j := strings.Index(got, `"tty7"`), strings.Index(got, `"kero"`); !(i >= 0 && i < j) {
		t.Fatalf("keys out of order:\n%s", got)
	}
	if !strings.Contains(got, "tty7-app") || !strings.Contains(got, "hook antigravity") {
		t.Fatalf("missing expected hook contents:\n%s", got)
	}
	// Re-installing is idempotent.
	if err := h.install(); err != nil {
		t.Fatal(err)
	}
	if again := read(t, h.file); again != got {
		t.Fatalf("installing again changed it:\n%s", again)
	}
	// Removing takes only kero's entry out and leaves tty7 intact.
	if err := h.remove(); err != nil {
		t.Fatal(err)
	}
	removed := read(t, h.file)
	if strings.Contains(removed, "kero") || !strings.Contains(removed, `"tty7"`) {
		t.Fatalf("after remove:\n%s", removed)
	}
	if h.state() != "missing" {
		t.Fatalf("state %s after remove, want missing", h.state())
	}

	// Outdated hook is detected and replaced.
	outdated := `{"kero":{"SessionStart":[{"type":"command","command":"'/old/kero/bin/kero' hook antigravity","timeout":10}]}}`
	if err := os.WriteFile(h.file, []byte(outdated), 0o600); err != nil {
		t.Fatal(err)
	}
	if st := h.state(); st != "outdated" {
		t.Fatalf("outdated state = %s, want outdated", st)
	}
	if err := h.install(); err != nil {
		t.Fatal(err)
	}
	if st := h.state(); st != "current" {
		t.Fatalf("after replacing outdated: %s", st)
	}

	// Unreadable / invalid JSON is reported and refused.
	badFile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badFile, []byte(`{"kero":{"SessionStart":"not-an-array"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	badHook := antigravityHook{badFile}
	if st := badHook.state(); st != "unreadable" {
		t.Fatalf("bad SessionStart state = %s, want unreadable", st)
	}
	if err := badHook.install(); err == nil {
		t.Fatal("installed into unreadable antigravity hooks.json")
	}
}

func startMockDaemon(t *testing.T, onAgentSession func(proto.AgentSessionParams) (proto.AgentSessionResult, error)) string {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "kerod.sock")
	if len(sock) > 95 {
		shortDir, err := os.MkdirTemp("/tmp", "kero-hook-test-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(shortDir) })
		sock = filepath.Join(shortDir, "k.sock")
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KERO_SOCKET", sock)
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	t.Cleanup(func() { ln.Close() })
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer c.Close()
				r := bufio.NewReader(c)
				line, err := proto.ReadLine(r, proto.MaxLine)
				if err != nil {
					return
				}
				var hello proto.Hello
				if json.Unmarshal(line, &hello) != nil {
					return
				}
				json.NewEncoder(c).Encode(proto.Welcome{Version: proto.Version, PID: os.Getpid()})
				for {
					line, err := proto.ReadLine(r, proto.MaxLine)
					if err != nil {
						return
					}
					var req struct {
						ID     int64           `json:"id"`
						Method string          `json:"method"`
						Params json.RawMessage `json:"params"`
					}
					if json.Unmarshal(line, &req) != nil {
						return
					}
					switch req.Method {
					case "pane.agentSession":
						var p proto.AgentSessionParams
						json.Unmarshal(req.Params, &p)
						res, err := onAgentSession(p)
						msg := proto.Message{ID: req.ID}
						if err != nil {
							msg.Error = err.Error()
						} else {
							msg.Result = res
						}
						json.NewEncoder(c).Encode(msg)
					case "state":
						json.NewEncoder(c).Encode(proto.Message{
							ID: req.ID,
							Result: proto.State{Sessions: []proto.Session{{
								ID:    "s1",
								Panes: []proto.Pane{{ID: "p1"}},
							}}},
						})
					default:
						json.NewEncoder(c).Encode(proto.Message{ID: req.ID, Result: true})
					}
				}
			}(conn)
		}
	}()
	return sock
}

func TestRunHookCodexAppServerAndAntigravity(t *testing.T) {
	t.Setenv("KERO_HOME", t.TempDir())
	t.Setenv("CODEX_THREAD_ID", "")

	// 1. Codex without KERO_PANE and no daemon running: silent nil, no stdout.
	t.Setenv("KERO_PANE", "")
	t.Setenv("KERO_SESSION", "")
	t.Setenv("KERO_SOCKET", filepath.Join(t.TempDir(), "missing.sock"))
	var out bytes.Buffer
	if err := runHook([]string{"codex"}, strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"019a-codex","cwd":"/repo"}`), &out); err != nil {
		t.Fatalf("codex without daemon: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("codex without daemon wrote stdout: %q", out.String())
	}

	// 2. Codex without KERO_PANE when daemon matches a pane -> reports and emits away context.
	var gotParams []proto.AgentSessionParams
	startMockDaemon(t, func(p proto.AgentSessionParams) (proto.AgentSessionResult, error) {
		gotParams = append(gotParams, p)
		if p.Dir == "/outside" {
			return proto.AgentSessionResult{}, fmt.Errorf("no pane runs %s session %q", p.Agent, p.Session)
		}
		return proto.AgentSessionResult{Pane: "p1", Session: "s1", Away: true}, nil
	})
	out.Reset()
	if err := runHook([]string{"codex"}, strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"019a-codex","cwd":"/repo"}`), &out); err != nil {
		t.Fatalf("codex matched: %v", err)
	}
	if len(gotParams) != 1 || gotParams[0].Agent != "codex" || gotParams[0].Session != "019a-codex" || gotParams[0].Dir != "/repo" {
		t.Fatalf("unexpected RPC params: %+v", gotParams)
	}
	if !strings.Contains(out.String(), `"hookSpecificOutput"`) || !strings.Contains(out.String(), "session s1, pane p1") || !strings.Contains(out.String(), "kero split p1") {
		t.Fatalf("unexpected codex away output: %s", out.String())
	}

	// 3. Codex without KERO_PANE when daemon finds no pane -> silent nil, no stdout.
	out.Reset()
	if err := runHook([]string{"codex"}, strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"019a-ext","cwd":"/outside"}`), &out); err != nil {
		t.Fatalf("codex unmatched: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("codex unmatched wrote stdout: %q", out.String())
	}

	// 4. Antigravity outside Kero -> outputs "{}\n".
	out.Reset()
	if err := runHook([]string{"antigravity"}, strings.NewReader(`{"hookEventName":"SessionStart","conversationId":"conv-1","cwd":"/repo"}`), &out); err != nil {
		t.Fatalf("antigravity outside: %v", err)
	}
	if strings.TrimSpace(out.String()) != "{}" {
		t.Fatalf("antigravity outside = %q, want {}", out.String())
	}

	// 5. Antigravity inside Kero -> reports conversationId and outputs injectSteps JSON.
	t.Setenv("KERO_PANE", "p1")
	t.Setenv("KERO_SESSION", "s1")
	gotParams = nil
	out.Reset()
	if err := runHook([]string{"antigravity"}, strings.NewReader(`{"hookEventName":"SessionStart","conversationId":"conv-1","cwd":"/repo"}`), &out); err != nil {
		t.Fatalf("antigravity inside: %v", err)
	}
	if len(gotParams) != 1 || gotParams[0].Agent != "antigravity" || gotParams[0].Session != "conv-1" {
		t.Fatalf("antigravity RPC params = %+v", gotParams)
	}
	var agyOut struct {
		InjectSteps []struct {
			SystemMessage struct {
				SystemMessage string `json:"systemMessage"`
			} `json:"systemMessage"`
		} `json:"injectSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &agyOut); err != nil {
		t.Fatalf("antigravity stdout invalid JSON: %v\n%s", err, out.String())
	}
	if len(agyOut.InjectSteps) != 1 || !strings.Contains(agyOut.InjectSteps[0].SystemMessage.SystemMessage, "session s1, pane p1") {
		t.Fatalf("unexpected antigravity injectSteps: %+v", agyOut)
	}

	// 6. Antigravity with empty cwd falls back to workspacePaths[0].
	gotParams = nil
	out.Reset()
	if err := runHook([]string{"antigravity"}, strings.NewReader(`{"hookEventName":"SessionStart","conversationId":"conv-ws","workspacePaths":["/ws/repo"]}`), &out); err != nil {
		t.Fatalf("antigravity workspacePaths: %v", err)
	}
	if len(gotParams) != 1 || gotParams[0].Session != "conv-ws" || gotParams[0].Dir != "/ws/repo" {
		t.Fatalf("antigravity workspacePaths RPC params = %+v", gotParams)
	}
}
