package main

import (
	"math"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/proto"

	"kero/locales"
)

const (
	// barMid is the line the title bar's contents center on: the traffic
	// lights, the sidebar's buttons, the panes' headers. topBar is the
	// sidebar's title bar, centered on it, and headerH a header of a card,
	// which starts cardGap down, centered on it too.
	barMid  = 30
	topBar  = 2 * barMid
	headerH = 2 * (barMid - cardGap)
	cardGap = 8  // around the card
	rowH    = 54 // session title + status/program
	rowH2   = 70 // with a third line for details
	// rowInset is the inset of historical rows and sidebar notes.
	rowInset = 12
)

// chrome is where the sidebar's animation puts things, in whole pixels so
// that edges stay crisp. Each is rounded once, from one progress quantized
// to the sidebar's pixels, so that they move together: rounded apart,
// they would step at different frames, and a title, their sum, would
// shake by a pixel as the spring settles.
type chrome struct {
	shown     float32 // the sidebar's progress, 0 hidden to 1 shown
	sideW     float32 // the sidebar's width as shown
	mainPad   float32 // the left padding of the cards' column
	headerPad float32 // the left padding of the header at the left edge
}

// buttonsLeft places the sidebar's buttons that the header holds while
// the sidebar hides, in the header's coordinates, so that they stay where
// they are in the window, after the traffic lights, as the card slides.
func (ch chrome) buttonsLeft(bar ui.TitleBar) float32 {
	return lead(bar) - (ch.sideW + ch.mainPad)
}

// lead is where the sidebar's buttons start in the window, with the sidebar
// shown or not: after the traffic lights on macOS; elsewhere inside the
// card's edge, which they come over as the sidebar hides.
func lead(bar ui.TitleBar) float32 {
	if ownControls {
		return bar.Left + 2*cardGap
	}
	return bar.Left + cardGap
}

var sidebarMotion = ui.Animation{Duration: 260 * time.Millisecond, Ease: ui.CubicBezier(0.25, 1, 0.5, 1)}

func chromeFor(progress, width float32, bar ui.TitleBar) chrome {
	round := func(v float32) float32 { return float32(math.Round(float64(v))) }
	w := round(width * min(max(progress, 0), 1))
	s := w / width
	pad := round(cardGap * (1 - s))
	// Where the header's title starts in the window, rounded once: after
	// the traffic lights and the sidebar's buttons with the sidebar
	// hidden, 16 into the card with it shown.
	hidden, shown := lead(bar)+16+60, width+16
	x := round(hidden + (shown-hidden)*s)
	return chrome{shown: s, sideW: w, mainPad: pad, headerPad: x - w - pad}
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func (a *App) view(c *ui.Context) {
	a.looks()
	pal := paletteFor(c)
	c.SetTheme(borderless(c, pal))

	if a.pendingToast != "" {
		c.Toast(a.pendingToast)
		a.pendingToast = ""
	}
	a.shortcuts(c)

	bar := c.TitleBar()
	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	backdrop(c, row)
	// A curve, not a spring: a spring's long tail crawls under a pixel a
	// frame, which whole pixels turn into hops and holds, a shake as it
	// settles. This one lands in about 13 frames, moving in every one.
	progress := row.AnimateTo("sidebar", b2f(!a.prefs.SidebarHidden), sidebarMotion)
	a.chrome = chromeFor(progress, a.prefs.SidebarWidth, bar)
	shown := a.chrome.shown
	// The right sidebar, beside a session's panes, comes and goes as the
	// left one does, in whole pixels, as the user shows or hides it: it
	// shows at once as the app opens with it shown, without a session for
	// a moment, and goes at once with the panes.
	work := a.workSession()
	rightW := a.rightWidth()
	// In a window too narrow for it beside the cards, it gets out of their
	// way, coming back as the window widens, unless shown anyway.
	winW, _ := c.Size()
	leftW := float32(0)
	if !a.prefs.SidebarHidden {
		leftW = a.prefs.SidebarWidth
	}
	a.side.crowded = winW > 0 && winW-leftW-rightW < minCards
	if !a.side.crowded {
		a.side.forced = false
	}
	right := float32(0)
	if p := row.AnimateTo("right", b2f(a.rightShown()), sidebarMotion); work != nil {
		right = float32(math.Round(float64(rightW * min(max(p, 0), 1))))
	}
	a.side.barW = right
	// The gaps around the cards move the window too; the right one closes
	// as the sidebar comes beside the cards.
	rightPad := float32(math.Round(float64(cardGap * (1 - right/rightW))))
	a.side.edge = rightPad
	// The cards' row's width in this frame, which the viewer's comes of:
	// the last frame's would lag a frame as the window resizes, and shake.
	a.side.rowW = winW - a.chrome.sideW - right - a.chrome.mainPad - rightPad
	a.side.cf = nil
	a.side.files.showed, a.side.files.shown = a.side.files.shown, false
	row.Children(func() {
		w := a.chrome.sideW
		side := ui.Row(c).Width(w).Shrink(0).AlignItems(ui.Stretch).ClipX()
		if h := a.local(); h.conn == connReady && h.cl != nil && !a.prefs.SidebarHidden && !a.pal.open && !a.picker.open && (a.confirm == nil || !a.confirm.open) {
			if files := side.DroppedFiles(); len(files) > 0 {
				go a.openProjectDirs(h, h.cl, slices.Clone(files), true)
			}
			if side.FileDragOver() {
				side.Background(pal.accent.Alpha(0.08)).Draw(func(p *ui.Painter, r ui.Rect) {
					p.Stroke(ui.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 8}, pal.accent.Alpha(0.55), 8, 1)
				})
			}
		}
		side.Children(func() {
			ui.Column(c).Width(a.prefs.SidebarWidth).Shrink(0).Margin(0, 0, 0, w-a.prefs.SidebarWidth).
				Opacity(0.4 + 0.6*shown).Children(func() { a.sidebar(c, pal, bar) })
		})
		main := ui.Column(c).Grow(1).MinWidth(0).Padding(cardGap, rightPad, cardGap, a.chrome.mainPad).DragWindow()
		main.Children(func() {
			if shown > 0.01 {
				a.resizeHandle(c, pal)
			}
			a.card(c, pal, bar, shown)
		})
		if right > 0 && work != nil {
			// Its content stays at its width as it slides in from the
			// window's edge.
			ui.Row(c.Key("right")).Width(right).Shrink(0).AlignItems(ui.Stretch).ClipX().Children(func() {
				ui.Column(c).Width(rightW).Shrink(0).AlignItems(ui.Stretch).Opacity(0.4 + 0.6*right/rightW).
					Children(func() { a.rightSidebar(c, pal, work) })
			})
			// Over the cards and the sidebar both, which would take the
			// pointer from it on either side of their edge.
			a.rightHandle(c, right)
		}
	})
	a.mainControls(c, pal, rightPad)
	// A page staged for a screenshot takes the window's size, under its
	// content, which hides it: at no opacity, it paints no hole there.
	for id, e := range a.webs {
		if e.staged && e.view != nil {
			ui.Box(c.Key("staged-" + id)).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Stretch).Opacity(0).Children(func() {
				ui.WebView(c.Key("web"), e.view).Grow(1)
			})
		}
	}
	a.paletteView(c, pal)
	a.pickerView(c, pal)
	a.confirmView(c, pal)
	a.worktreeView(c, pal)
	// A preview that showed no more fades in anew next time.
	if !a.previewShown {
		a.previewKey = ""
	}
	a.previewShown = false
	// An image previewed no more closes, and lets go of its bitmap.
	if !a.peek.shown {
		a.closePeek()
		a.peek = peek{}
	}
	a.peek.shown = false
	a.backupEdits(c)
	if later := a.later; len(later) > 0 {
		a.later = nil
		for _, fn := range later {
			fn()
		}
		c.Invalidate()
	}
}

// resizeHandle is the sidebar's edge, which a drag moves.
func (a *App) resizeHandle(c *ui.Context, pal *palette) {
	// A drag leaves the keyboard in the pane the user is in, unless that
	// pane is a browser: its page cannot hold it while the button is down.
	h := ui.Box(c.Key("resize")).Absolute().Left(-4).Top(0).Bottom(0).Width(9).Cursor(ui.CursorResizeColumn)
	if a.keepsFocus() {
		h.KeepFocus()
	}
	if dx, _, ok := h.Dragged(); ok {
		if a.sideDrag == 0 {
			a.sideDrag = a.prefs.SidebarWidth
		}
		a.sideDrag += dx
		a.prefs.SidebarWidth = min(max(a.sideDrag, minSidebar), maxSidebar)
	} else if a.sideDrag != 0 && !h.Pressed() {
		a.sideDrag = 0
		a.savePrefs()
	}
	// The handle has the pointer, or had it and let it go: the pane the
	// user is in takes the keyboard back, which the press took from it
	// (#398).
	a.handlePress("resize", h)
	if h.DoubleClicked() {
		a.prefs.SidebarWidth = defaultSidebar
		a.savePrefs()
	}
}

