package trace

import (
	"fmt"
	"sync"
	"time"
)

type RecordOptions struct {
	TraceID    string
	Profile    string
	Direction  Direction
	Raw        string
	SentAt     time.Time
	ReceivedAt time.Time
}

type Recorder struct {
	mu     sync.RWMutex
	nextID uint64
	now    func() time.Time
	traces []MessageTrace
}

func NewRecorder() *Recorder {
	return &Recorder{
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func NewRecorderWithClock(now func() time.Time) *Recorder {
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &Recorder{now: now}
}

func (r *Recorder) Record(message MessageTrace) MessageTrace {
	r.mu.Lock()
	defer r.mu.Unlock()
	if message.TraceID == "" {
		r.nextID++
		message.TraceID = fmt.Sprintf("trace-%06d", r.nextID)
	}
	message = cloneTrace(message)
	r.traces = append(r.traces, message)
	return cloneTrace(message)
}

func (r *Recorder) RecordRaw(opts RecordOptions) (MessageTrace, error) {
	recordedAt := r.now()
	if opts.Direction == DirectionOutbound && opts.SentAt.IsZero() {
		opts.SentAt = recordedAt
	}
	if opts.Direction == DirectionInbound && opts.ReceivedAt.IsZero() {
		opts.ReceivedAt = recordedAt
	}
	message, err := NewMessageTrace(BuildOptions{
		TraceID:    opts.TraceID,
		Profile:    opts.Profile,
		Direction:  opts.Direction,
		Raw:        opts.Raw,
		SentAt:     opts.SentAt,
		ReceivedAt: opts.ReceivedAt,
	})
	if err != nil {
		return MessageTrace{}, err
	}
	return r.Record(message), nil
}

func (r *Recorder) List() []MessageTrace {
	r.mu.RLock()
	defer r.mu.RUnlock()
	traces := make([]MessageTrace, len(r.traces))
	for i, message := range r.traces {
		traces[i] = cloneTrace(message)
	}
	return traces
}

func (r *Recorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.traces = nil
}

func cloneTrace(message MessageTrace) MessageTrace {
	if len(message.Fields) == 0 {
		return message
	}
	fields := make([]Field, len(message.Fields))
	copy(fields, message.Fields)
	message.Fields = fields
	return message
}
