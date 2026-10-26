package daemon

import (
	"os"
	"slices"

	"kero/internal/proto"
)

// follow moves a session into the project of its focused pane's shell
// directory, as soon as the daemon's 400ms loop reads it: the sidebar
// groups sessions by where their shells are, as it groups them by where
// they start. The project the session left goes with its last session
// when it was one the daemon made (proto.Project.Auto). It reports
// whether it changed what the app shows.
func (d *Daemon) follow(p *pane) bool {
	p.mu.Lock()
	if p.closed || p.proc == nil || p.info.Browser() || p.info.Status == proto.Exited {
		p.mu.Unlock()
		return false
	}
	pid, sid, dir := p.info.ID, p.sid, p.info.Dir
	if dir == "" || dir == p.followedDir {
		// Nothing to go by, or a directory follow has already acted on:
		// one the user's own move put the session in, rather than a cd.
		p.mu.Unlock()
		return false
	}
	p.mu.Unlock()
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		// The directory went away under the shell.
		return false
	}

	d.mu.Lock()
	s := d.sessionByIDLocked(sid)
	if s == nil || s.worktree != nil {
		// A session of its own worktree stays in the repository's project.
		// Settled, so the directory doesn't come up again.
		p.settleFollow()
		d.mu.Unlock()
		return false
	}
	// Only the pane the user is in moves the session: the others may work
	// in another repository, and would pull it back and forth.
	if lead := focusedLocked(s); lead == nil || lead.info.ID != pid {
		d.mu.Unlock()
		return false
	}
	target := d.projectFor("", dir)
	// Settled either way: follow acts once per directory.
	p.settleFollow()
	if target == s.project {
		d.mu.Unlock()
		return false
	}
	old := s.project
	d.regroupLocked(s, target)
	d.dropEmptyLocked(old)
	d.mu.Unlock()
	d.log.Printf("session %s follows %s into %s", s.id, dir, target)
	return true
}

// settleFollow marks the shell's directory as one follow has acted on, as
// when the user moves the session by hand: a directory the shell already
// left shouldn't undo the move.
func (p *pane) settleFollow() {
	p.mu.Lock()
	p.followedDir = p.info.Dir
	p.mu.Unlock()
}

func (d *Daemon) sessionByIDLocked(id string) *session {
	for _, s := range d.sessions {
		if s.id == id {
			return s
		}
	}
	return nil
}

// regroupLocked puts a session in a project, after the project's last
// session in the order; d.mu is held.
func (d *Daemon) regroupLocked(s *session, project string) {
	s.project = project
	i := slices.Index(d.sessions, s)
	d.sessions = slices.Delete(d.sessions, i, i+1)
	at := len(d.sessions)
	for j, o := range d.sessions {
		if o.project == project {
			at = j + 1
		}
	}
	d.sessions = slices.Insert(d.sessions, at, s)
}

// dropEmptyLocked removes a project the daemon made for a session's
// directory (proto.Project.Auto) when no session is in it: the sidebar
// has no use for an empty group. Projects the user opened stay. It
// reports whether it removed one; d.mu is held.
func (d *Daemon) dropEmptyLocked(id string) bool {
	if id == "" {
		return false
	}
	for _, s := range d.sessions {
		if s.project == id {
			return false
		}
	}
	i := slices.IndexFunc(d.projects, func(pr proto.Project) bool { return pr.ID == id })
	if i < 0 || !d.projects[i].Auto {
		return false
	}
	d.projects = slices.Delete(d.projects, i, i+1)
	return true
}

// pruneAutoProjects removes auto projects with no session, as after a
// restore that couldn't bring their sessions back.
func (d *Daemon) pruneAutoProjects() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.projects = slices.DeleteFunc(d.projects, func(pr proto.Project) bool {
		return pr.Auto && !slices.ContainsFunc(d.sessions, func(s *session) bool { return s.project == pr.ID })
	})
}
