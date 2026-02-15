package release

import (
	"sort"
	"testing"
	"time"
)

func TestNewReleaseIDUniqueAndSortedByTime(t *testing.T) {
	first, err := NewReleaseID(time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("NewReleaseID(first) error = %v", err)
	}
	second, err := NewReleaseID(time.Unix(1700000001, 0))
	if err != nil {
		t.Fatalf("NewReleaseID(second) error = %v", err)
	}
	if first == second {
		t.Fatalf("expected unique IDs, got identical %q", first)
	}
	ids := []string{second, first}
	sort.Strings(ids)
	if ids[0] != first || ids[1] != second {
		t.Fatalf("expected lexicographic time order, got %#v", ids)
	}
}
