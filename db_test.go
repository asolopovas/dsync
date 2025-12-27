package main

import (
	"testing"
)

func TestApplyDBReplacements(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		replacements []DBReplace
		want         string
	}{
		{
			name: "single replacement",
			sql:  "INSERT INTO users VALUES ('http://example.com');",
			replacements: []DBReplace{
				{From: "example.com", To: "localhost"},
			},
			want: "INSERT INTO users VALUES ('http://localhost');",
		},
		{
			name: "multiple replacements",
			sql:  "INSERT INTO users VALUES ('http://example.com', '/var/www/html');",
			replacements: []DBReplace{
				{From: "example.com", To: "localhost"},
				{From: "/var/www/html", To: "/app"},
			},
			want: "INSERT INTO users VALUES ('http://localhost', '/app');",
		},
		{
			name:         "no replacements",
			sql:          "INSERT INTO users VALUES ('http://example.com');",
			replacements: []DBReplace{},
			want:         "INSERT INTO users VALUES ('http://example.com');",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ApplyDBReplacements(tt.sql, tt.replacements); got != tt.want {
				t.Errorf("ApplyDBReplacements() = %v, want %v", got, tt.want)
			}
		})
	}
}
