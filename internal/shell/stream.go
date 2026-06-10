package shell

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"fix-tool/internal/message"
	"fix-tool/internal/order"
)

const (
	streamClOrdIDModeSequence = "sequence"
	streamClOrdIDModeRandom   = "random"
	streamSideModeFixed       = "fixed"
	streamSideModeAlternate   = "alternate"
	defaultStreamInterval     = time.Second
	defaultStreamClOrdID      = "STRM"
	defaultStreamStartSeq     = 1
)

type OrderStreamRequest struct {
	Order         order.NewRequest
	Interval      time.Duration
	Count         int
	ClOrdIDPrefix string
	ClOrdIDMode   string
	StartSeq      int
	SideMode      string
	SymbolSeq     []string
	QtySeq        []string
	PriceSeq      []string
}

type orderStreamStatus struct {
	Running   bool
	Sent      int
	Succeeded int
	Failed    int
	LastError string
}

type orderStream struct {
	mu      sync.Mutex
	order   OrderService
	record  func(order.Result) error
	orderMu *sync.Mutex
	task    *orderStreamTask
	status  orderStreamStatus
}

type orderStreamTask struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// defaultOrderStreamRequest 返回 shell stream 命令的默认控制参数。
func defaultOrderStreamRequest() OrderStreamRequest {
	return OrderStreamRequest{
		Interval:      defaultStreamInterval,
		ClOrdIDPrefix: defaultStreamClOrdID,
		ClOrdIDMode:   streamClOrdIDModeSequence,
		StartSeq:      defaultStreamStartSeq,
		SideMode:      streamSideModeFixed,
	}
}

// newOrderStream 创建单 shell 实例内的 order stream 管理器。
func newOrderStream(orderService OrderService, record func(order.Result) error, orderMu *sync.Mutex) *orderStream {
	return &orderStream{
		order:   orderService,
		record:  record,
		orderMu: orderMu,
	}
}

