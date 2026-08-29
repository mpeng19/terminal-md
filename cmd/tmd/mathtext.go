package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// convertMath rewrites LaTeX math in a markdown document into plain
// Unicode text so terminals can display it. Math is recognised as:
//
//	inline:  $...$  (not $$), \(...\)
//	display: $$...$$ (may span lines), \[...\]
//
// Math inside fenced code blocks (``` or ~~~), indented code blocks, and
// inline code spans (`...`) is left untouched. Single dollars follow the
// Pandoc rule: the opening $ must be followed by a non-space character, the
// closing $ must be preceded by a non-space character and must not be
// followed by a digit, so "$5 and $10" is not math. Escaped \$ is a literal
// dollar. No math delimiter may span a blank line.
//
// Inline math becomes plain text (the converted formula). Display math
// becomes its own paragraph: a fenced code block with language "math"
// containing the converted formula, with one blank line before and after,
// so alignment and spacing are preserved by the renderer.
func convertMath(md string) string {
	var out []byte
	for _, seg := range splitCodeSegments(md) {
		if seg.code {
			out = append(out, seg.text...)
			continue
		}
		out = appendMathText(out, seg.text)
	}
	res := string(out)
	// A display block at the very end of the document leaves a trailing
	// blank line; keep the document's original ending instead.
	if strings.HasSuffix(res, "\n\n") && !strings.HasSuffix(md, "\n\n") {
		res = strings.TrimRight(res, "\n") + "\n"
	}
	return res
}

