package message

import (
	"strings"
	"testing"
	"time"

	"github.com/quickfixgo/quickfix"
)

func TestBuildOrderMessages(t *testing.T) {
	now := time.Date(2026, 5, 19, 12, 0, 1, 0, time.UTC)

	tests := []struct {
		name      string
		build     func() (*quickfix.Message, error)
		wantType  string
		wantBody  map[int]string
		emptyBody []int
	}{
		{
			name: "new-order-single",
			build: func() (*quickfix.Message, error) {
				return BuildNewOrderSingle(NewOrderSingleRequest{
					ClOrdID:     "C001",
					Symbol:      "AAPL",
					Side:        "buy",
					OrderQty:    "100",
					Price:       "10.25",
					OrdType:     "limit",
					TimeInForce: "day",
					Now:         now,
				})
			},
			wantType: MsgTypeNewOrderSingle,
			wantBody: map[int]string{
				TagClOrdID:      "C001",
				TagHandlInst:    defaultHandlInst,
				TagSymbol:       "AAPL",
				TagSide:         SideBuy,
				TagOrderQty:     "100",
				TagOrdType:      OrdTypeLimit,
				TagPrice:        "10.25",
				TagTimeInForce:  TimeInForceDay,
				TagTransactTime: "20260519-12:00:01.000",
			},
		},
		{
			name: "cancel",
			build: func() (*quickfix.Message, error) {
				return BuildOrderCancelRequest(OrderCancelRequest{
					OrigClOrdID: "C001",
					ClOrdID:     "C002",
					OrderID:     "O001",
					Symbol:      "AAPL",
					Side:        "2",
					Now:         now,
				})
			},
			wantType: MsgTypeOrderCancelRequest,
			wantBody: map[int]string{
				TagOrigClOrdID:  "C001",
				TagClOrdID:      "C002",
				TagOrderID:      "O001",
				TagSymbol:       "AAPL",
				TagSide:         SideSell,
				TagTransactTime: "20260519-12:00:01.000",
			},
		},
		{
			name: "replace",
			build: func() (*quickfix.Message, error) {
				return BuildOrderCancelReplaceRequest(OrderCancelReplaceRequest{
					OrigClOrdID: "C001",
					ClOrdID:     "C003",
					OrderID:     "O001",
					Symbol:      "AAPL",
					Side:        "buy",
					OrderQty:    "200",
					Price:       "10.30",
					OrdType:     "2",
					TimeInForce: "gtc",
					Now:         now,
				})
			},
			wantType: MsgTypeOrderCancelReplaceRequest,
			wantBody: map[int]string{
				TagOrigClOrdID:  "C001",
				TagClOrdID:      "C003",
				TagOrderID:      "O001",
				TagHandlInst:    defaultHandlInst,
				TagSymbol:       "AAPL",
				TagSide:         SideBuy,
				TagOrderQty:     "200",
				TagOrdType:      OrdTypeLimit,
				TagPrice:        "10.30",
				TagTimeInForce:  TimeInForceGoodTillCancel,
				TagTransactTime: "20260519-12:00:01.000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageValue, err := tt.build()
			if err != nil {
				t.Fatalf("build error = %v", err)
			}
			if got := headerValue(t, messageValue, TagMsgType); got != tt.wantType {
				t.Fatalf("msg type = %q, want %q", got, tt.wantType)
			}
			for tagValue, want := range tt.wantBody {
				if got := bodyValue(t, messageValue, tagValue); got != want {
					t.Fatalf("tag %d = %q, want %q", tagValue, got, want)
				}
			}
			for _, tagValue := range tt.emptyBody {
				if got := optionalBodyValue(messageValue, tagValue); got != "" {
					t.Fatalf("tag %d = %q, want empty", tagValue, got)
				}
			}
		})
	}
}

