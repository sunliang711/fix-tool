package dictionary

import (
	"strings"

	"fix-tool/internal/config"
)

const (
	sourceStandard = "standard"
	sourceCustom   = "custom"
)

type CustomTag struct {
	Tag         int
	Name        string
	Type        string
	Required    bool
	Sensitive   bool
	Enums       map[string]string
	Description string
}

type FieldDefinition struct {
	Tag         int               `json:"tag"`
	Name        string            `json:"name"`
	Type        string            `json:"type,omitempty"`
	Required    bool              `json:"required,omitempty"`
	Sensitive   bool              `json:"sensitive,omitempty"`
	Enums       map[string]string `json:"enums,omitempty"`
	Description string            `json:"description,omitempty"`
	Source      string            `json:"source"`
}

type Dictionary struct {
	fields map[int]FieldDefinition
}

func New(customTags []CustomTag) *Dictionary {
	fields := standardFields()
	for tag, field := range fields {
		field.Source = sourceStandard
		fields[tag] = field
	}
	for _, customTag := range customTags {
		mergeCustomTag(fields, customTag)
	}
	return &Dictionary{fields: fields}
}

func NewFromConfig(customTags []config.CustomTagConfig) *Dictionary {
	tags := make([]CustomTag, 0, len(customTags))
	for _, customTag := range customTags {
		tags = append(tags, CustomTag{
			Tag:         customTag.Tag,
			Name:        customTag.Name,
			Type:        customTag.Type,
			Required:    customTag.Required,
			Sensitive:   customTag.Sensitive,
			Enums:       copyEnums(customTag.Enums),
			Description: customTag.Description,
		})
	}
	return New(tags)
}

func Standard() *Dictionary {
	return New(nil)
}

func (d *Dictionary) Lookup(tag int) (FieldDefinition, bool) {
	if d == nil {
		return FieldDefinition{}, false
	}
	field, ok := d.fields[tag]
	return field, ok
}

func (d *Dictionary) ExplainValue(tag int, value string) (string, bool) {
	field, ok := d.Lookup(tag)
	if !ok || len(field.Enums) == 0 {
		return "", false
	}
	explanation, ok := field.Enums[value]
	if ok {
		return explanation, true
	}
	explanation, ok = field.Enums[strings.ToLower(value)]
	return explanation, ok
}

func (d *Dictionary) IsSensitive(tag int) bool {
	field, ok := d.Lookup(tag)
	if !ok {
		return false
	}
	return field.Sensitive
}

func mergeCustomTag(fields map[int]FieldDefinition, customTag CustomTag) {
	if customTag.Tag <= 0 {
		return
	}
	customTag.Sensitive = customTag.Sensitive || isSensitiveName(customTag.Name)
	existing, ok := fields[customTag.Tag]
	if ok {
		if customTag.Name != "" {
			existing.Name = customTag.Name
		}
		if customTag.Description != "" {
			existing.Description = customTag.Description
		}
		if customTag.Required {
			existing.Required = true
		}
		if customTag.Sensitive {
			existing.Sensitive = true
		}
		existing.Source = sourceStandard
		fields[customTag.Tag] = existing
		return
	}
	fields[customTag.Tag] = FieldDefinition{
		Tag:         customTag.Tag,
		Name:        customTag.Name,
		Type:        customTag.Type,
		Required:    customTag.Required,
		Sensitive:   customTag.Sensitive,
		Enums:       copyEnums(customTag.Enums),
		Description: customTag.Description,
		Source:      sourceCustom,
	}
}

func copyEnums(enums map[string]string) map[string]string {
	if len(enums) == 0 {
		return nil
	}
	copied := make(map[string]string, len(enums))
	for value, name := range enums {
		copied[value] = name
	}
	return copied
}

func isSensitiveName(name string) bool {
	normalized := strings.ToLower(name)
	for _, word := range []string{"password", "passwd", "token", "secret", "signature", "rawdata", "account"} {
		if strings.Contains(normalized, word) {
			return true
		}
	}
	return false
}
