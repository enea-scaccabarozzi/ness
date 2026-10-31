#!/bin/sh
# Release gates, run from the repository root (R-UPD-091, R-UPD-092).
#
#   release-check.sh rc VERSION      prints the next tag, vVERSION-rc.N
#   release-check.sh stable VERSION  exits 1 unless VERSION may be released as stable
set -eu

die() {
	echo "$*" >&2
	exit 1
}

[ $# -eq 2 ] || die "usage: release-check.sh rc|stable VERSION"
mode=$1 ver=$2
rcs=$(git tag --list "v$ver-rc.*")

case "$mode" in
rc)
	git rev-parse -q --verify "refs/tags/v$ver" >/dev/null && die "v$ver is already released: run the rc workflow on release-please's pull request"
	awk -v h="## [$ver]" 'index($0, h) == 1 { found = 1 } END { exit !found }' CHANGELOG.md 2>/dev/null ||
		die "CHANGELOG.md has no ## [$ver] section: run the rc workflow on release-please's pull request"
	n=$(printf '%s' "$rcs" | grep -c . || true)
	echo "v$ver-rc.$((n + 1))"
	;;
stable)
	[ -n "$rcs" ] || die "no v$ver-rc.* tag: cut a release candidate with the rc workflow and run its field window first"
	head=$(git rev-parse 'HEAD^{tree}')
	match=
	for t in $rcs; do
		[ "$(git rev-parse "$t^{tree}")" = "$head" ] && match=$t
	done
	[ -n "$match" ] || die "no v$ver-rc.* tag has HEAD's tree: cut a new rc"
	if prev=$(git describe --tags --abbrev=0 --exclude '*-rc.*' 2>/dev/null); then
		[ -d "internal/cli/testdata/upgrade/$prev" ] || die "no upgrade fixture for $prev: run scripts/upgrade-fixture.sh $prev"
		since=$prev
	else
		since=$(git rev-list --max-parents=0 HEAD)
	fi
	# %B: a squash lists the folded subjects as nested commits in its body (D44). docs(sessions):
	# record commits do not match.
	git log --format=%B "$since..HEAD" | grep -Eq '^sessions(\(|!|:)' ||
		die "no sessions: commit since $since; add one (e.g. 'sessions: nothing to do') before releasing"
	;;
*) die "usage: release-check.sh rc|stable VERSION" ;;
esac
