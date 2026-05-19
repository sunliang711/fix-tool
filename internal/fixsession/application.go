package fixsession

import (
	"strconv"
	"strings"
	"time"

	"fix-tool/internal/config"

	"github.com/quickfixgo/quickfix"
)

const (
	defaultEventBuffer = 64
	msgTypeLogon       = "A"
	tagMsgType         = quickfix.Tag(35)
	tagUsername        = quickfix.Tag(553)
	tagPassword        = quickfix.Tag(554)
	soh                = "\x01"
	redactedValue      = "[REDACTED]"
)

type quickFIXApplication struct {
	events        chan Event
	username      string
	password      string
	logonTags     []config.LogonTagConfig
	sensitiveTags map[quickfix.Tag]struct{}
}

func NewApplication(profile config.ProfileConfig) Application {
	return newApplication(profile, make(chan Event, defaultEventBuffer))
}

func newApplication(profile config.ProfileConfig, events chan Event) *quickFIXApplication {
	return &quickFIXApplication{
		events:        events,
		username:      profile.Username,
		password:      profile.Password,
		logonTags:     profile.LogonTags,
		sensitiveTags: sensitiveTagsFromProfile(profile),
	}
}

func sensitiveTagsFromProfile(profile config.ProfileConfig) map[quickfix.Tag]struct{} {
	sensitiveTags := map[quickfix.Tag]struct{}{
		tagUsername: {},
		tagPassword: {},
	}
	for _, customFieldDef := range profile.CustomFieldDefs {
		if customFieldDef.Sensitive {
			sensitiveTags[quickfix.Tag(customFieldDef.Tag)] = struct{}{}
		}
	}
	return sensitiveTags
}

func (a *quickFIXApplication) Events() <-chan Event {
	return a.events
}

func (a *quickFIXApplication) OnCreate(sessionID quickfix.SessionID) {
	a.emit(Event{Type: EventCreate, SessionID: sessionID})
}

func (a *quickFIXApplication) OnLogon(sessionID quickfix.SessionID) {
	a.emit(Event{Type: EventLogon, SessionID: sessionID})
}

func (a *quickFIXApplication) OnLogout(sessionID quickfix.SessionID) {
	a.emit(Event{Type: EventLogout, SessionID: sessionID})
}

func (a *quickFIXApplication) ToAdmin(message *quickfix.Message, sessionID quickfix.SessionID) {
	if msgType(message) == msgTypeLogon {
		for _, logonTag := range a.logonTags {
			message.Body.SetString(quickfix.Tag(logonTag.Tag), logonTag.Value)
		}
		if a.username != "" {
			message.Body.SetString(tagUsername, a.username)
		}
		if a.password != "" {
			message.Body.SetString(tagPassword, a.password)
		}
	}
	a.emitMessage(EventToAdmin, message, sessionID)
}

func (a *quickFIXApplication) FromAdmin(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.emitMessage(EventFromAdmin, message, sessionID)
	return nil
}

func (a *quickFIXApplication) ToApp(message *quickfix.Message, sessionID quickfix.SessionID) error {
	a.emitMessage(EventToApp, message, sessionID)
	return nil
}

func (a *quickFIXApplication) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.emitMessage(EventFromApp, message, sessionID)
	return nil
}

func (a *quickFIXApplication) emitMessage(eventType EventType, message *quickfix.Message, sessionID quickfix.SessionID) {
	a.emit(Event{
		Type:      eventType,
		SessionID: sessionID,
		MsgType:   msgType(message),
		Message:   redactFIXMessage(message.String(), a.sensitiveTags),
	})
}

func (a *quickFIXApplication) emit(event Event) {
	event.Time = time.Now().UTC()
	select {
	case a.events <- event:
	default:
	}
}

func msgType(message *quickfix.Message) string {
	value, err := message.Header.GetString(tagMsgType)
	if err != nil {
		return ""
	}
	return value
}

func redactFIXMessage(raw string, sensitiveTags map[quickfix.Tag]struct{}) string {
	if raw == "" {
		return ""
	}
	fields := strings.Split(raw, soh)
	for i, field := range fields {
		eq := strings.IndexByte(field, '=')
		if eq <= 0 {
			continue
		}
		tag, err := strconv.Atoi(field[:eq])
		if err != nil {
			continue
		}
		if _, ok := sensitiveTags[quickfix.Tag(tag)]; ok {
			fields[i] = field[:eq+1] + redactedValue
		}
	}
	return strings.Join(fields, soh)
}
