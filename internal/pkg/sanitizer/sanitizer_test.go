package sanitizer

import "testing"

func TestSanitizeHTML(t *testing.T) {
	s := New()

	tests := []struct {
		name  string
		input string
		expected string
	}{
		{"no html", "hello world", "hello world"},
		{"with script tag", "<script>alert('xss')</script>hello", "hello"},
		{"with img tag", "<img src=x onerror='alert(1)'>hello", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.SanitizeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateLength(t *testing.T) {
	s := New()

	tests := []struct {
		name  string
		input string
		max   int
		valid bool
	}{
		{"within limit", "hello", 10, true},
		{"exactly at limit", "hello", 5, true},
		{"exceeds limit", "hello world", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.ValidateLength(tt.input, tt.max)
			if result != tt.valid {
				t.Errorf("got %v, want %v", result, tt.valid)
			}
		})
	}
}
