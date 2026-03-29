package domain

import "testing"

func TestNotFoundError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *NotFoundError
		expected string
	}{
		{
			name:     "resource and ID",
			err:      &NotFoundError{Resource: "collection", ID: "abc123"},
			expected: "collection not found: abc123",
		},
		{
			name:     "empty ID",
			err:      &NotFoundError{Resource: "item", ID: ""},
			expected: "item not found: ",
		},
		{
			name:     "both empty",
			err:      &NotFoundError{},
			expected: " not found: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		expected string
	}{
		{
			name:     "with field and message",
			err:      &ValidationError{Field: "host", Message: "required"},
			expected: "invalid host: required",
		},
		{
			name:     "message only (no field)",
			err:      &ValidationError{Field: "", Message: "something went wrong"},
			expected: "something went wrong",
		},
		{
			name:     "both empty",
			err:      &ValidationError{},
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
