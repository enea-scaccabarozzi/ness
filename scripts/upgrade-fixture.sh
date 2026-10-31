#!/bin/sh
# Builds the upgrade fixture of a release (R-UPD-094), run from the repository root:
#
#   sh scripts/upgrade-fixture.sh vX.Y.Z
#
# It builds ness at the local tag TAG, runs it in a scratch repository under mktemp -d (HOME
# and XDG_* there too, so nothing of this machine reaches the fixture) and writes
# internal/cli/testdata/upgrade/TAG/tree.txtar (the tracked tree after init) and state.sql (the
# store with one open program root, s-fixture). No network: GOPROXY=off, and the tag is local.
set -eu

[ $# -eq 1 ] || {
	echo "usage: upgrade-fixture.sh vX.Y.Z" >&2
	exit 1
}
tag=$1
out=$(pwd)/internal/cli/testdata/upgrade/$tag
git rev-parse -q --verify "refs/tags/$tag" >/dev/null || {
	echo "no local tag $tag" >&2
	exit 1
}

top=$(git rev-parse --show-toplevel)
tmp=$(mktemp -d)
src=$tmp/src
cleanup() {
	git -C "$top" worktree remove --force "$src" 2>/dev/null || true
	rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

git worktree add -q --detach "$src" "$tag"
GOPROXY=off GOFLAGS=-mod=mod go build -C "$src" -o "$tmp/ness" \
	-ldflags "-X main.version=${tag#v}" ./cmd/ness

# The scratch repository: named fixture, so the project name is not the temp dir's.
repo=$tmp/fixture
mkdir -p "$tmp/home" "$repo"
# shellcheck disable=SC2046 # one word per inherited GIT_*/NESS_* variable
unset $(env | sed -n -E 's/^((GIT|NESS)_[A-Za-z0-9_]*)=.*/\1/p') 2>/dev/null || true
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/home/.config" XDG_CACHE_HOME="$tmp/home/.cache"
export GIT_CONFIG_NOSYSTEM=1
git config --global user.name "ness fixture"
git config --global user.email fixture@example.invalid
git config --global init.defaultBranch main
git config --global commit.gpgsign false

cd "$repo"
git init -q
git commit -q --allow-empty -m init
"$tmp/ness" init --gate true --base main --profile program --yes >/dev/null
git add -A
git commit -q -m "chore: ness init"
"$tmp/ness" inbox >/dev/null

mkdir -p "$out"
python3 -I - .git/ness/state.sqlite3 "$out/state.sql" <<'EOF'
import sqlite3, sys
db = sqlite3.connect(sys.argv[1])
at = "2026-10-31T00:00:00Z"
db.execute(
    "INSERT INTO sessions (id, scope, created_at, phase_at, title, ready_at, first_ready_at, base, size)"
    " VALUES ('s-fixture', 'fixture', ?, ?, 'fixture', ?, ?, 'main', 'program')",
    (at, at, at, at))
db.commit()
version = db.execute("PRAGMA user_version").fetchone()[0]
with open(sys.argv[2], "w") as f:
    for line in db.iterdump():
        f.write(line + "\n")
    f.write("PRAGMA user_version = %d;\n" % version)  # iterdump leaves it out
EOF

# txtar of the tracked tree. A txtar holds only files, so each symlink is a "symlink PATH
# TARGET" line of the comment, and an executable file is refused rather than packed as plain.
python3 -I - "$out/tree.txtar" <<'EOF'
import os, subprocess, sys
entries = subprocess.run(["git", "ls-files", "-s", "-z"], check=True, capture_output=True).stdout.split(b"\0")
links, files = [], []
for e in filter(None, entries):
    meta, name = e.decode().split("\t", 1)
    mode = meta.split()[0]
    if mode == "120000":
        links.append("symlink %s %s\n" % (name, os.readlink(name)))
    elif mode == "100644":
        body = open(name).read()
        if body and not body.endswith("\n"):
            sys.exit("%s has no final newline; txtar would add one" % name)
        files.append("-- %s --\n%s" % (name, body))
    else:
        sys.exit("%s has mode %s; only plain files and symlinks are packed" % (name, mode))
with open(sys.argv[1], "w") as f:
    f.write("".join(links) + "".join(files))
EOF
echo "wrote $out"
