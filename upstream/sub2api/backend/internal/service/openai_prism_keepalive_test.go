package service

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The adapter remains blocked until the client receives a real network heartbeat.
// A buffered response cannot pass this check.
func TestPrismBrowserHeartbeatArrivesBeforeTerminal(t *testing.T) {
	for _, failing := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "failed"}[failing], func(t *testing.T) {
			release := make(chan struct{})
			adapter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				if failing {
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = io.WriteString(w, `{"error":{"type":"project_runtime_rate_limited","message":"Prism runtime is cooling down"}}`)
					return
				}
				_, _ = io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_fixture\",\"status\":\"completed\",\"model\":\"gpt-6.1-sol\",\"usage\":null,\"output\":[{\"type\":\"message\",\"content\":[{\"text\":\"answer\"}]}]}}\n\n")
			}))
			defer adapter.Close()
			s, account := prismTestService(adapter.URL)
			s.cfg.Gateway.StreamKeepaliveInterval = 1
			done := make(chan error, 1)
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, _ := gin.CreateTestContext(w)
				c.Request = r
				_, err := s.forwardPrismBrowser(r.Context(), c, account, []byte(`{"model":"gpt-6.1-sol","input":"fixture","stream":true}`), time.Now())
				done <- err
			}))
			defer gateway.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, gateway.URL, nil)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				close(release)
				t.Fatalf("no heartbeat before adapter terminal: %v", err)
			}
			defer resp.Body.Close()
			reader := bufio.NewReader(resp.Body)
			line, err := reader.ReadString('\n')
			close(release)
			require.NoError(t, err)
			require.Equal(t, ": keepalive\n", line)
			require.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))
			rest, err := io.ReadAll(reader)
			require.NoError(t, err)
			if failing {
				require.Error(t, <-done)
				require.Equal(t, 1, strings.Count(string(rest), "event: response.failed"))
				require.NotContains(t, string(rest), "event: response.completed")
			} else {
				require.NoError(t, <-done)
				require.Equal(t, 1, strings.Count(string(rest), "event: response.completed"))
				require.Contains(t, string(rest), "answer")
			}
		})
	}
}
