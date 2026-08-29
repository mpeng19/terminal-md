package main

import (
	"regexp"
	"strings"
)

// glamour's goldmark has no footnote extension, so footnotes are rewritten
// before rendering: references become superscripts and definitions become
// a superscript followed by the note text.
var (
	footnoteRef = regexp.MustCompile(`\[\^([^\]\s]+)\](:?)`)
	footnoteDef = regexp.MustCompile(`(?m)^\[\^([^\]\s]+)\]:[ \t]*`)
)

func convertFootnotes(src string) string {
	if !strings.Contains(src, "[^") || isFenced(src) {
		return src
	}
	src = footnoteDef.ReplaceAllStringFunc(src, func(m string) string {
		label := footnoteDef.FindStringSubmatch(m)[1]
		return superscriptLabel(label) + " "
	})
	return footnoteRef.ReplaceAllStringFunc(src, func(m string) string {
		sub := footnoteRef.FindStringSubmatch(m)
		if sub[2] == ":" {
			return m // a definition not at the start of a line; leave it
		}
		return superscriptLabel(sub[1])
	})
}

// superscriptLabel renders a footnote label as superscript characters where
// possible ("12" → "¹²"), else as "^[label]".
func superscriptLabel(label string) string {
	if strings.Trim(label, "0123456789") == "" {
		if s, ok := toScript(label, texSuperscripts); ok {
			return s
		}
	}
	return "^[" + label + "]"
}
