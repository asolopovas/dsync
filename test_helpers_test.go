package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

const (
	remoteURLForTest = "https://example.com"
	localURLForTest  = "http://local.test"
)

func urlReplacementForTest() []DBReplace {
	return []DBReplace{{From: remoteURLForTest, To: localURLForTest}}
}

func goSerializedOptionsForTest(validateSerialized bool, skipColumns ...string) ReplacementOptions {
	return ReplacementOptions{
		Engine:             DBReplaceEngineGoSerialized,
		ValidateSerialized: validateSerialized,
		Replacements:       urlReplacementForTest(),
		SkipColumns:        skipColumns,
	}
}

func transformSQLForTest(input string, options ReplacementOptions) (string, error) {
	var output bytes.Buffer
	if err := TransformSQLDump(strings.NewReader(input), &output, options); err != nil {
		return "", err
	}
	return output.String(), nil
}

func readStringForTest(t *testing.T, reader io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read string: %v", err)
	}
	return string(data)
}

func assertStringEqual(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s\ngot:  %s\nwant: %s", label, got, want)
	}
}
