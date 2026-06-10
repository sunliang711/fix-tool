package shell

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUIModelSubmitsInputAndKeepsPromptAtBottom(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 40
	model.height = 8

	for _, value := range []rune("help") {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		model = next.(tuiModel)
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(tuiModel)

	line, ok, err := reader.ReadLine(context.Background(), "")
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if !ok || line != "help" {
		t.Fatalf("ReadLine() = %q, %v, want help, true", line, ok)
	}
	rawView := model.View()
	if !strings.Contains(rawView, tuiPromptColor+"fix-tool> "+tuiColorReset+"|") {
		t.Fatalf("view = %q, want colored input prompt cursor", rawView)
	}
	view := stripTUIANSICodes(rawView)
	if !strings.Contains(view, "fix-tool> help") {
		t.Fatalf("view = %q, want submitted command in logs", view)
	}
	if !strings.Contains(view, "fix-tool> |") {
		t.Fatalf("view = %q, want fixed prompt input cursor", view)
	}
}

func TestTUIModelRendersHelpInsideInputPane(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 160
	model.height = 14

	lines := strings.Split(stripTUIANSICodes(model.View()), "\n")
	helpLines := 0
	for index, line := range lines {
		if strings.Contains(line, "Tab focus") {
			helpLines++
		}
		if !strings.Contains(line, "fix-tool> |") {
			continue
		}
		if !strings.Contains(line, "Tab focus") || !strings.Contains(line, "Enter run") {
			t.Fatalf("input line = %q, want inline help", line)
		}
		if index+1 >= len(lines) || !strings.Contains(lines[index+1], "└") {
			t.Fatalf("view lines = %q, want help inside input pane", lines)
		}
		if helpLines != 1 {
			t.Fatalf("view lines = %q, want one inline help line", lines)
		}
		return
	}
	t.Fatalf("view lines = %q, want input line", lines)
}

func TestTUIModelRendersSectionTitlesWithBorders(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 80
	model.height = 12

	view := stripTUIANSICodes(model.View())
	for _, want := range []string{"┌ Logs ", "┌ Heartbeat ", "┌ Command* "} {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, want section title %q", view, want)
		}
	}
	for _, want := range []string{"│", "┌", "┐", "└", "┘"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, want border %q", view, want)
		}
	}
}

func TestTUIModelRendersStoppedStreamStatus(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 100
	model.height = 14

	view := stripTUIANSICodes(model.View())
	want := `Stream: stopped sent=0 ok=0 failed=0 last_error=""`
	if !strings.Contains(view, want) {
		t.Fatalf("view = %q, want %q", view, want)
	}
}

func TestTUIModelRendersRunningStreamStatus(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 120
	model.height = 15

	next, _ := model.Update(tuiStreamStatusMsg(orderStreamStatus{
		Running:   true,
		Sent:      42,
		Succeeded: 42,
		Failed:    0,
		LastError: "",
	}))
	model = next.(tuiModel)

	view := stripTUIANSICodes(model.View())
	want := `Stream: running sent=42 ok=42 failed=0 last_error=""`
	if !strings.Contains(view, want) {
		t.Fatalf("view = %q, want %q", view, want)
	}
}

func TestTUIModelKeepsBlankLinesBetweenSections(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 80
	model.height = 14

	lines := strings.Split(stripTUIANSICodes(model.View()), "\n")
	assertBlankLineBeforeTitle(t, lines, "┌ Heartbeat ")
	assertBlankLineBeforeTitle(t, lines, "┌ Command")
}

func TestTUIModelAcceptsSpacesInInput(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	for _, value := range []rune("save") {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		model = next.(tuiModel)
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	model = next.(tuiModel)
	for _, value := range []rune("status") {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		model = next.(tuiModel)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(tuiModel)

	line, ok, err := reader.ReadLine(context.Background(), "")
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if !ok || line != "save status" {
		t.Fatalf("ReadLine() = %q, %v, want save status, true", line, ok)
	}
}

func TestTUIModelCommandEscSwitchesToNormalWithoutQuit(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 80
	model.height = 14

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)

	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatalf("Esc command = tea.QuitMsg, want no quit")
		}
	}
	if model.commandMode != tuiCommandModeNormal {
		t.Fatalf("commandMode = %v, want normal", model.commandMode)
	}
	if !strings.Contains(stripTUIANSICodes(model.View()), "-- NORMAL --") {
		t.Fatalf("view = %q, want normal mode label", stripTUIANSICodes(model.View()))
	}
}

