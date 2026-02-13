package links

import (
	"strings"
	"testing"
)

func TestCryptoSluggerGenerate(t *testing.T) {
	s := NewCryptoSlugger()

	t.Run("correct length", func(t *testing.T) {
		slug, err := s.Generate(8)
		if err != nil {
			t.Fatal(err)
		}
		if len(slug) != 8 {
			t.Errorf("got length %d, want 8", len(slug))
		}
	})

	t.Run("base62 alphabet only", func(t *testing.T) {
		slug, err := s.Generate(100)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range slug {
			if !strings.ContainsRune(base62Alphabet, c) {
				t.Errorf("slug contains non-base62 char: %q", c)
			}
		}
	})

	t.Run("zero length uses fallback", func(t *testing.T) {
		slug, err := s.Generate(0)
		if err != nil {
			t.Fatal(err)
		}
		if len(slug) != 6 {
			t.Errorf("got length %d, want 6 (fallback)", len(slug))
		}
	})

	t.Run("negative length uses fallback", func(t *testing.T) {
		slug, err := s.Generate(-5)
		if err != nil {
			t.Fatal(err)
		}
		if len(slug) != 6 {
			t.Errorf("got length %d, want 6 (fallback)", len(slug))
		}
	})

	t.Run("uniqueness over 100 calls", func(t *testing.T) {
		seen := make(map[string]struct{}, 100)
		for i := 0; i < 100; i++ {
			slug, err := s.Generate(10)
			if err != nil {
				t.Fatal(err)
			}
			if _, exists := seen[slug]; exists {
				t.Fatalf("duplicate slug on iteration %d: %q", i, slug)
			}
			seen[slug] = struct{}{}
		}
	})
}
