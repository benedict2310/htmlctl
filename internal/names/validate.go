package names

import (
	"fmt"
	"regexp"
)

const MaxResourceNameLength = 128

var resourceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func ValidateResourceName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > MaxResourceNameLength {
		return fmt.Errorf("name must be at most %d characters", MaxResourceNameLength)
	}
	if !resourceNamePattern.MatchString(name) {
		return fmt.Errorf("name must match %q", resourceNamePattern.String())
	}
	return nil
}
