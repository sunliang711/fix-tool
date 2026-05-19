package order

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"fix-tool/internal/fixsession"
	"fix-tool/internal/message"

	"github.com/quickfixgo/quickfix"
)

func TestServiceSendsNewOrderAndMatchesExecutionReport(t *testing.T) {
	manager := newFakeManager()
	manager.onStart = func() {
		manager.emit(fixsession.Event{Type: fixsession.EventLogon})
	}
	manager.session.onSend = func(value *quickfix.Message) {
		msgType := testHeaderValue(value, message.TagMsgType)
		manager.emit(fixsession.Event{
			Type:    fixsession.EventToApp,
			MsgType: msgType,
			Message: rawFromMessage(value),
		})
		manager.emit(fixsession.Event{
			Type:    fixsession.EventFromApp,
			MsgType: message.MsgTypeExecutionReport,
			Message: rawFIX(message.MsgTypeExecutionReport, map[int]string{
				message.TagClOrdID: "OTHER",
				message.TagOrderID: "O-OTHER",
			}),
		})
		manager.emit(fixsession.Event{
			Type:    fixsession.EventFromApp,
			MsgType: message.MsgTypeExecutionReport,
			Message: rawFIX(message.MsgTypeExecutionReport, map[int]string{
				message.TagClOrdID: "C001",
				message.TagOrderID: "O001",
			}),
		})
	}
	service := NewService(manager, Options{Timeout: time.Second})

	result, err := service.NewOrder(context.Background(), NewRequest{
		ClOrdID:  "C001",
		Symbol:   "AAPL",
		Side:     "buy",
		OrderQty: "100",
		Price:    "10.25",
	})
	if err != nil {
		t.Fatalf("NewOrder() error = %v", err)
	}
	if manager.starts != 1 {
		t.Fatalf("starts = %d, want 1", manager.starts)
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
	if len(manager.session.sent) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(manager.session.sent))
	}
	if manager.session.sent[0].msgType != message.MsgTypeNewOrderSingle {
		t.Fatalf("sent msg type = %q, want %q", manager.session.sent[0].msgType, message.MsgTypeNewOrderSingle)
	}
	if result.Request == nil || result.Request.MsgType != message.MsgTypeNewOrderSingle {
		t.Fatalf("request trace = %#v, want new order", result.Request)
	}
	if result.Response == nil || result.Response.ClOrdID != "C001" {
		t.Fatalf("response trace = %#v, want related execution report", result.Response)
	}
}

func TestMatchResponse(t *testing.T) {
	request := requestIdentity{
		MsgType:     message.MsgTypeOrderCancelReplaceRequest,
		ClOrdID:     "C002",
		OrigClOrdID: "C001",
		OrderID:     "O001",
	}

	tests := []struct {
		name    string
		event   fixsession.Event
		matches bool
	}{
		{
			name: "execution-report-by-clordid",
			event: inbound(message.MsgTypeExecutionReport, map[int]string{
				message.TagClOrdID: "C002",
			}),
			matches: true,
		},
		{
			name: "execution-report-by-orig-clordid",
			event: inbound(message.MsgTypeExecutionReport, map[int]string{
				message.TagOrigClOrdID: "C001",
			}),
			matches: true,
		},
		{
			name: "execution-report-by-order-id",
			event: inbound(message.MsgTypeExecutionReport, map[int]string{
				message.TagOrderID: "O001",
			}),
			matches: true,
		},
		{
			name: "unrelated-execution-report",
			event: inbound(message.MsgTypeExecutionReport, map[int]string{
				message.TagClOrdID: "OTHER",
			}),
			matches: false,
		},
		{
			name: "orig-clordid-does-not-match-new-clordid",
			event: inbound(message.MsgTypeExecutionReport, map[int]string{
				message.TagOrigClOrdID: "C002",
			}),
			matches: false,
		},
		{
			name:    "uncorrelated-reject",
			event:   inbound(message.MsgTypeReject, nil),
			matches: true,
		},
		{
			name:    "uncorrelated-business-message-reject",
			event:   inbound(message.MsgTypeBusinessMessageReject, nil),
			matches: true,
		},
		{
			name: "unrelated-correlated-reject",
			event: inbound(message.MsgTypeReject, map[int]string{
				message.TagClOrdID: "OTHER",
			}),
			matches: false,
		},
		{
			name: "outbound-event",
			event: fixsession.Event{
				Type:    fixsession.EventToApp,
				MsgType: message.MsgTypeExecutionReport,
				Message: rawFIX(message.MsgTypeExecutionReport, map[int]string{message.TagClOrdID: "C002"}),
			},
			matches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchResponse(request, tt.event); got != tt.matches {
				t.Fatalf("matchResponse() = %t, want %t", got, tt.matches)
			}
		})
	}
}

