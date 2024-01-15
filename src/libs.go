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
	Port      string             `json:"port"`
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
		"host": "user@host.com",
		"port": 22,
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

func GetRemoveSqlString(config JsonConfig) string {
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

	return stdout.String()
}

func createUserAndDB(dbName string, confLoc string) {

	query := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS `%s`@'%%' IDENTIFIED BY 'secret'; "+
			"CREATE DATABASE IF NOT EXISTS `%s`; "+
			"GRANT ALL PRIVILEGES ON `%s`.* TO `%s`@'%%';",
		dbName, dbName, dbName, dbName,
	)

	args := []string{
		"compose",
		"-f", confLoc,
		"exec",
		"-T",
		"mariadb",
		"mariadb",
		"-uroot",
		"-psecret",
		"-e",
	}

	// fmt.Println("docker " + strings.Join(args, " ") + " QueryString")
	cmd := exec.Command("docker", append(args, query)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Error:", err)
	}
}

func WriteRemoteToLocalDb(conf JsonConfig, dumpDB bool) {
	transformedSqlString := RemoteSqlStringToLocal(conf)

	msg := "Writing remote database `" + conf.Remote.Db + "` to local with replacements: "
	dashes := strings.Repeat("-", len(msg)+2)
	fmt.Println(dashes)
	fmt.Println("Database `" + conf.Local.Db + "` will be created if not exist")
	fmt.Println(msg)

	maxLen := 0
	for _, item := range conf.DbReplace {
		if len(item.From) > maxLen {
			maxLen = len(item.From)
		}
	}

	for _, item := range conf.DbReplace {
		fmt.Printf("%-*s -> %s\n", maxLen, item.From, item.To)
	}

	WriteToLocalDb(transformedSqlString, conf, dumpDB)

	fmt.Println(dashes + " \n")
}

func WriteToLocalDb(sqlDumpStr string, conf JsonConfig, dumpDb bool) {

	confLoc := os.Getenv("HOME") + "/www/dev/docker-compose.yml"

	createUserAndDB(conf.Local.Db, confLoc)

	sqlDump := []byte(sqlDumpStr)
	var stdin bytes.Buffer
	stdin.Write(sqlDump)

	if dumpDb {
		fmt.Println("saving db.sql...")
		os.WriteFile("db.sql", sqlDump, 0644)
	}

	args := []string{
		"compose",
		"-f", confLoc,
		"exec",
		"-T",
		"mariadb",
		"mariadb",
		"-uroot",
		"-psecret",
		conf.Local.Db,
	}

	// fmt.Println("docker " + strings.Join(args, " "))
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
	msg := "Syncing Files from remote to local using rsync"

	maxLen := 0
	for _, item := range conf.Sync {
		if len(item.Remote) > maxLen {
			maxLen = len(item.Local)
		}
	}
	dashes := strings.Repeat("-", maxLen*2+5)
	fmt.Println(dashes)
	fmt.Println(msg)

	fmt.Println(dashes)

	for _, syncItem := range conf.Sync {
		remotePath := addTrailingSlash(syncItem.Remote)
		localPath := addTrailingSlash(syncItem.Local)

		fmt.Printf("%-*s -> %s\n\n", maxLen, remotePath, localPath)

		fmt.Println("Excluding:")
		for _, v := range syncItem.Exclude {
			fmt.Println("  - " + v)
		}

		fmt.Println("")

		args := []string{
			"-azr",
			"-e",
			"ssh -p " + conf.Port,
			"--info=progress2",
		}

		for _, v := range syncItem.Exclude {
			args = append(args, "--exclude="+v)
		}

		args = append(args, conf.SshHost+":"+remotePath)
		args = append(args, localPath)

		// fmt.Println("rsync " + strings.Join(args, " "))
		cmd := exec.Command("rsync", args...)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Printf("cmd.Run() failed with %s\n", err)
		}

		fmt.Println("")

	}
}

func RemoteSqlStringToLocal(conf JsonConfig) string {
	sqlString := GetRemoveSqlString(conf)

	for _, item := range conf.DbReplace {
		sqlString = strings.Replace(sqlString, item.From, item.To, -1)
	}

	return sqlString
}
