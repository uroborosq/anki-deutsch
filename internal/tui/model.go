package tui

import (
	"anki/internal/deckbuilder"
	"anki/internal/deckbuilder/usecase"
	"anki/internal/flashcard"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Service is the inbound port the UI drives. It is an alias of usecase.Service so
// the package depends on the use-case port only and never on the adapters.
type Service = usecase.Service

// state enumerates the screens of the UI and the flow between them.
type state int

const (
	stateDeckPick     state = iota // 1. choose a deck
	stateModePick                  // 2. choose add vs. scan
	stateAddInput                  // 3a. type a word
	stateAddLoading                // 3b. PreviewWord (lookup) in flight
	stateAddPreview                // 3c. show the card, await confirm
	stateAddEdit                   // 3c'. edit the card's Front/Back before adding
	stateAddSaving                 // 3d. AddNote in flight
	stateAddResult                 // 3e. show note id or error
	stateScanLoading               // 4a. ScanDeck in flight
	stateScanList                  // 4b. review suggestions
	stateApplyLoading              // 4c. Apply in flight
	stateScanSummary               // 4d. updated/skipped summary
)

const (
	modeAddTitle  = "Add word"
	modeScanTitle = "Scan deck"
)

// listItem is a single row in the deck and mode pickers.
type listItem struct {
	title string
	desc  string
}

func (i listItem) Title() string { return i.title }

func (i listItem) Description() string { return i.desc }

func (i listItem) FilterValue() string { return i.title }

// Model is the root Bubble Tea model. It depends only on the Service port.
type Model struct {
	svc    Service
	styles Styles
	state  state

	width  int
	height int

	deck           flashcard.DeckName
	hasDefaultDeck bool

	deckList list.Model
	modeList list.Model
	input    textinput.Model
	spinner  spinner.Model

	// add preview/result
	previewNote flashcard.Note
	lastLemma   string // last word submitted for lookup, kept to refill the input
	lastID      uint64
	lastErr     error

	// add edit: Front is a single line, Back is multi-line. editFocus is 0 for
	// the Front field, 1 for the Back field. manualEdit is true when the editor
	// was opened directly from the input screen (ctrl+e) rather than from a
	// preview, so esc returns to the input instead of a non-existent preview.
	editFront  textinput.Model
	editBack   textarea.Model
	editFocus  int
	manualEdit bool

	// scan/apply
	suggestions []deckbuilder.Suggestion
	cursor      int
	selected    map[int]bool
	updated     int
	skipped     int
}

// New builds the UI. When defaultDeck is non-empty the deck picker is skipped and
// the UI opens on the mode screen for that deck.
func New(svc usecase.Service, defaultDeck string) *Model {
	st := DefaultStyles()

	ti := textinput.New()
	ti.Placeholder = "German word…"
	ti.Prompt = "› "
	ti.CharLimit = 64

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = st.Selected

	editFront := textinput.New()
	editFront.Prompt = "› "
	editFront.CharLimit = 128

	editBack := textarea.New()
	editBack.Prompt = "  "
	editBack.ShowLineNumbers = false
	editBack.CharLimit = 2048

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = st.Selected
	delegate.Styles.SelectedDesc = st.Faint

	deckList := list.New(nil, delegate, 0, 0)
	deckList.Title = "Pick a deck"
	deckList.SetShowStatusBar(false)
	deckList.SetShowHelp(false)
	deckList.Styles.Title = st.Title

	modeList := list.New([]list.Item{
		listItem{title: modeAddTitle, desc: "Add a single word to the deck"},
		listItem{title: modeScanTitle, desc: "Scan the deck and fill in blanks"},
	}, delegate, 0, 0)
	modeList.Title = "Choose a mode"
	modeList.SetShowStatusBar(false)
	modeList.SetShowHelp(false)
	modeList.SetFilteringEnabled(false)
	modeList.Styles.Title = st.Title

	m := &Model{
		svc:       svc,
		styles:    st,
		input:     ti,
		spinner:   sp,
		deckList:  deckList,
		modeList:  modeList,
		editFront: editFront,
		editBack:  editBack,
		selected:  map[int]bool{},
	}

	if defaultDeck != "" {
		m.deck = flashcard.DeckName(defaultDeck)
		m.hasDefaultDeck = true
		m.state = stateModePick
	} else {
		m.state = stateDeckPick
	}

	return m
}

// Init kicks off the deck fetch unless a default deck was supplied.
func (m *Model) Init() tea.Cmd {
	if m.hasDefaultDeck {
		return nil
	}

	return decksCmd(m.svc)
}

// Update is the pure transition function. Service methods are never called here
// directly — only through the tea.Cmds returned alongside the next model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd

		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case decksMsg:
		m.setDecks(msg.decks)
		return m, nil
	case decksErrMsg:
		m.lastErr = msg.err
		m.state = stateDeckPick

		return m, nil

	case previewDoneMsg:
		m.previewNote = msg.note
		m.lastErr = nil
		m.state = stateAddPreview

		return m, nil
	case addDoneMsg:
		m.lastID = msg.id
		m.lastErr = nil
		m.state = stateAddResult

		return m, nil
	case addErrMsg:
		m.lastErr = msg.err
		m.state = stateAddResult

		return m, nil
	case lookupErrMsg:
		m.lastErr = msg.err
		m.state = stateAddResult

		return m, nil

	case scanDoneMsg:
		m.setSuggestions(msg.suggestions)
		return m, nil
	case scanErrMsg:
		m.lastErr = msg.err
		m.state = stateScanSummary

		return m, nil

	case applyDoneMsg:
		m.updated = msg.updated
		m.skipped = msg.skipped
		m.lastErr = nil
		m.state = stateScanSummary

		return m, nil
	case applyErrMsg:
		m.lastErr = msg.err
		m.state = stateScanSummary

		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		// 'q' quits everywhere except while typing into a text field.
		if msg.String() == "q" && m.state != stateAddInput && m.state != stateAddEdit {
			return m, tea.Quit
		}

		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey routes key presses to the active screen.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateDeckPick:
		if msg.Type == tea.KeyEnter {
			if it, ok := m.deckList.SelectedItem().(listItem); ok {
				m.deck = flashcard.DeckName(it.title)
				m.state = stateModePick
			}

			return m, nil
		}

		var cmd tea.Cmd

		m.deckList, cmd = m.deckList.Update(msg)

		return m, cmd

	case stateModePick:
		switch msg.Type {
		case tea.KeyEnter:
			if it, ok := m.modeList.SelectedItem().(listItem); ok {
				return m.startMode(it.title)
			}

			return m, nil
		case tea.KeyEsc:
			if !m.hasDefaultDeck {
				m.state = stateDeckPick
			}

			return m, nil
		}

		var cmd tea.Cmd

		m.modeList, cmd = m.modeList.Update(msg)

		return m, cmd

	case stateAddInput:
		switch msg.Type {
		case tea.KeyEnter:
			lemma := strings.TrimSpace(m.input.Value())
			if lemma == "" {
				return m, nil
			}

			m.lastLemma = lemma
			m.state = stateAddLoading

			return m, tea.Batch(m.spinner.Tick, previewWordCmd(m.svc, lemma))
		case tea.KeyCtrlE:
			// Manual add: skip the dictionary and edit a blank card by hand.
			lemma := strings.TrimSpace(m.input.Value())
			if lemma == "" {
				return m, nil
			}

			return m.enterManualEdit(lemma)
		case tea.KeyCtrlU:
			// Quickly clear the input field.
			m.input.Reset()
			return m, nil
		case tea.KeyEsc:
			m.input.Blur()
			m.state = stateModePick

			return m, nil
		}

		var cmd tea.Cmd

		m.input, cmd = m.input.Update(msg)

		return m, cmd

	case stateAddPreview:
		switch msg.String() {
		case "enter", "y":
			// Confirm: store the previewed note in Anki.
			m.state = stateAddSaving
			return m, tea.Batch(m.spinner.Tick, addNoteCmd(m.svc, m.deck, m.previewNote))
		case "e":
			// Edit: open the Front/Back fields seeded from the preview.
			return m.enterEdit()
		case "esc", "n":
			// Cancel: discard the preview but keep the word in the input so it can
			// be corrected and resubmitted.
			return m.returnToInput(true)
		}

		return m, nil

	case stateAddEdit:
		return m.handleEditKey(msg)

	case stateAddResult:
		// Any key returns to the input. On failure (e.g. the word was not found)
		// keep the word so it can be fixed; on success start fresh.
		return m.returnToInput(m.lastErr != nil)

	case stateScanList:
		return m.handleScanKey(msg)

	case stateScanSummary:
		// Any key returns to the mode picker.
		m.state = stateModePick
		return m, nil

	case stateAddLoading, stateAddSaving, stateScanLoading, stateApplyLoading:
		// Ignore input while a Service call is in flight (quit handled above).
		return m, nil
	}

	return m, nil
}