// Start 启动后台 order new stream，同一 shell 内只允许一个任务运行。
func (s *orderStream) Start(ctx context.Context, request OrderStreamRequest) error {
	if s == nil {
		return fmt.Errorf("order stream is not available")
	}
	if s.order == nil {
		return fmt.Errorf("order service is not available")
	}
	s.mu.Lock()
	if s.task != nil {
		s.mu.Unlock()
		return fmt.Errorf("order stream is already running")
	}
	s.mu.Unlock()
	request = normalizeOrderStreamRequest(request)
	if err := validateOrderStreamRequest(request); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	task := &orderStreamTask{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	s.mu.Lock()
	if s.task != nil {
		s.mu.Unlock()
		cancel()
		return fmt.Errorf("order stream is already running")
	}
	s.task = task
	s.status = orderStreamStatus{Running: true}
	s.mu.Unlock()

	go s.run(runCtx, request, task)
	return nil
}

// Stop 取消当前后台 stream 任务，并等待 goroutine 退出。
func (s *orderStream) Stop() error {
	if s == nil {
		return fmt.Errorf("order stream is not available")
	}
	s.mu.Lock()
	task := s.task
	if task == nil {
		s.mu.Unlock()
		return fmt.Errorf("order stream is not running")
	}
	task.cancel()
	s.mu.Unlock()

	<-task.done
	return nil
}

// Status 返回当前 stream 状态快照。
func (s *orderStream) Status() orderStreamStatus {
	if s == nil {
		return orderStreamStatus{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// StopIfRunning 在 shell 退出时清理后台 stream，避免 goroutine 泄漏。
func (s *orderStream) StopIfRunning() {
	if s == nil {
		return
	}
	s.mu.Lock()
	task := s.task
	if task == nil {
		s.mu.Unlock()
		return
	}
	task.cancel()
	s.mu.Unlock()
	<-task.done
}

func (s *orderStream) run(ctx context.Context, request OrderStreamRequest, task *orderStreamTask) {
	defer close(task.done)
	defer s.finish(task)

	for i := 0; request.Count == 0 || i < request.Count; i++ {
		if err := ctx.Err(); err != nil {
			return
		}
		s.sendOne(ctx, buildOrderStreamNewRequest(request, i))
		if request.Count > 0 && i+1 >= request.Count {
			return
		}
		timer := time.NewTimer(request.Interval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (s *orderStream) sendOne(ctx context.Context, request order.NewRequest) {
	if s.orderMu != nil {
		s.orderMu.Lock()
		defer s.orderMu.Unlock()
	}
	result, err := s.order.NewOrder(ctx, request)
	if err != nil {
		s.markSendResult(err)
		return
	}
	if s.record != nil {
		err = s.record(result)
	}
	s.markSendResult(err)
}

func (s *orderStream) markSendResult(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Sent++
	if err != nil {
		s.status.Failed++
		s.status.LastError = err.Error()
		return
	}
	s.status.Succeeded++
}

func (s *orderStream) finish(task *orderStreamTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.task == task {
		s.task = nil
	}
	s.status.Running = false
}

// normalizeOrderStreamRequest 补齐默认值，兼容测试直接构造请求的场景。
func normalizeOrderStreamRequest(request OrderStreamRequest) OrderStreamRequest {
	if request.Interval <= 0 {
		request.Interval = defaultStreamInterval
	}
	if request.ClOrdIDPrefix == "" {
		request.ClOrdIDPrefix = defaultStreamClOrdID
	}
	if request.ClOrdIDMode == "" {
		request.ClOrdIDMode = streamClOrdIDModeSequence
	}
	if request.StartSeq == 0 {
		request.StartSeq = defaultStreamStartSeq
	}
	if request.SideMode == "" {
		request.SideMode = streamSideModeFixed
	}
	return request
}

// validateOrderStreamRequest 在启动后台任务前复用现有订单构造校验，避免持续发送无效请求。
func validateOrderStreamRequest(request OrderStreamRequest) error {
	request = normalizeOrderStreamRequest(request)
	if strings.TrimSpace(request.Order.Symbol) == "" && len(request.SymbolSeq) == 0 {
		return fmt.Errorf("缺少必填参数 --symbol 或 --symbol-seq")
	}
	if strings.TrimSpace(request.Order.Side) == "" {
		return fmt.Errorf("缺少必填参数 --side")
	}
	if strings.TrimSpace(request.Order.OrderQty) == "" && len(request.QtySeq) == 0 {
		return fmt.Errorf("缺少必填参数 --qty 或 --qty-seq")
	}
	if streamNeedsLimitPrice(request) && strings.TrimSpace(request.Order.Price) == "" && len(request.PriceSeq) == 0 {
		return fmt.Errorf("缺少必填参数 --price 或 --price-seq")
	}
	_, err := message.BuildNewOrderSingle(streamMessageRequest(buildOrderStreamNewRequest(request, 0)))
	return err
}

// streamNeedsLimitPrice 判断 stream 首笔订单是否应按 limit 单校验价格。
func streamNeedsLimitPrice(request OrderStreamRequest) bool {
	switch strings.TrimSpace(request.Order.OrdType) {
	case "", "limit", message.OrdTypeLimit:
		return true
	default:
		return false
	}
}

func streamMessageRequest(request order.NewRequest) message.NewOrderSingleRequest {
	return message.NewOrderSingleRequest{
		ClOrdID:     request.ClOrdID,
		Symbol:      request.Symbol,
		Side:        request.Side,
		OrderQty:    request.OrderQty,
		Price:       request.Price,
		OrdType:     request.OrdType,
		TimeInForce: request.TimeInForce,
		Tags:        request.Tags,
	}
}

// buildOrderStreamNewRequest 按 stream 规则生成单笔 NewOrder 请求。
func buildOrderStreamNewRequest(request OrderStreamRequest, index int) order.NewRequest {
	request = normalizeOrderStreamRequest(request)
	next := request.Order
	next.ClOrdID = streamClOrdID(request, index)
	if len(request.SymbolSeq) > 0 {
		next.Symbol = request.SymbolSeq[index%len(request.SymbolSeq)]
	}
	if len(request.QtySeq) > 0 {
		next.OrderQty = request.QtySeq[index%len(request.QtySeq)]
	}
	if len(request.PriceSeq) > 0 {
		next.Price = request.PriceSeq[index%len(request.PriceSeq)]
	}
	if request.SideMode == streamSideModeAlternate && next.Side != "" {
		if index%2 == 1 {
			next.Side = alternateSide(next.Side)
		}
	}
	return next
}

func streamClOrdID(request OrderStreamRequest, index int) string {
	switch request.ClOrdIDMode {
	case streamClOrdIDModeRandom:
		return fmt.Sprintf("%s-%s", request.ClOrdIDPrefix, randomHex(8))
	default:
		return fmt.Sprintf("%s-%d", request.ClOrdIDPrefix, request.StartSeq+index)
	}
}

func randomHex(size int) string {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(value)
}

func alternateSide(value string) string {
	switch value {
	case "buy":
		return "sell"
	case "sell":
		return "buy"
	case "1":
		return "2"
	case "2":
		return "1"
	default:
		return value
	}
}
