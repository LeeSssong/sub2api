package service

import (
	"bytes"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// normalizeAPIKeyProtocolUsage changes only client-visible usage. Anthropic
// excludes cache creation from input_tokens; OpenAI already includes it.
func normalizeAPIKeyProtocolUsage(account *Account, payload []byte, anthropic bool) ([]byte, error) {
	if !account.IsAPIKeyCacheCreationAsInputEnabled() {
		return payload, nil
	}
	if anthropic {
		for _, path := range []string{"usage", "message.usage"} {
			usage := gjson.GetBytes(payload, path)
			if !usage.IsObject() {
				continue
			}
			creation := usage.Get("cache_creation_input_tokens").Int()
			if creation > 0 {
				var err error
				payload, err = sjson.SetBytes(payload, path+".input_tokens", usage.Get("input_tokens").Int()+creation)
				if err != nil {
					return nil, err
				}
			}
			// message_start nests usage under message, unlike OpenAI Responses.
			for _, key := range []string{"cache_creation_input_tokens", "cache_creation.ephemeral_5m_input_tokens", "cache_creation.ephemeral_1h_input_tokens"} {
				if !usage.Get(key).Exists() {
					continue
				}
				var err error
				payload, err = sjson.SetBytes(payload, path+"."+key, 0)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return apiKeyDownstreamUsage(payload)
}

// apiKeyCacheOutputWriter is installed only on the Chat/Messages client output
// boundary. Upstream parsers and billing retain their original payloads.
type apiKeyCacheOutputWriter struct {
	gin.ResponseWriter
	account   *Account
	anthropic bool
	pending   []byte
}

func installAPIKeyCacheOutput(c *gin.Context, account *Account, anthropic bool) func() {
	if c == nil || !account.IsAPIKeyCacheCreationAsInputEnabled() {
		return func() {}
	}
	previous := c.Writer
	writer := &apiKeyCacheOutputWriter{ResponseWriter: previous, account: account, anthropic: anthropic}
	c.Writer = writer
	return func() {
		if len(writer.pending) > 0 {
			_, _ = previous.Write(writer.pending)
		}
		c.Writer = previous
	}
}

func (w *apiKeyCacheOutputWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func (w *apiKeyCacheOutputWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *apiKeyCacheOutputWriter) Write(payload []byte) (int, error) {
	w.Header().Del("Content-Length")
	w.pending = append(w.pending, payload...)
	streaming := strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") || bodyHasSSEFraming(w.pending)
	if streaming {
		for {
			end := bytes.IndexByte(w.pending, '\n')
			if end < 0 {
				break
			}
			line := w.pending[:end+1]
			output := line
			if bytes.HasPrefix(line, []byte("data:")) {
				value := bytes.TrimSpace(line[len("data:"):])
				if gjson.ValidBytes(value) {
					normalized, err := normalizeAPIKeyProtocolUsage(w.account, value, w.anthropic)
					if err != nil {
						return 0, err
					}
					start := bytes.Index(line, value)
					output = append(append(append([]byte(nil), line[:start]...), normalized...), line[start+len(value):]...)
				}
			}
			if _, err := w.ResponseWriter.Write(output); err != nil {
				return 0, err
			}
			w.pending = w.pending[end+1:]
		}
	} else if gjson.ValidBytes(bytes.TrimSpace(w.pending)) {
		normalized, err := normalizeAPIKeyProtocolUsage(w.account, w.pending, w.anthropic)
		if err != nil {
			return 0, err
		}
		if _, err := w.ResponseWriter.Write(normalized); err != nil {
			return 0, err
		}
		w.pending = nil
	} else if !strings.Contains(w.Header().Get("Content-Type"), "json") {
		if _, err := w.ResponseWriter.Write(w.pending); err != nil {
			return 0, err
		}
		w.pending = nil
	}
	return len(payload), nil
}

func (w *apiKeyCacheOutputWriter) WriteHeaderNow() {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeaderNow()
}

// apiKeyDownstreamUsage applies the account's cache-creation-as-input policy
// to client-visible usage after recording the original upstream measurement.
// OpenAI total input already includes cache creation: keep totals and cache
// reads intact, and clear every supported cache-write alias and TTL breakdown.
func apiKeyDownstreamUsage(payload []byte) ([]byte, error) {
	for _, path := range []string{"usage", "response.usage", "data.usage", "data.response.usage"} {
		usage := gjson.GetBytes(payload, path)
		if !usage.IsObject() {
			continue
		}
		normalized := []byte(usage.Raw)
		changed := false
		for _, field := range []string{
			"input_tokens_details.cache_write_tokens", "prompt_tokens_details.cache_write_tokens",
			"input_tokens_details.cache_creation_tokens", "prompt_tokens_details.cache_creation_tokens",
			"cache_write_tokens", "cache_creation_input_tokens", "cache_write_input_tokens", "cache_creation_tokens",
			"cache_creation.ephemeral_5m_input_tokens", "cache_creation.ephemeral_1h_input_tokens",
		} {
			if value := usage.Get(field); !value.Exists() || value.Raw == "0" {
				continue
			}
			var err error
			normalized, err = sjson.SetBytes(normalized, field, 0)
			if err != nil {
				return nil, err
			}
			changed = true
		}
		if changed {
			var err error
			payload, err = sjson.SetRawBytes(payload, path, normalized)
			if err != nil {
				return nil, err
			}
		}
	}
	return payload, nil
}

// normalizeAPIKeyCacheInputPayload runs only after capturing upstream usage.
// Keep SSE framing and unknown JSON fields intact, including large integers.
func normalizeAPIKeyCacheInputPayload(account *Account, payload []byte) ([]byte, error) {
	if !account.IsAPIKeyCacheCreationAsInputEnabled() {
		return payload, nil
	}
	if !bodyHasSSEFraming(payload) {
		return apiKeyDownstreamUsage(payload)
	}
	lines := bytes.Split(payload, []byte("\n"))
	for i, line := range lines {
		text := string(line)
		if !strings.HasPrefix(text, "data:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(text, "data:"))
		if value == "[DONE]" || !gjson.Valid(value) {
			continue
		}
		normalized, err := apiKeyDownstreamUsage([]byte(value))
		if err != nil {
			return nil, err
		}
		prefixEnd := bytes.Index(line, []byte(value))
		lines[i] = append(append(append([]byte(nil), line[:prefixEnd]...), normalized...), line[prefixEnd+len(value):]...)
	}
	return bytes.Join(lines, []byte("\n")), nil
}
