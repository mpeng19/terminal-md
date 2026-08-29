package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// fileConfig mirrors the TOML config file. Pointer fields distinguish
// "not set" from a zero value so the file only overrides what it mentions.
type fileConfig struct {
	Window windowConfig   `toml:"window"`
	Theme  themeConfig    `toml:"theme"`
	Keys   map[string]any `toml:"keys"`
}

type windowConfig struct {
	Size      *float64 `toml:"size"`
	MinWidth  *int     `toml:"min_width"`
	MinHeight *int     `toml:"min_height"`
	Mouse     *bool    `toml:"mouse"`
	Watch     *bool    `toml:"watch"`
}

type themeConfig struct {
	Name    string            `toml:"name"`
	Variant string            `toml:"variant"` // auto, dark or light
	Colors  map[string]string `toml:"colors"`
}

// defaultConfigPath is $TMD_CONFIG, else $XDG_CONFIG_HOME/tmd/config.toml,
// else ~/.config/tmd/config.toml.
func defaultConfigPath() string {
	if p := os.Getenv("TMD_CONFIG"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "tmd", "config.toml")
}

// loadConfig reads the config file. A missing file is not an error.
// Unknown settings are reported as warnings rather than errors so a typo
// doesn't lock the user out of the tool.
func loadConfig(path string) (cfg fileConfig, warnings []string, err error) {
	if path == "" {
		return cfg, nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil, nil
	}
	if err != nil {
		return cfg, nil, err
	}
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return cfg, nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, k := range md.Undecoded() {
		if len(k) > 0 && k[0] == "keys" {
			continue // decoded into a generic map; validated by keymap()
		}
		warnings = append(warnings, fmt.Sprintf("%s: unknown setting %q", path, k.String()))
	}
	if v := cfg.Theme.Variant; v != "" && v != "auto" && v != "dark" && v != "light" {
		return cfg, warnings, fmt.Errorf("%s: theme.variant must be auto, dark or light, not %q", path, v)
	}
	for k := range cfg.Theme.Colors {
		if _, ok := colorKeys[k]; !ok {
			return cfg, warnings, fmt.Errorf("%s: unknown theme.colors key %q (valid: %s)", path, k, colorKeyNames())
		}
	}
	return cfg, warnings, nil
}

// apply overlays the config on opts for every option the CLI didn't set.
func (c fileConfig) apply(opts *options, setByFlag map[string]bool) {
	w := c.Window
	if w.Size != nil && !setByFlag["size"] {
		opts.size = min(max(*w.Size, minSize), 1)
	}
	if w.MinWidth != nil && !setByFlag["min-width"] {
		opts.minW = max(*w.MinWidth, 0)
	}
	if w.MinHeight != nil && !setByFlag["min-height"] {
		opts.minH = max(*w.MinHeight, 0)
	}
	if w.Mouse != nil && !setByFlag["no-mouse"] {
		opts.mouse = *w.Mouse
	}
	if w.Watch != nil && !setByFlag["no-watch"] {
		opts.watch = *w.Watch
	}
	if c.Theme.Name != "" && !setByFlag["theme"] {
		opts.style = c.Theme.Name
	}
	opts.variant = c.Theme.Variant
	opts.colors = c.Theme.Colors
}

// keymap builds the key bindings: the defaults, with every action mentioned
// in [keys] (normal mode) or [keys.insert] replaced by the configured keys.
func (c fileConfig) keymap() (keymap, error) {
	km := defaultKeymap()
	normal := map[string]any{}
	var insert map[string]any
	for name, v := range c.Keys {
		if name == "insert" {
			m, ok := v.(map[string]any)
			if !ok {
				return km, errors.New("keys.insert must be a table of action = keys")
			}
			insert = m
			continue
		}
		normal[name] = v
	}
	if err := applyBindings(km.normal, normal, "keys"); err != nil {
		return km, err
	}
	if err := applyBindings(km.insert, insert, "keys.insert"); err != nil {
		return km, err
	}
	return km, nil
}

func applyBindings(b bindings, raw map[string]any, section string) error {
	for name, v := range raw {
		act := action(name)
		if _, ok := b[act]; !ok {
			return fmt.Errorf("%s: unknown action %q (valid: %s)", section, name, actionNames(b))
		}
		seqs, err := keyList(v)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", section, name, err)
		}
		b[act] = seqs
	}
	return nil
}

// keyList accepts "q" or ["q", "ctrl+c"].
func keyList(v any) ([]string, error) {
	switch v := v.(type) {
	case string:
		return []string{v}, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			s, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("expected a key name, got %v", e)
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, fmt.Errorf("expected a key name or a list of key names, got %v", v)
}

func actionNames(b bindings) string {
	names := make([]string, 0, len(b))
	for a := range b {
		names = append(names, string(a))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// writeDefaultConfig creates a commented config file at path.
func writeDefaultConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultConfigText), 0o644)
}

const defaultConfigText = `# tmd configuration. Every setting is optional; the values shown are the
# defaults. Command-line flags override this file.

[window]
# Fraction of the terminal the box occupies (0.2 – 1.0).
size = 0.75
# The box never gets smaller than this, unless the terminal is.
min_width = 100
min_height = 30
# Capture the mouse so the wheel scrolls (disables native text selection).
mouse = true
# Re-render when the file changes on disk.
watch = true

[theme]
# Built-in: default, github, github-dark, github-light, dark, light,
# dracula, tokyo-night, pink, notty. Or a path to a glamour JSON style.
name = "default"
# auto picks dark or light from the terminal background.
variant = "auto"

# Override individual colors of any theme. Values are hex ("#0969DA") or
# ANSI 256-color numbers ("39"). Keys: text, muted, heading, h1_fg, h1_bg,
# link, code, code_bg, code_theme (a chroma style name), quote, rule,
# border, accent, insert, notice, error.
[theme.colors]
# link = "#0969DA"
# border = "#3D444D"

# Normal-mode keys. A value is one key name or a list of them. Multi-key
# chords are written with spaces, e.g. "d d". Key names: letters, "space",
# "enter", "esc", "tab", "backspace", "up"/"down"/"left"/"right", "home",
# "end", "pgup"/"pgdown", "ctrl+x", "alt+x", "f1".
[keys]
quit      = ["q", "ctrl+c"]
down      = ["j", "down"]
up        = ["k", "up"]
page_down = ["space", "f", "pgdown"]
page_up   = ["b", "pgup"]
half_down = ["ctrl+d"]
half_up   = ["ctrl+u"]
top       = ["g", "home"]
bottom    = ["G", "end"]
left      = ["h", "left"]
right     = ["l", "right"]
reload    = ["r"]
edit      = ["i", "enter"]
append    = ["a"]
new_below = ["o"]
new_above = ["O"]
delete    = ["d d"]
undo      = ["u", "ctrl+z"]
redo      = ["ctrl+r"]
save      = ["ctrl+s"]

# Keys while editing a block. Everything else is typed into the block.
[keys.insert]
done = ["esc"]
save = ["ctrl+s"]
undo = ["ctrl+z"]
redo = ["ctrl+r"]
`
