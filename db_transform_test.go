package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestTransformSQLDump(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		options ReplacementOptions
	}{
		{
			name:    "skips guid column",
			input:   "INSERT INTO `wp_posts` (`ID`,`guid`,`post_content`) VALUES (1,'https://example.com/?p=1','Visit https://example.com'),(2,'https://example.com/?p=2','Visit https://example.com/page');",
			want:    "INSERT INTO `wp_posts` (`ID`,`guid`,`post_content`) VALUES (1,'https://example.com/?p=1','Visit http://local.test'),(2,'https://example.com/?p=2','Visit http://local.test/page');",
			options: goSerializedOptionsForTest(false, "guid"),
		},
		{
			name:    "repairs serialized PHP lengths",
			input:   "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('home','s:19:\"https://example.com\";'),('nested','a:1:{s:3:\"url\";s:19:\"https://example.com\";}');",
			want:    "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('home','s:17:\"http://local.test\";'),('nested','a:1:{s:3:\"url\";s:17:\"http://local.test\";}');",
			options: goSerializedOptionsForTest(true, "guid"),
		},
		{
			name:    "preserves SQL string escaping",
			input:   "INSERT INTO t (`value`) VALUES ('quote\\' slash\\\\ newline\\n tab\\t nul\\0');",
			want:    "INSERT INTO t (`value`) VALUES ('quote\\' slash\\\\ newline\\n tab\\t nul\\0');",
			options: ReplacementOptions{Engine: DBReplaceEngineGoSerialized},
		},
		{
			name:    "normalizes multiline inserts",
			input:   "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES\n('siteurl','https://example.com'),\n('home','https://example.com');\nCREATE TABLE t (id int);\n",
			want:    "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('siteurl','http://local.test'),('home','http://local.test');\nCREATE TABLE t (id int);\n",
			options: goSerializedOptionsForTest(false),
		},
		{
			name:    "keeps invalid serialized values when validation is disabled",
			input:   "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:5:\"abc\";'),('plain','https://example.com');",
			want:    "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:5:\"abc\";'),('plain','http://local.test');",
			options: goSerializedOptionsForTest(false),
		},
		{
			name:    "keeps invalid serialized values when validation is enabled",
			input:   "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:5:\"abc\";'),('plain','https://example.com');",
			want:    "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:5:\"abc\";'),('plain','http://local.test');",
			options: goSerializedOptionsForTest(true),
		},
		{
			name:    "does not rewrite invalid serialized matches",
			input:   "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:25:\"https://example.com\";');",
			want:    "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:25:\"https://example.com\";');",
			options: goSerializedOptionsForTest(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformStringForTest(tt.input, tt.options)
			if err != nil {
				t.Fatalf("TransformSQLDump failed: %v", err)
			}
			assertStringEqual(t, "unexpected SQL", got, tt.want)
		})
	}
}

func TestTransformSQLDumpNestedSerializedString(t *testing.T) {
	nested := `a:1:{s:3:"url";s:19:"https://example.com";}`
	serialized := fmt.Sprintf(`a:1:{s:6:"nested";s:%d:"%s";}`, len([]byte(nested)), nested)
	input := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('plugin','" + strings.ReplaceAll(serialized, "'", `\\'`) + "');"

	got, err := transformStringForTest(input, goSerializedOptionsForTest(false))
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}

	transformedNested := `a:1:{s:3:"url";s:17:"http://local.test";}`
	if !strings.Contains(got, fmt.Sprintf(`s:%d:"%s";`, len([]byte(transformedNested)), transformedNested)) {
		t.Fatalf("nested serialized string length was not repaired: %s", got)
	}
}

func TestWordPressLikeFixtureTransform(t *testing.T) {
	data, err := os.ReadFile("testdata/wordpress-like.sql")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got, err := transformStringForTest(string(data), goSerializedOptionsForTest(true, "guid"))
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	for _, guid := range []string{"'https://example.com/?p=1'", "'https://example.com/?p=2'"} {
		if !strings.Contains(got, guid) {
			t.Fatalf("guid value changed or disappeared: %s", got)
		}
	}
	for _, want := range []string{
		"Visit http://local.test",
		`"url":"http://local.test/page"`,
		`s:17:"http://local.test";`,
		`a:1:{s:3:"url";s:17:"http://local.test";}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("transformed fixture missing %q in:\n%s", want, got)
		}
	}
}

func TestPHPSerializationSupportsCommonTypes(t *testing.T) {
	input := `a:9:{s:3:"str";s:6:"Björk";s:4:"bool";b:1;s:4:"null";N;s:3:"int";i:42;s:5:"float";d:1.5;s:3:"obj";O:8:"stdClass":1:{s:3:"url";s:19:"https://example.com";}s:3:"arr";a:1:{s:19:"https://example.com";s:1:"x";}s:7:"softref";r:2;s:7:"hardref";R:2;}`
	got, err := transformSerializedPHP(input, urlReplacementForTest())
	if err != nil {
		t.Fatalf("transformSerializedPHP failed: %v", err)
	}
	for _, want := range []string{`s:17:"http://local.test"`, `s:6:"Björk"`, `r:2;`, `R:2;`} {
		if !strings.Contains(got, want) {
			t.Fatalf("serialized output missing %q: %s", want, got)
		}
	}
	if _, err := parsePHPSerialized(got); err != nil {
		t.Fatalf("transformed value is invalid: %v", err)
	}
}
