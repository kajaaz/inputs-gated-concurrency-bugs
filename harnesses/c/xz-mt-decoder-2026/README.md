# xz Multithreaded Decoder Progress Race

Data race in liblzma's multithreaded `.xz` decoder. When the main thread parses
a Block Header, it updates `coder->progress_in` without holding `coder->mutex`
(`src/liblzma/common/stream_decoder_mt.c:1071`). A worker thread that finishes a
block updates the same field under the mutex (line 460).

**Project**: [tukaani-project/xz](https://github.com/tukaani-project/xz)  
**Vulnerable version**: `v5.8.3` (and earlier releases with the threaded decoder)  
**Fix commit**: [`c6e3aadb`](https://github.com/tukaani-project/xz/commit/c6e3aadbb510e44cecfe870408ecfea1d1ca792c) "liblzma: mt dec: Protect progress_in update with mutex"  
**Fixed version**: `v5.8.4`

## Gate

The input is the compressed file. The race needs all of these conditions:

1. the decoder runs with at least two threads (`-T2` or more);
2. the Block Headers store both the compressed and the uncompressed size;
   otherwise `is_direct_mode_needed()` (line 1230) selects the single-threaded
   direct mode;
3. the file has more than one block, so a worker is still running when the main
   thread parses the next Block Header.

The block size does not open or close the gate, but it changes how likely the
schedule is: small blocks make workers finish while the main thread parses.

## Build

The program under test can be either the upstream `xz` binary or the local
`harness/main.c`, which calls `lzma_stream_decoder_mt()` directly. The local
harness makes the executable entry point visible in this dataset, while the
race itself remains in upstream `src/liblzma/common/stream_decoder_mt.c`.
`XZ_SANDBOX=no` is needed only for the upstream `xz` binary because the Landlock
sandbox is incompatible with `-fsanitize`.

```bash
git clone https://github.com/tukaani-project/xz && cd xz
git checkout v5.8.3
cmake -S . -B build-tsan -DCMAKE_BUILD_TYPE=Debug -DCMAKE_C_COMPILER=clang \
  -DCMAKE_C_FLAGS="-fsanitize=thread -g -O1" \
  -DCMAKE_EXE_LINKER_FLAGS="-fsanitize=thread" \
  -DBUILD_SHARED_LIBS=OFF -DXZ_NLS=OFF -DXZ_DOC=OFF -DXZ_SANDBOX=no
cmake --build build-tsan -j8 --target xz
```

Build the local harness against that liblzma:

```bash
clang -fsanitize=thread -g -O1 -I<xz>/src/liblzma/api \
  harness/main.c <xz>/build-tsan/src/liblzma/liblzma.a -lpthread \
  -o xz-mt-decoder
```

For KLEE, build liblzma to LLVM bitcode with `wllvm` or `gclang` and drive
`harness/main.c` with a symbolic input file or a symbolic in-memory buffer.

## Inputs

```bash
./make-inputs.sh inputs
```

## Trigger

```bash
# Trigger: several small blocks with stored sizes, threaded decoding.
build-tsan/xz -dc -T4 inputs/multi-4KiB.xz  > /dev/null
build-tsan/xz -dc -T4 inputs/multi-16KiB.xz > /dev/null
./xz-mt-decoder 4 inputs/multi-4KiB.xz > /dev/null

# Escapes.
build-tsan/xz -dc -T1 inputs/multi-4KiB.xz    > /dev/null  # one thread
build-tsan/xz -dc -T4 inputs/single.xz        > /dev/null  # no stored sizes
build-tsan/xz -dc -T4 inputs/oneblock-4KiB.xz > /dev/null  # one block
./xz-mt-decoder 1 inputs/multi-4KiB.xz > /dev/null         # one thread
```

## Observed results

Observed with clang ThreadSanitizer on Linux x86-64. The decompressed output is
always identical to the original data.

| Build | Input | Threads | TSan reports |
|---|---|---|---|
| 5.8.3 | `multi-4KiB.xz`, `multi-16KiB.xz` | 2, 4, 8 | race in 6 of 6 runs |
| 5.8.3 | `multi-64KiB.xz` | 4, 8 | some runs |
| 5.8.3 | `multi-256KiB.xz` | 4, 8 | none observed |
| 5.8.3 | `multi-4KiB.xz` | 1 | none |
| 5.8.3 | `single.xz` | 4, 8 | none |
| 5.8.3 | `oneblock-4KiB.xz` | 4 | none |
| 5.8.4 | `multi-4KiB.xz`, `multi-16KiB.xz` | 2, 4, 8 | none |
