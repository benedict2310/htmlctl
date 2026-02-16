package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(headers) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
			return err
		}
	}
	for i, row := range rows {
		if len(headers) > 0 && len(row) != len(headers) {
			return fmt.Errorf("table row %d has %d columns, expected %d", i, len(row), len(headers))
		}
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func OrNone(v *string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return "<none>"
	}
	return strings.TrimSpace(*v)
}

func Truncate(v string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(v) <= max {
		return v
	}
	if max <= 3 {
		return v[:max]
	}
	return v[:max-3] + "..."
}
