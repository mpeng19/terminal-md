package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// action is something a key (or key sequence) can trigger.
type action string

const (
	actNone      action = ""
	actQuit      action = "quit"
	actDown      action = "down" // cursor one line
	actUp        action = "up"
	actNextBlock action = "next_block" // cursor one block
	actPrevBlock action = "prev_block"
	actScrollDn  action = "scroll_down" // view one line, cursor stays
	actScrollUp  action = "scroll_up"
	actPageDown  action = "page_down"
	actPageUp    action = "page_up"
	actHalfDown  action = "half_down"
	actHalfUp    action = "half_up"
	actTop       action = "top"
	actBottom    action = "bottom"
	actLeft      action = "left"
	actRight     action = "right"
	actReload    action = "reload"
	actEdit      action = "edit"
	actAppend    action = "append"
	actNewBelow  action = "new_below"
	actNewAbove  action = "new_above"
	actDelete    action = "delete"
	actUndo      action = "undo"
	actRedo      action = "redo"
	actSave      action = "save"
	actSelect    action = "select"
	actDone      action = "done"
	actCancelKey action = "cancel" // internal: clears a pending chord/confirmation
)

// bindings maps actions to the key sequences that trigger them. A sequence
// is one or more key names ("q", "ctrl+s", "space"); multi-key chords like
// vim's "dd" are written as "d d".
type bindings map[action][]string

type keymap struct {
	normal bindings
	insert bindings
}

func defaultKeymap() keymap {
	return keymap{
		normal: bindings{
			actQuit:      {"q", "ctrl+c"},
			actDown:      {"down"},
			actUp:        {"up"},
			actNextBlock: {"j"},
			actPrevBlock: {"k"},
			actScrollDn:  {"ctrl+e"},
			actScrollUp:  {"ctrl+y"},
			actPageDown:  {"space", "f", "pgdown"},
			actPageUp:    {"b", "pgup"},
			actHalfDown:  {"ctrl+d"},
			actHalfUp:    {"ctrl+u"},
			actTop:       {"g", "home"},
			actBottom:    {"G", "end"},
			actLeft:      {"h", "left"},
			actRight:     {"l", "right"},
			actReload:    {"r"},
			actEdit:      {"i", "enter"},
			actAppend:    {"a"},
			actNewBelow:  {"o"},
			actNewAbove:  {"O"},
			actDelete:    {"d d"},
			actUndo:      {"u", "ctrl+z"},
			actRedo:      {"ctrl+r"},
			actSave:      {"ctrl+s"},
			actSelect:    {"v"},
		},
		insert: bindings{
			actDone: {"esc"},
			actSave: {"ctrl+s"},
			actUndo: {"ctrl+z"},
			actRedo: {"ctrl+r"},
		},
	}
}

// keyName normalises a key press to the names used in bindings.
func keyName(msg tea.KeyMsg) string {
	s := msg.String()
	if s == " " {
		return "space"
	}
	return s
}

// resolve matches a sequence of pressed keys against the bindings. It
// returns the matched action, or actNone plus whether the sequence is a
// prefix of some longer binding (so the caller should wait for more keys).
func (b bindings) resolve(seq []string) (action, bool) {
	prefix := false
	for act, seqs := range b {
		for _, s := range seqs {
			keys := strings.Fields(s)
			if len(keys) < len(seq) {
				continue
			}
			match := true
			for i := range seq {
				if keys[i] != seq[i] {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			if len(keys) == len(seq) {
				return act, false
			}
			prefix = true
		}
	}
	return actNone, prefix
}

// firstKey returns the primary key of an action, for hint text.
func (b bindings) firstKey(act action) string {
	if seqs := b[act]; len(seqs) > 0 {
		return strings.ReplaceAll(seqs[0], " ", "")
	}
	return ""
}
