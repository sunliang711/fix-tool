package fixsession

import (
	"strings"
	"testing"
	"time"

	"fix-tool/internal/config"

	"github.com/quickfixgo/quickfix"
)

func TestApplicationCapturesCallbacks(t *testing.T) {
	profile := validProfile()
	profile.Username = "account"
	profile.Password = "secret"
	profile.CustomFieldDefs = []config.CustomFieldDefConfig{
		{Tag: 9001, Name: "Token", Type: "STRING", Sensitive: true},
	}
	profile.LogonTags = []config.LogonTagConfig{
		{Tag: 9002, Value: "ALPHA"},
		{Tag: 9003, Value: "BETA"},
	}
	app := NewApplication(profile)
	sessionID := quickfix.SessionID{
		BeginString:  profile.BeginString,
		SenderCompID: profile.SenderCompID,
		TargetCompID: profile.TargetCompID,
	}

	logon := quickfix.NewMessage()
	logon.Header.SetString(tagMsgType, msgTypeLogon)
	logon.Body.SetString(quickfix.Tag(9001), "token-value")
	app.ToAdmin(logon, sessionID)
	event := assertCapturedEvent(t, app.Events(), EventToAdmin)
	if strings.Contains(event.Message, "token-value") {
		t.Fatalf("event message contains sensitive custom field def: %q", event.Message)
	}

	username, err := logon.Body.GetString(tagUsername)
	if err != nil {
		t.Fatalf("username field error = %v", err)
	}
	if username != "account" {
		t.Fatalf("username = %q, want %q", username, "account")
	}
	password, err := logon.Body.GetString(tagPassword)
	if err != nil {
		t.Fatalf("password field error = %v", err)
	}
	if password != "secret" {
		t.Fatalf("password = %q, want %q", password, "secret")
	}
	logonTag, err := logon.Body.GetString(quickfix.Tag(9002))
	if err != nil {
		t.Fatalf("logon tag field error = %v", err)
	}
	if logonTag != "ALPHA" {
		t.Fatalf("logon tag = %q, want ALPHA", logonTag)
	}
	secondLogonTag, err := logon.Body.GetString(quickfix.Tag(9003))
	if err != nil {
		t.Fatalf("second logon tag field error = %v", err)
	}
	if secondLogonTag != "BETA" {
		t.Fatalf("second logon tag = %q, want BETA", secondLogonTag)
	}

	app.OnLogon(sessionID)
	assertCapturedEvent(t, app.Events(), EventLogon)
	app.OnLogout(sessionID)
	assertCapturedEvent(t, app.Events(), EventLogout)
	if err := app.ToApp(messageWithType("D"), sessionID); err != nil {
		t.Fatalf("ToApp() error = %v", err)
	}
	assertCapturedEvent(t, app.Events(), EventToApp)
	if reject := app.FromAdmin(messageWithType("0"), sessionID); reject != nil {
		t.Fatalf("FromAdmin() reject = %v", reject)
	}
	assertCapturedEvent(t, app.Events(), EventFromAdmin)
	if reject := app.FromApp(messageWithType("8"), sessionID); reject != nil {
		t.Fatalf("FromApp() reject = %v", reject)
	}
	assertCapturedEvent(t, app.Events(), EventFromApp)
}

func TestApplicationRedactsSensitiveEventMessage(t *testing.T) {
	profile := validProfile()
	profile.Username = "account"
	profile.Password = "secret"
	app := NewApplication(profile)
	sessionID := quickfix.SessionID{
		BeginString:  profile.BeginString,
		SenderCompID: profile.SenderCompID,
		TargetCompID: profile.TargetCompID,
	}

	logon := quickfix.NewMessage()
	logon.Header.SetString(tagMsgType, msgTypeLogon)
	app.ToAdmin(logon, sessionID)

	event := assertCapturedEvent(t, app.Events(), EventToAdmin)
	if strings.Contains(event.Message, "account") {
		t.Fatalf("event message contains username: %q", event.Message)
	}
	if strings.Contains(event.Message, "secret") {
		t.Fatalf("event message contains password: %q", event.Message)
	}
	if !strings.Contains(event.Message, "553="+redactedValue) {
		t.Fatalf("event message = %q, want redacted username", event.Message)
	}
	if !strings.Contains(event.Message, "554="+redactedValue) {
		t.Fatalf("event message = %q, want redacted password", event.Message)
	}
}

func messageWithType(value string) *quickfix.Message {
	message := quickfix.NewMessage()
	message.Header.SetString(tagMsgType, value)
	return message
}

func assertCapturedEvent(t *testing.T, events <-chan Event, want EventType) Event {
	t.Helper()
	select {
	case event := <-events:
		if event.Type != want {
			t.Fatalf("event type = %s, want %s", event.Type, want)
		}
		if event.Time.IsZero() {
			t.Fatal("event time is zero")
		}
		return event
	case <-time.After(time.Second):
		t.Fatalf("event %s was not captured", want)
		return Event{}
	}
}
