# terminal-md demo

A quick tour of what `tmd` renders. Scroll with **j/k** or the arrow keys,
jump with **g/G**, and press **q** to leave.

## Text

Plain paragraphs wrap to the width of the box. *Emphasis*, **strong**,
~~strikethrough~~, and `inline code` are all supported, as are
[links](https://github.com/charmbracelet/glamour) and emoji shortcodes :rocket:.

> Blockquotes are rendered with a gutter, and can span
> multiple lines of wrapped text without losing the gutter.

## Lists

- Unordered lists
- With nested items
  - like this one
  - and this one
- Back at the top level

1. Ordered lists
2. Work too
3. And renumber correctly

- [x] Task lists
- [ ] Are supported

## Code

```go
package main

import "fmt"

func main() {
	fmt.Println("hello from a syntax-highlighted code block")
}
```

```sh
go install github.com/michaelpeng/terminal-md/cmd/tmd@latest
tmd README.md
```

## Tables

| Key       | Action              |
|-----------|---------------------|
| `j` / `↓` | scroll down a line  |
| `k` / `↑` | scroll up a line    |
| `space`   | page down           |
| `g` / `G` | go to top / bottom  |
| `r`       | reload the file     |
| `q`       | quit                |

## Headings

### Level three

#### Level four

##### Level five

---

## Long content

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis
nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.

Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu
fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in
culpa qui officia deserunt mollit anim id est laborum.

Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium
doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore
veritatis et quasi architecto beatae vitae dicta sunt explicabo.

Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed
quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt.

*The end.*
