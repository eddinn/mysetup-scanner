#!/usr/bin/env bash
# Builds the app's downloads into the directory given (docs/app.md, Releases): every target twice, refusing it when the bytes
# differ, then Windows as the program itself and Linux as a .tar.gz whose program is already executable, so it can be
# double-clicked once extracted (a browser download never keeps that permission). The archives are reproducible too: the
# same commit gives the same bytes: the archives' times are the commit's, or SOURCE_DATE_EPOCH's where it is set (needed to
# rebuild from a source archive, which has no Git history). SHA256SUMS covers what is published. Used by
# .forgejo/workflows/app.yml and .github/workflows/app-release.yml alike. Needs Go (the version in go.mod), bash, GNU tar and gzip.
#   tools/build-release.sh <out-dir>
set -euo pipefail
out=${1:?the output directory}
export CGO_ENABLED=0 GOTOOLCHAIN=${GOTOOLCHAIN:-local} GOFLAGS=${GOFLAGS:--mod=readonly}
root=$(cd "$(dirname "$0")/.." && pwd)
version=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$root/internal/scan/hardware.go")
pre=$(sed -n 's/^const Prerelease = "\(.*\)"$/\1/p' "$root/internal/scan/hardware.go")
full=$version${pre:+-$pre}
epoch=${SOURCE_DATE_EPOCH:-$(git -C "$root" log -1 --format=%ct)}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$out" "$work/a" "$work/b"

build() { # $1 GOOS, $2 GOARCH, $3 output file
    (cd "$root" && GOOS=$1 GOARCH=$2 go build -buildvcs=false -trimpath -ldflags='-s -w -buildid=' -o "$3" ./cmd/mysetup-scan)
}

package_linux() { # $1 directory with the program, $2 label, $3 archive
    local name="mysetup-scan-linux-$2" stage="$1/stage"
    mkdir -p "$stage/$name"
    cp "$1/mysetup-scan" "$stage/$name/mysetup-scan"
    cat > "$stage/$name/README.txt" <<EOF
MySetup scan app $full for Linux ($2)

1. Double-click mysetup-scan. If a box asks whether to run it, click Run (some file managers call it Execute).
   Your web browser opens a page with the report about this computer.
2. Follow the steps on that page to add the report to your build on mysetup.is.

If double-clicking does nothing, your file manager does not start programs. Follow the steps
"Linux: in a terminal" instead: https://github.com/eddinn/mysetup-scanner#the-app-step-by-step

It only reads hardware names: it sends nothing anywhere, writes no files and installs nothing.
The source, the checksums and how to check this download: https://github.com/eddinn/mysetup-scanner
EOF
    chmod 0755 "$stage/$name" "$stage/$name/mysetup-scan"
    chmod 0644 "$stage/$name/README.txt"
    tar --sort=name --format=gnu --owner=0 --group=0 --numeric-owner --mtime="@$epoch" -C "$stage" -cf - "$name" | gzip -n -9 > "$3"
}

for target in windows/amd64/x86-64 windows/arm64/arm64 linux/amd64/x86-64 linux/arm64/arm64; do
    os=${target%%/*}; rest=${target#*/}; arch=${rest%%/*}; label=${rest#*/}
    if [ "$os" = windows ]; then
        name="mysetup-scan-windows-$label.exe"
        build "$os" "$arch" "$work/a/$name"
        build "$os" "$arch" "$work/b/$name"
        cmp "$work/a/$name" "$work/b/$name"
        cp "$work/a/$name" "$out/$name"
    else
        for side in a b; do
            mkdir -p "$work/$side/$label"
            build "$os" "$arch" "$work/$side/$label/mysetup-scan"
            package_linux "$work/$side/$label" "$label" "$work/$side/mysetup-scan-linux-$label.tar.gz"
        done
        cmp "$work/a/$label/mysetup-scan" "$work/b/$label/mysetup-scan"
        cmp "$work/a/mysetup-scan-linux-$label.tar.gz" "$work/b/mysetup-scan-linux-$label.tar.gz"
        cp "$work/a/mysetup-scan-linux-$label.tar.gz" "$out/"
    fi
done
(cd "$out" && sha256sum mysetup-scan-* > SHA256SUMS && cat SHA256SUMS)
