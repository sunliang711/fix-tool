package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultTUIWidth     = 80
	defaultTUIHeight    = 24
	tuiLogsTitleHeight  = 1
	tuiSectionGapHeight = 1
	tuiHeartbeatHeight  = 3
	tuiHelpGapHeight    = 2
	tuiInputHeight      = 3 + tuiHelpGapHeight
	maxTUILogLines      = 10000
	tuiShutdownTimeout  = 5 * time.Second
	tuiOutputBufferSize = 4096
	tuiMouseScrollRows  = 3
	tuiPromptColor      = "\x1b[1;36m"
	tuiHeartbeatColor   = "\x1b[1;33m"
	tuiSelectionColor   = "\x1b[7m"
	tuiColorReset       = "\x1b[0m"
)

type TUILineReader struct {
	lines  chan string
	closed chan struct{}
	once   sync.Once
}

func NewTUILineReader() *TUILineReader {
	return &TUILineReader{
		lines:  make(chan string, 64),
		closed: make(chan struct{}),
	}
}

func (r *TUILineReader) Submit(line string) bool {
	select {
	case <-r.closed:
		return false
	case r.lines <- line:
		return true
	default:
		return false
	}
}

func (r *TUILineReader) ReadLine(ctx context.Context, _ string) (string, bool, error) {
	select {
	case <-r.closed:
		return "", false, nil
	case line := <-r.lines:
		return line, true, nil
	case <-ctx.Done():
		return "", false, ctx.Err()
	}
}

func (r *TUILineReader) Close() error {
	r.once.Do(func() {
		close(r.closed)
	})
	return nil
}

type TUIOutputWriter struct {
	messages chan string
	closed   chan struct{}
	once     sync.Once
}

func NewTUIOutputWriter() *TUIOutputWriter {
	return &TUIOutputWriter{
		messages: make(chan string, tuiOutputBufferSize),
		closed:   make(chan struct{}),
	}
}

func (w *TUIOutputWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	message := string(append([]byte(nil), data...))
	select {
	case <-w.closed:
		return len(data), nil
	case w.messages <- message:
		return len(data), nil
	}
}

func (w *TUIOutputWriter) Close() {
	w.once.Do(func() {
		close(w.closed)
	})
}

type TUIOptions struct {
	In         io.Reader
	Out        io.Writer
	Prompt     string
	Runner     *Runner
	LineReader *TUILineReader
	Output     *TUIOutputWriter
	Clipboard  TUIClipboard
}

type TUIClipboard interface {
	WriteText(string) error
}

type systemTUIClipboard struct {
}

func (systemTUIClipboard) WriteText(text string) error {
	command := exec.Command("pbcopy")
	command.Stdin = strings.NewReader(text)
	return command.Run()
}

func RunTUI(ctx context.Context, options TUIOptions) error {
	if options.Runner == nil {
		return fmt.Errorf("tui runner is required")
	}
	if options.LineReader == nil {
		return fmt.Errorf("tui line reader is required")
	}
	if options.Output == nil {
		return fmt.Errorf("tui output writer is required")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	runnerState := newTUIRunnerState()
	go func() {
		runnerState.finish(options.Runner.Run(ctx))
	}()

	model := newTUIModel(options.Prompt, options.LineReader, options.Output, runnerState)
	if options.Clipboard != nil {
		model.clipboard = options.Clipboard
	}
	program := tea.NewProgram(model, tea.WithInput(options.In), tea.WithOutput(options.Out), tea.WithAltScreen())
	_, programErr := program.Run()
	options.Output.Close()
	_ = options.LineReader.Close()
	cancel()

	var runnerErr error
	select {
	case <-runnerState.done:
		runnerErr = runnerState.err()
	case <-time.After(tuiShutdownTimeout):
		runnerErr = fmt.Errorf("tui runner shutdown timeout")
	}
	if programErr != nil {
		return programErr
	}
	if runnerErr != nil && !errors.Is(runnerErr, context.Canceled) {
		return runnerErr
	}
	return nil
}

type tuiOutputMsg string

type tuiDoneMsg struct {
	err error
}

type tuiHeartbeatState struct {
	outbound string
	inbound  string
	unknown  string
}

type tuiPane int

const (
	tuiPaneLogs tuiPane = iota
	tuiPaneHeartbeat
	tuiPaneCommand
)

type tuiMode int

const (
	tuiModeNormal tuiMode = iota
	tuiModeVisual
)

type tuiRunnerState struct {
	done chan struct{}
	mu   sync.Mutex
	errv error
}

func newTUIRunnerState() *tuiRunnerState {
	return &tuiRunnerState{done: make(chan struct{})}
}

func (s *tuiRunnerState) finish(err error) {
	s.mu.Lock()
	s.errv = err
	s.mu.Unlock()
	close(s.done)
}

func (s *tuiRunnerState) err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errv
}

