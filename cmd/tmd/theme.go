package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

// theme bundles the glamour style used to render markdown with the colors
// used for the box around it.
type theme struct {
	name   string
	styles ansi.StyleConfig
	chrome chrome
}

// chrome styles the parts of the UI that aren't markdown.
type chrome struct {
	border lipgloss.Style // box outline
	title  lipgloss.Style // file name in the top border
	dim    lipgloss.Style // hints, scroll percentage, hidden source
	notice lipgloss.Style // success messages
	error  lipgloss.Style // error messages
	cursor lipgloss.Style // marker for the current block
	insert lipgloss.Style // marker and label while editing
}

// colorKeys are the names accepted in [theme.colors] and used to define
// the built-in palettes.
var colorKeys = map[string]string{
	"text":       "body text",
	"muted":      "secondary text: hints, H5/H6, image captions",
	"heading":    "H2–H4",
	"h1_fg":      "H1 text (with h1_bg it becomes a highlighted block)",
	"h1_bg":      "H1 background, or \"none\"",
	"link":       "links",
	"code":       "inline code text",
	"code_bg":    "inline code background, or \"none\"",
	"code_theme": "chroma style for code blocks, e.g. github, dracula, monokai",
	"quote":      "block quotes",
	"rule":       "horizontal rules",
	"border":     "box border",
	"accent":     "title and cursor bar",
	"insert":     "editing marker and INSERT label",
	"notice":     "status messages",
	"error":      "error messages",
}

