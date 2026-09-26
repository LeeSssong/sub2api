package requestcapture

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMissingContentTypeSSEAndCommentsAreFiltered(t *testing.T) {
	input := ": heartbeat SECRET_COMMENT\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"api_key\":\"SECRET_KEY\",\"response\":{\"output\":\"hello\"}}\n\n"
	for _, size := range []int{1, 7, 4096} {
		f := newBodyFilter("", false)
		var out bytes.Buffer
		for offset := 0; offset < len(input); offset += size {
			out.Write(f.Write([]byte(input[offset:min(offset+size, len(input))])))
		}
		tail, reason := f.End()
		out.Write(tail)
		require.Empty(t, reason)
		require.Contains(t, out.String(), "response.completed")
		require.Contains(t, out.String(), "hello")
		require.NotContains(t, out.String(), "SECRET")
	}
}

func TestPlainTextJSONRequiresCompleteBoundedValidation(t *testing.T) {
	for _, ct := range []string{"text/plain", "text/plain; charset=utf-8"} {
		f := newBodyFilter(ct, false)
		require.Empty(t, f.Write([]byte(`{"error":{"code":"token_revoked"},"api_key":"SECRET","file_data":"MEDIA"}`)))
		out, reason := f.End()
		require.Equal(t, "media_metadata_only", reason)
		require.True(t, json.Valid(out))
		require.Contains(t, string(out), "token_revoked")
		require.NotContains(t, string(out), "SECRET")
		require.NotContains(t, string(out), "MEDIA")
	}
	for _, input := range []string{`<html>SECRET</html>`, `{"api_key":"SECRET"} trailing`, `{"safe":"` + strings.Repeat("x", 20000) + `","api_key":"SECRET"}`} {
		f := newBodyFilter("text/plain", false)
		require.Empty(t, f.Write([]byte(input)))
		out, reason := f.End()
		require.Equal(t, "unsupported_content_type", reason)
		require.NotContains(t, string(out), "SECRET")
		require.Contains(t, string(out), "sha256")
	}
}

func TestSSECommentsAreNotUnsupportedFields(t *testing.T) {
	f := newBodyFilter("text/event-stream", false)
	out := f.Write([]byte(": SECRET_COMMENT\n\ndata: {\"ok\":true}\n\n: trailing SECRET"))
	tail, reason := f.End()
	require.Empty(t, reason)
	require.NotContains(t, string(append(out, tail...)), "SECRET")
}

type terminalReadBody struct {
	payload []byte
	err     error
	closed  int
}

func (b *terminalReadBody) Read(p []byte) (int, error) {
	n := copy(p, b.payload)
	b.payload = b.payload[n:]
	if len(b.payload) == 0 {
		return n, b.err
	}
	return n, nil
}
func (b *terminalReadBody) Close() error { b.closed++; return nil }

func TestBodyReadErrorIsDiagnosticUntilForwardingFinishes(t *testing.T) {
	m, _ := testManager(t)
	task(t, m, "user", 1, false)
	s := m.Begin(Meta{UserID: 1})
	attempt := s.BeginAttempt(42)
	original := &terminalReadBody{payload: []byte("data: {\"type\":\"response.completed\"}\n\n"), err: errors.New("secret URL must never be persisted")}
	body := ObserveBody(original, s.NewStream("upstream_response", attempt, 0, "text/event-stream", nil))
	buf := make([]byte, 4096)
	n, err := body.Read(buf)
	require.Equal(t, original.err, err)
	require.Contains(t, string(buf[:n]), "response.completed")
	s.mu.Lock()
	alreadyFailed := s.resultError
	s.mu.Unlock()
	require.False(t, alreadyFailed, "a read diagnostic must wait for the forwarder to confirm the business outcome")
	require.NoError(t, body.Close())
	require.Equal(t, 1, original.closed)
	s.Finish(200)
	drain(t, m)
}

var _ io.ReadCloser = (*terminalReadBody)(nil)