type tuiModel struct {
	prompt     string
	reader     *TUILineReader
	output     *TUIOutputWriter
	done       *tuiRunnerState
	width      int
	height     int
	logs       []string
	pending    string
	input      []rune
	cursor     int
	history    []string
	historyPos int
	scroll     int
	mouse      bool
	focus      tuiPane
	mode       tuiMode
	logsCursor int
	hbCursor   int
	visualFrom int
	clipboard  TUIClipboard
	copyStatus string
	heartbeat  tuiHeartbeatState
	hbBlock    string
	hbDir      string
	fixBlock   []string
	fixDir     string
	fixHB      bool
	fixDecided bool
}

func newTUIModel(prompt string, reader *TUILineReader, output *TUIOutputWriter, done *tuiRunnerState) tuiModel {
	return tuiModel{
		prompt:     prompt,
		reader:     reader,
		output:     output,
		done:       done,
		width:      defaultTUIWidth,
		height:     defaultTUIHeight,
		focus:      tuiPaneCommand,
		clipboard:  systemTUIClipboard{},
		historyPos: -1,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(waitTUIOutput(m.output), waitTUIDone(m.done))
}

func (m tuiModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tuiOutputMsg:
		m.appendOutput(string(msg))
		return m, waitTUIOutput(m.output)
	case tuiDoneMsg:
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.appendLogLine("Error: " + msg.err.Error())
		}
		return m, tea.Quit
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg), nil
	}
	return m, nil
}

func (m tuiModel) View() string {
	width := m.width
	if width <= 0 {
		width = defaultTUIWidth
	}
	height := m.height
	if height < tuiMinimumHeight() {
		height = defaultTUIHeight
	}
	contentWidth := paneContentWidth(width)
	logHeight := m.logHeight()
	rows := m.visibleLogRows(contentWidth)
	scroll := clampValue(m.scroll, 0, maxScroll(len(rows), logHeight))
	bottom := len(rows) - scroll
	if bottom < 0 {
		bottom = 0
	}
	if bottom > len(rows) {
		bottom = len(rows)
	}
	start := bottom - logHeight
	if start < 0 {
		start = 0
	}
	visible := append([]string(nil), rows[start:bottom]...)
	logsContent := make([]string, 0, len(visible))
	for len(logsContent)+len(visible) < logHeight {
		logsContent = append(logsContent, strings.Repeat(" ", contentWidth))
	}
	for i, line := range visible {
		absolute := start + i
		logsContent = append(logsContent, m.renderSelectableLine(tuiPaneLogs, absolute, line, contentWidth))
	}
	heartbeatRows := m.heartbeatRows()
	heartbeatContent := make([]string, 0, len(heartbeatRows))
	for i, line := range heartbeatRows {
		if !m.isSelectedLine(tuiPaneHeartbeat, i) {
			line = colorTUIHeartbeatSeqNum(line)
		}
		heartbeatContent = append(heartbeatContent, m.renderSelectableLine(tuiPaneHeartbeat, i, line, contentWidth))
	}
	input := m.renderInput(contentWidth)
	help := fitRunes("Tab focus  v visual  j/k move  y copy  Enter run  PgUp/PgDown logs  F2 "+m.mouseLabel()+"  Ctrl+C/Ctrl+D exit"+m.copyStatusSuffix(), width)
	lines := renderPane("Logs", m.focus == tuiPaneLogs, logsContent, width)
	lines = append(lines, "")
	lines = append(lines, renderPane("Heartbeat", m.focus == tuiPaneHeartbeat, heartbeatContent, width)...)
	lines = append(lines, "")
	lines = append(lines, renderPane("Command", m.focus == tuiPaneCommand, []string{input}, width)...)
	for range tuiHelpGapHeight {
		lines = append(lines, "")
	}
	lines = append(lines, help)
	return strings.Join(lines, "\n")
}