func colorKeyNames() string {
	names := make([]string, 0, len(colorKeys))
	for k := range colorKeys {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

var builtinThemes = map[string]string{
	"default":      "glamour's dark/light style with clean headings (the default)",
	"github":       "GitHub's markdown colors, light or dark to match the terminal",
	"github-dark":  "GitHub dark",
	"github-light": "GitHub light",
	"dark":         "glamour dark",
	"light":        "glamour light",
	"dracula":      "glamour dracula",
	"tokyo-night":  "glamour tokyo-night",
	"pink":         "glamour pink",
	"notty":        "no colors (used automatically when piping)",
}

// themeNames lists the built-in theme names for help text.
func themeNames() string {
	names := make([]string, 0, len(builtinThemes))
	for n := range builtinThemes {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// loadTheme resolves a theme by name (or path to a glamour JSON style),
// picks the dark or light variant where the theme adapts, and applies any
// color overrides.
func loadTheme(name string, dark bool, overrides map[string]string) (theme, error) {
	var th theme
	switch name {
	case "", "auto", "default":
		th = baseTheme("default", dark)
	case "github":
		th = baseTheme("github", dark)
		th.apply(githubPalette(dark))
	case "github-dark", "github-light":
		d := name == "github-dark"
		th = baseTheme(name, d)
		th.apply(githubPalette(d))
	default:
		if cfg, ok := styles.DefaultStyles[name]; ok {
			th = theme{name: name, styles: *cfg, chrome: defaultChrome()}
			if name != "notty" && name != "ascii" {
				cleanHeadings(&th.styles) // plain text keeps its '#' markers
			}
			break
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return theme{}, fmt.Errorf("unknown theme %q (built-in: %s) and not a readable file: %w", name, themeNames(), err)
		}
		var cfg ansi.StyleConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return theme{}, fmt.Errorf("theme file %s: %w", name, err)
		}
		th = theme{name: name, styles: cfg, chrome: defaultChrome()}
	}
	th.apply(overrides)
	return th, nil
}

// baseTheme is glamour's dark or light style with clean headings.
func baseTheme(name string, dark bool) theme {
	cfg := styles.LightStyleConfig
	if dark {
		cfg = styles.DarkStyleConfig
	}
	cleanHeadings(&cfg)
	return theme{name: name, styles: cfg, chrome: defaultChrome()}
}

// cleanHeadings drops the literal "##" markers glamour puts in front of
// headings and gives each level a distinct look instead. Terminals can't
// change font size, so hierarchy comes from weight, underline and color.
func cleanHeadings(cfg *ansi.StyleConfig) {
	t, f := true, false
	cfg.H2.Prefix = ""
	cfg.H2.Underline = &t
	cfg.H2.Bold = &t
	cfg.H3.Prefix = ""
	cfg.H3.Bold = &t
	cfg.H4.Prefix = ""
	cfg.H4.Bold = &t
	cfg.H4.Italic = &t
	cfg.H5.Prefix = ""
	cfg.H5.Bold = &f
	cfg.H5.Italic = &t
	cfg.H6.Prefix = ""
	cfg.H6.Bold = &f
	cfg.H6.Italic = &t
	cfg.H6.Faint = &t
}

// apply sets the colors named in overrides (see colorKeys) on the theme.
func (th *theme) apply(overrides map[string]string) {
	cfg := &th.styles
	t, f := true, false
	str := func(s string) *string { return &s }
	optional := func(s string) *string {
		if s == "none" || s == "" {
			return nil
		}
		return &s
	}
	for _, k := range sortedKeys(overrides) {
		v := overrides[k]
		switch k {
		case "text":
			cfg.Document.Color = str(v)
			cfg.Item.Color = str(v)
			cfg.Enumeration.Color = str(v)
			cfg.Table.Color = str(v)
			cfg.CodeBlock.Color = str(v)
		case "muted":
			cfg.H5.Color = str(v)
			cfg.H6.Color = str(v)
			cfg.ImageText.Color = str(v)
			th.chrome.dim = th.chrome.dim.Foreground(lipgloss.Color(v))
		case "heading":
			cfg.Heading.Color = str(v)
			cfg.Heading.Bold = &t
		case "h1_fg":
			cfg.H1.Color = str(v)
		case "h1_bg":
			cfg.H1.BackgroundColor = optional(v)
			if cfg.H1.BackgroundColor == nil {
				cfg.H1.Prefix, cfg.H1.Suffix = "", ""
				cfg.H1.Underline = &t
			} else {
				cfg.H1.Prefix, cfg.H1.Suffix = " ", " "
				cfg.H1.Underline = &f
			}
		case "link":
			cfg.Link.Color = str(v)
			cfg.Link.Underline = &t
			cfg.LinkText.Color = str(v)
			cfg.Image.Color = str(v)
		case "code":
			cfg.Code.Color = str(v)
		case "code_bg":
			cfg.Code.BackgroundColor = optional(v)
		case "code_theme":
			cfg.CodeBlock.Chroma = nil
			cfg.CodeBlock.Theme = v
		case "quote":
			cfg.BlockQuote.Color = str(v)
		case "rule":
			cfg.HorizontalRule.Color = str(v)
		case "border":
			th.chrome.border = th.chrome.border.Foreground(lipgloss.Color(v))
		case "accent":
			th.chrome.title = th.chrome.title.Foreground(lipgloss.Color(v))
			th.chrome.cursor = th.chrome.cursor.Foreground(lipgloss.Color(v))
		case "insert":
			th.chrome.insert = th.chrome.insert.Foreground(lipgloss.Color(v))
		case "notice":
			th.chrome.notice = th.chrome.notice.Foreground(lipgloss.Color(v))
		case "error":
			th.chrome.error = th.chrome.error.Foreground(lipgloss.Color(v))
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func defaultChrome() chrome {
	return chrome{
		border: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#A0A0A0", Dark: "#585858"}),
		title:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5B3FBF", Dark: "#B39DFF"}),
		dim:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#8A8A8A"}),
		notice: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1F7A3F", Dark: "#7FD48A"}),
		error:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF7B72"}),
		cursor: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#5B3FBF", Dark: "#B39DFF"}),
		insert: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1F7A3F", Dark: "#7FD48A"}),
	}
}

// githubPalette is GitHub's markdown look (primer colors):
// https://primer.style/foundations/color
func githubPalette(dark bool) map[string]string {
	if dark {
		return map[string]string{
			"text": "#E6EDF3", "muted": "#8B949E", "heading": "#E6EDF3",
			"h1_fg": "#E6EDF3", "h1_bg": "none", "link": "#4493F8",
			"code": "#E6EDF3", "code_bg": "#343941", "code_theme": "github-dark",
			"quote": "#8B949E", "rule": "#3D444D",
			"border": "#3D444D", "accent": "#4493F8", "insert": "#3FB950",
			"notice": "#3FB950", "error": "#F85149",
		}
	}
	return map[string]string{
		"text": "#1F2328", "muted": "#59636E", "heading": "#1F2328",
		"h1_fg": "#1F2328", "h1_bg": "none", "link": "#0969DA",
		"code": "#1F2328", "code_bg": "#EFF1F3", "code_theme": "github",
		"quote": "#59636E", "rule": "#D1D9E0",
		"border": "#D1D9E0", "accent": "#0969DA", "insert": "#1A7F37",
		"notice": "#1A7F37", "error": "#CF222E",
	}
}
