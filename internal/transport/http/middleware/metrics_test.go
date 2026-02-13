package middleware

import "testing"

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			"UUID replacement",
			"/links/550e8400-e29b-41d4-a716-446655440000/stats",
			"/links/:id/stats",
		},
		{
			"ObjectID replacement",
			"/links/507f1f77bcf86cd799439011/stats",
			"/links/:id/stats",
		},
		{
			"numeric ID replacement",
			"/links/12345",
			"/links/:id",
		},
		{
			"no change for slug path",
			"/links/abcXYZ",
			"/links/abcXYZ",
		},
		{
			"multiple UUIDs",
			"/users/550e8400-e29b-41d4-a716-446655440000/links/660e8400-e29b-41d4-a716-446655440001",
			"/users/:id/links/:id",
		},
		{
			"root path unchanged",
			"/",
			"/",
		},
		{
			"health endpoint unchanged",
			"/health",
			"/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.path)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
