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
  -s, --style <name|path>  glamour style: auto, dark, light, dracula, notty,
                           or a path to a glamour JSON style (default: auto)
      --size <fraction>    fraction of the terminal the box occupies (default: 0.75)
      --min-width <cols>   box is never narrower than this, unless the terminal
                           is (default: 100)
      --min-height <rows>  box is never shorter than this, unless the terminal
                           is (default: 30)
      --no-mouse           don't capture the mouse (allows text selection)
      --no-watch           don't re-render when the file changes on disk
  -v, --version            print version
  -h, --help               show this help
```

### Keys

`tmd` is modal, like vim: you start in **normal** mode, where a bar in the
left margin marks the current block (a paragraph, heading, list, code block…).

| Normal mode          | Action                                   |
|----------------------|------------------------------------------|
| `↓` / `j`, `↑` / `k` | move the cursor a line                   |
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
| `r`                  | reload from disk                         |
| `q` / `ctrl+c`       | quit (asks first if there are unsaved changes) |

| Insert mode          | Action                                   |
|----------------------|------------------------------------------|
| `esc`                | finish editing (re-renders the block)    |
| `ctrl+s`             | save                                     |
| `ctrl+z` / `ctrl+r`  | undo / redo typing                       |
| `ctrl+a` / `ctrl+e`  | start / end of line                      |
| `ctrl+w`, `ctrl+k`   | delete word backwards, delete to end of line |

The mouse wheel scrolls too. Because mouse capture disables the terminal's
native text selection, pass `--no-mouse` if you want to copy text out of the box.

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
- **Style.** `auto` picks `dark` or `light` from the terminal's background
  color. Set `GLAMOUR_STYLE` or pass `--style` to override.
- **Piping.** When stdout is not a terminal (`tmd doc.md > out.txt`), the
  rendered Markdown is printed instead of opening the box, using the `notty`
  style unless one is given explicitly.

## Development

```sh
make build     # builds ./tmd
make install   # go install
./tmd examples/demo.md
```
