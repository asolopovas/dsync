package main

import (
	"encoding/json"
	"testing"
)

func TestReproUserIssue(t *testing.T) {
	// Case 1: User's messy config
	// "https:\\/\\/avianese.com" -> "https:\/\/avianese.com" in memory
	jsonConfig := `[
        {
            "from": "https:\\/\\/avianese.com",
            "to": "http:\\/\\/avianese.test"
        }
    ]`

	var replacements []DBReplace
	if err := json.Unmarshal([]byte(jsonConfig), &replacements); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	dumpContent := `Some content "https:\\/\\/avianese.com" end`
	expected := `Some content "http:\\/\\/avianese.test" end`

	result := ApplyDBReplacements(dumpContent, replacements)

	if result != expected {
		t.Errorf("Messy config failed.\nGot:      %s\nExpected: %s", result, expected)
	}

	// Case 2: Clean config (The ideal way)
	cleanReplacements := []DBReplace{
		{From: "https://avianese.com", To: "http://avianese.test"},
	}

	resultClean := ApplyDBReplacements(dumpContent, cleanReplacements)
	if resultClean != expected {
		t.Errorf("Clean config failed.\nGot:      %s\nExpected: %s", resultClean, expected)
	}
}
