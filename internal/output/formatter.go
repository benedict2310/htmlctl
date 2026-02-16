package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

func ParseFormat(v string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", string(FormatTable):
		return FormatTable, nil
	case string(FormatJSON):
		return FormatJSON, nil
	case string(FormatYAML):
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("invalid output format %q (expected table, json, or yaml)", v)
	}
}

func WriteStructured(w io.Writer, format Format, payload any) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return fmt.Errorf("encode json output: %w", err)
		}
		return nil
	case FormatYAML:
		data, err := yaml.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode yaml output: %w", err)
		}
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		_, err = w.Write(data)
		return err
	default:
		return fmt.Errorf("structured output is only supported for json/yaml")
	}
}
