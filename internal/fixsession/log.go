package fixsession

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"fix-tool/internal/config"
	"fix-tool/internal/dictionary"
	"fix-tool/internal/trace"

	"github.com/mattn/go-isatty"
	"github.com/quickfixgo/quickfix"
	"github.com/rs/zerolog"
)

const (
	directionOut = "out"
	directionIn  = "in"

	ansiReset    = "\x1b[0m"
	ansiOutgoing = "\x1b[38;5;213m"
	ansiIncoming = "\x1b[38;5;147m"
)

type zerologLogFactory struct {
	logger        zerolog.Logger
	sensitiveTags map[quickfix.Tag]struct{}
	dictionary    *dictionary.Dictionary
	messageOutput io.Writer
	messageColor  bool
	messageMu     *sync.Mutex
}

type zerologLog struct {
	logger        zerolog.Logger
	session       string
	sensitiveTags map[quickfix.Tag]struct{}
	dictionary    *dictionary.Dictionary
	messageOutput io.Writer
	messageColor  bool
	messageMu     *sync.Mutex
}

func newZerologLogFactory(logger zerolog.Logger, profile config.ProfileConfig) quickfix.LogFactory {
	return newZerologLogFactoryWithOptions(logger, profile, quickFIXLogOptions{})
}

type quickFIXLogOptions struct {
	messageOutput io.Writer
}

func newZerologLogFactoryWithOptions(logger zerolog.Logger, profile config.ProfileConfig, options quickFIXLogOptions) quickfix.LogFactory {
	return zerologLogFactory{
		logger:        logger,
		sensitiveTags: sensitiveTagsFromProfile(profile),
		dictionary:    dictionary.NewFromConfig(profile.CustomFieldDefs),
		messageOutput: options.messageOutput,
		messageColor:  shouldColorMessageOutput(options.messageOutput),
		messageMu:     &sync.Mutex{},
	}
}

func (f zerologLogFactory) Create() (quickfix.Log, error) {
	return zerologLog{
		logger:        f.logger,
		sensitiveTags: f.sensitiveTags,
		dictionary:    f.dictionary,
		messageOutput: f.messageOutput,
		messageColor:  f.messageColor,
		messageMu:     f.messageMu,
	}, nil
}

func (f zerologLogFactory) CreateSessionLog(sessionID quickfix.SessionID) (quickfix.Log, error) {
	return zerologLog{
		logger:        f.logger,
		session:       sessionID.String(),
		sensitiveTags: f.sensitiveTags,
		dictionary:    f.dictionary,
		messageOutput: f.messageOutput,
		messageColor:  f.messageColor,
		messageMu:     f.messageMu,
	}, nil
}

func (l zerologLog) OnIncoming(message []byte) {
	l.logMessage(directionIn, message)
}

func (l zerologLog) OnOutgoing(message []byte) {
	l.logMessage(directionOut, message)
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
	if l.messageOutput != nil {
		l.writeFIXMessage(direction, raw)
		return
	}
	parsed, err := trace.ParseRaw(raw)
	if err != nil {
		l.logUnparsedMessage(direction, raw, err)
		return
	}
	msgCode := firstParsedValue(parsed.Fields, int(tagMsgType))
	msgName := l.messageName(msgCode)
	event := l.logger.Info().
		Str("source", "quickfix")
	if l.session != "" {
		event = event.Str("session", l.session)
	}
	event = l.appendSummaryFields(event, parsed.Fields)
	event.Msg(l.messageLogTitle(direction, msgName, msgCode))
	l.logDebugMessage(direction, raw, msgName, msgCode)
}

func (l zerologLog) writeFIXMessage(direction string, raw string) {
	var builder strings.Builder
	titleColor := l.messageTitleColor(direction)
	if titleColor != "" {
		builder.WriteString(titleColor)
	}
	builder.WriteString(l.messageOutputTitle(direction))
	builder.WriteByte('\n')
	fmt.Fprintf(&builder, "Time:        %s\n", time.Now().UTC())
	fmt.Fprintf(&builder, "Session:     %s\n", l.messageOutputSession(raw))
	builder.WriteString("Content:\n")
	builder.WriteString("  Raw:\n")
	fmt.Fprintf(&builder, "    %s\n", trace.DisplayRaw(raw, "|"))
	builder.WriteString("  Pretty:\n")
	for _, line := range l.prettyFIXMessageLines(raw) {
		fmt.Fprintf(&builder, "    %s\n", line)
	}
	if titleColor != "" {
		builder.WriteString(ansiReset)
	}

	if l.messageMu == nil {
		_, _ = io.WriteString(l.messageOutput, builder.String())
		return
	}
	l.messageMu.Lock()
	defer l.messageMu.Unlock()
	_, _ = io.WriteString(l.messageOutput, builder.String())
}

func (l zerologLog) messageOutputTitle(direction string) string {
	switch direction {
	case directionOut:
		return "===> Outgoing FIX Msg: ===>"
	case directionIn:
		return "<=== Incoming FIX Msg: <==="
	default:
		return "---- FIX Msg: ----"
	}
}

func (l zerologLog) messageTitleColor(direction string) string {
	if !l.messageColor {
		return ""
	}
	switch direction {
	case directionOut:
		return ansiOutgoing
	case directionIn:
		return ansiIncoming
	default:
		return ""
	}
}

