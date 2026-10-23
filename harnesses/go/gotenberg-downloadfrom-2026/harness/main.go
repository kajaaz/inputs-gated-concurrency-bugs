// Harness for CVE-2026-45742 (Gotenberg v8.10.0 to v8.32.x):
// multipart downloadFrom handling starts one goroutine per entry, and each
// goroutine writes shared request maps without synchronization.
//
// Usage: ./gotenberg-downloadfrom <enabled> <entries>
//
//	<enabled>  1 if the downloadFrom feature is enabled, 0 otherwise
//	<entries>  number of downloadFrom entries in the request, one ASCII digit
//
// The race needs enabled == 1 and entries >= 2.
package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
)

type requestContext struct {
	files          map[string]string
	diskToOriginal map[string]string
	filesByField   map[string][]string
}

func main() {
	if len(os.Args) != 3 || len(os.Args[1]) != 1 || len(os.Args[2]) != 1 {
		fmt.Println("Usage: ./gotenberg-downloadfrom <enabled> <entries>")
		return
	}
	enabled := os.Args[1][0] == '1'
	entries := int(os.Args[2][0] - '0')
	if entries < 0 || entries > 9 {
		fmt.Println("Usage: ./gotenberg-downloadfrom <enabled> <entries>")
		return
	}

	ctx := requestContext{
		files:          make(map[string]string),
		diskToOriginal: make(map[string]string),
		filesByField:   make(map[string][]string),
	}
	if !enabled || entries == 0 {
		fmt.Println("OK")
		return
	}

	runtime.GOMAXPROCS(4)
	var ready sync.WaitGroup
	var done sync.WaitGroup
	release := make(chan struct{})
	ready.Add(entries)
	for i := 0; i < entries; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			filename := fmt.Sprintf("download-%d", i)
			path := fmt.Sprintf("/tmp/gotenberg-%d", i)
			ready.Done()
			<-release
			ctx.files[filename] = path
			ctx.diskToOriginal[path] = filename
			ctx.filesByField["downloadFrom"] = append(ctx.filesByField["downloadFrom"], path)
		}(i)
	}
	ready.Wait()
	close(release)
	done.Wait()
	fmt.Println("OK")
}
