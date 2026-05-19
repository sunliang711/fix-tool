package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"fix-tool/internal/fixsession"
	"fix-tool/internal/trace"

	"github.com/quickfixgo/quickfix"
)

func TestServiceLogonWaitsForEvent(t *testing.T) {
	manager := newFakeManager()
	manager.onStart = func() {
		manager.emit(fixsession.Event{Type: fixsession.EventToAdmin, MsgType: msgTypeLogon, Message: rawAdmin(msgTypeLogon, "")})
		manager.emit(fixsession.Event{Type: fixsession.EventFromAdmin, MsgType: msgTypeLogon, Message: rawAdmin(msgTypeLogon, "")})
		manager.emit(fixsession.Event{Type: fixsession.EventLogon})
	}
	service := NewService(manager, Options{Timeout: time.Second})

	result, err := service.Logon(context.Background())
	if err != nil {
		t.Fatalf("Logon() error = %v", err)
	}
	if manager.starts != 1 {
		t.Fatalf("starts = %d, want 1", manager.starts)
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
	if result.Request == nil || result.Request.MsgType != msgTypeLogon {
		t.Fatalf("request trace = %#v, want logon", result.Request)
	}
	if result.Response == nil || result.Response.MsgType != msgTypeLogon {
		t.Fatalf("response trace = %#v, want logon", result.Response)
	}
	if len(manager.session.sent) != 0 {
		t.Fatalf("sent messages = %d, want 0", len(manager.session.sent))
	}
}

func TestServiceSendsAdminMessages(t *testing.T) {
	tests := []struct {
		name            string
		run             func(context.Context, *Service) (Result, error)
		wantMsgType     string
		wantResponse    string
		wantTestReqID   string
		responseTestReq string
	}{
		{
			name: "logout",
			run: func(ctx context.Context, service *Service) (Result, error) {
				return service.Logout(ctx)
			},
			wantMsgType:  msgTypeLogout,
			wantResponse: msgTypeLogout,
		},
		{
			name: "heartbeat",
			run: func(ctx context.Context, service *Service) (Result, error) {
				return service.Heartbeat(ctx)
			},
			wantMsgType:  msgTypeHeartbeat,
			wantResponse: msgTypeHeartbeat,
		},
		{
			name: "test-request",
			run: func(ctx context.Context, service *Service) (Result, error) {
				return service.TestRequest(ctx, "ping-001")
			},
			wantMsgType:     msgTypeTestRequest,
			wantResponse:    msgTypeHeartbeat,
			wantTestReqID:   "ping-001",
			responseTestReq: "ping-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newFakeManager()
			manager.onStart = func() {
				manager.emit(fixsession.Event{Type: fixsession.EventLogon})
			}
			manager.session.onSend = func(message *quickfix.Message) {
				msgType := mustHeaderValue(t, message, tagMsgType)
				testReqID := bodyValue(message, tagTestReqID)
				manager.emit(fixsession.Event{
					Type:    fixsession.EventToAdmin,
					MsgType: msgType,
					Message: rawAdmin(msgType, testReqID),
				})
				manager.emit(fixsession.Event{
					Type:    fixsession.EventFromAdmin,
					MsgType: tt.wantResponse,
					Message: rawAdmin(tt.wantResponse, tt.responseTestReq),
				})
			}
			service := NewService(manager, Options{Timeout: time.Second})

			result, err := tt.run(context.Background(), service)
			if err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
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
			if manager.session.sent[0].msgType != tt.wantMsgType {
				t.Fatalf("sent msg type = %q, want %q", manager.session.sent[0].msgType, tt.wantMsgType)
			}
			if manager.session.sent[0].testReqID != tt.wantTestReqID {
				t.Fatalf("sent test req id = %q, want %q", manager.session.sent[0].testReqID, tt.wantTestReqID)
			}
			if result.Request == nil || result.Request.MsgType != tt.wantMsgType {
				t.Fatalf("request trace = %#v, want %s", result.Request, tt.wantMsgType)
			}
			if result.Response == nil || result.Response.MsgType != tt.wantResponse {
				t.Fatalf("response trace = %#v, want %s", result.Response, tt.wantResponse)
			}
			if tt.responseTestReq != "" && firstField(result.Response.Fields, int(tagTestReqID)) != tt.responseTestReq {
				t.Fatalf("response test req id = %q, want %q", firstField(result.Response.Fields, int(tagTestReqID)), tt.responseTestReq)
			}
		})
	}
}

