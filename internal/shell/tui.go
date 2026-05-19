package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	heartbeat  tuiHeartbeatState
	hbBlock    string
	hbDir      string
}

func newTUIModel(prompt string, reader *TUILineReader, output *TUIOutputWriter, done *tuiRunnerState) tuiModel {
	return tuiModel{
		prompt:     prompt,
		reader:     reader,
		output:     output,
		done:       done,
		width:      defaultTUIWidth,
		height:     defaultTUIHeight,
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
	if height <= tuiLogsTitleHeight+2*tuiSectionGapHeight+tuiInputHeight+m.heartbeatHeight() {
		height = defaultTUIHeight
	}
	logHeight := height - tuiLogsTitleHeight - 2*tuiSectionGapHeight - tuiInputHeight - m.heartbeatHeight()
	rows := m.visibleLogRows(width)
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
	for len(visible) < logHeight {
		visible = append([]string{""}, visible...)
	}
	input := m.renderInput(width)
	help := fitRunes("Enter run  Up/Down history  PgUp/PgDown logs  F2 "+m.mouseLabel()+"  Ctrl+L clear  Ctrl+C/Ctrl+D exit", width)
	lines := []string{titleDivider("Logs", width)}
	lines = append(lines, visible...)
	lines = append(lines, "")
	lines = append(lines, m.renderHeartbeat(width)...)
	lines = append(lines, "")
	lines = append(lines, titleDivider("Command", width), input)
	for range tuiHelpGapHeight {
		lines = append(lines, "")
	}
	lines = append(lines, help)
	return strings.Join(lines, "\n")
}

func (m tuiModel) handleKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
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
	if m.hbBlock != "" && !strings.HasPrefix(line, " ") {
		m.hbBlock = ""
		m.hbDir = ""
	}
	if m.consumeHeartbeatLine(line) {
		return
	}
	m.appendLogLine(line)
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
	if height <= tuiLogsTitleHeight+2*tuiSectionGapHeight+tuiInputHeight+m.heartbeatHeight() {
		height = defaultTUIHeight
	}
	return height - tuiLogsTitleHeight - 2*tuiSectionGapHeight - tuiInputHeight - m.heartbeatHeight()
}

func (m tuiModel) heartbeatHeight() int {
	return tuiHeartbeatHeight
}

func (m tuiModel) renderHeartbeat(width int) []string {
	return []string{
		titleDivider("Heartbeat", width),
		colorTUIHeartbeatSeqNum(fitRunes("OUT raw_message="+heartbeatValue(m.heartbeat.outbound), width)),
		colorTUIHeartbeatSeqNum(fitRunes("IN  raw_message="+heartbeatValue(m.heartbeat.inbound, m.heartbeat.unknown), width)),
	}
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