func TestTUIModelCommandNormalIBackToInsert(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	model = next.(tuiModel)

	if model.commandMode != tuiCommandModeInsert {
		t.Fatalf("commandMode = %v, want insert", model.commandMode)
	}
}

func TestTUIModelCommandNormalMovesCursor(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	for _, value := range []rune("alpha beta gamma") {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		model = next.(tuiModel)
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	if model.cursor != 15 {
		t.Fatalf("cursor = %d, want 15 after Esc", model.cursor)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = next.(tuiModel)
	if model.cursor != 14 {
		t.Fatalf("cursor = %d, want 14 after h", model.cursor)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = next.(tuiModel)
	if model.cursor != 15 {
		t.Fatalf("cursor = %d, want 15 after l", model.cursor)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	model = next.(tuiModel)
	if model.cursor != 11 {
		t.Fatalf("cursor = %d, want 11 after b", model.cursor)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	model = next.(tuiModel)
	if model.cursor != 6 {
		t.Fatalf("cursor = %d, want 6 after second b", model.cursor)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	model = next.(tuiModel)
	if model.cursor != 11 {
		t.Fatalf("cursor = %d, want 11 after w", model.cursor)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = next.(tuiModel)
	if model.cursor != 15 {
		t.Fatalf("cursor = %d, want 15 after e", model.cursor)
	}
}

func TestTUIModelCommandNormalCursorDoesNotInsertExtraCell(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 120
	model.height = 14

	for _, value := range []rune("order stream start") {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		model = next.(tuiModel)
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	for i := 0; i < 10; i++ {
		next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
		model = next.(tuiModel)
	}

	view := stripTUIANSICodes(model.View())
	if strings.Contains(view, "█") {
		t.Fatalf("view = %q, want no inserted block cursor in normal mode", view)
	}
	if !strings.Contains(view, "fix-tool> order stream start") {
		t.Fatalf("view = %q, want original input without shifted character", view)
	}
}

func TestTUIModelCommandNormalNextWordFromSeparator(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	for _, value := range []rune("alpha beta gamma") {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		model = next.(tuiModel)
	}
	model.cursor = 5
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	model = next.(tuiModel)

	if model.cursor != 6 {
		t.Fatalf("cursor = %d, want 6 after w from separator", model.cursor)
	}
}

func TestTUIModelCommandNormalEmptyInputKeysAreSafe(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	for _, key := range []rune{'h', 'l', 'w', 'b', 'e', 'd'} {
		next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		model = next.(tuiModel)
	}

	if len(model.input) != 0 || model.cursor != 0 {
		t.Fatalf("input=%q cursor=%d, want empty input cursor 0", string(model.input), model.cursor)
	}
}

func TestTUIModelCommandNormalDeletesCursorChar(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	for _, value := range []rune("abcd") {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		model = next.(tuiModel)
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	model = next.(tuiModel)

	if got := string(model.input); got != "abd" {
		t.Fatalf("input = %q, want abd", got)
	}
	if model.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", model.cursor)
	}
}

func TestTUIModelCommandNormalEnterSubmits(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	for _, value := range []rune("help") {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		model = next.(tuiModel)
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(tuiModel)

	line, ok, err := reader.ReadLine(context.Background(), "")
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if !ok || line != "help" {
		t.Fatalf("ReadLine() = %q, %v, want help, true", line, ok)
	}
}

func TestTUIModelCommandCtrlCStillQuits(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatalf("Ctrl+C command = nil, want quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Ctrl+C command = %T, want tea.QuitMsg", cmd())
	}
}

func TestTUIModelCommandNormalCtrlDClosesInput(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	model = next.(tuiModel)

	line, ok, err := reader.ReadLine(context.Background(), "")
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if ok || line != "" {
		t.Fatalf("ReadLine() = %q, %v, want empty line, false", line, ok)
	}
}

func TestTUIModelReadOnlyEscStillQuits(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatalf("Esc command = nil, want quit outside command pane")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Esc command = %T, want tea.QuitMsg", cmd())
	}
}

func TestTUIModelReadOnlyVisualEscExitsVisualOnly(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	model = next.(tuiModel)
	if model.mode != tuiModeVisual {
		t.Fatalf("mode = %v, want visual", model.mode)
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(tuiModel)

	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatalf("Esc command = tea.QuitMsg, want no quit from visual")
		}
	}
	if model.mode != tuiModeNormal {
		t.Fatalf("mode = %v, want normal", model.mode)
	}
}

func TestTUIModelCtrlDClosesInput(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	model = next.(tuiModel)

	line, ok, err := reader.ReadLine(context.Background(), "")
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if ok || line != "" {
		t.Fatalf("ReadLine() = %q, %v, want empty line, false", line, ok)
	}
}

func TestTUIRunnerStateKeepsResultAfterDoneMessage(t *testing.T) {
	done := newTUIRunnerState()
	want := errors.New("runner done")
	done.finish(want)

	message := waitTUIDone(done)()
	got, ok := message.(tuiDoneMsg)
	if !ok {
		t.Fatalf("message = %T, want tuiDoneMsg", message)
	}
	if !errors.Is(got.err, want) {
		t.Fatalf("message error = %v, want %v", got.err, want)
	}
	if !errors.Is(done.err(), want) {
		t.Fatalf("stored error = %v, want %v", done.err(), want)
	}
}

func TestTUIModelAppendsAndScrollsOutput(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 120
	model.height = 15

	next, _ := model.Update(tuiOutputMsg("one\ntwo\nthree\nfour\n"))
	model = next.(tuiModel)
	view := stripTUIANSICodes(model.View())
	if strings.Contains(view, "one") {
		t.Fatalf("view = %q, want oldest line out of viewport", view)
	}
	if !strings.Contains(view, "two") || !strings.Contains(view, "four") {
		t.Fatalf("view = %q, want latest log lines", view)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model = next.(tuiModel)
	view = stripTUIANSICodes(model.View())
	if !strings.Contains(view, "one") {
		t.Fatalf("view = %q, want scrolled log line", view)
	}
}

func TestTUIModelFocusesLatestLogLineByDefault(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 80
	model.height = 18

	next, _ := model.Update(tuiOutputMsg("alpha\nbeta\ngamma\n"))
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)

	if model.logsCursor != 2 {
		t.Fatalf("logsCursor = %d, want latest line index 2", model.logsCursor)
	}
	if model.scroll != 0 {
		t.Fatalf("scroll = %d, want bottom scroll", model.scroll)
	}
	if model.logsManual {
		t.Fatalf("logsManual = true, want default latest tracking")
	}
	view := model.View()
	if !strings.Contains(view, tuiSelectionColor+"gamma") {
		t.Fatalf("view = %q, want latest log line selected", view)
	}
}

func TestTUIModelKeepsManualLogCursorWhenRefocusing(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 80
	model.height = 18

	next, _ := model.Update(tuiOutputMsg("alpha\nbeta\ngamma\n"))
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)

	if model.logsCursor != 1 {
		t.Fatalf("logsCursor = %d, want preserved line index 1", model.logsCursor)
	}
	if !model.logsManual {
		t.Fatalf("logsManual = false, want manual history state")
	}
	view := model.View()
	if !strings.Contains(view, tuiSelectionColor+"beta") {
		t.Fatalf("view = %q, want manual log line selected", view)
	}
}

func TestTUIModelNewLogsFollowUnlessFocusedOnHistory(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 80
	model.height = 18

	next, _ := model.Update(tuiOutputMsg("alpha\nbeta\ngamma\n"))
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = next.(tuiModel)
	next, _ = model.Update(tuiOutputMsg("delta\n"))
	model = next.(tuiModel)

	if model.logsCursor != 1 {
		t.Fatalf("logsCursor = %d, want focused history line index 1", model.logsCursor)
	}
	if !strings.Contains(model.View(), tuiSelectionColor+"beta") {
		t.Fatalf("view = %q, want focused history line selected", model.View())
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tuiOutputMsg("epsilon\n"))
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)

	if model.logsCursor != 4 {
		t.Fatalf("logsCursor = %d, want latest line index 4", model.logsCursor)
	}
	if model.logsManual {
		t.Fatalf("logsManual = true, want latest tracking after background output")
	}
	if !strings.Contains(model.View(), tuiSelectionColor+"epsilon") {
		t.Fatalf("view = %q, want newest log line selected", model.View())
	}
}

func TestTUIModelLogEdgeShortcuts(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 80
	model.height = 18

	next, _ := model.Update(tuiOutputMsg("alpha\nbeta\ngamma\n"))
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	model = next.(tuiModel)

	if model.logsCursor != 0 {
		t.Fatalf("logsCursor = %d, want first line index 0", model.logsCursor)
	}
	if !model.logsManual {
		t.Fatalf("logsManual = false, want history state after g")
	}
	if !strings.Contains(model.View(), tuiSelectionColor+"alpha") {
		t.Fatalf("view = %q, want first log line selected", model.View())
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	model = next.(tuiModel)

	if model.logsCursor != 2 {
		t.Fatalf("logsCursor = %d, want latest line index 2", model.logsCursor)
	}
	if model.logsManual {
		t.Fatalf("logsManual = true, want latest tracking after G")
	}
	if !strings.Contains(model.View(), tuiSelectionColor+"gamma") {
		t.Fatalf("view = %q, want latest log line selected", model.View())
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyHome})
	model = next.(tuiModel)

	if model.logsCursor != 0 {
		t.Fatalf("logsCursor = %d, want first line index 0 after Home", model.logsCursor)
	}
	if !model.logsManual {
		t.Fatalf("logsManual = false, want history state after Home")
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = next.(tuiModel)

	if model.logsCursor != 2 {
		t.Fatalf("logsCursor = %d, want latest line index 2 after End", model.logsCursor)
	}
	if model.logsManual {
		t.Fatalf("logsManual = true, want latest tracking after End")
	}
}

func TestTUIModelRoutesHeartbeatRawToPanel(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 160
	model.height = 12

	message := strings.Join([]string{
		"2026-05-19T21:30:01+08:00 INF -> Heartbeat(0) direction=out source=quickfix",
		"2026-05-19T21:30:01+08:00 DBG -> Heartbeat(0) direction=out source=quickfix",
		"  raw_message:",
		"    8=FIX.4.4|9=59|35=0|34=6|49=CLIENT01|10=034|",
		"  pretty_message:",
		"    BeginString  8 = FIX.4.4",
		"2026-05-19T21:30:02+08:00 INF business log",
		"",
	}, "\n")

	next, _ := model.Update(tuiOutputMsg(message))
	model = next.(tuiModel)
	rawView := model.View()
	if !strings.Contains(rawView, tuiHeartbeatColor+"34=6"+tuiColorReset) {
		t.Fatalf("view = %q, want colored heartbeat MsgSeqNum", rawView)
	}
	view := stripTUIANSICodes(rawView)
	for _, unwanted := range []string{"Heartbeat(0)", "pretty_message", "8(BeginString)"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("view = %q, want no %q in main log or heartbeat panel", view, unwanted)
		}
	}
	for _, want := range []string{
		"business log",
		"Heartbeat",
		"OUT raw_message=8=FIX.4.4|9=59|35=0|34=6|49=CLIENT01|10=034|",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, want %q", view, want)
		}
	}
}

func TestTUIModelRoutesJSONHeartbeatRawToPanel(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 160
	model.height = 12

	message := strings.Join([]string{
		`{"level":"debug","direction":"in","msg_type":"Heartbeat","message":"<- Heartbeat(0)","raw_message":"8=FIX.4.4|9=59|35=0|34=7|49=BROKER01|10=035|","pretty_message":"8(BeginString)=FIX.4.4|35(MsgType:Heartbeat)=0|"}`,
		`{"level":"info","message":"business log"}`,
		"",
	}, "\n")

	next, _ := model.Update(tuiOutputMsg(message))
	model = next.(tuiModel)
	view := stripTUIANSICodes(model.View())
	for _, unwanted := range []string{"pretty_message", "8(BeginString)", "Heartbeat(0)"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("view = %q, want no %q in main log or heartbeat panel", view, unwanted)
		}
	}
	for _, want := range []string{
		"business log",
		"IN  raw_message=8=FIX.4.4|9=59|35=0|34=7|49=BROKER01|10=035|",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, want %q", view, want)
		}
	}
}

func TestTUIModelRoutesFormattedHeartbeatBlockToPanel(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 180
	model.height = 30

	message := strings.Join([]string{
		"===> Outgoing FIX Msg(Heartbeat): ===>",
		"Time:        2026-05-20 03:14:37.024773 +0000 UTC",
		"Session:     FIX.4.4:CLIENT01->BROKER01",
		"Content:",
		"  Raw:",
		"    8=FIX.4.4|9=59|35=0|34=9|49=CLIENT01|10=034|",
		"  Pretty:",
		"    MsgType:Heartbeat 35 = 0",
		"2026-05-20T03:14:38Z INF business log",
		"",
	}, "\n")

	next, _ := model.Update(tuiOutputMsg(message))
	model = next.(tuiModel)
	rawView := model.View()
	if !strings.Contains(rawView, tuiHeartbeatColor+"34=9"+tuiColorReset) {
		t.Fatalf("view = %q, want colored heartbeat MsgSeqNum", rawView)
	}
	view := stripTUIANSICodes(rawView)
	for _, unwanted := range []string{"Outgoing FIX Msg", "MsgType:Heartbeat", "35 = 0"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("view = %q, want formatted heartbeat block hidden from logs", view)
		}
	}
	for _, want := range []string{
		"business log",
		"OUT raw_message=8=FIX.4.4|9=59|35=0|34=9|49=CLIENT01|10=034|",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, want %q", view, want)
		}
	}
}

