package message

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/quickfixgo/quickfix"
)

const (
	MsgTypeNewOrderSingle            = "D"
	MsgTypeOrderCancelRequest        = "F"
	MsgTypeOrderCancelReplaceRequest = "G"
	MsgTypeExecutionReport           = "8"
	MsgTypeReject                    = "3"
	MsgTypeBusinessMessageReject     = "j"
	SideBuy                          = "1"
	SideSell                         = "2"
	OrdTypeMarket                    = "1"
	OrdTypeLimit                     = "2"
	TimeInForceDay                   = "0"
	TimeInForceGoodTillCancel        = "1"
	TimeInForceImmediateOrCancel     = "3"
	TimeInForceFillOrKill            = "4"
	defaultHandlInst                 = "1"
	defaultTransactTimeFormat        = "20060102-15:04:05.000"
	defaultNewClOrdIDPrefix          = "NEW"
	defaultCancelClOrdIDPrefix       = "CXL"
	defaultReplaceClOrdIDPrefix      = "RPL"
)

const (
	TagBeginString  = 8
	TagBodyLength   = 9
	TagCheckSum     = 10
	TagClOrdID      = 11
	TagHandlInst    = 21
	TagMsgSeqNum    = 34
	TagMsgType      = 35
	TagOrderID      = 37
	TagOrderQty     = 38
	TagOrdType      = 40
	TagOrigClOrdID  = 41
	TagPrice        = 44
	TagSenderCompID = 49
	TagSendingTime  = 52
	TagSide         = 54
	TagSymbol       = 55
	TagTargetCompID = 56
	TagTimeInForce  = 59
	TagTransactTime = 60
)

type CustomTag struct {
	Tag   int
	Value string
}

type NewOrderSingleRequest struct {
	ClOrdID     string
	Symbol      string
	Side        string
	OrderQty    string
	Price       string
	OrdType     string
	TimeInForce string
	Tags        []string
	Now         time.Time
}

type OrderCancelRequest struct {
	OrigClOrdID string
	ClOrdID     string
	OrderID     string
	Symbol      string
	Side        string
	Tags        []string
	Now         time.Time
}

type OrderCancelReplaceRequest struct {
	OrigClOrdID string
	ClOrdID     string
	OrderID     string
	Symbol      string
	Side        string
	OrderQty    string
	Price       string
	OrdType     string
	TimeInForce string
	Tags        []string
	Now         time.Time
}

func BuildNewOrderSingle(request NewOrderSingleRequest) (*quickfix.Message, error) {
	clOrdID := strings.TrimSpace(request.ClOrdID)
	if clOrdID == "" {
		clOrdID = generatedClOrdID(defaultNewClOrdIDPrefix)
	}
	symbol, err := requiredValue(request.Symbol, "symbol")
	if err != nil {
		return nil, err
	}
	side, err := NormalizeSide(request.Side)
	if err != nil {
		return nil, err
	}
	orderQty, err := positiveDecimal(request.OrderQty, "qty")
	if err != nil {
		return nil, err
	}
	ordType, err := normalizeOrdTypeWithDefault(request.OrdType)
	if err != nil {
		return nil, err
	}
	price, err := optionalPrice(request.Price, ordType)
	if err != nil {
		return nil, err
	}
	timeInForce, err := NormalizeTimeInForce(request.TimeInForce)
	if err != nil {
		return nil, err
	}

	message := newOrderMessage(MsgTypeNewOrderSingle, request.Now)
	message.Body.SetString(tag(TagClOrdID), clOrdID)
	message.Body.SetString(tag(TagHandlInst), defaultHandlInst)
	message.Body.SetString(tag(TagSymbol), symbol)
	message.Body.SetString(tag(TagSide), side)
	message.Body.SetString(tag(TagOrderQty), orderQty)
	message.Body.SetString(tag(TagOrdType), ordType)
	if price != "" {
		message.Body.SetString(tag(TagPrice), price)
	}
	if timeInForce != "" {
		message.Body.SetString(tag(TagTimeInForce), timeInForce)
	}
	if err := applyCustomTags(message, request.Tags); err != nil {
		return nil, err
	}
	return message, nil
}

func BuildOrderCancelRequest(request OrderCancelRequest) (*quickfix.Message, error) {
	origClOrdID, err := requiredValue(request.OrigClOrdID, "orig-cl-ord-id")
	if err != nil {
		return nil, err
	}
	clOrdID := strings.TrimSpace(request.ClOrdID)
	if clOrdID == "" {
		clOrdID = generatedClOrdID(defaultCancelClOrdIDPrefix)
	}
	symbol, err := requiredValue(request.Symbol, "symbol")
	if err != nil {
		return nil, err
	}
	side, err := NormalizeSide(request.Side)
	if err != nil {
		return nil, err
	}

	message := newOrderMessage(MsgTypeOrderCancelRequest, request.Now)
	message.Body.SetString(tag(TagOrigClOrdID), origClOrdID)
	message.Body.SetString(tag(TagClOrdID), clOrdID)
	if orderID := strings.TrimSpace(request.OrderID); orderID != "" {
		message.Body.SetString(tag(TagOrderID), orderID)
	}
	message.Body.SetString(tag(TagSymbol), symbol)
	message.Body.SetString(tag(TagSide), side)
	if err := applyCustomTags(message, request.Tags); err != nil {
		return nil, err
	}
	return message, nil
}

