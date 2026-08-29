package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigLoadsCleanly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := writeDefaultConfig(path); err != nil {
		t.Fatal(err)
	}
	cfg, warnings, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Errorf("default config produced warnings: %v", warnings)
	}
	km, err := cfg.keymap()
	if err != nil {
		t.Fatal(err)
	}
	def := defaultKeymap()
	for act, want := range def.normal {
		if got := km.normal[act]; len(got) != len(want) {
			t.Errorf("normal %s: got %v want %v", act, got, want)
		}
	}
	for act, want := range def.insert {
		if got := km.insert[act]; len(got) != len(want) {
			t.Errorf("insert %s: got %v want %v", act, got, want)
		}
	}
	if err := writeDefaultConfig(path); err == nil {
		t.Error("writing over an existing config should fail")
	}
}

func TestConfigOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte(`
[window]
size = 0.5
mouse = true
[theme]
name = "github"
variant = "light"
[theme.colors]
border = "#FF0000"
[keys]
quit = "x"
delete = ["d d", "ctrl+x"]
[keys.insert]
done = ["esc", "ctrl+g"]
`), 0o644)
	cfg, warnings, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	opts := options{size: defaultSize, mouse: false, watch: true, style: "auto"}
	cfg.apply(&opts, map[string]bool{"size": true})
	if opts.size != defaultSize {
		t.Errorf("size set by flag should win, got %v", opts.size)
	}
	if !opts.mouse {
		t.Error("mouse should be enabled by config")
	}
	if opts.style != "github" || opts.variant != "light" || opts.colors["border"] != "#FF0000" {
		t.Errorf("theme not applied: %+v", opts)
	}

	km, err := cfg.keymap()
	if err != nil {
		t.Fatal(err)
	}
	if got := km.normal[actQuit]; len(got) != 1 || got[0] != "x" {
		t.Errorf("quit = %v", got)
	}
	if got := km.normal[actDelete]; len(got) != 2 || got[1] != "ctrl+x" {
		t.Errorf("delete = %v", got)
	}
	if got := km.insert[actDone]; len(got) != 2 || got[1] != "ctrl+g" {
		t.Errorf("insert.done = %v", got)
	}
	if got := km.normal[actScrollDn]; len(got) != 1 || got[0] != "down" {
		t.Errorf("unmentioned actions should keep defaults, scroll_down = %v", got)
	}
	if act, _ := km.normal.resolve([]string{"ctrl+x"}); act != actDelete {
		t.Errorf("ctrl+x should resolve to delete, got %q", act)
	}
	if act, prefix := km.normal.resolve([]string{"d"}); act != actNone || !prefix {
		t.Errorf("d alone should be a chord prefix, got %q prefix=%v", act, prefix)
	}
}

func TestConfigErrors(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"bad action":  "[keys]\nfly = \"z\"\n",
		"bad variant": "[theme]\nvariant = \"sepia\"\n",
		"bad color":   "[theme.colors]\nbackground = \"#000\"\n",
		"bad toml":    "[window\nsize = 1\n",
	}
	for name, body := range cases {
		path := filepath.Join(dir, name+".toml")
		os.WriteFile(path, []byte(body), 0o644)
		cfg, _, err := loadConfig(path)
		if err == nil {
			_, err = cfg.keymap()
		}
		if err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, warnings, err := loadConfig(filepath.Join(dir, "typo.toml")); err != nil || warnings != nil {
		t.Errorf("missing config should be silent, got %v %v", warnings, err)
	}
	os.WriteFile(filepath.Join(dir, "typo.toml"), []byte("[window]\nsizee = 1\n"), 0o644)
	if _, warnings, err := loadConfig(filepath.Join(dir, "typo.toml")); err != nil || len(warnings) != 1 {
		t.Errorf("typo should warn once, got %v %v", warnings, err)
	}
}