func TestTUIModelKeepsFormattedNonHeartbeatBlockInLogs(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 180
	model.height = 30

	message := strings.Join([]string{
		"<=== Incoming FIX Msg(Logon): <===",
		"Time:        2026-05-20 03:14:37.024773 +0000 UTC",
		"Session:     FIX.4.4:CLIENT01->BROKER01",
		"Content:",
		"  Raw:",
		"    8=FIX.4.4|9=59|35=A|34=9|49=BROKER01|10=034|",
		"  Pretty:",
		"    MsgType:Logon 35 = A",
		"",
	}, "\n")

	next, _ := model.Update(tuiOutputMsg(message))
	model = next.(tuiModel)
	view := stripTUIANSICodes(model.View())
	for _, want := range []string{
		"Incoming FIX Msg",
		"35=A|",
		"MsgType:Logon",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, want %q", view, want)
		}
	}
	if strings.Contains(view, "IN  raw_message=8=FIX.4.4|9=59|35=A|") {
		t.Fatalf("view = %q, want non-heartbeat block outside heartbeat panel", view)
	}
}

func TestTUIModelStartsWithMouseWheelDisabled(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 120
	model.height = 15

	next, _ := model.Update(tuiOutputMsg("one\ntwo\nthree\nfour\nfive\n"))
	model = next.(tuiModel)
	next, _ = model.Update(tea.MouseMsg{Type: tea.MouseWheelUp})
	model = next.(tuiModel)

	view := stripTUIANSICodes(model.View())
	if strings.Contains(view, "one") {
		t.Fatalf("view = %q, want mouse wheel ignored while disabled", view)
	}
	if !strings.Contains(view, "F2 wheel:off") {
		t.Fatalf("view = %q, want mouse wheel state in help", view)
	}
}

