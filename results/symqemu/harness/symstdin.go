//go:build symstdin

// SymQEMU only makes stdin (or SYMCC_INPUT_FILE) symbolic, not argv. This file
// is added to each Go harness package with `go build -overlay` and rebuilds
// os.Args from space-separated stdin before main runs, so main.go is unchanged.
//
// The arguments alias the stdin buffer (alias reinterprets the slice header,
// which also builds with the go 1.14 module of the nethttp case). A regular
// string(b) conversion of a one-byte slice returns a pointer into the runtime's
// staticuint64s table indexed by the byte, and SymQEMU concretizes symbolic
// memory indices, so the argument would lose its symbolic value.
//
// Example: printf '1 2' | symqemu-x86_64 ./myprog-symqemu
package main

import (
	"io/ioutil"
	"os"
	"unsafe"
)

func alias(b []byte) string { return *(*string)(unsafe.Pointer(&b)) }

func init() {
	data, _ := ioutil.ReadAll(os.Stdin)
	args := os.Args[:1]
	start := -1
	for i, b := range data {
		if b == ' ' || b == '\n' {
			if start >= 0 {
				args = append(args, alias(data[start:i]))
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		args = append(args, alias(data[start:]))
	}
	os.Args = args
}
