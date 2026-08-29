package main

import "testing"

func TestDocumentRoundTrip(t *testing.T) {
	cases := []string{
		"",
		"\n",
		"hello",
		"hello\n",
		"# Title\n\nParagraph one\ncontinued.\n\n\n- a\n- b\n",
		"\n\nleading blanks\n",
		"para\n```go\nfmt.Println()\n\nmore\n```\n\nafter\n",
		"para\n```go\nunclosed\n\nstill code\n",
		"~~~~\n```\nnot a closer\n~~~~\ntext",
	}
	for _, src := range cases {
		if got := parseDocument(src).String(); got != src {
			t.Errorf("round trip mismatch\n src: %q\n got: %q", src, got)
		}
	}
}

func TestDocumentBlocks(t *testing.T) {
	d := parseDocument("# Title\n\npara\n```\ncode\n\nmore\n```\nlast\n")
	want := []string{"# Title", "para", "```\ncode\n\nmore\n```", "last"}
	if len(d.blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d: %+v", len(d.blocks), len(want), d.blocks)
	}
	for i, w := range want {
		if d.blocks[i].src != w {
			t.Errorf("block %d = %q, want %q", i, d.blocks[i].src, w)
		}
	}
}

func TestDocumentEdits(t *testing.T) {
	d := parseDocument("a\n\nb\n\nc\n")

	d.replaceBlock(1, "b1\n\nb2")
	if got, want := d.String(), "a\n\nb1\n\nb2\n\nc\n"; got != want {
		t.Errorf("replace/split: got %q want %q", got, want)
	}

	d.replaceBlock(2, "")
	if got, want := d.String(), "a\n\nb1\n\nc\n"; got != want {
		t.Errorf("replace with empty deletes: got %q want %q", got, want)
	}

	i := d.insertAfter(0)
	d.replaceBlock(i, "new")
	if got, want := d.String(), "a\n\nnew\n\nb1\n\nc\n"; got != want {
		t.Errorf("insertAfter: got %q want %q", got, want)
	}

	i = d.insertBefore(0)
	d.replaceBlock(i, "first")
	if got, want := d.String(), "first\n\na\n\nnew\n\nb1\n\nc\n"; got != want {
		t.Errorf("insertBefore: got %q want %q", got, want)
	}

	d.deleteBlock(len(d.blocks) - 1)
	if got, want := d.String(), "first\n\na\n\nnew\n\nb1\n"; got != want {
		t.Errorf("deleteBlock last: got %q want %q", got, want)
	}

	for len(d.blocks) > 1 {
		d.deleteBlock(0)
	}
	d.deleteBlock(0)
	if len(d.blocks) != 1 || d.blocks[0].src != "" {
		t.Errorf("deleting everything should leave one empty block, got %+v", d.blocks)
	}
	if d.String() != "" {
		t.Errorf("empty doc should stringify to empty, got %q", d.String())
	}
}

func TestFenceAfterParagraphKeepsSeparation(t *testing.T) {
	d := parseDocument("para\n```\ncode\n```\n")
	d.replaceBlock(1, "plain text")
	if got, want := d.String(), "para\n\nplain text\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
