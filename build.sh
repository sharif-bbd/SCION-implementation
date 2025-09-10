#!/bin/bash
pkill -f verifier-app || true
pkill -f client-app || true
sleep 1

set -euo pipefail

BIN_URL_BASE="https://polybox.ethz.ch/index.php/s/ticSkCFAkgUbP9w/download?path=%2F&files="

# First we check which architecture is being used
arch=$(uname -m)
case "$arch" in
    x86_64|amd64)
        VERIFIER_FILE="verifier-app-amd64"
        GOARCH="amd64"
        ;;
    aarch64|arm64)
        VERIFIER_FILE="verifier-app-arm64"
        GOARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $arch"
        exit 1
        ;;
esac

DOWNLOAD_URL="${BIN_URL_BASE}${VERIFIER_FILE}"

# Now we check whether the verifier binaries are already downloaded.
# If they are not downloaded already, we download them.
if [[ ! -f "verifier-app" ]]; then
    echo "Downloading verifier-app for GOARCH=$GOARCH"
    if ! curl -fLsS "$DOWNLOAD_URL" -o "verifier-app"; then
        echo "Failed to download $VERIFIER_FILE"
        rm -f "verifier-app"
        exit 1
    fi

    if [[ ! -s "verifier-app" ]]; then
        echo "Downloaded file verifier-app is empty!"
        rm -f "verifier-app"
        exit 1
    fi

    chmod +x "verifier-app"
    echo "Successfully downloaded $VERIFIER_FILE"
fi

echo "Building client-app for GOARCH=$GOARCH"
CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH go build -o "client-app" project/main.go

echo "Build complete."