package main

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func rawDumpForBenchmark() string {
	return strings.Repeat("INSERT INTO t VALUES ('https://example.com/path','https:\\/\\/example.com');\n", 1000)
}

func serializedDumpForBenchmark() string {
	return strings.Repeat("INSERT INTO `wp_options` (`option_name`,`option_value`) VALUES ('home','s:19:\"https://example.com\";'),('nested','a:1:{s:3:\"url\";s:19:\"https://example.com\";}');\n", 1000)
}

func benchmarkTransformSQLDump(b *testing.B, dump string, options ReplacementOptions) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(dump)))
	for range b.N {
		if err := TransformSQLDump(strings.NewReader(dump), io.Discard, options); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApplyDBReplacementsRaw(b *testing.B) {
	dump := rawDumpForBenchmark()
	replacements := urlReplacementForTest()

	b.ReportAllocs()
	b.SetBytes(int64(len(dump)))
	for range b.N {
		_ = ApplyDBReplacements(dump, replacements)
	}
}

func BenchmarkTransformSQLDumpRaw(b *testing.B) {
	benchmarkTransformSQLDump(b, rawDumpForBenchmark(), ReplacementOptions{
		Engine:       DBReplaceEngineRaw,
		Replacements: urlReplacementForTest(),
	})
}

func BenchmarkTransformSQLDumpGoSerialized(b *testing.B) {
	benchmarkTransformSQLDump(b, serializedDumpForBenchmark(), goSerializedOptionsForTest(true, "guid"))
}

func BenchmarkTransformSQLDumpLargeGoSerialized(b *testing.B) {
	var dump strings.Builder
	for i := range 10000 {
		fmt.Fprintf(&dump, "INSERT INTO `wp_options` (`option_id`,`option_name`,`option_value`) VALUES (%d,'home','s:19:\"https://example.com\";');\n", i)
	}
	benchmarkTransformSQLDump(b, dump.String(), goSerializedOptionsForTest(false))
}