// startMode launches the chosen mode from the mode picker.
func (m *Model) startMode(title string) (tea.Model, tea.Cmd) {
	switch title {
	case modeAddTitle:
		m.state = stateAddInput
		m.input.Reset()

		return m, m.input.Focus()
	case modeScanTitle:
		m.state = stateScanLoading
		return m, tea.Batch(m.spinner.Tick, scanDeckCmd(m.svc, m.deck))
	}

	return m, nil
}

// returnToInput goes back to the word-input screen. When prefill is true the last
// submitted word is restored (cursor at the end) so it can be edited; otherwise
// the field is cleared for a fresh word.
func (m *Model) returnToInput(prefill bool) (tea.Model, tea.Cmd) {
	m.state = stateAddInput

	if prefill && m.lastLemma != "" {
		m.input.SetValue(m.lastLemma)
		m.input.CursorEnd()
	} else {
		m.input.Reset()
	}

	return m, m.input.Focus()
}

// enterEdit seeds the edit fields from the current preview and opens the editor
// with the Front field focused.
func (m *Model) enterEdit() (tea.Model, tea.Cmd) {
	m.editFront.SetValue(m.previewNote.Front)
	m.editBack.SetValue(m.previewNote.Back)
	m.editFocus = 0
	m.manualEdit = false
	m.state = stateAddEdit

	return m, m.focusEditField()
}

