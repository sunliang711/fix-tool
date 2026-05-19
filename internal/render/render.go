package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"fix-tool/internal/dictionary"
	"fix-tool/internal/trace"
)

const (
	FormatTable Format = "table"
	FormatRaw   Format = "raw"
	FormatJSON  Format = "json"

	defaultDelimiter = "|"
	redactedValue    = "[REDACTED]"
)

type Format string

type Options struct {
	Format        Format
	RawDelimiter  string
	ShowSensitive bool
}

type Renderer struct {
	dictionary *dictionary.Dictionary
	options    Options
}

func NewRenderer(dict *dictionary.Dictionary, options Options) *Renderer {
	if dict == nil {
		dict = dictionary.Standard()
	}
	if options.Format == "" {
		options.Format = FormatTable
	}
	if options.RawDelimiter == "" {
		options.RawDelimiter = defaultDelimiter
	}
	return &Renderer{dictionary: dict, options: options}
}

func (r *Renderer) Render(message trace.MessageTrace, format Format) (string, error) {
	if format == "" {
		format = r.options.Format
	}
	switch format {
	case FormatTable:
		return r.Table(message)
	case FormatRaw:
		return r.Raw(message)
	case FormatJSON:
		return r.JSON(message)
	default:
		return "", fmt.Errorf("unsupported render format %q", format)
	}
}

func (r *Renderer) Raw(message trace.MessageTrace) (string, error) {
	fields, err := fieldsFor(message)
	if err != nil {
		return "", err
	}
	return r.rawFromFields(fields), nil
}

func (r *Renderer) Table(message trace.MessageTrace) (string, error) {
	fields, err := fieldsFor(message)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintf(writer, "TraceID\t%s\n", message.TraceID)
	fmt.Fprintf(writer, "Profile\t%s\n", message.Profile)
	fmt.Fprintf(writer, "Direction\t%s\n", message.Direction)
	fmt.Fprintf(writer, "MsgType\t%s\t%s\n", message.MsgType, r.enumText(trace.Field{Tag: 35, Value: message.MsgType}))
	if !message.SentAt.IsZero() {
		fmt.Fprintf(writer, "SentAt\t%s\n", message.SentAt.Format(time.RFC3339Nano))
	}
	if !message.ReceivedAt.IsZero() {
		fmt.Fprintf(writer, "ReceivedAt\t%s\n", message.ReceivedAt.Format(time.RFC3339Nano))
	}
	if message.Latency > 0 {
		fmt.Fprintf(writer, "Latency\t%s\n", message.Latency)
	}
	fmt.Fprintf(writer, "BodyLength\tvalid=%t\texpected=%s\tactual=%s\n", message.BodyLengthValid, message.BodyLength.Expected, message.BodyLength.Actual)
	fmt.Fprintf(writer, "CheckSum\tvalid=%t\texpected=%s\tactual=%s\n", message.CheckSumValid, message.CheckSum.Expected, message.CheckSum.Actual)
	fmt.Fprintf(writer, "Raw\t%s\n\n", r.rawFromFields(fields))
	fmt.Fprintf(writer, "Tag\tName\tValue\tEnum\tSensitive\n")
	for _, field := range fields {
		view := r.fieldView(field)
		fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%t\n", view.Tag, view.Name, view.Value, view.Enum, view.Sensitive)
	}
	if err := writer.Flush(); err != nil {
		return "", fmt.Errorf("render table: %w", err)
	}
	return builder.String(), nil
}

func (r *Renderer) JSON(message trace.MessageTrace) (string, error) {
	fields, err := fieldsFor(message)
	if err != nil {
		return "", err
	}
	view := traceView{
		TraceID:         message.TraceID,
		Profile:         message.Profile,
		Direction:       string(message.Direction),
		MsgType:         message.MsgType,
		MsgSeqNum:       message.MsgSeqNum,
		ClOrdID:         message.ClOrdID,
		OrderID:         message.OrderID,
		ExecType:        message.ExecType,
		OrdStatus:       message.OrdStatus,
		Raw:             r.rawFromFields(fields),
		SentAt:          formatTime(message.SentAt),
		ReceivedAt:      formatTime(message.ReceivedAt),
		LatencyMS:       message.Latency.Milliseconds(),
		BodyLength:      message.BodyLength,
		CheckSum:        message.CheckSum,
		BodyLengthValid: message.BodyLengthValid,
		CheckSumValid:   message.CheckSumValid,
	}
	for _, field := range fields {
		view.Fields = append(view.Fields, r.fieldView(field))
	}
	data, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return "", fmt.Errorf("render json: %w", err)
	}
	return string(data), nil
}

func (r *Renderer) rawFromFields(fields []trace.Field) string {
	var builder strings.Builder
	for _, field := range fields {
		view := r.fieldView(field)
		builder.WriteString(fmt.Sprintf("%d=%s%s", field.Tag, view.Value, r.options.RawDelimiter))
	}
	return builder.String()
}

func (r *Renderer) fieldView(field trace.Field) fieldView {
	definition, ok := r.dictionary.Lookup(field.Tag)
	name := fmt.Sprintf("Tag%d", field.Tag)
	sensitive := false
	if ok {
		name = definition.Name
		sensitive = definition.Sensitive
	}
	value := field.Value
	enum := ""
	if sensitive && !r.options.ShowSensitive {
		value = redactedValue
	} else {
		enum = r.enumText(field)
	}
	return fieldView{
		Tag:       field.Tag,
		Name:      name,
		Value:     value,
		Enum:      enum,
		Sensitive: sensitive,
	}
}

func (r *Renderer) enumText(field trace.Field) string {
	enum, ok := r.dictionary.ExplainValue(field.Tag, field.Value)
	if !ok {
		return ""
	}
	return enum
}

func fieldsFor(message trace.MessageTrace) ([]trace.Field, error) {
	if len(message.Fields) > 0 {
		fields := make([]trace.Field, len(message.Fields))
		copy(fields, message.Fields)
		return fields, nil
	}
	parsed, err := trace.ParseRaw(message.Raw)
	if err != nil {
		return nil, err
	}
	return parsed.Fields, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
