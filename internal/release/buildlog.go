package release

import (
	"fmt"
	"strings"
	"time"
)

type buildLog struct {
	lines []string
}

func newBuildLog() *buildLog {
	return &buildLog{lines: []string{}}
}

func (l *buildLog) Addf(format string, args ...any) {
	prefix := time.Now().UTC().Format(time.RFC3339Nano)
	l.lines = append(l.lines, fmt.Sprintf("%s %s", prefix, fmt.Sprintf(format, args...)))
}

func (l *buildLog) String() string {
	if len(l.lines) == 0 {
		return ""
	}
	return strings.Join(l.lines, "\n") + "\n"
}
