package raw

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/quickfixgo/quickfix"
)

const (
	tagBeginString  = 8
	tagBodyLength   = 9
	tagCheckSum     = 10
	tagMsgSeqNum    = 34
	tagMsgType      = 35
	tagSenderCompID = 49
	tagSendingTime  = 52
	tagTargetCompID = 56
	soh             = "\x01"
)

type Request struct {
	MsgType string
	Tags    []string
}

type CustomTag struct {
	Tag   int
	Value string
}

func BuildMessage(request Request) (*quickfix.Message, error) {
	msgType, tags, err := ValidateRequest(request)
	if err != nil {
		return nil, err
	}

	message := quickfix.NewMessage()
	message.Header.SetString(quickfix.Tag(tagMsgType), msgType)
	for _, customTag := range tags {
		message.Body.SetString(quickfix.Tag(customTag.Tag), customTag.Value)
	}
	return message, nil
}

func ValidateRequest(request Request) (string, []CustomTag, error) {
	msgType := strings.TrimSpace(request.MsgType)
	if msgType == "" {
		return "", nil, fmt.Errorf("缺少必填参数 --msg-type")
	}
	if strings.ContainsAny(msgType, "=\x01|") {
		return "", nil, fmt.Errorf("参数 --msg-type 不能包含 =、SOH 或 |")
	}
	tags, err := ParseTags(request.Tags)
	if err != nil {
		return "", nil, err
	}
	return msgType, tags, nil
}

func ParseTags(rawTags []string) ([]CustomTag, error) {
	tags := make([]CustomTag, 0, len(rawTags))
	for _, rawTag := range rawTags {
		key, value, ok := strings.Cut(rawTag, "=")
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
		if strings.Contains(value, soh) {
			return nil, fmt.Errorf("参数 --tag 的 value 不能包含真实 SOH 分隔符")
		}
		tags = append(tags, CustomTag{Tag: tagValue, Value: value})
	}
	return tags, nil
}

func isProtectedTag(tag int) bool {
	switch tag {
	case tagBeginString, tagBodyLength, tagCheckSum, tagMsgSeqNum,
		tagMsgType, tagSenderCompID, tagSendingTime, tagTargetCompID:
		return true
	default:
		return false
	}
}
