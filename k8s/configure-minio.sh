#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'EOF'
Usage: ./configure-minio.sh --user <username> --password <password> --data-path </absolute/path> --size <Gi> [options]

Options:
  -u, --user             MinIO access key / username (required)
  -p, --password         MinIO secret key / password (required)
  -d, --data-path        Absolute container path where the MinIO volume will be mounted (required)
  -s, --size             Requested PersistentVolumeClaim size in Gi (integer, required)
      --auth-user        Gateway AUTH_USERNAME value (requires all auth flags)
      --auth-password    Gateway AUTH_PASSWORD value (requires all auth flags)
      --auth-secret      Gateway AUTH_SECRET value (requires all auth flags)
  -h, --help             Show this help and exit

The script updates k8s/minio-statefulset.yaml and, when auth flags are provided,
k8s/gateway-service.yaml. Always commit or back up your changes before running.
EOF
}

MINIO_USER=""
MINIO_PASSWORD=""
MINIO_DATA_PATH=""
MINIO_SIZE_GI=""
AUTH_USER=""
AUTH_PASSWORD=""
AUTH_SECRET=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	-u|--user)
		MINIO_USER="${2:-}"
		shift 2
		;;
	-p|--password)
		MINIO_PASSWORD="${2:-}"
		shift 2
		;;
	-d|--data-path)
		MINIO_DATA_PATH="${2:-}"
		shift 2
		;;
	-s|--size)
		MINIO_SIZE_GI="${2:-}"
		shift 2
		;;
	--auth-user)
		AUTH_USER="${2:-}"
		shift 2
		;;
	--auth-password)
		AUTH_PASSWORD="${2:-}"
		shift 2
		;;
	--auth-secret)
		AUTH_SECRET="${2:-}"
		shift 2
		;;
	-h|--help)
		usage
		exit 0
		;;
	*)
		echo "Unknown option: $1" >&2
		usage
		exit 1
		;;
	esac
done

if [[ -z "$MINIO_USER" || -z "$MINIO_PASSWORD" || -z "$MINIO_DATA_PATH" || -z "$MINIO_SIZE_GI" ]]; then
	echo "Error: --user, --password, --data-path and --size are required." >&2
	usage
	exit 1
fi

