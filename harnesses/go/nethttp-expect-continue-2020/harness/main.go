// Harness for CVE-2020-15586 (Go net/http, fixed in Go 1.13.13 and 1.14.5):
// a Handler reads the request body while it writes a response, racing with the
// server's implicit "100 Continue" write on affected Go releases.
//
// Usage: ./nethttp-expect-continue <expect> <body-size>
//
//	<expect>     1 to send Expect: 100-continue, 0 otherwise
//	<body-size>  request body size, one ASCII digit
//
// The race needs expect == 1 and body-size > 0.
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
)

const requests = 64

func main() {
	if len(os.Args) != 3 || len(os.Args[1]) != 1 || len(os.Args[2]) != 1 {
		fmt.Println("Usage: ./nethttp-expect-continue <expect> <body-size>")
		return
	}
	expect := os.Args[1][0] == '1'
	bodySize := int(os.Args[2][0] - '0')
	if bodySize < 0 || bodySize > 9 {
		fmt.Println("Usage: ./nethttp-expect-continue <expect> <body-size>")
		return
	}

	runtime.GOMAXPROCS(4)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("listen:", err)
		return
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(ioutil.Discard, r.Body)
		}()
		_, _ = w.Write([]byte("response"))
		wg.Wait()
	})}
	go srv.Serve(ln)
	defer srv.Close()

	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.Repeat("x", bodySize)
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				return
			}
			defer conn.Close()
			expectHeader := ""
			if expect {
				expectHeader = "Expect: 100-continue\r\n"
			}
			_, _ = fmt.Fprintf(conn,
				"POST / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n%sContent-Length: %d\r\n\r\n%s",
				expectHeader, len(body), body)
			_, _ = io.Copy(ioutil.Discard, conn)
		}()
	}
	wg.Wait()
	fmt.Println("OK")
}
