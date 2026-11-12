#!/usr/bin/env bash
# Checks a Linux download as a member gets it (docs/app.md, Releases), on a computer with its processor: the .tar.gz holds one
# folder with README.txt and the program, executable and owned by no one in particular; extracted, the program says the app's
# version and gives a valid report. Used by .forgejo/workflows/app.yml (x86-64) and .github/workflows/app.yml (arm64).
#   tools/check-linux-download.sh <dir with the downloads> <x86-64 | arm64>
set -euo pipefail
dist=$(cd "${1:?the directory with the downloads}" && pwd)
label=${2:?x86-64 or arm64}
root=$(cd "$(dirname "$0")/.." && pwd)
version=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$root/internal/scan/hardware.go")
pre=$(sed -n 's/^const Prerelease = "\(.*\)"$/\1/p' "$root/internal/scan/hardware.go")
name=mysetup-scan-linux-$label
archive=$dist/$name.tar.gz
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"

tar --numeric-owner -tvzf "$archive" | awk '{print $1, $2, $6}' | LC_ALL=C sort > listing.txt
cat listing.txt
printf '%s\n' "drwxr-xr-x 0/0 $name/" "-rw-r--r-- 0/0 $name/README.txt" "-rwxr-xr-x 0/0 $name/mysetup-scan" | LC_ALL=C sort |
    diff - listing.txt
tar -xzf "$archive"
test -x "$name/mysetup-scan"
"$name/mysetup-scan" --version | tee version.txt
grep -qx "mysetup-scan $version${pre:+-$pre}" version.txt
"$name/mysetup-scan" --cli < /dev/null > report.txt
python3 -B "$root/tests/validate_report.py" report.txt
echo "$name.tar.gz works as extracted."
