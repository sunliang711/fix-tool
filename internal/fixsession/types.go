package fixsession

import (
	"context"
	"time"

	"github.com/quickfixgo/quickfix"
)

type EventType string

const (
	EventLogon     EventType = "Logon"
	EventLogout    EventType = "Logout"
	EventToAdmin   EventType = "ToAdmin"
	EventFromAdmin EventType = "FromAdmin"
	EventToApp     EventType = "ToApp"
	EventFromApp   EventType = "FromApp"
	EventCreate    EventType = "Create"
)

type Event struct {
	Type       EventType
	SessionID  quickfix.SessionID
	MsgType    string
	Message    string
	RawMessage string
	Time       time.Time
}

func (e Event) Raw() string {
	if e.RawMessage != "" {
		return e.RawMessage
	}
	return e.Message
}

func (e Event) DisplayMessage() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Raw()
}

type Manager interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Events() <-chan Event
	Session() Session
}

type Session interface {
	ID() quickfix.SessionID
	ProfileName() string
	Send(message *quickfix.Message) error
}

type Application interface {
	quickfix.Application
	Events() <-chan Event
}

type session struct {
	id          quickfix.SessionID
	profileName string
}

func (s session) ID() quickfix.SessionID {
	return s.id
}

func (s session) ProfileName() string {
	return s.profileName
}

func (s session) Send(message *quickfix.Message) error {
	return quickfix.SendToTarget(message, s.id)
}
