package sourceartifact

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSemanticAndCanonicalTextIgnoreLineEndingRepresentation(t *testing.T) {
	lf := []byte("first\nsecond\n")
	crlf := []byte("first\r\nsecond\r\n")
	cr := []byte("first\rsecond\r")

	wantSemantic := lf
	wantCanonical := crlf
	for name, input := range map[string][]byte{
		"lf":   lf,
		"crlf": crlf,
		"cr":   cr,
	} {
		t.Run(name, func(t *testing.T) {
			if got := SemanticText(input); !bytes.Equal(got, wantSemantic) {
				t.Fatalf("SemanticText() = %q, want %q", got, wantSemantic)
			}
			if got := CanonicalText(input); !bytes.Equal(got, wantCanonical) {
				t.Fatalf("CanonicalText() = %q, want %q", got, wantCanonical)
			}
		})
	}
}

func TestReadCanonicalReturnsCanonicalBytesWithoutChangingSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.md")
	source := []byte("first\nsecond\n")
	if err := os.WriteFile(path, source, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadCanonical(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte("first\r\nsecond\r\n"); !bytes.Equal(got, want) {
		t.Fatalf("ReadCanonical() = %q, want %q", got, want)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unchanged, source) {
		t.Fatalf("ReadCanonical changed source bytes: %q", unchanged)
	}
}
