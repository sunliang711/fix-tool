package trace

import "time"

const (
	tagBeginString = 8
	tagBodyLength  = 9
	tagCheckSum    = 10
	tagClOrdID     = 11
	tagMsgSeqNum   = 34
	tagMsgType     = 35
	tagOrderID     = 37
	tagOrdStatus   = 39
	tagExecType    = 150
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type Field struct {
	Tag   int    `json:"tag"`
	Value string `json:"value"`
}

type ValidationResult struct {
	Present  bool   `json:"present"`
	Valid    bool   `json:"valid"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type ParsedMessage struct {
	Raw             string           `json:"raw"`
	Fields          []Field          `json:"fields"`
	BodyLength      ValidationResult `json:"body_length"`
	CheckSum        ValidationResult `json:"checksum"`
	BodyLengthValid bool             `json:"body_length_valid"`
	CheckSumValid   bool             `json:"checksum_valid"`
}

type MessageTrace struct {
	TraceID         string           `json:"trace_id"`
	Profile         string           `json:"profile,omitempty"`
	Direction       Direction        `json:"direction"`
	MsgType         string           `json:"msg_type,omitempty"`
	MsgSeqNum       string           `json:"msg_seq_num,omitempty"`
	ClOrdID         string           `json:"cl_ord_id,omitempty"`
	OrderID         string           `json:"order_id,omitempty"`
	ExecType        string           `json:"exec_type,omitempty"`
	OrdStatus       string           `json:"ord_status,omitempty"`
	Raw             string           `json:"raw"`
	Fields          []Field          `json:"fields"`
	SentAt          time.Time        `json:"sent_at,omitempty"`
	ReceivedAt      time.Time        `json:"received_at,omitempty"`
	Latency         time.Duration    `json:"latency,omitempty"`
	BodyLength      ValidationResult `json:"body_length"`
	CheckSum        ValidationResult `json:"checksum"`
	BodyLengthValid bool             `json:"body_length_valid"`
	CheckSumValid   bool             `json:"checksum_valid"`
}

type BuildOptions struct {
	TraceID    string
	Profile    string
	Direction  Direction
	Raw        string
	SentAt     time.Time
	ReceivedAt time.Time
}

func NewMessageTrace(opts BuildOptions) (MessageTrace, error) {
	parsed, err := ParseRaw(opts.Raw)
	if err != nil {
		return MessageTrace{}, err
	}
	latency := time.Duration(0)
	if !opts.SentAt.IsZero() && !opts.ReceivedAt.IsZero() {
		latency = opts.ReceivedAt.Sub(opts.SentAt)
	}
	return MessageTrace{
		TraceID:         opts.TraceID,
		Profile:         opts.Profile,
		Direction:       opts.Direction,
		MsgType:         firstValue(parsed.Fields, tagMsgType),
		MsgSeqNum:       firstValue(parsed.Fields, tagMsgSeqNum),
		ClOrdID:         firstValue(parsed.Fields, tagClOrdID),
		OrderID:         firstValue(parsed.Fields, tagOrderID),
		ExecType:        firstValue(parsed.Fields, tagExecType),
		OrdStatus:       firstValue(parsed.Fields, tagOrdStatus),
		Raw:             parsed.Raw,
		Fields:          parsed.Fields,
		SentAt:          opts.SentAt,
		ReceivedAt:      opts.ReceivedAt,
		Latency:         latency,
		BodyLength:      parsed.BodyLength,
		CheckSum:        parsed.CheckSum,
		BodyLengthValid: parsed.BodyLengthValid,
		CheckSumValid:   parsed.CheckSumValid,
	}, nil
}

func firstValue(fields []Field, tag int) string {
	for _, field := range fields {
		if field.Tag == tag {
			return field.Value
		}
	}
	return ""
}
