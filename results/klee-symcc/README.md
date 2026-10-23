# KLEE and SymCC

Why neither tool appears in Table II (`klee-summary.tsv`, `symcc-summary.tsv`,
`logs/`).

- Go harnesses: both tools analyse LLVM bitcode, and Go's gc compiler does not
  produce it. KLEE rejects a Go ELF binary; SymCC compiles C and C++ only.
- liblzma: the harness compiles to bitcode with the xz headers, but the
  installed KLEE cannot load bitcode from the available Clang versions
  (`clang-14` gives `Invalid record`, `clang-18` an LLVM producer/reader
  mismatch). The SymCC build of xz fails: Clang 14 crashes on upstream
  `crc32_fast.c` (`logs/symcc_xz-mt-decoder-2026_build.log`).
- RocksDB: not run, as both tools would need the whole RocksDB library rebuilt
  as bitcode.
