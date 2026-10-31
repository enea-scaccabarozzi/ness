#!/bin/sh
# Fake unit extractor: every stdin line "unit ID: BODY" is one unit. A "#fail", "#junk", "#dup"
# "#loose" or "#path" line makes it exit 1, print invalid JSON, repeat an id, print upper-case keys,
# or print its last argument.
in=$(cat)
case $in in
*'#fail'*) echo "boom" >&2; exit 1 ;;
*'#junk'*) echo "not json"; exit 0 ;;
*'#dup'*) printf '{"id":"a","body":"1"}\n{"id":"a","body":"2"}\n'; exit 0 ;;
*'#loose'*) printf '{"ID":"a","BODY":"1"}\n'; exit 0 ;;
*'#path'*) printf '{"id":"path","body":"%s"}\n' "$1"; exit 0 ;;
esac
printf '%s\n' "$in" | sed -n 's/^unit \([^:]*\): \(.*\)$/{"id":"\1","body":"\2"}/p'
