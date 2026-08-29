package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	minBoxWidth   = 24
	minBoxHeight  = 6
	watchInterval = 500 * time.Millisecond
	noticeTimeout = 2500 * time.Millisecond
	hScrollStep   = 4
)

var (
	borderStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#A0A0A0", Dark: "#585858"})
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5B3FBF", Dark: "#B39DFF"})
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#8A8A8A"})
	noticeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1F7A3F", Dark: "#7FD48A"})
	errorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF7B72"})
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#5B3FBF", Dark: "#B39DFF"})
	insertStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1F7A3F", Dark: "#7FD48A"})
)

type mode int

const (
	modeNormal mode = iota
	modeInsert
)

// lineRef says which block (and which line within it) a content line shows.
// block is -1 for spacer lines.
type lineRef struct{ block, line int }

// taSnapshot is a point in the typing history of the inline editor.
type taSnapshot struct {
	value    string
	row, col int
}

// Messages.
type (
	tickMsg   struct{}
	reloadMsg struct {
		data  []byte
		mod   time.Time
		err   error
		force bool // explicit reload: apply even if there are unsaved changes
	}
)

type model struct {
	src      source
	opts     options
	keys     keymap
	doc      document
	savedSrc string    // document text as last loaded from / written to disk
	mod      time.Time // last known modification time of src.path

	undo, redo []document

	renderer  *glamour.TermRenderer
	renderErr error
	cache     map[string][]string // rendered lines per block source, at textW

	vp            viewport.Model
	width, height int // terminal size
	boxW, boxH    int // outer box size, including the border
	textW         int // columns available for text inside the box
	ready         bool

	mode       mode
	cursor     int // content line the cursor is on (normal mode)
	refs       []lineRef
	blockStart []int
	blockLen   []int

	editIdx      int
	ta           textarea.Model
	preEdit      document // document before the current edit, for undo
	taUndo       []taSnapshot
	taRedo       []taSnapshot
	lastEditKind string

	pending []string // keys of a partially typed chord
	confirm action   // action awaiting a second key press to confirm

	notice      string
	noticeErr   bool
	noticeUntil time.Time
}

func newModel(src source, data []byte, opts options) model {
	m := model{src: src, opts: opts, keys: defaultKeymap()}
	m.doc = parseDocument(string(data))
	m.savedSrc = m.doc.String()
	if !src.isStdin() {
		if info, err := os.Stat(src.path); err == nil {
			m.mod = info.ModTime()
		}
	}
	m.vp = viewport.New(0, 0)
	m.vp.KeyMap = viewport.KeyMap{} // keys are handled by the model
	m.vp.MouseWheelEnabled = opts.mouse
	m.vp.SetHorizontalStep(hScrollStep)
	return m
}

func (m model) Init() tea.Cmd {
	if m.opts.watch && !m.src.isStdin() {
		return watchCmd(m.src.path, m.mod)
	}
	return nil
}

// watchCmd polls the file and emits a reloadMsg when it changes.
func watchCmd(path string, last time.Time) tea.Cmd {
	return tea.Tick(watchInterval, func(time.Time) tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return reloadMsg{err: err, mod: last}
		}
		if !info.ModTime().After(last) {
			return tickMsg{}
		}
		data, err := os.ReadFile(path)
		return reloadMsg{data: data, mod: info.ModTime(), err: err}
	})
}

// reloadCmd re-reads the file right now.
func reloadCmd(path string) tea.Cmd {
	return func() tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return reloadMsg{err: err, force: true}
		}
		data, err := os.ReadFile(path)
		return reloadMsg{data: data, mod: info.ModTime(), err: err, force: true}
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		if m.mode == modeNormal {
			m.clampCursorToView()
			m.compose()
		}
		return m, cmd

	case tickMsg:
		m.expireNotice()
		return m, watchCmd(m.src.path, m.mod)

	case reloadMsg:
		return m.handleReload(msg)
	}

	if m.mode == modeInsert {
		// Cursor blink and other textarea housekeeping.
		cmd := m.updateTextarea(msg)
		m.compose()
		return m, cmd
	}
	return m, nil
}