func BuildOrderCancelReplaceRequest(request OrderCancelReplaceRequest) (*quickfix.Message, error) {
	origClOrdID, err := requiredValue(request.OrigClOrdID, "orig-cl-ord-id")
	if err != nil {
		return nil, err
	}
	clOrdID := strings.TrimSpace(request.ClOrdID)
	if clOrdID == "" {
		clOrdID = generatedClOrdID(defaultReplaceClOrdIDPrefix)
	}
	symbol, err := requiredValue(request.Symbol, "symbol")
	if err != nil {
		return nil, err
	}
	side, err := NormalizeSide(request.Side)
	if err != nil {
		return nil, err
	}
	orderQty, err := positiveDecimal(request.OrderQty, "qty")
	if err != nil {
		return nil, err
	}
	ordType, err := normalizeOrdTypeWithDefault(request.OrdType)
	if err != nil {
		return nil, err
	}
	price, err := optionalPrice(request.Price, ordType)
	if err != nil {
		return nil, err
	}
	timeInForce, err := NormalizeTimeInForce(request.TimeInForce)
	if err != nil {
		return nil, err
	}

	message := newOrderMessage(MsgTypeOrderCancelReplaceRequest, request.Now)
	message.Body.SetString(tag(TagOrigClOrdID), origClOrdID)
	message.Body.SetString(tag(TagClOrdID), clOrdID)
	if orderID := strings.TrimSpace(request.OrderID); orderID != "" {
		message.Body.SetString(tag(TagOrderID), orderID)
	}
	message.Body.SetString(tag(TagSymbol), symbol)
	message.Body.SetString(tag(TagSide), side)
	message.Body.SetString(tag(TagOrderQty), orderQty)
	message.Body.SetString(tag(TagOrdType), ordType)
	message.Body.SetString(tag(TagHandlInst), defaultHandlInst)
	if price != "" {
		message.Body.SetString(tag(TagPrice), price)
	}
	if timeInForce != "" {
		message.Body.SetString(tag(TagTimeInForce), timeInForce)
	}
	if err := applyCustomTags(message, request.Tags); err != nil {
		return nil, err
	}
	return message, nil
}

func NormalizeSide(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", fmt.Errorf("缺少必填参数 --side")
	}
	switch value {
	case "buy", SideBuy:
		return SideBuy, nil
	case "sell", SideSell:
		return SideSell, nil
	default:
		return "", fmt.Errorf("参数 --side 必须是 buy、sell、1 或 2")
	}
}

func NormalizeOrdType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "market", OrdTypeMarket:
		return OrdTypeMarket, nil
	case "limit", OrdTypeLimit:
		return OrdTypeLimit, nil
	default:
		return "", fmt.Errorf("参数 --ord-type 必须是 market、limit、1 或 2")
	}
}

func NormalizeTimeInForce(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "day", TimeInForceDay:
		return TimeInForceDay, nil
	case "gtc", TimeInForceGoodTillCancel:
		return TimeInForceGoodTillCancel, nil
	case "ioc", TimeInForceImmediateOrCancel:
		return TimeInForceImmediateOrCancel, nil
	case "fok", TimeInForceFillOrKill:
		return TimeInForceFillOrKill, nil
	default:
		return "", fmt.Errorf("参数 --time-in-force 必须是 day、gtc、ioc、fok、0、1、3 或 4")
	}
}

func ParseCustomTags(rawTags []string) ([]CustomTag, error) {
	tags := make([]CustomTag, 0, len(rawTags))
	for _, raw := range rawTags {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("参数 --tag 必须使用 key=value 格式")
		}
		tagValue, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil || tagValue <= 0 {
			return nil, fmt.Errorf("参数 --tag 的 key 必须是正整数 tag")
		}
		if isProtectedTag(tagValue) {
			return nil, fmt.Errorf("参数 --tag 不允许覆盖协议字段 %d", tagValue)
		}
		tags = append(tags, CustomTag{Tag: tagValue, Value: value})
	}
	return tags, nil
}

func normalizeOrdTypeWithDefault(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return OrdTypeLimit, nil
	}
	return NormalizeOrdType(value)
}

func optionalPrice(value string, ordType string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && ordType == OrdTypeLimit {
		return "", fmt.Errorf("缺少必填参数 --price")
	}
	if value == "" {
		return "", nil
	}
	return positiveDecimal(value, "price")
}

func requiredValue(value string, flag string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("缺少必填参数 --%s", flag)
	}
	return value, nil
}

func positiveDecimal(value string, flag string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("缺少必填参数 --%s", flag)
	}
	decimal, ok := new(big.Rat).SetString(value)
	if !ok || decimal.Sign() <= 0 {
		return "", fmt.Errorf("参数 --%s 必须是正数", flag)
	}
	return value, nil
}

func applyCustomTags(message *quickfix.Message, rawTags []string) error {
	tags, err := ParseCustomTags(rawTags)
	if err != nil {
		return err
	}
	for _, customTag := range tags {
		message.Body.SetString(tag(customTag.Tag), customTag.Value)
	}
	return nil
}

func newOrderMessage(msgType string, now time.Time) *quickfix.Message {
	message := quickfix.NewMessage()
	message.Header.SetString(tag(TagMsgType), msgType)
	message.Body.SetString(tag(TagTransactTime), transactTime(now))
	return message
}

func transactTime(now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return now.UTC().Format(defaultTransactTimeFormat)
}

func generatedClOrdID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

func isProtectedTag(value int) bool {
	switch value {
	case TagBeginString, TagBodyLength, TagCheckSum, TagClOrdID, TagHandlInst,
		TagMsgSeqNum, TagMsgType, TagOrderID, TagOrderQty, TagOrdType,
		TagOrigClOrdID, TagPrice, TagSenderCompID, TagSendingTime, TagSide,
		TagSymbol, TagTargetCompID, TagTimeInForce, TagTransactTime:
		return true
	default:
		return false
	}
}

func tag(value int) quickfix.Tag {
	return quickfix.Tag(value)
}
