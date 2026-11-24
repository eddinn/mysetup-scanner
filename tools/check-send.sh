#!/usr/bin/env bash
# Checks sending end to end on this computer (docs/app.md, Sending), as a browser and MySetup would, without either: a development
# build of the app (go build -tags dev), pointed at a stand-in for MySetup on this computer (tests/fake_site.py), serves its page.
# Another site's send is refused and reaches no site; the site's refusal comes back in its own words; the next send carries exactly
# the report the app prints, and the page answers with the one-time link to open. Then the page's Done ends the app. On Linux, the
# terminal's question and the printed link are checked too. Used by .forgejo/workflows/app.yml and
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
# Saved first: grep -q ends at its first match, and curl writing into a closed pipe fails the check (exit 23, ci0, 2026-11-13).
curl -s -o "$work/page.html" "$url"
grep -q "Open 127.0.0.1:${site##*:} in a new tab" "$work/page.html"

grep -q 'id="send">Send to 127.0.0.1:' "$work/page.html"
link="$site/scan/claim/AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCdE"

send() { # $1 the Origin; prints the answer's status
    curl -s -o "$work/answer" -w '%{http_code}' -X POST -H "Origin: $1" -H 'Content-Type: application/json' -d '{}' "${url}send"
}
test "$(send https://attacker.example)" = 403
test ! -e "$work/sends"
test "$(send "$origin")" = 422
grep -q '"message":"Sending is busy right now. Try again in a few minutes, or copy the report and paste it in the box on your build page."' "$work/answer"
test ! -e "$work/received"
test "$(send "$origin")" = 200
grep -q "\"claim_url\":\"$link\"" "$work/answer"
cmp "$work/received" "$work/report.txt"
test "$(wc -l < "$work/sends")" -eq 2

# In a terminal (--cli) the app asks whether to send after the report: here in script(1)'s terminal, on Linux, where it is always
# there (util-linux), with the program built here. Only y sends: the site's refusal is said in its words and y tries again; then it
# sends the same report and prints the link.
if [ "$(uname -s)" = Linux ] && [ -z "${1:-}" ]; then
    rm -f "$work/sends" "$work/received"
    printf 'y\ny\n' |
        MYSETUP_SCAN_SITE=$site script -qec "'$app_file' --cli > '$work/cli-report.txt'" /dev/null | tr -d '\r' > "$work/cli.txt"
    grep -q 'Sending is busy right now.' "$work/cli.txt"
    grep -A1 '^Sent. Open this link in your web browser, on this computer or another, to add the report to your build:$' "$work/cli.txt" > "$work/sent.txt"
    grep -qx "  $link" "$work/sent.txt"
    cmp "$work/cli-report.txt" "$work/report.txt"
    cmp "$work/received" "$work/report.txt"
    test "$(wc -l < "$work/sends")" -eq 2
    echo "The terminal asked, sent the report and printed the link."
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
echo "Sending works end to end: refused from another site, the site's refusal in its words, the same report and the link."
