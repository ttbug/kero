---
title: Sessions & Projects
description: How Kero groups terminal sessions by project and keeps them running in kerod.
---

# Sessions & Projects

Kero organizes your work in two levels: projects (folders) hold sessions (tasks), and every session keeps running in kerod until you close it.

## Sessions

A **session** is a unit of work: one task, one agent, one long-running process. It holds one or more [panes](./panes.md), each running a shell or a program.

| Action                     | Shortcut                                     |
| -------------------------- | -------------------------------------------- |
| New session                | <kbd>⌘</kbd> <kbd>T</kbd>                    |
| New session in home folder | <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>T</kbd>       |
| Go to session…             | <kbd>⌘</kbd> <kbd>P</kbd>                    |
| Jump to session needing you | <kbd>⌘</kbd> <kbd>J</kbd>                   |
| Next / previous session    | <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>]</kbd> / <kbd>[</kbd> |
| Rename session             | <kbd>⌘</kbd> <kbd>R</kbd>                    |
| Close session              | <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>W</kbd>       |

Name sessions after their task, such as `fix-login-redirect` or `review-pr-42`, so you can find them in the sidebar and target them from the [CLI](./cli.md).

Each row shows the agent or program, the session's status, and below it the agent's latest report (`kero report`), or the folder the session runs in when that is not its project's. A project's header shows the branch its sessions share.

### Where a new session starts

The `new-session-directory` setting picks the folder:

- `current` (default): the directory of the session you're in
- `project`: the project's root folder
- `home`: your home folder

### How an unnamed session is named

Until you or an agent name a session, the `session-name` setting (**Name sessions after** in Settings) picks what the sidebar shows:

- `focused` (default): the title of the program in the pane you're in. A pane running only its shell defers to the pane that needs you most.
- `lead`: the title of the program in the pane that needs you most, such as an agent that's working or waiting on you
- `folder`: the first pane's folder, whatever runs in it

## Projects

A **project** is a folder. Kero groups sessions under their project in the sidebar. Open one with <kbd>⌘</kbd> <kbd>O</kbd>, or let the folder pick it: a new session joins the project of the folder it starts in, or of the repository that folder is in, and a session whose shell `cd`s to another folder moves to that folder's project, made for a repository. A session you moved by hand stays where you put it. Turn the automatic move off with `group-by-folder = false`.

```sh
kero projects            # list projects
kero new -p my-app       # start a session in a project by name
```

## kerod

Sessions run in **kerod**, a daemon that the app starts on demand. Kero is a window onto kerod:

- Quitting Kero leaves every session running.
- Opening Kero again shows every session as it was, scrollback included.
- After kerod itself restarts, such as after a reboot, Kero starts your sessions again in their directories. Turn this off with `restore-sessions = false`.
- A pane that was running Claude Code, Codex, Pi or OpenCode resumes its conversation, as the agent's own resume option would, once [Kero's skill](./agents.md#install-the-skill) is set up for that agent. Other programs don't start again unasked: their pane comes back at a shell. Turn this off with `resume-agents = false`.

```sh
kero daemon status       # is kerod running?
kero daemon restart      # restart it (sessions start again)
kero daemon stop         # stop it and end every session
```
