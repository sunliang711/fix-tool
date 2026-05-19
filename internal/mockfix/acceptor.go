package mockfix

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	appconfig "fix-tool/internal/config"
	"fix-tool/internal/message"

	"github.com/quickfixgo/quickfix"
	qfconfig "github.com/quickfixgo/quickfix/config"
)

const (
	defaultHost              = "127.0.0.1"
	defaultProfileName       = "mock"
	defaultBeginString       = "FIX.4.4"
	defaultInitiatorCompID   = "SENDER"
	defaultAcceptorCompID    = "TARGET"
	defaultHeartbeatInterval = 2 * time.Second

	ReplaceExecutionReport = "execution_report"
	ReplaceSessionReject   = "reject"
	ReplaceBusinessReject  = "business_reject"

	SymbolSessionReject   = "MOCK-REJECT"
	SymbolBusinessReject  = "MOCK-BUSINESS-REJECT"
	adminMsgTypeHeartbeat = "0"
	adminMsgTypeTestReq   = "1"
)

const (
	tagCumQty                = 14
	tagExecID                = 17
	tagOrdStatus             = 39
	tagRefSeqNum             = 45
	tagText                  = 58
	tagExecType              = 150
	tagLeavesQty             = 151
	tagBusinessRejectRefID   = 379
	tagBusinessRejectReason  = 380
	tagBusinessRejectRefType = 372
	tagTestReqID             = 112
)

type Options struct {
	Host              string
	Port              int
	ProfileName       string
	BeginString       string
	InitiatorCompID   string
	AcceptorCompID    string
	HeartbeatInterval time.Duration
	ReplaceResponse   string
}

type Acceptor struct {
	mu       sync.Mutex
	options  Options
	app      *application
	acceptor *quickfix.Acceptor
	started  bool
}

func NewAcceptor(options Options) (*Acceptor, error) {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	if normalized.Port == 0 {
		port, err := freePort(normalized.Host)
		if err != nil {
			return nil, err
		}
		normalized.Port = port
	}

	settings, err := settingsFromOptions(normalized)
	if err != nil {
		return nil, err
	}
	app := newApplication(normalized)
	acceptor, err := quickfix.NewAcceptor(app, quickfix.NewMemoryStoreFactory(), settings, quickfix.NewNullLogFactory())
	if err != nil {
		return nil, fmt.Errorf("create mock fix acceptor: %w", err)
	}
	return &Acceptor{
		options:  normalized,
		app:      app,
		acceptor: acceptor,
	}, nil
}

func (a *Acceptor) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("start mock fix acceptor: %w", err)
	}
	if a == nil || a.acceptor == nil {
		return fmt.Errorf("mock fix acceptor is nil")
	}

	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return nil
	}
	if err := a.acceptor.Start(); err != nil {
		a.mu.Unlock()
		return fmt.Errorf("start mock fix acceptor: %w", err)
	}
	a.started = true
	a.mu.Unlock()
	return nil
}

func (a *Acceptor) Stop(ctx context.Context) error {
	if a == nil || a.acceptor == nil {
		return nil
	}

	a.mu.Lock()
	if !a.started {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	done := make(chan struct{})
	go func() {
		a.acceptor.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("stop mock fix acceptor: %w", ctx.Err())
	}

	a.mu.Lock()
	a.started = false
	a.mu.Unlock()
	return nil
}

func (a *Acceptor) Port() int {
	if a == nil {
		return 0
	}
	return a.options.Port
}

func (a *Acceptor) ProfileConfig() appconfig.ProfileConfig {
	if a == nil {
		return appconfig.ProfileConfig{}
	}
	return appconfig.ProfileConfig{
		Name:              a.options.ProfileName,
		BeginString:       a.options.BeginString,
		SenderCompID:      a.options.InitiatorCompID,
		TargetCompID:      a.options.AcceptorCompID,
		Host:              a.options.Host,
		Port:              a.options.Port,
		HeartbeatInterval: a.options.HeartbeatInterval.String(),
		ResetOnLogon:      true,
		TLS: appconfig.TLSConfig{
			Enabled: false,
		},
	}
}

func normalizeOptions(options Options) (Options, error) {
	if strings.TrimSpace(options.Host) == "" {
		options.Host = defaultHost
	}
	if strings.TrimSpace(options.ProfileName) == "" {
		options.ProfileName = defaultProfileName
	}
	if strings.TrimSpace(options.BeginString) == "" {
		options.BeginString = defaultBeginString
	}
	if strings.TrimSpace(options.InitiatorCompID) == "" {
		options.InitiatorCompID = defaultInitiatorCompID
	}
	if strings.TrimSpace(options.AcceptorCompID) == "" {
		options.AcceptorCompID = defaultAcceptorCompID
	}
	if options.HeartbeatInterval <= 0 {
		options.HeartbeatInterval = defaultHeartbeatInterval
	}
	if options.HeartbeatInterval < time.Second {
		return Options{}, fmt.Errorf("mock fix heartbeat interval must be at least one second")
	}
	if options.HeartbeatInterval%time.Second != 0 {
		return Options{}, fmt.Errorf("mock fix heartbeat interval must use whole seconds")
	}
	options.ReplaceResponse = normalizeReplaceResponse(options.ReplaceResponse)
	return options, nil
}

func normalizeReplaceResponse(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ReplaceSessionReject:
		return ReplaceSessionReject
	case ReplaceBusinessReject:
		return ReplaceBusinessReject
	default:
		return ReplaceExecutionReport
	}
}

