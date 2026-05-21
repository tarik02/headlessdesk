#!/usr/bin/env bash
set -euo pipefail

version="$(jq -r '.version' package.json)"

printf 'New tag: v%s\n' "${version}"
