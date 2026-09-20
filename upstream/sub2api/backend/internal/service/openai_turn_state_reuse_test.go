package service

import (
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeOpenAITurnStateTicket(decodedLen int, issued time.Time) string {
	raw := make([]byte, decodedLen)
	raw[0] = 0x80
	binary.BigEndian.PutUint64(raw[1:9], uint64(issued.Unix()))
	return base64.URLEncoding.EncodeToString(raw)
}

func TestParseOpenAITurnStateReuseTicket(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for _, decodedLen := range []int{217, 249} {
		ticket, err := ParseOpenAITurnStateReuseTicket(makeOpenAITurnStateTicket(decodedLen, now.Add(-time.Minute)), now)
		require.NoError(t, err)
		require.Equal(t, decodedLen, ticket.DecodedLength)
		require.Equal(t, OpenAITurnStateLifetime, ticket.ExpiresAt.Sub(ticket.IssuedAt))
	}
	for _, raw := range []string{
		strings.Repeat("a", 312), strings.Repeat("a", 356),
		makeOpenAITurnStateTicket(217, now.Add(-OpenAITurnStateLifetime)),
		makeOpenAITurnStateTicket(217, now.Add(OpenAITurnStateFutureSkew+time.Second)),
	} {
		_, err := ParseOpenAITurnStateReuseTicket(raw, now)
		require.Error(t, err)
	}
}

func TestApplyOpenAITurnStateReuseOutbound(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ticket, err := ParseOpenAITurnStateReuseTicket(makeOpenAITurnStateTicket(217, now.Add(-time.Minute)), now)
	require.NoError(t, err)
	tests := []struct {
		name string
		in   OpenAITurnStateDecisionInput
		want bool
	}{
		{"astra", OpenAITurnStateDecisionInput{Enabled: true, InScope: true, Endpoint: "/responses", Model: OpenAITurnStateHarvestModel, Ticket: &ticket, Now: now}, true},
		{"sol", OpenAITurnStateDecisionInput{Enabled: true, InScope: true, Endpoint: "/responses", Model: "gpt-5.6-sol", Ticket: &ticket, Now: now}, false},
		{"compact", OpenAITurnStateDecisionInput{Enabled: true, InScope: true, Endpoint: "/responses", Model: OpenAITurnStateHarvestModel, HasCompactionTrigger: true, Ticket: &ticket, Now: now}, false},
		{"compact endpoint", OpenAITurnStateDecisionInput{Enabled: true, InScope: true, Endpoint: "/responses/compact", Model: OpenAITurnStateHarvestModel, Ticket: &ticket, Now: now}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{"X-Codex-Turn-State": []string{"client-state"}}
			require.Equal(t, tt.want, ApplyOpenAITurnStateReuseOutbound(h, tt.in))
			if tt.want {
				require.Equal(t, ticket.Raw, h.Get(OpenAITurnStateHeader))
			} else {
				require.Equal(t, "client-state", h.Get(OpenAITurnStateHeader))
			}
		})
	}
}

func TestOpenAITurnStateCredentialHashIsolation(t *testing.T) {
	a := OpenAITurnStateCredentialHash("token-a", "acct")
	require.NotEqual(t, a, OpenAITurnStateCredentialHash("token-b", "acct"))
	require.NotEqual(t, a, OpenAITurnStateCredentialHash("token-a", "other"))
}