// updateTextarea forwards a message to the inline editor and re-fits its
// height. The editor is made effectively unbounded first so that it never
// scrolls internally: the outer viewport does all the scrolling.
func (m *model) updateTextarea(msg tea.Msg) tea.Cmd {
	m.ta.SetHeight(1 << 20)
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	m.fitTextarea()
	return cmd
}

func (m model) handleReload(msg reloadMsg) (tea.Model, tea.Cmd) {
	var next tea.Cmd
	if m.opts.watch && !m.src.isStdin() {
		next = watchCmd(m.src.path, msg.mod)
	}
	if msg.err != nil {
		m.setNotice(msg.err.Error(), true)
		return m, next
	}
	if !msg.force && (m.dirty() || m.mode == modeInsert) {
		// Don't clobber unsaved work; remember the new mtime so we only warn once.
		m.mod = msg.mod
		m.setNotice("file changed on disk — "+m.keys.normal.firstKey(actReload)+" to reload", true)
		return m, next
	}
	m.load(msg.data, msg.mod)
	m.setNotice("reloaded", false)
	return m, next
}

// load replaces the document with data from disk.
func (m *model) load(data []byte, mod time.Time) {
	if m.mode == modeInsert {
		m.mode = modeNormal
		m.ta.Blur()
	}
	m.doc = parseDocument(string(data))
	m.savedSrc = m.doc.String()
	m.mod = mod
	m.undo, m.redo = nil, nil
	m.compose()
	m.scrollToCursor()
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := keyName(msg)

	if m.mode == modeInsert {
		act, _ := m.keys.insert.resolve([]string{key})
		switch act {
		case actDone:
			m.finishEdit()
			return m, nil
		case actSave:
			m.save()
			return m, nil
		case actUndo:
			m.typingUndo()
			return m, nil
		case actRedo:
			m.typingRedo()
			return m, nil
		}
		m.recordTyping(msg)
		cmd := m.updateTextarea(msg)
		m.compose()
		m.scrollToEditCursor()
		return m, cmd
	}

	// Normal mode. Runes that arrived together (input coalesced while we
	// were busy) are handled one key at a time.
	if msg.Type == tea.KeyRunes && !msg.Paste && len(msg.Runes) > 1 {
		var (
			next tea.Model = m
			cmd  tea.Cmd
		)
		for _, r := range msg.Runes {
			next, cmd = next.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			if cmd != nil || next.(model).mode == modeInsert {
				break
			}
		}
		return next, cmd
	}
	seq := append(append([]string(nil), m.pending...), key)
	act, prefix := m.keys.normal.resolve(seq)
	switch {
	case act != actNone:
		m.pending = nil
	case prefix:
		m.pending = seq
		return m, nil
	case len(m.pending) > 0:
		// The chord fell through; retry with this key on its own.
		m.pending = nil
		act, prefix = m.keys.normal.resolve([]string{key})
		if prefix {
			m.pending = []string{key}
			return m, nil
		}
	}
	if act == actNone {
		if key == "esc" {
			m.confirm = actNone
			m.notice = ""
		}
		return m, nil
	}
	return m.do(act)
}

