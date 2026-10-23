#!/bin/sh
# Builds the trigger and escape inputs for the xz multithreaded decoder race.
# Any xz >= 5.4 can compress them; only the decoder under test must be 5.8.3.
set -eu
out=${1:-inputs}
mkdir -p "$out"

head -c 6000000 /dev/urandom | base64 > "$out/data.txt"
head -c 4000 "$out/data.txt" > "$out/small.txt"

# Trigger: many small blocks whose headers store both sizes.
xz -T4 --block-size=4KiB  -c "$out/data.txt" > "$out/multi-4KiB.xz"
xz -T4 --block-size=16KiB -c "$out/data.txt" > "$out/multi-16KiB.xz"

# Same gate, but larger blocks make the race window rarer.
xz -T4 --block-size=64KiB  -c "$out/data.txt" > "$out/multi-64KiB.xz"
xz -T4 --block-size=256KiB -c "$out/data.txt" > "$out/multi-256KiB.xz"

# Escape: one block without sizes, so the decoder uses direct mode.
xz -T1 -c "$out/data.txt" > "$out/single.xz"

# Escape: a single block, so no worker runs while the main thread parses.
xz -T4 --block-size=4KiB -c "$out/small.txt" > "$out/oneblock-4KiB.xz"