func (m tuiModel) handleKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		if m.mode == tuiModeVisual {
			m.mode = tuiModeNormal
			return m, nil
		}
		return m, tea.Quit
	case tea.KeyTab:
		m.focusNext(1)
		return m, nil
	case tea.KeyShiftTab:
		m.focusNext(-1)
		return m, nil
	}
	if m.focus != tuiPaneCommand {
		return m.handleReadOnlyKey(message)
	}
	switch message.Type {
	case tea.KeyCtrlD:
		_ = m.reader.Close()
	case tea.KeyCtrlL:
		m.logs = nil
		m.pending = ""
		m.scroll = 0
	case tea.KeyEnter:
		m.submitInput()
	case tea.KeyBackspace, tea.KeyCtrlH:
		m.backspace()
	case tea.KeyDelete:
		m.delete()
	case tea.KeyLeft:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyRight:
		if m.cursor < len(m.input) {
			m.cursor++
		}
	case tea.KeyHome, tea.KeyCtrlA:
		m.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		m.cursor = len(m.input)
	case tea.KeyUp:
		m.historyUp()
	case tea.KeyDown:
		m.historyDown()
	case tea.KeyF2:
		return m.toggleMouse()
	case tea.KeyPgUp:
		m.scrollUp(m.logHeight())
	case tea.KeyPgDown:
		m.scrollDown(m.logHeight())
	case tea.KeyCtrlU:
		m.input = nil
		m.cursor = 0
		m.historyPos = -1
	case tea.KeySpace:
		if len(message.Runes) == 0 {
			m.insertRunes([]rune{' '})
			return m, nil
		}
		m.insertRunes(message.Runes)
	case tea.KeyRunes:
		m.insertRunes(message.Runes)
	}
	return m, nil
}

func (m tuiModel) handleReadOnlyKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyCtrlL:
		m.logs = nil
		m.pending = ""
		m.scroll = 0
		m.logsCursor = 0
	case tea.KeyUp:
		m.moveFocusedCursor(-1)
	case tea.KeyDown:
		m.moveFocusedCursor(1)
	case tea.KeyPgUp:
		m.moveFocusedCursor(-m.logHeight())
	case tea.KeyPgDown:
		m.moveFocusedCursor(m.logHeight())
	case tea.KeyRunes:
		if len(message.Runes) != 1 {
			return m, nil
		}
		switch message.Runes[0] {
		case 'j':
			m.moveFocusedCursor(1)
		case 'k':
			m.moveFocusedCursor(-1)
		case 'v':
			if m.mode == tuiModeVisual {
				m.mode = tuiModeNormal
			} else {
				m.mode = tuiModeVisual
				m.visualFrom = m.focusedCursor()
			}
		case 'y':
			m.copySelection()
		}
	}
	return m, nil
}

func (m *tuiModel) focusNext(delta int) {
	order := []tuiPane{tuiPaneLogs, tuiPaneHeartbeat, tuiPaneCommand}
	index := 0
	for i, pane := range order {
		if pane == m.focus {
			index = i
			break
		}
	}
	index = (index + delta + len(order)) % len(order)
	m.focus = order[index]
	m.mode = tuiModeNormal
	m.copyStatus = ""
	m.clampFocusedCursor()
}

