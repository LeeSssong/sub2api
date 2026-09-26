package middleware

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// A successfully decoded request must retain its actual body allocation, not
// the worst-case decompression reservation, while its upstream stream runs.
func TestExcelBPSImageAdmissionDecodedStreamAllowsOtherRequests(t *testing.T) {
	for _, encoding := range []string{"gzip", "chunked", "unknown-zero"} {
		t.Run(encoding, func(t *testing.T) {
			decoded := []byte(`{"model":"test","input":"` + strings.Repeat("a", 4096) + `"}`)
			wire := decoded
			if encoding == "gzip" {
				var compressed bytes.Buffer
				w := gzip.NewWriter(&compressed)
				_, err := w.Write(decoded)
				require.NoError(t, err)
				require.NoError(t, w.Close())
				wire = compressed.Bytes()
			}
			entered := make(chan []byte, 1)
			done := make(chan struct{})
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(func() { cancel(); <-done })
			r := bpsImageTestRouter(bpsImageTestSettings{enabled: true}, func(c *gin.Context) {
				body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
				if err != nil {
					c.Status(http.StatusBadRequest)
					return
				}
				if c.GetHeader("Hold") == "true" {
					entered <- body
					<-c.Request.Context().Done()
				}
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(wire)).WithContext(ctx)
			req.Header.Set("Hold", "true")
			if encoding == "gzip" {
				req.Header.Set("Content-Encoding", encoding)
			} else if encoding == "chunked" {
				req.ContentLength = -1
			} else {
				req.ContentLength = 0
			}
			go func() {
				defer close(done)
				r.ServeHTTP(httptest.NewRecorder(), req)
			}()
			select {
			case body := <-entered:
				require.Equal(t, decoded, body)
			case <-time.After(5 * time.Second):
				t.Fatal("compressed request did not reach its streaming handler")
			}
			probe := httptest.NewRecorder()
			r.ServeHTTP(probe, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"input":"hello"}`)))
			require.Equal(t, http.StatusNoContent, probe.Code, "decoded stream monopolized the shared request budget: %s", probe.Body.String())
			if encoding == "gzip" {
				compressedProbe := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(wire))
				compressedProbe.Header.Set("Content-Encoding", "gzip")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, compressedProbe)
				require.Equal(t, http.StatusNoContent, w.Code, "preprocessing lease survived into the stream")
			}
		})
	}
}

func bpsImageGzip(t *testing.T, body []byte) []byte {
	t.Helper()
	var wire bytes.Buffer
	w := gzip.NewWriter(&wire)
	_, err := w.Write(body)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return wire.Bytes()
}

type bpsImageGatedBody struct {
	reader  io.Reader
	started chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (b *bpsImageGatedBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.started) })
	<-b.proceed
	return b.reader.Read(p)
}
func (*bpsImageGatedBody) Close() error { return nil }

func TestExcelBPSImageAdmissionPreprocessingIsBoundedSeparately(t *testing.T) {
	wire := bpsImageGzip(t, []byte(`{"input":"hello"}`))
	body := &bpsImageGatedBody{reader: bytes.NewReader(wire), started: make(chan struct{}), proceed: make(chan struct{})}
	done := make(chan struct{})
	var unblock sync.Once
	t.Cleanup(func() { unblock.Do(func() { close(body.proceed) }); <-done })
	r := bpsImageTestRouter(bpsImageTestSettings{enabled: true}, func(c *gin.Context) {
		_, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.Status(http.StatusNoContent)
	})
	first := httptest.NewRequest(http.MethodPost, "/responses", body)
	first.Header.Set("Content-Encoding", "gzip")
	go func() { defer close(done); r.ServeHTTP(httptest.NewRecorder(), first) }()
	select {
	case <-body.started:
	case <-time.After(5 * time.Second):
		t.Fatal("preprocessing did not start")
	}
	var reads atomic.Int32
	second := httptest.NewRequest(http.MethodPost, "/v1/responses", &bpsImageCountingBody{reads: &reads, reader: bytes.NewReader(wire)})
	second.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, second)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Zero(t, reads.Load(), "busy preprocessing must reject before reading another body")
	plain := httptest.NewRecorder()
	r.ServeHTTP(plain, httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"input":"hello"}`)))
	require.Equal(t, http.StatusNoContent, plain.Code, "slow preprocessing blocked ordinary traffic")
	unblock.Do(func() { close(body.proceed) })
	<-done
	valid := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(wire))
	valid.Header.Set("Content-Encoding", "gzip")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, valid)
	require.Equal(t, http.StatusNoContent, w.Code, "preprocessing was not released")
}

func TestExcelBPSImageAdmissionCompressedRequestsKeepSlotLimit(t *testing.T) {
	wire := bpsImageGzip(t, []byte(`{"input":"hello"}`))
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	t.Cleanup(func() { cancel(); wg.Wait() })
	entered := make(chan struct{}, 32)
	r := bpsImageTestRouter(bpsImageTestSettings{enabled: true}, func(c *gin.Context) {
		if c.GetHeader("Hold") == "true" {
			entered <- struct{}{}
			<-c.Request.Context().Done()
		}
		c.Status(http.StatusNoContent)
	})
	for i := 0; i < 32; i++ {
		req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(wire)).WithContext(ctx)
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Hold", "true")
		wg.Add(1)
		go func() { defer wg.Done(); r.ServeHTTP(httptest.NewRecorder(), req) }()
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d compressed streams were admitted", i)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader("hello")))
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "33rd request must not bypass the slot limit")
	cancel()
	wg.Wait()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader("hello")))
	require.Equal(t, http.StatusNoContent, w.Code, "cancelled streams leaked retained slots")
}

