package order

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"fix-tool/internal/fixsession"
	"fix-tool/internal/message"
	"fix-tool/internal/trace"

	"github.com/quickfixgo/quickfix"
)

const (
	DefaultTimeout     = 30 * time.Second
	defaultStopTimeout = 5 * time.Second
)

var (
	ErrTimeout            = errors.New("order command timeout")
	ErrSessionUnavailable = errors.New("fix session unavailable")
	ErrEventStreamClosed  = errors.New("fix event stream closed")
)

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

type NewRequest struct {
	ClOrdID     string
	Symbol      string
	Side        string
	OrderQty    string
	Price       string
	OrdType     string
	TimeInForce string
	Tags        []string
}

type CancelRequest struct {
	OrigClOrdID string
	ClOrdID     string
	OrderID     string
	Symbol      string
	Side        string
	Tags        []string
}

type ReplaceRequest struct {
	OrigClOrdID string
	ClOrdID     string
	OrderID     string
	Symbol      string
	Side        string
	OrderQty    string
	Price       string
	OrdType     string
	TimeInForce string
	Tags        []string
}

type commandSpec struct {
	name    string
	message *quickfix.Message
	request requestIdentity
}

type requestIdentity struct {
	MsgType     string
	ClOrdID     string
	OrigClOrdID string
	OrderID     string
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

func (s *Service) NewOrder(ctx context.Context, request NewRequest) (Result, error) {
	messageValue, err := message.BuildNewOrderSingle(message.NewOrderSingleRequest{
		ClOrdID:     request.ClOrdID,
		Symbol:      request.Symbol,
		Side:        request.Side,
		OrderQty:    request.OrderQty,
		Price:       request.Price,
		OrdType:     request.OrdType,
		TimeInForce: request.TimeInForce,
		Tags:        request.Tags,
	})
	if err != nil {
		return Result{}, err
	}
	return s.execute(ctx, commandSpec{
		name:    "order new",
		message: messageValue,
		request: identityFromMessage(messageValue),
	})
}

func (s *Service) CancelOrder(ctx context.Context, request CancelRequest) (Result, error) {
	messageValue, err := message.BuildOrderCancelRequest(message.OrderCancelRequest{
		OrigClOrdID: request.OrigClOrdID,
		ClOrdID:     request.ClOrdID,
		OrderID:     request.OrderID,
		Symbol:      request.Symbol,
		Side:        request.Side,
		Tags:        request.Tags,
	})
	if err != nil {
		return Result{}, err
	}
	return s.execute(ctx, commandSpec{
		name:    "order cancel",
		message: messageValue,
		request: identityFromMessage(messageValue),
	})
}

func (s *Service) ReplaceOrder(ctx context.Context, request ReplaceRequest) (Result, error) {
	messageValue, err := message.BuildOrderCancelReplaceRequest(message.OrderCancelReplaceRequest{
		OrigClOrdID: request.OrigClOrdID,
		ClOrdID:     request.ClOrdID,
		OrderID:     request.OrderID,
		Symbol:      request.Symbol,
		Side:        request.Side,
		OrderQty:    request.OrderQty,
		Price:       request.Price,
		OrdType:     request.OrdType,
		TimeInForce: request.TimeInForce,
		Tags:        request.Tags,
	})
	if err != nil {
		return Result{}, err
	}
	return s.execute(ctx, commandSpec{
		name:    "order replace",
		message: messageValue,
		request: identityFromMessage(messageValue),
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
	if err := session.Send(spec.message); err != nil {
		return Result{}, fmt.Errorf("%s send order message: %w", spec.name, err)
	}
	return s.waitExchange(ctx, spec, sentAt)
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

func (s *Service) waitLogon(ctx context.Context) error {
	for {
		event, err := nextEvent(ctx, s.manager.Events(), "order logon")
		if err != nil {
			return err
		}
		if event.Type == fixsession.EventLogon {
			return nil
		}
	}
}

func (s *Service) ensureLoggedOn(ctx context.Context) error {
	if s.loggedOn() {
		return nil
	}
	if err := s.waitLogon(ctx); err != nil {
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
		if event.Type == fixsession.EventToApp && eventMsgType(event) == spec.request.MsgType {
			requestTime = eventTime(event)
			request, err := traceFromEvent(traceID, profile, trace.DirectionOutbound, event, requestTime, time.Time{})
			if err != nil {
				return Result{}, err
			}
			result.Request = &request
			continue
		}
		if matchResponse(spec.request, event) {
			response, err := traceFromEvent(traceID, profile, trace.DirectionInbound, event, requestTime, eventTime(event))
			if err != nil {
				return Result{}, err
			}
			result.Response = &response
			return result, nil
		}
	}
}

func matchResponse(request requestIdentity, event fixsession.Event) bool {
	if event.Type != fixsession.EventFromApp && event.Type != fixsession.EventFromAdmin {
		return false
	}
	msgType := eventMsgType(event)
	switch msgType {
	case message.MsgTypeExecutionReport:
		return matchCorrelatedResponse(request, event, false)
	case message.MsgTypeReject, message.MsgTypeBusinessMessageReject:
		return matchCorrelatedResponse(request, event, true)
	default:
		return false
	}
}

func matchCorrelatedResponse(request requestIdentity, event fixsession.Event, allowUncorrelated bool) bool {
	fields := eventFields(event)
	hasCorrelation := fields[message.TagClOrdID] != "" || fields[message.TagOrigClOrdID] != "" || fields[message.TagOrderID] != ""
	if fields[message.TagClOrdID] != "" && fields[message.TagClOrdID] == request.ClOrdID {
		return true
	}
	if fields[message.TagOrigClOrdID] != "" && fields[message.TagOrigClOrdID] == request.OrigClOrdID {
		return true
	}
	if fields[message.TagOrderID] != "" && request.OrderID != "" && fields[message.TagOrderID] == request.OrderID {
		return true
	}
	// Reject 和 BusinessMessageReject 常见情况下没有订单关联字段，只能作为当前请求响应处理。
	return allowUncorrelated && !hasCorrelation
}

func identityFromMessage(value *quickfix.Message) requestIdentity {
	return requestIdentity{
		MsgType:     headerValue(value, message.TagMsgType),
		ClOrdID:     bodyValue(value, message.TagClOrdID),
		OrigClOrdID: bodyValue(value, message.TagOrigClOrdID),
		OrderID:     bodyValue(value, message.TagOrderID),
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
		TraceID:       traceID,
		Profile:       profile,
		Direction:     direction,
		Raw:           event.DisplayMessage(),
		ValidationRaw: event.Raw(),
		SentAt:        sentAt,
		ReceivedAt:    receivedAt,
	})
}

func eventMsgType(event fixsession.Event) string {
	if event.MsgType != "" {
		return event.MsgType
	}
	return eventFields(event)[message.TagMsgType]
}

func eventFields(event fixsession.Event) map[int]string {
	parsed, err := trace.ParseRaw(event.Raw())
	if err != nil {
		return map[int]string{}
	}
	fields := make(map[int]string, len(parsed.Fields))
	for _, field := range parsed.Fields {
		fields[field.Tag] = field.Value
	}
	return fields
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

func newTraceID(command string) string {
	return fmt.Sprintf("%s-%d", command, time.Now().UTC().UnixNano())
}

func eventTime(event fixsession.Event) time.Time {
	if event.Time.IsZero() {
		return time.Now().UTC()
	}
	return event.Time
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
