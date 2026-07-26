#!/usr/bin/env bash
#
# Differential regression + latency harness for the read API.
#
# The contract for a performance change is "faster, identical results". This
# script proves the second half: it records normalized responses BEFORE a change
# and diffs them against AFTER. A non-empty diff means behavior changed — stop and
# explain it, don't wave it through.
#
# Usage:
#   verify.sh record  <base_url> [out_dir]     # snapshot the baseline (run BEFORE the change)
#   verify.sh diff    <base_url> [out_dir]     # replay + diff vs baseline (run AFTER)
#   verify.sh bench   <base_url> [n]           # p50/p95 latency per path (needs `hey`, optional)
#
# base_url e.g. https://api.starrank.dev  or  http://localhost:3000
# out_dir defaults to ./baseline (next to this script).
#
# Response normalization strips volatile fields (fetchedAt/createdAt/updatedAt)
# recursively so only meaningful changes — ordering, membership, counts, values —
# surface in the diff. Everything else (item order included) is compared verbatim,
# because order IS the thing a sort/index change can break.

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# PERF_PATHS overrides the path list, e.g. to focus a run on the endpoints a
# change actually touches. Defaults to the full paths.txt next to this script.
paths_file="${PERF_PATHS:-$here/paths.txt}"

die() { echo "error: $*" >&2; exit 1; }
command -v curl >/dev/null || die "curl not found"
command -v jq   >/dev/null || die "jq not found"

# jq program: drop volatile timestamps at any depth, sort object keys for a stable
# textual diff. Array order is preserved on purpose.
normalize='def scrub: walk(if type == "object" then del(.fetchedAt, .createdAt, .updatedAt) else . end); scrub'

slug() { echo "$1" | sed 's#[^A-Za-z0-9]#_#g' | cut -c1-120; }

fetch_all() {
  local base="$1" dir="$2"
  mkdir -p "$dir"
  local n=0
  while IFS= read -r path; do
    [[ -z "$path" || "$path" == \#* ]] && continue
    local out="$dir/$(slug "$path").json"
    if ! curl -fsS --max-time 30 "$base$path" \
         | jq -S "$normalize" > "$out" 2>/dev/null; then
      echo "  ! FAILED  $path" >&2
      echo '{"__error__":"request or json parse failed"}' > "$out"
    fi
    n=$((n+1))
  done < "$paths_file"
  echo "$n"
}

cmd="${1:-}"; shift || true
case "$cmd" in
  record)
    base="${1:?base_url required}"; dir="${2:-$here/baseline}"
    echo "recording baseline from $base -> $dir"
    n=$(fetch_all "$base" "$dir")
    echo "recorded $n responses. Keep $dir; run '$0 diff <new_url>' after the change."
    ;;

  diff)
    base="${1:?base_url required}"; dir="${2:-$here/baseline}"
    [[ -d "$dir" ]] || die "no baseline at $dir; run 'record' first"
    tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
    echo "replaying against $base and diffing vs $dir"
    fetch_all "$base" "$tmp" >/dev/null
    changed=0
    while IFS= read -r path; do
      [[ -z "$path" || "$path" == \#* ]] && continue
      f="$(slug "$path").json"
      if [[ ! -f "$dir/$f" ]]; then echo "  + NEW    $path (no baseline)"; continue; fi
      if ! diff -q "$dir/$f" "$tmp/$f" >/dev/null; then
        changed=$((changed+1))
        echo "  ~ CHANGED $path"
        diff -u "$dir/$f" "$tmp/$f" | sed -n '1,40p' | sed 's/^/      /'
      else
        echo "  = same    $path"
      fi
    done < "$paths_file"
    echo
    if [[ "$changed" -eq 0 ]]; then
      echo "PASS: all responses identical — behavior preserved."
    else
      echo "FAIL: $changed endpoint(s) changed. Review each; only ship if intended."
      exit 1
    fi
    ;;

  bench)
    base="${1:?base_url required}"; n="${2:-300}"
    command -v hey >/dev/null || die "bench needs 'hey' (go install github.com/rakyll/hey@latest)"
    while IFS= read -r path; do
      [[ -z "$path" || "$path" == \#* ]] && continue
      echo "== $path"
      hey -n "$n" -c 10 "$base$path" 2>/dev/null \
        | grep -E '50%|95%|Requests/sec' | sed 's/^/   /'
    done < "$paths_file"
    ;;

  *)
    sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'
    exit 1
    ;;
esac
