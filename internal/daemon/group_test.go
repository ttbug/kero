package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kero/internal/proto"
	"kero/internal/settings"
)

// A session joins the project of the folder its shell cd's to, or its
// agent starts in; a folder of no project changes nothing, and a session
// the user moved by hand, or one with grouping off, stays.
func TestRegroup(t *testing.T) {
	d := testDaemon(t)
	one := d.projects[0]
	two := filepath.Join(t.TempDir(), "two")
	outside := filepath.Join(t.TempDir(), "outside")
	for _, dir := range []string{two, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	d.projects = append(d.projects, proto.Project{ID: "two", Name: "two", Dir: two})
	s := &session{id: "s", project: one.ID}
	d.sessions = []*session{s}

	if d.regroup(s.id, one.Dir) {
		t.Error("moved a session within its project")
	}
	if d.regroup(s.id, outside) {
		t.Error("moved a session into a folder of no project")
	}
	if !d.regroup(s.id, two) {
		t.Fatal("did not move the session to another project")
	}
	if s.project != "two" {
		t.Errorf("the session is in project %q, want two", s.project)
	}

	s.pinned = true
	before := s.project
	if d.regroup(s.id, one.Dir) {
		t.Error("moved a session the user pinned")
	}
	if s.project != before {
		t.Errorf("a pinned session moved to %q", s.project)
	}

	s.pinned = false
	if err := os.WriteFile(settings.Path(), []byte("group-by-folder = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if d.regroup(s.id, one.Dir) {
		t.Error("moved a session with grouping off")
	}
}

// A repository of no project yet makes one, as a session's shell cd's
// into it.
func TestRegroupRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	tmp := realPath(t, t.TempDir())
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "init", "-q")
	gitIn(t, repo, "commit", "-q", "--allow-empty", "-m", "init")

	d := testDaemon(t)
	one := d.projects[0]
	s := &session{id: "s", project: one.ID}
	d.sessions = []*session{s}
	if !d.regroup(s.id, filepath.Join(repo, "src")) {
		t.Fatal("did not move the session into the repository")
	}
	if got := projectWithID(d, s.project); got == nil || got.Dir != repo {
		t.Errorf("session in %+v, want the project of %s", got, repo)
	}
}

// projectWithID returns the project with id, or nil.
func projectWithID(d *Daemon, id string) *proto.Project {
	for i := range d.projects {
		if d.projects[i].ID == id {
			return &d.projects[i]
		}
	}
	return nil
}

// A repository under the home folder makes its own project, rather than
// joining the Home project that holds it; a plain folder still joins
// Home, and a project opened above the repository holds it.
func TestProjectForHome(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	tmp := realPath(t, t.TempDir())
	home := filepath.Join(tmp, "home")
	repo := filepath.Join(home, "work", "repo")
	other := filepath.Join(home, "work", "other")
	plain := filepath.Join(home, "scratch")
	for _, dir := range []string{repo, other, plain} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{repo, other} {
		gitIn(t, dir, "init", "-q")
		gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	}
	t.Setenv("HOME", home)

	d := testDaemon(t)
	d.projects = []proto.Project{{ID: "home", Name: "Home", Dir: proto.Home()}}
	if got := d.projectFor("", plain); got != "home" {
		t.Errorf("a plain folder in home got project %q, want home", got)
	}
	id := d.projectFor("", repo)
	if id == "home" {
		t.Fatal("the repository joined the Home project")
	}
	if got := projectWithID(d, id); got == nil || got.Dir != repo {
		t.Errorf("the repository got %+v, want a project of its own", got)
	}

	d.projects = append(d.projects, proto.Project{ID: "work", Name: "work", Dir: filepath.Join(home, "work")})
	if got := d.projectFor("", other); got != "work" {
		t.Errorf("the repository got project %q, want the project above it", got)
	}
}

// A linked worktree outside its repository joins the repository's
// project, rather than making another of the same folder.
func TestProjectForWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	tmp := realPath(t, t.TempDir())
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "init", "-q")
	gitIn(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	worktree := filepath.Join(tmp, "worktree")
	gitIn(t, repo, "worktree", "add", "-q", "-b", "side", worktree)

	d := testDaemon(t)
	d.projects = []proto.Project{{ID: "pr", Name: "repo", Dir: repo}}
	if got := d.projectFor("", worktree); got != "pr" {
		t.Errorf("project %q, want the repository's", got)
	}
	if len(d.projects) != 1 {
		t.Errorf("projects %+v, want the repository's alone", d.projects)
	}
}
