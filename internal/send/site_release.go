//go:build !dev

package send

// Site is the MySetup a release build sends to and leads the member to: MySetup itself, fixed, so nobody can be talked into
// sending a report elsewhere (MySetup's docs/scanner/design.md, Engineering choices). tools/build-release.sh refuses a release
// that names any other site.
func Site() string { return "https://mysetup.is" }

// Development says whether this is a development build, which may send to staging.
const Development = false
