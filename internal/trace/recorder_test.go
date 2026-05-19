package trace

import (
	"testing"
	"time"
)

func TestRecorderRecordsParsedTrace(t *testing.T) {
	now := time.Date(2026, 5, 19, 3, 0, 0, 0, time.UTC)
	recorder := NewRecorderWithClock(func() time.Time { return now })

	message, err := recorder.RecordRaw(RecordOptions{
		Profile:   "uat",
		Direction: DirectionOutbound,
		Raw:       readMessage(t, "logon.fix"),
	})
	if err != nil {
		t.Fatalf("RecordRaw() error = %v", err)
	}
	if message.TraceID != "trace-000001" {
		t.Fatalf("TraceID = %q, want %q", message.TraceID, "trace-000001")
	}
	if message.Profile != "uat" {
		t.Fatalf("Profile = %q, want %q", message.Profile, "uat")
	}
	if message.MsgType != "A" {
		t.Fatalf("MsgType = %q, want %q", message.MsgType, "A")
	}
	if !message.SentAt.Equal(now) {
		t.Fatalf("SentAt = %s, want %s", message.SentAt, now)
	}
	if !message.BodyLengthValid || !message.CheckSumValid {
		t.Fatalf("validation failed: body=%+v checksum=%+v", message.BodyLength, message.CheckSum)
	}

	traces := recorder.List()
	if len(traces) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(traces))
	}
	traces[0].TraceID = "mutated"
	traces[0].Fields[0].Value = "mutated"
	if recorder.List()[0].TraceID != "trace-000001" {
		t.Fatal("List() returned mutable recorder storage")
	}
	if recorder.List()[0].Fields[0].Value == "mutated" {
		t.Fatal("List() returned mutable field storage")
	}
}
