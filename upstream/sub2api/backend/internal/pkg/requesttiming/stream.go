package requesttiming

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// Stream evidence describes application body reads (after HTTP decoding), not
// TCP packets or remote client receipt. Bounds keep the existing 128 KiB timing
// persistence limit usable even with many retries and long streams.
const (
	maxStreams           = 4
	maxStreamEventTypes  = 24
	maxStreamSamples     = 16
	maxStreamReadWindows = 16
)

type StreamSummary struct {
	Attempt          int                  `json:"attempt"`
	Format           string               `json:"format"`
	ContentType      string               `json:"content_type"`
	ContentEncoding  string               `json:"content_encoding"`
	ReadCalls        int                  `json:"read_calls"`
	ReadBytes        int64                `json:"read_bytes"`
	FirstReadMS      *float64             `json:"first_read_ms,omitempty"`
	LastReadMS       float64              `json:"last_read_ms"`
	ReadWaitMS       float64              `json:"read_wait_ms"`
	MaxReadWaitMS    float64              `json:"max_read_wait_ms"`
	MaxConsumerGapMS float64              `json:"max_consumer_gap_ms"`
	EventCount       int                  `json:"event_count"`
	Truncated        bool                 `json:"truncated"`
	ReadWindows      []StreamReadWindow   `json:"read_windows,omitempty"`
	Events           []StreamEventSummary `json:"events,omitempty"`
	Samples          []StreamEventSample  `json:"samples,omitempty"`
}

type StreamReadWindow struct {
	Bucket  int64   `json:"bucket"` // Five-second buckets relative to request start.
	FirstMS float64 `json:"first_ms"`
	LastMS  float64 `json:"last_ms"`
	Calls   int     `json:"calls"`
	Bytes   int64   `json:"bytes"`
	WaitMS  float64 `json:"wait_ms"`
}

// Type is normalized to a fixed protocol vocabulary before retention. Unknown
// upstream names, payloads, headers, IDs, tool names and errors are never saved.
type StreamEventObservation struct {
	Type           string
	PayloadBytes   int
	ContentBytes   int
	EncryptedBytes int
	Reasoning      bool
	Semantic       bool
	Visible        bool
	TTFT           bool
}

type StreamEventSummary struct {
	Type           string  `json:"type"`
	Count          int     `json:"count"`
	FirstMS        float64 `json:"first_ms"`
	LastMS         float64 `json:"last_ms"`
	PayloadBytes   int64   `json:"payload_bytes"`
	ContentBytes   int64   `json:"content_bytes"`
	EncryptedBytes int64   `json:"encrypted_bytes"`
	ReasoningCount int     `json:"reasoning_count"`
	SemanticCount  int     `json:"semantic_count"`
	VisibleCount   int     `json:"visible_count"`
	TTFTCount      int     `json:"ttft_count"`
}

type StreamEventSample struct {
	Type           string  `json:"type"`
	AtMS           float64 `json:"at_ms"`
	LastReadMS     float64 `json:"last_read_ms"`
	PayloadBytes   int     `json:"payload_bytes"`
	ContentBytes   int     `json:"content_bytes"`
	EncryptedBytes int     `json:"encrypted_bytes"`
	Reasoning      bool    `json:"reasoning"`
	Semantic       bool    `json:"semantic"`
	Visible        bool    `json:"visible"`
	TTFT           bool    `json:"ttft"`
}

type streamKey struct{}
type streamTrace struct {
	c     *Collector
	index int
}
type streamBody struct {
	io.ReadCloser
	trace   *streamTrace
	lastEnd time.Time // Protected by the collector lock.
}

