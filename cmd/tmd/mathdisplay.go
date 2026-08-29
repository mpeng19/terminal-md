package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// stackLimits lays out one line of display math so that the limits of big
// operators sit above and below them, the way they do on paper:
//
//	 n
//	 ∑  xᵢ
//	i=1
//
// Limits are written in plain characters, which also sidesteps fonts that
// lack sub/superscript glyphs. Lines without such operators are returned
// unchanged.
func stackLimits(line string) []string {
	type limit struct {
		col          int // column of the operator in the middle line
		upper, lower string
	}
	var (
		mid    strings.Builder
		limits []limit
		col    int
		rs     = []rune(line)
	)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		mid.WriteRune(r)
		w := ansi.StringWidth(string(r))
		if !bigOps[r] {
			col += w
			continue
		}
		var lim limit
		lim.col = col
		col += w
		for {
			text, kind, n := readLimit(rs[i+1:])
			if n == 0 {
				break
			}
			switch kind {
			case '^':
				lim.upper = text
			case '_':
				lim.lower = text
			}
			i += n
		}
		if lim.upper != "" || lim.lower != "" {
			limits = append(limits, lim)
		}
	}
	if len(limits) == 0 {
		return []string{line}
	}

	// Center each limit on its operator; shift everything right if a limit
	// would start before column 0.
	shift := 0
	for _, l := range limits {
		for _, t := range []string{l.upper, l.lower} {
			if start := l.col - (ansi.StringWidth(t)-1)/2; start < -shift {
				shift = -start
			}
		}
	}
	place := func(pick func(limit) string) (string, bool) {
		var b strings.Builder
		any := false
		for _, l := range limits {
			t := pick(l)
			if t == "" {
				continue
			}
			any = true
			start := shift + l.col - (ansi.StringWidth(t)-1)/2
			if cur := ansi.StringWidth(b.String()); start < cur+1 && cur > 0 {
				start = cur + 1
			}
			b.WriteString(strings.Repeat(" ", start-ansi.StringWidth(b.String())))
			b.WriteString(t)
		}
		return b.String(), any
	}
	var out []string
	if top, ok := place(func(l limit) string { return l.upper }); ok {
		out = append(out, top)
	}
	out = append(out, strings.Repeat(" ", shift)+mid.String())
	if bottom, ok := place(func(l limit) string { return l.lower }); ok {
		out = append(out, bottom)
	}
	return out
}

// readLimit reads one limit attached to an operator from the start of rs:
// a run of superscript or subscript characters, or the "^(…)"/"_(…)" and
// "^x"/"_x" fallbacks. It returns the limit in plain characters, which
// script it was, and how many runes were consumed (0 if none).
func readLimit(rs []rune) (text string, kind rune, n int) {
	if len(rs) == 0 {
		return "", 0, 0
	}
	if rs[0] == '^' || rs[0] == '_' {
		kind = rs[0]
		if len(rs) > 1 && rs[1] == '(' {
			depth := 0
			for j := 1; j < len(rs); j++ {
				switch rs[j] {
				case '(':
					depth++
				case ')':
					depth--
					if depth == 0 {
						return string(rs[2:j]), kind, j + 1
					}
				}
			}
			return "", 0, 0
		}
		if len(rs) > 1 && rs[1] != ' ' {
			return string(rs[1]), kind, 2
		}
		return "", 0, 0
	}
	var b strings.Builder
	for _, table := range []struct {
		back map[rune]rune
		kind rune
	}{{superscriptBack, '^'}, {subscriptBack, '_'}} {
		for n < len(rs) {
			plain, ok := table.back[rs[n]]
			if !ok {
				break
			}
			b.WriteRune(plain)
			n++
		}
		if n > 0 {
			return b.String(), table.kind, n
		}
	}
	return "", 0, 0
}

var superscriptBack, subscriptBack = invert(texSuperscripts), invert(texSubscripts)

func invert(m map[rune]rune) map[rune]rune {
	out := make(map[rune]rune, len(m))
	for plain, script := range m {
		if plain == ' ' || script == ' ' {
			continue // spaces end a limit; they are never part of one
		}
		if prev, ok := out[script]; !ok || plain < prev {
			out[script] = plain
		}
	}
	return out
}