// iconButton is a quiet square button with an icon, as in a toolbar.
func iconButton(c *ui.Context, key string, svg *ui.SVG, label string, pal *palette, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Size(28, 26).Radius(6).Center().TextColor(pal.textMuted).Label(label).Tooltip(label)
	if b.Pressed() {
		b.Background(pal.rowHover.Alpha(0.12))
	} else if b.Hovered() {
		b.Background(pal.rowHover)
		b.TextColor(pal.text)
	}
	b.Children(func() { ui.Icon(c, svg).Size(16, 16) })
	if b.Clicked() {
		fn()
	}
}

// tipsAt are where pointerTooltip's tips show, by key: where the pointer
// rested as each came, which they keep while they show.
var tipsAt = map[string]ui.Point{}

// pointerTooltip shows s as el's tooltip, as Element.Tooltip does, but
// where macOS shows its tooltips: below the pointer where it came to rest,
// clear of the arrow, rather than centered above el, which a long row
// puts far from where the user looks.
func pointerTooltip(c *ui.Context, el ui.Element, key, s string) {
	el.Description(s)
	const below = 18 // the arrow's height, from its tip
	px, py, over := el.PointerPosition()
	at, showing := tipsAt[key]
	if !showing {
		b := el.Bounds()
		at = ui.Point{X: b.X + px, Y: b.Y + py + below}
	}
	var spot ui.Element
	if over {
		ui.Overlay(c, func() {
			spot = ui.Box(c.Key(key+"-at")).Absolute().Left(at.X).Top(at.Y).Size(0, 0).PassThrough()
		})
	}
	shown := false
	ui.TooltipBase(c, el, func(tip ui.Element) {
		shown = true
		th := c.Theme()
		fill, text := th.Inverse, th.InverseText
		if fill == ui.Transparent {
			fill = th.Text
		}
		if text == ui.Transparent {
			text = th.Background
		}
		tip.MaxWidth(th.Space(80)).Padding(th.Space(1.25), th.Space(2)).Radius(th.Space(1.25)).
			Background(fill).TextColor(text).FontSize(th.FontSize-1).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.2))
		tip.Children(func() { ui.Text(c, s) })
		if over {
			tip.AttachTo(spot, ui.AnchorTopLeft, ui.AnchorTopLeft)
		}
	})
	if shown {
		tipsAt[key] = at
	} else {
		delete(tipsAt, key)
	}
}

func kbd(c *ui.Context, pal *palette, keys string) {
	ui.Text(c, keys).FontSize(11).TextColor(pal.textMuted).Padding(1, 5).Radius(4).
		Background(pal.kbd).SingleLine()
}

// --- the sidebar ---

func (a *App) sidebar(c *ui.Context, pal *palette, bar ui.TitleBar) {
	ui.Row(c).Height(topBar).Shrink(0).AlignItems(ui.Center).Padding(0, 12, 0, lead(bar)).Gap(2).DragWindow().Children(func() {
		// After the traffic lights on macOS, at the right; elsewhere the
		// window controls are at the right, and the buttons at the left,
		// where they stay as the sidebar hides (chrome.buttonsLeft).
		if runtime.GOOS == "darwin" {
			ui.Spacer(c)
		}
		iconButton(c, "hide-sidebar", sidebarIcon, a.tip(locales.SidebarHide(), "sidebar.toggle"), pal, a.toggleSidebar)
		iconButton(c, "new-session", plusIcon, a.tip(locales.ActionNewSession(), "session.new"), pal, func() { a.newSession("") })
	})
	// The brand, as the sidebar's reference has it: a mark and a wordmark
	// over a tagline and a subtle glowing arc, at the top of the list.
	ui.Column(c.Key("brand")).Shrink(0).Margin(-13, 0, 0, 0).Padding(6, 12, 8, 14).Gap(3).DragWindow().AlignItems(ui.Center).Children(func() {
		ui.Row(c.Key("wordmark")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, terminalIcon).Size(25, 25).TextColor(pal.accent)
			ui.Text(c, "Xview").FontSize(24).FontWeight(700).TextColor(pal.text).SingleLine()
		})
		ui.Text(c, "All In One 终端工作台").FontSize(12).TextColor(pal.textMuted).SingleLine()
		brandArc(c, pal)
	})
	// The space between and below the rows moves the window, as the top
	// bar does, and a double click there starts a session, as ⌘T does.
	list := ui.Scroll(c.Key("sessions")).Grow(1).Padding(0, 6, 16, 6).DragWindow()
	if list.WindowDoubleClicked() {
		a.defer_(func() { a.newSession("") })
	}
	list.Children(func() {
		projects := 0
		for _, h := range a.hosts {
			projects += len(a.hostGroups(h))
		}
		sidebarProjectSection(c.Key("tasks"), pal, projects, a.sessionCount())
		n := 0  // the rows shown, for ⌘1–9 (Ctrl+1–9 elsewhere)
		gi := 0 // the groups shown, the first of which sits higher
		// Where the project dragged would go, as the last frame found it;
		// this frame's groups find it again.
		at := a.projMove.at
		a.projMove.at = -1
		// This machine's projects first, unlabeled; then each host's,
		// under its name.
		if local := a.local(); local.loaded {
			for _, g := range a.hostGroups(local) {
				a.groupView(c, pal, g, gi, at, &n)
				gi++
			}
			if len(a.st.Sessions) == 0 && local.conn == connReady && len(a.hosts) == 1 {
				ui.Text(c, locales.SidebarNoSessions()).FontSize(12.5).TextColor(pal.textFaint).Padding(8, 10)
			}
		}
		// Then each host's, flat, tagged with its name: a host has a row
		// of its own only while it needs one (hostNeedsRow).
		for _, h := range a.hosts[1:] {
			groups := a.hostGroups(h)
			if a.hostNeedsRow(h) {
				top := float32(18)
				if gi == 0 {
					top = 2
				}
				a.hostLine(c, pal, h, top)
				gi++
			}
			for i, g := range groups {
				g.first = i == 0
				a.groupView(c, pal, g, gi, at, &n)
				gi++
			}
		}
		if a.set.SessionHistory {
			a.endedSections(c, pal)
		} else {
			sidebarSection(c.Key("recent"), pal, locales.EndedTitle(), "")
		}
	})
	a.sidebarFooter(c, pal)
	c.After(20 * time.Second) // times ago move on
}

func brandArc(c *ui.Context, pal *palette) {
	arc := ui.Box(c.Key("brand-arc")).AlignSelf(ui.Stretch).Height(14).Margin(4, 0, 0, 0).PassThrough()
	arc.Draw(func(p *ui.Painter, r ui.Rect) {
		const segs = 28
		yBase := r.Y + r.H - 2
		const lift = float32(6.5)
		px, py := r.X, yBase
		for i := 1; i <= segs; i++ {
			t := float32(i) / segs
			nx := r.X + r.W*t
			u := 2*t - 1
			ny := yBase - lift*(1-u*u)
			mid := 1 - abs(2*(t-0.5/segs)-1)
			alpha := 0.04 + 0.24*mid*mid
			p.Line(px, py, nx, ny, 1.2, pal.textMuted.Alpha(alpha))
			px, py = nx, ny
		}
		cx, cy := r.X+r.W/2, yBase-lift
		for _, glow := range []struct {
			rad   float32
			color ui.Color
		}{
			{7.5, pal.accent.Alpha(0.14)},
			{4.5, pal.accent.Alpha(0.32)},
			{2.6, pal.accent},
		} {
			var dot ui.Path
			dot.Circle(cx, cy, glow.rad)
			p.FillPath(&dot, glow.color)
		}
	})
}

func sidebarProjectSection(c *ui.Context, pal *palette, projects, sessions int) {
	title := strings.TrimSpace(strings.TrimSuffix(locales.SidebarProjects(projects), itoa(projects)))
	count := strings.TrimSpace(strings.TrimPrefix(locales.SidebarTasks(sessions), "·"))
	ui.Row(c).Height(28).Padding(0, 4).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(pal.textMuted).SingleLine()
		ui.Text(c, itoa(projects)).FontSize(10.5).FontWeight(600).FontFeatures("tnum").
			TextColor(pal.textMuted).Padding(1, 6).Radius(5).Background(pal.chip).SingleLine()
		ui.Spacer(c)
		ui.Text(c, count).FontSize(11.5).FontFeatures("tnum").TextColor(pal.textFaint).SingleLine()
	})
}

func sidebarSection(c *ui.Context, pal *palette, title, count string) {
	ui.Row(c).Height(30).Padding(0, 6).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(pal.textMuted).SingleLine()
		if count != "" {
			ui.Text(c, count).FontSize(11.5).TextColor(pal.textFaint).FontFeatures("tnum").SingleLine()
		}
		ui.Box(c).Grow(1).Height(1).Background(pal.textFaint.Alpha(0.18))
	})
}

