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
		{
			name: "JSON escaped slashes (level 1)",
			sql:  "INSERT INTO options VALUES ('siteurl', 'http:\\/\\/example.com');",
			replacements: []DBReplace{
				{From: "example.com", To: "some.domain.com"},
			},
			want: "INSERT INTO options VALUES ('siteurl', 'http:\\/\\/some.domain.com');",
		},
		{
			name: "double escaped slashes (level 2)",
			sql:  "INSERT INTO options VALUES ('siteurl', 'http:\\\\/\\\\/example.com');",
			replacements: []DBReplace{
				{From: "example.com", To: "some.domain.com"},
			},
			want: "INSERT INTO options VALUES ('siteurl', 'http:\\\\/\\\\/some.domain.com');",
		},
		{
			name: "triple escaped slashes (level 3)",
			sql:  "INSERT INTO options VALUES ('siteurl', 'http:\\\\\\/\\\\\\/example.com');",
			replacements: []DBReplace{
				{From: "example.com", To: "some.domain.com"},
			},
			want: "INSERT INTO options VALUES ('siteurl', 'http:\\\\\\/\\\\\\/some.domain.com');",
		},
		{
			name: "quadruple escaped slashes (level 4)",
			sql:  "INSERT INTO options VALUES ('siteurl', 'http:\\\\\\\\/\\\\\\\\/example.com');",
			replacements: []DBReplace{
				{From: "example.com", To: "some.domain.com"},
			},
			want: "INSERT INTO options VALUES ('siteurl', 'http:\\\\\\\\/\\\\\\\\/some.domain.com');",
		},
		{
			name: "mixed normal and escaped",
			sql:  "INSERT INTO options VALUES ('normal', 'http://example.com'), ('json', 'http:\\/\\/example.com');",
			replacements: []DBReplace{
				{From: "example.com", To: "some.domain.com"},
			},
			want: "INSERT INTO options VALUES ('normal', 'http://some.domain.com'), ('json', 'http:\\/\\/some.domain.com');",
		},
		{
			name: "full URL with protocol replacement - normal",
			sql:  "INSERT INTO options VALUES ('siteurl', 'https://some.domain.com');",
			replacements: []DBReplace{
				{From: "https://some.domain.com", To: "http://local.test"},
			},
			want: "INSERT INTO options VALUES ('siteurl', 'http://local.test');",
		},
		{
			name: "full URL with protocol replacement - JSON escaped",
			sql:  "INSERT INTO options VALUES ('siteurl', 'https:\\/\\/some.domain.com');",
			replacements: []DBReplace{
				{From: "https://some.domain.com", To: "http://local.test"},
			},
			want: "INSERT INTO options VALUES ('siteurl', 'http:\\/\\/local.test');",
		},
		{
			name: "full URL with protocol replacement - double escaped",
			sql:  "INSERT INTO options VALUES ('siteurl', 'https:\\\\/\\\\/some.domain.com');",
			replacements: []DBReplace{
				{From: "https://some.domain.com", To: "http://local.test"},
			},
			want: "INSERT INTO options VALUES ('siteurl', 'http:\\\\/\\\\/local.test');",
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
