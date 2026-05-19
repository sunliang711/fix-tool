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
	Type      EventType
	SessionID quickfix.SessionID
	MsgType   string
	Message   string
	Time      time.Time
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
