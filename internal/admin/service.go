package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"fix-tool/internal/fixsession"
	"fix-tool/internal/trace"

	"github.com/quickfixgo/quickfix"
)

const (
	msgTypeHeartbeat      = "0"
	msgTypeTestRequest    = "1"
	msgTypeReject         = "3"
	msgTypeLogout         = "5"
	msgTypeLogon          = "A"
	msgTypeBusinessReject = "j"

	tagMsgType              = quickfix.Tag(35)
	tagRefSeqNum            = quickfix.Tag(45)
	tagText                 = quickfix.Tag(58)
	tagTestReqID            = quickfix.Tag(112)
	tagSessionRejectReason  = quickfix.Tag(373)
	tagBusinessRejectRefID  = quickfix.Tag(379)
	tagBusinessRejectReason = quickfix.Tag(380)
	soh                     = "\x01"

	DefaultTimeout     = 30 * time.Second
	defaultStopTimeout = 5 * time.Second
)

var (
	ErrTimeout               = errors.New("admin command timeout")
	ErrSessionUnavailable    = errors.New("fix session unavailable")
	ErrEventStreamClosed     = errors.New("fix event stream closed")
	ErrLogonRejected         = errors.New("logon rejected by counterparty")
	ErrLogonEnded            = errors.New("fix session ended before logon")
	ErrTestRequestIDRequired = errors.New("test request id is required")
	ErrTestRequestIDInvalid  = errors.New("test request id cannot contain SOH delimiter")
)

type LogonDiagnostics struct {
	Timeout                     time.Duration
	Session                     string
	LogonSent                   bool
	LogonResponseSeen           bool
	LastEvent                   string
	LastAdminMsgType            string
	LastAdminText               string
	LastAdminRefSeqNum          string
	LastAdminSessionRejectCode  string
	LastAdminBusinessRejectID   string
	LastAdminBusinessRejectCode string
}

type LogonError struct {
	Err         error
	Diagnostics LogonDiagnostics
}