func (a *App) groupView(c *ui.Context, pal *palette, g group, gi, at int, n *int) {
	// The group, its header and its rows, takes a project dragged to
	// another place: a line shows where it would go.
	grp := ui.Column(c.Key("group-" + g.key())).Shrink(0)
	a.projectTarget(grp, g, at)
	moving := g.project.ID != "" && a.projMove.id == g.project.ID
	grp.Opacity(grp.Animate("moving", 1-0.6*b2f(moving), 120*time.Millisecond))
	grp.Children(func() {
		a.groupBody(c, pal, g, gi, moving, n)
		if a.projMove.id != "" && a.projMove.host == g.host && at >= 0 {
			a.insertionLine(c, pal, g, gi, at)
		}
	})
}

func (a *App) groupBody(c *ui.Context, pal *palette, g group, gi int, moving bool, n *int) {
	id, key := g.project.ID, g.key()
	collapsed := a.prefs.Collapsed[key]
	needs := 0
	hasSelected := false
	for _, s := range g.sessions {
		if s.Status == proto.Waiting {
			needs++
		}
		if s.ID == a.selected {
			hasSelected = true
		}
	}
	top := float32(10)
	if gi == 0 {
		top = 4
	}
	borderCol := pal.sideCardBorder
	if hasSelected {
		alpha := float32(0.28)
		if !pal.dark {
			alpha = 0.24
		}
		borderCol = pal.accent.Alpha(alpha)
	}
	card := ui.Column(c.Key("card-"+key)).Margin(top, 0, 0, 0).Radius(10).
		Background(pal.sideCard).Border(1, borderCol).Clip()
	card.Children(func() {
		hdr := ui.Row(c.Key("project-"+key)).Height(34).Padding(0, 8, 0, 8).Gap(6).AlignItems(ui.Center)
		// While a project is dragged, the headers it passes do not light up.
		hovered := hdr.Hovered() && a.projMove.id == ""
		renaming := id != "" && a.renamingProject == id
		// Dragged, the header takes its project to another place among its
		// host's: a chip of it follows the pointer, and its group fades where
		// it was.
		if id != "" && !renaming && len(a.hostProjects(g.host)) > 1 {
			hdr.Drag(projectDrag{g.host, id})
			if hdr.Dragging() {
				a.projMove.host, a.projMove.id = g.host, id
				floating(hdr.Background(pal.popover), pal)
			} else if moving {
				a.defer_(func() {
					if a.projMove.id == id {
						a.projMove = projectMove{at: -1}
					}
				})
			}
		}
		if hdr.Clicked() && !renaming {
			a.toggleProject(key)
		}
		pointerTooltip(c, hdr, "tip-project-"+key, g.host.shortDir(g.project.Dir))
		hdr.ContextMenu(func(m *ui.Menu) {
			if m.Item(locales.ActionNewSession()).Chosen() {
				a.newSessionOn(g.host, id)
			}
			label := locales.SidebarCollapse()
			if collapsed {
				label = locales.SidebarExpand()
			}
			if m.Item(label).Chosen() {
				a.toggleProject(key)
			}
			if m.Item(locales.SessionRename()).Disabled(id == "").Chosen() {
				a.startProjectRename(id)
			}
			a.projectMenu(m, g)
			m.Separator()
			// A remote group's menu has its host's, as the host has no header.
			if g.host.remote() {
				m.Submenu(g.host.name, func(m *ui.Menu) { a.hostMenu(m, g.host) })
			}
			// Close All Sessions ends the sessions the project holds, not the
			// project itself, asking first as closing the project does: what
			// runs in them ends with them.
			if m.Item(locales.SidebarCloseAllSessions()).Disabled(len(g.sessions) == 0).Chosen() {
				name, count := g.project.Name, len(g.sessions)
				a.confirm = &confirmation{
					open:    true,
					title:   locales.ConfirmCloseAllSessionsTitle(name),
					detail:  locales.ConfirmCloseAllSessionsDetail(count),
					action:  locales.SidebarCloseAllSessions(),
					destroy: true,
					run:     func() { a.closeAllSessions(g) },
				}
			}
			if m.Item(locales.SidebarCloseProjectMenu()).Disabled(id == "").Chosen() {
				name, count := g.project.Name, len(g.sessions)
				a.confirm = &confirmation{
					open:    true,
					title:   locales.ConfirmCloseTitle(name),
					detail:  locales.ConfirmCloseProjectDetail(count),
					action:  locales.SidebarCloseProject(),
					destroy: true,
					run:     func() { a.call(id, "project.close", proto.IDParams{ID: id}, nil) },
				}
			}
		})
		hdr.Children(func() {
			chev := ui.Icon(c, chevronIcon).Size(9, 9).TextColor(pal.textFaint).Shrink(0)
			chev.Rotate(chev.AnimateTo("turn", 90*(1-b2f(collapsed)), ui.Snappy))
			if renaming {
				in := ui.TextInputBase(c.Key("rename"), &a.renameBuf).Grow(1).MinWidth(0).FontSize(12.5).FontWeight(600).AutoFocus()
				if in.Shortcut(0, ui.KeyEscape) {
					a.renamingProject = ""
					a.focusTerminal()
				} else if in.Submitted() || (!in.Focused() && !in.Pressed() && in.Valid() && a.renameStarted(c)) {
					a.commitProjectRename()
				}
				return
			}
			ui.Icon(c, folderIcon).Size(14, 14).TextColor(pal.textFaint).Shrink(0)
			ui.Text(c, g.project.Name).FontSize(13).FontWeight(650).TextColor(pal.text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
			// The branch its sessions share: a chip past the name, in the branch color.
			if b := groupBranch(g); b != "" {
				chip := ui.Row(c.Key("branch")).Height(18).Padding(0, 5).Gap(3).Radius(5).Background(pal.branchChip).AlignItems(ui.Center).Shrink(1).MinWidth(0)
				chip.Children(func() {
					ui.Icon(c, branchIcon).Size(9, 9).TextColor(pal.branchText).Shrink(0)
					ui.Text(c, b).FontSize(10.5).TextColor(pal.branchText).SingleLine().Ellipsis("…").MaxWidth(80)
				})
			}
			if g.host.remote() {
				a.hostTag(c, pal, g.host)
			}
			ui.Spacer(c)
			// A host with no row of its own tells how the app stands with it
			// here: reconnecting, or wanting an update or restart.
			if g.first && g.host.remote() && !a.hostNeedsRow(g.host) {
				a.hostState(c, pal, g.host)
			}
			if needs > 0 {
				ui.Text(c, locales.SidebarNeedsYou(needs)).FontSize(11.5).FontWeight(500).TextColor(pal.attention).SingleLine()
			}
			if a.set.SidebarCount {
				ui.Textf(c, "%d", len(g.sessions)).FontSize(11.5).TextColor(pal.textFaint).FontFeatures("tnum").Shrink(0)
			}
			if hovered && id != "" {
				cl := ui.ButtonBase(c.Key("close")).Size(20, 20).Radius(5).Center().TextColor(pal.textMuted).Label(locales.SidebarCloseProject()).Tooltip(locales.SidebarCloseProject())
				if cl.Hovered() {
					cl.Background(pal.rowHover).TextColor(pal.text)
				}
				cl.Children(func() { ui.Icon(c, closeIcon).Size(11, 11) })
				if cl.Clicked() {
					a.confirmCloseProject(g)
				}
			}
			add := ui.ButtonBase(c.Key("add")).Size(22, 22).Radius(6).Center().
				Border(1, pal.divider).TextColor(pal.textMuted).
				Label(locales.NewSessionIn(g.project.Name)).Tooltip(locales.ActionNewSession())
			if add.Hovered() {
				add.Background(pal.rowHover).TextColor(pal.text)
			}
			add.Children(func() { ui.Icon(c, plusIcon).Size(12, 12) })
			if add.Clicked() {
				a.newSessionOn(g.host, id)
			}
		})
		if collapsed {
			// Every session goes with the header, the one shown included.
			return
		}
		a.projectFilesRow(c, pal, g)
		if len(g.sessions) == 0 {
			return
		}
		ui.Box(c).AlignSelf(ui.Stretch).Height(1).Background(pal.divider)
		now := c.Now()
		detailed := a.set.SidebarRows != "compact"
		rowHt := float32(rowH)
		midOff := rowHt / 2
		if detailed {
			rowHt = rowH2
			midOff = 25
		}
		var workingY []float32
		curY := float32(5)
		for _, s := range g.sessions {
			if s.Status == proto.Working {
				workingY = append(workingY, curY+midOff)
			}
			curY += rowHt + 2
			if len(s.Panes) > 1 && a.prefs.Expanded[s.ID] {
				curY += float32(len(s.Panes)) * 31
			}
		}
		rows := ui.Column(c.Key("rows-"+key)).Padding(5, 6, 6, 4).Gap(2)
		rows.Children(func() {
			for i, s := range g.sessions {
				a.sessionRow(c, pal, s, n, now, i == 0, i == len(g.sessions)-1)
			}
			if len(workingY) > 0 {
				a.treeDataFlow(c, pal, key, workingY)
			}
		})
	})
}

func groupGit(g group, selected string) *proto.GitInfo {
	var first *proto.GitInfo
	for _, s := range g.sessions {
		gi := sessionGit(s)
		if gi == nil {
			continue
		}
		if s.ID == selected {
			return gi
		}
		if first == nil {
			first = gi
		}
	}
	return first
}

func (a *App) projectFilesRow(c *ui.Context, pal *palette, g group) {
	gg := groupGit(g, a.selected)
	hasDiff := gg != nil && (gg.Files > 0 || gg.Added > 0 || gg.Removed > 0)
	tab := sideFiles
	title := "Project Files"
	shortcut := a.keys("files.toggle")
	if hasDiff {
		tab = sideChanges
		title = "Files & Diff"
		shortcut = a.keys("changes.toggle")
	}
	ui.Box(c).AlignSelf(ui.Stretch).Height(1).Background(pal.divider)
	row := ui.Row(c.Key("files-"+g.key())).Height(28).Padding(0, 8, 0, 10).Gap(6).AlignItems(ui.Center)
	if len(g.sessions) > 0 {
		if row.Hovered() {
			row.Background(pal.rowHover)
		}
		if row.Clicked() {
			a.defer_(func() { a.openProjectSide(g, tab) })
		}
	}
	row.Children(func() {
		if hasDiff {
			ui.Box(c).Size(6, 6).Radius(3).Background(pal.attention).Shrink(0)
			ui.Text(c, title).FontSize(11.5).FontWeight(500).TextColor(pal.textMuted).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
		} else {
			ui.Icon(c, fileIcon).Size(12, 12).TextColor(pal.textFaint).Shrink(0)
			ui.Text(c, title).FontSize(11.5).TextColor(pal.textFaint).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
		}
		ui.Spacer(c)
		if hasDiff {
			a.lineCounts(c, pal, gg.Added, gg.Removed, 11)
			if gg.Files > 0 {
				ui.Text(c, itoa(gg.Files)).FontSize(10.5).FontWeight(600).FontFeatures("tnum").
					TextColor(pal.attention).Padding(1, 5).Radius(4).Background(pal.attnSoft).SingleLine().Shrink(0)
			}
		}
		if len(g.sessions) > 0 && shortcut != "" {
			ui.Text(c, shortcut).FontSize(10.5).TextColor(pal.textFaint).
				Padding(1, 5).Radius(4).Background(pal.chip).SingleLine().Shrink(0)
		}
	})
}

func (a *App) openProjectSide(g group, tab string) {
	if len(g.sessions) == 0 {
		return
	}
	for _, s := range g.sessions {
		if s.ID == a.selected {
			a.toggleSide(tab)
			return
		}
	}
	target := g.sessions[0]
	a.selectSession(target.ID)
	a.showTab(tab)
	if tab == sideChanges {
		a.openView(target, sideChanges, nil)
		if a.from == nil {
			a.focusViewer()
		}
	}
}

// confirmCloseProject asks before closing a project, which ends its
// sessions.
func (a *App) confirmCloseProject(g group) {
	name, count := g.project.Name, len(g.sessions)
	a.confirm = &confirmation{
		open:    true,
		title:   locales.ConfirmCloseTitle(name),
		detail:  locales.ConfirmCloseProjectDetail(count),
		action:  locales.SidebarCloseProject(),
		destroy: true,
		run:     func() { a.call(g.project.ID, "project.close", proto.IDParams{ID: g.project.ID}, nil) },
	}
}

// groupBranch is the branch a project's sessions share, for its header:
// the one branch they all have; "" when they tell of none, or of several.
func groupBranch(g group) string {
	branch := ""
	for _, s := range g.sessions {
		gi := sessionGit(s)
		if gi == nil {
			continue
		}
		if b := branchName(gi); b != "" {
			if branch == "" {
				branch = b
			} else if b != branch {
				return ""
			}
		}
	}
	return branch
}

func (a *App) sessionRow(c *ui.Context, pal *palette, s *proto.Session, n *int, now time.Time, first, last bool) {
	*n++
	selected := s.ID == a.selected
	detailed := a.set.SidebarRows != "compact"
	h := a.hostOf(s.ID)
	height := float32(rowH)
	if detailed {
		height = rowH2
	}
	borderCol := ui.Transparent
	if selected {
		borderCol = pal.rowSelBorder
	}
	row := ui.Row(c.Key(s.ID)).Height(height).Padding(0, 8, 0, 0).AlignItems(ui.Stretch).Radius(8).Border(1, borderCol)
	hovered := row.Hovered()
	bg := ui.Transparent
	switch {
	case selected:
		bg = pal.rowSelected
	case hovered:
		bg = pal.rowHover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	// A session of a host the app lost shows as it was, faded, until the
	// host is back.
	if h.remote() && h.conn != connReady {
		row.Opacity(0.55)
	}
	if (row.Pressed() || row.Clicked()) && !selected {
		a.selectSession(s.ID)
	}
	if row.DoubleClicked() {
		a.startRename(s.ID)
	}
	expanded := len(s.Panes) > 1 && a.prefs.Expanded[s.ID]
	tip := h.shortDir(s.WorkDir())
	if s.Process != "" {
		tip = s.Process + " · " + tip
	}
	if s.Message != "" {
		tip = s.Message + "\n" + tip
	}
	g := sessionGit(s)
	if g != nil {
		tip += "\n" + gitSummary(g)
	}
	pointerTooltip(c, row, "tip-"+s.ID, tip)
	row.ContextMenu(func(m *ui.Menu) { a.sessionMenu(m, s) })
	row.Children(func() {
		if a.renaming == s.ID {
			in := ui.TextInputBase(c.Key("rename"), &a.renameBuf).Grow(1).MinWidth(0).Margin(0, 10, 0, 28).FontSize(13.5).AutoFocus()
			if in.Submitted() || in.Shortcut(0, ui.KeyEscape) {
				if in.Shortcut(0, ui.KeyEscape) {
					a.renaming = ""
					a.focusTerminal()
				} else {
					a.commitRename()
				}
			} else if !in.Focused() && !in.Pressed() && in.Valid() && a.renameStarted(c) {
				a.commitRename()
			}
			return
		}
		a.sessionTree(c, pal, first, last, detailed)
		col := programColor(pal, &s.Pane)
		if s.Process == "" && !s.Browser() && s.Status != proto.Exited {
			col = pal.accent
		}
		iconBorder := pal.divider
		if selected {
			iconBorder = pal.accent.Alpha(0.35)
		}
		iconWrap := ui.Column(c).Shrink(0).Padding(0, 8, 0, 0).Justify(ui.Center)
		if detailed {
			iconWrap.Justify(ui.Start).Padding(11, 8, 0, 0)
		}
		iconWrap.Children(func() {
			ui.Box(c).Size(26, 26).Radius(7).Center().Background(pal.sideIconBg).Border(1, iconBorder).
				Children(func() { renderProgramIcon(c, &s.Pane, 14, col, false) })
		})
		body := ui.Column(c).Grow(1).MinWidth(0).Justify(ui.Center).Gap(3)
		if detailed {
			body.Padding(5, 0, 5, 0)
		}
		body.Children(func() {
			ui.Row(c).Height(20).Gap(6).AlignItems(ui.Center).Children(func() {
				name := ui.Text(c, a.sessionTitle(s)).FontSize(13).FontWeight(550).
					SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).TextColor(pal.text.Alpha(0.92))
				if selected || s.Unread {
					name.FontWeight(600).TextColor(pal.text)
				}
				if len(s.Panes) > 1 {
					a.panesToggle(c, pal, s, expanded)
				}
				ui.Spacer(c)
				if hovered {
					cl := ui.ButtonBase(c.Key("close")).Size(18, 18).Radius(5).Center().TextColor(pal.textMuted).Label(locales.ActionCloseSession()).Tooltip(locales.ActionCloseSession())
					if cl.Hovered() {
						cl.Background(pal.rowHover).TextColor(pal.text)
					}
					cl.Children(func() { ui.Icon(c, closeIcon).Size(11, 11) })
					if cl.Clicked() {
						a.defer_(func() { a.closeSession(s.ID, true) })
					}
				} else if ts := sessionTimeAgo(s, now); ts != "" {
					ui.Text(c, ts).FontSize(11).TextColor(pal.textFaint).FontFeatures("tnum").SingleLine().Shrink(0)
				}
			})
			ui.Row(c).Height(16).Gap(5).AlignItems(ui.Center).Children(func() {
				if s.Status == proto.Working {
					workingDots(c, pal, 10)
				} else {
					statusDot(c, pal, s, 6)
				}
				stCol := sessionStatusColor(pal, s)
				if s.Status == proto.Working {
					stCol = pal.accent
				}
				ui.Text(c, sessionStatusLabel(s)).FontSize(11).FontWeight(500).TextColor(stCol).SingleLine().Shrink(0)
				ui.Text(c, "·").FontSize(11).TextColor(pal.textFaint).SingleLine().Shrink(0)
				ui.Text(c, sidebarProgram(&s.Pane)).FontSize(11).TextColor(pal.textFaint).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
			})
			if detailed {
				line, msg := rowDetail(s, g, h.shortDir(s.WorkDir()))
				fg := pal.textFaint
				if msg {
					fg = pal.textMuted
				}
				ui.Text(c, line).FontSize(11).TextColor(fg).SingleLine().Ellipsis("…").MinWidth(0)
			}
		})
	})
	if expanded {
		for i := range s.Panes {
			a.paneRow(c, pal, s, &s.Panes[i], last)
		}
	}
}

func (a *App) sessionTitle(s *proto.Session) string {
	return s.SidebarTitle(a.set.SessionName, locales.SidebarNewSession())
}

func sessionTimeAgo(s *proto.Session, now time.Time) string {
	ts := s.Activity
	if ts == 0 {
		ts = s.Created
	}
	return ago(ts, now)
}

func sidebarProgram(p *proto.Pane) string {
	if p.Browser() {
		return "Browser"
	}
	if name := proto.AgentNames[p.Agent]; name != "" {
		return name
	}
	if p.Agent != "" {
		return p.Agent
	}
	if p.Process != "" {
		return p.Process
	}
	return "Shell"
}

// panesToggle is a split session's count of panes by its name, which
// shows or hides them under its row.
func (a *App) panesToggle(c *ui.Context, pal *palette, s *proto.Session, expanded bool) {
	label := locales.SidebarShowPanes()
	if expanded {
		label = locales.SidebarHidePanes()
	}
	b := ui.ButtonBase(c.Key("panes")).Height(18).Padding(0, 4).Gap(2).Radius(5).AlignItems(ui.Center).Shrink(0).
		TextColor(pal.textFaint).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(pal.chip).TextColor(pal.text)
	}
	if b.Clicked() {
		id := s.ID
		a.defer_(func() { a.toggleSessionPanes(id) })
	}
	b.Children(func() {
		ui.Textf(c, "%d", len(s.Panes)).FontSize(12).FontFeatures("tnum").SingleLine()
		chev := ui.Icon(c, chevronIcon).Size(9, 9)
		chev.Rotate(chev.AnimateTo("turn", 90*b2f(expanded), ui.Snappy))
	})
}