// TrackStream wraps the body once, above any nested HTTP timing wrappers, so
// bytes are not counted twice. Call before starting the scanner/read pump.
func TrackStream(ctx context.Context, resp *http.Response, format string) context.Context {
	c := From(ctx)
	if c == nil || resp == nil || resp.Body == nil {
		return ctx
	}
	if existing, ok := resp.Body.(*streamBody); ok && existing.trace.c == c {
		return context.WithValue(ctx, streamKey{}, existing.trace)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.finished {
		return ctx
	}
	if len(c.data.Streams) >= maxStreams {
		c.data.StreamsDropped++
		return context.WithValue(ctx, streamKey{}, (*streamTrace)(nil))
	}
	attempt := 0
	if tr, ok := ctx.Value(attemptKey{}).(*Trace); ok && tr.c == c {
		attempt = tr.index + 1
	}
	if format != "responses" && format != "chat_completions" {
		format = "unknown"
	}
	contentType := "other"
	if strings.EqualFold(strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]), "text/event-stream") {
		contentType = "event_stream"
	}
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	switch encoding {
	case "", "identity":
		encoding = "identity"
	case "gzip", "br", "deflate", "zstd":
	default:
		encoding = "other"
	}
	tr := &streamTrace{c: c, index: len(c.data.Streams)}
	c.data.Streams = append(c.data.Streams, StreamSummary{Attempt: attempt, Format: format, ContentType: contentType, ContentEncoding: encoding})
	resp.Body = &streamBody{ReadCloser: resp.Body, trace: tr}
	return context.WithValue(ctx, streamKey{}, tr)
}

func (b *streamBody) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := b.ReadCloser.Read(p)
	b.recordRead(start, time.Now(), n)
	return n, err
}

func (b *streamBody) recordRead(start, end time.Time, n int) {
	c := b.trace.c
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.finished {
		return
	}
	d := &c.data.Streams[b.trace.index]
	wait := float64(end.Sub(start)) / float64(time.Millisecond)
	d.ReadCalls++
	d.ReadWaitMS += wait
	d.MaxReadWaitMS = max(d.MaxReadWaitMS, wait)
	if !b.lastEnd.IsZero() {
		d.MaxConsumerGapMS = max(d.MaxConsumerGapMS, float64(start.Sub(b.lastEnd))/float64(time.Millisecond))
	}
	b.lastEnd = end
	if n <= 0 {
		return
	}
	at := c.offset(end)
	d.ReadBytes += int64(n)
	if d.FirstReadMS == nil {
		d.FirstReadMS = &at
	}
	d.LastReadMS = at
	bucket := int64(at / 5000)
	if len(d.ReadWindows) > 0 && d.ReadWindows[len(d.ReadWindows)-1].Bucket == bucket {
		w := &d.ReadWindows[len(d.ReadWindows)-1]
		w.LastMS = at
		w.Calls++
		w.Bytes += int64(n)
		w.WaitMS += wait
		return
	}
	w := StreamReadWindow{Bucket: bucket, FirstMS: at, LastMS: at, Calls: 1, Bytes: int64(n), WaitMS: wait}
	if len(d.ReadWindows) == maxStreamReadWindows {
		copy(d.ReadWindows[maxStreamReadWindows/2:], d.ReadWindows[maxStreamReadWindows/2+1:])
		d.ReadWindows[len(d.ReadWindows)-1] = w
		d.Truncated = true
	} else {
		d.ReadWindows = append(d.ReadWindows, w)
	}
}

