package requesttiming

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamReadWaitAndConsumerGapAreSeparate(t *testing.T) {
	start := time.Now()
	c := New(start, 0)
	resp := &http.Response{Body: io.NopCloser(strings.NewReader("abc"))}
	ctx := TrackStream(With(context.Background(), c), resp, "responses")
	b := resp.Body.(*streamBody)
	b.recordRead(start.Add(time.Second), start.Add(11*time.Second), 2)
	b.recordRead(start.Add(15*time.Second), start.Add(16*time.Second), 1)
	StreamEvent(ctx, StreamEventObservation{Type: "response.output_text.delta", PayloadBytes: 40, ContentBytes: 3, Semantic: true, Visible: true, TTFT: true})
	c.Finish(200, false)
	c.WhenFinished(func(s Snapshot) {
		if len(s.Streams) != 1 {
			t.Fatalf("missing stream: %+v", s)
		}
		d := s.Streams[0]
		if d.ReadCalls != 2 || d.ReadBytes != 3 || d.ReadWaitMS != 11000 || d.MaxReadWaitMS != 10000 || d.MaxConsumerGapMS != 4000 {
			t.Fatalf("upstream wait and consumer stall conflated: %+v", d)
		}
		if d.FirstReadMS == nil || *d.FirstReadMS != 11000 || d.LastReadMS != 16000 || len(d.ReadWindows) != 2 {
			t.Fatalf("lost read chronology: %+v", d)
		}
		if d.EventCount != 1 || d.Events[0].ContentBytes != 3 || d.Events[0].TTFTCount != 1 {
			t.Fatalf("lost event: %+v", d)
		}
	})
}

func TestStreamEvidenceIsBoundedAndKeepsTail(t *testing.T) {
	c := New(time.Now(), 0)
	ctx := TrackStream(With(context.Background(), c), &http.Response{Body: io.NopCloser(strings.NewReader(""))}, "responses")
	for range 1000 {
		StreamEvent(ctx, StreamEventObservation{Type: "response.output_text.delta", PayloadBytes: 100, ContentBytes: 10, Visible: true, TTFT: true})
	}
	StreamEvent(ctx, StreamEventObservation{Type: "response.completed", PayloadBytes: 80})
	c.Finish(200, false)
	c.WhenFinished(func(s Snapshot) {
		d := s.Streams[0]
		if d.EventCount != 1001 || d.Events[0].Count != 1000 || d.Events[0].ContentBytes != 10000 {
			t.Fatalf("truncation lost totals: %+v", d)
		}
		if len(d.Samples) > 16 || !d.Truncated || d.Samples[0].Type != "response.output_text.delta" || d.Samples[len(d.Samples)-1].Type != "response.completed" {
			t.Fatalf("unbounded or missing head/tail: %+v", d)
		}
	})
}

func TestStreamUnknownTypesAndHeadersCannotRetainSecrets(t *testing.T) {
	c := New(time.Now(), 0)
	resp := &http.Response{Header: http.Header{"Content-Encoding": {"secret-encoding"}, "Content-Type": {"text/event-stream; secret=value"}, "Authorization": {"Bearer private-key"}}, Body: io.NopCloser(strings.NewReader("private-body"))}
	ctx := TrackStream(With(context.Background(), c), resp, "responses")
	got, readErr := io.ReadAll(resp.Body)
	if readErr != nil || string(got) != "private-body" {
		t.Fatalf("instrumentation changed body: %q, %v", got, readErr)
	}
	StreamEvent(ctx, StreamEventObservation{Type: "private-event-name", PayloadBytes: 50, ContentBytes: 12})
	c.Finish(200, false)
	c.WhenFinished(func(s Snapshot) {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"secret-encoding", "secret=value", "private-key", "private-body", "private-event-name"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("retained secret %q", secret)
			}
		}
		if s.Streams[0].Events[0].Type != "unknown" || s.Streams[0].ContentType != "event_stream" || s.Streams[0].ContentEncoding != "other" {
			t.Fatalf("lost safe protocol enums: %+v", s.Streams[0])
		}
	})
}

