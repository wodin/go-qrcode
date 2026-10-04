#!/bin/sh
# Builds dist/qrcode-logo.zip, the skill as chat apps take it for upload.
#
# The Linux binaries are built into the skill's bin/ first. A skill run where
# there is network access downloads the release instead and needs none of
# them; they are for the sandboxes that have none, which are all Linux.
set -eu

cd "$(dirname "$0")/.."

skill=skills/qrcode-logo
archive=dist/qrcode-logo.zip

for arch in amd64 arm64; do
	GOOS=linux GOARCH=$arch CGO_ENABLED=0 \
		go build -ldflags="-s -w" -trimpath \
		-o "$skill/bin/qrcode-linux-$arch" ./qrcode
done

mkdir -p dist

# zip adds to an archive that is already there, and would keep whatever the
# skill no longer has.
if [ -e "$archive" ]; then
	rm "$archive"
fi

(cd skills && zip -q -r -X "../$archive" qrcode-logo -x '*/__pycache__/*' '*.DS_Store')

echo "$archive"
