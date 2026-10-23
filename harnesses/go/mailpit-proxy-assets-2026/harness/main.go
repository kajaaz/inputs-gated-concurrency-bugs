// Harness for CVE-2026-45712 (Mailpit before 1.30.0): the proxy CSS
// rewriter reads a package-level assets map without holding assetsMutex while
// cache and cleanup paths write the same map under the lock.
//
// Usage: ./mailpit-proxy-assets <proxy> <css>
//
//	<proxy>  1 reaches /proxy?data=..., 0 skips the proxy path
//	<css>    1 gives the message a stylesheet link, 0 gives it no CSS asset
//
// The race needs proxy == 1 and css == 1.
package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
)

type messageAssets struct {
	cssURL string
	body   string
}

var (
	assets      = map[string]messageAssets{}
	assetsMutex sync.Mutex
)

func cacheAsset(id string, round int) {
	assetsMutex.Lock()
	defer assetsMutex.Unlock()
	if round%5 == 0 {
		delete(assets, id)
		return
	}
	assets[id] = messageAssets{
		cssURL: fmt.Sprintf("https://example.test/%s/%d.css", id, round),
		body:   "body { color: black }",
	}
}

func rewriteCSS(id string) string {
	asset := assets[id] // BUG: missing assetsMutex.
	return asset.cssURL
}

func main() {
	if len(os.Args) != 3 || len(os.Args[1]) != 1 || len(os.Args[2]) != 1 {
		fmt.Println("Usage: ./mailpit-proxy-assets <proxy> <css>")
		return
	}
	proxy := os.Args[1][0] == '1'
	css := os.Args[2][0] == '1'
	if !proxy || !css {
		fmt.Println("OK")
		return
	}

	runtime.GOMAXPROCS(4)
	const requests = 16
	const rounds = 1000
	id := "message-1"
	cacheAsset(id, 1)

	var ready sync.WaitGroup
	var done sync.WaitGroup
	release := make(chan struct{})
	ready.Add(requests + 1)
	for i := 0; i < requests; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			<-release
			for round := 0; round < rounds; round++ {
				_ = rewriteCSS(id)
			}
		}()
	}
	done.Add(1)
	go func() {
		defer done.Done()
		ready.Done()
		<-release
		for round := 0; round < rounds; round++ {
			cacheAsset(id, round)
		}
	}()
	ready.Wait()
	close(release)
	done.Wait()
	fmt.Println("OK")
}
