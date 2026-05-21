#!/usr/bin/env bash
set -euo pipefail

version="${1#v}"

notes="$(
  awk -v version="${version}" '
    $0 == "## " version {
      found = 1
      print
      next
    }
    found && /^## / {
      exit
    }
    found {
      print
    }
  ' CHANGELOG.md
)"

if [ -n "${notes}" ]; then
  printf '%s\n' "${notes}"
else
  printf 'Release v%s\n' "${version}"
fi
