#!/bin/sh
# A gate that moves the target under itself. usage: move_gate.sh REPO STATE MODE
# It counts its runs in STATE/count and appends NESS_BASE to STATE/bases. On its first run (every
# run for code-always) it commits onto main: a file under docs/sessions (record), a code file
# (code, code-always), a.txt (conflict, against the source's a.txt) or a rename of code.txt into
# docs/sessions (rename), and writes the new sha to STATE/moved. With main checked out in REPO it
# commits there; otherwise in a detached scratch worktree, then moves main with update-ref.
set -e
repo=$1 state=$2 mode=$3
n=$(( $(cat "$state/count" 2>/dev/null || echo 0) + 1 ))
echo "$n" > "$state/count"
echo "$NESS_BASE" >> "$state/bases"
[ "$n" = 1 ] || [ "$mode" = code-always ] || exit 0
dir=$repo
if [ "$(git -C "$repo" symbolic-ref -q HEAD || true)" != refs/heads/main ]; then
	dir=$state/wt
	git -C "$repo" worktree add -q --detach "$dir" main
fi
case $mode in
record) f=docs/sessions/x/note-$n.md ;;
code|code-always) f=code-$n.txt ;;
conflict) f=a.txt ;;
rename) f= ; mkdir -p "$dir/docs/sessions/x"; git -C "$dir" mv code.txt docs/sessions/x/code.txt ;;
*) echo "unknown mode $mode" >&2; exit 2 ;;
esac
if [ -n "$f" ]; then
	mkdir -p "$dir/$(dirname "$f")"
	echo "moved $n" > "$dir/$f"
	git -C "$dir" add "$f"
fi
git -C "$dir" -c user.email=t@x -c user.name=t -c commit.gpgsign=false commit -q -m "move $n"
git -C "$dir" rev-parse HEAD > "$state/moved"
if [ "$dir" != "$repo" ]; then
	git -C "$repo" update-ref refs/heads/main "$(cat "$state/moved")"
	git -C "$repo" worktree remove --force "$dir"
fi
