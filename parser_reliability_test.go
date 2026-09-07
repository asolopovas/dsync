package main

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPHPRejectsUntrustedSizesAndDepth(t *testing.T) {
	values := []string{`s:-1:"";`, `s:9223372036854775807:"x";`, `s:99999999999999999999999999:"";`, `a:-1:{}`, `a:9223372036854775807:{}`, `O:-1:"":0:{}`, `O:9223372036854775807:"X":0:{}`, `O:1:"X":-1:{}`, `O:1:"X":999999999999999999999999:{}`, `b:2;`, `bX1;`, `r:-1;`, `R:0;`, `d:garbage;`, strings.Repeat("a:1:{i:0;", maxSerializedDepth+2) + "N;" + strings.Repeat("}", maxSerializedDepth+2)}
	for _, value := range values {
		if _, err := parsePHPSerialized(value); err == nil {
			t.Errorf("accepted %q", value)
		}
		got, err := transformSQLString(value, ReplacementOptions{Replacements: []DBReplace{{From: "X", To: "Y"}}, ValidateSerialized: true})
		if isSerializedPHP(value) && (err != nil || got != value) {
			t.Errorf("invalid value changed: %q -> %q (%v)", value, got, err)
		}
	}
	nested := serializePHPValue(phpValue{Kind: phpString, String: `s:99:"example";`})
	got, err := transformSerializedPHP(nested, []DBReplace{{From: "example", To: "changed"}})
	if err != nil || got != nested {
		t.Fatalf("nested invalid value changed: %q %v", got, err)
	}
}

func TestSQLCommentsQuotesAndMalformedInput(t *testing.T) {
	options := ReplacementOptions{Engine: DBReplaceEngineGoSerialized, Replacements: []DBReplace{{From: "remote", To: "local"}}}
	for _, prefix := range []string{"-- comment ; 'remote'\n", "# comment ; remote\n", "/* comment ; 'remote' */\n", "/*!40101 SET @X=';'; */;\n"} {
		input := prefix + "INSERT INTO `semi;table` (`v`) VALUES ('remote;it''s');"
		got, err := transformSQLForTest(input, options)
		if err != nil || got != prefix+"INSERT INTO `semi;table` (`v`) VALUES ('local;it\\'s');" {
			t.Fatalf("%q => %q %v", input, got, err)
		}
	}
	for _, input := range []string{"INSERT INTO t VALUES ('x';", "INSERT INTO t VALUES ('x'", "INSERT INTO t VALUES (,);", "INSERT INTO t VALUES ('x'),;", "INSERT INTO t VALUES ('x')('y');", "INSERT INTO t;", "INSERT INTO t VALUES ('x"} {
		if _, err := transformSQLForTest(input, options); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	for _, input := range []string{`SELECT 'a;''b';`, `SELECT "a;""b";`, `SELECT ` + "`a;``b`" + `;`, `/* ; */ SELECT 1;`, `-- ;
SELECT 1;`} {
		reader := bufio.NewReader(strings.NewReader(input + "SELECT 2;"))
		got, err := readSQLStatement(reader)
		if err != nil || got != input {
			t.Fatalf("framing %q: %q %v", input, got, err)
		}
	}
	for _, input := range []string{`SELECT 'x`, `/* missing end`} {
		_, err := readSQLStatement(bufio.NewReader(strings.NewReader(input)))
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("truncated %q: %v", input, err)
		}
	}
}

func FuzzPHPSerialized(f *testing.F) {
	for _, seed := range []string{`N;`, `a:1:{i:0;s:3:"abc";}`, `a:2:{i:0;s:3:"abc";i:1;R:2;}`, `O:1:"X":1:{s:1:"x";r:1;}`, `s:-1:"";`, `a:999999999999999999999999:{}`, `O:-1:"":0:{}`, strings.Repeat("a:1:{i:0;", 24) + "N;" + strings.Repeat("}", 24)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		parsed, err := parsePHPSerialized(input)
		if err != nil {
			return
		}
		encoded := serializePHPValue(parsed)
		reparsed, err := parsePHPSerialized(encoded)
		if err != nil {
			t.Fatalf("round trip: %v", err)
		}
		if serializePHPValue(reparsed) != encoded {
			t.Fatal("unstable round trip")
		}
		transformed, err := transformSerializedPHP(input, []DBReplace{{From: "abc", To: "longer ü"}})
		if err == nil {
			if _, err := parsePHPSerialized(transformed); err != nil {
				t.Fatalf("invalid transformation: %v", err)
			}
		}
	})
}

func FuzzSQLDump(f *testing.F) {
	for _, seed := range []string{"INSERT INTO t VALUES ('x');", "-- ;\nINSERT INTO t VALUES ('a''b');", "INSERT INTO t VALUES ('s:-1:\"\";');", "/* unfinished", "INSERT INTO t VALUES ('\\"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		options := ReplacementOptions{Engine: DBReplaceEngineGoSerialized, ValidateSerialized: true, Replacements: []DBReplace{{From: "remote", To: "local"}}}
		first, err := transformSQLForTest(input, options)
		if err != nil {
			return
		}
		second, err := transformSQLForTest(first, options)
		if err != nil {
			t.Fatalf("transformed SQL rejected: %v", err)
		}
		if second != first {
			t.Fatal("non-idempotent noncascading replacement")
		}
	})
}
