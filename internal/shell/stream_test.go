package shell

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"fix-tool/internal/order"
)

func TestOrderStreamBuildSequenceClOrdID(t *testing.T) {
	request := OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "AAPL",
			Side:     "buy",
			OrderQty: "100",
			Price:    "10",
		},
		ClOrdIDPrefix: "S",
		ClOrdIDMode:   streamClOrdIDModeSequence,
		StartSeq:      7,
	}

	got := buildOrderStreamNewRequest(request, 2)
	if got.ClOrdID != "S-9" {
		t.Fatalf("ClOrdID = %q, want S-9", got.ClOrdID)
	}
}

func TestOrderStreamBuildRandomClOrdID(t *testing.T) {
	request := OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "AAPL",
			Side:     "buy",
			OrderQty: "100",
			Price:    "10",
		},
		ClOrdIDPrefix: "R",
		ClOrdIDMode:   streamClOrdIDModeRandom,
	}

	first := buildOrderStreamNewRequest(request, 0)
	second := buildOrderStreamNewRequest(request, 1)
	if !strings.HasPrefix(first.ClOrdID, "R-") {
		t.Fatalf("first ClOrdID = %q, want R- prefix", first.ClOrdID)
	}
	if !strings.HasPrefix(second.ClOrdID, "R-") {
		t.Fatalf("second ClOrdID = %q, want R- prefix", second.ClOrdID)
	}
	if first.ClOrdID == second.ClOrdID {
		t.Fatalf("random ClOrdID duplicated: %q", first.ClOrdID)
	}
}

func TestOrderStreamBuildAlternateAndSequences(t *testing.T) {
	request := OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "AAPL",
			Side:     "buy",
			OrderQty: "100",
			Price:    "10.25",
		},
		SideMode:  streamSideModeAlternate,
		SymbolSeq: []string{"AAPL", "MSFT"},
		QtySeq:    []string{"100", "200"},
		PriceSeq:  []string{"10.25", "10.30", "10.35"},
	}

	first := buildOrderStreamNewRequest(request, 0)
	second := buildOrderStreamNewRequest(request, 1)
	third := buildOrderStreamNewRequest(request, 2)
	if first.Symbol != "AAPL" || first.Side != "buy" || first.OrderQty != "100" || first.Price != "10.25" {
		t.Fatalf("first request = %#v, want first sequence values", first)
	}
	if second.Symbol != "MSFT" || second.Side != "sell" || second.OrderQty != "200" || second.Price != "10.30" {
		t.Fatalf("second request = %#v, want alternate and second sequence values", second)
	}
	if third.Symbol != "AAPL" || third.Side != "buy" || third.OrderQty != "100" || third.Price != "10.35" {
		t.Fatalf("third request = %#v, want wrapped sequence values", third)
	}
}

