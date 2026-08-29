package main

import "strings"

// block is one top-level chunk of markdown: a run of non-blank lines, or a
// fenced code block (which may contain blank lines). Blocks are the unit of
// editing: each is rendered independently, and the one under the cursor can
// be swapped for a text editor showing its raw markdown.
type block struct {
	src   string // raw markdown, without a trailing newline
	blank int    // number of blank lines that followed it in the source
}

// document is a markdown file split into blocks, with enough bookkeeping to
// reproduce the original text byte-for-byte (modulo whitespace-only lines).
type document struct {
	blocks  []block
	leading int  // blank lines before the first block
	finalNL bool // whether the source ended with a newline
}

func parseDocument(src string) document {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	d := document{finalNL: strings.HasSuffix(src, "\n")}
	src = strings.TrimSuffix(src, "\n")

	var lines []string
	if src != "" || d.finalNL {
		lines = strings.Split(src, "\n")
	}

	i := 0
	for i < len(lines) && isBlank(lines[i]) {
		d.leading++
		i++
	}
	for i < len(lines) {
		start := i
		if fence := fenceMarker(lines[i]); fence != "" {
			i++
			for i < len(lines) && !closesFence(lines[i], fence) {
				i++
			}
			if i < len(lines) {
				i++ // include the closing fence
			}
		} else {
			i++
			for i < len(lines) && !isBlank(lines[i]) && fenceMarker(lines[i]) == "" {
				i++
			}
		}
		b := block{src: strings.Join(lines[start:i], "\n")}
		for i < len(lines) && isBlank(lines[i]) {
			b.blank++
			i++
		}
		d.blocks = append(d.blocks, b)
	}
	if len(d.blocks) == 0 {
		d.blocks = []block{{}}
	}
	return d
}

// String reassembles the markdown source. Empty blocks contribute nothing.
func (d document) String() string {
	last := -1
	for i, b := range d.blocks {
		if b.src != "" {
			last = i
		}
	}
	var sb strings.Builder
	sb.WriteString(strings.Repeat("\n", d.leading))
	for i, b := range d.blocks {
		if b.src == "" {
			continue
		}
		sb.WriteString(b.src)
		if i != last || d.finalNL {
			sb.WriteByte('\n')
		}
		sb.WriteString(strings.Repeat("\n", b.blank))
	}
	return sb.String()
}

func (d document) clone() document {
	c := d
	c.blocks = append([]block(nil), d.blocks...)
	return c
}

// replaceBlock swaps block i for the blocks parsed from text (which may be
// zero, one or several), keeping the surrounding blank lines sensible.
func (d *document) replaceBlock(i int, text string) {
	old := d.blocks[i]
	var repl []block
	for _, b := range parseDocument(text).blocks {
		if b.src != "" {
			repl = append(repl, b)
		}
	}
	if len(repl) > 0 {
		repl[len(repl)-1].blank = old.blank
		if i > 0 && d.blocks[i-1].blank == 0 && !isFenced(repl[0].src) {
			d.blocks[i-1].blank = 1
		}
		if i+1 < len(d.blocks) && old.blank == 0 && !isFenced(repl[len(repl)-1].src) {
			repl[len(repl)-1].blank = 1
		}
	}
	d.splice(i, 1, repl...)
}

// insertAfter adds an empty block after block i and returns its index.
func (d *document) insertAfter(i int) int {
	nb := block{blank: d.blocks[i].blank}
	d.blocks[i].blank = max(d.blocks[i].blank, 1)
	d.splice(i+1, 0, nb)
	return i + 1
}

// insertBefore adds an empty block before block i and returns its index.
func (d *document) insertBefore(i int) int {
	if i > 0 {
		d.blocks[i-1].blank = max(d.blocks[i-1].blank, 1)
	}
	d.splice(i, 0, block{blank: 1})
	return i
}

func (d *document) deleteBlock(i int) {
	gone := d.blocks[i]
	if i > 0 {
		prev := &d.blocks[i-1]
		if i == len(d.blocks)-1 {
			prev.blank = gone.blank // prev becomes the last block
		} else {
			prev.blank = max(prev.blank, gone.blank, 1)
		}
	}
	d.splice(i, 1)
}

// splice replaces n blocks starting at i with repl, always leaving at least
// one block in the document.
func (d *document) splice(i, n int, repl ...block) {
	out := make([]block, 0, len(d.blocks)-n+len(repl))
	out = append(out, d.blocks[:i]...)
	out = append(out, repl...)
	out = append(out, d.blocks[i+n:]...)
	if len(out) == 0 {
		out = []block{{}}
	}
	d.blocks = out
}

func isBlank(line string) bool { return strings.TrimSpace(line) == "" }

func isFenced(src string) bool {
	first, _, _ := strings.Cut(src, "\n")
	return fenceMarker(first) != ""
}

// fenceMarker returns the fence run (e.g. "```" or "~~~~") if line opens a
// fenced code block, else "".
func fenceMarker(line string) string {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 || s == "" || (s[0] != '`' && s[0] != '~') {
		return ""
	}
	c := s[0]
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	if n < 3 {
		return ""
	}
	if c == '`' && strings.Contains(s[n:], "`") {
		return "" // backtick fences can't have backticks in the info string
	}
	return s[:n]
}

// closesFence reports whether line closes a block opened with fence.
func closesFence(line, fence string) bool {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 {
		return false
	}
	c := fence[0]
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	return n >= len(fence) && strings.TrimSpace(s[n:]) == ""
}
