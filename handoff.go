package main

import (
	"kero/internal/proto"
)

// handoffFrom is the agent whose task a session's menu can hand over to
// another agent: its focused pane's, else any pane's; "" for none.
func handoffFrom(s *proto.Session) string {
	if s == nil {
		return ""
	}
	panes := s.Panes
	if len(panes) == 0 {
		panes = []proto.Pane{s.Pane}
	}
	from := ""
	for i := range panes {
		p := &panes[i]
		agent := p.Agent
		if p.AgentSession != nil && (agent == "" || agent == p.AgentSession.Agent) {
			agent = p.AgentSession.Agent
		}
		if agent == "" {
			continue
		}
		if p.ID == s.Focus {
			return agent
		}
		if from == "" {
			from = agent
		}
	}
	return from
}

// handoff starts agent in a new session beside session id, told to carry
// on the task of the agent in it, and shows it.
func (a *App) handoff(id, agent string) {
	s := a.session(id)
	if s == nil {
		return
	}
	h := a.hostOf(id)
	cl := h.cl
	if cl == nil {
		a.callOn(h, "handoff", nil, nil) // says the host is not connected
		return
	}
	p := proto.HandoffParams{ID: id, Agent: agent, Cols: s.Cols, Rows: s.Rows}
	go func() {
		var ns proto.Session
		err := cl.Call("handoff", p, &ns)
		a.update(func() {
			if err != nil {
				a.toast(err.Error())
				return
			}
			delete(a.prefs.Collapsed, ns.Project)
			a.addPending(h, ns)
			a.selectSession(ns.ID)
		})
	}()
}
