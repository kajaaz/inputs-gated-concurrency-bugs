#!/bin/sh
# Builds, for every Go case, with harness/symstdin.go injected by -overlay so
# the arguments come from stdin:
#   harness/myprog-symqemu       same flags as myprog-zorya (no -race,
#                                -gcflags="all=-N -l"), run under SymQEMU
#   bin/<case>-race-stdin        go build -race, the oracle that replays the
#                                inputs SymQEMU generates
# The toolchain is pinned to go1.26.5, the one used for myprog-zorya and the
# race-detector binaries: the http2 race is in the stdlib's vendored
# h2_bundle.go and no longer reproduces with go1.26.10.
set -u
GO=${GO:-$HOME/Programs/go1.26.5/go/bin/go}
export GOTOOLCHAIN=local
E=$(cd "$(dirname "$0")" && pwd)
C=$(cd "$E/../../harnesses/go" && pwd)
mkdir -p "$E/logs" "$E/work" "$E/bin"
for d in "$C"/*/; do
  case=$(basename "$d")
  if [ "$case" = free5gc-bsf-put-2026 ]; then
    src=$d/bsf-v1.0.1; pkg=./cmd/bombharness
  else
    src=$d/harness; pkg=.
  fi
  pkgdir=$(cd "$src/$pkg" && pwd)
  ov=$E/work/overlay-$case.json
  printf '{"Replace":{"%s/symstdin.go":"%s/harness/symstdin.go"}}\n' "$pkgdir" "$E" > "$ov"

  log=$E/logs/symqemu_${case}_build.log
  echo "# command: cd $src && GOTOOLCHAIN=local $GO build -tags symstdin -overlay $ov -gcflags=\"all=-N -l\" -o $d/harness/myprog-symqemu $pkg" > "$log"
  (cd "$src" && $GO build -tags symstdin -overlay "$ov" -gcflags="all=-N -l" \
    -o "$d/harness/myprog-symqemu" "$pkg") >> "$log" 2>&1
  echo "# exit: $?" >> "$log"
  echo "$case symqemu $(tail -n 1 "$log")"

  log=$E/logs/go-race_${case}_stdin-build.log
  echo "# command: cd $src && GOTOOLCHAIN=local $GO build -race -tags symstdin -overlay $ov -o $E/bin/$case-race-stdin $pkg" > "$log"
  (cd "$src" && $GO build -race -tags symstdin -overlay "$ov" \
    -o "$E/bin/$case-race-stdin" "$pkg") >> "$log" 2>&1
  echo "# exit: $?" >> "$log"
  echo "$case race $(tail -n 1 "$log")"
done
