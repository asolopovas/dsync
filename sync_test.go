package main

import "testing"

func TestEnsureTrailingSlash(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"empty", "", "/"},
		{"no slash", "/path/to/dir", "/path/to/dir/"},
		{"with slash", "/path/to/dir/", "/path/to/dir/"},
		{"root", "/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ensureTrailingSlash(tt.path); got != tt.want {
				t.Errorf("ensureTrailingSlash() = %v, want %v", got, tt.want)
			}
		})
	}
}
