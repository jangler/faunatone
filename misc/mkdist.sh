#!/usr/bin/env bash

# puts together a release folder for a target GOOS.
# requires dos2unix and either util-linux rename or perl rename.
# cross-compilation not currently supported.

set -euo pipefail

if [ "$(basename $(pwd))" == misc ]; then
	echo "error: run this script from the parent directory."
	exit 1
fi

case "$GOOS" in
	linux)
		go build -tags static -ldflags "-s -w" -o ftone ./faunatone/
		;;
	windows)
		 CGO_LDFLAGS="-static-libgcc -static-libstdc++ -Wl,-Bstatic -lstdc++ -lpthread -Wl,-Bdynamic -mwindows" \
		 	go build -tags static -ldflags "-s -w -H windowsgui" -o ftone.exe ./faunatone/
		;;
	"")
		echo "error: GOOS not set"
		exit 1
		;;
	*)
		echo "error: unsupported GOOS: $GOOS"
		exit 1
		;;
esac

dir="dist/faunatone-$(git describe --tags)-${GOOS}64"
mkdir -p "$dir"
mv ftone* "$dir"
cp -r assets docs faunatone/config README.md "$dir"
cd "$dir"
if [[ $(rename --version) == *util-linux* ]]; then
	rename ftone faunatone ftone*
	rename .md .txt *.md docs/*.md
else
	rename s/ftone/faunatone/ ftone*
	rename s/.md/.txt/ *.md docs/*.md
fi
if [ "$GOOS" == windows ]; then
	unix2dos *.txt docs/*.txt config/* config/keymaps/*
fi
cd -