func (e *LogonError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *LogonError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func LogonDiagnosticsFromError(err error) (LogonDiagnostics, bool) {
	var logonErr *LogonError
	if errors.As(err, &logonErr) {
		return logonErr.Diagnostics, true
	}
	return LogonDiagnostics{}, false
}

type Options struct {
	Timeout      time.Duration
	StopTimeout  time.Duration
	KeepSession  bool
	SessionState SessionState
}

// SessionState 用于 shell 内跨 admin/order service 共享登录状态，避免重复等待已被消费的 Logon 事件。
type SessionState interface {
	LoggedOn() bool
	SetLoggedOn(bool)
}

type Service struct {
	manager     fixsession.Manager
	timeout     time.Duration
	stopTimeout time.Duration
	keepSession bool
	state       SessionState
}

type Result struct {
	Request  *trace.MessageTrace
	Response *trace.MessageTrace
}

type commandSpec struct {
	name          string
	msgType       string
	buildMessage  func() *quickfix.Message
	matchResponse func(fixsession.Event) bool
}

func NewService(manager fixsession.Manager, options Options) *Service {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	stopTimeout := options.StopTimeout
	if stopTimeout <= 0 {
		stopTimeout = defaultStopTimeout
	}
	return &Service{
		manager:     manager,
		timeout:     timeout,
		stopTimeout: stopTimeout,
		keepSession: options.KeepSession,
		state:       sessionStateOrDefault(options.SessionState),
	}
}

func (s *Service) Logon(ctx context.Context) (result Result, err error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if err := s.start(ctx); err != nil {
		return Result{}, err
	}
	defer s.stopAfterCommand(&err)

	if s.loggedOn() {
		return Result{}, nil
	}
	result, err = s.waitLogon(ctx, true)
	if err != nil {
		return Result{}, err
	}
	s.setLoggedOn(true)
	return result, nil
}

func (s *Service) Logout(ctx context.Context) (result Result, err error) {
	return s.execute(ctx, commandSpec{
		name:    "logout",
		msgType: msgTypeLogout,
		buildMessage: func() *quickfix.Message {
			return newAdminMessage(msgTypeLogout)
		},
		matchResponse: func(event fixsession.Event) bool {
			return event.Type == fixsession.EventFromAdmin && event.MsgType == msgTypeLogout
		},
	})
}

func (s *Service) Heartbeat(ctx context.Context) (result Result, err error) {
	return s.execute(ctx, commandSpec{
		name:    "heartbeat",
		msgType: msgTypeHeartbeat,
		buildMessage: func() *quickfix.Message {
			return newAdminMessage(msgTypeHeartbeat)
		},
		matchResponse: func(event fixsession.Event) bool {
			return event.Type == fixsession.EventFromAdmin && event.MsgType == msgTypeHeartbeat
		},
	})
}

func (s *Service) TestRequest(ctx context.Context, id string) (result Result, err error) {
	if id == "" {
		return Result{}, ErrTestRequestIDRequired
	}
	if strings.Contains(id, soh) {
		return Result{}, ErrTestRequestIDInvalid
	}
	return s.execute(ctx, commandSpec{
		name:    "test-request",
		msgType: msgTypeTestRequest,
		buildMessage: func() *quickfix.Message {
			message := newAdminMessage(msgTypeTestRequest)
			message.Body.SetString(tagTestReqID, id)
			return message
		},
		matchResponse: func(event fixsession.Event) bool {
			return event.Type == fixsession.EventFromAdmin &&
				event.MsgType == msgTypeHeartbeat &&
				eventValue(event, int(tagTestReqID)) == id
		},
	})
}

func (s *Service) execute(ctx context.Context, spec commandSpec) (result Result, err error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if err := s.start(ctx); err != nil {
		return Result{}, err
	}
	defer s.stopAfterCommand(&err)

	if err := s.ensureLoggedOn(ctx); err != nil {
		return Result{}, err
	}

	session, err := s.session()
	if err != nil {
		return Result{}, err
	}
	sentAt := time.Now().UTC()
	if err := session.Send(spec.buildMessage()); err != nil {
		return Result{}, fmt.Errorf("%s send admin message: %w", spec.name, err)
	}
	result, err = s.waitExchange(ctx, spec, sentAt)
	if err == nil && spec.msgType == msgTypeLogout {
		s.setLoggedOn(false)
	}
	return result, err
}

func (s *Service) start(ctx context.Context) error {
	if s == nil || s.manager == nil {
		return ErrSessionUnavailable
	}
	if err := s.manager.Start(ctx); err != nil {
		return fmt.Errorf("start fix session: %w", err)
	}
	return nil
}

func (s *Service) stopAfterCommand(err *error) {
	if s.keepSession {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.stopTimeout)
	defer cancel()
	if stopErr := s.manager.Stop(ctx); stopErr != nil && *err == nil {
		*err = fmt.Errorf("stop fix session: %w", stopErr)
	}
	s.setLoggedOn(false)
}

func (s *Service) session() (fixsession.Session, error) {
	session := s.manager.Session()
	if session == nil {
		return nil, ErrSessionUnavailable
	}
	return session, nil
}

func (s *Service) waitLogon(ctx context.Context, collectTrace bool) (Result, error) {
	traceID := newTraceID("logon")
	profile := s.profileName()
	var result Result
	var requestTime time.Time
	diagnostics := s.newLogonDiagnostics()

	for {
		select {
		case event, ok := <-s.manager.Events():
			if !ok {
				return result, ErrEventStreamClosed
			}
			diagnostics.observe(event)
			if collectTrace && event.Type == fixsession.EventToAdmin && event.MsgType == msgTypeLogon {
				requestTime = eventTime(event)
				request, err := traceFromEvent(traceID, profile, trace.DirectionOutbound, event, requestTime, time.Time{})
				if err != nil {
					return Result{}, err
				}
				result.Request = &request
				continue
			}
			if collectTrace && event.Type == fixsession.EventFromAdmin && event.MsgType == msgTypeLogon {
				response, err := traceFromEvent(traceID, profile, trace.DirectionInbound, event, requestTime, eventTime(event))
				if err != nil {
					return Result{}, err
				}
				result.Response = &response
				continue
			}
			if isLogonRejectEvent(event) {
				return result, newLogonError(fmt.Errorf("logon rejected by counterparty: %w", ErrLogonRejected), diagnostics)
			}
			if event.Type == fixsession.EventLogout {
				return result, newLogonError(fmt.Errorf("logon ended before success: %w", ErrLogonEnded), diagnostics)
			}
			if event.Type == fixsession.EventLogon {
				return result, nil
			}
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return result, newLogonError(fmt.Errorf("logon: %w", ErrTimeout), diagnostics)
			}
			return result, fmt.Errorf("logon canceled: %w", ctx.Err())
		}
	}
}

func (s *Service) newLogonDiagnostics() LogonDiagnostics {
	diagnostics := LogonDiagnostics{
		Timeout: s.timeout,
	}
	if session, err := s.session(); err == nil && session != nil {
		diagnostics.Session = session.ID().String()
	}
	return diagnostics
}

func newLogonError(err error, diagnostics LogonDiagnostics) *LogonError {
	return &LogonError{
		Err:         err,
		Diagnostics: diagnostics,
	}
}

