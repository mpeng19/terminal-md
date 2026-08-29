// Command tmd previews a Markdown file in a centered, scrollable box that
// takes up most of the terminal, without permanently replacing what was on
// screen: the alternate screen buffer is used, so quitting restores the
// previous terminal contents.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

const version = "0.1.0"

const (
	defaultSize   = 0.75 // fraction of the terminal the box occupies
	minSize       = 0.2
	minBoxWidth   = 24
	minBoxHeight  = 6
	watchInterval = 500 * time.Millisecond
	noticeTimeout = 2 * time.Second
)

// options are the command-line settings.
type options struct {
	style string  // glamour style name or path to a JSON style; "auto" = detect
	size  float64 // fraction of the terminal width/height the box occupies
	mouse bool    // capture the mouse so the wheel scrolls the box
	watch bool    // re-render when the file changes on disk
}

// source is where the markdown came from.
type source struct {
	path  string // "" when reading from stdin
	label string // what to show in the title bar
}

func (s source) isStdin() bool { return s.path == "" }

func main() {
	opts, path := parseArgs()

	src, data, err := load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmd: %v\n", err)
		os.Exit(1)
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		// Not attached to a terminal (e.g. piped into a file or another
		// program): just print the rendered markdown.
		if err := renderPlain(data, opts.style); err != nil {
			fmt.Fprintf(os.Stderr, "tmd: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Query the terminal background before Bubble Tea takes over the tty;
	// this also warms lipgloss's cache so adaptive colors don't query later.
	detected := detectStyle()
	if opts.style == "auto" {
		opts.style = detected
	}

	progOpts := []tea.ProgramOption{tea.WithAltScreen()}
	if opts.mouse {
		progOpts = append(progOpts, tea.WithMouseCellMotion())
	}
	if _, err := tea.NewProgram(newModel(src, data, opts), progOpts...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tmd: %v\n", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

func usage() {
	fmt.Fprintf(os.Stderr, `tmd %s — preview a Markdown file in a centered box in your terminal

Usage:
  tmd [options] <file.md>
  cat file.md | tmd

Options:
  -s, --style <name|path>  glamour style: auto, dark, light, dracula, notty,
                           or a path to a glamour JSON style (default: auto)
      --size <fraction>    fraction of the terminal the box occupies (default: %.2f)
      --no-mouse           don't capture the mouse (allows text selection)
      --no-watch           don't re-render when the file changes on disk
  -v, --version            print version
  -h, --help               show this help

Keys:
  ↑/k ↓/j  scroll        space/f, b   page down/up      d/u  half page
  g / G    top / bottom  ←/h →/l      scroll sideways   r    reload
  q / esc  quit
`, version, defaultSize)
}

// parseArgs handles flags in any position (before or after the file).
func parseArgs() (options, string) {
	opts := options{style: "auto", size: defaultSize, mouse: true, watch: true}
	var positional []string
	fail := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "tmd: "+format+"\n\n", a...)
		usage()
		os.Exit(2)
	}

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		needValue := func() string {
			if hasValue {
				return value
			}
			if i+1 >= len(args) {
				fail("flag --%s needs a value", name)
			}
			i++
			return args[i]
		}
		switch name {
		case "s", "style":
			opts.style = needValue()
		case "size":
			f, err := strconv.ParseFloat(needValue(), 64)
			if err != nil {
				fail("invalid --size %q: expected a number such as 0.75", value)
			}
			opts.size = min(max(f, minSize), 1)
		case "no-mouse":
			opts.mouse = false
		case "no-watch":
			opts.watch = false
		case "v", "version":
			fmt.Println("tmd " + version)
			os.Exit(0)
		case "h", "help":
			usage()
			os.Exit(0)
		default:
			fail("unknown flag %s", arg)
		}
	}

	switch len(positional) {
	case 0:
		if term.IsTerminal(int(os.Stdin.Fd())) {
			fail("no file given")
		}
		return opts, "-"
	case 1:
		return opts, positional[0]
	default:
		fail("expected one file, got %d", len(positional))
	}
	return opts, ""
}

// load reads the markdown from a file, or from stdin when path is "-".
func load(path string) (source, []byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return source{}, nil, fmt.Errorf("reading stdin: %w", err)
		}
		return source{label: "stdin"}, data, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return source{}, nil, err
	}
	if info.IsDir() {
		return source{}, nil, fmt.Errorf("%s is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return source{}, nil, err
	}
	return source{path: path, label: path}, data, nil
}

// detectStyle picks a glamour style from the environment or the terminal's
// background color.
func detectStyle() string {
	if s := os.Getenv("GLAMOUR_STYLE"); s != "" {
		return s
	}
	if lipgloss.HasDarkBackground() {
		return "dark"
	}
	return "light"
}

// renderPlain writes the rendered markdown to stdout for non-terminal output.
func renderPlain(data []byte, style string) error {
	if style == "auto" {
		style = os.Getenv("GLAMOUR_STYLE")
		if style == "" {
			style = "notty"
		}
	}
	width := 80
	if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
		width = c
	}
	out, err := renderMarkdown(data, width, style)
	if err != nil {
		return err
	}
	_, err = fmt.Println(out)
	return err
}

