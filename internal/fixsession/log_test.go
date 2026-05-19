package fixsession

import (
	"bytes"
	"strings"
	"testing"

	"github.com/quickfixgo/quickfix"
	"github.com/rs/zerolog"
)

func TestZerologLogFactoryLogsQuickFIXEvents(t *testing.T) {
	var out bytes.Buffer
	logger := zerolog.New(&out).Level(zerolog.DebugLevel)
	logFactory := newZerologLogFactory(logger, validProfile())
	sessionID := quickfix.SessionID{
		BeginString:  "FIX.4.4",
		SenderCompID: "SENDER",
		TargetCompID: "TARGET",
	}
	log, err := logFactory.CreateSessionLog(sessionID)
	if err != nil {
		t.Fatalf("CreateSessionLog() error = %v", err)
	}

	log.OnEvent("Failed to connect: dial tcp 127.0.0.1:9876")

	got := out.String()
	if !strings.Contains(got, "Failed to connect") {
		t.Fatalf("log output = %q, want quickfix event", got)
	}
	if !strings.Contains(got, sessionID.String()) {
		t.Fatalf("log output = %q, want session", got)
	}
}

func TestZerologLogFactoryRedactsQuickFIXEventMessages(t *testing.T) {
	var out bytes.Buffer
	logger := zerolog.New(&out).Level(zerolog.DebugLevel)
	profile := validProfile()
	profile.Username = "account"
	profile.Password = "secret"
	logFactory := newZerologLogFactory(logger, profile)
	log, err := logFactory.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	message := quickfix.NewMessage()
	message.Header.SetString(tagMsgType, msgTypeLogon)
	message.Body.SetString(tagUsername, profile.Username)
	message.Body.SetString(tagPassword, profile.Password)

	log.OnEvent("Invalid Session State: Received Msg " + message.String())

	got := out.String()
	if strings.Contains(got, profile.Username) {
		t.Fatalf("log output contains username: %q", got)
	}
	if strings.Contains(got, profile.Password) {
		t.Fatalf("log output contains password: %q", got)
	}
	if strings.Contains(got, "\x01") {
		t.Fatalf("log output contains SOH delimiter: %q", got)
	}
	if !strings.Contains(got, "<FIX message>") {
		t.Fatalf("log output = %q, want embedded message omitted", got)
	}
	if strings.Contains(got, "553=") || strings.Contains(got, "554=") {
		t.Fatalf("log output contains embedded FIX fields: %q", got)
	}
}

func TestZerologLogFactorySummarizesEmbeddedFIXEventMessage(t *testing.T) {
	var out bytes.Buffer
	logger := zerolog.New(&out).Level(zerolog.DebugLevel)
	logFactory := newZerologLogFactory(logger, validProfile())
	log, err := logFactory.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	raw := "8=FIX.4.4|9=79|35=5|34=1|49=BROKER01|52=20260519-08:36:57.014|56=CLIENT01|58=missing username|10=087|"

	log.OnEvent("Invalid Session State: Received Msg " + raw + " while waiting for Logon")

	got := out.String()
	for _, want := range []string{
		"Invalid Session State: Received Msg <FIX message> while waiting for Logon",
		`"last_msg_type":"5"`,
		`"last_msg_name":"Logout"`,
		`"last_text":"missing username"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("log output = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, raw) {
		t.Fatalf("log output = %q, want embedded raw omitted", got)
	}
	if strings.Contains(got, "pretty_message") {
		t.Fatalf("log output = %q, want no pretty message on quickfix event", got)
	}
}

func TestZerologLogFactoryRedactsQuickFIXMessages(t *testing.T) {
	var out bytes.Buffer
	logger := zerolog.New(&out).Level(zerolog.DebugLevel)
	profile := validProfile()
	profile.Username = "account"
	profile.Password = "secret"
	logFactory := newZerologLogFactory(logger, profile)
	log, err := logFactory.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	message := quickfix.NewMessage()
	message.Header.SetString(tagMsgType, msgTypeLogon)
	message.Body.SetString(tagUsername, profile.Username)
	message.Body.SetString(tagPassword, profile.Password)

	log.OnOutgoing([]byte(message.String()))

	got := out.String()
	if strings.Contains(got, profile.Username) {
		t.Fatalf("log output contains username: %q", got)
	}
	if strings.Contains(got, profile.Password) {
		t.Fatalf("log output contains password: %q", got)
	}
	if !strings.Contains(got, redactedValue) {
		t.Fatalf("log output = %q, want redacted value", got)
	}
	if !strings.Contains(got, "pretty_message") {
		t.Fatalf("log output = %q, want pretty message", got)
	}
	if !strings.Contains(got, "raw_message") {
		t.Fatalf("log output = %q, want raw message at debug level", got)
	}
	for _, want := range []string{
		"-> Logon(A)",
		"35(MsgType:Logon)=A|",
		"35=A|",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("log output = %q, want %q", got, want)
		}
	}
}

func TestZerologLogFactoryLogsServerToClientMessages(t *testing.T) {
	var out bytes.Buffer
	logger := zerolog.New(&out).Level(zerolog.InfoLevel)
	logFactory := newZerologLogFactory(logger, validProfile())
	log, err := logFactory.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	message := quickfix.NewMessage()
	message.Header.SetString(tagMsgType, "5")
	message.Body.SetString(quickfix.Tag(58), "missing username")

	log.OnIncoming([]byte(message.String()))

	got := out.String()
	for _, want := range []string{
		"<- Logout(5)",
		`"reason":"missing username"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("log output = %q, want %q", got, want)
		}
	}
	for _, unwanted := range []string{"raw_message", "pretty_message", `"direction":"in"`, `"msg_code":"5"`, `"msg_type":"Logout"`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("log output = %q, want no %q at info level", got, unwanted)
		}
	}
}

func TestZerologLogFactoryLogsRawMessagesAtDebugLevel(t *testing.T) {
	var out bytes.Buffer
	logger := zerolog.New(&out).Level(zerolog.DebugLevel)
	logFactory := newZerologLogFactory(logger, validProfile())
	log, err := logFactory.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	message := quickfix.NewMessage()
	message.Header.SetString(tagMsgType, "5")
	message.Body.SetString(quickfix.Tag(58), "missing username")

	log.OnIncoming([]byte(message.String()))

	got := out.String()
	for _, want := range []string{
		"<- Logout(5)",
		"raw_message",
		"pretty_message",
		"35=5|",
		"35(MsgType:Logout)=5|",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("log output = %q, want %q", got, want)
		}
	}
}
