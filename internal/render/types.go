package render

import "fix-tool/internal/trace"

type fieldView struct {
	Tag       int    `json:"tag"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Value     string `json:"value"`
	Enum      string `json:"enum,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
}

type traceView struct {
	TraceID         string                 `json:"trace_id"`
	Profile         string                 `json:"profile,omitempty"`
	Direction       string                 `json:"direction,omitempty"`
	MsgType         string                 `json:"msg_type,omitempty"`
	MsgSeqNum       string                 `json:"msg_seq_num,omitempty"`
	ClOrdID         string                 `json:"cl_ord_id,omitempty"`
	OrderID         string                 `json:"order_id,omitempty"`
	ExecType        string                 `json:"exec_type,omitempty"`
	OrdStatus       string                 `json:"ord_status,omitempty"`
	Raw             string                 `json:"raw"`
	SentAt          string                 `json:"sent_at,omitempty"`
	ReceivedAt      string                 `json:"received_at,omitempty"`
	LatencyMS       int64                  `json:"latency_ms,omitempty"`
	Fields          []fieldView            `json:"fields"`
	BodyLength      trace.ValidationResult `json:"body_length"`
	CheckSum        trace.ValidationResult `json:"checksum"`
	BodyLengthValid bool                   `json:"body_length_valid"`
	CheckSumValid   bool                   `json:"checksum_valid"`
}