func (m *tuiModel) moveFocusedCursor(delta int) {
	if delta == 0 {
		return
	}
	switch m.focus {
	case tuiPaneLogs:
		rows := m.visibleLogRows(paneContentWidth(m.widthOrDefault()))
		if len(rows) == 0 {
			m.logsCursor = 0
			return
		}
		m.logsCursor = clampValue(m.logsCursor+delta, 0, len(rows)-1)
		m.ensureLogsCursorVisible(len(rows))
	case tuiPaneHeartbeat:
		rows := m.heartbeatRows()
		if len(rows) == 0 {
			m.hbCursor = 0
			return
		}
		m.hbCursor = clampValue(m.hbCursor+delta, 0, len(rows)-1)
	}
}

func (m *tuiModel) clampFocusedCursor() {
	switch m.focus {
	case tuiPaneLogs:
		rows := m.visibleLogRows(paneContentWidth(m.widthOrDefault()))
		if len(rows) == 0 {
			m.logsCursor = 0
			return
		}
		m.logsCursor = clampValue(m.logsCursor, 0, len(rows)-1)
		m.ensureLogsCursorVisible(len(rows))
	case tuiPaneHeartbeat:
		rows := m.heartbeatRows()
		if len(rows) == 0 {
			m.hbCursor = 0
			return
		}
		m.hbCursor = clampValue(m.hbCursor, 0, len(rows)-1)
	}
}

func (m tuiModel) focusedCursor() int {
	switch m.focus {
	case tuiPaneLogs:
		return m.logsCursor
	case tuiPaneHeartbeat:
		return m.hbCursor
	default:
		return 0
	}
}

func (m *tuiModel) ensureLogsCursorVisible(rowCount int) {
	height := m.logHeight()
	if rowCount == 0 || height <= 0 {
		m.scroll = 0
		return
	}
	start, bottom := logVisibleRange(rowCount, height, m.scroll)
	if m.logsCursor < start {
		bottom = m.logsCursor + height
		if bottom > rowCount {
			bottom = rowCount
		}
		m.scroll = rowCount - bottom
	} else if m.logsCursor >= bottom {
		bottom = m.logsCursor + 1
		m.scroll = rowCount - bottom
	}
	m.clampScroll()
}

func (m *tuiModel) copySelection() {
	rows := m.copyRowsForFocusedPane()
	if len(rows) == 0 {
		m.copyStatus = "  copy:none"
		m.mode = tuiModeNormal
		return
	}
	if m.clipboard == nil {
		m.copyStatus = "  copy:unavailable"
		m.mode = tuiModeNormal
		return
	}
	text := strings.Join(rows, "\n")
	if err := m.clipboard.WriteText(text); err != nil {
		m.copyStatus = "  copy:error"
		m.mode = tuiModeNormal
		return
	}
	m.copyStatus = fmt.Sprintf("  copied:%d", len(rows))
	m.mode = tuiModeNormal
}