func TestOrderStreamCountAutoEnds(t *testing.T) {
	service := &recordingOrderService{}
	stream := newOrderStream(service, nil, nil)

	err := stream.Start(context.Background(), OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "AAPL",
			Side:     "buy",
			OrderQty: "100",
			Price:    "10",
		},
		Interval: time.Millisecond,
		Count:    2,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForStream(t, stream, false)
	status := stream.Status()
	if status.Running {
		t.Fatal("Running = true, want false")
	}
	if status.Sent != 2 || status.Succeeded != 2 || status.Failed != 0 {
		t.Fatalf("status = %#v, want sent=2 success=2 failed=0", status)
	}
	if got := len(service.requests()); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestOrderStreamStartRejectsInvalidOrder(t *testing.T) {
	service := &recordingOrderService{}
	stream := newOrderStream(service, nil, nil)

	err := stream.Start(context.Background(), OrderStreamRequest{})
	if err == nil || !strings.Contains(err.Error(), "缺少必填参数 --symbol 或 --symbol-seq") {
		t.Fatalf("Start() error = %v, want missing symbol", err)
	}
	if stream.Status().Running {
		t.Fatal("Running = true, want false")
	}
	if got := len(service.requests()); got != 0 {
		t.Fatalf("requests = %d, want 0", got)
	}
}

func TestOrderStreamStartAcceptsSequenceRequiredEquivalents(t *testing.T) {
	tests := []struct {
		name    string
		request OrderStreamRequest
	}{
		{
			name: "symbol sequence replaces symbol",
			request: OrderStreamRequest{
				Order: order.NewRequest{
					Side:     "buy",
					OrderQty: "100",
					Price:    "10",
				},
				SymbolSeq: []string{"AAPL", "MSFT"},
				Count:     1,
			},
		},
		{
			name: "qty sequence replaces qty",
			request: OrderStreamRequest{
				Order: order.NewRequest{
					Symbol: "AAPL",
					Side:   "buy",
					Price:  "10",
				},
				QtySeq: []string{"100", "200"},
				Count:  1,
			},
		},
		{
			name: "price sequence replaces price",
			request: OrderStreamRequest{
				Order: order.NewRequest{
					Symbol:   "AAPL",
					Side:     "buy",
					OrderQty: "100",
				},
				PriceSeq: []string{"10", "10.1"},
				Count:    1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &recordingOrderService{}
			stream := newOrderStream(service, nil, nil)

			err := stream.Start(context.Background(), tt.request)
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			waitForStream(t, stream, false)
			if got := len(service.requests()); got != 1 {
				t.Fatalf("requests = %d, want 1", got)
			}
		})
	}
}

func TestOrderStreamStartAcceptsMarketWithoutPrice(t *testing.T) {
	service := &recordingOrderService{}
	stream := newOrderStream(service, nil, nil)

	err := stream.Start(context.Background(), OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "AAPL",
			Side:     "buy",
			OrderQty: "100",
			OrdType:  "market",
		},
		Count: 1,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForStream(t, stream, false)
	if got := len(service.requests()); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestOrderStreamStartRejectsMissingSideWithSequences(t *testing.T) {
	service := &recordingOrderService{}
	stream := newOrderStream(service, nil, nil)

	err := stream.Start(context.Background(), OrderStreamRequest{
		SymbolSeq: []string{"AAPL", "MSFT"},
		QtySeq:    []string{"100", "200"},
		PriceSeq:  []string{"10", "10.1"},
	})
	if err == nil || !strings.Contains(err.Error(), "缺少必填参数 --side") {
		t.Fatalf("Start() error = %v, want missing side", err)
	}
	if stream.Status().Running {
		t.Fatal("Running = true, want false")
	}
	if got := len(service.requests()); got != 0 {
		t.Fatalf("requests = %d, want 0", got)
	}
}

func TestOrderStreamStartRejectsDuplicate(t *testing.T) {
	service := newBlockingOrderService()
	stream := newOrderStream(service, nil, nil)
	err := stream.Start(context.Background(), OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "AAPL",
			Side:     "buy",
			OrderQty: "100",
			Price:    "10",
		},
		Interval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForCall(t, service.called)

	err = stream.Start(context.Background(), OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "MSFT",
			Side:     "sell",
			OrderQty: "200",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("Start() error = %v, want already running", err)
	}

	close(service.release)
	if err := stream.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestOrderStreamStopCancelsRunningTask(t *testing.T) {
	service := newBlockingOrderService()
	stream := newOrderStream(service, nil, nil)
	err := stream.Start(context.Background(), OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "AAPL",
			Side:     "buy",
			OrderQty: "100",
			Price:    "10",
		},
		Interval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForCall(t, service.called)
	close(service.release)

	if err := stream.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	status := stream.Status()
	if status.Running {
		t.Fatal("Running = true, want false")
	}
	if status.Sent != 1 {
		t.Fatalf("Sent = %d, want 1", status.Sent)
	}
}

func TestOrderStreamStatusTracksFailures(t *testing.T) {
	service := &recordingOrderService{err: errors.New("send failed")}
	stream := newOrderStream(service, nil, nil)
	err := stream.Start(context.Background(), OrderStreamRequest{
		Order: order.NewRequest{
			Symbol:   "AAPL",
			Side:     "buy",
			OrderQty: "100",
			Price:    "10",
		},
		Interval: time.Millisecond,
		Count:    1,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForStream(t, stream, false)
	status := stream.Status()
	if status.Sent != 1 || status.Succeeded != 0 || status.Failed != 1 {
		t.Fatalf("status = %#v, want one failure", status)
	}
	if status.LastError != "send failed" {
		t.Fatalf("LastError = %q, want send failed", status.LastError)
	}
}

type recordingOrderService struct {
	mu            sync.Mutex
	err           error
	requestsValue []order.NewRequest
}

func (s *recordingOrderService) NewOrder(context.Context, order.NewRequest) (order.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return order.Result{}, s.err
	}
	s.requestsValue = append(s.requestsValue, order.NewRequest{})
	return order.Result{}, nil
}

func (s *recordingOrderService) CancelOrder(context.Context, order.CancelRequest) (order.Result, error) {
	return order.Result{}, nil
}

func (s *recordingOrderService) ReplaceOrder(context.Context, order.ReplaceRequest) (order.Result, error) {
	return order.Result{}, nil
}

func (s *recordingOrderService) requests() []order.NewRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]order.NewRequest, len(s.requestsValue))
	copy(result, s.requestsValue)
	return result
}

type blockingOrderService struct {
	calledOnce sync.Once
	called     chan struct{}
	release    chan struct{}
}

func newBlockingOrderService() *blockingOrderService {
	return &blockingOrderService{
		called:  make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (s *blockingOrderService) NewOrder(ctx context.Context, request order.NewRequest) (order.Result, error) {
	s.calledOnce.Do(func() {
		close(s.called)
	})
	select {
	case <-s.release:
	case <-ctx.Done():
		return order.Result{}, ctx.Err()
	}
	return order.Result{}, nil
}

func (s *blockingOrderService) CancelOrder(context.Context, order.CancelRequest) (order.Result, error) {
	return order.Result{}, nil
}

func (s *blockingOrderService) ReplaceOrder(context.Context, order.ReplaceRequest) (order.Result, error) {
	return order.Result{}, nil
}

func waitForStream(t *testing.T, stream *orderStream, running bool) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("stream running did not become %t, status=%#v", running, stream.Status())
		case <-ticker.C:
			if stream.Status().Running == running {
				return
			}
		}
	}
}

func waitForCall(t *testing.T, called <-chan struct{}) {
	t.Helper()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("NewOrder was not called")
	}
}
