//go:build unit

package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type convergenceMockUpstream struct {
	service.HTTPUpstream
	stream bool
	calls  atomic.Int64
}

func (u *convergenceMockUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls.Add(1)
	terminal := `data: {"type":"response.completed","response":{"id":"resp_native_load","status":"completed","model":"gpt-5.1","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n"
	if !u.stream {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(10 * time.Millisecond):
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(terminal))}, nil
	}
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		for i := 0; i < 20; i++ {
			select {
			case <-req.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
			event := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"x\"}\n\n"
			if i == 19 {
				event = terminal
			}
			if _, err := io.WriteString(writer, event); err != nil {
				return
			}
		}
	}()
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: reader}, nil
}

// Mock-only burst through the actual Responses handler. No network or paid upstream.
func TestNativeConvergenceHTTPMockBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		for _, workers := range []int{1, 32, 128} {
			t.Run(fmt.Sprintf("stream_%t_workers_%d", stream, workers), func(t *testing.T) {
				n := 1000
				if stream && workers == 1 {
					n = 20
				}
				u := &convergenceMockUpstream{stream: stream}
				h := newOpenAIResponsesFailoverTestHandler(t, u)
				before := runtime.NumGoroutine()
				jobs := make(chan int)
				var wg sync.WaitGroup
				durations := make([]time.Duration, n)
				var bad atomic.Int64
				for w := 0; w < workers; w++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for i := range jobs {
							c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
							if stream {
								c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"gpt-5.1","stream":true,"input":"hello"}`))
								c.Request.Header.Set("Content-Type", "application/json")
							}
							started := time.Now()
							h.Responses(c)
							durations[i] = time.Since(started)
							if rec.Code != 200 || rec.Body.Len() == 0 {
								bad.Add(1)
							}
							if stream && !strings.Contains(rec.Body.String(), "response.completed") {
								bad.Add(1)
							}
						}
					}()
				}
				for i := 0; i < n; i++ {
					jobs <- i
				}
				close(jobs)
				wg.Wait()
				sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
				require.Zero(t, bad.Load(), "empty/error/incomplete responses")
				require.EqualValues(t, n, u.calls.Load(), "no retries on successful upstream")
				require.Eventually(t, func() bool { return runtime.NumGoroutine() <= before+12 }, 2*time.Second, 10*time.Millisecond)
				t.Logf("requests=%d workers=%d stream=%t p99=%s goroutines_before=%d after=%d", n, workers, stream, durations[(n-1)*99/100], before, runtime.NumGoroutine())
			})
		}
	}
}
