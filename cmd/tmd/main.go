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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const version = "0.1.0"

const (
	defaultSize      = 0.75 // fraction of the terminal the box occupies
	minSize          = 0.2
	defaultMinWidth  = 100 // the box never gets narrower than this (unless the terminal is)
	defaultMinHeight = 30  // the box never gets shorter than this (unless the terminal is)
)

// options are the command-line settings.
type options struct {
	style string  // theme name or path to a glamour JSON style; "auto" = detect
	size  float64 // fraction of the terminal width/height the box occupies
	minW  int     // lower bound on the box width, capped at the terminal width
	minH  int     // lower bound on the box height, capped at the terminal height
	mouse bool    // capture the mouse so the wheel scrolls the box
	watch bool    // re-render when the file changes on disk

	variant string            // theme variant from config: auto, dark or light
	colors  map[string]string // theme color overrides from config
	config  string            // config file path
}

// source is where the markdown came from.
type source struct {
	path  string // "" when reading from stdin
	label string // what to show in the title bar
}

func (s source) isStdin() bool { return s.path == "" }

func main() {
	opts, path, setByFlag := parseArgs()

	cfg, warnings, err := loadConfig(opts.config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmd: %v\n", err)
		os.Exit(1)
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "tmd: warning: %s\n", w)
	}
	cfg.apply(&opts, setByFlag)
	keys, err := cfg.keymap()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmd: %s: %v\n", opts.config, err)
		os.Exit(1)
	}

	src, data, err := load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmd: %v\n", err)
		os.Exit(1)
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		// Not attached to a terminal (e.g. piped into a file or another
		// program): just print the rendered markdown, without colors unless
		// a theme was asked for on the command line.
		style := "notty"
		if setByFlag["theme"] {
			style = opts.style
		}
		if err := renderPlain(data, style, opts.colors); err != nil {
			fmt.Fprintf(os.Stderr, "tmd: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Query the terminal background before Bubble Tea takes over the tty;
	// this also warms lipgloss's cache so adaptive colors don't query later.
	dark := detectDark()
	switch opts.variant {
	case "dark":
		dark = true
	case "light":
		dark = false
	}
	th, err := loadTheme(opts.style, dark, opts.colors)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmd: %v\n", err)
		os.Exit(1)
	}

	progOpts := []tea.ProgramOption{tea.WithAltScreen()}
	if opts.mouse {
		progOpts = append(progOpts, tea.WithMouseCellMotion())
	}
	if _, err := tea.NewProgram(newModel(src, data, opts, th, keys), progOpts...).Run(); err != nil {
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
  -t, --theme <name|path>  color theme: %s,
                           or a path to a glamour JSON style (default: auto —
                           "default", matched to the terminal background)
      --size <fraction>    fraction of the terminal the box occupies (default: %.2f)
      --min-width <cols>   box is never narrower than this, unless the terminal
                           is (default: %d)
      --min-height <rows>  box is never shorter than this, unless the terminal
                           is (default: %d)
      --no-mouse           don't capture the mouse (the wheel then only works
                           if the terminal turns it into arrow keys). With
                           the mouse captured, press v to release it and
                           select text, v again to resume
      --no-watch           don't re-render when the file changes on disk
      --config <path>      config file (default: %s)
      --init-config        write a commented default config file and exit
  -v, --version            print version
  -h, --help               show this help

Config:
  Window size, theme and every key binding can be set in the config file;
  run 'tmd --init-config' to create one with the defaults spelled out.

Keys (defaults):
  j / k         next / previous block     ↑ / ↓         move a line
  space / b     page down / up            ctrl+d / u    half page down / up
  ctrl+e / y    scroll a line             h / l, ← / →  scroll sideways
  g / G         top / bottom              r             reload from disk
  i / enter     edit block                a             edit block, cursor at end
  o / O         new block below / above   dd            delete block
  u / ctrl+z    undo                      ctrl+r        redo
  ctrl+s        save                      v             select mode (releases the mouse)
  q / ctrl+c    quit
  While editing: esc finishes, ctrl+s saves, ctrl+z / ctrl+r undo / redo.
`, version, themeNames(), defaultSize, defaultMinWidth, defaultMinHeight, defaultConfigPath())
}

// parseArgs handles flags in any position (before or after the file). It
// also reports which options were set explicitly so the config file can
// fill in the rest.
func parseArgs() (options, string, map[string]bool) {
	opts := options{style: "auto", size: defaultSize, minW: defaultMinWidth, minH: defaultMinHeight,
		mouse: true, watch: true, config: defaultConfigPath()}
	set := map[string]bool{}
	initConfig := false
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
		case "t", "theme", "s", "style":
			opts.style = needValue()
			set["theme"] = true
		case "config":
			opts.config = needValue()
		case "init-config":
			initConfig = true
		case "size":
			raw := needValue()
			f, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				fail("invalid --size %q: expected a number such as 0.75", raw)
			}
			opts.size = min(max(f, minSize), 1)
			set["size"] = true
		case "min-width", "min-height":
			raw := needValue()
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				fail("invalid --%s %q: expected a non-negative integer", name, raw)
			}
			if name == "min-width" {
				opts.minW = n
			} else {
				opts.minH = n
			}
			set[name] = true
		case "mouse", "no-mouse":
			opts.mouse = name == "mouse"
			set["mouse"] = true
		case "no-watch":
			opts.watch = false
			set[name] = true
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

	if initConfig {
		if err := writeDefaultConfig(opts.config); err != nil {
			fmt.Fprintf(os.Stderr, "tmd: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("wrote " + opts.config)
		os.Exit(0)
	}

	switch len(positional) {
	case 0:
		if term.IsTerminal(int(os.Stdin.Fd())) {
			fail("no file given")
		}
		return opts, "-", set
	case 1:
		return opts, positional[0], set
	default:
		fail("expected one file, got %d", len(positional))
	}
	return opts, "", set
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

// detectDark reports whether the terminal has a dark background. GLAMOUR_STYLE
// is honoured as an escape hatch for terminals that can't be queried.
func detectDark() bool {
	switch os.Getenv("GLAMOUR_STYLE") {
	case "light":
		return false
	case "dark":
		return true
	}
	return lipgloss.HasDarkBackground()
}

// renderPlain writes the rendered markdown to stdout for non-terminal output.
func renderPlain(data []byte, style string, colors map[string]string) error {
	th, err := loadTheme(style, true, colors)
	if err != nil {
		return err
	}
	width := 80
	if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
		width = c
	}
	r, err := newRenderer(width, th)
	if err != nil {
		return err
	}
	doc := parseDocument(string(data))
	var blocks []string
	for i, b := range doc.blocks {
		blocks = append(blocks, preprocess(b.src, i == 0 && doc.hasFrontMatter()))
	}
	out, err := r.Render(strings.Join(blocks, "\n\n"))
	if err != nil {
		return fmt.Errorf("rendering markdown: %w", err)
	}
	_, err = fmt.Println(strings.Trim(out, "\n"))
	return err
}

// newRenderer builds a glamour renderer for the given wrap width and theme.
func newRenderer(width int, th theme) (*glamour.TermRenderer, error) {
	cfg := th.styles
	// Horizontal rules span the available width instead of glamour's fixed
	// "--------".
	rule := "─"
	if th.name == "notty" || th.name == "ascii" {
		rule = "-"
	}
	margin := 0
	if cfg.Document.Margin != nil {
		margin = int(*cfg.Document.Margin)
	}
	cfg.HorizontalRule.Format = "\n" + strings.Repeat(rule, max(width-2*margin, 1)) + "\n"
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(cfg),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		return nil, fmt.Errorf("theme %q: %w", th.name, err)
	}
	return r, nil
}