// StreamEvent records raw, pre-transformation classification. Comparing it to
// the existing first_visible/first_semantic events exposes transformation or
// classification differences without retaining content.
func StreamEvent(ctx context.Context, observation StreamEventObservation) {
	if ctx == nil {
		return
	}
	tr, _ := ctx.Value(streamKey{}).(*streamTrace)
	if tr == nil {
		return
	}
	c := tr.c
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.finished {
		return
	}
	o := observation
	o.Type = safeStreamEventType(o.Type)
	o.PayloadBytes = max(0, o.PayloadBytes)
	o.ContentBytes = max(0, o.ContentBytes)
	o.EncryptedBytes = max(0, o.EncryptedBytes)
	d := &c.data.Streams[tr.index]
	d.EventCount++
	at := c.offset(time.Now())
	index := -1
	for i := range d.Events {
		if d.Events[i].Type == o.Type {
			index = i
			break
		}
	}
	if index < 0 && len(d.Events) < maxStreamEventTypes {
		index = len(d.Events)
		d.Events = append(d.Events, StreamEventSummary{Type: o.Type, FirstMS: at})
	}
	if index >= 0 {
		e := &d.Events[index]
		e.Count++
		e.LastMS = at
		e.PayloadBytes += int64(o.PayloadBytes)
		e.ContentBytes += int64(o.ContentBytes)
		e.EncryptedBytes += int64(o.EncryptedBytes)
		if o.Reasoning {
			e.ReasoningCount++
		}
		if o.Semantic {
			e.SemanticCount++
		}
		if o.Visible {
			e.VisibleCount++
		}
		if o.TTFT {
			e.TTFTCount++
		}
	} else {
		d.Truncated = true
	}
	sample := StreamEventSample{Type: o.Type, AtMS: at, LastReadMS: d.LastReadMS, PayloadBytes: o.PayloadBytes, ContentBytes: o.ContentBytes, EncryptedBytes: o.EncryptedBytes, Reasoning: o.Reasoning, Semantic: o.Semantic, Visible: o.Visible, TTFT: o.TTFT}
	if len(d.Samples) == maxStreamSamples {
		copy(d.Samples[maxStreamSamples/2:], d.Samples[maxStreamSamples/2+1:])
		d.Samples[len(d.Samples)-1] = sample
		d.Truncated = true
	} else {
		d.Samples = append(d.Samples, sample)
	}
}

func safeStreamEventType(value string) string {
	switch value {
	case "[DONE]", "invalid", "keepalive", "error", "chat.chunk", "chat.usage",
		"response.created", "response.in_progress", "response.completed", "response.done", "response.failed", "response.incomplete",
		"response.output_item.added", "response.output_item.done", "response.content_part.added", "response.content_part.done",
		"response.output_text.delta", "response.output_text.done", "response.refusal.delta", "response.refusal.done",
		"response.reasoning_text.delta", "response.reasoning_text.done", "response.reasoning_summary_text.delta", "response.reasoning_summary_text.done",
		"response.reasoning_summary_part.added", "response.reasoning_summary_part.done",
		"response.function_call_arguments.delta", "response.function_call_arguments.done", "response.custom_tool_call_input.delta", "response.custom_tool_call_input.done",
		"response.audio.delta", "response.audio.done", "response.audio_transcript.delta", "response.audio_transcript.done",
		"response.image_generation_call.in_progress", "response.image_generation_call.generating", "response.image_generation_call.partial_image", "response.image_generation_call.completed",
		"response.web_search_call.in_progress", "response.web_search_call.searching", "response.web_search_call.completed",
		"response.file_search_call.in_progress", "response.file_search_call.searching", "response.file_search_call.completed",
		"response.code_interpreter_call.in_progress", "response.code_interpreter_call.interpreting", "response.code_interpreter_call.completed",
		"response.code_interpreter_call_code.delta", "response.code_interpreter_call_code.done":
		return value
	default:
		return "unknown"
	}
}

func cloneStreams(src []StreamSummary) []StreamSummary {
	if len(src) == 0 {
		return nil
	}
	out := append([]StreamSummary{}, src...)
	for i := range out {
		out[i].ReadWindows = append([]StreamReadWindow{}, src[i].ReadWindows...)
		out[i].Events = append([]StreamEventSummary{}, src[i].Events...)
		out[i].Samples = append([]StreamEventSample{}, src[i].Samples...)
		if src[i].FirstReadMS != nil {
			v := *src[i].FirstReadMS
			out[i].FirstReadMS = &v
		}
	}
	return out
}