// paneRow is a pane of an expanded split session, under its row: its
// program, its name, where it works and its status. A click shows the
// session with that pane focused.
func (a *App) paneRow(c *ui.Context, pal *palette, s *proto.Session, p *proto.Pane, last bool) {
	sid, pid := s.ID, p.ID
	h := a.hostOf(sid)
	// Indent the pane beneath its session in the project tree.
	inset := float32(46)
	row := ui.Row(c.Key(sid+"/"+pid)).Height(28).Padding(0, 10, 0, inset).Gap(8).AlignItems(ui.Center).Radius(7).Margin(0, 0, 1, 0)
	if !last {
		row.Draw(func(p *ui.Painter, r ui.Rect) {
			x := r.X + 9
			p.Line(x, r.Y-2, x, r.Y+r.H+2, 1.25, pal.sideTree)
		})
	}
	focused := s.ID == a.selected && a.focusOf(s) == pid
	switch {
	case focused:
		row.Background(pal.rowSelected)
	case row.Hovered():
		row.Background(pal.rowHover)
	}
	if pal.dark {
		row.Border(1, ui.Transparent)
	}
	if h.remote() && h.conn != connReady {
		row.Opacity(0.55)
	}
	if row.Clicked() {
		a.defer_(func() {
			s := a.session(sid)
			if s == nil || s.PaneByID(pid) == nil {
				return
			}
			a.setFocus(s, pid)
			a.selectSession(sid)
			a.focusPane(pid)
		})
	}
	// Where it works: a branch other than the default, else a folder
	// other than the session's.
	dir := h.shortDir(p.WorkDir())
	where := ""
	if g := p.Git; g != nil && g.Default != "" && g.Branch != g.Default {
		where = branchName(g)
	} else if p.WorkDir() != s.WorkDir() && p.WorkDir() != "" {
		where = proto.DirName(p.WorkDir())
	}
	tip := dir
	if p.Message != "" {
		tip = p.Message + "\n" + tip
	}
	pointerTooltip(c, row, "tip-"+sid+"/"+pid, tip)
	row.ContextMenu(func(m *ui.Menu) {
		if s.Zoomed != pid && m.Item(locales.ActionZoomPane()).Chosen() {
			a.defer_(func() {
				if s := a.session(sid); s != nil && s.PaneByID(pid) != nil {
					a.zoomPane(s, pid)
					a.selectSession(sid)
				}
			})
		}
		if m.Item(locales.PaneClose()).Chosen() {
			a.defer_(func() { a.closePane(a.session(sid), pid, true) })
		}
	})
	row.Children(func() {
		renderProgramIcon(c, p, 13, programColor(pal, p), false)
		name := ui.Text(c, p.PaneName()).FontSize(12.5).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).TextColor(pal.textMuted)
		if focused || p.Unread {
			name.TextColor(pal.text)
		}
		if where != "" && where != p.PaneName() {
			ui.Text(c, where).FontSize(12).TextColor(pal.textFaint).SingleLine().Ellipsis("…").Shrink(3).MinWidth(24).MaxWidth(100)
		}
		ui.Spacer(c)
		statusIconP(c, pal, p.Status, p.Unread, p.Progress, 13)
	})
}

