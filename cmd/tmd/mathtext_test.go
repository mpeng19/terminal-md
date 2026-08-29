package main

import (
	"strings"
	"testing"
)

func TestLatexToUnicode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Greek letters
		{"greek lower", `\alpha + \beta`, "α + β"},
		{"greek mixed", `\Omega \omega \epsilon \varepsilon \phi \varphi \Gamma \Delta`, "Ω ω ε ε φ φ Γ Δ"},
		{"greek adjacent", `\alpha\beta`, "αβ"},

		// Superscripts and subscripts
		{"sup digit", `x^2`, "x²"},
		{"sup group", `x^{10}`, "x¹⁰"},
		{"sub letter", `a_i`, "aᵢ"},
		{"sub group", `x_{ij}`, "xᵢⱼ"},
		{"sup negative", `e^{-x}`, "e⁻ˣ"},
		{"sup expression", `x^{y+z}`, "xʸ⁺ᶻ"},
		{"sup fallback multi", `x^{qw}`, "x^(qw)"},
		{"sup fallback single", `x^q`, "x^q"},
		{"sub expression", `x_{n+1}`, "xₙ₊₁"},
		{"sub fallback", `T_{cd}`, "T_(cd)"},
		{"sum limits", `\sum_{i=1}^{n} i`, "∑ⁿᵢ₌₁ i"},
		{"integral limits", `\int_0^1 f(x)\,dx`, "∫₀¹ f(x) dx"},
		{"lim fallback", `\lim_{x \to 0} \sin x`, "lim_(x → 0) sin x"},
		{"degrees", `90^\circ`, "90°"},
		{"log base", `\log_2 n`, "log₂ n"},
		{"sup star", `x^*`, "x*"},
		{"sup infinity", `\sum_{k=0}^{\infty} \frac{x^k}{k!}`, "∑^∞ₖ₌₀ xᵏ/(k!)"},
		{"euler", `e^{i\pi} + 1 = 0`, "e^(iπ) + 1 = 0"},
		{"greek scripts", `\alpha^2 + \beta_1`, "α² + β₁"},
		{"sequence", `x_1, x_2, \ldots, x_n`, "x₁, x₂, …, xₙ"},

		// Fractions
		{"frac half", `\frac{1}{2}`, "½"},
		{"frac three quarters", `\frac{3}{4}`, "¾"},
		{"frac simple", `\frac{a}{b}`, "a/b"},
		{"frac expression", `\frac{x+1}{2}`, "(x+1)/2"},
		{"dfrac", `\dfrac{\pi}{4}`, "π/4"},
		{"tfrac", `\tfrac{1}{3}`, "⅓"},
		{"frac with sup", `\frac{x^2}{2}`, "x²/2"},
		{"frac derivative", `\frac{dy}{dx}`, "dy/dx"},
		{"frac partial", `\frac{\partial f}{\partial x}`, "(∂ f)/(∂ x)"},
		{"frac no braces", `\frac12`, "½"},

		// Roots
		{"sqrt simple", `\sqrt{x}`, "√x"},
		{"sqrt expression", `\sqrt{x+1}`, "√(x+1)"},
		{"sqrt index", `\sqrt[3]{8}`, "³√8"},
		{"sqrt index expression", `\sqrt[n]{x+1}`, "ⁿ√(x+1)"},
		{"sqrt no braces", `\sqrt2`, "√2"},
		{"sqrt of fraction", `\sqrt{\frac{1}{2}}`, "√½"},

		// Operators, relations and symbols
		{"binary ops", `a \times b \cdot c \div d \pm e \mp f`, "a × b · c ÷ d ± e ∓ f"},
		{"relations", `a \leq b \le c \geq d \ge e \neq f \ne g`, "a ≤ b ≤ c ≥ d ≥ e ≠ f ≠ g"},
		{"more relations", `\approx \equiv \sim \simeq \propto`, "≈ ≡ ∼ ≃ ∝"},
		{"calculus", `\infty \partial \nabla`, "∞ ∂ ∇"},
		{"big operators", `\sum \prod \int \iint \oint`, "∑ ∏ ∫ ∬ ∮"},
		{"sets", `x \in A, y \notin B`, "x ∈ A, y ∉ B"},
		{"set ops", `\subset \subseteq \supset \cup \cap \emptyset \varnothing`, "⊂ ⊆ ⊃ ∪ ∩ ∅ ∅"},
		{"quantifiers", `\forall x \exists y`, "∀ x ∃ y"},
		{"logic", `\neg p \lnot q \land r \wedge s \lor t \vee u`, "¬ p ¬ q ∧ r ∧ s ∨ t ∨ u"},
		{"arrows", `A \to B \rightarrow C \leftarrow D`, "A → B → C ← D"},
		{"double arrows", `P \Rightarrow Q \implies R \Leftrightarrow S \iff T \mapsto U`, "P ⇒ Q ⇒ R ⇔ S ⇔ T ↦ U"},
		{"dots", `\ldots \dots \cdots \vdots \ddots`, "… … ⋯ ⋮ ⋱"},
		{"geometry", `\angle \perp \parallel \degree \prime`, "∠ ⊥ ∥ ° ′"},
		{"letterlike", `\hbar \ell \Re \Im \aleph`, "ℏ ℓ ℜ ℑ ℵ"},
		{"brackets", `\langle x \rangle \lfloor y \rfloor \lceil z \rceil`, "⟨ x ⟩ ⌊ y ⌋ ⌈ z ⌉"},
		{"norm", `\|x\|`, "‖x‖"},
		{"functions", `\lim \sin \cos \tan \log \ln \exp \max \min \sup \inf \det \gcd`,
			"lim sin cos tan log ln exp max min sup inf det gcd"},
		{"function call", `\sin(x) + \cos(y)`, "sin(x) + cos(y)"},
		{"negations", `\not= \not\in \not\subset`, "≠ ∉ ⊄"},
		{"binom", `\binom{n}{k}`, "C(n,k)"},
		{"pmod", `a \pmod{n}`, "a (mod n)"},
		{"bmod", `a \bmod n`, "a mod n"},

		// Alphabets and text
		{"mathbb", `\mathbb{N} \mathbb{Z} \mathbb{Q} \mathbb{R} \mathbb{C} \mathbb{X}`, "ℕ ℤ ℚ ℝ ℂ X"},
		{"mathbb with sup", `x \in \mathbb{R}^n`, "x ∈ ℝⁿ"},
		{"mathcal", `\mathcal{L}`, "\U0001D4DB"},
		{"mathcal fallback", `\mathcal{1}`, "1"},
		{"mathbf lower", `\mathbf{x}`, "\U0001D431"},
		{"mathbf upper", `\mathbf{A}`, "\U0001D400"},
		{"boldsymbol", `\boldsymbol{v}`, "\U0001D42F"},
		{"mathrm", `\mathrm{d}x`, "dx"},
		{"text", `\text{if } x > 0`, "if x > 0"},
		{"textrm", `\textrm{for all}`, "for all"},
		{"operatorname", `\operatorname{rank}(A)`, "rank(A)"},
		{"operatorname star", `\operatorname*{argmax}_x`, "argmaxₓ"},

		// Accents
		{"hat", `\hat{x}`, "x\u0302"},
		{"hat no braces", `\hat x`, "x\u0302"},
		{"bar", `\bar{x}`, "x\u0304"},
		{"vec", `\vec{v}`, "v\u20D7"},
		{"tilde", `\tilde{x}`, "x\u0303"},
		{"dot", `\dot{x}`, "x\u0307"},
		{"ddot", `\ddot{x}`, "x\u0308"},
		{"overline", `\overline{x}`, "x\u0305"},
		{"overline multi", `\overline{AB}`, "A\u0305B\u0305"},
		{"dot product", `\vec{v} \cdot \vec{w}`, "v\u20D7 · w\u20D7"},

		// Delimiters, spacing and line breaks
		{"left right", `\left(\frac{a}{b}\right)`, "(a/b)"},
		{"left dot", `\left. \frac{df}{dx} \right|_{x=0}`, "df/dx |ₓ₌₀"},
		{"left right braces", `\left\{ x \right\}`, "{ x }"},
		{"escaped braces", `\{ x \}`, "{ x }"},
		{"big delimiters", `\bigl( x \bigr)`, "( x )"},
		{"thin space", `a\,b`, "a b"},
		{"other thin spaces", `a\;b\:c\!d`, "a b c d"},
		{"quad", `a\quad b`, "a  b"},
		{"qquad", `a\qquad b`, "a    b"},
		{"tie", `a~b`, "a b"},
		{"line break", `a \\ b`, "a\nb"},
		{"alignment tab", `a & b`, "a b"},
		{"displaystyle dropped", `\displaystyle \sum`, "∑"},
		{"escaped specials", `50\% \$5 a\_b`, "50% $5 a_b"},

		// Environments
		{"aligned", `\begin{aligned} a &= b \\ c &= d \end{aligned}`, "a = b\nc = d"},
		{"align star", `\begin{align*} x &= 1 \\ y &= 2 \end{align*}`, "x = 1\ny = 2"},
		{"cases", `f(x) = \begin{cases} x & x \geq 0 \\ -x & \text{otherwise} \end{cases}`,
			"f(x) =\n  x x ≥ 0\n  -x otherwise"},
		{"pmatrix", `\begin{pmatrix} a & b \\ c & d \end{pmatrix}`, "( a b\n  c d )"},
		{"bmatrix", `\begin{bmatrix} 1 & 0 \\ 0 & 1 \end{bmatrix}`, "[ 1 0\n  0 1 ]"},
		{"pmatrix one row", `\begin{pmatrix} 1 \end{pmatrix}`, "( 1 )"},
		{"matrix", `\begin{matrix} a & b \\ c & d \end{matrix}`, "a b\nc d"},
		{"vmatrix", `\begin{vmatrix} a \end{vmatrix}`, "| a |"},
		{"trailing row break", `\begin{aligned} a \\ b \\ \end{aligned}`, "a\nb"},

		// Unknown commands, grouping and whitespace
		{"unknown command", `\foo`, "foo"},
		{"unknown with arg", `\foo{x}`, "foox"},
		{"group removed", `{x}`, "x"},
		{"groups adjacent", `{a}{b}`, "ab"},
		{"whitespace collapsed", `  a   b  `, "a b"},
		{"plain passthrough", `a + b = c, (d) [e] f/g < h > i | j * k .`, "a + b = c, (d) [e] f/g < h > i | j * k ."},
		{"prime", `f'(x)`, "f'(x)"},
		{"physics", `E = mc^2`, "E = mc²"},
		{"area", `\pi r^2`, "π r²"},
		{"empty", ``, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := latexToUnicode(tc.in); got != tc.want {
				t.Errorf("latexToUnicode(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestConvertMath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Inline math
		{"inline dollars", "The area is $\\pi r^2$ here.", "The area is π r² here."},
		{"inline parens", "Use \\(x^2\\) now.", "Use x² now."},
		{"several inline", "$a$ and $b_1$.", "a and b₁."},
		{"inline across newline", "so $a +\nb$ holds", "so a + b holds"},
		{"inline not across blank line", "$a\n\nb$", "$a\n\nb$"},
		{"inline after code span", "`code` then $x$", "`code` then x"},

		// Display math
		{"display single line", "Intro\n$$E = mc^2$$\nOutro\n",
			"Intro\n\n```math\nE = mc²\n```\n\nOutro\n"},
		{"display multi line", "Intro\n$$\n\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}\n$$\nOutro\n",
			"Intro\n\n```math\n n\n ∑ i = (n(n+1))/2\ni=1\n```\n\nOutro\n"},
		{"display brackets", "A\n\\[x^2\\]\nB\n", "A\n\n```math\nx²\n```\n\nB\n"},
		{"display already spaced", "A\n\n$$x$$\n\nB\n", "A\n\n```math\nx\n```\n\nB\n"},
		{"display at end no newline", "A\n$$x$$", "A\n\n```math\nx\n```\n"},
		{"display at end", "A\n\n$$x^2$$\n", "A\n\n```math\nx²\n```\n"},
		{"display at start", "$$x$$\nB\n", "```math\nx\n```\n\nB\n"},
		{"display mid paragraph", "Text $$x$$ more", "Text\n\n```math\nx\n```\n\nmore"},
		{"display in list keeps indent", "- item\n  $$\n  x^2\n  $$\n- next\n",
			"- item\n\n  ```math\n  x²\n  ```\n\n- next\n"},
		{"display aligned", "$$\n\\begin{aligned}\na &= b \\\\\nc &= d\n\\end{aligned}\n$$\n",
			"```math\na = b\nc = d\n```\n"},
		{"display cases", "$$\nf(x) = \\begin{cases} x & \\text{if } x \\geq 0 \\\\ -x & \\text{otherwise} \\end{cases}\n$$\n",
			"```math\nf(x) =\n  x if x ≥ 0\n  -x otherwise\n```\n"},
		{"display not across blank line", "$$a\n\nb$$\n", "$$a\n\nb$$\n"},
		{"unmatched double dollars", "$$ alone", "$$ alone"},

		// Code is left alone
		{"fenced code", "```\n$x^2$\n```\n", "```\n$x^2$\n```\n"},
		{"fenced code with info", "```latex\n$$a$$\n```\nafter $x^2$\n", "```latex\n$$a$$\n```\nafter x²\n"},
		{"tilde fence", "~~~\n$x$\n~~~\n", "~~~\n$x$\n~~~\n"},
		{"unclosed fence", "```\n$x$\n", "```\n$x$\n"},
		{"indented code", "para\n\n    $x^2$\n\nafter $y^2$\n", "para\n\n    $x^2$\n\nafter y²\n"},
		{"indented code after heading", "# Title\n    $x$\n", "# Title\n    $x$\n"},
		{"list paragraph is not code", "1. First\n\n    Text $x^2$ here\n", "1. First\n\n    Text x² here\n"},
		{"code inside list", "- item\n\n        $x^2$\n", "- item\n\n        $x^2$\n"},
		{"inline code", "Use `$x^2$` literally", "Use `$x^2$` literally"},
		{"double backtick code", "``a $x$ b`` and $y$", "``a $x$ b`` and y"},

		// Dollar amounts and escapes
		{"dollar amounts", "costs $5 and $10 today", "costs $5 and $10 today"},
		{"single dollar amount", "It costs $5.", "It costs $5."},
		{"space after opening dollar", "$ x$", "$ x$"},
		{"space before closing dollar", "$x $", "$x $"},
		{"closing dollar before digit", "$a$1 and $b$", "$a$1 and b"},
		{"escaped dollars", "Price: \\$5 and \\$10", "Price: \\$5 and \\$10"},
		{"escaped opening dollar", "\\$x$", "\\$x$"},
		{"escaped dollar inside math", "$a \\$ b$", "a $ b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := convertMath(tc.in); got != tc.want {
				t.Errorf("convertMath(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestConvertMathNoMathUnchanged(t *testing.T) {
	doc := strings.Join([]string{
		"# Title",
		"",
		"Some _emphasis_ and **bold** with a [link](http://example.com/a_b).",
		"Snake_case_words and 2^10 and a\\b backslash.",
		"",
		"- item one",
		"- item two",
		"    continued",
		"",
		"> quote with `code $x$`",
		"",
		"```go",
		"fmt.Println(\"$HOME\")",
		"```",
		"",
		"    indented $code$",
		"",
		"| a | b |",
		"|---|---|",
		"| 1 | 2 |",
		"",
		"The end costs $5 and $10.",
		"",
	}, "\n")
	if got := convertMath(doc); got != doc {
		t.Errorf("document without math was changed\n got: %q\nwant: %q", got, doc)
	}
	if got := convertMath(""); got != "" {
		t.Errorf("empty document became %q", got)
	}
}

func TestConvertMathStopsAtCodeSpans(t *testing.T) {
	in := "Not math: it costs $5 and $10 today, and `$x$` in code stays literal."
	if got := convertMath(in); got != in {
		t.Errorf("got %q, want unchanged", got)
	}
	in = "price $10, then $x$ later, and `a $ b`"
	if got, want := convertMath(in), "price $10, then x later, and `a $ b`"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBigOperatorLimits(t *testing.T) {
	cases := map[string]string{
		`\sum_{i=1}^{n} x_i`:  "∑ⁿᵢ₌₁ xᵢ",
		`\sum^{n}_{i=1} x_i`:  "∑ⁿᵢ₌₁ xᵢ",
		`\int_0^1 f`:          "∫₀¹ f", // short lower limit keeps the usual order
		`x_{ij}^2`:            "xᵢⱼ²",  // not a big operator
		`\lim_{x \to 0} f(x)`: "lim_(x → 0) f(x)",
	}
	for in, want := range cases {
		if got := latexToUnicode(in); got != want {
			t.Errorf("latexToUnicode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStackLimits(t *testing.T) {
	got := stackLimits("∑ⁿᵢ₌₁ i = (n(n+1))/2")
	want := []string{
		" n",
		" ∑ i = (n(n+1))/2",
		"i=1",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	got = stackLimits("∫₀¹ f(x) dx + ∏_(k=1)^N a_k")
	want = []string{
		"1           N",
		"∫ f(x) dx + ∏ a_k",
		"0          k=1",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := stackLimits("x² + y²"); len(got) != 1 || got[0] != "x² + y²" {
		t.Errorf("lines without big operators must be unchanged, got %q", got)
	}
	if out := convertMath("$$\\sum_{i=1}^{n} i$$"); !strings.Contains(out, " n\n ∑ i\ni=1") {
		t.Errorf("display math should stack limits, got %q", out)
	}
}