// do performs a normal-mode action.
func (m model) do(act action) (tea.Model, tea.Cmd) {
	confirmed := m.confirm == act
	m.confirm = actNone
	if confirmed {
		m.notice = ""
	}
	blk := m.cursorBlock()

	switch act {
	case actQuit:
		if m.dirty() && !confirmed {
			m.confirm = actQuit
			m.setNotice("unsaved changes: press again to quit without saving, or "+m.keys.normal.firstKey(actSave)+" to save", true)
			return m, nil
		}
		return m, tea.Quit

	case actDown:
		m.moveCursor(1)
	case actUp:
		m.moveCursor(-1)
	case actPageDown:
		m.vp.PageDown()
		m.clampCursorToView()
	case actPageUp:
		m.vp.PageUp()
		m.clampCursorToView()
	case actHalfDown:
		m.vp.HalfPageDown()
		m.clampCursorToView()
	case actHalfUp:
		m.vp.HalfPageUp()
		m.clampCursorToView()
	case actTop:
		m.setCursor(0, 0)
		m.vp.GotoTop()
	case actBottom:
		m.setCursor(len(m.doc.blocks)-1, 1<<30)
		m.vp.GotoBottom()
	case actLeft:
		m.vp.ScrollLeft(hScrollStep)
	case actRight:
		m.vp.ScrollRight(hScrollStep)

	case actReload:
		if m.src.isStdin() {
			m.setNotice("stdin can't be reloaded", true)
			return m, nil
		}
		if m.dirty() && !confirmed {
			m.confirm = actReload
			m.setNotice("unsaved changes: press again to discard them and reload", true)
			return m, nil
		}
		return m, reloadCmd(m.src.path)

	case actEdit:
		return m, m.startEdit(blk, false, m.doc.clone())
	case actAppend:
		return m, m.startEdit(blk, true, m.doc.clone())
	case actNewBelow:
		before := m.doc.clone()
		i := m.doc.insertAfter(blk)
		m.compose()
		return m, m.startEdit(i, true, before)
	case actNewAbove:
		before := m.doc.clone()
		i := m.doc.insertBefore(blk)
		m.compose()
		return m, m.startEdit(i, true, before)
	case actDelete:
		if m.doc.String() == "" {
			return m, nil
		}
		m.pushUndo()
		m.doc.deleteBlock(blk)
		m.compose()
		m.setCursor(blk, 0)
		m.setNotice("block deleted — "+m.keys.normal.firstKey(actUndo)+" to undo", false)

	case actUndo:
		if len(m.undo) == 0 {
			m.setNotice("already at oldest change", true)
			break
		}
		m.redo = append(m.redo, m.doc)
		m.doc = m.undo[len(m.undo)-1]
		m.undo = m.undo[:len(m.undo)-1]
	case actRedo:
		if len(m.redo) == 0 {
			m.setNotice("already at newest change", true)
			break
		}
		m.undo = append(m.undo, m.doc)
		m.doc = m.redo[len(m.redo)-1]
		m.redo = m.redo[:len(m.redo)-1]
	case actSave:
		m.save()
	}

	m.compose()
	switch act {
	case actPageDown, actPageUp, actHalfDown, actHalfUp, actTop, actBottom, actLeft, actRight:
	default:
		m.scrollToCursor()
	}
	return m, nil
}

func (m *model) pushUndo() {
	m.undo = append(m.undo, m.doc.clone())
	m.redo = nil
}

// ---------------------------------------------------------------------------
// Editing
// ---------------------------------------------------------------------------

func newTextarea(width int) textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.MaxWidth = 0
	ta.Placeholder = "type markdown…"
	plain := lipgloss.NewStyle()
	ta.FocusedStyle.Base = plain
	ta.FocusedStyle.CursorLine = plain
	ta.FocusedStyle.Text = plain
	ta.FocusedStyle.EndOfBuffer = plain
	ta.FocusedStyle.Placeholder = dimStyle
	ta.BlurredStyle = ta.FocusedStyle
	ta.SetWidth(width)
	return ta
}

// startEdit opens block i in the inline editor. before is the document state
// to restore on undo (it may predate an inserted empty block).
func (m *model) startEdit(i int, atEnd bool, before document) tea.Cmd {
	m.preEdit = before
	m.editIdx = i
	m.ta = newTextarea(m.textW)
	m.ta.SetValue(m.doc.blocks[i].src) // leaves the cursor at the end
	if !atEnd {
		// Land on the source line that corresponds to the cursor's position
		// within the rendered block.
		off, n := m.cursorOffset(), max(m.blockLen[i], 1)
		row := off * m.ta.LineCount() / n
		for m.ta.Line() > row {
			m.ta.CursorUp()
		}
		m.ta.CursorStart()
	}
	m.taUndo, m.taRedo = nil, nil
	m.lastEditKind = ""
	m.mode = modeInsert
	cmd := m.ta.Focus()
	m.fitTextarea()
	m.compose()
	m.scrollToEditCursor()
	return cmd
}