func TestNormalizeEnums(t *testing.T) {
	sideTests := map[string]string{
		"buy":  SideBuy,
		"1":    SideBuy,
		"sell": SideSell,
		"2":    SideSell,
	}
	for value, want := range sideTests {
		got, err := NormalizeSide(value)
		if err != nil {
			t.Fatalf("NormalizeSide(%q) error = %v", value, err)
		}
		if got != want {
			t.Fatalf("NormalizeSide(%q) = %q, want %q", value, got, want)
		}
	}

	ordTypeTests := map[string]string{
		"market": OrdTypeMarket,
		"1":      OrdTypeMarket,
		"limit":  OrdTypeLimit,
		"2":      OrdTypeLimit,
	}
	for value, want := range ordTypeTests {
		got, err := NormalizeOrdType(value)
		if err != nil {
			t.Fatalf("NormalizeOrdType(%q) error = %v", value, err)
		}
		if got != want {
			t.Fatalf("NormalizeOrdType(%q) = %q, want %q", value, got, want)
		}
	}

	timeInForceTests := map[string]string{
		"day": TimeInForceDay,
		"0":   TimeInForceDay,
		"gtc": TimeInForceGoodTillCancel,
		"1":   TimeInForceGoodTillCancel,
		"ioc": TimeInForceImmediateOrCancel,
		"3":   TimeInForceImmediateOrCancel,
		"fok": TimeInForceFillOrKill,
		"4":   TimeInForceFillOrKill,
	}
	for value, want := range timeInForceTests {
		got, err := NormalizeTimeInForce(value)
		if err != nil {
			t.Fatalf("NormalizeTimeInForce(%q) error = %v", value, err)
		}
		if got != want {
			t.Fatalf("NormalizeTimeInForce(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestCustomTags(t *testing.T) {
	messageValue, err := BuildNewOrderSingle(NewOrderSingleRequest{
		ClOrdID:  "C001",
		Symbol:   "AAPL",
		Side:     "buy",
		OrderQty: "100",
		Price:    "10.25",
		Tags:     []string{"9001=abc=def"},
	})
	if err != nil {
		t.Fatalf("BuildNewOrderSingle() error = %v", err)
	}
	if got := bodyValue(t, messageValue, 9001); got != "abc=def" {
		t.Fatalf("tag 9001 = %q, want abc=def", got)
	}
	if got := bodyValue(t, messageValue, TagPrice); got != "10.25" {
		t.Fatalf("tag 44 = %q, want original price", got)
	}

	for _, raw := range []string{
		"8=FIX.4.4",
		"9=1",
		"10=000",
		"34=1",
		"35=8",
		"49=SENDER",
		"52=20260519-12:00:01.000",
		"56=TARGET",
		"11=C999",
		"21=3",
		"37=O999",
		"38=999",
		"40=1",
		"41=C998",
		"44=10.99",
		"54=2",
		"55=MSFT",
		"59=1",
		"60=20260519-12:00:01.000",
	} {
		if _, err := ParseCustomTags([]string{raw}); err == nil {
			t.Fatalf("ParseCustomTags(%q) error = nil, want protected tag error", raw)
		}
	}
}

func TestRequiredErrorsAreChinese(t *testing.T) {
	_, err := BuildNewOrderSingle(NewOrderSingleRequest{
		ClOrdID:  "C001",
		Side:     "buy",
		OrderQty: "100",
		Price:    "10.25",
	})
	if err == nil || !strings.Contains(err.Error(), "缺少必填参数 --symbol") {
		t.Fatalf("missing symbol error = %v, want Chinese message", err)
	}

	_, err = BuildNewOrderSingle(NewOrderSingleRequest{
		ClOrdID:  "C001",
		Symbol:   "AAPL",
		Side:     "buy",
		OrderQty: "100",
	})
	if err == nil || !strings.Contains(err.Error(), "缺少必填参数 --price") {
		t.Fatalf("missing price error = %v, want Chinese message", err)
	}

	_, err = BuildOrderCancelReplaceRequest(OrderCancelReplaceRequest{
		OrigClOrdID: "C001",
		ClOrdID:     "C003",
		Side:        "buy",
		OrderQty:    "200",
		Price:       "10.30",
	})
	if err == nil || !strings.Contains(err.Error(), "缺少必填参数 --symbol") {
		t.Fatalf("missing replace symbol error = %v, want Chinese message", err)
	}

	_, err = BuildOrderCancelReplaceRequest(OrderCancelReplaceRequest{
		OrigClOrdID: "C001",
		ClOrdID:     "C003",
		Symbol:      "AAPL",
		OrderQty:    "200",
		Price:       "10.30",
	})
	if err == nil || !strings.Contains(err.Error(), "缺少必填参数 --side") {
		t.Fatalf("missing replace side error = %v, want Chinese message", err)
	}
}

func headerValue(t *testing.T, messageValue *quickfix.Message, tagValue int) string {
	value, err := messageValue.Header.GetString(quickfix.Tag(tagValue))
	if err != nil {
		t.Fatalf("header tag %d error = %v", tagValue, err)
	}
	return value
}

func bodyValue(t *testing.T, messageValue *quickfix.Message, tagValue int) string {
	value, err := messageValue.Body.GetString(quickfix.Tag(tagValue))
	if err != nil {
		t.Fatalf("body tag %d error = %v", tagValue, err)
	}
	return value
}

func optionalBodyValue(messageValue *quickfix.Message, tagValue int) string {
	value, err := messageValue.Body.GetString(quickfix.Tag(tagValue))
	if err != nil {
		return ""
	}
	return value
}
