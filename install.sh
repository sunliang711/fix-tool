#!/bin/sh
set -e

REPO="sunliang711/fix-tool"
APP="fix-tool"
INSTALL_DIR="/usr/local/bin"
VERSION=""

usage() {
    cat <<'EOF'
Install fix-tool CLI from GitHub releases.

Usage:
  install.sh [options]

Options:
  -v, --version VERSION   Version to install (e.g. v0.1.0), default: latest
  -d, --dir DIR           Install directory, default: /usr/local/bin
  -h, --help              Show this help

Examples:
  ./install.sh
  ./install.sh -v v0.1.0
  ./install.sh -d ~/.local/bin
  curl -fsSL https://raw.githubusercontent.com/sunliang711/fix-tool/main/install.sh -o install.sh
  sh install.sh -v v0.1.0
EOF
}

log() {
    printf '%s\n' "$*"
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

while [ $# -gt 0 ]; do
    case "$1" in
        -v|--version)
            [ $# -ge 2 ] || die "missing value for $1"
            VERSION="$2"
            shift 2
            ;;
        -d|--dir)
            [ $# -ge 2 ] || die "missing value for $1"
            INSTALL_DIR="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            usage
            die "unknown option: $1"
            ;;
    esac
done

need_cmd uname
need_cmd tr
need_cmd curl
need_cmd grep
need_cmd awk
need_cmd sed
need_cmd tar
need_cmd mktemp
need_cmd install

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux|darwin) ;;
    *) die "unsupported OS: $OS" ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) die "unsupported architecture: $ARCH" ;;
esac

if [ -z "$VERSION" ]; then
    log "Fetching latest version..."
    TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" \
        | grep -o '"tag_name": *"\(cli/\)\{0,1\}v[^"]*"' \
        | head -1 \
        | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    [ -n "$TAG" ] || die "failed to determine latest version"
    VERSION=${TAG#cli/}
else
    case "$VERSION" in
        v*) ;;
        *) VERSION="v${VERSION}" ;;
    esac
    TAG="$VERSION"
fi

PACKAGE="${APP}_${VERSION}_${OS}_${ARCH}"
ARCHIVE="${PACKAGE}.tar.gz"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

download_release_file() {
    file_name="$1"
    output_path="$2"
    primary_url="https://github.com/${REPO}/releases/download/${TAG}/${file_name}"

    if curl -fSL --progress-bar -o "$output_path" "$primary_url"; then
        return 0
    fi

    case "$TAG" in
        cli/*) fallback_tag=${TAG#cli/} ;;
        *)     fallback_tag="cli/${TAG}" ;;
    esac
    fallback_url="https://github.com/${REPO}/releases/download/${fallback_tag}/${file_name}"
    curl -fSL --progress-bar -o "$output_path" "$fallback_url"
}

log "Downloading ${ARCHIVE}..."
if ! download_release_file "$ARCHIVE" "${TMPDIR}/${ARCHIVE}"; then
    die "download failed. Check that version ${VERSION} exists for ${OS}-${ARCH}."
fi

log "Verifying checksum..."
if download_release_file "checksums.txt" "${TMPDIR}/checksums.txt"; then
    EXPECTED=$(grep " ${ARCHIVE}$" "${TMPDIR}/checksums.txt" | awk '{print $1}')
    if [ -z "$EXPECTED" ]; then
        log "Warning: no checksum found for ${ARCHIVE}, skipping verification"
    else
        if command -v sha256sum >/dev/null 2>&1; then
            ACTUAL=$(sha256sum "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')
        elif command -v shasum >/dev/null 2>&1; then
            ACTUAL=$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')
        else
            die "required command not found: sha256sum or shasum"
        fi
        if [ "$EXPECTED" != "$ACTUAL" ]; then
            printf 'Error: checksum mismatch\n  expected: %s\n  actual:   %s\n' "$EXPECTED" "$ACTUAL" >&2
            exit 1
        fi
        log "Checksum verified."
    fi
else
    log "Warning: checksums.txt not found, skipping verification"
fi

tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
[ -f "${TMPDIR}/${PACKAGE}/${APP}" ] || die "archive does not contain ${APP}"

SUDO=""
if [ ! -d "$INSTALL_DIR" ]; then
    if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
        command -v sudo >/dev/null 2>&1 || die "no write permission to ${INSTALL_DIR} and sudo not available"
        SUDO="sudo"
    fi
elif [ ! -w "$INSTALL_DIR" ]; then
    command -v sudo >/dev/null 2>&1 || die "no write permission to ${INSTALL_DIR} and sudo not available"
    SUDO="sudo"
fi

$SUDO mkdir -p "$INSTALL_DIR"
$SUDO install -m 755 "${TMPDIR}/${PACKAGE}/${APP}" "${INSTALL_DIR}/${APP}"

log "Installed ${APP} ${VERSION} to ${INSTALL_DIR}/${APP}"
cat <<EOF

Next steps:
  ${APP} --help
  ${APP} docs
  ${APP} config example --output config-example.toml
EOF