func TestServiceReturnsChineseRequiredErrorBeforeStart(t *testing.T) {
	manager := newFakeManager()
	service := NewService(manager, Options{Timeout: time.Second})

	_, err := service.NewOrder(context.Background(), NewRequest{
		Side:     "buy",
		OrderQty: "100",
		Price:    "10.25",
	})
	if err == nil || !strings.Contains(err.Error(), "缺少必填参数 --symbol") {
		t.Fatalf("NewOrder() error = %v, want Chinese required error", err)
	}
	if manager.starts != 0 {
		t.Fatalf("starts = %d, want 0", manager.starts)
	}
}

func TestServiceReturnsTimeout(t *testing.T) {
	manager := newFakeManager()
	manager.onStart = func() {
		manager.emit(fixsession.Event{Type: fixsession.EventLogon})
	}
	manager.session.onSend = func(value *quickfix.Message) {
		msgType := testHeaderValue(value, message.TagMsgType)
		manager.emit(fixsession.Event{
			Type:    fixsession.EventToApp,
			MsgType: msgType,
			Message: rawFromMessage(value),
		})
	}
	service := NewService(manager, Options{Timeout: 20 * time.Millisecond})

	_, err := service.CancelOrder(context.Background(), CancelRequest{
		OrigClOrdID: "C001",
		ClOrdID:     "C002",
		Symbol:      "AAPL",
		Side:        "sell",
	})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("CancelOrder() error = %v, want ErrTimeout", err)
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
}

type fakeManager struct {
	events  chan fixsession.Event
	session *fakeSession
	onStart func()
	starts  int
	stops   int
}

func newFakeManager() *fakeManager {
	sessionID := quickfix.SessionID{
		BeginString:  "FIX.4.4",
		SenderCompID: "SENDER",
		TargetCompID: "TARGET",
	}
	return &fakeManager{
		events: make(chan fixsession.Event, 16),
		session: &fakeSession{
			id:          sessionID,
			profileName: "test",
		},
	}
}

func (m *fakeManager) Start(context.Context) error {
	m.starts++
	if m.onStart != nil {
		m.onStart()
	}
	return nil
}

func (m *fakeManager) Stop(context.Context) error {
	m.stops++
	return nil
}

func (m *fakeManager) Events() <-chan fixsession.Event {
	return m.events
}

func (m *fakeManager) Session() fixsession.Session {
	return m.session
}

func (m *fakeManager) emit(event fixsession.Event) {
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	m.events <- event
}

type fakeSession struct {
	id          quickfix.SessionID
	profileName string
	sent        []sentMessage
	onSend      func(*quickfix.Message)
}

type sentMessage struct {
	msgType string
	clOrdID string
}

func (s *fakeSession) ID() quickfix.SessionID {
	return s.id
}

func (s *fakeSession) ProfileName() string {
	return s.profileName
}

func (s *fakeSession) Send(value *quickfix.Message) error {
	s.sent = append(s.sent, sentMessage{
		msgType: testHeaderValue(value, message.TagMsgType),
		clOrdID: testBodyValue(value, message.TagClOrdID),
	})
	if s.onSend != nil {
		s.onSend(value)
	}
	return nil
}

func inbound(msgType string, fields map[int]string) fixsession.Event {
	return fixsession.Event{
		Type:    fixsession.EventFromApp,
		MsgType: msgType,
		Message: rawFIX(msgType, fields),
	}
}

func rawFromMessage(value *quickfix.Message) string {
	fields := make(map[int]string)
	for _, tagValue := range value.Body.Tags() {
		fields[int(tagValue)] = testBodyValue(value, int(tagValue))
	}
	return rawFIX(testHeaderValue(value, message.TagMsgType), fields)
}

func rawFIX(msgType string, fields map[int]string) string {
	value := quickfix.NewMessage()
	value.Header.SetString(quickfix.Tag(message.TagBeginString), "FIX.4.4")
	value.Header.SetString(quickfix.Tag(message.TagMsgType), msgType)
	value.Header.SetString(quickfix.Tag(34), "1")
	value.Header.SetString(quickfix.Tag(49), "SENDER")
	value.Header.SetString(quickfix.Tag(52), time.Now().UTC().Format("20060102-15:04:05.000"))
	value.Header.SetString(quickfix.Tag(56), "TARGET")
	for tagValue, fieldValue := range fields {
		value.Body.SetString(quickfix.Tag(tagValue), fieldValue)
	}
	return value.String()
}

func testHeaderValue(value *quickfix.Message, tagValue int) string {
	result, err := value.Header.GetString(quickfix.Tag(tagValue))
	if err != nil {
		return ""
	}
	return result
}

func testBodyValue(value *quickfix.Message, tagValue int) string {
	result, err := value.Body.GetString(quickfix.Tag(tagValue))
	if err != nil {
		return ""
	}
	return result
}
