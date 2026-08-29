# terminal-md (`tmd`)

Preview — and edit — a Markdown file in a centered box that takes up most of your terminal.

```sh
tmd README.md
```

`tmd` opens a bordered, scrollable box centered in the terminal (75% of the
width and height by default, filling more of it on small windows), renders the Markdown inside it with
[glamour](https://github.com/charmbracelet/glamour), and restores whatever was
on screen when you quit — it uses the alternate screen buffer, so your
scrollback and prompt come back untouched.

```
            ╭─ examples/demo.md ─────────────────────────────╮
            │                                                │
            │    terminal-md demo                            │
            │                                                │
            │   A quick tour of what  tmd  renders. Scroll   │
            │   with j/k or the arrow keys, jump with g/G,   │
            │   and press q to leave.                        │
            │                                                │
            │   ## Text                                      │
            │                                                │
            ╰─ 0% ──── ↑↓ scroll · g/G top/end · q quit ─────╯
```

## Install

Requires Go 1.24+ (older `go` binaries download the right toolchain
automatically when `GOTOOLCHAIN=auto`, which is the default).

```sh
git clone https://github.com/michaelpeng/terminal-md
cd terminal-md
go install ./cmd/tmd   # or: make install
```

This puts `tmd` in `$(go env GOPATH)/bin` (usually `~/go/bin`). Make sure that
directory is on your `PATH`:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Usage

```
tmd [options] <file.md>
cat file.md | tmd

Options:
  -t, --theme <name|path>  color theme: dark, default, dracula, github,
                           github-dark, github-light, light, notty, pink,
                           tokyo-night, or a path to a glamour JSON style
                           (default: auto — "default", matched to the
                           terminal background)
      --size <fraction>    fraction of the terminal the box occupies (default: 0.75)
      --min-width <cols>   box is never narrower than this, unless the terminal
                           is (default: 100)
      --min-height <rows>  box is never shorter than this, unless the terminal
                           is (default: 30)
      --no-mouse           don't capture the mouse (press v in the app to
                           release it temporarily instead)
      --no-watch           don't re-render when the file changes on disk
      --config <path>      config file (default: ~/.config/tmd/config.toml)
      --init-config        write a commented default config file and exit
  -v, --version            print version
  -h, --help               show this help
```

### Keys

`tmd` is modal, like vim: you start in **normal** mode, where a bar in the
left margin marks the current block (a paragraph, heading, list, code block…).

| Normal mode          | Action                                   |
|----------------------|------------------------------------------|
| `j` / `k`            | move the cursor a line                   |
| `↓` / `↑`            | scroll the view a line (what the wheel sends) |
| `space` / `f`, `b`   | page down / up                           |
| `ctrl+d` / `ctrl+u`  | half page down / up                      |
| `g` / `G`            | top / bottom                             |
| `→` / `l`, `←` / `h` | scroll sideways (wide code blocks)       |
| `i` / `enter`        | edit the current block                   |
| `a`                  | edit the current block, cursor at the end|
| `o` / `O`            | new block below / above                  |
| `dd`                 | delete the current block                 |
| `u` / `ctrl+z`       | undo                                     |
| `ctrl+r`             | redo                                     |
| `ctrl+s`             | save                                     |
| `v`                  | select mode: release / recapture the mouse |
| `r`                  | reload from disk                         |
| `q` / `ctrl+c`       | quit (asks first if there are unsaved changes) |

| Insert mode          | Action                                   |
|----------------------|------------------------------------------|
| `esc`                | finish editing (re-renders the block); `esc` + a key in quick succession works too |
| `ctrl+s`             | save                                     |
| `ctrl+z` / `ctrl+r`  | undo / redo typing                       |
| `ctrl+a` / `ctrl+e`  | start / end of line                      |
| `ctrl+w`, `ctrl+k`   | delete word backwards, delete to end of line |

### Mouse, scrolling and copying

The mouse wheel scrolls the box. Capturing the mouse disables the terminal's
native text selection, so press **`v`** to enter *select mode*: the mouse is
released, `SELECT` shows in the footer, and you can drag to select and copy
as usual (in tmux with `mouse on`, that's copy-on-release). Press `v` again
to resume wheel scrolling. Many terminals also let you bypass capture by
holding a modifier while dragging (Shift in most, Option in iTerm2).

Run with `--no-mouse` (or `mouse = false` in the config) if you never want
the mouse captured; the wheel then only scrolls if your terminal or tmux
turns it into `↑`/`↓` for full-screen apps.

### Editing

Press `i` on any block and it turns into an inline editor showing that
block's raw markdown, while the rest of the document stays rendered. Type
plain markdown, press `esc`, and the block re-renders in place. Blank lines
inside the editor split it into several blocks; an emptied block is removed.

- **Undo / redo** (`u`, `ctrl+z` / `ctrl+r`) work at block granularity in
  normal mode and at word granularity while typing.
- **Save** with `ctrl+s` in either mode. The title shows `[+]` while there
  are unsaved changes, and `q` asks before discarding them.
- Whitespace in unedited blocks is preserved byte-for-byte, so saving a file
  you only viewed produces no diff.

### Behaviour

- **Live reload.** The file is polled twice a second and re-rendered when it
  changes, so you can keep `tmd` open next to your editor. If you have
  unsaved edits it warns instead of reloading (`r` reloads explicitly).
  Disable with `--no-watch`.
- **Sizing.** The box takes 75% of the terminal (`--size`), but never shrinks
  below 100×30 (`--min-width` / `--min-height`) so text stays readable in
  small windows — on an 80×24 terminal it simply fills the window. It
  re-lays-out and re-wraps when the terminal is resized.
- **Themes.** `default` is glamour's dark/light style; `github` mimics
  GitHub's markdown colors. Both pick their light or dark variant from the
  terminal's background (set `GLAMOUR_STYLE=dark|light` if detection fails).
  Pass `--theme` to choose another built-in, or a path to a
  [glamour JSON style](https://github.com/charmbracelet/glamour/tree/master/styles)
  for something custom.
- **Headings.** A terminal can't change font size, so headings are shown
  without their `#` markers and get their hierarchy from styling instead:
  H1 is a highlighted block, H2 bold + underlined, H3 bold, H4 bold italic,
  H5/H6 italic and muted.
- **Piping.** When stdout is not a terminal (`tmd doc.md > out.txt`), the
  rendered Markdown is printed instead of opening the box, using the `notty`
  style unless one is given explicitly.

## What renders

Everything glamour handles — headings (ATX and setext), emphasis, links,
images, nested and task lists, definition lists, block quotes, fenced and
indented code with syntax highlighting, tables with alignment, HTML,
emoji shortcodes — plus a few things it doesn't:

- **Math.** LaTeX in `$…$`, `$$…$$`, `\(…\)` and `\[…\]` is converted to
  Unicode: `$e^{i\pi} + 1 = 0$` → e^(iπ) + 1 = 0, `$\sum_{i=1}^{n} x_i$` →
  ∑ᵢ₌₁ⁿ xᵢ, `\frac{1}{2}` → ½, `\sqrt{x^2+y^2}` → √(x² + y²), Greek letters,
  operators, arrows, `\mathbb{R}` → ℝ, accents, `cases`/`aligned`/matrix
  environments. Display math is set off as its own block with the limits of
  sums, products and integrals stacked above and below the operator:

  ```
   n
   ∑ i = (n(n+1))/2
  i=1
  ```

  Dollar amounts (`$5 and $10`) and anything in code are left alone.
- **Footnotes.** `[^1]` references and `[^1]:` definitions become
  superscripts.
- **YAML front matter** at the top of a file is shown as a `yaml` block.

`examples/kitchen-sink.md` exercises all of it.

## Configuration

Everything above — window size, theme, colors and every key binding — can be
set in `~/.config/tmd/config.toml` (`$XDG_CONFIG_HOME/tmd/config.toml`, or
`$TMD_CONFIG`, or `--config <path>`). Create one with all the defaults
spelled out and commented:

```sh
tmd --init-config
```

A smaller example:

```toml
[window]
size = 0.9          # fraction of the terminal (0.2 – 1.0)
min_width = 80      # never smaller than this, unless the terminal is
min_height = 24
mouse = true
watch = true

[theme]
name = "github"     # default, github, github-dark, github-light, dark, light,
                    # dracula, tokyo-night, pink, notty, or a glamour JSON path
variant = "auto"    # auto | dark | light

[theme.colors]      # override any theme's colors: hex or ANSI-256 numbers
link = "#0969DA"
border = "#3D444D"
code_theme = "monokai"   # chroma style for code blocks

[keys]              # normal mode; a key name or a list, chords as "d d"
quit = ["q", "ctrl+c"]
edit = ["i", "enter"]
delete = ["d d", "ctrl+x"]

[keys.insert]       # while editing a block
done = ["esc", "ctrl+g"]
save = "ctrl+s"
```

Precedence is defaults → config file → command-line flags. An action listed
under `[keys]` replaces its default keys entirely; actions you don't mention
keep theirs. Color keys: `text`, `muted`, `heading`, `h1_fg`, `h1_bg`, `link`,
`code`, `code_bg`, `code_theme`, `quote`, `rule`, `border`, `accent`,
`insert`, `notice`, `error`. Unknown settings produce a warning; unknown
actions or colors are errors.

## Development

```sh
make build     # builds ./tmd
make install   # go install
./tmd examples/demo.md
```