func TestTUIModelMouseWheelScrollsOutput(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 120
	model.height = 15

	next, _ := model.Update(tuiOutputMsg("one\ntwo\nthree\nfour\nfive\n"))
	model = next.(tuiModel)
	view := stripTUIANSICodes(model.View())
	if strings.Contains(view, "one") {
		t.Fatalf("view = %q, want oldest line out of viewport", view)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyF2})
	model = next.(tuiModel)
	view = stripTUIANSICodes(model.View())
	if !strings.Contains(view, "F2 wheel:on") {
		t.Fatalf("view = %q, want enabled mouse wheel state in help", view)
	}

	next, _ = model.Update(tea.MouseMsg{Type: tea.MouseWheelUp})
	model = next.(tuiModel)
	view = stripTUIANSICodes(model.View())
	if !strings.Contains(view, "one") {
		t.Fatalf("view = %q, want mouse wheel to scroll up", view)
	}

	next, _ = model.Update(tea.MouseMsg{Type: tea.MouseWheelDown})
	model = next.(tuiModel)
	view = stripTUIANSICodes(model.View())
	if strings.Contains(view, "one") || !strings.Contains(view, "five") {
		t.Fatalf("view = %q, want mouse wheel to scroll down", view)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyF2})
	model = next.(tuiModel)
	view = stripTUIANSICodes(model.View())
	if !strings.Contains(view, "F2 wheel:off") {
		t.Fatalf("view = %q, want disabled mouse wheel state in help", view)
	}
}