// enterManualEdit skips the dictionary lookup: it seeds a blank card from the
// typed word (Front = word, empty Back, tagged manual) and opens the editor on
// the Back field so the translation can be filled in by hand. It reuses the
// edit→preview→add path — ctrl+s returns to the preview, enter writes it to Anki.
func (m *Model) enterManualEdit(lemma string) (tea.Model, tea.Cmd) {
	m.lastLemma = lemma
	m.previewNote = flashcard.Note{
		Front: lemma,
		Tags:  []string{"german", "manual"},
	}
	m.input.Blur()
	m.editFront.SetValue(lemma)
	m.editBack.SetValue("")
	m.editFocus = 1 // focus the Back field — the Front is already the word
	m.manualEdit = true
	m.state = stateAddEdit

	return m, m.focusEditField()
}

// handleEditKey drives the Front/Back editor. ctrl+s commits the edits back into
// previewNote and returns to the preview; esc discards them; tab switches fields.
func (m *Model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.editFront.Blur()
		m.editBack.Blur()

		// Manual entry has no preview to fall back to — go straight back to the
		// word input, keeping the typed word so it can be retried.
		if m.manualEdit {
			return m.returnToInput(true)
		}

		m.state = stateAddPreview

		return m, nil
	case tea.KeyCtrlS:
		m.previewNote.Front = strings.TrimSpace(m.editFront.Value())
		m.previewNote.Back = strings.TrimRight(m.editBack.Value(), "\n")
		m.editFront.Blur()
		m.editBack.Blur()
		m.state = stateAddPreview

		return m, nil
	case tea.KeyTab, tea.KeyShiftTab:
		m.editFocus = 1 - m.editFocus
		return m, m.focusEditField()
	}

	var cmd tea.Cmd
	if m.editFocus == 0 {
		m.editFront, cmd = m.editFront.Update(msg)
	} else {
		m.editBack, cmd = m.editBack.Update(msg)
	}

	return m, cmd
}

// focusEditField focuses the field named by editFocus and blurs the other,
// returning the focused field's cursor-blink command.
func (m *Model) focusEditField() tea.Cmd {
	if m.editFocus == 0 {
		m.editBack.Blur()
		return m.editFront.Focus()
	}

	m.editFront.Blur()

	return m.editBack.Focus()
}

// handleScanKey drives the suggestion review screen.
func (m *Model) handleScanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.suggestions)-1 {
			m.cursor++
		}
	case " ":
		if len(m.suggestions) > 0 && !m.suggestions[m.cursor].Skipped {
			m.selected[m.cursor] = !m.selected[m.cursor]
		}
	case "a":
		chosen := m.chosenSuggestions()
		m.state = stateApplyLoading

		return m, tea.Batch(m.spinner.Tick, applyCmd(m.svc, chosen))
	case "esc":
		m.state = stateModePick
	}

	return m, nil
}

