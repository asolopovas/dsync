#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd -P)"
APP_NAME="dsync"
OUTPUT_DIR="${REPO_ROOT}/releases/dev"
VERSION_FILE="${REPO_ROOT}/version"

usage() {
	cat <<EOF
Usage: ${0##*/} [--output DIR] [--version VERSION]

Builds release archives for Linux, macOS, and Windows into releases/dev/.

Options:
  --output DIR       Output directory (default: releases/dev)
  --version VERSION  Version string (default: contents of ./version)
  -h, --help         Show this help
EOF
}

VERSION=""
while [[ $# -gt 0 ]]; do
	case "$1" in
	--output)
		[[ $# -ge 2 ]] || {
			printf 'ERROR: --output requires a value\n' >&2
			exit 1
		}
		OUTPUT_DIR="$2"
		shift 2
		;;
	--version)
		[[ $# -ge 2 ]] || {
			printf 'ERROR: --version requires a value\n' >&2
			exit 1
		}
		VERSION="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf 'ERROR: unknown argument: %s\n' "$1" >&2
		usage >&2
		exit 1
		;;
	esac
done

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || {
		printf 'ERROR: required command not found: %s\n' "$1" >&2
		exit 1
	}
}

archive_zip() {
	local -r source="$1"
	local -r archive="$2"
	local -r source_dir="$(dirname -- "$source")"
	local -r file="$(basename -- "$source")"

	if command -v zip >/dev/null 2>&1; then
		(cd "$source_dir" && zip -q "$archive" "$file")
	elif command -v python3 >/dev/null 2>&1; then
		python3 - "$source" "$archive" <<'PY'
import pathlib
import sys
import zipfile
source = pathlib.Path(sys.argv[1])
archive = pathlib.Path(sys.argv[2])
with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as zf:
    zf.write(source, arcname=source.name)
PY
	else
		printf 'ERROR: zip or python3 is required for Windows archives\n' >&2
		exit 1
	fi
}

build_target() {
	local -r goos="$1"
	local -r goarch="$2"
	local ext=""
	local archive_ext="tar.gz"
	if [[ "$goos" == "windows" ]]; then
		ext=".exe"
		archive_ext="zip"
	fi

	local -r target_name="${APP_NAME}_${VERSION}_${goos}_${goarch}"
	local -r work_dir="${OUTPUT_DIR}/${target_name}"
	local -r binary="${work_dir}/${APP_NAME}${ext}"

	mkdir -p "$work_dir"
	printf 'building %s/%s\n' "$goos" "$goarch"
	(cd "$REPO_ROOT" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED="${CGO_ENABLED:-0}" go build -trimpath -ldflags="-s -w" -o "$binary" .)

	if [[ "$goos" != "windows" ]]; then
		chmod +x "$binary"
		tar -C "$OUTPUT_DIR" -czf "${OUTPUT_DIR}/${target_name}.${archive_ext}" "$target_name/${APP_NAME}"
	else
		archive_zip "$binary" "${OUTPUT_DIR}/${target_name}.${archive_ext}"
	fi
	rm -rf -- "$work_dir"
}

main() {
	require_cmd go
	require_cmd tar
	require_cmd sha256sum

	if [[ -z "$VERSION" ]]; then
		[[ -f "$VERSION_FILE" ]] || {
			printf 'ERROR: missing version file: %s\n' "$VERSION_FILE" >&2
			exit 1
		}
		VERSION="$(tr -d '[:space:]' <"$VERSION_FILE")"
	fi
	[[ -n "$VERSION" ]] || {
		printf 'ERROR: version is empty\n' >&2
		exit 1
	}

	rm -rf -- "$OUTPUT_DIR"
	mkdir -p "$OUTPUT_DIR"

	build_target linux amd64
	build_target linux arm64
	build_target darwin amd64
	build_target darwin arm64
	build_target windows amd64
	build_target windows arm64

	(cd "$OUTPUT_DIR" && sha256sum ./* >checksums.txt)
	printf 'release artifacts written to %s\n' "$OUTPUT_DIR"
}

main "$@"
