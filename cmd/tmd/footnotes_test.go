package main

import "testing"

func TestConvertFootnotes(t *testing.T) {
	cases := map[string]string{
		"text[^1] more[^note].":              "text¹ more^[note].",
		"[^1]: The note.":                    "¹ The note.",
		"[^12]: Twelve\n[^x]: X":             "¹² Twelve\n^[x] X",
		"no footnotes [here]":                "no footnotes [here]",
		"```\ncode[^1]\n```":                 "```\ncode[^1]\n```",
		"a [^1]: inline colon is left alone": "a [^1]: inline colon is left alone",
	}
	for in, want := range cases {
		if got := convertFootnotes(in); got != want {
			t.Errorf("convertFootnotes(%q) = %q, want %q", in, got, want)
		}
	}
}