// chosenSuggestions returns the selected, non-skipped suggestions to apply.
func (m *Model) chosenSuggestions() []deckbuilder.Suggestion {
	chosen := make([]deckbuilder.Suggestion, 0, len(m.suggestions))
	for i, s := range m.suggestions {
		if m.selected[i] && !s.Skipped {
			chosen = append(chosen, s)
		}
	}

	return chosen
}

// setDecks loads the deck picker from a Decks result.
func (m *Model) setDecks(decks []flashcard.DeckName) {
	items := make([]list.Item, 0, len(decks))
	for _, d := range decks {
		items = append(items, listItem{title: string(d), desc: "Anki deck"})
	}

	m.deckList.SetItems(items)
	m.state = stateDeckPick
}

// setSuggestions loads the review screen, preselecting every fillable suggestion.
func (m *Model) setSuggestions(suggestions []deckbuilder.Suggestion) {
	m.suggestions = suggestions

	m.selected = make(map[int]bool, len(suggestions))
	for i, s := range suggestions {
		if !s.Skipped {
			m.selected[i] = true
		}
	}

	m.cursor = 0
	m.state = stateScanList
}

// setSize keeps the embedded components sized to the window.
func (m *Model) setSize(w, h int) {
	m.width, m.height = w, h

	listH := max(h-4, 1)

	m.deckList.SetSize(w, listH)
	m.modeList.SetSize(w, listH)

	iw := w - 8
	if iw > 0 {
		m.input.Width = iw
		m.editFront.Width = iw
	}

	if tw := w - 4; tw > 0 {
		m.editBack.SetWidth(tw)
	}

	m.editBack.SetHeight(6)
}

// View renders the active screen. All colors come from m.styles.
func (m *Model) View() string {
	switch m.state {
	case stateDeckPick:
		return m.viewWithError(m.deckList.View())
	case stateModePick:
		return m.modeList.View()
	case stateAddInput:
		return m.viewAddInput()
	case stateAddLoading:
		return m.styles.Title.Render(" Looking up ") + "\n\n" +
			m.spinner.View() + " " + m.styles.Faint.Render("searching the dictionary…")
	case stateAddPreview:
		return m.viewAddPreview()
	case stateAddEdit:
		return m.viewAddEdit()
	case stateAddSaving:
		return m.styles.Title.Render(" Adding ") + "\n\n" +
			m.spinner.View() + " " + m.styles.Faint.Render("contacting Anki…")
	case stateAddResult:
		return m.viewAddResult()
	case stateScanLoading:
		return m.styles.Title.Render(" Scanning ") + "\n\n" +
			m.spinner.View() + " " + m.styles.Faint.Render("looking for blanks…")
	case stateScanList:
		return m.viewScanList()
	case stateApplyLoading:
		return m.styles.Title.Render(" Applying ") + "\n\n" +
			m.spinner.View() + " " + m.styles.Faint.Render("updating notes…")
	case stateScanSummary:
		return m.viewScanSummary()
	}

	return ""
}

func (m *Model) viewWithError(body string) string {
	if m.lastErr == nil {
		return body
	}

	return body + "\n" + m.styles.Err.Render("error: "+m.lastErr.Error())
}

func (m *Model) viewAddInput() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(" Add a word "))
	b.WriteString("\n")
	b.WriteString(m.styles.Faint.Render("deck: " + string(m.deck)))
	b.WriteString("\n\n")
	b.WriteString(m.styles.ActiveInput.Render(m.input.View()))
	b.WriteString("\n\n")
	b.WriteString(m.hints("enter", "add", "ctrl+e", "manual", "ctrl+u", "clear", "esc", "back", "ctrl+c", "quit"))

	return b.String()
}

// viewAddPreview renders the card that PreviewWord built and asks the user to
// confirm before it is written to Anki.
func (m *Model) viewAddPreview() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(" Preview "))
	b.WriteString("\n")
	b.WriteString(m.styles.Faint.Render("deck: " + string(m.deck)))
	b.WriteString("\n\n")

	front := m.previewNote.Front
	b.WriteString(m.styles.Faint.Render("Front  "))
	b.WriteString(m.styles.Gender(firstArticle(front)).Render(front))
	b.WriteString("\n")

	b.WriteString(m.styles.Faint.Render("Back   "))

	for i, line := range strings.Split(m.previewNote.Back, "\n") {
		if i > 0 {
			b.WriteString("       ")
		}

		b.WriteString(m.styles.Item.Render(line))
		b.WriteString("\n")
	}

	if len(m.previewNote.Tags) > 0 {
		b.WriteString(m.styles.Faint.Render("tags   " + strings.Join(m.previewNote.Tags, ", ")))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.hints("enter", "add", "e", "edit", "esc", "cancel", "ctrl+c", "quit"))

	return b.String()
}

