package basispoints

import (
	"io"
	"strings"
	"testing"
)

func TestClientProtocolFailureHidesTransportDiagnostics(t *testing.T) {
	source := testSource()
	source["tools"] = []any{object{"type": "function", "name": "shell", "parameters": object{"type": "object"}}}
	_, b := mustPrepare(t, source, "", nil)
	event := object{"type": "response.completed", "response": object{"id": "resp_failed", "status": "completed", "output": []any{nativeCall(object{"name": "shell", "arguments": object{}})}}}
	call := event["response"].(object)["output"].([]any)[0].(object)
	call["arguments"] = object{"code": "const PRIVATE_SOURCE = await doSomething();"}
	stream := b.Stream(io.NopCloser(strings.NewReader("data: " + quoted(event) + "\n\n")))
	defer stream.Close()
	raw, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE_SOURCE", "json_offset", "format=text_or_code", "OfficeJS", "bytes="} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("client received transport diagnostic %q: %s", secret, raw)
		}
	}
	if !strings.Contains(string(raw), `"code":"basispoints_protocol_error"`) {
		t.Fatalf("missing stable code: %s", raw)
	}
}

func TestUpstreamTerminalErrorsDoNotEchoPrivateFields(t *testing.T) {
	for _, kind := range []string{"response.failed", "response.incomplete", "error"} {
		t.Run(kind, func(t *testing.T) {
			_, b := mustPrepare(t, testSource(), "", nil)
			event := object{"type": kind, "message": "PRIVATE_DIAGNOSTIC", "error": object{"message": "PRIVATE_DIAGNOSTIC"}, "response": object{"status": "failed", "id": "resp_x", "usage": object{"input_tokens": 3}, "output": []any{object{"text": "PRIVATE_DIAGNOSTIC"}}, "error": object{"code": "secret_code", "message": "PRIVATE_DIAGNOSTIC"}}}
			stream := b.Stream(io.NopCloser(strings.NewReader("data: " + quoted(event) + "\n\n")))
			defer stream.Close()
			raw, err := io.ReadAll(stream)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "PRIVATE_DIAGNOSTIC") || strings.Contains(string(raw), "secret_code") {
				t.Fatalf("error payload leaked: %s", raw)
			}
			if !strings.Contains(string(raw), `"input_tokens":3`) {
				t.Fatalf("lost terminal usage: %s", raw)
			}
		})
	}
}

func TestForcedToolChoicesRequireNativeCapability(t *testing.T) {
	for _, choice := range []string{`"required"`, `{"type":"function","name":"shell"}`, `{"type":"custom","name":"apply_patch"}`, `{"type":"allowed_tools","mode":"required","tools":[{"type":"function","name":"shell"}]}`} {
		body := []byte(`{"tool_choice":` + choice + `}`)
		if got := NativeFallbackReason(body); got != "tool_choice" {
			t.Errorf("choice %s: got %q", choice, got)
		}
	}
	for _, choice := range []string{`"auto"`, `"none"`, `null`} {
		if got := NativeFallbackReason([]byte(`{"tool_choice":` + choice + `}`)); got != "" {
			t.Errorf("supported choice %s: %q", choice, got)
		}
	}
}
