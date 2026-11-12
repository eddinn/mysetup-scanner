#!/usr/bin/env bash
# Checks sending end to end on this computer (docs/app.md, Sending), as a browser and MySetup would, without either: a development
# build of the app (go build -tags dev), pointed at a stand-in for MySetup on this computer (tests/fake_site.py), serves its page.
# Another site's send is refused and reaches no site; a wrong code reaches the site and comes back in the site's own words; the
# right code sends exactly the report the app prints, and the page answers with the review's address. Then the page's Done ends
# the app. On Linux, the terminal's question for the code is checked too. Used by .forgejo/workflows/app.yml and
# .github/workflows/app.yml. Needs Go, Python 3 and curl. Given a program, it checks the page's sending with that development build
# instead of building one (from WSL, a Windows one: WSLENV=MYSETUP_SCAN_SITE passes the site on).
#   tools/check-send.sh [program]
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
pids=()
cleanup() {
    for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
    rm -rf "$work"
}
trap cleanup EXIT
# On Windows, python3 may be only the store's stand-in, which runs nothing.
python=python3
"$python" -c '' 2>/dev/null || python=python

app_file=$work/scan-app.exe
if [ -n "${1:-}" ]; then
    app_file=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
else
    (cd "$root" && go build -tags dev -trimpath -o "$app_file" ./cmd/mysetup-scan)
fi
"$python" -B "$root/tests/fake_site.py" "$work" &
pids+=("$!")
for _ in $(seq 50); do
    [ -s "$work/port" ] && break
    sleep 0.2
done
site="http://127.0.0.1:$(cat "$work/port")"
"$app_file" --cli < /dev/null > "$work/report.txt" 2> /dev/null

MYSETUP_SCAN_SITE=$site "$app_file" --no-browser 2> "$work/page.txt" &
app=$!
pids+=("$app")
url=
for _ in $(seq 50); do
    url=$(grep -o 'http://127\.0\.0\.1:[0-9]*/t/[0-9a-f]\{64\}/' "$work/page.txt" | head -n 1 || true)
    [ -n "$url" ] && break
    sleep 0.2
done
test -n "$url"
origin=${url%%/t/*}
grep -q "This is a development build, for tests only." "$work/page.txt"
curl -s "$url" | grep -q "Open 127.0.0.1:${site##*:} in a new tab"

send() { # $1 the Origin, $2 the code; prints the answer's status
    curl -s -o "$work/answer" -w '%{http_code}' -X POST -H "Origin: $1" -H 'Content-Type: application/json' -d "{\"code\": \"$2\"}" "${url}send"
}
test "$(send https://attacker.example K7QD-M2XP)" = 403
test ! -e "$work/codes"
test "$(send "$origin" AAAA-AAAA)" = 422
grep -q '"message":"This code is wrong, was already used or has expired. Get a new code on your build page and type it again."' "$work/answer"
test ! -e "$work/received"
test "$(send "$origin" K7QD-M2XP)" = 200
grep -q "\"review_url\":\"$site/setups/01M7/components/scan/01M8\"" "$work/answer"
cmp "$work/received" "$work/report.txt"
printf '%s\n' AAAA-AAAA K7QD-M2XP | cmp - "$work/codes"

# In a terminal (--cli) the app asks for the code after the report: here in script(1)'s terminal, on Linux, where it is always
# there (util-linux), with the program built here. A wrong code is said in the site's words and the next is taken; the right one
# sends the same report.
if [ "$(uname -s)" = Linux ] && [ -z "${1:-}" ]; then
    rm -f "$work/codes" "$work/received"
    printf 'AAAA-AAAA\nK7QD-M2XP\n' |
        MYSETUP_SCAN_SITE=$site script -qec "'$app_file' --cli > '$work/cli-report.txt'" /dev/null | tr -d '\r' > "$work/cli.txt"
    grep -q '^This code is wrong, was already used or has expired.' "$work/cli.txt"
    grep -A1 '^Sent. Open this address in your web browser to review it:$' "$work/cli.txt" |
        grep -qx "  $site/setups/01M7/components/scan/01M8"
    cmp "$work/cli-report.txt" "$work/report.txt"
    cmp "$work/received" "$work/report.txt"
    printf '%s\n' AAAA-AAAA K7QD-M2XP | cmp - "$work/codes"
    echo "The terminal asked for the code and sent the report."
fi

test "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H "Origin: $origin" "${url}done")" = 204
for _ in $(seq 25); do
    kill -0 "$app" 2>/dev/null || break
    sleep 0.2
done
if kill -0 "$app" 2>/dev/null; then
    echo "The app did not end after Done."
    exit 1
fi
wait "$app"
echo "Sending works end to end: refused from another site, the site's words for a wrong code, the same report for the right one."
