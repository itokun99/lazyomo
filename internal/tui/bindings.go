package tui

import "strings"

// This file owns the ONE binding table (P5): every key the TUI handles is
// declared here exactly once, and dispatch (update.go), the keybar, and the
// help overlay all render from it. The consistency test pins that contract:
// the keybar for a context is exactly the table's bar subset joined, and
// the help text contains every table entry. No key is handled outside the
// table except printable-rune input (filter/edit/add buffers, picker query)
// and the global ctrl+c quit.

// Action names one TUI operation. Dispatch switches on it; the keybar and
// help show the human pair.
type Action int

const (
	actNone Action = iota
	actMoveUp
	actMoveDown
	actFocusLeft
	actFocusRight
	actNextPane
	actPrevPane
	actPrevSection
	actNextSection
	actJumpVisible
	actDrillDown
	actClimbUp
	actOpenEdit
	actOpenAdd
	actOpenDelete
	actToggle
	actSave
	actReload
	actHelp
	actQuit
	actFilter
	actFilterDone
	actFilterClear
	actFilterBackspace
	actConfirm
	actCancel
	actSubmit
	actBackspace
	actHelpDown
	actHelpUp
	actPickerFilter
	actPickerSelect
	actPickerCancel
	actPickerPreview
	actPickerRaw
	actPickerMove
	actPickerClear
)

// Binding declares one key inside one context: the tea match token, the
// display label, a short description, and the action. Bar marks the pairs
// the keybar shows; every binding still dispatches and appears in help.
type Binding struct {
	Key   string
	Label string
	Desc  string
	Act   Action
	Arg   int
	Bar   bool
}

// ContextTable is the binding table for one focus/overlay context. The
// picker context declares todo 13's keys ahead of its UI so the table
// already models every context string.
type ContextTable struct {
	Name  string
	Title string
	Keys  []Binding
}

// Context names, shared by dispatch, keybar, and help.
const (
	ctxSections = "sections"
	ctxEntries  = "entries"
	ctxDetail   = "detail"
	ctxFilter   = "filter"
	ctxConfirm  = "confirm"
	ctxEdit     = "edit"
	ctxAdd      = "add"
	ctxError    = "error"
	ctxHelp     = "help"
	ctxPicker   = "picker"
)

