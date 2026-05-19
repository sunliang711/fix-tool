package dictionary

import (
	"strings"

	"fix-tool/internal/config"
)

const (
	sourceStandard = "standard"
	sourceCustom   = "custom"
)

type CustomFieldDef struct {
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

func New(customFieldDefs []CustomFieldDef) *Dictionary {
	fields := standardFields()
	for tag, field := range fields {
		field.Source = sourceStandard
		fields[tag] = field
	}
	for _, customFieldDef := range customFieldDefs {
		mergeCustomFieldDef(fields, customFieldDef)
	}
	return &Dictionary{fields: fields}
}

func NewFromConfig(customFieldDefs []config.CustomFieldDefConfig) *Dictionary {
	defs := make([]CustomFieldDef, 0, len(customFieldDefs))
	for _, customFieldDef := range customFieldDefs {
		defs = append(defs, CustomFieldDef{
			Tag:         customFieldDef.Tag,
			Name:        customFieldDef.Name,
			Type:        customFieldDef.Type,
			Required:    customFieldDef.Required,
			Sensitive:   customFieldDef.Sensitive,
			Enums:       copyEnums(customFieldDef.Enums),
			Description: customFieldDef.Description,
		})
	}
	return New(defs)
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

func mergeCustomFieldDef(fields map[int]FieldDefinition, customFieldDef CustomFieldDef) {
	if customFieldDef.Tag <= 0 {
		return
	}
	customFieldDef.Sensitive = customFieldDef.Sensitive || isSensitiveName(customFieldDef.Name)
	existing, ok := fields[customFieldDef.Tag]
	if ok {
		if customFieldDef.Name != "" {
			existing.Name = customFieldDef.Name
		}
		if customFieldDef.Description != "" {
			existing.Description = customFieldDef.Description
		}
		if customFieldDef.Required {
			existing.Required = true
		}
		if customFieldDef.Sensitive {
			existing.Sensitive = true
		}
		existing.Source = sourceStandard
		fields[customFieldDef.Tag] = existing
		return
	}
	fields[customFieldDef.Tag] = FieldDefinition{
		Tag:         customFieldDef.Tag,
		Name:        customFieldDef.Name,
		Type:        customFieldDef.Type,
		Required:    customFieldDef.Required,
		Sensitive:   customFieldDef.Sensitive,
		Enums:       copyEnums(customFieldDef.Enums),
		Description: customFieldDef.Description,
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