func TestExcelBPSImageAdmissionDecodeFailuresReleaseBothBudgets(t *testing.T) {
	r := bpsImageTestRouter(bpsImageTestSettings{enabled: true}, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for i := 0; i < 40; i++ {
		req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader("invalid gzip"))
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, "decode error leaked a reservation at request %d", i)
	}
	wire := bpsImageGzip(t, []byte(`{"input":"hello"}`))
	req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(wire))
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestExcelBPSImageAdmissionResizeRetainsAndReleasesActualWeight(t *testing.T) {
	budget := &bpsImageAdmissionBudget{limitBytes: 512 << 20, maxRequests: 32}
	lease, ok := budget.acquire(8 << 20)
	require.True(t, ok)
	require.True(t, lease.resize(32<<20))
	other, ok := budget.acquire(480 << 20)
	require.True(t, ok)
	require.False(t, lease.resize(33<<20), "resize must enforce the retained-body cap")
	lease.release()
	lease.release()
	require.False(t, lease.resize(1), "released leases cannot reacquire capacity")
	next, ok := budget.acquire(32 << 20)
	require.True(t, ok, "release subtracted the initial rather than resized weight")
	next.release()
	other.release()
	require.Zero(t, budget.bytes)
	require.Zero(t, budget.requests)
}

func TestExcelBPSImageAdmissionConfiguredDecodedLimit(t *testing.T) {
	for _, encoding := range []string{"gzip", "chunked"} {
		t.Run(encoding, func(t *testing.T) {
			decoded := []byte(strings.Repeat("a", (1<<20)+1))
			wire := decoded
			if encoding == "gzip" {
				wire = bpsImageGzip(t, decoded)
			}
			r := bpsImageTestRouter(bpsImageTestSettings{enabled: true, bodyLimitMiB: 1}, func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(wire))
			if encoding == "gzip" {
				req.Header.Set("Content-Encoding", encoding)
			} else {
				req.ContentLength = -1
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
			valid := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(bpsImageGzip(t, []byte(`{"input":"hello"}`))))
			valid.Header.Set("Content-Encoding", "gzip")
			w = httptest.NewRecorder()
			r.ServeHTTP(w, valid)
			require.Equal(t, http.StatusNoContent, w.Code, "body rejection must release both resource pools")
		})
	}
}

func TestExcelBPSImageAdmissionConfiguredBudgetRemainsHardLimit(t *testing.T) {
	budget := &bpsImageAdmissionBudget{}
	budget.configure(512<<20, 512)
	var leases []*bpsImageAdmissionLease
	for i := 0; i < 64; i++ {
		lease, ok := budget.acquire(bpsImageBodyWeight(1))
		require.True(t, ok)
		leases = append(leases, lease)
	}
	_, ok := budget.acquire(bpsImageBodyWeight(1))
	require.False(t, ok, "raising the slot cap must not raise the configured memory budget")
	budget.configure(1024<<20, 2)
	_, ok = budget.acquire(1)
	require.False(t, ok, "a lower request cap applies to new requests immediately")
	for _, lease := range leases {
		lease.release()
	}
	first, ok := budget.acquire(400 << 20)
	require.True(t, ok)
	second, ok := budget.acquire(400 << 20)
	require.True(t, ok)
	_, ok = budget.acquire(1)
	require.False(t, ok)
	budget.configure(512<<20, 2)
	require.True(t, first.resize(200<<20), "live budget reductions must still allow held bodies to shrink")
	require.False(t, first.resize(201<<20), "held requests must not grow beyond the new limit")
	first.release()
	second.release()
	require.Zero(t, budget.bytes)
}

func TestExcelBPSImageAdmissionAccountsDecodedBytesThroughStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	t.Cleanup(func() { cancel(); wg.Wait() })
	entered := make(chan int, 2)
	r := bpsImageTestRouter(bpsImageTestSettings{enabled: true}, func(c *gin.Context) {
		if c.GetHeader("Hold") == "true" {
			body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
			if err != nil {
				c.Status(http.StatusBadRequest)
				return
			}
			entered <- len(body)
			<-c.Request.Context().Done()
		}
		c.Status(http.StatusNoContent)
	})
	wire := bpsImageGzip(t, bytes.Repeat([]byte("a"), 32<<20))
	startStream := func() {
		req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(wire)).WithContext(ctx)
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Hold", "true")
		wg.Add(1)
		go func() { defer wg.Done(); r.ServeHTTP(httptest.NewRecorder(), req) }()
		select {
		case n := <-entered:
			require.Equal(t, 32<<20, n)
		case <-time.After(5 * time.Second):
			t.Fatal("retained reservation was not available")
		}
	}
	startStream()
	// 32 MiB retained at 8x plus 33 MiB at 8x cannot fit the 512 MiB pool.
	tooMuch := bpsImageGzip(t, bytes.Repeat([]byte("a"), 33<<20))
	req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(tooMuch))
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "retained memory was charged by compressed size")
	// A failed resize must release its initial live lease AND decode reservation.
	startStream()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader("hello")))
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "decoded bodies stopped counting during SSE")
	cancel()
	wg.Wait()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader("hello")))
	require.Equal(t, http.StatusNoContent, w.Code, "stream completion leaked resized reservations")
}

func TestExcelBPSImageAdmissionRejectsDecodedOverflowAndReleasesBudgets(t *testing.T) {
	wire := bpsImageGzip(t, bytes.Repeat([]byte("a"), (64<<20)+1))
	r := bpsImageTestRouter(bpsImageTestSettings{enabled: true}, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(wire))
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	require.Contains(t, w.Body.String(), "basispoints_image_body_too_large")
	valid := bpsImageGzip(t, []byte(`{"input":"hello"}`))
	req = httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(valid))
	req.Header.Set("Content-Encoding", "gzip")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code, "overflow leaked preprocessing or retained capacity")
}