func (m tuiModel) copyRowsForFocusedPane() []string {
	var rows []string
	cursor := m.focusedCursor()
	switch m.focus {
	case tuiPaneLogs:
		rows = m.visibleLogRows(paneContentWidth(m.widthOrDefault()))
	case tuiPaneHeartbeat:
		rows = m.heartbeatRows()
	default:
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	start, end := cursor, cursor
	if m.mode == tuiModeVisual {
		start, end = orderedRange(m.visualFrom, cursor)
	}
	start = clampValue(start, 0, len(rows)-1)
	end = clampValue(end, 0, len(rows)-1)
	return append([]string(nil), rows[start:end+1]...)
}

func (m tuiModel) handleMouse(message tea.MouseMsg) tuiModel {
	if !m.mouse {
		return m
	}
	switch message.Type {
	case tea.MouseWheelUp:
		m.scrollUp(tuiMouseScrollRows)
	case tea.MouseWheelDown:
		m.scrollDown(tuiMouseScrollRows)
	}
	return m
}

func (m tuiModel) toggleMouse() (tea.Model, tea.Cmd) {
	m.mouse = !m.mouse
	if m.mouse {
		return m, tea.EnableMouseCellMotion
	}
	return m, tea.DisableMouse
}

func (m tuiModel) mouseLabel() string {
	if m.mouse {
		return "wheel:on"
	}
	return "wheel:off"
}

func (m *tuiModel) submitInput() {
	line := strings.TrimSpace(string(m.input))
	if line == "" {
		m.input = nil
		m.cursor = 0
		m.historyPos = -1
		return
	}
	m.appendLogLine(m.prompt + line)
	if !m.reader.Submit(line) {
		m.appendLogLine("Error: command queue is full")
	}
	m.history = append(m.history, line)
	m.historyPos = -1
	m.input = nil
	m.cursor = 0
	m.scroll = 0
}

func (m *tuiModel) insertRunes(value []rune) {
	if len(value) == 0 {
		return
	}
	next := make([]rune, 0, len(m.input)+len(value))
	next = append(next, m.input[:m.cursor]...)
	next = append(next, value...)
	next = append(next, m.input[m.cursor:]...)
	m.input = next
	m.cursor += len(value)
	m.historyPos = -1
}

func (m *tuiModel) backspace() {
	if m.cursor <= 0 {
		return
	}
	m.input = append(m.input[:m.cursor-1], m.input[m.cursor:]...)
	m.cursor--
	m.historyPos = -1
}

func (m *tuiModel) delete() {
	if m.cursor >= len(m.input) {
		return
	}
	m.input = append(m.input[:m.cursor], m.input[m.cursor+1:]...)
	m.historyPos = -1
}

func (m *tuiModel) historyUp() {
	if len(m.history) == 0 {
		return
	}
	if m.historyPos < 0 {
		m.historyPos = len(m.history) - 1
	} else if m.historyPos > 0 {
		m.historyPos--
	}
	m.input = []rune(m.history[m.historyPos])
	m.cursor = len(m.input)
}

func (m *tuiModel) historyDown() {
	if len(m.history) == 0 || m.historyPos < 0 {
		return
	}
	if m.historyPos >= len(m.history)-1 {
		m.historyPos = -1
		m.input = nil
		m.cursor = 0
		return
	}
	m.historyPos++
	m.input = []rune(m.history[m.historyPos])
	m.cursor = len(m.input)
}

func (m *tuiModel) appendOutput(output string) {
	m.pending += strings.ReplaceAll(output, "\r\n", "\n")
	for {
		line, rest, ok := strings.Cut(m.pending, "\n")
		if !ok {
			return
		}
		m.routeLogLine(line)
		m.pending = rest
	}
}

func (m *tuiModel) routeLogLine(line string) {
	if m.consumeFIXMessageBlockLine(line) {
		return
	}
	if m.hbBlock != "" && !strings.HasPrefix(line, " ") {
		m.hbBlock = ""
		m.hbDir = ""
	}
	if m.consumeHeartbeatLine(line) {
		return
	}
	m.appendLogLine(line)
}

func (m *tuiModel) consumeFIXMessageBlockLine(line string) bool {
	if direction, ok := fixMessageBlockDirection(line); ok {
		m.flushFIXMessageBlock()
		m.fixBlock = append(m.fixBlock[:0], line)
		m.fixDir = direction
		m.fixHB = false
		m.fixDecided = false
		return true
	}
	if len(m.fixBlock) == 0 {
		return false
	}
	if !isFIXMessageBlockLine(line) {
		m.flushFIXMessageBlock()
		return false
	}
	m.fixBlock = append(m.fixBlock, line)
	if raw := fixMessageBlockRaw(line); raw != "" {
		m.fixDecided = true
		if fixRawField(raw, "35") == "0" {
			m.fixHB = true
			m.setHeartbeatRaw(m.fixDir, raw)
		}
	}
	if m.fixHB {
		return true
	}
	if m.fixDecided {
		m.flushFIXMessageBlock()
	}
	return true
}

func (m *tuiModel) flushFIXMessageBlock() {
	if len(m.fixBlock) == 0 {
		return
	}
	if !m.fixHB {
		for _, line := range m.fixBlock {
			m.appendLogLine(line)
		}
	}
	m.fixBlock = nil
	m.fixDir = ""
	m.fixHB = false
	m.fixDecided = false
}

func (m *tuiModel) consumeHeartbeatLine(line string) bool {
	if ok, direction, raw := heartbeatJSONLine(line); ok {
		if raw != "" {
			m.setHeartbeatRaw(direction, raw)
		}
		return true
	}
	trimmed := strings.TrimSpace(line)
	if m.hbBlock != "" {
		switch trimmed {
		case "raw_message:":
			m.hbBlock = "raw"
			return true
		case "pretty_message:":
			m.hbBlock = "pretty"
			return true
		}
		if m.hbBlock == "raw" && trimmed != "" && trimmed != "raw_message:" {
			m.setHeartbeatRaw(m.hbDir, trimmed)
		}
		return true
	}
	if !isHeartbeatLine(line) {
		return false
	}
	direction := heartbeatDirection(line)
	if raw := rawMessageValue(line); raw != "" {
		m.setHeartbeatRaw(direction, raw)
		return true
	}
	if strings.Contains(line, "view=raw") || strings.Contains(line, "raw_message") {
		m.hbBlock = "raw"
		m.hbDir = direction
		return true
	}
	if strings.Contains(line, "view=pretty") || strings.Contains(line, "pretty_message") {
		m.hbBlock = "pretty"
		m.hbDir = direction
		return true
	}
	m.hbBlock = "message"
	m.hbDir = direction
	return true
}

func (m *tuiModel) setHeartbeatRaw(direction string, raw string) {
	switch direction {
	case "out":
		m.heartbeat.outbound = raw
	case "in":
		m.heartbeat.inbound = raw
	default:
		m.heartbeat.unknown = raw
	}
}

func (m *tuiModel) appendLogLine(line string) {
	m.logs = append(m.logs, line)
	if len(m.logs) > maxTUILogLines {
		m.logs = m.logs[len(m.logs)-maxTUILogLines:]
	}
	if m.scroll > 0 {
		m.scroll++
		m.clampScroll()
	}
}

func (m tuiModel) visibleLogRows(width int) []string {
	rows := make([]string, 0, len(m.logs))
	for _, line := range m.logs {
		rows = append(rows, wrapRunes(line, width)...)
	}
	if m.pending != "" {
		rows = append(rows, wrapRunes(m.pending, width)...)
	}
	return rows
}

func (m tuiModel) renderInput(width int) string {
	value := append([]rune(nil), m.input[:m.cursor]...)
	value = append(value, '█')
	value = append(value, m.input[m.cursor:]...)
	line := fitRunes(m.prompt+string(value), width)
	return colorTUIInputPrompt(line, m.prompt)
}

func (m tuiModel) logHeight() int {
	height := m.height
	if height < tuiMinimumHeight() {
		height = defaultTUIHeight
	}
	return maxValue(1, height-2-2*tuiSectionGapHeight-m.heartbeatHeight()-3-tuiHelpGapHeight-1)
}

func (m tuiModel) heartbeatHeight() int {
	return tuiHeartbeatHeight + 1
}

func (m tuiModel) heartbeatRows() []string {
	return []string{
		"OUT raw_message=" + heartbeatValue(m.heartbeat.outbound),
		"IN  raw_message=" + heartbeatValue(m.heartbeat.inbound, m.heartbeat.unknown),
	}
}

func (m tuiModel) renderSelectableLine(pane tuiPane, index int, line string, width int) string {
	line = fitRunes(line, width)
	line += strings.Repeat(" ", width-runeLenWithoutANSI(line))
	if m.focus != pane {
		return line
	}
	if m.isSelectedLine(pane, index) {
		return tuiSelectionColor + line + tuiColorReset
	}
	return line
}

func (m tuiModel) isSelectedLine(pane tuiPane, index int) bool {
	if m.focus != pane {
		return false
	}
	if m.mode == tuiModeVisual {
		start, end := orderedRange(m.visualFrom, m.focusedCursor())
		return index >= start && index <= end
	}
	return index == m.focusedCursor()
}

func renderPane(title string, focused bool, content []string, width int) []string {
	if width < 4 {
		width = defaultTUIWidth
	}
	contentWidth := paneContentWidth(width)
	label := " " + title + " "
	if focused {
		label = " " + title + "* "
	}
	top := "┌" + label + strings.Repeat("─", maxValue(0, width-2-len([]rune(label)))) + "┐"
	bottom := "└" + strings.Repeat("─", width-2) + "┘"
	lines := []string{top}
	for _, line := range content {
		if runeLenWithoutANSI(line) > contentWidth {
			line = fitRunes(line, contentWidth)
		}
		line += strings.Repeat(" ", maxValue(0, contentWidth-runeLenWithoutANSI(line)))
		lines = append(lines, "│"+line+"│")
	}
	lines = append(lines, bottom)
	return lines
}

func paneContentWidth(width int) int {
	if width <= 2 {
		return 1
	}
	return width - 2
}

func tuiMinimumHeight() int {
	return 2 + 2*tuiSectionGapHeight + (tuiHeartbeatHeight + 1) + 3 + tuiHelpGapHeight + 1
}

func orderedRange(a int, b int) (int, int) {
	if a <= b {
		return a, b
	}
	return b, a
}

func logVisibleRange(rowCount int, height int, scroll int) (int, int) {
	scroll = clampValue(scroll, 0, maxScroll(rowCount, height))
	bottom := rowCount - scroll
	if bottom < 0 {
		bottom = 0
	}
	if bottom > rowCount {
		bottom = rowCount
	}
	start := bottom - height
	if start < 0 {
		start = 0
	}
	return start, bottom
}

func (m tuiModel) copyStatusSuffix() string {
	return m.copyStatus
}

func titleDivider(title string, width int) string {
	if width <= 0 {
		width = defaultTUIWidth
	}
	label := "── " + title + " "
	runes := []rune(label)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return label + strings.Repeat("─", width-len(runes))
}

func colorTUIInputPrompt(line string, prompt string) string {
	if prompt == "" || !strings.HasPrefix(line, prompt) {
		return line
	}
	return tuiPromptColor + prompt + tuiColorReset + line[len(prompt):]
}

func colorTUIHeartbeatSeqNum(line string) string {
	const tag = "34="
	var builder strings.Builder
	start := 0
	for {
		index := strings.Index(line[start:], tag)
		if index < 0 {
			builder.WriteString(line[start:])
			return builder.String()
		}
		index += start
		if index > 0 && line[index-1] != '|' && line[index-1] != '=' && line[index-1] != ' ' {
			builder.WriteString(line[start : index+len(tag)])
			start = index + len(tag)
			continue
		}
		end := index + len(tag)
		for end < len(line) && line[end] != '|' && line[end] != ' ' {
			end++
		}
		if end == index+len(tag) {
			builder.WriteString(line[start : index+len(tag)])
			start = index + len(tag)
			continue
		}
		builder.WriteString(line[start:index])
		builder.WriteString(tuiHeartbeatColor)
		builder.WriteString(line[index:end])
		builder.WriteString(tuiColorReset)
		start = end
	}
}

func (m tuiModel) widthOrDefault() int {
	if m.width <= 0 {
		return defaultTUIWidth
	}
	return m.width
}

func (m *tuiModel) scrollUp(lines int) {
	if lines <= 0 {
		return
	}
	m.scroll += lines
	m.clampScroll()
}

func (m *tuiModel) scrollDown(lines int) {
	if lines <= 0 {
		return
	}
	m.scroll -= lines
	m.clampScroll()
}

func (m *tuiModel) clampScroll() {
	rows := m.visibleLogRows(m.widthOrDefault())
	m.scroll = clampValue(m.scroll, 0, maxScroll(len(rows), m.logHeight()))
}

func maxScroll(rows int, height int) int {
	if rows <= height {
		return 0
	}
	return rows - height
}

func maxValue(a int, b int) int {
	if a >= b {
		return a
	}
	return b
}

func clampValue(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func heartbeatValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "-"
}

func isHeartbeatLine(line string) bool {
	return strings.Contains(line, "Heartbeat(0)") ||
		strings.Contains(line, "msg_type=Heartbeat") ||
		strings.Contains(line, "msg_code=0")
}

func heartbeatDirection(line string) string {
	if strings.Contains(line, "direction=out") || strings.Contains(line, "-> Heartbeat(0)") {
		return "out"
	}
	if strings.Contains(line, "direction=in") || strings.Contains(line, "<- Heartbeat(0)") {
		return "in"
	}
	return ""
}

func rawMessageValue(line string) string {
	index := strings.Index(line, "raw_message=")
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(line[index+len("raw_message="):])
}

func fixMessageBlockDirection(line string) (string, bool) {
	if strings.Contains(line, "Outgoing FIX Msg") {
		return "out", true
	}
	if strings.Contains(line, "Incoming FIX Msg") {
		return "in", true
	}
	return "", false
}

func fixMessageBlockRaw(line string) string {
	value := strings.TrimSpace(line)
	value = strings.TrimSpace(strings.TrimPrefix(value, "|"))
	if !strings.HasPrefix(value, "8=") || fixRawField(value, "35") == "" {
		return ""
	}
	return value
}

func isFIXMessageBlockLine(line string) bool {
	if strings.HasPrefix(line, "|") {
		return true
	}
	if strings.HasPrefix(line, "  ") {
		return true
	}
	return strings.HasPrefix(line, "Time:") ||
		strings.HasPrefix(line, "Session:") ||
		strings.HasPrefix(line, "Content:")
}

func fixRawField(raw string, tag string) string {
	prefix := tag + "="
	for _, field := range strings.Split(raw, "|") {
		if value, ok := strings.CutPrefix(field, prefix); ok {
			return value
		}
	}
	return ""
}

func runeLenWithoutANSI(value string) int {
	count := 0
	inEscape := false
	for _, valueRune := range value {
		if inEscape {
			if valueRune == 'm' {
				inEscape = false
			}
			continue
		}
		if valueRune == '\x1b' {
			inEscape = true
			continue
		}
		count++
	}
	return count
}

func heartbeatJSONLine(line string) (bool, string, string) {
	if !strings.HasPrefix(strings.TrimSpace(line), "{") {
		return false, "", ""
	}
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return false, "", ""
	}
	message, _ := event["message"].(string)
	msgType, _ := event["msg_type"].(string)
	msgCode, _ := event["msg_code"].(string)
	heartbeat := strings.Contains(message, "Heartbeat(0)") || msgType == "Heartbeat" || msgCode == "0"
	if !heartbeat {
		return false, "", ""
	}
	direction, _ := event["direction"].(string)
	if direction == "" {
		direction = heartbeatDirection(message)
	}
	raw, _ := event["raw_message"].(string)
	return true, direction, raw
}

func waitTUIOutput(output *TUIOutputWriter) tea.Cmd {
	return func() tea.Msg {
		select {
		case message := <-output.messages:
			return tuiOutputMsg(message)
		case <-output.closed:
			return nil
		}
	}
}

func waitTUIDone(done *tuiRunnerState) tea.Cmd {
	return func() tea.Msg {
		if done == nil {
			return nil
		}
		<-done.done
		return tuiDoneMsg{err: done.err()}
	}
}

func wrapRunes(value string, width int) []string {
	if width <= 0 {
		width = defaultTUIWidth
	}
	runes := []rune(value)
	if len(runes) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, len(runes)/width+1)
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	lines = append(lines, string(runes))
	return lines
}

func fitRunes(value string, width int) string {
	if width <= 0 {
		width = defaultTUIWidth
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[len(runes)-width:])
	}
	return "…" + string(runes[len(runes)-width+1:])
}