if [[ "$MINIO_DATA_PATH" != /* ]]; then
	echo "Error: --data-path must be an absolute path inside the container (e.g. /data)." >&2
	exit 1
fi

if [[ "$MINIO_DATA_PATH" =~ [[:space:]] ]]; then
	echo "Error: --data-path cannot contain spaces." >&2
	exit 1
fi

if ! [[ "$MINIO_SIZE_GI" =~ ^[0-9]+$ ]]; then
	echo "Error: --size must be an integer (Gi)." >&2
	exit 1
fi

if [[ -n "$AUTH_USER" || -n "$AUTH_PASSWORD" || -n "$AUTH_SECRET" ]]; then
	if [[ -z "$AUTH_USER" || -z "$AUTH_PASSWORD" || -z "$AUTH_SECRET" ]]; then
		echo "Error: --auth-user, --auth-password, and --auth-secret must be provided together." >&2
		exit 1
	fi
fi

MANIFEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST_FILE="${MANIFEST_DIR}/minio-statefulset.yaml"
GATEWAY_MANIFEST_FILE="${MANIFEST_DIR}/gateway-service.yaml"

if [[ ! -f "$MANIFEST_FILE" ]]; then
	echo "Error: manifest not found at ${MANIFEST_FILE}" >&2
	exit 1
fi

if [[ -n "$AUTH_USER" ]] && [[ ! -f "$GATEWAY_MANIFEST_FILE" ]]; then
	echo "Error: gateway manifest not found at ${GATEWAY_MANIFEST_FILE}" >&2
	exit 1
fi

export MINIO_USER MINIO_PASSWORD MINIO_DATA_PATH MINIO_SIZE_GI MANIFEST_FILE
export AUTH_USER AUTH_PASSWORD AUTH_SECRET GATEWAY_MANIFEST_FILE

python3 <<'PY'
import os
import pathlib
import re
import sys

manifest_path = pathlib.Path(os.environ["MANIFEST_FILE"])
contents = manifest_path.read_text(encoding="utf-8")

user = os.environ["MINIO_USER"]
password = os.environ["MINIO_PASSWORD"]
data_path = os.environ["MINIO_DATA_PATH"]
size_gi = os.environ["MINIO_SIZE_GI"]
storage_value = f"{size_gi}Gi"

def escape(value: str) -> str:
    return value.replace("\\", "\\\\").replace('"', '\\"')

user_escaped = escape(user)
password_escaped = escape(password)

def apply(pattern, repl, label):
    global contents
    new_contents, count = re.subn(pattern, repl, contents, count=1, flags=re.MULTILINE)
    if count != 1:
        sys.exit(f"Failed to update {label}; is the manifest structure unchanged?")
    contents = new_contents

apply(
    r"(-\s+name:\s+MINIO_ACCESS_KEY\s*\n\s+value:\s+)" r'"[^"]+"' r"(\s+# managed by configure-minio\.sh)",
    rf'\g<1>"{user_escaped}"\g<2>',
    "MinIO access key",
)
apply(
    r"(-\s+name:\s+MINIO_SECRET_KEY\s*\n\s+value:\s+)" r'"[^"]+"' r"(\s+# managed by configure-minio\.sh)",
    rf'\g<1>"{password_escaped}"\g<2>',
    "MinIO secret key",
)
apply(
    r"(args:\s*\n\s*-\s*server\s*\n\s*-\s*)" r"[^\s]+" r"(\s+# managed by configure-minio\.sh)",
    rf"\g<1>{data_path}\g<2>",
    "MinIO data path (args)",
)
apply(
    r"(-\s+name:\s+minio-data\s*\n\s+mountPath:\s+)" r"[^\s]+" r"(\s+# managed by configure-minio\.sh)",
    rf"\g<1>{data_path}\g<2>",
    "MinIO volume mount path",
)
apply(
    r"(storage:\s+)" r"[^\s]+" r"(\s+# managed by configure-minio\.sh)",
    rf"\g<1>{storage_value}\g<2>",
    "PersistentVolumeClaim size",
)

manifest_path.write_text(contents, encoding="utf-8")

auth_user = os.environ.get("AUTH_USER")
auth_password = os.environ.get("AUTH_PASSWORD")
auth_secret = os.environ.get("AUTH_SECRET")
gateway_manifest = os.environ.get("GATEWAY_MANIFEST_FILE")

if auth_user and auth_password and auth_secret and gateway_manifest:
    gw_path = pathlib.Path(gateway_manifest)
    gw_contents = gw_path.read_text(encoding="utf-8")

    auth_user_esc = escape(auth_user)
    auth_password_esc = escape(auth_password)
    auth_secret_esc = escape(auth_secret)

    def apply_gateway(text, pattern, repl, label):
        new_contents, count = re.subn(pattern, repl, text, count=1, flags=re.MULTILINE)
        if count != 1:
            sys.exit(f"Failed to update {label}; is the gateway manifest structure unchanged?")
        return new_contents

    gw_contents = apply_gateway(
        gw_contents,
        r"(-\s+name:\s+AUTH_USERNAME\s*\n\s+value:\s+)" r'"[^"]+"' r"(\s+# managed by configure-minio\.sh)",
        rf'\g<1>"{auth_user_esc}"\g<2>',
        "AUTH_USERNAME",
    )
    gw_contents = apply_gateway(
        gw_contents,
        r"(-\s+name:\s+AUTH_PASSWORD\s*\n\s+value:\s+)" r'"[^"]+"' r"(\s+# managed by configure-minio\.sh)",
        rf'\g<1>"{auth_password_esc}"\g<2>',
        "AUTH_PASSWORD",
    )
    gw_contents = apply_gateway(
        gw_contents,
        r"(-\s+name:\s+AUTH_SECRET\s*\n\s+value:\s+)" r'"[^"]+"' r"(\s+# managed by configure-minio\.sh)",
        rf'\g<1>"{auth_secret_esc}"\g<2>',
        "AUTH_SECRET",
    )

    gw_path.write_text(gw_contents, encoding="utf-8")
PY

if [[ -n "$AUTH_USER" ]]; then
	echo "Updated ${MANIFEST_FILE} (MinIO) and ${GATEWAY_MANIFEST_FILE} (gateway auth)."
else
	echo "Updated ${MANIFEST_FILE} with user '${MINIO_USER}', custom data path '${MINIO_DATA_PATH}', and size ${MINIO_SIZE_GI}Gi."
fi
