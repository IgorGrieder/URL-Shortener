package config

import (
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	t.Run("returns value when set", func(t *testing.T) {
		t.Setenv("TEST_KEY", "hello")
		if got := GetEnv("TEST_KEY", "fallback"); got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})

	t.Run("returns fallback when unset", func(t *testing.T) {
		if got := GetEnv("UNSET_KEY_12345", "fb"); got != "fb" {
			t.Errorf("got %q, want %q", got, "fb")
		}
	})

	t.Run("returns fallback when empty", func(t *testing.T) {
		t.Setenv("TEST_KEY", "")
		if got := GetEnv("TEST_KEY", "fb"); got != "fb" {
			t.Errorf("got %q, want %q", got, "fb")
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Setenv("TEST_KEY", "  value  ")
		if got := GetEnv("TEST_KEY", "fb"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})

	t.Run("whitespace-only returns fallback", func(t *testing.T) {
		t.Setenv("TEST_KEY", "   ")
		if got := GetEnv("TEST_KEY", "fb"); got != "fb" {
			t.Errorf("got %q, want %q", got, "fb")
		}
	})
}

func TestGetEnvInt(t *testing.T) {
	t.Run("parses valid int", func(t *testing.T) {
		t.Setenv("TEST_INT", "42")
		if got := GetEnvInt("TEST_INT", 0); got != 42 {
			t.Errorf("got %d, want 42", got)
		}
	})

	t.Run("returns fallback on missing", func(t *testing.T) {
		if got := GetEnvInt("UNSET_INT_12345", 7); got != 7 {
			t.Errorf("got %d, want 7", got)
		}
	})

	t.Run("returns fallback on invalid", func(t *testing.T) {
		t.Setenv("TEST_INT", "not_a_number")
		if got := GetEnvInt("TEST_INT", 7); got != 7 {
			t.Errorf("got %d, want 7", got)
		}
	})

	t.Run("returns fallback on empty", func(t *testing.T) {
		t.Setenv("TEST_INT", "")
		if got := GetEnvInt("TEST_INT", 99); got != 99 {
			t.Errorf("got %d, want 99", got)
		}
	})
}

func TestGetEnvDuration(t *testing.T) {
	t.Run("parses valid duration", func(t *testing.T) {
		t.Setenv("TEST_DUR", "5s")
		if got := GetEnvDuration("TEST_DUR", time.Second); got != 5*time.Second {
			t.Errorf("got %v, want 5s", got)
		}
	})

	t.Run("returns fallback on missing", func(t *testing.T) {
		fb := 3 * time.Second
		if got := GetEnvDuration("UNSET_DUR_12345", fb); got != fb {
			t.Errorf("got %v, want %v", got, fb)
		}
	})

	t.Run("returns fallback on invalid", func(t *testing.T) {
		t.Setenv("TEST_DUR", "badvalue")
		fb := 2 * time.Second
		if got := GetEnvDuration("TEST_DUR", fb); got != fb {
			t.Errorf("got %v, want %v", got, fb)
		}
	})
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"normal", "a,b,c", 3},
		{"with spaces", " a , b , c ", 3},
		{"empty entries", "a,,b,,,c", 3},
		{"empty string", "", 0},
		{"single value", "only", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitCSV(tt.raw)
			if len(got) != tt.want {
				t.Errorf("SplitCSV(%q) returned %d items, want %d", tt.raw, len(got), tt.want)
			}
			// Verify no empty or whitespace-only entries
			for _, v := range got {
				if v == "" {
					t.Errorf("SplitCSV(%q) contains empty entry", tt.raw)
				}
			}
		})
	}
}
