package main

import (
	"encoding/json"
	"testing"
)

func TestReproUserIssue(t *testing.T) {
	// Case 1: User's messy config
	// "https:\\/\\/example.com" -> "https:\/\/example.com" in memory
	jsonConfig := `[
        {
            "from": "https:\\/\\/example.com",
            "to": "http:\\/\\/example.test"
        }
    ]`

	var replacements []DBReplace
	if err := json.Unmarshal([]byte(jsonConfig), &replacements); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	dumpContent := `Some content "https:\\/\\/example.com" end`
	expected := `Some content "http:\\/\\/example.test" end`

	result := ApplyDBReplacements(dumpContent, replacements)

	if result != expected {
		t.Errorf("Messy config failed.\nGot:      %s\nExpected: %s", result, expected)
	}

	// Case 2: Clean config (The ideal way)
	cleanReplacements := []DBReplace{
		{From: "https://example.com", To: "http://example.test"},
	}

	resultClean := ApplyDBReplacements(dumpContent, cleanReplacements)
	if resultClean != expected {
		t.Errorf("Clean config failed.\nGot:      %s\nExpected: %s", resultClean, expected)
	}
}
