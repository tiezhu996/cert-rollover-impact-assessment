package util

import "testing"

func TestRedactNestedObjectNoPanic(t *testing.T) {
	value := map[string]any{"name": "gateway", "settings": map[string]any{"timeout": 30}}
	out := RedactValue("", value)
	_ = out
}

func TestRedactArrayNoPanic(t *testing.T) {
	value := []any{"a", "b", map[string]any{"k": "v"}}
	out := RedactValue("", value)
	_ = out
}

func TestRedactedJSONHandlesEmpty(t *testing.T) {
	out := RedactedJSON(nil)
	if out != "null" && out != "{}" {
		t.Fatalf("unexpected redaction output %q", out)
	}
}