func TestStreamSnapshotOwnsItsSlicesAndStopsAtFinish(t *testing.T) {
	c := New(time.Now(), 0)
	ctx := TrackStream(With(context.Background(), c), &http.Response{Body: io.NopCloser(strings.NewReader("x"))}, "responses")
	StreamEvent(ctx, StreamEventObservation{Type: "response.created", PayloadBytes: 10})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				StreamEvent(ctx, StreamEventObservation{Type: "response.output_text.delta", ContentBytes: 1})
			}
		}()
	}
	wg.Wait()
	c.Finish(200, false)
	c.WhenFinished(func(s Snapshot) { s.Streams[0].Events[0].Type = "mutated"; s.Streams[0].Samples[0].Type = "mutated" })
	StreamEvent(ctx, StreamEventObservation{Type: "response.completed"})
	c.WhenFinished(func(s Snapshot) {
		if s.Streams[0].EventCount != 401 || s.Streams[0].Events[0].Type != "response.created" || s.Streams[0].Samples[0].Type != "response.created" {
			t.Fatalf("mutable or late data: %+v", s.Streams[0])
		}
	})
}

func TestStreamDiagnosticsFitPersistenceLimitWithMaximumAttempts(t *testing.T) {
	c := New(time.Now(), 0)
	ctx := With(context.Background(), c)
	for i := range 32 {
		SetDiagnostic(ctx, fmt.Sprintf("diagnostic_%d", i), strings.Repeat("x", 512))
	}
	eventTypes := []string{
		"response.created", "response.in_progress", "response.completed", "response.done", "response.failed", "response.incomplete",
		"response.output_item.added", "response.output_item.done", "response.content_part.added", "response.content_part.done",
		"response.output_text.delta", "response.output_text.done", "response.refusal.delta", "response.refusal.done",
		"response.reasoning_text.delta", "response.reasoning_text.done", "response.reasoning_summary_text.delta", "response.reasoning_summary_text.done",
		"response.reasoning_summary_part.added", "response.reasoning_summary_part.done",
		"response.function_call_arguments.delta", "response.function_call_arguments.done", "response.custom_tool_call_input.delta", "response.custom_tool_call_input.done",
	}
	for i := range 32 {
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://example.invalid", nil)
		req, tr := StartAttempt(req, int64(i+1), 0)
		resp := &http.Response{Request: req, Body: io.NopCloser(strings.NewReader(""))}
		streamCtx := TrackStream(req.Context(), resp, "responses")
		for j := range 100 {
			StreamEvent(streamCtx, StreamEventObservation{Type: eventTypes[j%len(eventTypes)], PayloadBytes: 1000000, ContentBytes: 300000, EncryptedBytes: 500000, Reasoning: true, Semantic: true, Visible: true, TTFT: true})
			if b, ok := resp.Body.(*streamBody); ok {
				start := c.start.Add(time.Duration(j) * 5 * time.Second)
				b.recordRead(start, start.Add(time.Second), 100)
			}
		}
		tr.Response(nil, nil)
	}
	for range 256 {
		Observe(ctx, "build_upstream_request")()
	}
	c.Finish(200, false)
	c.WhenFinished(func(s Snapshot) {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) >= 128*1024 || s.StreamsDropped == 0 {
			t.Fatalf("diagnostics exceed persistence bound or silently drop: %d bytes, %+v", len(raw), s)
		}
		t.Logf("maximum-attempt timing JSON: %d bytes", len(raw))
	})
}

func TestTrackStreamWithoutCollectorIsInert(t *testing.T) {
	ctx := context.Background()
	body := io.NopCloser(strings.NewReader("x"))
	resp := &http.Response{Body: body}
	if TrackStream(ctx, resp, "responses") != ctx || resp.Body != body {
		t.Fatal("changed uninstrumented request")
	}
	StreamEvent(ctx, StreamEventObservation{Type: "response.created"})
	StreamEvent(nil, StreamEventObservation{Type: "response.created"})
}

func TestDownstreamWritesSeparateFlushBlocking(t *testing.T) {
	c := New(time.Now(), 0)
	c.Written(time.Now().Add(-100*time.Millisecond), 3, nil, false)
	c.Written(time.Now().Add(-200*time.Millisecond), 0, nil, true)
	c.Finish(200, false)
	c.WhenFinished(func(s Snapshot) {
		raw, _ := json.Marshal(s)
		var d map[string]any
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		if d["downstream_write_calls"] != float64(1) || d["downstream_flush_calls"] != float64(1) {
			t.Fatalf("missing separate operation counters: %s", raw)
		}
		if d["downstream_max_write_ms"].(float64) < 100 || d["downstream_max_flush_ms"].(float64) < 200 {
			t.Fatalf("blocked operations hidden: %s", raw)
		}
	})
}