func (d *LogonDiagnostics) observe(event fixsession.Event) {
	if d == nil {
		return
	}
	d.LastEvent = string(event.Type)
	if event.Type != fixsession.EventToAdmin && event.Type != fixsession.EventFromAdmin {
		return
	}
	d.LastAdminMsgType = event.MsgType
	if event.Type == fixsession.EventToAdmin && event.MsgType == msgTypeLogon {
		d.LogonSent = true
	}
	if event.Type == fixsession.EventFromAdmin && event.MsgType == msgTypeLogon {
		d.LogonResponseSeen = true
	}
	d.LastAdminText = eventValue(event, int(tagText))
	d.LastAdminRefSeqNum = eventValue(event, int(tagRefSeqNum))
	d.LastAdminSessionRejectCode = eventValue(event, int(tagSessionRejectReason))
	d.LastAdminBusinessRejectID = eventValue(event, int(tagBusinessRejectRefID))
	d.LastAdminBusinessRejectCode = eventValue(event, int(tagBusinessRejectReason))
}

func isLogonRejectEvent(event fixsession.Event) bool {
	if event.Type != fixsession.EventFromAdmin {
		return false
	}
	switch event.MsgType {
	case msgTypeLogout, msgTypeReject, msgTypeBusinessReject:
		return true
	default:
		return false
	}
}

func (s *Service) ensureLoggedOn(ctx context.Context) error {
	if s.loggedOn() {
		return nil
	}
	if _, err := s.waitLogon(ctx, false); err != nil {
		return err
	}
	s.setLoggedOn(true)
	return nil
}

func (s *Service) waitExchange(ctx context.Context, spec commandSpec, sentAt time.Time) (Result, error) {
	traceID := newTraceID(spec.name)
	profile := s.profileName()
	result := Result{}
	requestTime := sentAt

	for {
		event, err := nextEvent(ctx, s.manager.Events(), spec.name)
		if err != nil {
			return Result{}, err
		}
		if event.Type == fixsession.EventToAdmin && event.MsgType == spec.msgType {
			requestTime = eventTime(event)
			request, err := traceFromEvent(traceID, profile, trace.DirectionOutbound, event, requestTime, time.Time{})
			if err != nil {
				return Result{}, err
			}
			result.Request = &request
			continue
		}
		if spec.matchResponse(event) {
			response, err := traceFromEvent(traceID, profile, trace.DirectionInbound, event, requestTime, eventTime(event))
			if err != nil {
				return Result{}, err
			}
			result.Response = &response
			return result, nil
		}
	}
}

func nextEvent(ctx context.Context, events <-chan fixsession.Event, command string) (fixsession.Event, error) {
	select {
	case event, ok := <-events:
		if !ok {
			return fixsession.Event{}, ErrEventStreamClosed
		}
		return event, nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fixsession.Event{}, fmt.Errorf("%s: %w", command, ErrTimeout)
		}
		return fixsession.Event{}, fmt.Errorf("%s canceled: %w", command, ctx.Err())
	}
}

func traceFromEvent(traceID string, profile string, direction trace.Direction, event fixsession.Event, sentAt time.Time, receivedAt time.Time) (trace.MessageTrace, error) {
	return trace.NewMessageTrace(trace.BuildOptions{
		TraceID:    traceID,
		Profile:    profile,
		Direction:  direction,
		Raw:        event.Message,
		SentAt:     sentAt,
		ReceivedAt: receivedAt,
	})
}

func newAdminMessage(msgType string) *quickfix.Message {
	message := quickfix.NewMessage()
	message.Header.SetString(tagMsgType, msgType)
	return message
}

func newTraceID(command string) string {
	return fmt.Sprintf("%s-%d", command, time.Now().UTC().UnixNano())
}

func eventTime(event fixsession.Event) time.Time {
	if event.Time.IsZero() {
		return time.Now().UTC()
	}
	return event.Time
}

func eventValue(event fixsession.Event, tag int) string {
	parsed, err := trace.ParseRaw(event.Message)
	if err != nil {
		return ""
	}
	for _, field := range parsed.Fields {
		if field.Tag == tag {
			return field.Value
		}
	}
	return ""
}

func (s *Service) profileName() string {
	session, err := s.session()
	if err != nil {
		return ""
	}
	return session.ProfileName()
}

func (s *Service) loggedOn() bool {
	if s == nil || s.state == nil {
		return false
	}
	return s.state.LoggedOn()
}

func (s *Service) setLoggedOn(value bool) {
	if s == nil || s.state == nil {
		return
	}
	s.state.SetLoggedOn(value)
}

func sessionStateOrDefault(state SessionState) SessionState {
	if state != nil {
		return state
	}
	return &memorySessionState{}
}

type memorySessionState struct {
	mu       sync.RWMutex
	loggedOn bool
}

func (s *memorySessionState) LoggedOn() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loggedOn
}

func (s *memorySessionState) SetLoggedOn(value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loggedOn = value
}
