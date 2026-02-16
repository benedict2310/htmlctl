package diff

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteTableFormatsGroupedOutputAndSummary(t *testing.T) {
	var buf bytes.Buffer
	result := Result{
		Changes: []FileChange{
			{
				Path:         "components/header.html",
				ResourceType: "components",
				ChangeType:   ChangeModified,
				OldHash:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				NewHash:      "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			{
				Path:         "assets/logo.svg",
				ResourceType: "assets",
				ChangeType:   ChangeAdded,
				NewHash:      "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			},
		},
		Summary: Summary{
			Added:     1,
			Modified:  1,
			Removed:   0,
			Unchanged: 3,
		},
	}

	if err := WriteTable(&buf, result, DisplayOptions{}); err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}
	out := buf.String()
	for _, want := range []string{"COMPONENTS:", "ASSETS:", "aaaaaaaa", "bbbbbbbb", "cccccccc", "1 added, 1 modified, 0 removed, 3 unchanged"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got: %s", want, out)
		}
	}
}

func TestWriteTableColorizesChangeLabels(t *testing.T) {
	var buf bytes.Buffer
	result := Result{
		Changes: []FileChange{
			{
				Path:         "assets/logo.svg",
				ResourceType: "assets",
				ChangeType:   ChangeRemoved,
				OldHash:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
		Summary: Summary{Removed: 1},
	}

	if err := WriteTable(&buf, result, DisplayOptions{Color: true}); err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b[31mremoved\x1b[0m") {
		t.Fatalf("expected ANSI colorized change label, got: %s", buf.String())
	}
}

func TestWriteTableNoChangesMessage(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, Result{}, DisplayOptions{}); err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}
	if !strings.Contains(buf.String(), "No changes detected.") {
		t.Fatalf("expected no changes message, got: %s", buf.String())
	}
}
