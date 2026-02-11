package tui

import "github.com/charmbracelet/bubbles/key"

// SharedKeyMap defines keybindings available on all screens.
type SharedKeyMap struct {
	ForceQuit key.Binding
	Quit      key.Binding
}

// SharedKeys are available on all screens.
var SharedKeys = SharedKeyMap{
	ForceQuit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "force quit"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
}

// ListKeyMap defines keybindings for the main list/results screen.
type ListKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Expand       key.Binding
	Collapse     key.Binding
	Search       key.Binding
	SortByName   key.Binding
	SortByStatus key.Binding
	SortByPath   key.Binding
	Filter       key.Binding
	Edit         key.Binding
	AddApp       key.Binding
	AddEntry     key.Binding
	Delete       key.Binding
	Restore      key.Binding
	Install      key.Binding
	Toggle       key.Binding
	ShowDetail   key.Binding
	NewOperation key.Binding
	QuitOrEnter  key.Binding
}

// ListKeys are the keybindings for the list screen.
var ListKeys = ListKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Expand: key.NewBinding(
		key.WithKeys("enter", "l", "right"),
		key.WithHelp("l/→", "expand"),
	),
	Collapse: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/←", "collapse"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	SortByName: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "name"),
	),
	SortByStatus: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "status"),
	),
	SortByPath: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "path"),
	),
	Filter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "filter"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
	AddApp: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "add app"),
	),
	AddEntry: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add entry"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "delete", "backspace"),
		key.WithHelp("d", "delete"),
	),
	Restore: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "restore"),
	),
	Install: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "install"),
	),
	Toggle: key.NewBinding(
		key.WithKeys("tab", " "),
		key.WithHelp("tab", "toggle"),
	),
	ShowDetail: key.NewBinding(
		key.WithKeys("enter", "l", "right"),
		key.WithHelp("enter", "detail"),
	),
	NewOperation: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "new operation"),
	),
	QuitOrEnter: key.NewBinding(
		key.WithKeys("q", "enter"),
		key.WithHelp("q/enter", "quit"),
	),
}

// MultiSelectKeyMap defines keybindings for the multi-select mode.
type MultiSelectKeyMap struct {
	Toggle  key.Binding
	Clear   key.Binding
	Restore key.Binding
	Install key.Binding
	Delete  key.Binding
}

// MultiSelectKeys are the keybindings for multi-select mode.
var MultiSelectKeys = MultiSelectKeyMap{
	Toggle: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "toggle"),
	),
	Clear: key.NewBinding(
		key.WithKeys("esc"),
	),
	Restore: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "restore"),
	),
	Install: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "install"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
}

// SearchKeyMap defines keybindings for search mode.
type SearchKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
}

// SearchKeys are the keybindings for search mode.
var SearchKeys = SearchKeyMap{
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
	),
}

// ConfirmKeyMap defines keybindings for confirmation dialogs.
type ConfirmKeyMap struct {
	Yes key.Binding
	No  key.Binding
}

// ConfirmKeys are the keybindings for confirmation dialogs.
var ConfirmKeys = ConfirmKeyMap{
	Yes: key.NewBinding(
		key.WithKeys("y", "Y", "enter"),
		key.WithHelp("y/enter", "yes"),
	),
	No: key.NewBinding(
		key.WithKeys("n", "N", "esc"),
		key.WithHelp("n/esc", "no"),
	),
}

// DetailKeyMap defines keybindings for the detail popup.
type DetailKeyMap struct {
	Close key.Binding
}

// DetailKeys are the keybindings for the detail popup.
var DetailKeys = DetailKeyMap{
	Close: key.NewBinding(
		key.WithKeys("esc", "enter"),
		key.WithHelp("h/←/esc", "close"),
	),
}

// FormNavKeyMap defines keybindings for form navigation (not editing).
type FormNavKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	TabNext key.Binding
	TabPrev key.Binding
	Edit    key.Binding
	Save    key.Binding
	Cancel  key.Binding
	Toggle  key.Binding
	Delete  key.Binding
}

// FormNavKeys are the keybindings for form navigation.
var FormNavKeys = FormNavKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	TabNext: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next"),
	),
	TabPrev: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev"),
	),
	Edit: key.NewBinding(
		key.WithKeys("enter", "e"),
		key.WithHelp("e", "edit"),
	),
	Save: key.NewBinding(
		key.WithKeys("s", "ctrl+s"),
		key.WithHelp("s", "save"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	Toggle: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "backspace", "delete"),
		key.WithHelp("d", "delete"),
	),
}

// TextEditKeyMap defines keybindings when editing a text field.
type TextEditKeyMap struct {
	Confirm  key.Binding
	Cancel   key.Binding
	SaveForm key.Binding
}

// TextEditKeys are the keybindings for text editing mode.
var TextEditKeys = TextEditKeyMap{
	// Confirm: key.NewBinding(
	// 	key.WithKeys("enter", "tab"),
	// 	key.WithHelp("enter/tab", "save"),
	// ),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel edit"),
	),
	SaveForm: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "save form"),
	),
}

// SuggestionKeyMap defines keybindings for autocomplete suggestions.
type SuggestionKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Accept key.Binding
	Cancel key.Binding
}

// SuggestionKeys are the keybindings for suggestion navigation.
var SuggestionKeys = SuggestionKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑/↓", "select"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↑/↓", "select"),
	),
	Accept: key.NewBinding(
		key.WithKeys("tab", "enter"),
		key.WithHelp("tab/enter", "accept"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel edit"),
	),
}

// SummaryKeyMap defines keybindings for the summary/confirmation screen.
type SummaryKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
}

// SummaryKeys are the keybindings for the summary screen.
var SummaryKeys = SummaryKeyMap{
	Confirm: key.NewBinding(
		key.WithKeys("y", "Y", "enter"),
		key.WithHelp("y/enter", "confirm"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("n", "N", "esc"),
		key.WithHelp("n/esc", "cancel"),
	),
}

// DiffPickerKeyMap defines keybindings for the diff file picker.
type DiffPickerKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Cancel key.Binding
}

// DiffPickerKeys are the keybindings for the diff picker.
var DiffPickerKeys = DiffPickerKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}

// FilePickerKeyMap defines keybindings for the file picker.
type FilePickerKeyMap struct {
	Toggle  key.Binding
	Confirm key.Binding
	Cancel  key.Binding
}

// FilePickerKeys are the keybindings for the file picker.
var FilePickerKeys = FilePickerKeyMap{
	Toggle: key.NewBinding(
		key.WithKeys(" ", "tab"),
		key.WithHelp("space/tab", "toggle"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}

// ModeChooserKeyMap defines keybindings for the file add mode chooser.
type ModeChooserKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Cancel key.Binding
}

// ModeChooserKeys are the keybindings for the mode chooser.
var ModeChooserKeys = ModeChooserKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}

// FilesListKeyMap defines keybindings for the files list within forms.
type FilesListKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Edit   key.Binding
	Delete key.Binding
	Save   key.Binding
}

// FilesListKeys are the keybindings for the files list.
var FilesListKeys = FilesListKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Edit: key.NewBinding(
		key.WithKeys("enter", " ", "e"),
		key.WithHelp("e", "edit"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "backspace", "delete"),
		key.WithHelp("d", "delete"),
	),
	Save: key.NewBinding(
		key.WithKeys("s", "ctrl+s"),
		key.WithHelp("s", "save"),
	),
}
