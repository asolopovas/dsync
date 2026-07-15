package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func escapedURL(url string, level int) string {
	slash := map[int]string{
		1: `\/`,
		2: `\\/`,
		3: `\\\/`,
		4: `\\\\/`,
	}[level]
	if slash == "" {
		return url
	}
	return strings.ReplaceAll(url, "/", slash)
}

func TestApplyDBReplacements(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		replacements []DBReplace
		want         string
	}{
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
			replacements: nil,
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
		tests = append(tests, struct {
			name         string
			sql          string
			replacements []DBReplace
			want         string
		}{
			name:         fmt.Sprintf("escaped slashes level %d", level),
			sql:          fmt.Sprintf("INSERT INTO options VALUES ('siteurl', '%s');", escapedURL(from, level)),
			replacements: []DBReplace{{From: "example.com", To: "some.domain.com"}},
			want:         fmt.Sprintf("INSERT INTO options VALUES ('siteurl', '%s');", escapedURL(to, level)),
		})
	}

	for _, level := range []int{0, 1, 2} {
		from := "https://some.domain.com"
		to := "http://local.test"
		tests = append(tests, struct {
			name         string
			sql          string
			replacements []DBReplace
			want         string
		}{
			name:         fmt.Sprintf("full URL level %d", level),
			sql:          fmt.Sprintf("INSERT INTO options VALUES ('siteurl', '%s');", escapedURL(from, level)),
			replacements: []DBReplace{{From: from, To: to}},
			want:         fmt.Sprintf("INSERT INTO options VALUES ('siteurl', '%s');", escapedURL(to, level)),
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

func TestApplyDBReplacementsHandlesEscapedConfigValues(t *testing.T) {
	var escapedConfig []DBReplace
	configJSON := `[{"from":"https:\\/\\/example.com","to":"http:\\/\\/example.test"}]`
	if err := json.Unmarshal([]byte(configJSON), &escapedConfig); err != nil {
		t.Fatalf("unmarshal escaped config: %v", err)
	}

	dump := `Some content "https:\/\/example.com" end`
	want := `Some content "http:\/\/example.test" end`
	tests := []struct {
		name         string
		replacements []DBReplace
	}{
		{"escaped config", escapedConfig},
		{"plain config", []DBReplace{{From: "https://example.com", To: "http://example.test"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ApplyDBReplacements(dump, tt.replacements); got != want {
				t.Fatalf("ApplyDBReplacements() = %s, want %s", got, want)
			}
		})
	}
}

func TestMySQLDumpFlagsUseExtendedInserts(t *testing.T) {
	flags := make(map[string]bool)
	for _, flag := range mysqlDumpFlags() {
		flags[flag] = true
	}

	for _, required := range []string{"--complete-insert", "--extended-insert"} {
		if !flags[required] {
			t.Errorf("mysqlDumpFlags() missing %q", required)
		}
	}
	if flags["--skip-extended-insert"] {
		t.Error("mysqlDumpFlags() disables extended inserts")
	}
}

func TestStartSpinnerHonorsDisabledOutput(t *testing.T) {
	spinner := startSpinner("quiet test")
	if spinner.IsActive {
		t.Fatal("startSpinner() started live output while pterm output is disabled")
	}
	if spinner.Text != "quiet test" {
		t.Fatalf("startSpinner() text = %q, want %q", spinner.Text, "quiet test")
	}
}
