#!/bin/sh
# Install ness from its GitHub releases (R-UPD-093).
#
#   curl -fsSL https://raw.githubusercontent.com/enea-scaccabarozzi/ness/main/install.sh | sh
#   sh install.sh [--channel stable|rc]
#
# Environment: NESS_VERSION pins a version (X.Y.Z or vX.Y.Z); NESS_INSTALL_DIR is the target
# directory (default ~/.local/bin); NESS_RELEASES_URL is the releases API base. Needs curl or
# wget, tar, and sha256sum or shasum.
set -eu

# Everything runs from main, called on the last line, so a truncated download runs nothing.
main() {
	die() {
		echo "install.sh: $*" >&2
		exit 1
	}

	channel=stable
	while [ $# -gt 0 ]; do
		case "$1" in
		--channel)
			[ $# -ge 2 ] || die "--channel needs stable or rc"
			channel=$2
			shift 2
			;;
		*) die "unknown argument $1; usage: install.sh [--channel stable|rc]" ;;
		esac
	done
	case "$channel" in stable | rc) ;; *) die "--channel is stable or rc, not $channel" ;; esac

	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) arch=$(uname -m) ;;
	esac
	nobuild="no ness build for $os/$arch; build from source: go install github.com/enea-scaccabarozzi/ness/cmd/ness@latest"
	case "$os/$arch" in linux/amd64 | linux/arm64) ;; *) die "$nobuild" ;; esac

	# fetch URL FILE: curl, or wget when curl is missing or cannot start.
	fetch() {
		if command -v curl >/dev/null 2>&1; then
			rc=0
			curl -fsSL -o "$2" "$1" || rc=$?
			[ "$rc" -ne 126 ] && [ "$rc" -ne 127 ] && return "$rc"
		fi
		command -v wget >/dev/null 2>&1 || die "neither curl nor wget can run"
		wget -qO "$2" "$1"
	}

	command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 || die "needs sha256sum or shasum"
	dir=
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp" ${dir:+"$dir/.ness.install"}' EXIT
	trap 'exit 1' HUP INT TERM

	base=${NESS_RELEASES_URL:-https://api.github.com/repos/enea-scaccabarozzi/ness}
	fetch "$base/releases?per_page=100" "$tmp/releases.json" || die "cannot list the releases at $base"

	# One release per line, newest first: each line starts at its "tag_name".
	tr -d '\n' <"$tmp/releases.json" | awk '{ gsub(/"tag_name"/, "\n&"); print }' | grep '^"tag_name"' >"$tmp/releases" || true
	tag_of() { sed 's/^"tag_name": *"\([^"]*\)".*/\1/'; }

	if [ -n "${NESS_VERSION:-}" ]; then
		tag=v${NESS_VERSION#v}
		release=$(grep "^\"tag_name\": *\"$tag\"" "$tmp/releases" | head -n 1) || true
		[ -n "$release" ] || die "no release $tag at $base"
	elif [ "$channel" = rc ]; then
		release=$(head -n 1 "$tmp/releases")
	else
		release=$(grep '"prerelease": *false' "$tmp/releases" | head -n 1) || true
	fi
	[ -n "$release" ] || die "no $channel release at $base"
	tag=$(printf '%s\n' "$release" | tag_of)
	version=${tag#v}

	name="ness_${version}_${os}_${arch}.tar.gz"
	url_of() {
		printf '%s\n' "$release" | grep -o '"browser_download_url": *"[^"]*"' | sed 's/.*"\([^"]*\)"$/\1/' |
			grep "/$1\$" | head -n 1 || true
	}
	archive_url=$(url_of "$name")
	[ -n "$archive_url" ] || die "$nobuild"
	sums_url=$(url_of checksums.txt)
	[ -n "$sums_url" ] || die "$tag has no checksums.txt"

	fetch "$archive_url" "$tmp/$name" || die "cannot download $archive_url"
	fetch "$sums_url" "$tmp/checksums.txt" || die "cannot download $sums_url"
	grep "  $name\$" "$tmp/checksums.txt" >"$tmp/sum" || die "checksums.txt lists no $name"
	if command -v sha256sum >/dev/null 2>&1; then
		(cd "$tmp" && sha256sum -c sum) >/dev/null 2>&1 || die "checksum mismatch for $name"
	else
		(cd "$tmp" && shasum -a 256 -c sum) >/dev/null 2>&1 || die "checksum mismatch for $name"
	fi

	tar -xzf "$tmp/$name" -C "$tmp" ness
	dir=${NESS_INSTALL_DIR:-$HOME/.local/bin}
	mkdir -p "$dir"
	# Copy then rename, so a running ness is replaced, not overwritten in place.
	cp "$tmp/ness" "$dir/.ness.install"
	chmod 0755 "$dir/.ness.install"
	mv -f "$dir/.ness.install" "$dir/ness"

	echo "ness $version installed to $dir/ness"
	case ":$PATH:" in
	*":$dir:"*) ;;
	*) echo "note: $dir is not on your PATH; add it: export PATH=\"$dir:\$PATH\"" ;;
	esac
}

main "$@"
