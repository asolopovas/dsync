package dsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type JsonConfig struct {
	SshHost   string             `json:"sshHost"`
	Remote    RemoteHostSettings `json:"remote"`
	Local     LocalHostSettings  `json:"local"`
	DbReplace []DbReplace        `json:"dbReplace"`
	Sync      []Sync             `json:"sync"`
}

type HostSettings struct {
	Host string `json:"host"`
	Db   string `json:"db"`
}

type RemoteHostSettings struct {
	*HostSettings
}

type LocalHostSettings struct {
	*HostSettings
}

type Sync struct {
	Remote  string   `json:"remote"`
	Local   string   `json:"local"`
	Exclude []string `json:"exclude"`
}

type DbReplace struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func GetJsonConfig(configPath string) (JsonConfig, error) {
	result := JsonConfig{}

	jsonConfig, err := os.ReadFile(configPath)

	json.Unmarshal([]byte(jsonConfig), &result)
	return result, err
}

func GenConfig() {
	defaultConf := []byte(`{
		"sshHost": "user@host.com",
		"remote": {
			"db": "db"
		},
		"local": {
			"db": "db"
		},
		"sync": [
			{
				"remote": "/home/user/public_html/wp-content/plugins",
				"local": "/home/user/www/host.test/wp-content/plugins",
				"exclude": [
					"some-plugins"
				]
			},
			{
				"remote": "/home/username/public_html/wp-content/uploads",
				"local": "/home/usernmae/www/host.test/wp-content/uploads",
				"exclude": []
			}
		],
		"dbReplace": [
			{
				"from": "host.com",
				"to": "host.test"
			},
			{
				"from": "/home/host/public_html",
				"to": "/home/user/www/project"
			}
		]
	}
`)

	writeErr := os.WriteFile("dsync-config.json", defaultConf, 0644)
	ErrChk(writeErr)

}

func DumpRemoteDB(config JsonConfig) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	// var stdin bytes.Buffer
	args := []string{config.SshHost, "mysqldump -uroot", config.Remote.Db}

	cmd := exec.Command("ssh", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Start()
	err := cmd.Wait()
	ErrChk(err)

	return stdout.Bytes(), err
}

func WriteToLocalDB(sqlDumpStr string, conf JsonConfig, dumpDb bool) {
	sqlDump := []byte(sqlDumpStr)
	var stdin bytes.Buffer
	stdin.Write(sqlDump)

	if dumpDb {
		os.WriteFile("db.sql", sqlDump, 0644)
	}

	os.Chdir(os.Getenv("HOME") + "/www/dev")

	args := []string{
		"compose",
		"exec",
		"-T",
		"mariadb",
		"mysql",
		"-uroot",
		"-psecret",
		conf.Local.Db,
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdin = &stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()
}

func addTrailingSlash(str string) string {
	match, _ := regexp.MatchString("/$", str)
	if match {
		return str
	}
	return str + "/"
}

func SyncFiles(conf JsonConfig) {
	for _, syncItem := range conf.Sync {
		remotePath := addTrailingSlash(syncItem.Remote)
		localPath := addTrailingSlash(syncItem.Local)

		args := []string{
			"-azr",
			"--info=progress2",
		}

		for _, v := range syncItem.Exclude {
			args = append(args, "--exclude="+v)
		}

		args = append(args, conf.SshHost+":"+remotePath)
		args = append(args, localPath)

		cmd := exec.Command("rsync", args...)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Printf("cmd.Run() failed with %s\n", err)
		}
	}
}

func SyncDb(conf JsonConfig) (string, error) {
	var localDump string
	remoteDump, err := DumpRemoteDB(conf)
	if err == nil {
		return "", err
	}

	// Replace String in Local DB
	localDump = string(remoteDump)
	for _, item := range conf.DbReplace {
		localDump = strings.Replace(localDump, item.From, item.To, -1)
	}

	return localDump, nil
}
