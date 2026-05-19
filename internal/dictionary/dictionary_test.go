package dictionary

import (
	"testing"

	"fix-tool/internal/config"
)

func TestDictionaryResolvesStandardFieldsAndEnums(t *testing.T) {
	dict := Standard()

	field, ok := dict.Lookup(35)
	if !ok {
		t.Fatal("Lookup(35) ok = false, want true")
	}
	if field.Name != "MsgType" {
		t.Fatalf("field name = %q, want %q", field.Name, "MsgType")
	}
	enum, ok := dict.ExplainValue(35, "D")
	if !ok {
		t.Fatal("ExplainValue(35, D) ok = false, want true")
	}
	if enum != "NewOrderSingle" {
		t.Fatalf("enum = %q, want %q", enum, "NewOrderSingle")
	}
	if !dict.IsSensitive(553) {
		t.Fatal("Username tag is not sensitive")
	}
}

func TestDictionaryAppliesCustomTags(t *testing.T) {
	dict := New([]CustomTag{
		{
			Tag:       9001,
			Name:      "SessionToken",
			Type:      "STRING",
			Sensitive: true,
			Enums: map[string]string{
				"A": "Alpha",
			},
		},
	})

	field, ok := dict.Lookup(9001)
	if !ok {
		t.Fatal("Lookup(9001) ok = false, want true")
	}
	if field.Name != "SessionToken" {
		t.Fatalf("field name = %q, want %q", field.Name, "SessionToken")
	}
	if field.Source != sourceCustom {
		t.Fatalf("field source = %q, want %q", field.Source, sourceCustom)
	}
	if !field.Sensitive {
		t.Fatal("custom sensitive field is not sensitive")
	}
	enum, ok := dict.ExplainValue(9001, "A")
	if !ok || enum != "Alpha" {
		t.Fatalf("enum = %q, ok = %t, want Alpha true", enum, ok)
	}
}

func TestDictionaryCustomTagOverridesDisplayOnlyForStandardTag(t *testing.T) {
	dict := New([]CustomTag{
		{
			Tag:  35,
			Name: "Message Kind",
			Type: "INT",
			Enums: map[string]string{
				"D": "CustomNewOrder",
			},
		},
	})

	field, ok := dict.Lookup(35)
	if !ok {
		t.Fatal("Lookup(35) ok = false, want true")
	}
	if field.Name != "Message Kind" {
		t.Fatalf("field name = %q, want %q", field.Name, "Message Kind")
	}
	if field.Type != "STRING" {
		t.Fatalf("field type = %q, want standard type STRING", field.Type)
	}
	enum, ok := dict.ExplainValue(35, "D")
	if !ok || enum != "NewOrderSingle" {
		t.Fatalf("enum = %q, ok = %t, want standard enum NewOrderSingle true", enum, ok)
	}
}

func TestDictionaryBuildsFromConfig(t *testing.T) {
	dict := NewFromConfig([]config.CustomTagConfig{
		{
			Tag:  9002,
			Name: "AccessToken",
			Type: "STRING",
		},
	})

	if !dict.IsSensitive(9002) {
		t.Fatal("AccessToken custom tag should be sensitive by name")
	}
}
