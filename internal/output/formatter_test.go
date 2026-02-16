package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	if got, err := ParseFormat(""); err != nil || got != FormatTable {
		t.Fatalf("ParseFormat(\"\") got=%q err=%v", got, err)
	}
	if got, err := ParseFormat("json"); err != nil || got != FormatJSON {
		t.Fatalf("ParseFormat(json) got=%q err=%v", got, err)
	}
	if got, err := ParseFormat("yaml"); err != nil || got != FormatYAML {
		t.Fatalf("ParseFormat(yaml) got=%q err=%v", got, err)
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Fatalf("expected invalid format error")
	}
}

func TestWriteStructuredJSONAndYAML(t *testing.T) {
	payload := map[string]any{"website": "futurelab", "count": 2}

	jsonOut := &bytes.Buffer{}
	if err := WriteStructured(jsonOut, FormatJSON, payload); err != nil {
		t.Fatalf("WriteStructured(JSON) error = %v", err)
	}
	if !strings.Contains(jsonOut.String(), "\"website\": \"futurelab\"") {
		t.Fatalf("unexpected json output: %s", jsonOut.String())
	}

	yamlOut := &bytes.Buffer{}
	if err := WriteStructured(yamlOut, FormatYAML, payload); err != nil {
		t.Fatalf("WriteStructured(YAML) error = %v", err)
	}
	if !strings.Contains(yamlOut.String(), "website: futurelab") {
		t.Fatalf("unexpected yaml output: %s", yamlOut.String())
	}
}

func TestWriteTable(t *testing.T) {
	out := &bytes.Buffer{}
	err := WriteTable(out, []string{"NAME", "STATUS"}, [][]string{{"futurelab", "active"}})
	if err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}
	if !strings.Contains(out.String(), "NAME") || !strings.Contains(out.String(), "futurelab") {
		t.Fatalf("unexpected table output: %s", out.String())
	}
}