func TestServiceReturnsTimeout(t *testing.T) {
	manager := newFakeManager()
	manager.onStart = func() {
		manager.emit(fixsession.Event{Type: fixsession.EventLogon})
	}
	manager.session.onSend = func(message *quickfix.Message) {
		msgType := mustHeaderValue(t, message, tagMsgType)
		manager.emit(fixsession.Event{Type: fixsession.EventToAdmin, MsgType: msgType, Message: rawAdmin(msgType, "")})
	}
	service := NewService(manager, Options{Timeout: 20 * time.Millisecond})

	_, err := service.Heartbeat(context.Background())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Heartbeat() error = %v, want ErrTimeout", err)
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
}

func TestServiceKeepSessionReusesLoggedOnState(t *testing.T) {
	manager := newFakeManager()
	manager.onStart = func() {
		if manager.starts == 1 {
			manager.emit(fixsession.Event{Type: fixsession.EventToAdmin, MsgType: msgTypeLogon, Message: rawAdmin(msgTypeLogon, "")})
			manager.emit(fixsession.Event{Type: fixsession.EventFromAdmin, MsgType: msgTypeLogon, Message: rawAdmin(msgTypeLogon, "")})
			manager.emit(fixsession.Event{Type: fixsession.EventLogon})
		}
	}
	manager.session.onSend = func(message *quickfix.Message) {
		msgType := mustHeaderValue(t, message, tagMsgType)
		manager.emit(fixsession.Event{Type: fixsession.EventToAdmin, MsgType: msgType, Message: rawAdmin(msgType, "")})
		manager.emit(fixsession.Event{Type: fixsession.EventFromAdmin, MsgType: msgType, Message: rawAdmin(msgType, "")})
	}
	service := NewService(manager, Options{Timeout: time.Second, KeepSession: true})

	if _, err := service.Logon(context.Background()); err != nil {
		t.Fatalf("Logon() error = %v", err)
	}
	if _, err := service.Heartbeat(context.Background()); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if manager.starts != 2 {
		t.Fatalf("starts = %d, want 2", manager.starts)
	}
	if manager.stops != 0 {
		t.Fatalf("stops = %d, want 0", manager.stops)
	}
}

func TestServiceRequiresTestRequestID(t *testing.T) {
	service := NewService(newFakeManager(), Options{Timeout: time.Second})

	_, err := service.TestRequest(context.Background(), "")
	if !errors.Is(err, ErrTestRequestIDRequired) {
		t.Fatalf("TestRequest() error = %v, want ErrTestRequestIDRequired", err)
	}
}

func TestServiceRejectsSOHInTestRequestID(t *testing.T) {
	service := NewService(newFakeManager(), Options{Timeout: time.Second})

	_, err := service.TestRequest(context.Background(), "ping"+soh+"35=D")
	if !errors.Is(err, ErrTestRequestIDInvalid) {
		t.Fatalf("TestRequest() error = %v, want ErrTestRequestIDInvalid", err)
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
	msgType   string
	testReqID string
}

func (s *fakeSession) ID() quickfix.SessionID {
	return s.id
}

func (s *fakeSession) ProfileName() string {
	return s.profileName
}

func (s *fakeSession) Send(message *quickfix.Message) error {
	s.sent = append(s.sent, sentMessage{
		msgType:   mustHeaderValue(nil, message, tagMsgType),
		testReqID: bodyValue(message, tagTestReqID),
	})
	if s.onSend != nil {
		s.onSend(message)
	}
	return nil
}

func rawAdmin(msgType string, testReqID string) string {
	message := quickfix.NewMessage()
	message.Header.SetString(quickfix.Tag(8), "FIX.4.4")
	message.Header.SetString(tagMsgType, msgType)
	message.Header.SetString(quickfix.Tag(34), "1")
	message.Header.SetString(quickfix.Tag(49), "SENDER")
	message.Header.SetString(quickfix.Tag(52), time.Now().UTC().Format("20060102-15:04:05.000"))
	message.Header.SetString(quickfix.Tag(56), "TARGET")
	if testReqID != "" {
		message.Body.SetString(tagTestReqID, testReqID)
	}
	return message.String()
}

func mustHeaderValue(t *testing.T, message *quickfix.Message, tag quickfix.Tag) string {
	value, err := message.Header.GetString(tag)
	if err != nil {
		if t != nil {
			t.Fatalf("header tag %d error = %v", tag, err)
		}
		return ""
	}
	return value
}

func bodyValue(message *quickfix.Message, tag quickfix.Tag) string {
	value, err := message.Body.GetString(tag)
	if err != nil {
		return ""
	}
	return value
}

func firstField(fields []trace.Field, tag int) string {
	for _, field := range fields {
		if field.Tag == tag {
			return field.Value
		}
	}
	return ""
}