// viewAddEdit renders the Front/Back editor opened from the preview.
func (m *Model) viewAddEdit() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(" Edit "))
	b.WriteString("\n")
	b.WriteString(m.styles.Faint.Render("deck: " + string(m.deck)))
	b.WriteString("\n\n")

	b.WriteString(m.fieldLabel("Front", m.editFocus == 0))
	b.WriteString("\n")
	b.WriteString(m.editFront.View())
	b.WriteString("\n\n")

	b.WriteString(m.fieldLabel("Back", m.editFocus == 1))
	b.WriteString("\n")
	b.WriteString(m.editBack.View())
	b.WriteString("\n\n")

	b.WriteString(m.hints("tab", "switch field", "ctrl+s", "save", "esc", "cancel"))

	return b.String()
}

// fieldLabel renders an edit-field label, accented when the field is focused.
func (m *Model) fieldLabel(name string, active bool) string {
	if active {
		return m.styles.Selected.Render("▌ " + name)
	}

	return m.styles.Faint.Render("  " + name)
}

func (m *Model) viewAddResult() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(" Result "))
	b.WriteString("\n\n")

	if m.lastErr != nil {
		b.WriteString(m.styles.Err.Render("✗ " + m.lastErr.Error()))
	} else {
		b.WriteString(m.styles.OK.Render(fmt.Sprintf("✓ added note %d", m.lastID)))
	}

	b.WriteString("\n\n")
	b.WriteString(m.hints("any key", "add another", "ctrl+c", "quit"))

	return b.String()
}

func (m *Model) viewScanList() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(" Suggestions "))
	b.WriteString("\n\n")

	if len(m.suggestions) == 0 {
		b.WriteString(m.styles.Faint.Render("nothing to fill — every note is complete."))
		b.WriteString("\n\n")
		b.WriteString(m.hints("esc", "back", "ctrl+c", "quit"))

		return b.String()
	}

	for i, s := range m.suggestions {
		cursor := "  "
		if i == m.cursor {
			cursor = m.styles.Selected.Render("▌ ")
		}

		box := "[ ]"
		if s.Skipped {
			box = m.styles.Warn.Render("[-]")
		} else if m.selected[i] {
			box = m.styles.Checkbox.Render("[x]")
		}

		line := m.styles.Gender(articleOf(s)).Render(s.Lemma)

		detail := s.Preview
		if s.Skipped {
			detail = m.styles.Skipped.Render("skipped: " + s.Reason)
		}

		fmt.Fprintf(&b, "%s%s %s  %s\n", cursor, box, line, detail)
	}

	b.WriteString("\n")
	b.WriteString(m.hints("space", "toggle", "a", "apply", "esc", "back", "ctrl+c", "quit"))

	return b.String()
}

func (m *Model) viewScanSummary() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(" Summary "))
	b.WriteString("\n\n")

	if m.lastErr != nil {
		b.WriteString(m.styles.Err.Render("✗ " + m.lastErr.Error()))
	} else {
		b.WriteString(m.styles.OK.Render(fmt.Sprintf("✓ updated %d", m.updated)))
		b.WriteString("   ")
		b.WriteString(m.styles.Warn.Render(fmt.Sprintf("skipped %d", m.skipped)))
	}

	b.WriteString("\n\n")
	b.WriteString(m.hints("any key", "back to menu", "ctrl+c", "quit"))

	return b.String()
}

// hints renders alternating key/label pairs in the footer.
func (m *Model) hints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, m.styles.KeyHint.Render(pairs[i])+" "+m.styles.Faint.Render(pairs[i+1]))
	}

	return strings.Join(parts, m.styles.Faint.Render(" · "))
}

// articleOf extracts a German article ("der"/"die"/"das") from a suggestion's
// fields or preview so the lemma can be tinted by gender.
func articleOf(s deckbuilder.Suggestion) string {
	for _, v := range s.Fields {
		if a := firstArticle(v); a != "" {
			return a
		}
	}

	return firstArticle(s.Preview)
}

func firstArticle(text string) string {
	for w := range strings.FieldsSeq(strings.ToLower(text)) {
		switch w {
		case "der", "die", "das":
			return w
		}
	}

	return ""
}
