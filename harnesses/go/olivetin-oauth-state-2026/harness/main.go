// Harness for CVE-2026-28789 (OliveTin before 3000.10.3): OAuth2 login and
// callback handlers access registeredStates without synchronization.
//
// Usage: ./olivetin-oauth-state <oauth> <requests>
//
//	<oauth>     1 enables OAuth2, 0 disables it
//	<requests>  number of concurrent OAuth2 requests, one ASCII digit
//
// The race needs oauth == 1 and requests >= 2.
package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
)

type oauth2State struct {
	nonce       string
	redirectURI string
}

var registeredStates = map[string]*oauth2State{}

func login(i, round int) {
	state := fmt.Sprintf("state-%d-%d", i, round)
	registeredStates[state] = &oauth2State{
		nonce:       fmt.Sprintf("nonce-%d", round),
		redirectURI: "/",
	}
}

func callback(i, round int) {
	state := fmt.Sprintf("state-%d-%d", i, round)
	_ = registeredStates[state]
	if round%2 == 0 {
		delete(registeredStates, state)
	}
}

func main() {
	if len(os.Args) != 3 || len(os.Args[1]) != 1 || len(os.Args[2]) != 1 {
		fmt.Println("Usage: ./olivetin-oauth-state <oauth> <requests>")
		return
	}
	oauth := os.Args[1][0] == '1'
	requests := int(os.Args[2][0] - '0')
	if requests < 0 || requests > 9 {
		fmt.Println("Usage: ./olivetin-oauth-state <oauth> <requests>")
		return
	}
	if !oauth || requests < 2 {
		for i := 0; i < requests; i++ {
			login(i, 0)
			callback(i, 0)
		}
		fmt.Println("OK")
		return
	}

	runtime.GOMAXPROCS(4)
	var ready sync.WaitGroup
	var done sync.WaitGroup
	release := make(chan struct{})
	ready.Add(requests)
	for i := 0; i < requests; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-release
			for round := 0; round < 1000; round++ {
				login(i, round)
				callback(i, round-1)
			}
		}(i)
	}
	ready.Wait()
	close(release)
	done.Wait()
	fmt.Println("OK")
}
