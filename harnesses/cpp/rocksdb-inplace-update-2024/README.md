# RocksDB In-Place Update Race

Data race in RocksDB's memtable. With `inplace_update_support`, `MemTable::Update()`
overwrites an existing value in place when the new value is no larger than the
old one. It rewrites the length prefix with `EncodeVarint32` before it takes the
per-key write lock (`db/memtable.cc:1503` in v9.2.1). A concurrent `Get()` reads
the same prefix in `SaveValue` (`db/memtable.cc:1067`) under the read lock.

**Project**: [facebook/rocksdb](https://github.com/facebook/rocksdb)  
**Vulnerable version**: `v9.2.1`  
**Fix commit**: [`933ac0e0`](https://github.com/facebook/rocksdb/commit/933ac0e05ce8c14847d0c67160c3c5869eae9656) "Fix locking for `ColumnFamilyOptions::inplace_update_support`" ([PR #12624](https://github.com/facebook/rocksdb/pull/12624))  
**Fixed version**: `v9.3.1`

## Gate

The race needs both of these input conditions:

1. the column family enables `inplace_update_support`;
2. an overwrite is no larger than the value already stored for the key.

Without the option, every `Put` inserts a new memtable entry. With a larger
value, `Update()` also inserts a new entry instead of rewriting the old one.

## Harness

The harness opens a database in a temporary directory, stores a 64-byte value,
and starts two threads that loop on `Get()`. The main thread then overwrites the
key 20,000 times. It prints `OK` at the end; ThreadSanitizer reports the race.

```text
Usage: ./rocksdb-inplace-update <inplace> <sizes>
  <inplace>  1 sets inplace_update_support, 0 does not
  <sizes>    1 overwrites with constant-size values,
             0 makes every overwrite one byte longer
```

## Build

```bash
git clone https://github.com/facebook/rocksdb && cd rocksdb
git checkout v9.2.1
cmake -S . -B build-tsan -DCMAKE_BUILD_TYPE=Debug \
  -DCMAKE_C_COMPILER=clang -DCMAKE_CXX_COMPILER=clang++ \
  -DCMAKE_C_FLAGS="-fsanitize=thread -g -O1" \
  -DCMAKE_CXX_FLAGS="-fsanitize=thread -g -O1" \
  -DWITH_TESTS=OFF -DWITH_TOOLS=OFF -DWITH_BENCHMARK_TOOLS=OFF \
  -DWITH_CORE_TOOLS=OFF -DWITH_TRACE_TOOLS=OFF -DWITH_GFLAGS=OFF \
  -DWITH_SNAPPY=OFF -DWITH_LZ4=OFF -DWITH_ZLIB=OFF -DWITH_ZSTD=OFF -DWITH_BZ2=OFF \
  -DFAIL_ON_WARNINGS=OFF -DROCKSDB_BUILD_SHARED=OFF -DPORTABLE=1 -DUSE_RTTI=1
cmake --build build-tsan -j8 --target rocksdb

cd <this directory>/harness
clang++ -std=c++17 -fsanitize=thread -g -O1 -I<rocksdb>/include main.cc \
  <rocksdb>/build-tsan/librocksdb.a -lpthread -ldl -o rocksdb-inplace-update
```

For KLEE, build RocksDB and the harness to LLVM bitcode with `wllvm` or
`gclang`, and make the two arguments symbolic.

## Trigger

```bash
# Trigger: in-place updates with values that fit.
./rocksdb-inplace-update 1 1

# Escapes.
./rocksdb-inplace-update 1 0   # values grow, so no in-place rewrite
./rocksdb-inplace-update 0 1   # option off
```

## Observed results

Three runs per input, with clang ThreadSanitizer on Linux x86-64.

| Library | Input | TSan reports per run |
|---|---|---|
| v9.2.1 | `1 1` | 1, 1, 1 (`MemTable::Update` against `SaveValue`) |
| v9.2.1 | `1 0` | 0, 0, 0 |
| v9.2.1 | `0 1` | 0, 0, 0 |
| v9.3.1 | `1 1`, `1 0`, `0 1` | 0 |
