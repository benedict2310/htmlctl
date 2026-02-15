package renderer

import "testing"

func TestNormalizeLFString(t *testing.T) {
	in := "a\r\nb\r\nc\n"
	out := normalizeLFString(in)
	if out != "a\nb\nc\n" {
		t.Fatalf("normalizeLFString() = %q", out)
	}
}

func TestHashedFilenameUsesSHA256Prefix(t *testing.T) {
	got := hashedFilename("logo.svg", []byte("hello"))
	want := "logo-2cf24dba5fb0.svg"
	if got != want {
		t.Fatalf("hashedFilename() = %q, want %q", got, want)
	}
}
