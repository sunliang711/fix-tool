package fixsession

import (
	"fmt"
	"strings"

	"fix-tool/internal/config"
	"fix-tool/internal/dictionary"
	"fix-tool/internal/trace"

	"github.com/quickfixgo/quickfix"
	"github.com/rs/zerolog"
)

const (
	directionClientToServer = "client_to_server"
	directionServerToClient = "server_to_client"
)

type zerologLogFactory struct {
	logger        zerolog.Logger
	sensitiveTags map[quickfix.Tag]struct{}
	dictionary    *dictionary.Dictionary
}

type zerologLog struct {
	logger        zerolog.Logger
	session       string
	sensitiveTags map[quickfix.Tag]struct{}
	dictionary    *dictionary.Dictionary
}

func newZerologLogFactory(logger zerolog.Logger, profile config.ProfileConfig) quickfix.LogFactory {
	return zerologLogFactory{
		logger:        logger,
		sensitiveTags: sensitiveTagsFromProfile(profile),
		dictionary:    dictionary.NewFromConfig(profile.CustomFieldDefs),
	}
}

func (f zerologLogFactory) Create() (quickfix.Log, error) {
	return zerologLog{
		logger:        f.logger,
		sensitiveTags: f.sensitiveTags,
		dictionary:    f.dictionary,
	}, nil
}

func (f zerologLogFactory) CreateSessionLog(sessionID quickfix.SessionID) (quickfix.Log, error) {
	return zerologLog{
		logger:        f.logger,
		session:       sessionID.String(),
		sensitiveTags: f.sensitiveTags,
		dictionary:    f.dictionary,
	}, nil
}

func (l zerologLog) OnIncoming(message []byte) {
	l.logMessage(directionServerToClient, message)
}

func (l zerologLog) OnOutgoing(message []byte) {
	l.logMessage(directionClientToServer, message)
}

func (l zerologLog) OnEvent(message string) {
	message = trace.DisplayRaw(redactFIXMessage(message, l.sensitiveTags), "|")
	event := l.logger.Info()
	if l.isWarningEvent(message) {
		event = l.logger.Warn()
	}
	if l.session != "" {
		event = event.Str("session", l.session)
	}
	if embedded, ok := l.embeddedFIXMessage(message); ok {
		event = l.appendMessageSummary(event, embedded)
		message = l.omitEmbeddedFIXMessage(message, embedded)
	}
	event.Str("source", "quickfix").Msg(message)
}

func (l zerologLog) OnEventf(format string, values ...interface{}) {
	l.OnEvent(fmt.Sprintf(format, values...))
}

func (l zerologLog) logMessage(direction string, message []byte) {
	raw := redactFIXMessage(string(message), l.sensitiveTags)
	event := l.logger.Info().
		Str("source", "quickfix").
		Str("direction", direction).
		Str("raw_message", trace.DisplayRaw(raw, "|")).
		Str("pretty_message", l.prettyFIXMessage(raw))
	if l.session != "" {
		event = event.Str("session", l.session)
	}
	if msgType := rawValue(raw, int(tagMsgType)); msgType != "" {
		event = event.Str("msg_type", msgType)
		if l.dictionary != nil {
			msgName, ok := l.dictionary.ExplainValue(int(tagMsgType), msgType)
			if ok {
				event = event.Str("msg_name", msgName)
			}
		}
	}
	event.Msg(l.messageLogTitle(direction))
}

func (l zerologLog) isWarningEvent(message string) bool {
	return containsAny(message, []string{
		"Failed",
		"Invalid",
		"Rejected",
		"Reject",
		"Timeout",
		"timeout",
		"error",
		"disconnected",
		"Disconnected",
	})
}

func containsAny(value string, parts []string) bool {
	for _, part := range parts {
		if part != "" && contains(value, part) {
			return true
		}
	}
	return false
}

func contains(value string, part string) bool {
	return strings.Contains(value, part)
}

func rawValue(raw string, tag int) string {
	parsed, err := trace.ParseRaw(raw)
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

func (l zerologLog) appendMessageSummary(event *zerolog.Event, raw string) *zerolog.Event {
	parsed, err := trace.ParseRaw(raw)
	if err != nil {
		return event
	}
	msgType := firstParsedValue(parsed.Fields, int(tagMsgType))
	if msgType != "" {
		event = event.Str("last_msg_type", msgType)
		if l.dictionary != nil {
			if msgName, ok := l.dictionary.ExplainValue(int(tagMsgType), msgType); ok {
				event = event.Str("last_msg_name", msgName)
			}
		}
	}
	if text := firstParsedValue(parsed.Fields, 58); text != "" {
		event = event.Str("last_text", text)
	}
	return event
}

func firstParsedValue(fields []trace.Field, tag int) string {
	for _, field := range fields {
		if field.Tag == tag {
			return field.Value
		}
	}
	return ""
}

func (l zerologLog) messageLogTitle(direction string) string {
	switch direction {
	case directionClientToServer:
		return "FIX client -> server"
	case directionServerToClient:
		return "FIX server -> client"
	default:
		return "FIX message"
	}
}

func (l zerologLog) embeddedFIXMessage(message string) (string, bool) {
	const startMarker = "Received Msg "
	start := strings.Index(message, startMarker)
	if start < 0 {
		return "", false
	}
	start += len(startMarker)
	end := strings.Index(message[start:], " while ")
	if end < 0 {
		end = len(message)
	} else {
		end += start
	}
	raw := strings.TrimSpace(message[start:end])
	if _, err := trace.ParseRaw(raw); err != nil {
		return "", false
	}
	return raw, true
}

func (l zerologLog) omitEmbeddedFIXMessage(message string, raw string) string {
	return strings.Replace(message, raw, "<FIX message>", 1)
}

func (l zerologLog) prettyFIXMessage(raw string) string {
	parsed, err := trace.ParseRaw(raw)
	if err != nil {
		return trace.DisplayRaw(raw, "|")
	}
	var builder strings.Builder
	for _, field := range parsed.Fields {
		value := field.Value
		if _, ok := l.sensitiveTags[quickfix.Tag(field.Tag)]; ok {
			value = redactedValue
		}
		builder.WriteString(fmt.Sprintf("%d(%s)=%s|", field.Tag, l.prettyFieldName(field), value))
	}
	return builder.String()
}

func (l zerologLog) prettyFieldName(field trace.Field) string {
	if l.dictionary == nil {
		return fmt.Sprintf("Tag%d", field.Tag)
	}
	definition, ok := l.dictionary.Lookup(field.Tag)
	if !ok {
		return fmt.Sprintf("Tag%d", field.Tag)
	}
	if enum, ok := l.dictionary.ExplainValue(field.Tag, field.Value); ok {
		return fmt.Sprintf("%s:%s", definition.Name, enum)
	}
	return definition.Name
}
