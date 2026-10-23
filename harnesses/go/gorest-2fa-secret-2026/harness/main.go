// Harness for CVE-2026-48154 (gorest before 1.12.2): several 2FA handlers
// access a package-level in-memory secret map without synchronization.
//
// Usage: ./gorest-2fa-secret <memory> <ops>
//
//	<memory>  1 uses the in-memory 2FA store, 0 skips it
//	<ops>     number of concurrent 2FA operations, one ASCII digit
//
// The race needs memory == 1 and ops >= 2.
package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
)

type secret2FA struct {
	secret   string
	passHash []byte
}

var inMemorySecret2FA = map[uint64]secret2FA{}

func touchSecret(user uint64, round int) {
	if round%3 == 0 {
		inMemorySecret2FA[user] = secret2FA{
			secret:   fmt.Sprintf("secret-%d-%d", user, round),
			passHash: []byte{byte(round)},
		}
		return
	}
	if round%3 == 1 {
		_ = inMemorySecret2FA[user]
		return
	}
	delete(inMemorySecret2FA, user)
}

func main() {
	if len(os.Args) != 3 || len(os.Args[1]) != 1 || len(os.Args[2]) != 1 {
		fmt.Println("Usage: ./gorest-2fa-secret <memory> <ops>")
		return
	}
	memory := os.Args[1][0] == '1'
	ops := int(os.Args[2][0] - '0')
	if ops < 0 || ops > 9 {
		fmt.Println("Usage: ./gorest-2fa-secret <memory> <ops>")
		return
	}
	if !memory || ops < 2 {
		for i := 0; i < ops; i++ {
			touchSecret(uint64(i), i)
		}
		fmt.Println("OK")
		return
	}

	runtime.GOMAXPROCS(4)
	var ready sync.WaitGroup
	var done sync.WaitGroup
	release := make(chan struct{})
	ready.Add(ops)
	for i := 0; i < ops; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-release
			for round := 0; round < 1000; round++ {
				touchSecret(uint64(i%2), round+i)
			}
		}(i)
	}
	ready.Wait()
	close(release)
	done.Wait()
	fmt.Println("OK")
}
