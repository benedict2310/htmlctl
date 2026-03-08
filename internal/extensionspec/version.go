package extensionspec

import (
	"fmt"
	"strconv"
	"strings"
)

type semanticVersion struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease []string
}

func CompareVersions(actual, required string) (int, error) {
	actualVersion, err := parseSemanticVersion(actual)
	if err != nil {
		return 0, fmt.Errorf("parse actual version: %w", err)
	}
	requiredVersion, err := parseSemanticVersion(required)
	if err != nil {
		return 0, fmt.Errorf("parse required version: %w", err)
	}
	return compareSemanticVersions(actualVersion, requiredVersion), nil
}

func VersionSatisfiesMinimum(actual, minimum string) error {
	compare, err := CompareVersions(actual, minimum)
	if err != nil {
		return err
	}
	if compare < 0 {
		return fmt.Errorf("version %q is lower than required minimum %q", actual, minimum)
	}
	return nil
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	normalized := strings.TrimSpace(value)
	if strings.HasPrefix(normalized, "v") || strings.HasPrefix(normalized, "V") {
		normalized = normalized[1:]
	}
	if !semverPattern.MatchString(normalized) {
		return semanticVersion{}, fmt.Errorf("%q is not valid semver", value)
	}

	buildSplit := strings.SplitN(normalized, "+", 2)
	core := buildSplit[0]

	var prerelease []string
	preSplit := strings.SplitN(core, "-", 2)
	if len(preSplit) == 2 {
		prerelease = strings.Split(preSplit[1], ".")
	}

	coreParts := strings.Split(preSplit[0], ".")
	major, _ := strconv.Atoi(coreParts[0])
	minor, _ := strconv.Atoi(coreParts[1])
	patch, _ := strconv.Atoi(coreParts[2])

	return semanticVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: prerelease,
	}, nil
}

func compareSemanticVersions(a, b semanticVersion) int {
	switch {
	case a.Major != b.Major:
		return compareInt(a.Major, b.Major)
	case a.Minor != b.Minor:
		return compareInt(a.Minor, b.Minor)
	case a.Patch != b.Patch:
		return compareInt(a.Patch, b.Patch)
	default:
		return comparePreRelease(a.PreRelease, b.PreRelease)
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func comparePreRelease(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}

	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		aPart := a[i]
		bPart := b[i]
		aNum, aErr := strconv.Atoi(aPart)
		bNum, bErr := strconv.Atoi(bPart)

		switch {
		case aErr == nil && bErr == nil:
			if aNum != bNum {
				return compareInt(aNum, bNum)
			}
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		case aPart != bPart:
			if aPart < bPart {
				return -1
			}
			return 1
		}
	}

	return compareInt(len(a), len(b))
}