// finishEdit writes the editor's text back into the document.
func (m *model) finishEdit() {
	row := m.ta.Line()
	m.doc.replaceBlock(m.editIdx, m.ta.Value())
	m.mode = modeNormal
	m.ta.Blur()
	if m.doc.String() != m.preEdit.String() {
		m.undo = append(m.undo, m.preEdit)
		m.redo = nil
	}
	m.compose()
	m.setCursor(m.editIdx, row)
	m.compose()
	m.scrollToCursor()
}

// fitTextarea sizes the editor to show all of its (wrapped) lines.
func (m *model) fitTextarea() {
	rows := 0
	for _, line := range strings.Split(m.ta.Value(), "\n") {
		rows += wrappedRows(line, m.ta.Width())
	}
	m.ta.SetHeight(max(rows, 1))
}

// recordTyping keeps the insert-mode undo history, coalescing runs of
// typing into word-sized steps.
func (m *model) recordTyping(msg tea.KeyMsg) {
	kind := editKind(msg)
	if kind == "" {
		m.lastEditKind = ""
		return
	}
	k := msg.String()
	boundary := kind != m.lastEditKind || (kind == "ins" && (k == " " || k == "enter"))
	if boundary {
		m.taUndo = append(m.taUndo, m.taState())
		m.taRedo = nil
	}
	m.lastEditKind = kind
}

// editKind classifies a key press as inserting ("ins"), deleting ("del") or
// not changing the text ("").
func editKind(msg tea.KeyMsg) string {
	if msg.Type == tea.KeyRunes || msg.Paste {
		return "ins"
	}
	switch msg.String() {
	case "enter", "tab", "ctrl+v", "ctrl+m":
		return "ins"
	case "backspace", "delete", "ctrl+h", "ctrl+d", "ctrl+k", "ctrl+u", "ctrl+w",
		"alt+backspace", "alt+d", "ctrl+t", "alt+c", "alt+l", "alt+u":
		return "del"
	}
	return ""
}

func (m *model) taState() taSnapshot {
	info := m.ta.LineInfo()
	return taSnapshot{value: m.ta.Value(), row: m.ta.Line(), col: info.StartColumn + info.ColumnOffset}
}

func (m *model) restoreTA(s taSnapshot) {
	m.ta.SetValue(s.value)
	for m.ta.Line() > s.row {
		m.ta.CursorUp()
	}
	m.ta.SetCursor(s.col)
	m.lastEditKind = ""
	m.fitTextarea()
	m.compose()
	m.scrollToEditCursor()
}

func (m *model) typingUndo() {
	if len(m.taUndo) == 0 {
		m.setNotice("nothing to undo", true)
		return
	}
	m.taRedo = append(m.taRedo, m.taState())
	s := m.taUndo[len(m.taUndo)-1]
	m.taUndo = m.taUndo[:len(m.taUndo)-1]
	m.restoreTA(s)
}

func (m *model) typingRedo() {
	if len(m.taRedo) == 0 {
		m.setNotice("nothing to redo", true)
		return
	}
	m.taUndo = append(m.taUndo, m.taState())
	s := m.taRedo[len(m.taRedo)-1]
	m.taRedo = m.taRedo[:len(m.taRedo)-1]
	m.restoreTA(s)
}

// currentSource is the document text including any in-progress edit.
func (m *model) currentSource() string {
	if m.mode == modeInsert {
		d := m.doc.clone()
		d.replaceBlock(m.editIdx, m.ta.Value())
		return d.String()
	}
	return m.doc.String()
}

func (m *model) dirty() bool { return m.currentSource() != m.savedSrc }

// save writes the document (including any in-progress edit) to disk.
func (m *model) save() {
	if m.src.isStdin() {
		m.setNotice("can't save: input came from stdin", true)
		return
	}
	src := m.currentSource()
	perm := os.FileMode(0o644)
	if info, err := os.Stat(m.src.path); err == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(m.src.path, []byte(src), perm); err != nil {
		m.setNotice("save failed: "+err.Error(), true)
		return
	}
	m.savedSrc = src
	if info, err := os.Stat(m.src.path); err == nil {
		m.mod = info.ModTime()
	}
	m.setNotice("saved", false)
}

// ---------------------------------------------------------------------------
// Layout & rendering
// ---------------------------------------------------------------------------