func TestTUIModelCopiesVisualSelectionFromLogs(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	clipboard := &fakeTUIClipboard{}
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.clipboard = clipboard
	model.width = 80
	model.height = 18

	next, _ := model.Update(tuiOutputMsg("alpha\nbeta\ngamma\n"))
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	model = next.(tuiModel)
	view := model.View()
	if !strings.Contains(view, tuiSelectionColor+"alpha") {
		t.Fatalf("view = %q, want selected log line", view)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = next.(tuiModel)

	if clipboard.text != "alpha\nbeta" {
		t.Fatalf("clipboard = %q, want selected log lines", clipboard.text)
	}
	if strings.Contains(clipboard.text, "│") || strings.Contains(clipboard.text, "┌") {
		t.Fatalf("clipboard = %q, want content without border", clipboard.text)
	}
	if model.mode != tuiModeNormal {
		t.Fatalf("mode = %v, want normal after yank", model.mode)
	}
	if !strings.Contains(stripTUIANSICodes(model.View()), "copied:2") {
		t.Fatalf("view = %q, want copy status", stripTUIANSICodes(model.View()))
	}
}

func TestTUIModelCopiesHeartbeatPaneLine(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	clipboard := &fakeTUIClipboard{}
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.clipboard = clipboard
	model.width = 100
	model.height = 18

	next, _ := model.Update(tuiOutputMsg(strings.Join([]string{
		"===> Outgoing FIX Msg(Heartbeat): ===>",
		"Time:        2026-05-20 03:14:37.024773 +0000 UTC",
		"Session:     FIX.4.4:CLIENT01->BROKER01",
		"Content:",
		"  Raw:",
		"    8=FIX.4.4|9=59|35=0|34=9|49=CLIENT01|10=034|",
		"  Pretty:",
		"    MsgType:Heartbeat 35 = 0",
		"",
	}, "\n")))
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = next.(tuiModel)

	want := "OUT raw_message=8=FIX.4.4|9=59|35=0|34=9|49=CLIENT01|10=034|"
	if clipboard.text != want {
		t.Fatalf("clipboard = %q, want %q", clipboard.text, want)
	}
	if strings.Contains(clipboard.text, "│") || strings.Contains(clipboard.text, "┌") {
		t.Fatalf("clipboard = %q, want content without border", clipboard.text)
	}
}

func TestTUIModelSelectedHeartbeatLineUsesFullLineSelectionColor(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 100
	model.height = 18

	next, _ := model.Update(tuiOutputMsg(strings.Join([]string{
		"===> Outgoing FIX Msg(Heartbeat): ===>",
		"Time:        2026-05-20 03:14:37.024773 +0000 UTC",
		"Session:     FIX.4.4:CLIENT01->BROKER01",
		"Content:",
		"  Raw:",
		"    8=FIX.4.4|9=59|35=0|34=9|49=CLIENT01|10=034|",
		"  Pretty:",
		"    MsgType:Heartbeat 35 = 0",
		"",
	}, "\n")))
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(tuiModel)

	view := model.View()
	selected := tuiSelectionColor + "OUT raw_message=8=FIX.4.4|9=59|35=0|34=9|49=CLIENT01|10=034|"
	if !strings.Contains(view, selected) {
		t.Fatalf("view = %q, want heartbeat line selected from start", view)
	}
	if strings.Contains(view, tuiHeartbeatColor+"34=9"+tuiColorReset) {
		t.Fatalf("view = %q, want no nested heartbeat color inside selected line", view)
	}
}

type fakeTUIClipboard struct {
	text string
}

func (c *fakeTUIClipboard) WriteText(text string) error {
	c.text = text
	return nil
}

func stripTUIANSICodes(value string) string {
	pattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return pattern.ReplaceAllString(value, "")
}

func assertBlankLineBeforeTitle(t *testing.T, lines []string, title string) {
	t.Helper()
	for index, line := range lines {
		if !strings.HasPrefix(line, title) {
			continue
		}
		if index == 0 || lines[index-1] != "" {
			t.Fatalf("view lines = %q, want blank line before %q", lines, title)
		}
		return
	}
	t.Fatalf("view lines = %q, want title %q", lines, title)
}
