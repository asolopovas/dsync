package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateConfigOmitsDerivedDBOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dsync-config.json")
	if err := GenerateConfig(path); err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("generated config is not valid JSON: %v", err)
	}

	for _, key := range []string{"dbReplaceEngine", "validateSerialized", "skipColumns"} {
		if _, ok := raw[key]; ok {
			t.Fatalf("generated config includes derived option %q", key)
		}
	}
}

func TestReplacementOptionsFromMinimalWordPressConfig(t *testing.T) {
	cfg := &Config{
		Sync: []SyncPath{{Local: "/home/user/www/example.test/wp-content/uploads"}},
	}
	replacements := []DBReplace{{From: "https://example.com", To: "http://example.test"}}

	options := ReplacementOptionsFromConfig(cfg, replacements)
	if options.Engine != DBReplaceEngineGoSerialized {
		t.Fatalf("Engine = %q, want %q", options.Engine, DBReplaceEngineGoSerialized)
	}
	if !options.ValidateSerialized {
		t.Fatal("ValidateSerialized = false, want true by default")
	}
	if !containsFold(options.SkipColumns, "guid") {
		t.Fatalf("SkipColumns = %v, want guid default", options.SkipColumns)
	}
}

func TestReplacementOptionsUseNoopEngineWithoutReplacements(t *testing.T) {
	cfg := &Config{
		Sync: []SyncPath{{Local: "/home/user/www/example.test/wp-content/uploads"}},
	}

	options := ReplacementOptionsFromConfig(cfg, nil)
	if options.Engine != DBReplaceEngineNone {
		t.Fatalf("Engine = %q, want %q", options.Engine, DBReplaceEngineNone)
	}
}

func TestReplacementOptionsRespectExplicitValidationOverride(t *testing.T) {
	validateSerialized := false
	cfg := &Config{ValidateSerialized: &validateSerialized}

	options := ReplacementOptionsFromConfig(cfg, []DBReplace{{From: "a", To: "b"}})
	if options.ValidateSerialized {
		t.Fatal("ValidateSerialized = true, want explicit false override")
	}
}
