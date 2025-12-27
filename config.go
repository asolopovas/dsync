package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	SSHHost   string       `json:"sshHost"`
	Port      string       `json:"port"`
	Remote    HostSettings `json:"remote"`
	Local     HostSettings `json:"local"`
	DBReplace []DBReplace  `json:"dbReplace"`
	Sync      []SyncPath   `json:"sync"`
}

type HostSettings struct {
	Host string `json:"host"`
	DB   string `json:"db"`
}

type SyncPath struct {
	Remote  string   `json:"remote"`
	Local   string   `json:"local"`
	Exclude []string `json:"exclude"`
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
				Remote: "/home/username/public_html/wp-content/uploads",
				Local:  "/home/usernmae/www/host.test/wp-content/uploads",
			},
		},
		DBReplace: []DBReplace{
			{From: "host.com", To: "host.test"},
			{From: "/home/host/public_html", To: "/home/user/www/project"},
		},
	}

	data, err := json.MarshalIndent(defaultConf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