// latexToUnicode converts a single LaTeX math expression (no delimiters)
// into readable Unicode text.
func latexToUnicode(tex string) string {
	s := renderTex(parseTex(tex))
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

// ---------------------------------------------------------------------
// Markdown scanning
// ---------------------------------------------------------------------

// mdSegment is a run of consecutive lines that are either all code (left
// untouched) or all prose (scanned for math).
type mdSegment struct {
	text string
	code bool
}

// splitCodeSegments walks the document line by line and separates fenced
// and indented code blocks from everything else. It tracks list-item
// content indentation so that paragraphs inside list items are not
// mistaken for indented code.
func splitCodeSegments(md string) []mdSegment {
	var segs []mdSegment
	add := func(line string, code bool) {
		if n := len(segs); n > 0 && segs[n-1].code == code {
			segs[n-1].text += line
			return
		}
		segs = append(segs, mdSegment{text: line, code: code})
	}

	var (
		inFence   bool
		fenceChar byte
		fenceLen  int
		inPara    bool  // inside a paragraph: indented lines are lazy continuations
		stack     []int // content indent of each open list item
	)
	top := func() int {
		if len(stack) == 0 {
			return 0
		}
		return stack[len(stack)-1]
	}

	for _, line := range strings.SplitAfter(md, "\n") {
		if line == "" {
			continue
		}
		ind, rest := leadingIndent(line)
		blank := strings.TrimSpace(line) == ""

		if inFence {
			add(line, true)
			ch, n, after, ok := mathFenceMarker(rest)
			if ok && ch == fenceChar && n >= fenceLen && ind <= top()+3 && strings.TrimSpace(after) == "" {
				inFence = false
			}
			continue
		}
		if blank {
			inPara = false
			add(line, false)
			continue
		}

		if item, off := listItemOffset(rest); item && ind <= top()+3 {
			for len(stack) > 0 && ind < stack[len(stack)-1] {
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, ind+off)
			inPara = strings.TrimSpace(rest[minInt(off, len(rest)):]) != ""
			add(line, false)
			continue
		}
		if !inPara {
			for len(stack) > 0 && ind < stack[len(stack)-1] {
				stack = stack[:len(stack)-1]
			}
		}
		content := top()

		if ch, n, _, ok := mathFenceMarker(rest); ok && ind <= content+3 {
			inFence, fenceChar, fenceLen = true, ch, n
			inPara = false
			add(line, true)
			continue
		}
		if !inPara && ind >= content+4 {
			add(line, true) // indented code block
			continue
		}
		if !isATXHeading(rest) {
			inPara = true
		}
		add(line, false)
	}
	return segs
}

// leadingIndent returns the indentation of line in columns (tabs advance to
// the next multiple of four) and the line after the indentation.
func leadingIndent(line string) (int, string) {
	cols := 0
	i := 0
	for ; i < len(line); i++ {
		switch line[i] {
		case ' ':
			cols++
		case '\t':
			cols += 4 - cols%4
		default:
			return cols, line[i:]
		}
	}
	return cols, ""
}

// fenceMarker reports whether rest (a line with indentation stripped)
// starts with a code fence, returning the fence character, its length and
// the info string that follows it.
func mathFenceMarker(rest string) (ch byte, n int, after string, ok bool) {
	if rest == "" || (rest[0] != '`' && rest[0] != '~') {
		return 0, 0, "", false
	}
	ch = rest[0]
	for n < len(rest) && rest[n] == ch {
		n++
	}
	after = rest[n:]
	if n < 3 || (ch == '`' && strings.Contains(after, "`")) {
		return 0, 0, "", false
	}
	return ch, n, after, true
}

// listItemOffset reports whether rest starts with a list marker and, if so,
// the column offset (relative to the marker) at which the item's content
// starts.
func listItemOffset(rest string) (bool, int) {
	i := 0
	if rest != "" && (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') {
		i = 1
	} else {
		for i < len(rest) && i < 9 && isDigitByte(rest[i]) {
			i++
		}
		if i == 0 || i >= len(rest) || (rest[i] != '.' && rest[i] != ')') {
			return false, 0
		}
		i++
	}
	if i < len(rest) && !isSpaceByte(rest[i]) {
		return false, 0
	}
	spaces := 0
	for i+spaces < len(rest) && rest[i+spaces] == ' ' {
		spaces++
	}
	if spaces == 0 || spaces > 4 {
		spaces = 1 // empty item, or content that is itself an indented code block
	}
	return true, i + spaces
}

func isATXHeading(rest string) bool {
	n := 0
	for n < len(rest) && rest[n] == '#' {
		n++
	}
	return n >= 1 && n <= 6 && (n == len(rest) || isSpaceByte(rest[n]))
}

// appendMathText scans prose for math delimiters, skipping inline code
// spans and backslash escapes, and appends the converted text to out.
func appendMathText(out []byte, s string) []byte {
	segStart := len(out)
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			switch s[i+1] {
			case '(':
				if j := findMathClose(s, i+2, `\)`); j >= 0 {
					out = append(out, latexToUnicode(s[i+2:j])...)
					i = j + 2
					continue
				}
			case '[':
				if j := findMathClose(s, i+2, `\]`); j >= 0 {
					out, i = appendDisplayMath(out, segStart, s, i, s[i+2:j], j+2)
					continue
				}
			}
			out = append(out, c, s[i+1]) // escaped character, e.g. \$
			i += 2
		case c == '`':
			n := 0
			for i+n < len(s) && s[i+n] == '`' {
				n++
			}
			if end := findCodeSpanEnd(s, i+n, n); end >= 0 {
				out = append(out, s[i:end+n]...)
				i = end + n
			} else {
				out = append(out, s[i:i+n]...)
				i += n
			}
		case c == '$' && i+1 < len(s) && s[i+1] == '$':
			if j := findMathClose(s, i+2, "$$"); j >= 0 {
				out, i = appendDisplayMath(out, segStart, s, i, s[i+2:j], j+2)
				continue
			}
			out = append(out, "$$"...)
			i += 2
		case c == '$':
			if i+1 < len(s) && !isSpaceByte(s[i+1]) {
				if j := findMathClose(s, i+1, "$"); j >= 0 {
					out = append(out, latexToUnicode(s[i+1:j])...)
					i = j + 1
					continue
				}
			}
			out = append(out, c)
			i++
		default:
			out = append(out, c)
			i++
		}
	}
	return out
}

// findCodeSpanEnd returns the index of a run of exactly n backticks at or
// after start, or -1 when the span is not closed.
func findCodeSpanEnd(s string, start, n int) int {
	for j := start; j < len(s); {
		if s[j] != '`' {
			j++
			continue
		}
		k := j
		for k < len(s) && s[k] == '`' {
			k++
		}
		if k-j == n {
			return j
		}
		j = k
	}
	return -1
}

// findMathClose returns the index of the closing delimiter at or after
// start, or -1. Backslash escapes are skipped and a blank line aborts the
// search. For a single "$" the Pandoc closing rule applies: it must be
// preceded by a non-space character and not followed by a digit; a bare
// "$" that fails the rule ends the search, so "$5 and $10" stays literal.
func findMathClose(s string, start int, close string) int {
	for j := start; j < len(s); j++ {
		if strings.HasPrefix(s[j:], close) {
			if close != "$" || (!isSpaceByte(s[j-1]) && (j+1 >= len(s) || !isDigitByte(s[j+1]))) {
				return j
			}
			return -1
		}
		switch s[j] {
		case '\\':
			j++
		case '`':
			return -1 // a math span never contains a code span boundary
		case '\n':
			if blankLineAt(s, j+1) {
				return -1
			}
		}
	}
	return -1
}

