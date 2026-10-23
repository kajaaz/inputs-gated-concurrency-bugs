// Harness for CVE-2023-45286 (go-resty v2.10.0): a request retry puts the
// same pooled body buffer back twice, so two later requests share one buffer.
//
// Usage: ./resty-pool-retry <retries> <status>
//
//	<retries>  retry count configured on the client (one ASCII digit)
//	<status>   status of the first answer to each request (three ASCII digits)
//
// The race needs retries >= 1 and status >= 500: only then does a retry occur.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-resty/resty/v2"
)

const requests = 32

func main() {
	if len(os.Args) != 3 || len(os.Args[1]) != 1 || len(os.Args[2]) != 3 {
		fmt.Println("Usage: ./resty-pool-retry <retries> <status>")
		return
	}
	retries := int(os.Args[1][0] - '0')
	status := 100*int(os.Args[2][0]-'0') + 10*int(os.Args[2][1]-'0') + int(os.Args[2][2]-'0')
	if retries < 0 || retries > 9 || status < 100 || status > 599 {
		fmt.Println("Usage: ./resty-pool-retry <retries> <status>")
		return
	}

	var seen sync.Map
	var mixed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body := string(raw)
		if strings.Count(body, "req-") > 1 {
			mixed.Add(1)
		}
		if _, retried := seen.LoadOrStore(body, true); !retried {
			w.WriteHeader(status)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := resty.New().
		SetRetryCount(retries).
		SetRetryWaitTime(0).
		SetRetryMaxWaitTime(0).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			return err != nil || r.StatusCode() >= 500
		})

	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = client.R().SetBody(fmt.Sprintf("req-%02d", i)).Post(server.URL)
		}(i)
	}
	wg.Wait()

	if mixed.Load() > 0 {
		fmt.Printf("LEAK: %d request bodies carried another request's body\n", mixed.Load())
		return
	}
	fmt.Println("OK")
}