func settingsFromOptions(options Options) (*quickfix.Settings, error) {
	settings := quickfix.NewSettings()
	settings.GlobalSettings().Set("ConnectionType", "acceptor")
	settings.GlobalSettings().Set(qfconfig.SocketAcceptHost, options.Host)
	settings.GlobalSettings().Set(qfconfig.SocketAcceptPort, strconv.Itoa(options.Port))

	sessionSettings := quickfix.NewSessionSettings()
	sessionSettings.Set(qfconfig.BeginString, options.BeginString)
	// Acceptor 视角的 Sender/Target 与客户端 profile 正好相反。
	sessionSettings.Set(qfconfig.SenderCompID, options.AcceptorCompID)
	sessionSettings.Set(qfconfig.TargetCompID, options.InitiatorCompID)
	sessionSettings.Set(qfconfig.HeartBtInt, strconv.Itoa(int(options.HeartbeatInterval/time.Second)))
	sessionSettings.Set(qfconfig.ResetOnLogon, "Y")

	if _, err := settings.AddSession(sessionSettings); err != nil {
		return nil, fmt.Errorf("add mock fix session settings: %w", err)
	}
	return settings, nil
}

func freePort(host string) (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, fmt.Errorf("allocate mock fix port: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("allocate mock fix port: unexpected address %s", listener.Addr())
	}
	return tcpAddr.Port, nil
}

type application struct {
	options     Options
	mu          sync.Mutex
	nextOrderID int
	nextExecID  int
	orders      map[string]string
}

func newApplication(options Options) *application {
	return &application{
		options: options,
		orders:  make(map[string]string),
	}
}

func (a *application) OnCreate(quickfix.SessionID) {
}

func (a *application) OnLogon(quickfix.SessionID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextOrderID = 0
	a.nextExecID = 0
	a.orders = make(map[string]string)
}

func (a *application) OnLogout(quickfix.SessionID) {
}

func (a *application) ToAdmin(*quickfix.Message, quickfix.SessionID) {
}

func (a *application) ToApp(*quickfix.Message, quickfix.SessionID) error {
	return nil
}

func (a *application) FromAdmin(value *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	switch headerString(value, message.TagMsgType) {
	case adminMsgTypeHeartbeat:
		a.sendAdminAsync(sessionID, adminMsgTypeHeartbeat, "")
	case adminMsgTypeTestReq:
		a.sendAdminAsync(sessionID, adminMsgTypeHeartbeat, bodyString(value, tagTestReqID))
	}
	return nil
}

func (a *application) FromApp(value *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	var err error
	switch headerString(value, message.TagMsgType) {
	case message.MsgTypeNewOrderSingle:
		err = a.respondOrder(sessionID, value, "0", "0")
	case message.MsgTypeOrderCancelRequest:
		err = a.respondOrder(sessionID, value, "4", "4")
	case message.MsgTypeOrderCancelReplaceRequest:
		err = a.respondReplace(sessionID, value)
	default:
		err = a.sendBusinessReject(sessionID, value, "Mock unsupported message type")
	}
	if err != nil {
		return quickfix.NewBusinessMessageRejectError("Mock response send failed", 0, nil)
	}
	return nil
}

func (a *application) respondReplace(sessionID quickfix.SessionID, value *quickfix.Message) error {
	switch responseKind(a.options.ReplaceResponse, value) {
	case ReplaceSessionReject:
		return a.sendSessionReject(sessionID, value, "Mock replace session reject")
	case ReplaceBusinessReject:
		return a.sendBusinessReject(sessionID, value, "Mock replace business reject")
	default:
		return a.respondOrder(sessionID, value, "5", "0")
	}
}

