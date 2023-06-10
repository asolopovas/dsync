# Dsync
Dsync is a Go-built tool that syncs files and databases between different environments, majorly catering to WordPress projects. It utilizes `rsync` for file sync, MySQL's `mysqldump` for database sync, and allows process customization via a JSON config file.

## Features
- File and Database sync using `rsync` and MySQL's `mysqldump` respectively.
- Process customization with a JSON config file.
- Default config file generation.

## Prerequisites
- Go 1.20 or later.
- Access to both local and remote servers.
- SSH access to the remote server.

## Installation
Clone the repo and build the app:
```
git clone https://github.com/asolopovas/dsync.git go build -o $ABSOLUTE_PATH_TO_DSYNC main.go
```
Add it to your system path or copy executable `sudo cp dsync /usr/local/bin/`.

## Usage
Dsync operates on a config file (`dsync-config.json`). Generate a default config file using `-g` flag:
```
dsync -g
```

Modify `dsync-config.json` as per your needs. The config file comprises:

- SSH Host details.
- Remote and Local host settings.
- File and Directory sync details.
- Database replace rules for various environments.

After configuring, sync files and database with:

Sync All:
```
dsync -a
```

Sync Files only:
```
dsync -f
```

Sync Database only:
```
dsync -d
```

For a custom config path use `-c` flag:
```
dsync -c "/path/to/your/config.json"
```

## Contributing
Open issues, submit pull requests, and share feedback.

## License
MIT License.