// treeElbowPath builds a vertical-to-horizontal rounded corner connector
// from (x, topY) down to (x, midY-rad), curving smoothly to (endX, midY).
func treeElbowPath(x, topY, midY, endX, rad float32) *ui.Path {
	const arcSegs = 6
	var path ui.Path
	path.MoveTo(x, topY)
	cornerY := midY - rad
	if topY < cornerY {
		path.LineTo(x, cornerY)
	}
	cx, cy := x+rad, cornerY
	for i := 1; i <= arcSegs; i++ {
		theta := math.Pi - (math.Pi/2)*float64(i)/arcSegs
		px := cx + rad*float32(math.Cos(theta))
		py := cy + rad*float32(math.Sin(theta))
		path.LineTo(px, py)
	}
	path.LineTo(endX, midY)
	return &path
}

// sessionTree connects a project's divider to its sessions with rounded
// branches in the neutral tree color.
func (a *App) sessionTree(c *ui.Context, pal *palette, first, last, detailed bool) {
	box := ui.Box(c.Key("tree")).Width(22).Shrink(0).AlignSelf(ui.Stretch).PassThrough()
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		x := r.X + 8
		endX := r.X + r.W - 2
		const rad = float32(6)
		midY := r.Y + r.H/2
		if detailed {
			midY = r.Y + 24
		}
		topY := r.Y - 2
		if first {
			topY = r.Y - 6
		}
		botY := r.Y + r.H + 1

		if last {
			p.StrokePath(treeElbowPath(x, topY, midY, endX, rad), 1.25, pal.sideTree)
			return
		}
		p.Line(x, topY, x, botY, 1.25, pal.sideTree)
		p.StrokePath(treeElbowPath(x, midY-rad, midY, endX, rad), 1.25, pal.sideTree)
	})
}

// treeDataFlow paints animated glowing data pulses flowing down the tree
// from the project divider into each working session.
func (a *App) treeDataFlow(c *ui.Context, pal *palette, key string, targets []float32) {
	flow := ui.Box(c.Key("flow-"+key)).Absolute().Left(0).Top(0).Bottom(0).Width(28).PassThrough()
	flow.Draw(func(p *ui.Painter, r ui.Rect) {
		p.AnimationFrame()
		const (
			rad         = float32(6)
			speed       = float32(95)
			stride      = float32(135)
			packetLen   = float32(48)
			slicesCount = 7
		)
		x := r.X + 13
		endX := r.X + 25
		nowSec := float64(p.Now().UnixMilli()) / 1000.0
		hi := pal.accent.Mix(ui.Hex("#ffffff"), 0.55)
		for idx, midY := range targets {
			if midY <= rad {
				continue
			}
			path := treeElbowPath(x, r.Y, r.Y+midY, endX, rad)
			totalLen := (midY - rad) + float32(math.Pi/2)*rad + (endX - (x + rad))
			p.StrokePath(path, 1.35, pal.accent.Alpha(0.22))

			offset := float32(math.Mod(nowSec*float64(speed)+float64(idx)*45, float64(stride)))
			for head := offset; head-packetLen < totalLen; head += stride {
				for k := range slicesCount {
					u0 := float32(k) / slicesCount
					u1 := float32(k+1) / slicesCount
					s0 := head - packetLen*(1-u0)
					s1 := head - packetLen*(1-u1)
					if s1 <= 0 || s0 >= totalLen {
						continue
					}
					t0 := max(float32(0), s0/totalLen)
					t1 := min(float32(1), s1/totalLen)
					if t1 <= t0+0.001 {
						continue
					}
					u := (u0 + u1) / 2
					intensity := u * u
					seg := path.Trim(t0, t1)
					p.StrokePath(seg, 5.2, pal.accent.Alpha(0.22*intensity))
					p.StrokePath(seg, 2.8, pal.accent.Alpha(0.50*intensity))
					p.StrokePath(seg, 1.6, pal.accent.Alpha(0.95*intensity))
					if k >= slicesCount-2 {
						p.StrokePath(seg, 1.3, hi.Alpha(0.85*intensity))
					}
				}
			}
		}
	})
}