func (a *application) respondOrder(sessionID quickfix.SessionID, value *quickfix.Message, execType string, ordStatus string) error {
	switch responseKind(ReplaceExecutionReport, value) {
	case ReplaceSessionReject:
		return a.sendSessionReject(sessionID, value, "Mock session reject")
	case ReplaceBusinessReject:
		return a.sendBusinessReject(sessionID, value, "Mock business reject")
	}

	response := quickfix.NewMessage()
	response.Header.SetString(quickfix.Tag(message.TagMsgType), message.MsgTypeExecutionReport)
	clOrdID := bodyString(value, message.TagClOrdID)
	origClOrdID := bodyString(value, message.TagOrigClOrdID)
	orderID := a.orderIDFor(clOrdID, origClOrdID, bodyString(value, message.TagOrderID))
	orderQty := bodyString(value, message.TagOrderQty)
	if orderQty == "" {
		orderQty = "0"
	}

	response.Body.SetString(quickfix.Tag(message.TagOrderID), orderID)
	response.Body.SetString(quickfix.Tag(tagExecID), a.execID())
	response.Body.SetString(quickfix.Tag(message.TagClOrdID), clOrdID)
	if origClOrdID != "" {
		response.Body.SetString(quickfix.Tag(message.TagOrigClOrdID), origClOrdID)
	}
	copyBodyString(response, value, message.TagSymbol)
	copyBodyString(response, value, message.TagSide)
	copyBodyString(response, value, message.TagOrderQty)
	copyBodyString(response, value, message.TagPrice)
	response.Body.SetString(quickfix.Tag(tagExecType), execType)
	response.Body.SetString(quickfix.Tag(tagOrdStatus), ordStatus)
	response.Body.SetString(quickfix.Tag(tagCumQty), "0")
	response.Body.SetString(quickfix.Tag(tagLeavesQty), orderQty)
	return quickfix.SendToTarget(response, sessionID)
}

func (a *application) sendBusinessReject(sessionID quickfix.SessionID, value *quickfix.Message, text string) error {
	response := quickfix.NewMessage()
	response.Header.SetString(quickfix.Tag(message.TagMsgType), message.MsgTypeBusinessMessageReject)
	response.Body.SetString(quickfix.Tag(tagBusinessRejectRefType), headerString(value, message.TagMsgType))
	if clOrdID := bodyString(value, message.TagClOrdID); clOrdID != "" {
		response.Body.SetString(quickfix.Tag(tagBusinessRejectRefID), clOrdID)
	}
	response.Body.SetString(quickfix.Tag(tagBusinessRejectReason), "0")
	response.Body.SetString(quickfix.Tag(tagText), text)
	return quickfix.SendToTarget(response, sessionID)
}

func (a *application) sendSessionReject(sessionID quickfix.SessionID, value *quickfix.Message, text string) error {
	response := quickfix.NewMessage()
	response.Header.SetString(quickfix.Tag(message.TagMsgType), message.MsgTypeReject)
	response.Body.SetString(quickfix.Tag(tagRefSeqNum), headerString(value, message.TagMsgSeqNum))
	response.Body.SetString(quickfix.Tag(tagBusinessRejectRefType), headerString(value, message.TagMsgType))
	response.Body.SetString(quickfix.Tag(tagText), text)
	return quickfix.SendToTarget(response, sessionID)
}

func (a *application) sendAdminAsync(sessionID quickfix.SessionID, msgType string, testReqID string) {
	go func() {
		response := quickfix.NewMessage()
		response.Header.SetString(quickfix.Tag(message.TagMsgType), msgType)
		if testReqID != "" {
			response.Body.SetString(quickfix.Tag(tagTestReqID), testReqID)
		}
		_ = quickfix.SendToTarget(response, sessionID)
	}()
}

func responseKind(defaultKind string, value *quickfix.Message) string {
	switch strings.ToUpper(bodyString(value, message.TagSymbol)) {
	case SymbolSessionReject:
		return ReplaceSessionReject
	case SymbolBusinessReject:
		return ReplaceBusinessReject
	default:
		return defaultKind
	}
}

func (a *application) orderIDFor(clOrdID string, origClOrdID string, provided string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if provided != "" {
		if clOrdID != "" {
			a.orders[clOrdID] = provided
		}
		return provided
	}
	if origClOrdID != "" {
		if orderID := a.orders[origClOrdID]; orderID != "" {
			if clOrdID != "" {
				a.orders[clOrdID] = orderID
			}
			return orderID
		}
	}
	if clOrdID != "" {
		if orderID := a.orders[clOrdID]; orderID != "" {
			return orderID
		}
	}
	a.nextOrderID++
	orderID := fmt.Sprintf("MOCK-ORDER-%06d", a.nextOrderID)
	if clOrdID != "" {
		a.orders[clOrdID] = orderID
	}
	return orderID
}

func (a *application) execID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextExecID++
	return fmt.Sprintf("MOCK-EXEC-%06d", a.nextExecID)
}

func copyBodyString(dst *quickfix.Message, src *quickfix.Message, tagValue int) {
	value := bodyString(src, tagValue)
	if value == "" {
		return
	}
	dst.Body.SetString(quickfix.Tag(tagValue), value)
}

func headerString(value *quickfix.Message, tagValue int) string {
	result, err := value.Header.GetString(quickfix.Tag(tagValue))
	if err != nil {
		return ""
	}
	return result
}

func bodyString(value *quickfix.Message, tagValue int) string {
	result, err := value.Body.GetString(quickfix.Tag(tagValue))
	if err != nil {
		return ""
	}
	return result
}