func (m *model) setNotice(text string, isErr bool) {
	m.notice, m.noticeErr = text, isErr
	m.noticeUntil = time.Now().Add(noticeTimeout)
}

func (m *model) expireNotice() {
	if m.notice != "" && m.confirm == actNone && time.Now().After(m.noticeUntil) {
		m.notice = ""
	}
}

// layout sizes the box and viewport from the terminal size: a fraction of
// the terminal, but never smaller than the configured minimum so that text
// stays readable in small windows, and never larger than the terminal.
func (m *model) layout() {
	bw := int(float64(m.width)*m.opts.size + 0.5)
	bh := int(float64(m.height)*m.opts.size + 0.5)
	bw = max(bw, m.opts.minW, minBoxWidth)
	bh = max(bh, m.opts.minH, minBoxHeight)
	m.boxW = min(bw, m.width)
	m.boxH = min(bh, m.height)
	m.vp.Width = max(m.boxW-4, 1) // border + one space of padding on each side
	m.vp.Height = max(m.boxH-2, 1)

	textW := max(m.vp.Width-2, 1) // cursor marker column + space
	if textW != m.textW || m.renderer == nil {
		m.textW = textW
		m.renderer, m.renderErr = newRenderer(textW, m.opts.style)
		m.cache = map[string][]string{}
		if m.mode == modeInsert {
			m.ta.SetWidth(textW)
			m.fitTextarea()
		}
	}
	m.compose()
	if m.mode == modeInsert {
		m.scrollToEditCursor()
	} else {
		m.scrollToCursor()
	}
}

// renderBlock renders one block's markdown to lines, cached by source.
func (m *model) renderBlock(src string) []string {
	if lines, ok := m.cache[src]; ok {
		return lines
	}
	var out string
	if m.renderErr != nil {
		out = errorStyle.Render("render error: " + m.renderErr.Error())
	} else if m.renderer != nil {
		if s, err := m.renderer.Render(src); err == nil {
			out = trimBlankLines(s)
		} else {
			out = errorStyle.Render("render error: " + err.Error())
		}
	}
	var lines []string
	if out == "" {
		// Nothing visible (e.g. an HTML comment): show the source, dimmed.
		for _, l := range strings.Split(src, "\n") {
			lines = append(lines, dimStyle.Render(l))
		}
	} else {
		lines = strings.Split(out, "\n")
	}
	m.cache[src] = lines
	return lines
}

// trimBlankLines drops leading and trailing lines that are visually empty
// (glamour pads some blocks with styled whitespace).
func trimBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(ansi.Strip(lines[start])) == "" {
		start++
	}
	for end > start && strings.TrimSpace(ansi.Strip(lines[end-1])) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}

// compose rebuilds the viewport content from the blocks, keeping the cursor
// on the same block and line.
func (m *model) compose() {
	blk, off := m.cursorBlock(), m.cursorOffset()

	lines := []string{""}
	refs := []lineRef{{-1, 0}}
	m.blockStart = make([]int, len(m.doc.blocks))
	m.blockLen = make([]int, len(m.doc.blocks))
	for i, b := range m.doc.blocks {
		var bl []string
		if m.mode == modeInsert && i == m.editIdx {
			bl = strings.Split(m.ta.View(), "\n")
		} else {
			bl = m.renderBlock(b.src)
		}
		mark := " "
		switch {
		case m.mode == modeInsert && i == m.editIdx:
			mark = insertStyle.Render("▍")
		case m.mode == modeNormal && i == blk:
			mark = cursorStyle.Render("▍")
		}
		m.blockStart[i] = len(lines)
		m.blockLen[i] = len(bl)
		for j, l := range bl {
			lines = append(lines, mark+" "+l)
			refs = append(refs, lineRef{i, j})
		}
		if i < len(m.doc.blocks)-1 {
			lines = append(lines, "")
			refs = append(refs, lineRef{-1, 0})
		}
	}
	m.refs = refs
	m.vp.SetContent(strings.Join(lines, "\n"))
	m.setCursor(blk, off)
}