// blankLineAt reports whether the line starting at k is blank (or the text
// ends there).
func blankLineAt(s string, k int) bool {
	for k < len(s) && (s[k] == ' ' || s[k] == '\t' || s[k] == '\r') {
		k++
	}
	return k >= len(s) || s[k] == '\n'
}

// appendDisplayMath emits a ```math fenced block for the formula tex, which
// started at s[start] and whose closing delimiter ends at s[end]. It returns
// the new output and the position in s from which scanning resumes. The
// block is separated from surrounding text by blank lines and keeps the
// indentation of the line the math started on.
func appendDisplayMath(out []byte, segStart int, s string, start int, tex string, end int) ([]byte, int) {
	lineStart := strings.LastIndexByte(s[:start], '\n') + 1
	k := lineStart
	for k < start && (s[k] == ' ' || s[k] == '\t') {
		k++
	}
	indent := s[lineStart:k]

	for len(out) > segStart && isSpaceByte(out[len(out)-1]) {
		out = out[:len(out)-1]
	}
	if len(out) > 0 {
		if out[len(out)-1] != '\n' {
			out = append(out, '\n')
		}
		out = append(out, '\n')
	}
	out = append(out, indent+"```math\n"...)
	for _, line := range strings.Split(latexToUnicode(tex), "\n") {
		out = append(out, indent+line+"\n"...)
	}
	out = append(out, indent+"```\n\n"...)

	// Skip whitespace after the closing delimiter; when it contained a line
	// break, resume at the start of the next line so its indentation stays.
	j, resume := end, -1
	for j < len(s) && isSpaceByte(s[j]) {
		if s[j] == '\n' {
			resume = j + 1
		}
		j++
	}
	if resume < 0 || j == len(s) {
		resume = j
	}
	return out, resume
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
func isDigitByte(c byte) bool { return '0' <= c && c <= '9' }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------
// LaTeX tokenizer
// ---------------------------------------------------------------------

type texKind int

const (
	texChar  texKind = iota // a single character; " " stands for any run of whitespace
	texCmd                  // a control sequence; text is the name without the backslash
	texGroup                // a {...} group
)

type texNode struct {
	kind texKind
	text string
	kids []texNode
}

type texParser struct {
	src []rune
	pos int
}

// parseTex tokenizes a LaTeX expression into a tree of nodes.
func parseTex(s string) []texNode {
	p := &texParser{src: []rune(s)}
	return p.parse(false)
}

// parse reads nodes until the end of input or, when inGroup is set, the
// closing brace of the current group.
func (p *texParser) parse(inGroup bool) []texNode {
	var nodes []texNode
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		p.pos++
		switch {
		case r == '}':
			if inGroup {
				return nodes
			}
			// A stray closing brace is dropped.
		case r == '{':
			nodes = append(nodes, texNode{kind: texGroup, kids: p.parse(true)})
		case unicode.IsSpace(r):
			for p.pos < len(p.src) && unicode.IsSpace(p.src[p.pos]) {
				p.pos++
			}
			nodes = append(nodes, texNode{kind: texChar, text: " "})
		case r == '\\':
			if p.pos >= len(p.src) {
				nodes = append(nodes, texNode{kind: texChar, text: `\`})
				break
			}
			start := p.pos
			for p.pos < len(p.src) && isASCIILetter(p.src[p.pos]) {
				p.pos++
			}
			if p.pos == start {
				p.pos++ // control symbol such as \, or \\
			}
			nodes = append(nodes, texNode{kind: texCmd, text: string(p.src[start:p.pos])})
		default:
			nodes = append(nodes, texNode{kind: texChar, text: string(r)})
		}
	}
	return nodes
}

func isASCIILetter(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

// ---------------------------------------------------------------------
// LaTeX renderer
// ---------------------------------------------------------------------

// renderTex converts a node list into text.
func renderTex(nodes []texNode) string {
	var b strings.Builder
	for i := 0; i < len(nodes); {
		i = renderTexNode(&b, nodes, i)
	}
	return b.String()
}

// renderTexNode renders nodes[i] (and any arguments it consumes) and
// returns the index of the next node to render.
func renderTexNode(b *strings.Builder, nodes []texNode, i int) int {
	n := nodes[i]
	switch n.kind {
	case texGroup:
		b.WriteString(renderTex(n.kids))
	case texCmd:
		return renderTexCmd(b, nodes, i)
	default:
		switch n.text {
		case " ", "&", "~":
			writeSpace(b)
		case "^", "_":
			arg, next := texArg(nodes, i+1)
			writeScript(b, n.text, renderTex(arg))
			return next
		default:
			b.WriteString(n.text)
		}
	}
	return i + 1
}

func renderTexCmd(b *strings.Builder, nodes []texNode, i int) int {
	name := nodes[i].text
	i++
	if sym, ok := texSymbols[name]; ok {
		b.WriteString(sym)
		return i
	}
	if texDropped[name] {
		if name == "left" || name == "right" || name == "middle" {
			// \left. and \right. use an invisible delimiter: drop the dot too.
			if arg, next := texArg(nodes, i); len(arg) == 1 && arg[0].kind == texChar && arg[0].text == "." {
				return next
			}
		}
		return i
	}
	if mark, ok := texAccents[name]; ok {
		arg, next := texArg(nodes, i)
		b.WriteString(applyAccent(renderTex(arg), mark, name == "overline" || name == "underline"))
		return next
	}
	if f, ok := texAlphabets[name]; ok {
		arg, next := texArg(nodes, i)
		b.WriteString(strings.Map(f, renderTex(arg)))
		return next
	}
	if texPlain[name] {
		if j := skipTexSpaces(nodes, i); j < len(nodes) && nodes[j].kind == texChar && nodes[j].text == "*" {
			i = j + 1 // \operatorname*
		}
		arg, next := texArg(nodes, i)
		b.WriteString(renderTex(arg))
		return next
	}
	if texDroppedWithArg[name] {
		_, next := texArg(nodes, i)
		return next
	}

	switch name {
	case `\`, "newline", "cr":
		b.WriteString("\n")
	case ",", ";", ":", "!", " ", "thinspace", "medspace", "thickspace", "enspace", "enskip":
		writeSpace(b)
	case "quad":
		b.WriteString("  ")
	case "qquad":
		b.WriteString("    ")
	case "frac", "dfrac", "tfrac", "cfrac":
		num, next := texArg(nodes, i)
		den, next := texArg(nodes, next)
		writeFraction(b, renderTex(num), renderTex(den))
		return next
	case "sqrt":
		return renderTexSqrt(b, nodes, i)
	case "binom", "dbinom", "tbinom":
		top, next := texArg(nodes, i)
		bottom, next := texArg(nodes, next)
		b.WriteString("C(" + renderTex(top) + "," + renderTex(bottom) + ")")
		return next
	case "pmod":
		arg, next := texArg(nodes, i)
		b.WriteString("(mod " + renderTex(arg) + ")")
		return next
	case "not":
		arg, next := texArg(nodes, i)
		s := renderTex(arg)
		if neg, ok := texNegations[s]; ok {
			b.WriteString(neg)
		} else {
			b.WriteString(s + "̸")
		}
		return next
	case "begin":
		return renderTexEnv(b, nodes, i)
	case "end": // stray \end without a matching \begin
		_, next := texArg(nodes, i)
		return next
	default:
		b.WriteString(name) // unknown command: keep the name without its backslash
	}
	return i
}

// texArg returns the next argument after index i: the contents of a group,
// or the single node found there. Whitespace before the argument is skipped.
func texArg(nodes []texNode, i int) ([]texNode, int) {
	i = skipTexSpaces(nodes, i)
	if i >= len(nodes) {
		return nil, i
	}
	if nodes[i].kind == texGroup {
		return nodes[i].kids, i + 1
	}
	return nodes[i : i+1], i + 1
}

func skipTexSpaces(nodes []texNode, i int) int {
	for i < len(nodes) && nodes[i].kind == texChar && nodes[i].text == " " {
		i++
	}
	return i
}

// writeSpace appends a single space unless the output is empty or already
// ends with whitespace, collapsing runs of spacing.
func writeSpace(b *strings.Builder) {
	if s := b.String(); s != "" && !strings.HasSuffix(s, " ") && !strings.HasSuffix(s, "\n") {
		b.WriteByte(' ')
	}
}

// writeScript renders a superscript ("^") or subscript ("_") whose rendered
// argument is s, using Unicode super/subscript characters when every
// character has one and falling back to ^(...) / _(...) otherwise.
func writeScript(b *strings.Builder, op, s string) {
	if s == "" {
		b.WriteString(op)
		return
	}
	table := texSubscripts
	if op == "^" {
		table = texSuperscripts
		switch s {
		case "∘": // ^\circ, degrees
			b.WriteString("°")
			return
		case "′", "*", "'":
			b.WriteString(s)
			return
		}
	}
	if t, ok := toScript(s, table); ok {
		b.WriteString(t)
		return
	}
	b.WriteString(op)
	if utf8.RuneCountInString(s) == 1 {
		b.WriteString(s)
	} else {
		b.WriteString("(" + s + ")")
	}
}

// toScript maps every rune of s through table, reporting false if any rune
// has no mapping.
func toScript(s string, table map[rune]rune) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		m, ok := table[r]
		if !ok {
			return "", false
		}
		b.WriteRune(m)
	}
	return b.String(), true
}

func writeFraction(b *strings.Builder, num, den string) {
	if v, ok := texVulgarFractions[num+"/"+den]; ok {
		b.WriteString(v)
		return
	}
	b.WriteString(fractionOperand(num) + "/" + fractionOperand(den))
}

func fractionOperand(s string) string {
	if isSimpleTexToken(s) {
		return s
	}
	return "(" + s + ")"
}

// isSimpleTexToken reports whether s reads as one token (letters, digits,
// combining marks, super/subscripts) rather than an expression.
func isSimpleTexToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || unicode.Is(unicode.No, r) {
			continue
		}
		if !strings.ContainsRune("'′.∞", r) {
			return false
		}
	}
	return true
}

func renderTexSqrt(b *strings.Builder, nodes []texNode, i int) int {
	index := ""
	if j := skipTexSpaces(nodes, i); j < len(nodes) && nodes[j].kind == texChar && nodes[j].text == "[" {
		k := j + 1
		for k < len(nodes) && !(nodes[k].kind == texChar && nodes[k].text == "]") {
			k++
		}
		index = renderTex(nodes[j+1 : k])
		i = minInt(k+1, len(nodes))
	}
	arg, next := texArg(nodes, i)
	s := renderTex(arg)
	if index != "" {
		if t, ok := toScript(index, texSuperscripts); ok {
			b.WriteString(t)
		} else {
			b.WriteString("(" + index + ")")
		}
	}
	b.WriteString("√")
	if isSimpleTexToken(s) {
		b.WriteString(s)
	} else {
		b.WriteString("(" + s + ")")
	}
	return next
}

// applyAccent attaches a combining mark to s: to every character when each
// is set (overline/underline), otherwise once at the end.
func applyAccent(s, mark string, each bool) string {
	if s == "" {
		return ""
	}
	if !each {
		return s + mark
	}
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(r)
		if !unicode.IsSpace(r) {
			b.WriteString(mark)
		}
	}
	return b.String()
}

// renderTexEnv renders \begin{name} ... \end{name}. Rows separated by \\
// are placed on separate lines; alignment tabs become spaces.
func renderTexEnv(b *strings.Builder, nodes []texNode, i int) int {
	nameArg, i := texArg(nodes, i)
	name := strings.TrimSpace(renderTex(nameArg))

	end, after, depth := len(nodes), len(nodes), 0
	for j := i; j < len(nodes); j++ {
		if nodes[j].kind != texCmd || (nodes[j].text != "begin" && nodes[j].text != "end") {
			continue
		}
		arg, next := texArg(nodes, j+1)
		if strings.TrimSpace(renderTex(arg)) != name {
			continue
		}
		if nodes[j].text == "begin" {
			depth++
			continue
		}
		if depth == 0 {
			end, after = j, next
			break
		}
		depth--
	}

	rows := texRows(nodes[i:end])
	text := formatTexRows(name, rows)
	if len(rows) > 1 {
		if cur := b.String(); strings.TrimSpace(cur[strings.LastIndex(cur, "\n")+1:]) != "" {
			b.WriteString("\n")
		}
		b.WriteString(text)
		b.WriteString("\n")
	} else {
		b.WriteString(text)
	}
	return after
}

// texRows splits an environment body on \\ and renders each row.
func texRows(body []texNode) []string {
	var rows []string
	start := 0
	for j := 0; j <= len(body); j++ {
		if j < len(body) && !(body[j].kind == texCmd && (body[j].text == `\` || body[j].text == "cr")) {
			continue
		}
		rows = append(rows, strings.TrimSpace(renderTex(body[start:j])))
		start = j + 1
	}
	for len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	return rows
}

func formatTexRows(name string, rows []string) string {
	open, close := "", ""
	switch strings.TrimSuffix(name, "*") {
	case "cases", "dcases", "rcases":
		for k := range rows {
			rows[k] = "  " + rows[k]
		}
	case "pmatrix", "psmallmatrix":
		open, close = "(", ")"
	case "bmatrix", "bsmallmatrix":
		open, close = "[", "]"
	case "Bmatrix":
		open, close = "{", "}"
	case "vmatrix":
		open, close = "|", "|"
	case "Vmatrix":
		open, close = "‖", "‖"
	}
	if open != "" && len(rows) > 0 {
		for k := range rows {
			if k == 0 {
				rows[k] = open + " " + rows[k]
			} else {
				rows[k] = "  " + rows[k]
			}
		}
		rows[len(rows)-1] += " " + close
	}
	return strings.Join(rows, "\n")
}

// ---------------------------------------------------------------------
// Tables
// ---------------------------------------------------------------------

// texSymbols maps command names to their Unicode text.
var texSymbols = map[string]string{
	// Greek
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε", "varepsilon": "ε",
	"zeta": "ζ", "eta": "η", "theta": "θ", "vartheta": "ϑ", "iota": "ι", "kappa": "κ", "varkappa": "ϰ",
	"lambda": "λ", "mu": "μ", "nu": "ν", "xi": "ξ", "omicron": "ο", "pi": "π", "varpi": "ϖ",
	"rho": "ρ", "varrho": "ϱ", "sigma": "σ", "varsigma": "ς", "tau": "τ", "upsilon": "υ",
	"phi": "φ", "varphi": "φ", "chi": "χ", "psi": "ψ", "omega": "ω",
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ", "Pi": "Π",
	"Sigma": "Σ", "Upsilon": "Υ", "Phi": "Φ", "Psi": "Ψ", "Omega": "Ω",

	// Binary operators
	"times": "×", "cdot": "·", "div": "÷", "pm": "±", "mp": "∓", "ast": "∗", "star": "⋆",
	"circ": "∘", "bullet": "•", "oplus": "⊕", "ominus": "⊖", "otimes": "⊗", "odot": "⊙",
	"setminus": "∖", "wedge": "∧", "land": "∧", "vee": "∨", "lor": "∨", "cup": "∪", "cap": "∩",
	"sqcup": "⊔", "sqcap": "⊓", "amalg": "⨿", "diamond": "⋄",

	// Relations
	"leq": "≤", "le": "≤", "leqslant": "≤", "geq": "≥", "ge": "≥", "geqslant": "≥",
	"neq": "≠", "ne": "≠", "approx": "≈", "equiv": "≡", "sim": "∼", "simeq": "≃", "cong": "≅",
	"propto": "∝", "ll": "≪", "gg": "≫", "prec": "≺", "succ": "≻", "preceq": "≼", "succeq": "≽",
	"in": "∈", "notin": "∉", "ni": "∋", "subset": "⊂", "subseteq": "⊆", "subsetneq": "⊊",
	"supset": "⊃", "supseteq": "⊇", "supsetneq": "⊋", "sqsubseteq": "⊑", "sqsupseteq": "⊒",
	"perp": "⊥", "parallel": "∥", "nparallel": "∦", "mid": "|", "nmid": "∤",
	"vdash": "⊢", "dashv": "⊣", "models": "⊨", "top": "⊤", "bot": "⊥", "asymp": "≍",
	"doteq": "≐", "triangleq": "≜", "coloneqq": "≔", "colon": ":",

	// Arrows
	"to": "→", "rightarrow": "→", "leftarrow": "←", "gets": "←", "leftrightarrow": "↔",
	"Rightarrow": "⇒", "implies": "⇒", "Leftarrow": "⇐", "impliedby": "⇐",
	"Leftrightarrow": "⇔", "iff": "⇔", "mapsto": "↦", "longmapsto": "⟼",
	"longrightarrow": "⟶", "longleftarrow": "⟵", "Longrightarrow": "⟹", "Longleftarrow": "⟸",
	"longleftrightarrow": "⟷", "Longleftrightarrow": "⟺", "hookrightarrow": "↪", "hookleftarrow": "↩",
	"uparrow": "↑", "downarrow": "↓", "Uparrow": "⇑", "Downarrow": "⇓", "updownarrow": "↕",
	"nearrow": "↗", "searrow": "↘", "swarrow": "↙", "nwarrow": "↖",
	"rightharpoonup": "⇀", "leftharpoonup": "↼", "rightleftharpoons": "⇌",

	// Big operators
	"sum": "∑", "prod": "∏", "coprod": "∐", "int": "∫", "iint": "∬", "iiint": "∭", "oint": "∮",
	"oiint": "∯", "bigcup": "⋃", "bigcap": "⋂", "bigoplus": "⨁", "bigotimes": "⨂", "bigvee": "⋁",
	"bigwedge": "⋀", "bigsqcup": "⨆",

	// Miscellaneous symbols
	"infty": "∞", "partial": "∂", "nabla": "∇", "emptyset": "∅", "varnothing": "∅",
	"forall": "∀", "exists": "∃", "nexists": "∄", "neg": "¬", "lnot": "¬",
	"ldots": "…", "dots": "…", "dotsc": "…", "dotso": "…", "cdots": "⋯", "dotsb": "⋯", "dotsi": "⋯",
	"vdots": "⋮", "ddots": "⋱", "angle": "∠", "measuredangle": "∡", "triangle": "△",
	"degree": "°", "prime": "′", "dprime": "″", "hbar": "ℏ", "ell": "ℓ", "Re": "ℜ", "Im": "ℑ",
	"aleph": "ℵ", "beth": "ℶ", "wp": "℘", "imath": "ı", "jmath": "ȷ", "therefore": "∴",
	"because": "∵", "complement": "∁", "surd": "√", "checkmark": "✓", "dagger": "†",
	"ddagger": "‡", "S": "§", "P": "¶", "pounds": "£", "copyright": "©", "square": "□",
	"Box": "□", "blacksquare": "■", "Diamond": "◇", "clubsuit": "♣", "diamondsuit": "♢",
	"heartsuit": "♡", "spadesuit": "♠", "flat": "♭", "natural": "♮", "sharp": "♯",
	"infinity": "∞", "eth": "ð",

	// Delimiters
	"langle": "⟨", "rangle": "⟩", "lfloor": "⌊", "rfloor": "⌋", "lceil": "⌈", "rceil": "⌉",
	"|": "‖", "Vert": "‖", "lVert": "‖", "rVert": "‖", "vert": "|", "lvert": "|", "rvert": "|",
	"lbrace": "{", "rbrace": "}", "lbrack": "[", "rbrack": "]", "backslash": `\`,
	"{": "{", "}": "}", "%": "%", "&": "&", "#": "#", "_": "_", "$": "$",

	// Function names
	"arccos": "arccos", "arcsin": "arcsin", "arctan": "arctan", "arg": "arg", "cos": "cos",
	"cosh": "cosh", "cot": "cot", "coth": "coth", "csc": "csc", "deg": "deg", "det": "det",
	"dim": "dim", "exp": "exp", "gcd": "gcd", "hom": "hom", "inf": "inf", "ker": "ker",
	"lg": "lg", "lim": "lim", "liminf": "liminf", "limsup": "limsup", "ln": "ln", "log": "log",
	"max": "max", "min": "min", "Pr": "Pr", "sec": "sec", "sin": "sin", "sinh": "sinh",
	"sup": "sup", "tan": "tan", "tanh": "tanh", "mod": "mod", "bmod": "mod",
}

// texDropped lists commands that produce no output on their own.
var texDropped = map[string]bool{
	"left": true, "right": true, "middle": true,
	"big": true, "Big": true, "bigg": true, "Bigg": true,
	"bigl": true, "bigr": true, "bigm": true, "Bigl": true, "Bigr": true, "Bigm": true,
	"biggl": true, "biggr": true, "biggm": true, "Biggl": true, "Biggr": true, "Biggm": true,
	"displaystyle": true, "textstyle": true, "scriptstyle": true, "scriptscriptstyle": true,
	"limits": true, "nolimits": true, "nonumber": true, "notag": true, "hline": true,
	"mathstrut": true, "allowbreak": true, "negthinspace": true, "negmedspace": true,
	"negthickspace": true, "strut": true,
}

// texDroppedWithArg lists commands whose single argument is discarded too.
var texDroppedWithArg = map[string]bool{
	"phantom": true, "hphantom": true, "vphantom": true, "label": true, "tag": true,
	"hspace": true, "vspace": true, "smash": true,
}

// texPlain lists commands whose argument is rendered as plain text.
var texPlain = map[string]bool{
	"text": true, "textrm": true, "textit": true, "textbf": true, "textsf": true, "texttt": true,
	"textnormal": true, "mathrm": true, "mathit": true, "mathsf": true, "mathtt": true,
	"mathnormal": true, "mathfrak": true, "operatorname": true, "mbox": true, "hbox": true,
}

// texAccents maps accent commands to their combining character.
var texAccents = map[string]string{
	"hat": "̂", "widehat": "̂", "bar": "̄", "vec": "⃗", "overrightarrow": "⃗",
	"tilde": "̃", "widetilde": "̃", "dot": "̇", "ddot": "̈", "dddot": "⃛",
	"overline": "̅", "underline": "̲", "check": "̌", "breve": "̆",
	"acute": "́", "grave": "̀", "mathring": "̊",
}

// texAlphabets maps font commands to the rune mapping they apply.
var texAlphabets = map[string]func(rune) rune{
	"mathbb": doubleStruckRune, "mathcal": scriptRune, "mathscr": scriptRune,
	"mathbf": boldRune, "boldsymbol": boldRune, "bm": boldRune, "pmb": boldRune,
}

func doubleStruckRune(r rune) rune {
	switch r {
	case 'N':
		return 'ℕ'
	case 'Z':
		return 'ℤ'
	case 'Q':
		return 'ℚ'
	case 'R':
		return 'ℝ'
	case 'C':
		return 'ℂ'
	case 'P':
		return 'ℙ'
	case 'H':
		return 'ℍ'
	}
	return r
}

// scriptRune maps capitals to Mathematical Bold Script (U+1D4D0...).
func scriptRune(r rune) rune {
	if 'A' <= r && r <= 'Z' {
		return 0x1D4D0 + (r - 'A')
	}
	return r
}

// boldRune maps letters and digits to Mathematical Bold.
func boldRune(r rune) rune {
	switch {
	case 'A' <= r && r <= 'Z':
		return 0x1D400 + (r - 'A')
	case 'a' <= r && r <= 'z':
		return 0x1D41A + (r - 'a')
	case '0' <= r && r <= '9':
		return 0x1D7CE + (r - '0')
	}
	return r
}

// texNegations maps a rendered symbol to its negated form for \not.
var texNegations = map[string]string{
	"=": "≠", "∈": "∉", "<": "≮", ">": "≯", "≡": "≢", "⊂": "⊄", "⊆": "⊈", "⊃": "⊅", "⊇": "⊉",
	"∼": "≁", "≈": "≉", "≤": "≰", "≥": "≱", "∃": "∄", "|": "∤", "∥": "∦", "≅": "≇", "→": "↛",
}

var texVulgarFractions = map[string]string{
	"1/2": "½", "1/3": "⅓", "2/3": "⅔", "1/4": "¼", "3/4": "¾", "1/5": "⅕", "2/5": "⅖",
	"3/5": "⅗", "4/5": "⅘", "1/6": "⅙", "5/6": "⅚", "1/8": "⅛", "3/8": "⅜", "5/8": "⅝", "7/8": "⅞",
}

var texSuperscripts = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴', '5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾', ' ': ' ',
	'a': 'ᵃ', 'b': 'ᵇ', 'c': 'ᶜ', 'd': 'ᵈ', 'e': 'ᵉ', 'f': 'ᶠ', 'g': 'ᵍ', 'h': 'ʰ', 'i': 'ⁱ',
	'j': 'ʲ', 'k': 'ᵏ', 'l': 'ˡ', 'm': 'ᵐ', 'n': 'ⁿ', 'o': 'ᵒ', 'p': 'ᵖ', 'r': 'ʳ', 's': 'ˢ',
	't': 'ᵗ', 'u': 'ᵘ', 'v': 'ᵛ', 'w': 'ʷ', 'x': 'ˣ', 'y': 'ʸ', 'z': 'ᶻ',
	'A': 'ᴬ', 'B': 'ᴮ', 'D': 'ᴰ', 'E': 'ᴱ', 'G': 'ᴳ', 'H': 'ᴴ', 'I': 'ᴵ', 'J': 'ᴶ', 'K': 'ᴷ',
	'L': 'ᴸ', 'M': 'ᴹ', 'N': 'ᴺ', 'O': 'ᴼ', 'P': 'ᴾ', 'R': 'ᴿ', 'T': 'ᵀ', 'U': 'ᵁ', 'V': 'ⱽ', 'W': 'ᵂ',
	'α': 'ᵅ', 'β': 'ᵝ', 'γ': 'ᵞ', 'δ': 'ᵟ', 'ε': 'ᵋ', 'θ': 'ᶿ', 'φ': 'ᵠ', 'χ': 'ᵡ',
}

var texSubscripts = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄', '5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉',
	'+': '₊', '-': '₋', '=': '₌', '(': '₍', ')': '₎', ' ': ' ',
	'a': 'ₐ', 'e': 'ₑ', 'h': 'ₕ', 'i': 'ᵢ', 'j': 'ⱼ', 'k': 'ₖ', 'l': 'ₗ', 'm': 'ₘ', 'n': 'ₙ',
	'o': 'ₒ', 'p': 'ₚ', 'r': 'ᵣ', 's': 'ₛ', 't': 'ₜ', 'u': 'ᵤ', 'v': 'ᵥ', 'x': 'ₓ',
	'β': 'ᵦ', 'γ': 'ᵧ', 'ρ': 'ᵨ', 'φ': 'ᵩ', 'χ': 'ᵪ',
}
