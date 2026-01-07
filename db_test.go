package main

import (
	"fmt"
	"strings"
	"testing"
)

func escapeLevel(url string, level int) string {
	switch level {
	case 1:
		return strings.ReplaceAll(url, "/", `\/`)
	case 2:
		return strings.ReplaceAll(url, "/", `\\/`)
	case 3:
		return strings.ReplaceAll(url, "/", `\\\/`)
	case 4:
		return strings.ReplaceAll(url, "/", `\\\\/`)
	case 5:
		return strings.ReplaceAll(url, "/", `\\\\\/`)
	default:
		return url
	}
}

type testCase struct {
	name         string
	sql          string
	replacements []DBReplace
	want         string
}

func TestApplyDBReplacements(t *testing.T) {
	tests := []testCase{
		{
			name:         "single replacement",
			sql:          "INSERT INTO users VALUES ('http://example.com');",
			replacements: []DBReplace{{From: "example.com", To: "localhost"}},
			want:         "INSERT INTO users VALUES ('http://localhost');",
		},
		{
			name:         "multiple replacements",
			sql:          "INSERT INTO users VALUES ('http://example.com', '/var/www/html');",
			replacements: []DBReplace{{From: "example.com", To: "localhost"}, {From: "/var/www/html", To: "/app"}},
			want:         "INSERT INTO users VALUES ('http://localhost', '/app');",
		},
		{
			name:         "no replacements",
			sql:          "INSERT INTO users VALUES ('http://example.com');",
			replacements: []DBReplace{},
			want:         "INSERT INTO users VALUES ('http://example.com');",
		},
		{
			name:         "mixed normal and escaped",
			sql:          "INSERT INTO options VALUES ('normal', 'http://example.com'), ('json', 'http:\\/\\/example.com');",
			replacements: []DBReplace{{From: "example.com", To: "some.domain.com"}},
			want:         "INSERT INTO options VALUES ('normal', 'http://some.domain.com'), ('json', 'http:\\/\\/some.domain.com');",
		},
	}

	for _, level := range []int{1, 2, 3, 4} {
		from := "http://example.com"
		to := "http://some.domain.com"
		tests = append(tests, testCase{
			name:         fmt.Sprintf("escaped slashes (level %d) - domain only", level),
			sql:          fmt.Sprintf("INSERT INTO options VALUES ('siteurl', '%s');", escapeLevel(from, level)),
			replacements: []DBReplace{{From: "example.com", To: "some.domain.com"}},
			want:         fmt.Sprintf("INSERT INTO options VALUES ('siteurl', '%s');", escapeLevel(to, level)),
		})
	}

	for _, level := range []int{0, 1, 2} {
		from := "https://some.domain.com"
		to := "http://local.test"
		levelName := "normal"
		if level > 0 {
			levelName = fmt.Sprintf("level %d", level)
		}
		tests = append(tests, testCase{
			name:         fmt.Sprintf("full URL with protocol - %s", levelName),
			sql:          fmt.Sprintf("INSERT INTO options VALUES ('siteurl', '%s');", escapeLevel(from, level)),
			replacements: []DBReplace{{From: from, To: to}},
			want:         fmt.Sprintf("INSERT INTO options VALUES ('siteurl', '%s');", escapeLevel(to, level)),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ApplyDBReplacements(tt.sql, tt.replacements); got != tt.want {
				t.Errorf("ApplyDBReplacements() = %v, want %v", got, tt.want)
			}
		})
	}
}
