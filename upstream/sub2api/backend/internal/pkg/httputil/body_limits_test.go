package httputil

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"testing"
)

type repeatingBodyReader struct{}

func (repeatingBodyReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

func TestReadRequestBodyRejectsDecompressionOverflow(t *testing.T) {
	var wire bytes.Buffer
	w := gzip.NewWriter(&wire)
	if _, err := io.CopyN(w, repeatingBodyReader{}, (64<<20)+1); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := newRequestWithBody(t, wire.Bytes(), "gzip")
	body, err := ReadRequestBodyWithPrealloc(req)
	var limit *http.MaxBytesError
	if !errors.As(err, &limit) {
		t.Fatalf("over-limit gzip must fail instead of returning truncated input; bytes=%d err=%v", len(body), err)
	}
	if limit.Limit != 64<<20 {
		t.Fatalf("decoded limit = %d, want 64 MiB", limit.Limit)
	}
	if body != nil {
		t.Fatal("overflow must not return a partial request")
	}
}
