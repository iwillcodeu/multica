#!/usr/bin/env bash
set -euo pipefail

# dotenv_export loads KEY=VALUE pairs from a .env-style file into the current
# shell environment without doing any `$VAR` expansion.
#
# Supports:
# - Blank lines and lines starting with `#`
# - Optional leading `export `
# - Values with or without quotes ("..." or '...')
#
# Intentionally does NOT support multiline values.
dotenv_export() {
  local env_file="${1:-}"
  if [[ -z "$env_file" || ! -f "$env_file" ]]; then
    echo "dotenv_export: missing env file: ${env_file}" >&2
    return 1
  fi

  local line key value
  while IFS= read -r line || [[ -n "$line" ]]; do
    # Trim leading whitespace
    line="${line#"${line%%[!$' \t']*}"}"
    [[ -z "$line" ]] && continue
    [[ "${line:0:1}" == "#" ]] && continue

    if [[ "$line" == export\ * ]]; then
      line="${line#export }"
      line="${line#"${line%%[!$' \t']*}"}"
    fi

    # Must look like KEY=...
    [[ "$line" == *"="* ]] || continue
    key="${line%%=*}"
    value="${line#*=}"

    # Trim key whitespace
    key="${key%"${key##*[!$' \t']}"}"
    key="${key#"${key%%[!$' \t']*}"}"

    # Only accept normal env var keys
    [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue

    # Trim leading whitespace from value (keep trailing as-is)
    value="${value#"${value%%[!$' \t']*}"}"

    # Strip simple surrounding quotes
    if [[ "$value" == \"*\" && "$value" == *\" ]]; then
      value="${value:1:${#value}-2}"
    elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
      value="${value:1:${#value}-2}"
    fi

    # Assign without eval (no expansion)
    printf -v "$key" "%s" "$value"
    export "$key"
  done <"$env_file"
}

