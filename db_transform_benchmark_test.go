package main

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func BenchmarkApplyDBReplacementsRaw(b *testing.B) {
	dump := strings.Repeat("INSERT INTO t VALUES ('https://example.com/path','https:\\/\\/example.com');\n", 1000)
	replacements := []DBReplace{{From: "https://example.com", To: "http://local.test"}}

	b.ReportAllocs()
	b.SetBytes(int64(len(dump)))
	for range b.N {
		_ = ApplyDBReplacements(dump, replacements)
	}
}

func BenchmarkTransformSQLDumpRaw(b *testing.B) {
	dump := strings.Repeat("INSERT INTO t VALUES ('https://example.com/path','https:\\/\\/example.com');\n", 1000)
	options := ReplacementOptions{
		Engine:       DBReplaceEngineRaw,
		Replacements: []DBReplace{{From: "https://example.com", To: "http://local.test"}},
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(dump)))
	for range b.N {
		if err := TransformSQLDump(strings.NewReader(dump), io.Discard, options); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransformSQLDumpGoSerialized(b *testing.B) {
	dump := strings.Repeat("INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('home','s:19:\"https://example.com\";'),('nested','a:1:{s:3:\"url\";s:19:\"https://example.com\";}');\n", 1000)
	options := ReplacementOptions{
		Engine:             DBReplaceEngineGoSerialized,
		ValidateSerialized: true,
		Replacements:       []DBReplace{{From: "https://example.com", To: "http://local.test"}},
		SkipColumns:        []string{"guid"},
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(dump)))
	for range b.N {
		if err := TransformSQLDump(strings.NewReader(dump), io.Discard, options); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransformSQLDumpLargeGoSerialized(b *testing.B) {
	var dump strings.Builder
	for i := range 10000 {
		fmt.Fprintf(&dump, "INSERT INTO `wp_options` (`option_id`,`option_name`,`option_value`) VALUES (%d,'home','s:19:\"https://example.com\";');\n", i)
	}
	dumpString := dump.String()
	options := ReplacementOptions{
		Engine:       DBReplaceEngineGoSerialized,
		Replacements: []DBReplace{{From: "https://example.com", To: "http://local.test"}},
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(dumpString)))
	for range b.N {
		if err := TransformSQLDump(strings.NewReader(dumpString), io.Discard, options); err != nil {
			b.Fatal(err)
		}
	}
}