// renderMarkdown renders markdown to ANSI text wrapped at width.
func renderMarkdown(data []byte, width int, style string) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		return "", fmt.Errorf("style %q: %w", style, err)
	}
	out, err := r.Render(string(data))
	if err != nil {
		return "", fmt.Errorf("rendering markdown: %w", err)
	}
	return strings.Trim(out, "\n"), nil
}

// ---------------------------------------------------------------------------
// TUI
// ---------------------------------------------------------------------------

var (
	borderStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#A0A0A0", Dark: "#585858"})
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5B3FBF", Dark: "#B39DFF"})
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#8A8A8A"})
	noticeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1F7A3F", Dark: "#7FD48A"})
	errorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF7B72"})
)

type keyMap struct {
	Quit   key.Binding
	Top    key.Binding
	Bottom key.Binding
	Reload key.Binding
}

var keys = keyMap{
	Quit:   key.NewBinding(key.WithKeys("q", "esc", "ctrl+c")),
	Top:    key.NewBinding(key.WithKeys("g", "home")),
	Bottom: key.NewBinding(key.WithKeys("G", "end")),
	Reload: key.NewBinding(key.WithKeys("r")),
}

// Messages.
type (
	tickMsg   struct{}
	reloadMsg struct {
		data []byte
		mod  time.Time
		err  error
	}
)

type model struct {
	src  source
	opts options
	data []byte
	mod  time.Time // last known modification time of src.path

	vp            viewport.Model
	width, height int // terminal size
	boxW, boxH    int // outer box size, including the border
	ready         bool

	renderErr   error
	notice      string
	noticeErr   bool
	noticeUntil time.Time
}

func newModel(src source, data []byte, opts options) model {
	m := model{src: src, opts: opts, data: data}
	if !src.isStdin() {
		if info, err := os.Stat(src.path); err == nil {
			m.mod = info.ModTime()
		}
	}
	m.vp = viewport.New(0, 0)
	m.vp.MouseWheelEnabled = opts.mouse
	m.vp.SetHorizontalStep(4)
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
			return reloadMsg{err: err}
		}
		data, err := os.ReadFile(path)
		return reloadMsg{data: data, mod: info.ModTime(), err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.rerender()
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Top):
			m.vp.GotoTop()
			return m, nil
		case key.Matches(msg, keys.Bottom):
			m.vp.GotoBottom()
			return m, nil
		case key.Matches(msg, keys.Reload):
			if m.src.isStdin() {
				m.setNotice("stdin can't be reloaded", true)
				return m, nil
			}
			return m, reloadCmd(m.src.path)
		}

	case tickMsg:
		m.expireNotice()
		return m, watchCmd(m.src.path, m.mod)

	case reloadMsg:
		var cmd tea.Cmd
		if m.opts.watch && !m.src.isStdin() {
			cmd = watchCmd(m.src.path, msg.mod)
		}
		if msg.err != nil {
			m.setNotice(msg.err.Error(), true)
			return m, cmd
		}
		m.data, m.mod = msg.data, msg.mod
		m.rerender()
		m.setNotice("reloaded", false)
		return m, cmd
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *model) setNotice(text string, isErr bool) {
	m.notice, m.noticeErr = text, isErr
	m.noticeUntil = time.Now().Add(noticeTimeout)
}

func (m *model) expireNotice() {
	if m.notice != "" && time.Now().After(m.noticeUntil) {
		m.notice = ""
	}
}

// layout sizes the box and viewport from the terminal size.
func (m *model) layout() {
	bw := int(float64(m.width)*m.opts.size + 0.5)
	bh := int(float64(m.height)*m.opts.size + 0.5)
	m.boxW = min(max(bw, minBoxWidth), m.width)
	m.boxH = min(max(bh, minBoxHeight), m.height)
	m.vp.Width = max(m.boxW-4, 1) // border + one space of padding on each side
	m.vp.Height = max(m.boxH-2, 1)
}

// rerender renders the markdown at the current viewport width, keeping the
// scroll position where possible.
func (m *model) rerender() {
	out, err := renderMarkdown(m.data, m.vp.Width, m.opts.style)
	m.renderErr = err
	if err != nil {
		out = errorStyle.Render("Error: " + err.Error())
	}
	offset := m.vp.YOffset
	m.vp.SetContent("\n" + out)
	m.vp.SetYOffset(offset)
}

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

	var right, rightStyled string
	if m.notice != "" {
		st := noticeStyle
		if m.noticeErr {
			st = errorStyle
		}
		right = " " + m.notice + " ─"
		rightStyled = " " + st.Render(m.notice) + borderStyle.Render(" ─")
	} else {
		hints := []string{"↑↓ scroll", "g/G top/end", "r reload", "q quit"}
		for len(hints) > 0 {
			text := strings.Join(hints, " · ")
			if len(left)+ansi.StringWidth(text)+3 <= inner {
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
	}
	return borderStyle.Render("╰─ ") + dimStyle.Render(pct) + borderStyle.Render(" "+strings.Repeat("─", fill)) +
		rightStyled + borderStyle.Render("╯")
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