func TestForwardingOutcomeSeparatesTailDiagnosticsFromBusinessFailure(t *testing.T) {
	cases := []struct {
		name, payload    string
		outcome          ForwardingOutcome
		readError        error
		priorStatus      int
		priorReadError   bool
		clientWriteError bool
		wantRecord       bool
		wantOutcome      ForwardingOutcome
	}{
		{name: "successful_terminal_and_cancel_same_read", payload: `{"type":"response.completed","response":{"output":"PRIVATE_SUCCESS"}}`, outcome: OutcomeSuccess, readError: context.Canceled},
		{name: "successful_terminal_exceeds_capture_budget", payload: `{"type":"response.completed","response":{"output":"` + strings.Repeat("x", 20000) + `"}}`, outcome: OutcomeSuccess, readError: io.ErrUnexpectedEOF},
		{name: "http_rejection_then_success", payload: `{"type":"response.completed"}`, outcome: OutcomeSuccess, readError: context.Canceled, priorStatus: 401, wantRecord: true, wantOutcome: OutcomeSuccess},
		{name: "failed_read_then_success", payload: `{"type":"response.completed"}`, outcome: OutcomeSuccess, priorReadError: true, wantRecord: true, wantOutcome: OutcomeSuccess},
		{name: "terminal_failure_cannot_be_erased", payload: `{"type":"response.failed","response":{"error":{"code":"failed"}}}`, outcome: OutcomeSuccess, wantRecord: true, wantOutcome: OutcomeFailed},
		{name: "clean_eof_without_terminal", payload: `{"type":"response.output_text.delta","delta":"unfinished"}`, outcome: OutcomeIncomplete, wantRecord: true, wantOutcome: OutcomeIncomplete},
		{name: "cancel_without_terminal", payload: `{"type":"response.output_text.delta","delta":"unfinished"}`, outcome: OutcomeIncomplete, readError: context.Canceled, wantRecord: true, wantOutcome: OutcomeIncomplete},
		{name: "explicit_incomplete", payload: `{"type":"response.incomplete"}`, outcome: OutcomeIncomplete, wantRecord: true, wantOutcome: OutcomeIncomplete},
		{name: "client_write_failed_after_terminal", payload: `{"type":"response.completed"}`, outcome: OutcomeSuccess, clientWriteError: true, wantRecord: true, wantOutcome: OutcomeClientDisconnected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, store := testManager(t)
			target := task(t, m, "user", 1, false)
			s := m.Begin(Meta{UserID: 1})
			s.ClientRequest([]byte(`{"input":"PRIVATE_SUCCESS"}`), "application/json", nil)
			if tc.priorStatus != 0 {
				prior := s.BeginAttempt(41)
				s.AttemptResponse(prior, tc.priorStatus, nil, nil)
				st := s.NewStream("upstream_response", prior, 0, "text/plain", nil)
				_, _ = st.Write([]byte(`{"error":{"code":"token_revoked"},"api_key":"SECRET"}`))
				require.NoError(t, st.Close())
				s.RecordForwardingOutcome(prior, OutcomeFailed)
			}
			if tc.priorReadError {
				prior := s.BeginAttempt(41)
				st := s.NewStream("upstream_response", prior, 0, "text/event-stream", nil)
				b := ObserveBody(&terminalReadBody{err: io.ErrUnexpectedEOF}, st)
				_, _ = b.Read(make([]byte, 1))
				require.NoError(t, b.Close())
				s.RecordForwardingOutcome(prior, OutcomeIncomplete)
			}
			attempt := s.BeginAttempt(42)
			s.AttemptResponse(attempt, http.StatusOK, nil, nil)
			payload := []byte("data: " + tc.payload + "\n\n")
			readErr := tc.readError
			if readErr == nil {
				readErr = io.EOF
			}
			original := &terminalReadBody{payload: append([]byte(nil), payload...), err: readErr}
			observed := ObserveBody(original, s.NewStream("upstream_response", attempt, 0, "", nil))
			buf := make([]byte, len(payload)+1)
			n, err := observed.Read(buf)
			require.Equal(t, readErr, err)
			require.Equal(t, payload, buf[:n])
			require.NoError(t, observed.Close())
			require.Equal(t, 1, original.closed)
			if tc.clientWriteError {
				s.MarkError("client_write_failed")
			}
			s.RecordForwardingOutcome(attempt, tc.outcome)
			s.Finish(http.StatusOK)
			drain(t, m)
			rows, err := m.Records(context.Background(), target.ID, "", false, 10, 0)
			require.NoError(t, err)
			updated, err := m.Task(context.Background(), target.ID)
			require.NoError(t, err)
			var diagnosticCount int64
			if tc.readError != nil {
				diagnosticCount++
			}
			if tc.priorReadError {
				diagnosticCount++
			}
			require.Equal(t, diagnosticCount, updated.ReadDiagnostics)
			if !tc.wantRecord {
				require.Empty(t, rows)
				require.Zero(t, updated.Requests)
				require.Zero(t, m.Stats().UsedBytes)
				store.mu.Lock()
				require.Empty(t, store.records)
				store.mu.Unlock()
				return
			}
			require.Len(t, rows, 1)
			require.True(t, rows[0].IsError)
			require.Equal(t, tc.wantOutcome, rows[0].FinalOutcome)
			if tc.priorStatus != 0 {
				require.Equal(t, 401, rows[0].Attempts[0].Status)
				require.Equal(t, OutcomeFailed, rows[0].Attempts[0].Outcome)
				require.Equal(t, OutcomeSuccess, rows[0].Attempts[1].Outcome)
				require.True(t, rows[0].ReadDiagnostics[0].TerminalConfirmed)
				require.Contains(t, partsText(t, m, rows[0]), "token_revoked")
				require.NotContains(t, partsText(t, m, rows[0]), "SECRET")
			}
		})
	}
}