// workingDots draws an animated 2x2 dot grid for a working session.
func workingDots(c *ui.Context, pal *palette, size float32) ui.Element {
	box := ui.Box(c).Size(size, size).Shrink(0)
	box.Label(locales.StatusWorking())
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		p.AnimationFrame()
		step := int(p.Now().UnixMilli()/180) % 4
		offs := [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		sp := r.W * 0.24
		rad := max(1.1, r.W*0.14)
		for i, o := range offs {
			alpha := float32(0.28)
			if i == step {
				alpha = 1
			} else if (i+1)%4 == step {
				alpha = 0.58
			}
			var dot ui.Path
			dot.Circle(cx+o[0]*sp, cy+o[1]*sp, rad)
			p.FillPath(&dot, pal.accent.Alpha(alpha))
		}
	})
	return box
}

// statusDot is a session's status as a filled dot, beside the words it
// says it in.
func statusDot(c *ui.Context, pal *palette, s *proto.Session, size float32) ui.Element {
	box := ui.Box(c).Size(size, size).Shrink(0)
	box.Label(sessionStatusLabel(s))
	col := sessionStatusColor(pal, s)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		var dot ui.Path
		dot.Circle(r.X+r.W/2, r.Y+r.H/2, r.W/2)
		p.FillPath(&dot, col)
	})
	return box
}

// sessionStatusLabel is a session's status as its row says it.
func sessionStatusLabel(s *proto.Session) string {
	switch {
	case s.Status == proto.Working:
		return locales.StatusWorking()
	case s.Status == proto.Waiting:
		return locales.StatusNeedsYou()
	case s.Status == proto.Failed:
		return locales.StatusFailed()
	case s.Status == proto.Exited:
		return locales.StatusExited()
	case s.Unread:
		return locales.StatusDone()
	}
	return locales.StatusReady()
}

// sessionStatusColor is the color of a session's status in its row.
func sessionStatusColor(pal *palette, s *proto.Session) ui.Color {
	switch {
	case s.Status == proto.Waiting:
		return pal.attention
	case s.Status == proto.Failed:
		return pal.danger
	case s.Status == proto.Exited:
		return pal.textFaint
	}
	return pal.success
}

// rowDetail is the second line of a session's detailed row: its program's
// status message, else its worktree's branch and changes, else its folder,
// dir. msg tells it is a message, which reads stronger than the others.
func rowDetail(s *proto.Session, g *proto.GitInfo, dir string) (line string, msg bool) {
	if s.Message != "" {
		// A notification's text can run over lines: the row has one.
		return strings.Join(strings.Fields(s.Message), " "), true
	}
	if g != nil {
		return gitSummary(g), false
	}
	return dir, false
}

// renameStarted keeps a rename from ending in the frame it began in,
// before its input took the focus.
func (a *App) renameStarted(c *ui.Context) bool {
	if a.renameFrames < 3 {
		a.renameFrames++
		c.Invalidate()
		return false
	}
	return true
}

func (a *App) sessionMenu(m *ui.Menu, s *proto.Session) {
	id := s.ID
	if a.menuShortcut(m.Item(locales.SessionRename()), "session.rename").Chosen() {
		a.startRename(id)
	}
	if s.Status == proto.Exited {
		if m.Item(locales.CommonRestart()).Chosen() {
			a.restartSession(id)
		}
	}
	if s.Unread {
		if m.Item(locales.SessionMarkRead()).Chosen() {
			a.call(id, "view", proto.ViewParams{ID: id}, func(error) { a.hostOf(id).viewSent = nil; a.reportView() })
		}
	}
	h := a.hostOf(id)
	m.Submenu(locales.SessionMoveToProject(), func(m *ui.Menu) {
		// A session moves among its host's projects alone.
		for _, p := range a.projectsOf(h) {
			if m.Item(p.Name).Checked(p.ID == s.Project).Disabled(p.ID == s.Project).Chosen() {
				pid := p.ID
				a.defer_(func() { a.moveSession(id, pid) })
			}
		}
	})
	if from := handoffFrom(s); from != "" {
		m.Submenu(locales.SessionHandOff(), func(m *ui.Menu) {
			for _, to := range proto.HandoffAgents {
				if to == from {
					continue
				}
				if m.Item(proto.AgentNames[to]).Chosen() {
					a.defer_(func() { a.handoff(id, to) })
				}
			}
		})
	}
	m.Separator()
	if m.Item(locales.SessionCopyId()).Chosen() {
		a.copyText = id
	}
	if m.Item(locales.SessionCopyDirectory()).Disabled(s.WorkDir() == "").Chosen() {
		a.copyText = s.WorkDir()
	}
	if m.Item(locales.SessionRevealIn(fileManager())).Disabled(s.WorkDir() == "" || h.remote()).Chosen() {
		a.openLink(id, s.WorkDir())
	}
	m.Separator()
	if a.menuShortcut(m.Item(locales.ActionCloseSession()), "session.close").Chosen() {
		a.defer_(func() { a.closeSession(id, true) })
	}
}

// sidebarFooter is the sidebar's foot: an inline list of waiting/approval
// sessions when any need attention, then the bottom bar with running/waiting
// agent counts at the left and palette, project, and settings buttons at the right.
func (a *App) sidebarFooter(c *ui.Context, pal *palette) {
	ready := a.local().conn == connReady
	var waitingList, workingList, doneList []*proto.Session
	if ready {
		waiting := func(s *proto.Session) bool { return s.Status == proto.Waiting }
		working := func(s *proto.Session) bool { return s.Status == proto.Working }
		done := func(s *proto.Session) bool { return s.Status == proto.Idle && s.Unread }
		waitingList = a.footerSessions(waiting)
		workingList = a.footerSessions(working)
		doneList = a.footerSessions(done)
		if len(waitingList) > 0 {
			a.sidebarAttentionCard(c, pal, waitingList)
		}
	}
	ui.Box(c).AlignSelf(ui.Stretch).Height(1).Background(pal.divider)
	ui.Row(c).Height(36).Shrink(0).Padding(0, 10, 2, 10).Gap(4).AlignItems(ui.Center).DragWindow().Children(func() {
		switch a.local().conn {
		case connReady:
			if a.daemonStale() {
				a.restartChip(c, pal)
			}
			compact := a.prefs.SidebarWidth < 245 || a.daemonStale() || (len(waitingList) > 0 && len(workingList) > 0 && len(doneList) > 0)
			if len(workingList) > 0 {
				a.footerChip(c, pal, proto.Working, false, workingList, compact, pal.accent, pal.textMuted, ui.Transparent,
					locales.StatusWorking(), locales.FooterWorking, locales.FooterGoToIt(), func() {
						a.jumpTo(func(s *proto.Session) bool { return s.Status == proto.Working })
					})
			}
			if len(waitingList) > 0 {
				a.footerChip(c, pal, proto.Waiting, false, waitingList, compact, pal.attention, pal.attention, pal.attnSoft,
					locales.StatusNeedsYou(), locales.FooterNeedsYou, a.tip(locales.FooterGoToIt(), "session.attention"), a.jumpToAttention)
			}
			if len(doneList) > 0 {
				a.footerChip(c, pal, proto.Idle, true, doneList, compact, pal.textMuted, pal.textMuted, ui.Transparent,
					"", locales.FooterDone, locales.FooterGoToIt(), func() {
						a.jumpTo(func(s *proto.Session) bool { return s.Status == proto.Idle && s.Unread })
					})
			}
		case connConnecting:
			a.footerNote(c, pal, pal.textFaint, locales.StatusConnecting())
		default:
			a.footerNote(c, pal, pal.danger, locales.StatusDisconnected())
		}
		ui.Spacer(c)
		iconButton(c, "palette", searchIcon, a.tip(locales.ActionGoToSession(), "palette.open"), pal, a.openPalette)
		iconButton(c, "open-project", folderIcon, a.tip(locales.ActionOpenProject(), "project.open"), pal, a.openProject)
		iconButton(c, "settings", gearIcon, a.tip(locales.SettingsTitle(), "settings.open"), pal, func() { a.openSettings("") })
	})
}

