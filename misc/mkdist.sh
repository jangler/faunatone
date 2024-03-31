#!/usr/bin/env bash

# puts together a release folder for the host OS.
# requires dos2unix.

set -euo pipefail
IFS=$'\t\n'

if [ "$(basename $(pwd))" == misc ]; then
	echo "error: run this script from the parent directory."
	exit 1
fi

case $(uname) in
	Darwin)
		echo "error: macOS build not implemented"
		exit 1
		;;
	Linux)
	    GOOS=linux
		go build -tags static -ldflags "-s -w" -o ftone ./faunatone/
		;;
	*)
		GOOS=windows
		CGO_LDFLAGS="-static-libgcc -static-libstdc++ -Wl,-Bstatic -lstdc++ -lpthread -Wl,-Bdynamic -mwindows" \
		 	go build -tags static -ldflags "-s -w -H windowsgui" -o ftone.exe ./faunatone/
		;;
esac

dir="dist/faunatone-$(git describe --tags)-${GOOS}64"
mkdir -p "$dir"
mv ftone* "$dir"
cp -r assets docs faunatone/config README.md "$dir"
cd "$dir"
for f in ftone*; do
	mv "$f" "${f/ftone/faunatone}"
done
for f in *.md docs/*.md; do
	mv "$f" "${f%.md}.txt"
done
if [ "$GOOS" == windows ]; then
	unix2dos *.txt docs/*.txt config/* config/keymaps/*
fi
cd -
