# Dsync

Dsync is a command-line tool written in Go for synchronizing files and databases between local and remote environments. It is designed primarily for web development workflows (e.g., WordPress) but is flexible enough for other use cases. It uses `rsync` for file synchronization and `mysqldump`/`mariadb-dump` for database synchronization.

## Features

- **File Synchronization:** Efficient file syncing using `rsync`.
- **Database Synchronization:** Supports MySQL and MariaDB. Automatically handles database dumps, transfers, and imports.
- **Search and Replace:** Performs string replacements on the database dump during synchronization (useful for changing domain names).
- **Reverse Sync:** Support for syncing from local to remote environments with automatic remote backups.
- **Configuration:** Simple JSON configuration file.
- **SSH Support:** Configurable SSH host and port.

## Prerequisites

- Go 1.20 or later (for building from source).
- `rsync` installed on both local and remote machines.
- `mysqldump` or `mariadb-dump` installed on both local and remote machines.
- SSH access to the remote server.

## Installation

### Using `go install`

```bash
go install github.com/asolopovas/dsync@latest
```

### Building from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/asolopovas/dsync.git
   cd dsync
   ```

2. Build the binary:
   ```bash
   go build -o dsync main.go
   ```

3. Move the binary to a directory in your system PATH (e.g., `/usr/local/bin`):
   ```bash
   sudo mv dsync /usr/local/bin/
   ```

## Configuration

Dsync requires a configuration file, typically named `dsync-config.json`. You can generate a default configuration file using the `-g` flag.

```bash
dsync -g
```

### Configuration File Structure

```json
{
  "sshHost": "user@remote-host.com",
  "port": "22",
  "remote": {
    "host": "remote-db-host",
    "db": "remote_db_name"
  },
  "local": {
    "host": "localhost",
    "db": "local_db_name"
  },
  "dbReplace": [
    {
      "from": "http://remote-site.com",
      "to": "http://local-site.test"
    }
  ],
  "sync": [
    {
      "remote": "/var/www/html/wp-content/uploads/",
      "local": "./wp-content/uploads/",
      "exclude": [
        "*.log",
        "cache/"
      ]
    }
  ]
}
```

- **sshHost**: The SSH connection string (user@host).
- **port**: The SSH port (default is usually 22).
- **remote/local**: Database connection settings for remote and local environments.
- **dbReplace**: List of string replacements to apply to the database dump.
- **sync**: List of file paths to synchronize. Supports exclude patterns.

## Usage

Run `dsync` from the directory containing your configuration file, or specify the path using the `-c` flag.

### Common Commands

**Sync everything (files and database) from remote to local:**
```bash
dsync -a
```

**Sync only files:**
```bash
dsync -f
```

**Sync only database:**
```bash
dsync -d
```

**Reverse sync (Local to Remote):**
Use the `-r` flag to sync from your local machine to the remote server.
```bash
dsync -a -r
```

**Dump database to file:**
```bash
dsync --dump
```

**Show version:**
```bash
dsync -v
```

### Flags

- `-a`, `--all`: Sync both files and database.
- `-f`, `--files`: Sync files only.
- `-d`, `--db`: Sync database only.
- `-r`, `--reverse`: Reverse sync (Local to Remote).
- `--dump`: Dump database to a file without importing.
- `-c`, `--config`: Specify a custom configuration file path (default: `dsync-config.json`).
- `-g`, `--gen`: Generate a default configuration file.
- `-v`, `--version`: Display version information.

## License

MIT License
