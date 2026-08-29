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

// palette is the handful of colors a built-in theme is generated from.
type palette struct {
	text, muted, heading, link, code, codeBg, quote, rule string
	h1Fg, h1Bg                                            string // "" = plain bold heading
	codeTheme                                             string // chroma style name
	border, accent, insert, notice, errorC                string
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

// loadTheme resolves a theme by name (or path to a glamour JSON style).
// dark selects the variant for themes that adapt to the terminal background.
func loadTheme(name string, dark bool) (theme, error) {
	switch name {
	case "", "auto", "default":
		if dark {
			return glamourTheme("default", styles.DarkStyleConfig, defaultChrome()), nil
		}
		return glamourTheme("default", styles.LightStyleConfig, defaultChrome()), nil
	case "github":
		if dark {
			return paletteTheme("github", githubDark, styles.DarkStyleConfig), nil
		}
		return paletteTheme("github", githubLight, styles.LightStyleConfig), nil
	case "github-dark":
		return paletteTheme(name, githubDark, styles.DarkStyleConfig), nil
	case "github-light":
		return paletteTheme(name, githubLight, styles.LightStyleConfig), nil
	}
	if cfg, ok := styles.DefaultStyles[name]; ok {
		return glamourTheme(name, *cfg, defaultChrome()), nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return theme{}, fmt.Errorf("unknown theme %q (built-in: %s) and not a readable file: %w", name, themeNames(), err)
	}
	var cfg ansi.StyleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return theme{}, fmt.Errorf("theme file %s: %w", name, err)
	}
	return theme{name: name, styles: cfg, chrome: defaultChrome()}, nil
}

// glamourTheme wraps one of glamour's stock styles with clean headings.
func glamourTheme(name string, cfg ansi.StyleConfig, ch chrome) theme {
	cleanHeadings(&cfg)
	return theme{name: name, styles: cfg, chrome: ch}
}

// cleanHeadings drops the literal "##" markers glamour puts in front of
// headings and gives each level a distinct look instead. Terminals can't
// change font size, so hierarchy comes from weight, underline and color.
func cleanHeadings(cfg *ansi.StyleConfig) {
	t := true
	f := false
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

// paletteTheme builds a full style from a palette, using base for
// structural settings (margins, indents, bullets) that don't change.
func paletteTheme(name string, p palette, base ansi.StyleConfig) theme {
	cfg := base
	cleanHeadings(&cfg)
	t := true
	f := false
	str := func(s string) *string { return &s }

	cfg.Document.Color = str(p.text)
	cfg.Heading.Color = str(p.heading)
	cfg.Heading.Bold = &t
	if p.h1Bg != "" {
		cfg.H1.Color, cfg.H1.BackgroundColor = str(p.h1Fg), str(p.h1Bg)
		cfg.H1.Prefix, cfg.H1.Suffix = " ", " "
	} else {
		cfg.H1.Color, cfg.H1.BackgroundColor = str(p.heading), nil
		cfg.H1.Prefix, cfg.H1.Suffix = "", ""
		cfg.H1.Underline = &t
	}
	cfg.H5.Color = str(p.muted)
	cfg.H6.Color = str(p.muted)
	cfg.BlockQuote.Color = str(p.quote)
	cfg.BlockQuote.IndentToken = str("▌ ")
	cfg.HorizontalRule.Color = str(p.rule)
	cfg.Link.Color = str(p.link)
	cfg.Link.Underline = &t
	cfg.LinkText.Color = str(p.link)
	cfg.LinkText.Bold = &f
	cfg.Image.Color = str(p.link)
	cfg.ImageText.Color = str(p.muted)
	cfg.Code.Color = str(p.code)
	cfg.Code.BackgroundColor = str(p.codeBg)
	cfg.CodeBlock.Color = str(p.text)
	cfg.CodeBlock.Chroma = nil
	cfg.CodeBlock.Theme = p.codeTheme
	cfg.Item.Color = str(p.text)
	cfg.Enumeration.Color = str(p.text)
	cfg.Table.Color = str(p.text)
	cfg.DefinitionTerm.Bold = &t

	color := func(s string) lipgloss.Color { return lipgloss.Color(s) }
	ch := chrome{
		border: lipgloss.NewStyle().Foreground(color(p.border)),
		title:  lipgloss.NewStyle().Bold(true).Foreground(color(p.accent)),
		dim:    lipgloss.NewStyle().Foreground(color(p.muted)),
		notice: lipgloss.NewStyle().Bold(true).Foreground(color(p.notice)),
		error:  lipgloss.NewStyle().Bold(true).Foreground(color(p.errorC)),
		cursor: lipgloss.NewStyle().Foreground(color(p.accent)),
		insert: lipgloss.NewStyle().Bold(true).Foreground(color(p.insert)),
	}
	return theme{name: name, styles: cfg, chrome: ch}
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

// GitHub's markdown colors (primer): https://primer.style/foundations/color
var (
	githubLight = palette{
		text: "#1F2328", muted: "#59636E", heading: "#1F2328", link: "#0969DA",
		code: "#1F2328", codeBg: "#EFF1F3", quote: "#59636E", rule: "#D1D9E0",
		codeTheme: "github",
		border:    "#D1D9E0", accent: "#0969DA", insert: "#1A7F37", notice: "#1A7F37", errorC: "#CF222E",
	}
	githubDark = palette{
		text: "#E6EDF3", muted: "#8B949E", heading: "#E6EDF3", link: "#4493F8",
		code: "#E6EDF3", codeBg: "#343941", quote: "#8B949E", rule: "#3D444D",
		codeTheme: "github-dark",
		border:    "#3D444D", accent: "#4493F8", insert: "#3FB950", notice: "#3FB950", errorC: "#F85149",
	}
)
