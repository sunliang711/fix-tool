package shell

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fix-tool/internal/admin"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/message"
	"fix-tool/internal/order"
	"fix-tool/internal/render"
	"fix-tool/internal/trace"

	"github.com/quickfixgo/quickfix"
)

func TestRunnerContinuesAfterCommandError(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	manager := &runnerFakeManager{}
	adminService := &stubAdminService{
		heartbeatErr: errors.New("heartbeat failed"),
		logonResult: admin.Result{
			Request: testTrace(t, "logon-request", trace.DirectionOutbound, "A"),
		},
	}
	runner := NewRunner(Options{
		In:      strings.NewReader("heartbeat\nlogon\nexit\n"),
		Out:     &out,
		ErrOut:  &errOut,
		Admin:   adminService,
		Order:   &stubOrderService{},
		Manager: manager,
		Renderer: render.NewRenderer(nil, render.Options{
			Format: render.FormatRaw,
		}),
		Format: render.FormatRaw,
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "heartbeat failed") {
		t.Fatalf("errOut = %q, want heartbeat error", errOut.String())
	}
	if strings.Contains(out.String(), "35=A|") {
		t.Fatalf("out = %q, want no immediate admin trace render", out.String())
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
}

func TestRunnerTraceListRendersRecordedTraces(t *testing.T) {
	var out bytes.Buffer
	adminService := &stubAdminService{
		logonResult: admin.Result{
			Request:  testTrace(t, "logon-request", trace.DirectionOutbound, "A"),
			Response: testTrace(t, "logon-response", trace.DirectionInbound, "A"),
		},
	}
	runner := NewRunner(Options{
		In:      strings.NewReader("logon\ntrace list\nexit\n"),
		Out:     &out,
		ErrOut:  &bytes.Buffer{},
		Admin:   adminService,
		Order:   &stubOrderService{},
		Manager: &runnerFakeManager{},
		Renderer: render.NewRenderer(nil, render.Options{
			Format: render.FormatRaw,
		}),
		Format: render.FormatRaw,
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"Trace 1", "Trace 2", "35=A|"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("out = %q, want %q", out.String(), want)
		}
	}
	for _, unwanted := range []string{"Request", "Response"} {
		if strings.Contains(out.String(), unwanted) {
			t.Fatalf("out = %q, want no immediate admin trace title %q", out.String(), unwanted)
		}
	}
}

func TestRunnerExitStopsSession(t *testing.T) {
	input := newLineThenBlockReadCloser("exit\n")
	manager := &runnerFakeManager{}
	runner := NewRunner(Options{
		In:      input,
		Out:     &bytes.Buffer{},
		ErrOut:  &bytes.Buffer{},
		Admin:   &stubAdminService{},
		Order:   &stubOrderService{},
		Manager: manager,
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
	select {
	case <-input.readDone:
	case <-time.After(time.Second):
		t.Fatal("reader goroutine did not exit")
	}
}

func TestRunnerHelpCommand(t *testing.T) {
	var out bytes.Buffer
	runner := NewRunner(Options{
		In:      strings.NewReader("help\n?\nexit\n"),
		Out:     &out,
		ErrOut:  &bytes.Buffer{},
		Admin:   &stubAdminService{},
		Order:   &stubOrderService{},
		Manager: &runnerFakeManager{},
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"Commands:", "help, ?", "logon", "order new", "save <file>", "trace list", "exit", "Up/Down"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("out = %q, want %q", out.String(), want)
		}
	}
	if count := strings.Count(out.String(), "Commands:"); count != 2 {
		t.Fatalf("help count = %d, want 2 in %q", count, out.String())
	}
}

func TestRunnerUsesLineReader(t *testing.T) {
	var out bytes.Buffer
	reader := &fakeLineReader{
		lines: []string{"help", "exit"},
	}
	runner := NewRunner(Options{
		LineReader: reader,
		Out:        &out,
		ErrOut:     &bytes.Buffer{},
		Admin:      &stubAdminService{},
		Order:      &stubOrderService{},
		Manager:    &runnerFakeManager{},
		Prompt:     "fix-tool> ",
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !reader.closed {
		t.Fatal("line reader was not closed")
	}
	if got, want := strings.Join(reader.prompts, ","), "fix-tool> ,fix-tool> "; got != want {
		t.Fatalf("prompts = %q, want %q", got, want)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatalf("out = %q, want help text", out.String())
	}
}

func TestRunnerSaveCommandRecordsTranscript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.log")
	transcript := NewTranscriptRecorder("fix-tool> ")
	var out bytes.Buffer
	outWriter := transcript.Wrap(&out)
	runner := NewRunner(Options{
		In:         strings.NewReader("save " + path + "\nhelp\nsave status\nsave stop\nexit\n"),
		Out:        outWriter,
		ErrOut:     transcript.Wrap(&bytes.Buffer{}),
		Admin:      &stubAdminService{},
		Order:      &stubOrderService{},
		Manager:    &runnerFakeManager{},
		Transcript: transcript,
		Prompt:     "fix-tool> ",
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"# fix-tool shell transcript",
		"saving shell transcript to " + path,
		"fix-tool> help",
		"Commands:",
		"fix-tool> save status",
		"save active file=" + path,
		"fix-tool> save stop",
		"saving shell transcript stopped file=" + path,
		"# stopped_at=",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("transcript = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "fix-tool> fix-tool>") {
		t.Fatalf("transcript = %q, want no duplicate prompt", got)
	}
}

func TestRunnerContextCancelWhileIdleStopsSession(t *testing.T) {
	input := newBlockingReadCloser()
	manager := &runnerFakeManager{}
	runner := NewRunner(Options{
		In:      input,
		Out:     &bytes.Buffer{},
		ErrOut:  &bytes.Buffer{},
		Admin:   &stubAdminService{},
		Order:   &stubOrderService{},
		Manager: manager,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- runner.Run(ctx)
	}()
	select {
	case <-input.readStarted:
	case <-time.After(time.Second):
		t.Fatal("reader did not start")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancel")
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
	select {
	case <-input.readDone:
	case <-time.After(time.Second):
		t.Fatal("reader goroutine did not exit")
	}
}

func TestRunnerReusesManagerSessionAndLoggedOnState(t *testing.T) {
	var out bytes.Buffer
	manager := newInteractiveFakeManager(t)
	state := NewSessionState()
	runner := NewRunner(Options{
		In:     strings.NewReader("logon\norder new --symbol AAPL --side buy --qty 100 --price 10.25\nexit\n"),
		Out:    &out,
		ErrOut: &bytes.Buffer{},
		Admin: admin.NewService(manager, admin.Options{
			Timeout:      time.Second,
			KeepSession:  true,
			SessionState: state,
		}),
		Order: order.NewService(manager, order.Options{
			Timeout:      time.Second,
			KeepSession:  true,
			SessionState: state,
		}),
		Manager: manager,
		Renderer: render.NewRenderer(nil, render.Options{
			Format: render.FormatRaw,
		}),
		Format: render.FormatRaw,
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if manager.starts != 2 {
		t.Fatalf("starts = %d, want 2", manager.starts)
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
	if len(manager.session.sent) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(manager.session.sent))
	}
	if manager.session.sent[0] != message.MsgTypeNewOrderSingle {
		t.Fatalf("sent message = %q, want NewOrderSingle", manager.session.sent[0])
	}
	if !strings.Contains(out.String(), "35=D|") {
		t.Fatalf("out = %q, want order trace", out.String())
	}
}

type stubAdminService struct {
	logonResult  admin.Result
	heartbeatErr error
}

func (s *stubAdminService) Logon(context.Context) (admin.Result, error) {
	return s.logonResult, nil
}

func (s *stubAdminService) Logout(context.Context) (admin.Result, error) {
	return admin.Result{}, nil
}

func (s *stubAdminService) Heartbeat(context.Context) (admin.Result, error) {
	if s.heartbeatErr != nil {
		return admin.Result{}, s.heartbeatErr
	}
	return admin.Result{}, nil
}

func (s *stubAdminService) TestRequest(context.Context, string) (admin.Result, error) {
	return admin.Result{}, nil
}

type stubOrderService struct{}

func (s *stubOrderService) NewOrder(context.Context, order.NewRequest) (order.Result, error) {
	return order.Result{}, nil
}

func (s *stubOrderService) CancelOrder(context.Context, order.CancelRequest) (order.Result, error) {
	return order.Result{}, nil
}

func (s *stubOrderService) ReplaceOrder(context.Context, order.ReplaceRequest) (order.Result, error) {
	return order.Result{}, nil
}

type fakeLineReader struct {
	lines   []string
	prompts []string
	closed  bool
}

func (r *fakeLineReader) ReadLine(ctx context.Context, prompt string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	r.prompts = append(r.prompts, prompt)
	if len(r.lines) == 0 {
		return "", false, nil
	}
	line := r.lines[0]
	r.lines = r.lines[1:]
	return line, true, nil
}

func (r *fakeLineReader) Close() error {
	r.closed = true
	return nil
}

type runnerFakeManager struct {
	stops int
}

func (m *runnerFakeManager) Start(context.Context) error {
	return nil
}

func (m *runnerFakeManager) Stop(context.Context) error {
	m.stops++
	return nil
}

func (m *runnerFakeManager) Events() <-chan fixsession.Event {
	return nil
}

func (m *runnerFakeManager) Session() fixsession.Session {
	return nil
}

type blockingReadCloser struct {
	readStarted chan struct{}
	closed      chan struct{}
	readDone    chan struct{}
	startOnce   sync.Once
	closeOnce   sync.Once
	doneOnce    sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{
		readStarted: make(chan struct{}),
		closed:      make(chan struct{}),
		readDone:    make(chan struct{}),
	}
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	r.startOnce.Do(func() {
		close(r.readStarted)
	})
	<-r.closed
	r.doneOnce.Do(func() {
		close(r.readDone)
	})
	return 0, io.EOF
}

func (r *blockingReadCloser) Close() error {
	r.closeOnce.Do(func() {
		close(r.closed)
	})
	return nil
}

type lineThenBlockReadCloser struct {
	mu        sync.Mutex
	data      []byte
	sent      bool
	closed    chan struct{}
	readDone  chan struct{}
	closeOnce sync.Once
	doneOnce  sync.Once
}

func newLineThenBlockReadCloser(line string) *lineThenBlockReadCloser {
	return &lineThenBlockReadCloser{
		data:     []byte(line),
		closed:   make(chan struct{}),
		readDone: make(chan struct{}),
	}
}

func (r *lineThenBlockReadCloser) Read(p []byte) (int, error) {
	r.mu.Lock()
	if !r.sent {
		r.sent = true
		n := copy(p, r.data)
		r.mu.Unlock()
		return n, nil
	}
	r.mu.Unlock()

	<-r.closed
	r.doneOnce.Do(func() {
		close(r.readDone)
	})
	return 0, io.EOF
}

func (r *lineThenBlockReadCloser) Close() error {
	r.closeOnce.Do(func() {
		close(r.closed)
	})
	return nil
}

type interactiveFakeManager struct {
	t       *testing.T
	events  chan fixsession.Event
	session *interactiveFakeSession
	started bool
	starts  int
	stops   int
}

func newInteractiveFakeManager(t *testing.T) *interactiveFakeManager {
	sessionID := quickfix.SessionID{
		BeginString:  "FIX.4.4",
		SenderCompID: "SENDER",
		TargetCompID: "TARGET",
	}
	manager := &interactiveFakeManager{
		t:      t,
		events: make(chan fixsession.Event, 16),
		session: &interactiveFakeSession{
			id:          sessionID,
			profileName: "test",
		},
	}
	manager.session.manager = manager
	return manager
}

func (m *interactiveFakeManager) Start(context.Context) error {
	m.starts++
	if m.started {
		return nil
	}
	m.started = true
	m.emit(fixsession.Event{Type: fixsession.EventToAdmin, MsgType: "A", Message: rawAdminMessage("A")})
	m.emit(fixsession.Event{Type: fixsession.EventFromAdmin, MsgType: "A", Message: rawAdminMessage("A")})
	m.emit(fixsession.Event{Type: fixsession.EventLogon})
	return nil
}

func (m *interactiveFakeManager) Stop(context.Context) error {
	m.stops++
	m.started = false
	return nil
}

func (m *interactiveFakeManager) Events() <-chan fixsession.Event {
	return m.events
}

func (m *interactiveFakeManager) Session() fixsession.Session {
	return m.session
}

func (m *interactiveFakeManager) emit(event fixsession.Event) {
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	m.events <- event
}

type interactiveFakeSession struct {
	manager     *interactiveFakeManager
	id          quickfix.SessionID
	profileName string
	sent        []string
}

func (s *interactiveFakeSession) ID() quickfix.SessionID {
	return s.id
}

func (s *interactiveFakeSession) ProfileName() string {
	return s.profileName
}

func (s *interactiveFakeSession) Send(value *quickfix.Message) error {
	msgType := headerValue(value, message.TagMsgType)
	s.sent = append(s.sent, msgType)
	if msgType == message.MsgTypeNewOrderSingle {
		clOrdID := bodyValue(value, message.TagClOrdID)
		s.manager.emit(fixsession.Event{
			Type:    fixsession.EventToApp,
			MsgType: msgType,
			Message: rawOrderMessage(msgType, map[int]string{
				message.TagClOrdID: clOrdID,
			}),
		})
		s.manager.emit(fixsession.Event{
			Type:    fixsession.EventFromApp,
			MsgType: message.MsgTypeExecutionReport,
			Message: rawOrderMessage(message.MsgTypeExecutionReport, map[int]string{
				message.TagClOrdID: clOrdID,
				message.TagOrderID: "O001",
			}),
		})
	}
	return nil
}

func testTrace(t *testing.T, traceID string, direction trace.Direction, msgType string) *trace.MessageTrace {
	t.Helper()
	message, err := trace.NewMessageTrace(trace.BuildOptions{
		TraceID:   traceID,
		Profile:   "test",
		Direction: direction,
		Raw:       rawAdminMessage(msgType),
		SentAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("NewMessageTrace() error = %v", err)
	}
	return &message
}

func rawAdminMessage(msgType string) string {
	return rawOrderMessage(msgType, nil)
}

func rawOrderMessage(msgType string, fields map[int]string) string {
	value := quickfix.NewMessage()
	value.Header.SetString(quickfix.Tag(message.TagBeginString), "FIX.4.4")
	value.Header.SetString(quickfix.Tag(message.TagMsgType), msgType)
	value.Header.SetString(quickfix.Tag(message.TagMsgSeqNum), "1")
	value.Header.SetString(quickfix.Tag(message.TagSenderCompID), "SENDER")
	value.Header.SetString(quickfix.Tag(message.TagSendingTime), time.Now().UTC().Format("20060102-15:04:05.000"))
	value.Header.SetString(quickfix.Tag(message.TagTargetCompID), "TARGET")
	for tagValue, fieldValue := range fields {
		value.Body.SetString(quickfix.Tag(tagValue), fieldValue)
	}
	return value.String()
}

func headerValue(value *quickfix.Message, tagValue int) string {
	result, err := value.Header.GetString(quickfix.Tag(tagValue))
	if err != nil {
		return ""
	}
	return result
}

func bodyValue(value *quickfix.Message, tagValue int) string {
	result, err := value.Body.GetString(quickfix.Tag(tagValue))
	if err != nil {
		return ""
	}
	return result
}
