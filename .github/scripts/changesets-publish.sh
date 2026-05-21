#!/usr/bin/env bash
set -euo pipefail

version="$(node -p "require('./package.json').version")"

printf 'New tag: v%s\n' "${version}"