func TestReadDiagnosticsUseSafeCategoriesAndBoundedMetadata(t *testing.T) {
	m, _ := testManager(t)
	target := task(t, m, "user", 1, false)
	s := m.Begin(Meta{UserID: 1})
	cases := []struct {
		err   error
		class string
	}{
		{fmt.Errorf("secret URL: %w", context.Canceled), "cancelled"},
		{fmt.Errorf("secret URL: %w", context.DeadlineExceeded), "timeout"},
		{io.ErrUnexpectedEOF, "unexpected_eof"},
		{gzip.ErrChecksum, "decompression"},
		{flate.CorruptInputError(12), "decompression"},
		{errors.New("https://SECRET:SECRET@example.com/?token=SECRET"), "other"},
	}
	for _, tc := range cases {
		attempt := s.BeginAttempt(42)
		body := ObserveBody(&terminalReadBody{err: tc.err}, s.NewStream("upstream_response", attempt, 0, "text/event-stream", nil))
		_, err := body.Read(make([]byte, 1))
		require.Equal(t, tc.err, err)
		require.NoError(t, body.Close())
	}
	s.Finish(200)
	drain(t, m)
	rows, err := m.Records(context.Background(), target.ID, "", false, 10, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	for i, tc := range cases {
		d := rows[0].ReadDiagnostics[i]
		require.Equal(t, tc.class, d.Class)
		require.Equal(t, "upstream_response", d.Direction)
		require.Equal(t, "read_error", d.CloseReason)
		require.False(t, d.TerminalConfirmed)
	}
	raw, err := json.Marshal(rows[0])
	require.NoError(t, err)
	require.NotContains(t, string(raw), "SECRET")
	require.NotContains(t, string(raw), "secret URL")
}

func TestMissingContentTypeFailureStillRetainedAcrossByteChunks(t *testing.T) {
	m, _ := testManager(t)
	target := task(t, m, "user", 1, false)
	s := m.Begin(Meta{UserID: 1})
	attempt := s.BeginAttempt(42)
	st := s.NewStream("upstream_response", attempt, 0, "", nil)
	for _, b := range []byte(": heartbeat\n\nevent: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"fixture\"}}}\n\n") {
		_, _ = st.Write([]byte{b})
	}
	require.NoError(t, st.Close())
	s.Finish(200)
	drain(t, m)
	rows, err := m.Records(context.Background(), target.ID, "", false, 10, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.False(t, rows[0].Partial)
	require.Equal(t, "fixture", rows[0].ErrorCode)
	require.Equal(t, OutcomeFailed, rows[0].Attempts[0].Outcome)
}

func TestReadDiagnosticLimitDoesNotLoseFailedAttempt(t *testing.T) {
	m, _ := testManager(t)
	target := task(t, m, "user", 1, false)
	s := m.Begin(Meta{UserID: 1})
	for i := 0; i < 17; i++ {
		attempt := s.BeginAttempt(int64(i + 1))
		observed := ObserveBody(&terminalReadBody{err: context.Canceled}, s.NewStream("upstream_response", attempt, 0, "text/event-stream", nil))
		_, _ = observed.Read(make([]byte, 1))
		require.NoError(t, observed.Close())
		if i < 16 {
			s.RecordForwardingOutcome(attempt, OutcomeSuccess)
		}
	}
	s.Finish(200)
	drain(t, m)
	rows, err := m.Records(context.Background(), target.ID, "", false, 10, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1, "diagnostic storage limits cannot erase an unconfirmed failed read")
	require.Len(t, rows[0].ReadDiagnostics, 16)
	require.EqualValues(t, 17, rows[0].ReadDiagnosticCount)
}

func TestHistoricalFailureDoesNotInventFinalFailure(t *testing.T) {
	m, _ := testManager(t)
	target := task(t, m, "user", 1, false)
	s := m.Begin(Meta{UserID: 1})
	prior := s.BeginAttempt(41)
	s.AttemptResponse(prior, 401, nil, nil)
	s.RecordForwardingOutcome(prior, OutcomeFailed)
	current := s.BeginAttempt(42)
	s.AttemptResponse(current, 200, nil, nil)
	st := s.NewStream("upstream_response", current, 0, "application/json", nil)
	_, _ = st.Write([]byte(`{"output":"completed"}`))
	require.NoError(t, st.Close())
	s.Finish(200)
	drain(t, m)
	rows, err := m.Records(context.Background(), target.ID, "", false, 10, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].FinalOutcome, "the final outcome is unconfirmed without a forwarder result")
	require.Equal(t, OutcomeFailed, rows[0].Attempts[0].Outcome)
}
