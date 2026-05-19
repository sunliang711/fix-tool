package trace

import (
	"fmt"
	"strconv"
	"strings"
)

const soh = "\x01"

type positionedField struct {
	Field
	start int
	end   int
	next  int
}

func ParseRaw(raw string) (ParsedMessage, error) {
	normalized := NormalizeRaw(raw)
	fields, err := parseFields(normalized)
	if err != nil {
		return ParsedMessage{}, err
	}
	if len(fields) == 0 {
		return ParsedMessage{}, fmt.Errorf("parse FIX message: empty message")
	}
	publicFields := make([]Field, 0, len(fields))
	for _, field := range fields {
		publicFields = append(publicFields, field.Field)
	}
	bodyLength := validateBodyLength(normalized, fields)
	checkSum := validateCheckSum(normalized, fields)
	return ParsedMessage{
		Raw:             normalized,
		Fields:          publicFields,
		BodyLength:      bodyLength,
		CheckSum:        checkSum,
		BodyLengthValid: bodyLength.Valid,
		CheckSumValid:   checkSum.Valid,
	}, nil
}

func NormalizeRaw(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, soh) {
		return raw
	}
	return strings.ReplaceAll(raw, "|", soh)
}

func DisplayRaw(raw string, delimiter string) string {
	if delimiter == "" {
		delimiter = "|"
	}
	return strings.ReplaceAll(NormalizeRaw(raw), soh, delimiter)
}

func parseFields(raw string) ([]positionedField, error) {
	fields := make([]positionedField, 0)
	for start := 0; start < len(raw); {
		end := strings.Index(raw[start:], soh)
		if end < 0 {
			end = len(raw)
		} else {
			end += start
		}
		if end == start {
			if end == len(raw)-len(soh) {
				break
			}
			return nil, fmt.Errorf("parse FIX message: empty field at offset %d", start)
		}
		part := raw[start:end]
		eq := strings.IndexByte(part, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("parse FIX message: invalid field %q", part)
		}
		tag, err := strconv.Atoi(part[:eq])
		if err != nil || tag <= 0 {
			return nil, fmt.Errorf("parse FIX message: invalid tag %q", part[:eq])
		}
		next := end
		if end < len(raw) {
			next = end + len(soh)
		}
		fields = append(fields, positionedField{
			Field: Field{
				Tag:   tag,
				Value: part[eq+1:],
			},
			start: start,
			end:   end,
			next:  next,
		})
		start = next
	}
	return fields, nil
}

func validateBodyLength(raw string, fields []positionedField) ValidationResult {
	bodyLengthField, ok := firstPositionedField(fields, tagBodyLength)
	if !ok {
		return ValidationResult{Present: false, Valid: false, Detail: "missing BodyLength"}
	}
	expected, err := strconv.Atoi(bodyLengthField.Value)
	if err != nil || expected < 0 {
		return ValidationResult{
			Present:  true,
			Valid:    false,
			Expected: bodyLengthField.Value,
			Detail:   "invalid BodyLength value",
		}
	}
	checkSumField, ok := lastPositionedField(fields, tagCheckSum)
	if !ok {
		return ValidationResult{
			Present:  true,
			Valid:    false,
			Expected: bodyLengthField.Value,
			Detail:   "missing CheckSum",
		}
	}
	actual := checkSumField.start - bodyLengthField.next
	if actual < 0 {
		actual = 0
	}
	return ValidationResult{
		Present:  true,
		Valid:    expected == actual,
		Expected: strconv.Itoa(expected),
		Actual:   strconv.Itoa(actual),
	}
}

func validateCheckSum(raw string, fields []positionedField) ValidationResult {
	checkSumField, ok := lastPositionedField(fields, tagCheckSum)
	if !ok {
		return ValidationResult{Present: false, Valid: false, Detail: "missing CheckSum"}
	}
	if len(checkSumField.Value) != 3 {
		return ValidationResult{
			Present:  true,
			Valid:    false,
			Expected: checkSumField.Value,
			Detail:   "invalid CheckSum value",
		}
	}
	expected, err := strconv.Atoi(checkSumField.Value)
	if err != nil || expected < 0 || expected > 255 {
		return ValidationResult{
			Present:  true,
			Valid:    false,
			Expected: checkSumField.Value,
			Detail:   "invalid CheckSum value",
		}
	}
	actual := 0
	for _, b := range []byte(raw[:checkSumField.start]) {
		actual += int(b)
	}
	actual %= 256
	return ValidationResult{
		Present:  true,
		Valid:    expected == actual,
		Expected: fmt.Sprintf("%03d", expected),
		Actual:   fmt.Sprintf("%03d", actual),
	}
}

func firstPositionedField(fields []positionedField, tag int) (positionedField, bool) {
	for _, field := range fields {
		if field.Tag == tag {
			return field, true
		}
	}
	return positionedField{}, false
}

func lastPositionedField(fields []positionedField, tag int) (positionedField, bool) {
	for i := len(fields) - 1; i >= 0; i-- {
		if fields[i].Tag == tag {
			return fields[i], true
		}
	}
	return positionedField{}, false
}
