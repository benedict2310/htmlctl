package extensionspec

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		want    int
		wantErr bool
	}{
		{name: "equal", a: "1.2.3", b: "1.2.3", want: 0},
		{name: "leading v", a: "v1.2.3", b: "1.2.3", want: 0},
		{name: "newer patch", a: "1.2.4", b: "1.2.3", want: 1},
		{name: "older minor", a: "1.1.9", b: "1.2.0", want: -1},
		{name: "release beats prerelease", a: "1.2.3", b: "1.2.3-rc.1", want: 1},
		{name: "prerelease compare", a: "1.2.3-rc.2", b: "1.2.3-rc.10", want: -1},
		{name: "build metadata ignored", a: "1.2.3+build.5", b: "1.2.3+build.6", want: 0},
		{name: "invalid", a: "dev", b: "1.2.3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersions(tt.a, tt.b)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error comparing %q and %q", tt.a, tt.b)
				}
				return
			}
			if err != nil {
				t.Fatalf("CompareVersions(%q, %q) error = %v", tt.a, tt.b, err)
			}
			if got != tt.want {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVersionSatisfiesMinimum(t *testing.T) {
	if err := VersionSatisfiesMinimum("1.2.3", "1.2.3"); err != nil {
		t.Fatalf("expected exact version to satisfy minimum, got %v", err)
	}
	if err := VersionSatisfiesMinimum("1.2.2", "1.2.3"); err == nil {
		t.Fatalf("expected lower version to fail minimum check")
	}
}
