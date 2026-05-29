package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func transformStringForTest(input string, options ReplacementOptions) (string, error) {
	var output bytes.Buffer
	err := TransformSQLDump(strings.NewReader(input), &output, options)
	return output.String(), err
}

func TestTransformSQLDumpGoSerializedSkipsGUID(t *testing.T) {
	input := "INSERT INTO `wp_posts` (`ID`,`guid`,`post_content`) VALUES (1,'https://example.com/?p=1','Visit https://example.com'),(2,'https://example.com/?p=2','Visit https://example.com/page');"
	want := "INSERT INTO `wp_posts` (`ID`,`guid`,`post_content`) VALUES (1,'https://example.com/?p=1','Visit http://local.test'),(2,'https://example.com/?p=2','Visit http://local.test/page');"

	got, err := transformStringForTest(input, ReplacementOptions{
		Engine:       DBReplaceEngineGoSerialized,
		Replacements: []DBReplace{{From: "https://example.com", To: "http://local.test"}},
		SkipColumns:  []string{"guid"},
	})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected SQL\ngot:  %s\nwant: %s", got, want)
	}
}

func TestTransformSQLDumpSerializedPHP(t *testing.T) {
	input := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('home','s:19:\"https://example.com\";'),('nested','a:1:{s:3:\"url\";s:19:\"https://example.com\";}');"
	want := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('home','s:17:\"http://local.test\";'),('nested','a:1:{s:3:\"url\";s:17:\"http://local.test\";}');"

	got, err := transformStringForTest(input, ReplacementOptions{
		Engine:             DBReplaceEngineGoSerialized,
		ValidateSerialized: true,
		Replacements:       []DBReplace{{From: "https://example.com", To: "http://local.test"}},
		SkipColumns:        []string{"guid"},
	})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected SQL\ngot:  %s\nwant: %s", got, want)
	}
}

func TestTransformSQLDumpNestedSerializedString(t *testing.T) {
	nested := `a:1:{s:3:"url";s:19:"https://example.com";}`
	serialized := fmt.Sprintf(`a:1:{s:6:"nested";s:%d:"%s";}`, len([]byte(nested)), nested)
	input := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('plugin','" + strings.ReplaceAll(serialized, "'", `\\'`) + "');"

	got, err := transformStringForTest(input, ReplacementOptions{
		Engine:       DBReplaceEngineGoSerialized,
		Replacements: []DBReplace{{From: "https://example.com", To: "http://local.test"}},
	})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	transformedNested := `a:1:{s:3:"url";s:17:"http://local.test";}`
	if !strings.Contains(got, fmt.Sprintf(`s:%d:"%s";`, len([]byte(transformedNested)), transformedNested)) {
		t.Fatalf("nested serialized string length was not repaired: %s", got)
	}
}

func TestSQLStringEscapingRoundTrip(t *testing.T) {
	input := "INSERT INTO t (`value`) VALUES ('quote\\' slash\\\\ newline\\n tab\\t nul\\0');"
	got, err := transformStringForTest(input, ReplacementOptions{Engine: DBReplaceEngineGoSerialized})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	want := "INSERT INTO t (`value`) VALUES ('quote\\' slash\\\\ newline\\n tab\\t nul\\0');"
	if got != want {
		t.Fatalf("unexpected SQL\ngot:  %s\nwant: %s", got, want)
	}
}

func TestMultilineInsertStatement(t *testing.T) {
	input := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES\n('siteurl','https://example.com'),\n('home','https://example.com');\nCREATE TABLE t (id int);\n"
	want := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('siteurl','http://local.test'),('home','http://local.test');\nCREATE TABLE t (id int);\n"

	got, err := transformStringForTest(input, ReplacementOptions{
		Engine:       DBReplaceEngineGoSerialized,
		Replacements: []DBReplace{{From: "https://example.com", To: "http://local.test"}},
	})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected SQL\ngot:  %s\nwant: %s", got, want)
	}
}

func TestWordPressLikeFixtureTransform(t *testing.T) {
	data, err := os.ReadFile("testdata/wordpress-like.sql")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got, err := transformStringForTest(string(data), ReplacementOptions{
		Engine:             DBReplaceEngineGoSerialized,
		ValidateSerialized: true,
		Replacements:       []DBReplace{{From: "https://example.com", To: "http://local.test"}},
		SkipColumns:        []string{"guid"},
	})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	if !strings.Contains(got, "'https://example.com/?p=1'") || !strings.Contains(got, "'https://example.com/?p=2'") {
		t.Fatalf("guid values changed or disappeared: %s", got)
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

func TestTransformSQLDumpSkipsInvalidSerializedWhenValidationDisabled(t *testing.T) {
	input := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:5:\"abc\";'),('plain','https://example.com');"
	want := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:5:\"abc\";'),('plain','http://local.test');"

	got, err := transformStringForTest(input, ReplacementOptions{
		Engine:       DBReplaceEngineGoSerialized,
		Replacements: []DBReplace{{From: "https://example.com", To: "http://local.test"}},
	})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected SQL\ngot:  %s\nwant: %s", got, want)
	}
}

func TestTransformSQLDumpSkipsInvalidSerializedWhenValidationEnabled(t *testing.T) {
	input := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:5:\"abc\";'),('plain','https://example.com');"
	want := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:5:\"abc\";'),('plain','http://local.test');"

	got, err := transformStringForTest(input, ReplacementOptions{
		Engine:             DBReplaceEngineGoSerialized,
		ValidateSerialized: true,
		Replacements:       []DBReplace{{From: "https://example.com", To: "http://local.test"}},
	})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected SQL\ngot:  %s\nwant: %s", got, want)
	}
}

func TestTransformSQLDumpSkipsInvalidSerializedWhenValidationEnabledAndReplacementMatches(t *testing.T) {
	input := "INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('bad','s:25:\"https://example.com\";');"
	want := input

	got, err := transformStringForTest(input, ReplacementOptions{
		Engine:             DBReplaceEngineGoSerialized,
		ValidateSerialized: true,
		Replacements:       []DBReplace{{From: "https://example.com", To: "http://local.test"}},
	})
	if err != nil {
		t.Fatalf("TransformSQLDump failed: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected SQL\ngot:  %s\nwant: %s", got, want)
	}
}

func TestPHPSerializationSupportsCommonTypes(t *testing.T) {
	input := `a:9:{s:3:"str";s:6:"Björk";s:4:"bool";b:1;s:4:"null";N;s:3:"int";i:42;s:5:"float";d:1.5;s:3:"obj";O:8:"stdClass":1:{s:3:"url";s:19:"https://example.com";}s:3:"arr";a:1:{s:19:"https://example.com";s:1:"x";}s:7:"softref";r:2;s:7:"hardref";R:2;}`
	got, err := transformSerializedPHP(input, []DBReplace{{From: "https://example.com", To: "http://local.test"}})
	if err != nil {
		t.Fatalf("transformSerializedPHP failed: %v", err)
	}
	if !strings.Contains(got, `s:17:"http://local.test"`) {
		t.Fatalf("replacement missing in serialized output: %s", got)
	}
	if !strings.Contains(got, `s:6:"Björk"`) {
		t.Fatalf("UTF-8 byte length was not preserved: %s", got)
	}
	if !strings.Contains(got, `r:2;`) || !strings.Contains(got, `R:2;`) {
		t.Fatalf("PHP references were not preserved: %s", got)
	}
	if _, err := parsePHPSerialized(got); err != nil {
		t.Fatalf("transformed value is invalid: %v", err)
	}
}