// bindingTables is the single source of truth for keys.
var bindingTables = []ContextTable{
	{
		Name: ctxSections, Title: "Side panel",
		Keys: []Binding{
			{Key: "j", Label: "j/k", Desc: "move", Act: actMoveDown, Bar: true},
			{Key: "k", Label: "j/k", Desc: "move", Act: actMoveUp},
			{Key: "down", Label: "j/k", Desc: "move", Act: actMoveDown},
			{Key: "up", Label: "j/k", Desc: "move", Act: actMoveUp},
			{Key: "enter", Label: "enter", Desc: "open", Act: actDrillDown, Bar: true},
			{Key: "esc", Label: "esc", Desc: "back", Act: actClimbUp},
			{Key: "l", Label: "h/l", Desc: "pane", Act: actFocusRight},
			{Key: "h", Label: "h/l", Desc: "pane", Act: actFocusLeft},
			{Key: "right", Label: "h/l", Desc: "pane", Act: actFocusRight},
			{Key: "left", Label: "h/l", Desc: "pane", Act: actFocusLeft},
			{Key: "tab", Label: "tab", Desc: "next pane", Act: actNextPane},
			{Key: "shift+tab", Label: "S-tab", Desc: "prev pane", Act: actPrevPane},
			{Key: "[", Label: "[ ]", Desc: "cycle", Act: actPrevSection, Bar: true},
			{Key: "]", Label: "[ ]", Desc: "cycle", Act: actNextSection},
			{Key: "1", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 0, Bar: true},
			{Key: "2", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 1},
			{Key: "3", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 2},
			{Key: "4", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 3},
			{Key: "5", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 4},
			{Key: "6", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 5},
			{Key: "7", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 6},
			{Key: "8", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 7},
			{Key: "9", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 8},
			{Key: "0", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 9},
			{Key: "s", Label: "s", Desc: "save", Act: actSave, Bar: true},
			{Key: "r", Label: "r", Desc: "reload", Act: actReload, Bar: true},
			{Key: "?", Label: "?", Desc: "help", Act: actHelp, Bar: true},
			{Key: "q", Label: "q", Desc: "quit", Act: actQuit, Bar: true},
		},
	},
	{
		Name: ctxEntries, Title: "Entries",
		Keys: []Binding{
			{Key: "j", Label: "j/k", Desc: "move", Act: actMoveDown},
			{Key: "k", Label: "j/k", Desc: "move", Act: actMoveUp},
			{Key: "down", Label: "j/k", Desc: "move", Act: actMoveDown},
			{Key: "up", Label: "j/k", Desc: "move", Act: actMoveUp},
			{Key: "enter", Label: "enter", Desc: "detail", Act: actDrillDown},
			{Key: "esc", Label: "esc", Desc: "back", Act: actClimbUp},
			{Key: "e", Label: "e", Desc: "edit", Act: actOpenEdit, Bar: true},
			{Key: "a", Label: "a", Desc: "add", Act: actOpenAdd, Bar: true},
			{Key: "d", Label: "d", Desc: "del", Act: actOpenDelete, Bar: true},
			{Key: " ", Label: "space", Desc: "toggle", Act: actToggle, Bar: true},
			{Key: "/", Label: "/", Desc: "filter", Act: actFilter},
			{Key: "h", Label: "h/l", Desc: "pane", Act: actFocusLeft},
			{Key: "l", Label: "h/l", Desc: "pane", Act: actFocusRight},
			{Key: "left", Label: "h/l", Desc: "pane", Act: actFocusLeft},
			{Key: "right", Label: "h/l", Desc: "pane", Act: actFocusRight},
			{Key: "tab", Label: "tab", Desc: "next pane", Act: actNextPane},
			{Key: "shift+tab", Label: "S-tab", Desc: "prev pane", Act: actPrevPane},
			{Key: "[", Label: "[ ]", Desc: "cycle", Act: actPrevSection},
			{Key: "]", Label: "[ ]", Desc: "cycle", Act: actNextSection},
			{Key: "1", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 0},
			{Key: "2", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 1},
			{Key: "3", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 2},
			{Key: "4", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 3},
			{Key: "5", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 4},
			{Key: "6", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 5},
			{Key: "7", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 6},
			{Key: "8", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 7},
			{Key: "9", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 8},
			{Key: "0", Label: "1-9,0", Desc: "jump", Act: actJumpVisible, Arg: 9},
			{Key: "s", Label: "s", Desc: "save", Act: actSave, Bar: true},
			{Key: "r", Label: "r", Desc: "reload", Act: actReload, Bar: true},
			{Key: "?", Label: "?", Desc: "help", Act: actHelp, Bar: true},
			{Key: "q", Label: "q", Desc: "quit", Act: actQuit, Bar: true},
		},
	},
	{
		Name: ctxDetail, Title: "Detail",
		Keys: []Binding{
			{Key: "j", Label: "j/k", Desc: "move", Act: actMoveDown, Bar: true},
			{Key: "k", Label: "j/k", Desc: "move", Act: actMoveUp},
			{Key: "down", Label: "j/k", Desc: "move", Act: actMoveDown},
			{Key: "up", Label: "j/k", Desc: "move", Act: actMoveUp},
			{Key: "e", Label: "e", Desc: "edit", Act: actOpenEdit, Bar: true},
			{Key: " ", Label: "space", Desc: "toggle", Act: actToggle, Bar: true},
			{Key: "h", Label: "h/l", Desc: "pane", Act: actFocusLeft},
			{Key: "l", Label: "h/l", Desc: "pane", Act: actFocusRight},
			{Key: "left", Label: "h/l", Desc: "pane", Act: actFocusLeft},
			{Key: "right", Label: "h/l", Desc: "pane", Act: actFocusRight},
			{Key: "tab", Label: "tab", Desc: "next pane", Act: actNextPane},
			{Key: "shift+tab", Label: "S-tab", Desc: "prev pane", Act: actPrevPane},
			{Key: "esc", Label: "esc", Desc: "back", Act: actClimbUp, Bar: true},
			{Key: "s", Label: "s", Desc: "save", Act: actSave, Bar: true},
			{Key: "r", Label: "r", Desc: "reload", Act: actReload, Bar: true},
			{Key: "?", Label: "?", Desc: "help", Act: actHelp, Bar: true},
			{Key: "q", Label: "q", Desc: "quit", Act: actQuit, Bar: true},
		},
	},
	{
		Name: ctxFilter, Title: "Filter",
		Keys: []Binding{
			{Key: "enter", Label: "enter", Desc: "done", Act: actFilterDone, Bar: true},
			{Key: "esc", Label: "esc", Desc: "clear", Act: actFilterClear, Bar: true},
			{Key: "backspace", Label: "bsp", Desc: "delete", Act: actFilterBackspace},
		},
	},
	{
		Name: ctxConfirm, Title: "Confirm",
		Keys: []Binding{
			{Key: "enter", Label: "enter", Desc: "confirm", Act: actConfirm, Bar: true},
			{Key: "esc", Label: "esc", Desc: "cancel", Act: actCancel, Bar: true},
		},
	},
	{
		Name: ctxEdit, Title: "Edit",
		Keys: []Binding{
			{Key: "enter", Label: "enter", Desc: "apply", Act: actSubmit, Bar: true},
			{Key: "esc", Label: "esc", Desc: "cancel", Act: actCancel, Bar: true},
			{Key: "backspace", Label: "bsp", Desc: "delete", Act: actBackspace},
		},
	},
	{
		Name: ctxAdd, Title: "Add",
		Keys: []Binding{
			{Key: "enter", Label: "enter", Desc: "add", Act: actSubmit, Bar: true},
			{Key: "esc", Label: "esc", Desc: "cancel", Act: actCancel, Bar: true},
			{Key: "backspace", Label: "bsp", Desc: "delete", Act: actBackspace},
		},
	},
	{
		Name: ctxError, Title: "Error",
		Keys: []Binding{
			{Key: "enter", Label: "enter", Desc: "close", Act: actCancel, Bar: true},
			{Key: "esc", Label: "esc", Desc: "close", Act: actCancel, Bar: true},
		},
	},
	{
		Name: ctxHelp, Title: "Help",
		Keys: []Binding{
			{Key: "?", Label: "?", Desc: "close", Act: actCancel, Bar: true},
			{Key: "esc", Label: "esc", Desc: "close", Act: actCancel, Bar: true},
			{Key: "j", Label: "j/k", Desc: "scroll", Act: actHelpDown},
			{Key: "k", Label: "j/k", Desc: "scroll", Act: actHelpUp},
			{Key: "down", Label: "j/k", Desc: "scroll", Act: actHelpDown},
			{Key: "up", Label: "j/k", Desc: "scroll", Act: actHelpUp},
		},
	},
	{
		Name: ctxPicker, Title: "Picker (next wave)",
		Keys: []Binding{
			{Key: "enter", Label: "enter", Desc: "select", Act: actPickerSelect, Bar: true},
			{Key: "esc", Label: "esc", Desc: "cancel", Act: actPickerCancel, Bar: true},
			{Key: "tab", Label: "tab", Desc: "preview", Act: actPickerPreview, Bar: true},
			{Key: "ctrl+e", Label: "C-e", Desc: "raw", Act: actPickerRaw, Bar: true},
			{Key: "up", Label: "up/dn", Desc: "move", Act: actPickerMove, Bar: true},
			{Key: "down", Label: "up/dn", Desc: "move", Act: actPickerMove},
			{Key: "ctrl+n", Label: "C-n/p", Desc: "move", Act: actPickerMove},
			{Key: "ctrl+p", Label: "C-n/p", Desc: "move", Act: actPickerMove},
			{Key: "pgup", Label: "pgup/dn", Desc: "page", Act: actPickerMove},
			{Key: "pgdown", Label: "pgup/dn", Desc: "page", Act: actPickerMove},
			{Key: "home", Label: "home/end", Desc: "ends", Act: actPickerMove},
			{Key: "end", Label: "home/end", Desc: "ends", Act: actPickerMove},
			{Key: "backspace", Label: "bsp", Desc: "delete", Act: actBackspace},
			{Key: "ctrl+u", Label: "C-u", Desc: "clear", Act: actPickerClear},
		},
	},
}

// tableFor returns the binding table for a context name.
func tableFor(name string) *ContextTable {
	for i := range bindingTables {
		if bindingTables[i].Name == name {
			return &bindingTables[i]
		}
	}
	return nil
}

// lookupAction resolves a key string inside one context table.
func lookupAction(table *ContextTable, key string) (Binding, bool) {
	if table == nil {
		return Binding{}, false
	}
	for _, b := range table.Keys {
		if b.Key == key {
			return b, true
		}
	}
	return Binding{}, false
}

// keybarFor renders the keybar line for a context: exactly the table's bar
// subset, "label desc" pairs joined with two spaces.
func keybarFor(name string) string {
	table := tableFor(name)
	if table == nil {
		return ""
	}
	seen := map[string]bool{}
	parts := []string{}
	for _, b := range table.Keys {
		if !b.Bar {
			continue
		}
		pair := b.Label + " " + b.Desc
		if seen[pair] {
			continue
		}
		seen[pair] = true
		parts = append(parts, pair)
	}
	return strings.Join(parts, "  ")
}