// cursorBlock returns the block the cursor is on (0 if none).
func (m *model) cursorBlock() int {
	if m.cursor >= 0 && m.cursor < len(m.refs) && m.refs[m.cursor].block >= 0 {
		return m.refs[m.cursor].block
	}
	return 0
}

// cursorOffset returns the cursor's line within its block.
func (m *model) cursorOffset() int {
	if m.cursor >= 0 && m.cursor < len(m.refs) && m.refs[m.cursor].block >= 0 {
		return m.refs[m.cursor].line
	}
	return 0
}

// setCursor puts the cursor on line off of block blk, clamped.
func (m *model) setCursor(blk, off int) {
	if len(m.blockStart) == 0 {
		m.cursor = 0
		return
	}
	blk = min(max(blk, 0), len(m.blockStart)-1)
	off = min(max(off, 0), max(m.blockLen[blk]-1, 0))
	m.cursor = m.blockStart[blk] + off
}

// moveCursor moves the cursor n block lines down (or up when negative),
// skipping spacer lines.
func (m *model) moveCursor(n int) {
	dir := 1
	if n < 0 {
		dir, n = -1, -n
	}
	for ; n > 0; n-- {
		i := m.cursor + dir
		for i >= 0 && i < len(m.refs) && m.refs[i].block < 0 {
			i += dir
		}
		if i < 0 || i >= len(m.refs) {
			break
		}
		m.cursor = i
	}
}

// scrollToCursor scrolls the viewport just enough to show the cursor line.
func (m *model) scrollToCursor() { m.ensureVisible(m.cursor) }

func (m *model) ensureVisible(line int) {
	if line < m.vp.YOffset {
		m.vp.SetYOffset(line)
	} else if line >= m.vp.YOffset+m.vp.Height {
		m.vp.SetYOffset(line - m.vp.Height + 1)
	}
}

// clampCursorToView moves the cursor into the visible area after the
// viewport was scrolled independently of it.
func (m *model) clampCursorToView() {
	top, bottom := m.vp.YOffset, m.vp.YOffset+m.vp.Height-1
	if m.cursor >= top && m.cursor <= bottom {
		return
	}
	i, dir := top, 1
	if m.cursor > bottom {
		i, dir = bottom, -1
	}
	for i >= 0 && i < len(m.refs) && i >= top && i <= bottom {
		if m.refs[i].block >= 0 {
			m.cursor = i
			return
		}
		i += dir
	}
}

// scrollToEditCursor keeps the inline editor's cursor row visible.
func (m *model) scrollToEditCursor() {
	if m.editIdx >= len(m.blockStart) {
		return
	}
	row := 0
	lines := strings.Split(m.ta.Value(), "\n")
	for i := 0; i < m.ta.Line() && i < len(lines); i++ {
		row += wrappedRows(lines[i], m.ta.Width())
	}
	row += m.ta.LineInfo().RowOffset
	m.ensureVisible(m.blockStart[m.editIdx] + row)
}

