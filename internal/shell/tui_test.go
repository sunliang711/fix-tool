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
	if !strings.Contains(rawView, tuiPromptColor+"fix-tool> "+tuiColorReset+"█") {
		t.Fatalf("view = %q, want colored input prompt", rawView)
	}
	view := stripTUIANSICodes(rawView)
	if !strings.Contains(view, "fix-tool> help") {
		t.Fatalf("view = %q, want submitted command in logs", view)
	}
	if !strings.Contains(view, "fix-tool> █") {
		t.Fatalf("view = %q, want fixed prompt input", view)
	}
}

func TestTUIModelKeepsTwoBlankLinesBeforeHelp(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 120
	model.height = 14

	lines := strings.Split(stripTUIANSICodes(model.View()), "\n")
	for index, line := range lines {
		if line != "fix-tool> █" {
			continue
		}
		if index+3 >= len(lines) {
			t.Fatalf("view lines = %q, want help after input gap", lines)
		}
		if lines[index+1] != "" || lines[index+2] != "" {
			t.Fatalf("view lines = %q, want two blank lines before help", lines)
		}
		if !strings.HasPrefix(lines[index+3], "Enter run") {
			t.Fatalf("view lines = %q, want help after two blank lines", lines)
		}
		return
	}
	t.Fatalf("view lines = %q, want input line", lines)
}

func TestTUIModelRendersSectionTitlesWithoutSideBorders(t *testing.T) {
	reader := NewTUILineReader()
	output := NewTUIOutputWriter()
	done := newTUIRunnerState()
	model := newTUIModel("fix-tool> ", reader, output, done)
	model.width = 80
	model.height = 12

	view := stripTUIANSICodes(model.View())
	for _, want := range []string{"── Logs ", "── Heartbeat ", "── Command "} {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, want section title %q", view, want)
		}
	}
	for _, unwanted := range []string{"│", "┌", "┐", "└", "┘"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("view = %q, want no side border %q", view, unwanted)
		}
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
	assertBlankLineBeforeTitle(t, lines, "── Heartbeat ")
	assertBlankLineBeforeTitle(t, lines, "── Command ")
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
	model.height = 14

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
	model.height = 20

	message := strings.Join([]string{
		"===> Outgoing FIX Msg: ===>",
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
	model.height = 20

	message := strings.Join([]string{
		"<=== Incoming FIX Msg: <===",
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
