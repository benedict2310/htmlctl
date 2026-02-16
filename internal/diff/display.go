package diff

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/benedict2310/htmlctl/internal/output"
)

type DisplayOptions struct {
	Color bool
}

func AutoColor(w io.Writer) bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func WriteTable(w io.Writer, result Result, opts DisplayOptions) error {
	grouped := map[string][]FileChange{}
	for _, change := range result.Changes {
		group := change.ResourceType
		if strings.TrimSpace(group) == "" {
			group = "assets"
		}
		grouped[group] = append(grouped[group], change)
	}

	groupOrder := []string{"pages", "components", "styles", "scripts", "assets"}
	printedAny := false
	for _, group := range groupOrder {
		changes := grouped[group]
		if len(changes) == 0 {
			continue
		}
		if printedAny {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s:\n", strings.ToUpper(group))
		rows := make([][]string, 0, len(changes))
		for _, change := range changes {
			rows = append(rows, []string{
				colorize(string(change.ChangeType), change.ChangeType, opts.Color),
				change.Path,
				shortHash(change.OldHash),
				shortHash(change.NewHash),
			})
		}
		if err := output.WriteTable(w, []string{"CHANGE", "PATH", "OLD_HASH", "NEW_HASH"}, rows); err != nil {
			return err
		}
		printedAny = true
	}

	if !printedAny {
		fmt.Fprintln(w, "No changes detected.")
	}
	fmt.Fprintf(
		w,
		"%d added, %d modified, %d removed, %d unchanged\n",
		result.Summary.Added,
		result.Summary.Modified,
		result.Summary.Removed,
		result.Summary.Unchanged,
	)
	return nil
}

func shortHash(hash string) string {
	v := strings.TrimSpace(hash)
	if v == "" {
		return "-"
	}
	v = strings.TrimPrefix(strings.ToLower(v), "sha256:")
	if len(v) > 8 {
		return v[:8]
	}
	return v
}

func colorize(v string, changeType ChangeType, enabled bool) string {
	if !enabled {
		return v
	}
	var color string
	switch changeType {
	case ChangeAdded:
		color = "32"
	case ChangeModified:
		color = "33"
	case ChangeRemoved:
		color = "31"
	default:
		return v
	}
	return "\x1b[" + color + "m" + v + "\x1b[0m"
}
