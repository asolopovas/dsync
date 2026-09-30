package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestConfigEnvironmentMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dsync-config.json")
	original := `{"sshHost":"user@example.com","port":"22","remote":{"db":"remote"},"local":{"db":"local"},"sync":[{"remote":"/remote","local":"/local","exclude":["cache"],"replace":true}],"dbReplace":[{"from":"https://example.com","to":"http://local.test"}],"future":{"enabled":true}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(dir, ".env")
	existing := "APP_NAME=Project\nDSYNC_LOCAL_DB=existing\n"
	if err := os.WriteFile(envPath, []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Local.DB != "existing" || cfg.Remote.DB != "remote" || cfg.SSHHost != "user@example.com" {
		t.Fatalf("incorrect resolved endpoints")
	}
	if len(cfg.Sync[0].Exclude) != 1 || !cfg.Sync[0].Replace {
		t.Fatal("lost sync options")
	}
	template := readTestFile(t, path)
	if !strings.Contains(template, "${DSYNC_SSH_HOST}") || !strings.Contains(template, `"future"`) {
		t.Fatal("template missing reference or unknown field")
	}
	env := readTestFile(t, envPath)
	if !strings.HasPrefix(env, existing) || strings.Count(env, "DSYNC_LOCAL_DB=") != 1 {
		t.Fatal("existing env was modified")
	}
	if _, err := LoadConfig(path); err != nil {
		t.Fatal(err)
	}
	if readTestFile(t, path) != template || readTestFile(t, envPath) != env {
		t.Fatal("second load changed files")
	}
	for _, p := range []string{path, envPath} {
		info, _ := os.Stat(p)
		if info.Mode().Perm() != 0600 {
			t.Fatal("file not private")
		}
	}
}

func TestConfigEnvironmentLiteralValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dsync-config.json")
	value := "quotes\" ' backslash\\ $literal # newline\nnext"
	data, _ := json.Marshal(Config{SSHHost: "host", Port: "22", DBReplace: []DBReplace{{From: value, To: "to"}}})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.DBReplace[0].From != value {
			t.Fatal("literal value changed")
		}
	}
}

func TestConfigEnvironmentMissingReferenceDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dsync-config.json")
	data := `{"sshHost":"host","port":"${DSYNC_TEST_MISSING_482}"}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("accepted missing variable")
	}
	if readTestFile(t, path) != data {
		t.Fatal("modified config on failure")
	}
	if _, err := os.Stat(filepath.Join(dir, ".env")); !os.IsNotExist(err) {
		t.Fatal("created env on failure")
	}
}

func TestCheckConfigDatabaseOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dsync-config.json")
	data := `{"sshHost":"host","port":"22","remote":{"db":"remote"},"local":{"db":"local"},"sync":[]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--check-config", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