func (a *App) footerSessions(is func(*proto.Session) bool) []*proto.Session {
	var out []*proto.Session
	for _, g := range a.groups() {
		for _, s := range g.sessions {
			if h := a.hostOf(s.ID); h.remote() && h.conn != connReady {
				continue
			}
			if is(s) {
				out = append(out, s)
			}
		}
	}
	return out
}

func waitingKindText(kind string) string {
	switch kind {
	case "permission":
		return locales.PaneNeedsPermission()
	case "question":
		return locales.PaneHasQuestion()
	case "auth":
		return locales.PaneNeedsSignIn()
	}
	return ""
}

func statusPulseDot(c *ui.Context, col ui.Color, size float32) ui.Element {
	box := ui.Box(c).Size(size, size).Shrink(0)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		p.AnimationFrame()
		wave := float32(0.5 + 0.5*math.Sin(float64(p.Now().UnixMilli()%1600)/1600*2*math.Pi))
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		rad := min(r.W, r.H) / 2
		var halo ui.Path
		halo.Circle(cx, cy, rad*0.95)
		p.FillPath(&halo, col.Alpha(0.15+0.20*wave))
		var dot ui.Path
		dot.Circle(cx, cy, rad*0.55)
		p.FillPath(&dot, col)
	})
	return box
}

func (a *App) footerSessionTip(list []*proto.Session, summary, hint string) string {
	var b strings.Builder
	b.WriteString(summary)
	b.WriteString(" · ")
	b.WriteString(hint)
	limit := min(len(list), 4)
	for i := range limit {
		s := list[i]
		b.WriteByte('\n')
		b.WriteString(a.sessionTitle(s))
		b.WriteString(" · ")
		b.WriteString(sidebarProgram(&s.Pane))
		if s.Message != "" {
			b.WriteString(": ")
			b.WriteString(strings.Join(strings.Fields(s.Message), " "))
		} else if k := waitingKindText(s.Kind); k != "" {
			b.WriteString(": ")
			b.WriteString(k)
		}
	}
	if len(list) > limit {
		b.WriteString("\n+")
		b.WriteString(itoa(len(list) - limit))
	}
	return b.String()
}

func (a *App) sidebarAttentionCard(c *ui.Context, pal *palette, waiting []*proto.Session) {
	card := ui.Column(c.Key("attention-card")).Shrink(0).Margin(4, 6, 6, 6).Padding(7, 8, 6, 8).Gap(3).
		Radius(9).Background(pal.attnSoft).Border(1, pal.attention.Alpha(0.28))
	card.Children(func() {
		ui.Row(c).Height(18).Gap(6).AlignItems(ui.Center).Children(func() {
			statusPulseDot(c, pal.attention, 8)
			ui.Text(c, locales.StatusNeedsYou()).FontSize(11).FontWeight(650).TextColor(pal.attention).SingleLine()
			ui.Text(c, itoa(len(waiting))).FontSize(10).FontWeight(650).FontFeatures("tnum").
				TextColor(pal.attention).Padding(0, 5).Radius(4).Background(pal.attention.Alpha(0.16)).SingleLine()
			ui.Spacer(c)
			if k := a.keys("session.attention"); k != "" {
				ui.Text(c, k).FontSize(10).TextColor(pal.attention.Alpha(0.8)).SingleLine()
			}
		})
		shown := min(len(waiting), 3)
		for i := range shown {
			s := waiting[i]
			id := s.ID
			prog := sidebarProgram(&s.Pane)
			name := a.sessionTitle(s)
			tip := name + " · " + prog
			if s.Message != "" {
				tip += "\n" + strings.Join(strings.Fields(s.Message), " ")
			} else if k := waitingKindText(s.Kind); k != "" {
				tip += "\n" + k
			}
			b := ui.ButtonBase(c.Key("attn-"+id)).Height(24).Padding(0, 6).Gap(6).Radius(6).AlignItems(ui.Center).
				Label(name + " · " + prog).Tooltip(tip)
			if b.Hovered() {
				b.Background(pal.rowHover)
			}
			b.Children(func() {
				col := programColor(pal, &s.Pane)
				if s.Process == "" && !s.Browser() && s.Status != proto.Exited {
					col = pal.attention
				}
				renderProgramIcon(c, &s.Pane, 12, col, false)
				ui.Text(c, name).FontSize(11.5).FontWeight(550).TextColor(pal.text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
				ui.Spacer(c)
				ui.Text(c, prog).FontSize(10.5).TextColor(pal.textMuted).SingleLine().Shrink(0)
			})
			if b.Clicked() {
				a.defer_(func() {
					if target := a.session(id); target != nil {
						delete(a.prefs.Collapsed, a.groupKey(target))
						a.selectSession(id)
					}
				})
			}
		}
		if len(waiting) > shown {
			more := ui.ButtonBase(c.Key("attn-more")).Height(20).Padding(0, 6).Radius(5).AlignItems(ui.Center).
				Label(locales.FooterNeedsYou(len(waiting)))
			if more.Hovered() {
				more.Background(pal.rowHover)
			}
			more.Children(func() {
				ui.Text(c, "+"+itoa(len(waiting)-shown)+" · "+locales.FooterGoToIt()).
					FontSize(10.5).FontWeight(500).TextColor(pal.attention).SingleLine()
			})
			if more.Clicked() {
				a.jumpToAttention()
			}
		}
	})
}

// footerChip counts the sessions in list, showing a pulsing dot or status
// icon with a short label; clicking jumps to the next matching session.
func (a *App) footerChip(c *ui.Context, pal *palette, st proto.Status, unread bool, list []*proto.Session, compact bool, dotCol, fg, bg ui.Color, shortLabel string, say func(int) string, hint string, jump func()) {
	n := len(list)
	if n == 0 {
		return
	}
	words := say(n)
	b := ui.ButtonBase(c.Key("chip-"+string(st))).Height(22).Padding(0, 7).Gap(5).Radius(11).AlignItems(ui.Center).
		Label(words).Tooltip(a.footerSessionTip(list, words, hint))
	if bg != ui.Transparent {
		b.Background(bg)
	}
	if b.Hovered() {
		b.Background(pal.rowHover)
	}
	b.Children(func() {
		if st == proto.Working || st == proto.Waiting {
			statusPulseDot(c, dotCol, 8)
		} else {
			statusIcon(c, pal, st, unread, 12)
		}
		text := itoa(n)
		if !compact && shortLabel != "" {
			text = itoa(n) + " " + shortLabel
		}
		ui.Text(c, text).FontSize(11.5).FontWeight(500).TextColor(fg).FontFeatures("tnum").SingleLine()
	})
	if b.Clicked() {
		jump()
	}
}

// restartChip offers to restart kerod into this version of Kero, while it
// runs another.
func (a *App) restartChip(c *ui.Context, pal *palette) {
	b := ui.ButtonBase(c.Key("chip-restart")).Height(24).Padding(0, 8, 0, 8).Gap(6).Radius(12).AlignItems(ui.Center).
		Label(locales.CommonRestartService()).Tooltip(locales.UpdateRestartTip())
	if b.Hovered() {
		b.Background(pal.rowHover)
	}
	b.Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(pal.attention)
		ui.Text(c, locales.CommonRestartService()).FontSize(12).FontWeight(500).TextColor(pal.attention).SingleLine()
	})
	if b.Clicked() {
		a.offerRestart()
	}
}

// footerNote says how the connection to kerod is, by a dot.
func (a *App) footerNote(c *ui.Context, pal *palette, dot ui.Color, label string) {
	ui.Row(c).Gap(7).Padding(0, 6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(dot)
		ui.Text(c, label).FontSize(12).TextColor(pal.textFaint).SingleLine().
			Tooltip(locales.FooterServiceTip())
	})
}

// ago is how long ago a time in Unix milliseconds was, shortly, as
// proto.Ago writes it, in the language the app shows.
func ago(ms int64, now time.Time) string {
	if ms == 0 {
		return ""
	}
	d := now.Sub(time.UnixMilli(ms))
	switch {
	case d < time.Minute:
		return locales.AgoNow()
	case d < time.Hour:
		return locales.AgoMinutes(int(d / time.Minute))
	case d < 24*time.Hour:
		return locales.AgoHours(int(d / time.Hour))
	case d < 7*24*time.Hour:
		return locales.AgoDays(int(d / (24 * time.Hour)))
	}
	return locales.AgoWeeks(int(d / (7 * 24 * time.Hour)))
}

// statusIcon draws a session's status, as the sidebar shows it: a ring
// turning while it works, an amber ring when it needs the user, a filled
// check when it finished unseen, a faint check once seen.
func statusIcon(c *ui.Context, pal *palette, st proto.Status, unread bool, size float32) ui.Element {
	return statusIconP(c, pal, st, unread, -1, size)
}

