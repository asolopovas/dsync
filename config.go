package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Config struct {
	SSHHost   string       `json:"sshHost"`
	Port      string       `json:"port"`
	Remote    HostSettings `json:"remote"`
	Local     HostSettings `json:"local"`
	DBReplace []DBReplace  `json:"dbReplace"`
	// Deprecated optional overrides. Minimal configs omit these; dsync derives safe defaults.
	DBReplaceEngine    string     `json:"dbReplaceEngine,omitempty"`
	ValidateSerialized *bool      `json:"validateSerialized,omitempty"`
	SkipColumns        []string   `json:"skipColumns,omitempty"`
	Sync               []SyncPath `json:"sync"`
}

type HostSettings struct {
	Host string `json:"host"`
	DB   string `json:"db"`
}

type SyncPath struct {
	Remote  string   `json:"remote"`
	Local   string   `json:"local"`
	Exclude []string `json:"exclude"`
	Replace bool     `json:"replace,omitempty"`
}

type DBReplace struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

func GenerateConfig(path string) error {
	defaultConf := Config{
		SSHHost: "user@host.com",
		Port:    "22",
		Remote: HostSettings{
			DB: "db",
		},
		Local: HostSettings{
			DB: "db",
		},
		Sync: []SyncPath{
			{
				Remote:  "/home/user/public_html/wp-content/plugins",
				Local:   "/home/user/www/host.test/wp-content/plugins",
				Exclude: []string{"some-plugins"},
			},
			{
				Remote: "/home/user/public_html/wp-content/uploads",
				Local:  "/home/user/www/host.test/wp-content/uploads",
			},
		},
		DBReplace: []DBReplace{
			{From: "https://some.domain.com", To: "http://local.test"},
			{From: "/home/remote/public_html", To: "/home/user/www/project"},
		},
	}

	data, err := json.MarshalIndent(defaultConf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks only the fields used by the requested operations. Unknown JSON
// fields remain accepted by encoding/json for compatibility with existing configs.
func (cfg *Config) Validate(files, database, reverse bool) error {
	if !files && !database {
		return nil
	}
	host := cfg.SSHHost
	if host == "" || strings.HasPrefix(host, "-") || strings.ContainsAny(host, "/\\'\";|&$`()<>\x00") || strings.ContainsFunc(host, unicode.IsSpace) {
		return fmt.Errorf("invalid sshHost")
	}
	parts := strings.Split(host, "@")
	if len(parts) > 2 || parts[0] == "" {
		return fmt.Errorf("invalid sshHost")
	}
	hostname := parts[len(parts)-1]
	if hostname == "" || strings.HasPrefix(hostname, "-") {
		return fmt.Errorf("invalid sshHost")
	}
	if strings.ContainsAny(hostname, ":[]") && net.ParseIP(strings.Trim(hostname, "[]")) == nil {
		return fmt.Errorf("invalid sshHost")
	}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 || strings.Trim(cfg.Port, "0123456789") != "" {
		return fmt.Errorf("port must be numeric in range 1..65535")
	}
	if files {
		if len(cfg.Sync) == 0 {
			return fmt.Errorf("file sync requires at least one sync path")
		}
		for i, path := range cfg.Sync {
			if strings.TrimSpace(path.Local) == "" || strings.TrimSpace(path.Remote) == "" || strings.ContainsRune(path.Local+path.Remote, 0) {
				return fmt.Errorf("sync[%d] requires local and remote directories", i)
			}
		}
	}
	if database {
		if strings.TrimSpace(cfg.Local.DB) == "" || strings.TrimSpace(cfg.Remote.DB) == "" || strings.ContainsRune(cfg.Local.DB+cfg.Remote.DB, 0) {
			return fmt.Errorf("database sync requires local.db and remote.db")
		}
	}
	switch strings.TrimSpace(cfg.DBReplaceEngine) {
	case "", DBReplaceEngineNone, DBReplaceEngineRaw, DBReplaceEngineGoSerialized:
	default:
		return fmt.Errorf("unsupported dbReplaceEngine %q", cfg.DBReplaceEngine)
	}
	for i, replacement := range cfg.DBReplace {
		source := replacement.From
		if reverse && database {
			source = replacement.To
		}
		if source == "" {
			return fmt.Errorf("dbReplace[%d] has empty effective source", i)
		}
	}
	return nil
}
