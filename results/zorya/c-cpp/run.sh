#!/usr/bin/env bash
# Zorya (data-race profile) on the C and C++ harnesses, from the trigger and
# from one benign input each. Runs are sequential: Zorya writes to a shared
# results directory, which is copied after each run.
set -u
O=$(cd "$(dirname "$0")" && pwd)
LIMIT=${LIMIT:-900}
XZ=$O/bin/xz-mt-decoder-nopie
XZIN=${XZIN:-$O/inputs/multi-64k-4KiB.xz}
RDB=$O/bin/rocksdb-inplace-update-nopie
RES=$HOME/Documents/zorya/results

runs=(
  "xz-mt-decoder-2026|trigger|$XZ|4 $XZIN"
  "xz-mt-decoder-2026|benign|$XZ|1 $XZIN"
  "rocksdb-inplace-update-2024|trigger|$RDB|1 1"
  "rocksdb-inplace-update-2024|benign|$RDB|0 1"
)

mkdir -p "$O/logs" "$O/results"
[ -f "$O/summary.tsv" ] || printf 'case\tinputs\targs\tlimit_s\texit\tseconds\tstatus\tlog\n' > "$O/summary.tsv"
for r in "${runs[@]}"; do
  IFS='|' read -r case label bin args <<< "$r"
  log=$O/logs/${case}_${label}.log
  echo "# command: timeout ${LIMIT}s zorya $bin --analysis data-race --arg $args" > "$log"
  start=$(date +%s)
  # shellcheck disable=SC2086
  timeout "${LIMIT}s" zorya "$bin" --analysis data-race --arg $args >> "$log" 2>&1
  code=$?
  secs=$(( $(date +%s) - start ))
  status=$(rg -o 'race_found|no_concurrency|gate_reached_schedule_not_explored' "$log" | tail -1)
  [ -z "$status" ] && { [ "$code" = 124 ] && status=timeout || status=other; }
  rm -rf "$O/results/${case}_${label}"
  cp -r "$RES" "$O/results/${case}_${label}" 2>/dev/null
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$case" "$label" "$args" "$LIMIT" "$code" "$secs" "$status" "$log" >> "$O/summary.tsv"
  echo "$case $label exit=$code ${secs}s status=$status"
done
