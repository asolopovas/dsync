package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"

	"github.com/joho/godotenv"
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

	data, err = migrateConfigEnvironment(path, data)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to resolve config: %w", err)
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

var configVariable = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func migrateConfigEnvironment(path string, data []byte) ([]byte, error) {
	envPath := filepath.Join(filepath.Dir(path), ".env")
	envData, err := os.ReadFile(envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read .env: %w", err)
	}
	values, err := godotenv.Unmarshal(string(envData))
	if err != nil {
		return nil, fmt.Errorf("invalid .env syntax")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	additions := make(map[string]string)
	changed := false
	resolve := func(value *string, key string) error {
		if *value == "" {
			return nil
		}
		if !configVariable.MatchString(*value) {
			if _, exists := values[key]; !exists {
				values[key] = *value
				additions[key] = *value
			}
			*value = "${" + key + "}"
			changed = true
		}
		for _, match := range configVariable.FindAllStringSubmatch(*value, -1) {
			if _, exists := values[match[1]]; !exists {
				if _, exists := os.LookupEnv(match[1]); !exists {
					return fmt.Errorf("missing .env variable %s", match[1])
				}
			}
		}
		return nil
	}
	for _, field := range []struct{ name, key string }{{"sshHost", "DSYNC_SSH_HOST"}, {"port", "DSYNC_PORT"}} {
		if entry, exists := raw[field.name]; exists {
			var value string
			if err := json.Unmarshal(entry, &value); err != nil {
				return nil, err
			}
			if err := resolve(&value, field.key); err != nil {
				return nil, err
			}
			raw[field.name], _ = json.Marshal(value)
		}
	}
	for _, name := range []string{"remote", "local"} {
		if entry, exists := raw[name]; exists {
			var host map[string]json.RawMessage
			if err := json.Unmarshal(entry, &host); err != nil {
				return nil, err
			}
			for _, field := range []string{"host", "db"} {
				if entry, exists := host[field]; exists {
					var value string
					if err := json.Unmarshal(entry, &value); err != nil {
						return nil, err
					}
					if err := resolve(&value, "DSYNC_"+strings.ToUpper(name)+"_"+strings.ToUpper(field)); err != nil {
						return nil, err
					}
					host[field], _ = json.Marshal(value)
				}
			}
			raw[name], _ = json.Marshal(host)
		}
	}
	for _, group := range []struct {
		name   string
		fields []string
	}{{"sync", []string{"remote", "local"}}, {"dbReplace", []string{"from", "to"}}} {
		if entry, exists := raw[group.name]; exists {
			var rows []map[string]json.RawMessage
			if err := json.Unmarshal(entry, &rows); err != nil {
				return nil, err
			}
			for i, row := range rows {
				for _, field := range group.fields {
					if entry, exists := row[field]; exists {
						var value string
						if err := json.Unmarshal(entry, &value); err != nil {
							return nil, err
						}
						if err := resolve(&value, fmt.Sprintf("DSYNC_%s_%d_%s", strings.ToUpper(group.name), i, strings.ToUpper(field))); err != nil {
							return nil, err
						}
						row[field], _ = json.Marshal(value)
					}
				}
			}
			raw[group.name], _ = json.Marshal(rows)
		}
	}
	template, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, err
	}
	// Expand decoded strings, never raw JSON, so quotes and backslashes stay valid.
	var tree any
	if err := json.Unmarshal(template, &tree); err != nil {
		return nil, err
	}
	var expand func(any) (any, error)
	expand = func(node any) (any, error) {
		switch value := node.(type) {
		case string:
			var missing string
			result := configVariable.ReplaceAllStringFunc(value, func(reference string) string {
				key := configVariable.FindStringSubmatch(reference)[1]
				if v, exists := values[key]; exists {
					return v
				}
				if v, exists := os.LookupEnv(key); exists {
					return v
				}
				missing = key
				return ""
			})
			if missing != "" {
				return nil, fmt.Errorf("missing .env variable %s", missing)
			}
			return result, nil
		case map[string]any:
			for key, child := range value {
				resolved, err := expand(child)
				if err != nil {
					return nil, err
				}
				value[key] = resolved
			}
		case []any:
			for i, child := range value {
				resolved, err := expand(child)
				if err != nil {
					return nil, err
				}
				value[i] = resolved
			}
		}
		return node, nil
	}
	resolved, err := expand(tree)
	if err != nil {
		return nil, err
	}
	result, err := json.Marshal(resolved)
	if err != nil {
		return nil, err
	}
	if len(additions) > 0 {
		appended, err := godotenv.Marshal(additions)
		if err != nil {
			return nil, err
		}
		if len(envData) > 0 && envData[len(envData)-1] != '\n' {
			envData = append(envData, '\n')
		}
		envData = append(envData, []byte(appended+"\n")...)
		if err := writePrivateConfig(envPath, envData); err != nil {
			return nil, err
		}
	}
	if changed {
		if err := writePrivateConfig(path, append(template, '\n')); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func writePrivateConfig(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace nonregular file %s", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".dsync-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	if err := errors.Join(writeErr, syncErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