func (l zerologLog) messageOutputSession(raw string) string {
	parsed, err := trace.ParseRaw(raw)
	if err == nil {
		beginString := firstParsedValue(parsed.Fields, 8)
		sender := firstParsedValue(parsed.Fields, 49)
		target := firstParsedValue(parsed.Fields, 56)
		if beginString != "" && sender != "" && target != "" {
			return beginString + ":" + sender + "->" + target
		}
	}
	if l.session == "" {
		return "-"
	}
	return l.session
}

func shouldColorMessageOutput(output io.Writer) bool {
	if output == nil || os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := output.(*os.File)
	return ok && isatty.IsTerminal(file.Fd())
}

func (l zerologLog) prettyFIXMessageLines(raw string) []string {
	parsed, err := trace.ParseRaw(raw)
	if err != nil {
		return []string{trace.DisplayRaw(raw, "|")}
	}
	type prettyField struct {
		name  string
		tag   string
		value string
	}
	fields := make([]prettyField, 0, len(parsed.Fields))
	maxNameWidth := 0
	maxTagWidth := 0
	for _, field := range parsed.Fields {
		value := field.Value
		if _, ok := l.sensitiveTags[quickfix.Tag(field.Tag)]; ok {
			value = redactedValue
		}
		name := l.prettyFieldName(field)
		tag := strconv.Itoa(field.Tag)
		fields = append(fields, prettyField{
			name:  name,
			tag:   tag,
			value: value,
		})
		if len(name) > maxNameWidth {
			maxNameWidth = len(name)
		}
		if len(tag) > maxTagWidth {
			maxTagWidth = len(tag)
		}
	}
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		lines = append(lines, fmt.Sprintf("%-*s %*s = %s", maxNameWidth, field.name, maxTagWidth, field.tag, field.value))
	}
	return lines
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

func (l zerologLog) appendSummaryFields(event *zerolog.Event, fields []trace.Field) *zerolog.Event {
	if seq := firstParsedValue(fields, 34); seq != "" {
		event = event.Str("seq", seq)
	}
	if sender := firstParsedValue(fields, 49); sender != "" {
		event = event.Str("sender", sender)
	}
	if target := firstParsedValue(fields, 56); target != "" {
		event = event.Str("target", target)
	}
	for _, field := range []struct {
		tag int
		key string
	}{
		{tag: 58, key: "reason"},
		{tag: 11, key: "cl_ord_id"},
		{tag: 41, key: "orig_cl_ord_id"},
		{tag: 37, key: "order_id"},
		{tag: 17, key: "exec_id"},
		{tag: 55, key: "symbol"},
		{tag: 54, key: "side"},
		{tag: 38, key: "qty"},
		{tag: 44, key: "price"},
		{tag: 40, key: "ord_type"},
		{tag: 59, key: "time_in_force"},
		{tag: 150, key: "exec_type"},
		{tag: 39, key: "ord_status"},
	} {
		if value := firstParsedValue(fields, field.tag); value != "" {
			event = event.Str(field.key, l.displayValue(field.tag, value))
		}
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

func (l zerologLog) logUnparsedMessage(direction string, raw string, err error) {
	event := l.logger.Info().
		Str("source", "quickfix").
		Err(err)
	if l.session != "" {
		event = event.Str("session", l.session)
	}
	event.Msg(l.messageLogTitle(direction, "", ""))
	l.logDebugMessage(direction, raw, "", "")
}

func (l zerologLog) logDebugMessage(direction string, raw string, msgName string, msgCode string) {
	if l.logger.GetLevel() > zerolog.DebugLevel {
		return
	}
	event := l.messageDebugEvent(direction).
		Str("raw_message", trace.DisplayRaw(raw, "|")).
		Str("pretty_message", l.prettyFIXMessage(raw))
	event.Msg(l.messageLogTitle(direction, msgName, msgCode))
}

func (l zerologLog) messageDebugEvent(direction string) *zerolog.Event {
	event := l.logger.Debug().
		Str("source", "quickfix").
		Str("direction", direction)
	if l.session != "" {
		event = event.Str("session", l.session)
	}
	return event
}

func (l zerologLog) messageLogTitle(direction string, msgName string, msgCode string) string {
	title := l.directionArrow(direction)
	if msgName == "" && msgCode == "" {
		return title + " FIX message"
	}
	if msgName == "" {
		return title + " MsgType(" + msgCode + ")"
	}
	if msgCode == "" {
		return title + " " + msgName
	}
	return title + " " + msgName + "(" + msgCode + ")"
}

func (l zerologLog) directionArrow(direction string) string {
	switch direction {
	case directionOut:
		return "->"
	case directionIn:
		return "<-"
	default:
		return "--"
	}
}

func (l zerologLog) messageName(msgCode string) string {
	if msgCode == "" || l.dictionary == nil {
		return ""
	}
	msgName, ok := l.dictionary.ExplainValue(int(tagMsgType), msgCode)
	if !ok {
		return ""
	}
	return msgName
}

func (l zerologLog) displayValue(tag int, value string) string {
	if l.dictionary == nil {
		return value
	}
	display, ok := l.dictionary.ExplainValue(tag, value)
	if !ok {
		return value
	}
	return display
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