// wrappedRows mirrors the textarea's soft-wrap so we can size it exactly.
func wrappedRows(line string, width int) int {
	if width <= 0 {
		return 1
	}
	var (
		rows        = 1
		rowW, wordW int
		spaces      int
	)
	for _, r := range line {
		rw := ansi.StringWidth(string(r))
		if unicode.IsSpace(r) {
			spaces++
		} else {
			wordW += rw
		}
		if spaces > 0 {
			if rowW+wordW+spaces > width {
				rows++
				rowW = wordW + spaces
			} else {
				rowW += wordW + spaces
			}
			spaces, wordW = 0, 0
		} else if wordW+rw > width {
			if rowW > 0 {
				rows++
			}
			rowW, wordW = wordW, 0
		}
	}
	if rowW+wordW+spaces >= width {
		rows++
	}
	return rows
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() string {
	if !m.ready {
		return ""
	}
	if m.width < minBoxWidth || m.height < minBoxHeight {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			dimStyle.Render("terminal too small"))
	}

	inner := m.boxW - 2 // columns between the vertical borders
	var b strings.Builder
	b.WriteString(m.topBorder(inner))
	b.WriteByte('\n')
	side := borderStyle.Render("│")
	for _, line := range strings.Split(m.vp.View(), "\n") {
		b.WriteString(side)
		b.WriteByte(' ')
		b.WriteString(fit(line, m.vp.Width))
		b.WriteByte(' ')
		b.WriteString(side)
		b.WriteByte('\n')
	}
	b.WriteString(m.bottomBorder(inner))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

// topBorder draws ╭─ title ─────╮ across inner columns.
func (m model) topBorder(inner int) string {
	title := m.src.label
	if m.dirty() {
		title += " [+]"
	}
	if maxTitle := inner - 4; maxTitle > 0 && ansi.StringWidth(title) > maxTitle {
		title = "…" + ansi.TruncateLeft(title, ansi.StringWidth(title)-maxTitle+1, "")
	} else if maxTitle <= 0 {
		title = ""
	}
	fill := inner - ansi.StringWidth(title)
	if title != "" {
		fill -= 4 // "─ " + " " + trailing "─"
		return borderStyle.Render("╭─ ") + titleStyle.Render(title) + borderStyle.Render(" "+strings.Repeat("─", fill+1)+"╮")
	}
	return borderStyle.Render("╭" + strings.Repeat("─", fill) + "╮")
}

// bottomBorder draws ╰─ 42% ──── hints ─╯ across inner columns.
func (m model) bottomBorder(inner int) string {
	var pct string
	if m.vp.TotalLineCount() <= m.vp.Height {
		pct = "all"
	} else {
		pct = fmt.Sprintf("%d%%", int(m.vp.ScrollPercent()*100+0.5))
	}
	left := "─ " + pct + " "
	leftStyled := borderStyle.Render("─ ") + dimStyle.Render(pct) + borderStyle.Render(" ")
	if m.mode == modeInsert {
		left = "─ INSERT · " + pct + " "
		leftStyled = borderStyle.Render("─ ") + insertStyle.Render("INSERT") + borderStyle.Render(" · ") + dimStyle.Render(pct) + borderStyle.Render(" ")
	}

	var right, rightStyled string
	if m.notice != "" {
		st := noticeStyle
		if m.noticeErr {
			st = errorStyle
		}
		right = " " + m.notice + " ─"
		rightStyled = " " + st.Render(m.notice) + borderStyle.Render(" ─")
	} else {
		hints := m.hints()
		for len(hints) > 0 {
			text := strings.Join(hints, " · ")
			if ansi.StringWidth(left)+ansi.StringWidth(text)+3 <= inner {
				right = " " + text + " ─"
				rightStyled = " " + dimStyle.Render(text) + borderStyle.Render(" ─")
				break
			}
			hints = hints[:len(hints)-1]
		}
	}
	fill := inner - ansi.StringWidth(left) - ansi.StringWidth(right)
	if fill < 0 {
		right, rightStyled = "", ""
		fill = inner - ansi.StringWidth(left)
		if fill < 0 {
			left, leftStyled = "", ""
			fill = inner
		}
	}
	return borderStyle.Render("╰") + leftStyled + borderStyle.Render(strings.Repeat("─", fill)) +
		rightStyled + borderStyle.Render("╯")
}

// hints lists key hints for the footer, most important first.
func (m model) hints() []string {
	k := func(b bindings, act action) string {
		return strings.NewReplacer("ctrl+", "^", "space", "␣").Replace(b.firstKey(act))
	}
	if m.mode == modeInsert {
		b := m.keys.insert
		return []string{k(b, actDone) + " done", k(b, actSave) + " save", k(b, actUndo) + " undo", k(b, actRedo) + " redo"}
	}
	b := m.keys.normal
	return []string{
		k(b, actEdit) + " edit", k(b, actNewBelow) + " new", k(b, actDelete) + " delete",
		k(b, actUndo) + " undo", k(b, actSave) + " save", k(b, actQuit) + " quit",
	}
}

// fit pads or truncates s to exactly w terminal cells.
func fit(s string, w int) string {
	sw := ansi.StringWidth(s)
	switch {
	case sw > w:
		return ansi.Truncate(s, w, "")
	case sw < w:
		return s + strings.Repeat(" ", w-sw)
	}
	return s
}