// statusIconP is statusIcon with a program's progress (OSC 7501), 0–100,
// which a working ring fills instead of turning; -1 for none.
func statusIconP(c *ui.Context, pal *palette, st proto.Status, unread bool, progress int, size float32) ui.Element {
	box := ui.Box(c).Size(size, size).Shrink(0)
	switch {
	case st == proto.Working:
		box.Label(locales.StatusWorking())
	case st == proto.Waiting:
		box.Label(locales.StatusNeedsYou())
	case st == proto.Exited:
		box.Label(locales.StatusExited())
	case st == proto.Failed:
		box.Label(locales.StatusFailed())
	case unread:
		box.Label(locales.StatusDone())
	}
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		rad := min(r.W, r.H)/2 - 1
		switch {
		case st == proto.Working && progress >= 0:
			var track ui.Path
			track.Circle(cx, cy, rad)
			p.StrokePath(&track, 1.7, pal.text.Alpha(0.13))
			// From the top, clockwise: the circle starts at 3 o'clock.
			p.Transform(ui.Rotation(-90), cx, cy, func() {
				var arc ui.Path
				arc.Circle(cx, cy, rad)
				p.StrokePath(arc.Trim(0, max(0.02, float32(progress)/100)), 1.7, pal.text)
			})
		case st == proto.Working:
			var track ui.Path
			track.Circle(cx, cy, rad)
			p.StrokePath(&track, 1.7, pal.text.Alpha(0.13))
			p.AnimationFrame()
			deg := float32(p.Now().UnixMilli()%1000) / 1000 * 360
			p.Transform(ui.Rotation(deg), cx, cy, func() {
				var arc ui.Path
				arc.Circle(cx, cy, rad)
				p.StrokePath(arc.Trim(0, 0.3), 1.7, pal.text)
			})
		case st == proto.Waiting:
			var ring ui.Path
			ring.Circle(cx, cy, rad)
			p.StrokePath(&ring, 1.7, pal.attention)
			p.AnimationFrame()
			pulse := float32(0.5 + 0.5*math.Sin(float64(p.Now().UnixMilli()%1600)/1600*2*math.Pi))
			var dot ui.Path
			dot.Circle(cx, cy, rad*0.42)
			p.FillPath(&dot, pal.attention.Alpha(0.65+0.35*pulse))
		case st == proto.Failed:
			var disc ui.Path
			disc.Circle(cx, cy, rad+0.5)
			col := pal.danger
			if !unread {
				col = pal.danger.Alpha(0.75)
			}
			p.FillPath(&disc, col)
			p.Line(cx, cy-rad*0.45, cx, cy+rad*0.08, 1.7, pal.onDone)
			var dot ui.Path
			dot.Circle(cx, cy+rad*0.42, 1)
			p.FillPath(&dot, pal.onDone)
		case st == proto.Exited:
			col := pal.textFaint
			if unread {
				col = pal.danger
			}
			var ring ui.Path
			ring.Circle(cx, cy, rad)
			p.StrokePath(&ring, 1.5, col)
			k := rad * 0.38
			p.Line(cx-k, cy-k, cx+k, cy+k, 1.5, col)
			p.Line(cx+k, cy-k, cx-k, cy+k, 1.5, col)
		case unread:
			var disc ui.Path
			disc.Circle(cx, cy, rad+0.5)
			p.FillPath(&disc, pal.done)
			check := (&ui.Path{}).MoveTo(cx-rad*0.42, cy+0.2).LineTo(cx-rad*0.08, cy+rad*0.36).LineTo(cx+rad*0.46, cy-rad*0.34)
			p.StrokePath(check, 1.6, pal.onDone)
		default:
			check := (&ui.Path{}).MoveTo(cx-rad*0.62, cy+0.4).LineTo(cx-rad*0.12, cy+rad*0.5).LineTo(cx+rad*0.66, cy-rad*0.46)
			p.StrokePath(check, 1.5, pal.textFaint)
		}
	})
	return box
}

// --- the card ---

// workSession is the session whose panes the window shows, nil for none:
// a message shows in their place.
func (a *App) workSession() *proto.Session {
	s := a.session(a.selected)
	if s == nil || a.hostOf(s.ID).conn == connVersion || !a.loaded {
		return nil
	}
	return s
}

func (a *App) card(c *ui.Context, pal *palette, bar ui.TitleBar, shown float32) {
	s := a.session(a.selected)
	h := a.local()
	if s != nil {
		h = a.hostOf(s.ID)
	}
	if s := a.workSession(); s != nil {
		v := a.viewOf(s)
		if v == nil {
			a.panes(c, pal, bar, shown, s, true)
			return
		}
		// The panes, then the viewer.
		row := ui.Row(c.Key("with-side")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Bind(a.paneRef("with-side"))
		w := a.side.rowW
		row.Children(func() {
			ui.Column(c.Key("panes")).Grow(1).Basis(0).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
				a.panes(c, pal, bar, shown, s, false)
			})
			a.sideEdge(c, pal, w)
			a.viewerCard(c, pal, s, v, a.sideWidth(w))
		})
		return
	}
	// No session: one card, for what the app has to say.
	card := paned(ui.Column(c).Grow(1).MinHeight(0).Radius(14).Background(pal.card), pal).Clip()
	card.Children(func() {
		left := a.chrome.headerPad
		ui.Row(c).Height(headerH).Shrink(0).AlignItems(ui.Center).Padding(0, 10, 0, left).DragWindow().Children(func() {
			if shown < 0.5 {
				ui.Row(c).Absolute().Left(a.chrome.buttonsLeft(bar)).Top(0).Bottom(0).AlignItems(ui.Center).Gap(2).Opacity(1 - 2*shown).Children(func() {
					iconButton(c, "show-sidebar", sidebarIcon, a.tip(locales.SidebarShow(), "sidebar.toggle"), pal, a.toggleSidebar)
					iconButton(c, "new-session-2", plusIcon, a.tip(locales.ActionNewSession(), "session.new"), pal, func() { a.newSession("") })
				})
			}
			ui.Text(c, "Xview").FontSize(14).FontWeight(600).TextColor(pal.text)
			ui.Spacer(c)
			a.controlsRoom(c, 0)
		})
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			switch {
			case h.conn == connVersion && h.remote():
				a.message(c, pal, locales.HostVersionTitle(h.name), a.hostVersionDetail(h), locales.HostUpdateAction(h.name), func() { a.offerHostUpdate(h) })
			case h.conn == connVersion:
				a.message(c, pal, locales.UpdatedTitle(), locales.UpdatedDetail(), locales.CommonRestartService(), a.restartDaemon)
			case h.conn == connFailed && h.cl == nil && !h.remote():
				a.message(c, pal, locales.ConnCantReach(), h.err, locales.CommonTryAgain(), func() { h.retries = 0; h.conn = connConnecting; a.connect() })
			case !a.loaded:
				ui.Column(c).Fill().Center().Children(func() {
					ui.Spinner(c).Label(locales.StatusConnectingLabel())
				})
			default:
				a.emptyState(c, pal)
			}
		})
	})
}

func (a *App) message(c *ui.Context, pal *palette, title, detail, action string, fn func()) {
	ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
		ui.Text(c, title).FontSize(17).FontWeight(600).TextColor(pal.text)
		ui.Text(c, detail).FontSize(13).TextColor(pal.textMuted).MaxWidth(420).TextAlign(ui.Center).LineHeight(1.4)
		if action != "" {
			ui.Box(c).Height(8)
			if button(c, "msg-action", action, "", true, pal) {
				fn()
			}
		}
	})
}

func (a *App) emptyState(c *ui.Context, pal *palette) {
	ui.Column(c).Fill().Center().Gap(0).Padding(24).Children(func() {
		ui.Box(c).Size(52, 52).Radius(14).Center().Background(pal.chip).TextColor(pal.textMuted).Margin(0, 0, 18, 0).Children(func() {
			ui.Icon(c, terminalIcon).Size(26, 26)
		})
		ui.Text(c, locales.EmptyTitle()).FontSize(19).FontWeight(650).TextColor(pal.text)
		ui.Text(c, locales.EmptyDetail()).
			FontSize(13.5).TextColor(pal.textMuted).MaxWidth(380).TextAlign(ui.Center).LineHeight(1.45).Margin(8, 0, 22, 0)
		ui.Row(c).Gap(10).Children(func() {
			if button(c, "empty-new", locales.ActionNewSession(), a.keys("session.new"), true, pal) {
				a.newSession("")
			}
			if button(c, "empty-open", locales.ActionOpenProject(), a.keys("project.open"), false, pal) {
				a.openProject()
			}
		})
		ui.Column(c).Margin(40, 0, 0, 0).Padding(14, 16).Gap(6).Radius(10).Background(pal.chip).MaxWidth(420).Children(func() {
			ui.Text(c, locales.EmptyForAgents()).FontSize(11.5).FontWeight(600).TextColor(pal.textMuted)
			ui.Text(c, "kero new -n review -- claude\nkero prompt review \"run the tests and fix them\"\nkero wait review && kero read review").
				Font("monospace").FontSize(12).TextColor(pal.text).LineHeight(1.5).Selectable()
		})
	})
}
