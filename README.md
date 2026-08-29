# terminal-md (`tmd`)

Preview a Markdown file in a centered box that takes up most of your terminal.

```sh
tmd README.md
```

`tmd` opens a bordered, scrollable box centered in the terminal (75% of the
width and height by default), renders the Markdown inside it with
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
      --no-mouse           don't capture the mouse (allows text selection)
      --no-watch           don't re-render when the file changes on disk
  -v, --version            print version
  -h, --help               show this help
```

### Keys

| Key                | Action               |
|--------------------|----------------------|
| `↓` / `j`, `↑` / `k` | scroll a line        |
| `space` / `f`, `b` | page down / up       |
| `d` / `u`          | half page down / up  |
| `g` / `G`          | top / bottom         |
| `→` / `l`, `←` / `h` | scroll sideways (wide code blocks) |
| `r`                | reload the file      |
| `q` / `esc`        | quit                 |

The mouse wheel scrolls too. Because mouse capture disables the terminal's
native text selection, pass `--no-mouse` if you want to copy text out of the box.

### Behaviour

- **Live reload.** The file is polled twice a second and re-rendered when it
  changes, so you can keep `tmd` open next to your editor. Disable with
  `--no-watch`.
- **Resizing.** The box re-lays-out and re-wraps when the terminal is resized.
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
