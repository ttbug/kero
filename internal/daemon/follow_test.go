package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kero/internal/proto"
)

// followTestDaemon is a daemon with no project of its own: sessions make
// theirs from their directories.
func followTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	d := testDaemon(t)
	d.projects = nil
	return d
}

// mkdirs makes sibling directories under one temporary directory.
func mkdirs(t *testing.T, names ...string) []string {
	t.Helper()
	base := t.TempDir()
	dirs := make([]string, len(names))
	for i, name := range names {
		dirs[i] = filepath.Join(base, name)
		if err := os.MkdirAll(dirs[i], 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return dirs
}

func projectByID(t *testing.T, d *Daemon, id string) (proto.Project, bool) {
	t.Helper()
	for _, p := range d.state().Projects {
		if p.ID == id {
			return p, true
		}
	}
	return proto.Project{}, false
}

func projectNamed(t *testing.T, d *Daemon, name string) (proto.Project, bool) {
	t.Helper()
	for _, p := range d.state().Projects {
		if p.Name == name {
			return p, true
		}
	}
	return proto.Project{}, false
}

func namedProjectID(t *testing.T, d *Daemon, name string) string {
	t.Helper()
	p, ok := projectNamed(t, d, name)
	if !ok {
		t.Fatalf("no project %q", name)
	}
	return p.ID
}

func setPaneDir(t *testing.T, p *pane, dir string) {
	t.Helper()
	p.mu.Lock()
	p.info.Dir = dir
	p.mu.Unlock()
}

func sessionProject(t *testing.T, d *Daemon, id string) string {
	t.Helper()
	for _, s := range d.state().Sessions {
		if s.ID == id {
			return s.Project
		}
	}
	t.Fatalf("no session %s", id)
	return ""
}

// A repository under a project that holds other directories gets a
// project of its own: a group at $HOME mustn't hold every repository
// under it, so that a session that cds into one moves there.
func TestProjectForRepositoryUnderHome(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	d := followTestDaemon(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "iohang_agent")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-q")

	inHome, err := d.create(proto.CreateParams{})
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := projectByID(t, d, inHome.Project); !ok || p.Name != "Home" {
		t.Fatalf("a session with no directory starts in %+v, want Home", p)
	}
	inRepo, err := d.create(proto.CreateParams{Dir: repo})
	if err != nil {
		t.Fatal(err)
	}
	if inRepo.Project == inHome.Project {
		t.Fatal("a repository under Home stayed in the Home project")
	}
	if p, ok := projectByID(t, d, inRepo.Project); !ok || p.Name != "iohang_agent" || !p.Auto {
		t.Errorf("the repository's project = %+v, want the auto project iohang_agent", p)
	}

	// A directory in the repository stays with it.
	sub := filepath.Join(repo, "internal")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	inSub, err := d.create(proto.CreateParams{Dir: sub})
	if err != nil {
		t.Fatal(err)
	}
	if inSub.Project != inRepo.Project {
		t.Errorf("a directory of the repository left its project")
	}
}

// A plain folder gets a group of its own, as a repository does, even
// when a group the daemon made holds the folder above it; a project the
// user opened holds everything in it.
func TestProjectForPlainDirectory(t *testing.T) {
	d := followTestDaemon(t)
	dirs := mkdirs(t, "auto", "auto/inner", "mine", "mine/inner")
	auto, inner, mine, mineInner := dirs[0], dirs[1], dirs[2], dirs[3]

	s, err := d.create(proto.CreateParams{Dir: auto})
	if err != nil {
		t.Fatal(err)
	}
	p := d.panes[s.Panes[0].ID]
	setPaneDir(t, p, inner)
	if !d.follow(p) {
		t.Fatal("follow didn't move the session into the plain folder")
	}
	got := sessionProject(t, d, s.ID)
	if pr, ok := projectByID(t, d, got); !ok || pr.Name != "inner" || pr.Dir != inner || !pr.Auto {
		t.Errorf("a plain folder under an auto group = %+v, want the auto project inner at it", pr)
	}

	// The directory itself keeps its group when the shell stays in it.
	if _, err := createProject(t, d, proto.ProjectParams{Dir: mine, Name: "Mine"}); err != nil {
		t.Fatal(err)
	}
	s2, err := d.create(proto.CreateParams{Dir: mineInner})
	if err != nil {
		t.Fatal(err)
	}
	if pr, _ := projectByID(t, d, s2.Project); pr.Name != "Mine" {
		t.Errorf("a folder under the user's project is in %+v, want Mine", pr)
	}
}

// A session starts in the project of its directory, and moves with its
// shell into the project of a directory the shell cds to once that
// directory has stayed put; the project it left goes if the daemon made
// it.
func TestSessionFollowsShellDirectory(t *testing.T) {
	d := followTestDaemon(t)
	dirs := mkdirs(t, "alpha", "beta")
	alpha, beta := dirs[0], dirs[1]
	s, err := d.create(proto.CreateParams{Dir: alpha})
	if err != nil {
		t.Fatal(err)
	}
	start, ok := projectNamed(t, d, "alpha")
	if !ok || !start.Auto || s.Project != start.ID {
		t.Fatalf("the session starts in %+v, project %q", start, s.Project)
	}
	p := d.panes[s.Panes[0].ID]

	setPaneDir(t, p, beta)
	if !d.follow(p) {
		t.Fatal("follow didn't move the session")
	}
	if got := sessionProject(t, d, s.ID); got != namedProjectID(t, d, "beta") {
		t.Errorf("session project = %q, want beta", got)
	}
	if _, ok := projectByID(t, d, start.ID); ok {
		t.Error("the empty auto project of the directory left stayed")
	}
	// One move per directory: the shell staying there moves nothing else.
	if d.follow(p) {
		t.Error("follow moved again for the same directory")
	}
}

// Only the pane the user is in moves the session: a background pane may
// work in another repository.
func TestFollowFromFocusedPaneOnly(t *testing.T) {
	d := followTestDaemon(t)
	dirs := mkdirs(t, "alpha", "beta", "gamma")
	alpha, beta, gamma := dirs[0], dirs[1], dirs[2]
	s, err := d.create(proto.CreateParams{Dir: alpha})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.split(proto.CreateParams{Split: s.ID, Dir: beta}); err != nil {
		t.Fatal(err)
	}
	sess := d.state().Sessions[0]
	lead := d.panes[sess.Pane.ID]
	var other *pane
	for _, pi := range sess.Panes {
		if pi.ID != sess.Pane.ID {
			other = d.panes[pi.ID]
		}
	}
	if other == nil {
		t.Fatal("the split made no second pane")
	}

	setPaneDir(t, other, gamma)
	if d.follow(other) {
		t.Fatal("a pane the user isn't in moved the session")
	}
	if got := sessionProject(t, d, s.ID); got != sess.Project {
		t.Errorf("session project = %q, want %q", got, sess.Project)
	}

	setPaneDir(t, lead, gamma)
	if !d.follow(lead) {
		t.Fatal("the focused pane didn't move the session")
	}
	if got := sessionProject(t, d, s.ID); got != namedProjectID(t, d, "gamma") {
		t.Errorf("session project = %q, want gamma", got)
	}
}

// A session of its own worktree stays in its repository's project.
func TestFollowKeepsWorktreeSession(t *testing.T) {
	d := followTestDaemon(t)
	repo := gitRepo(t)
	s, err := d.create(proto.CreateParams{Dir: repo, Worktree: "fix/login"})
	if err != nil {
		t.Fatal(err)
	}
	p := d.panes[s.Panes[0].ID]
	setPaneDir(t, p, t.TempDir())
	if d.follow(p) {
		t.Fatal("a worktree session left its repository")
	}
	if got := sessionProject(t, d, s.ID); got != s.Project {
		t.Errorf("session project = %q, want %q", got, s.Project)
	}
}

// A project the user named or opened stays when its last session leaves;
// one the daemon made for a directory goes, whether the session moved or
// closed.
func TestEmptyProjectRules(t *testing.T) {
	d := followTestDaemon(t)
	dirs := mkdirs(t, "alpha", "beta", "gamma")
	alpha, beta, gamma := dirs[0], dirs[1], dirs[2]
	s, err := d.create(proto.CreateParams{Dir: alpha})
	if err != nil {
		t.Fatal(err)
	}
	auto := s.Project
	// Naming the group the daemon made makes it the user's: it stays empty.
	raw, err := json.Marshal(proto.ProjectParams{ID: auto, Name: "Alpha mine"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.call(nil, "project.rename", raw); err != nil {
		t.Fatal(err)
	}
	mine, err := createProject(t, d, proto.ProjectParams{Dir: beta, Name: "Mine"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.move(proto.MoveParams{ID: s.ID, Project: mine.ID}); err != nil {
		t.Fatal(err)
	}
	if _, ok := projectByID(t, d, auto); !ok {
		t.Error("a renamed project went with its last session's move")
	}

	// A group the daemon made from a directory goes with its session.
	s2, err := d.create(proto.CreateParams{Dir: gamma})
	if err != nil {
		t.Fatal(err)
	}
	auto2 := s2.Project
	if err := d.removeSession(s2.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := projectByID(t, d, auto2); ok {
		t.Error("an unnamed auto project stayed empty")
	}

	// The user's project stays when its last session closes.
	if err := d.removeSession(s.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := projectByID(t, d, mine.ID); !ok {
		t.Error("the project the user opened went with its last session")
	}
}

// A session moved by hand stays where the user put it while its shell
// keeps its directory: the directory doesn't undo the move.
func TestManualMoveIsNotUndone(t *testing.T) {
	d := followTestDaemon(t)
	dirs := mkdirs(t, "alpha", "beta")
	alpha, beta := dirs[0], dirs[1]
	s, err := d.create(proto.CreateParams{Dir: alpha})
	if err != nil {
		t.Fatal(err)
	}
	mine, err := createProject(t, d, proto.ProjectParams{Dir: beta, Name: "Mine"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.move(proto.MoveParams{ID: s.ID, Project: mine.ID}); err != nil {
		t.Fatal(err)
	}
	p := d.panes[s.Panes[0].ID]
	if d.follow(p) {
		t.Fatal("follow undid the user's move")
	}
	if got := sessionProject(t, d, s.ID); got != mine.ID {
		t.Errorf("session project = %q, want %q", got, mine.ID)
	}
}
