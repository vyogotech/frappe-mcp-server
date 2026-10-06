#!/bin/sh
# Regenerate THIRD_PARTY_NOTICES from the modules linked into the shipped binaries.
# With --check it regenerates into a temporary file and fails if the committed one differs.
set -eu

cd "$(dirname "$0")/.."
out=THIRD_PARTY_NOTICES
[ "${1:-}" = "--check" ] && out=$(mktemp)
modules=$(mktemp)
trap 'rm -f "$modules"' EXIT

go list -deps -f '{{with .Module}}{{.Path}} {{.Version}} {{.Dir}}{{end}}' . ./cmd/mcp-stdio |
	grep -v '^frappe-mcp-server' | grep . | sort -u >"$modules"

{
	cat <<'EOF'
Third-party notices for frappe-mcp-server

The released binaries (frappe-mcp-server and frappe-mcp-server-stdio) statically link the Go
modules below. Each licence is reproduced in full, which is what these licences require of a
binary redistribution: BSD-3-Clause clause 2, MIT's notice clause, and Apache-2.0 section 4(d)
for the modules that ship a NOTICE file.

frappe-mcp-server itself is under the MIT licence; see LICENSE.

Regenerate with `make notices` (scripts/third-party-notices.sh); `make notices-check` fails when
this file no longer matches the module graph.

EOF
	while read -r path version dir; do
		printf '\n================================================================================\n'
		printf '%s %s\n' "$path" "$version"
		for name in LICENSE LICENSE.txt LICENSE.md LICENCE COPYING NOTICE; do
			[ -f "$dir/$name" ] || continue
			printf '\n--- %s ---\n\n' "$name"
			cat "$dir/$name"
		done
	done <"$modules"
} >"$out"

# A module whose licence text we never found would leave a silent gap in the notices.
missing=$(while read -r path version dir; do
	found=
	for name in LICENSE LICENSE.txt LICENSE.md LICENCE COPYING NOTICE; do
		[ -f "$dir/$name" ] && found=1
	done
	[ -n "$found" ] || printf '%s %s\n' "$path" "$version"
done <"$modules")
if [ -n "$missing" ]; then
	echo "no licence file found for:" >&2
	echo "$missing" >&2
	exit 1
fi

if [ "${1:-}" = "--check" ]; then
	if ! diff -u THIRD_PARTY_NOTICES "$out"; then
		rm -f "$out"
		echo "THIRD_PARTY_NOTICES is out of date; run: make notices" >&2
		exit 1
	fi
	rm -f "$out"
fi
