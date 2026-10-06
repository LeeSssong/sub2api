package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	OpenAITurnStateHeader       = "X-Codex-Turn-State"
	OpenAITurnStateHarvestModel = "gpt-6-astra"
	OpenAITurnStateLifetime     = 3570 * time.Second
	OpenAITurnStateRenewAfter   = 3000 * time.Second
	OpenAITurnStateFutureSkew   = 30 * time.Second
)

var ErrInvalidOpenAITurnStateTicket = errors.New("invalid codex turn-state ticket")

type OpenAITurnStateTicket struct {
	Raw           string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	DecodedLength int
}

type OpenAITurnStateKey struct {
	AccountID      int64
	Model          string
	CredentialHash string
}

type OpenAITurnStateStore interface {
	Get(ctx context.Context, key OpenAITurnStateKey, now time.Time) (OpenAITurnStateTicket, bool, error)
	Put(ctx context.Context, key OpenAITurnStateKey, ticket OpenAITurnStateTicket) (bool, error)
	DeleteIfMatch(ctx context.Context, key OpenAITurnStateKey, rawTicket string) (bool, error)
	AcquireLease(ctx context.Context, key OpenAITurnStateKey, owner string, ttl time.Duration) (bool, error)
	ReleaseLease(ctx context.Context, key OpenAITurnStateKey, owner string) error
}

func ParseOpenAITurnStateReuseTicket(raw string, now time.Time) (OpenAITurnStateTicket, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) != 292 && len(raw) != 332 {
		return OpenAITurnStateTicket{}, fmt.Errorf("%w: encoded length %d", ErrInvalidOpenAITurnStateTicket, len(raw))
	}
	decoded, err := base64.URLEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(raw)
	}
	if err != nil || (len(decoded) != 217 && len(decoded) != 249) {
		return OpenAITurnStateTicket{}, fmt.Errorf("%w: base64url shape", ErrInvalidOpenAITurnStateTicket)
	}
	if decoded[0] != 0x80 || (len(decoded)-57)%16 != 0 {
		return OpenAITurnStateTicket{}, fmt.Errorf("%w: binary shape", ErrInvalidOpenAITurnStateTicket)
	}
	issued := time.Unix(int64(binary.BigEndian.Uint64(decoded[1:9])), 0)
	if issued.After(now.Add(OpenAITurnStateFutureSkew)) {
		return OpenAITurnStateTicket{}, fmt.Errorf("%w: issued in future", ErrInvalidOpenAITurnStateTicket)
	}
	expires := issued.Add(OpenAITurnStateLifetime)
	if !now.Before(expires) {
		return OpenAITurnStateTicket{}, fmt.Errorf("%w: expired", ErrInvalidOpenAITurnStateTicket)
	}
	return OpenAITurnStateTicket{Raw: raw, IssuedAt: issued, ExpiresAt: expires, DecodedLength: len(decoded)}, nil
}

func (t OpenAITurnStateTicket) Valid(now time.Time) bool {
	return t.Raw != "" && now.Before(t.ExpiresAt)
}
func (t OpenAITurnStateTicket) RenewalDue(now time.Time) bool {
	return t.Valid(now) && !now.Before(t.IssuedAt.Add(OpenAITurnStateRenewAfter))
}

func OpenAITurnStateCredentialHash(accessToken, accountID string) string {
	sum := sha256.Sum256([]byte(accessToken + "\x00" + accountID))
	return hex.EncodeToString(sum[:])
}

type OpenAITurnStateDecisionInput struct {
	Enabled              bool
	InScope              bool
	Endpoint             string
	Model                string
	HasCompactionTrigger bool
	Ticket               *OpenAITurnStateTicket
	Now                  time.Time
}

func ApplyOpenAITurnStateReuseOutbound(headers http.Header, in OpenAITurnStateDecisionInput) bool {
	if headers == nil || !in.Enabled || !in.InScope || in.Ticket == nil || in.HasCompactionTrigger {
		return false
	}
	if strings.TrimSpace(in.Model) != OpenAITurnStateHarvestModel || strings.TrimRight(strings.TrimSpace(in.Endpoint), "/") != "/responses" {
		return false
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	if !in.Ticket.Valid(now) {
		return false
	}
	headers.Set(OpenAITurnStateHeader, in.Ticket.Raw)
	return true
}
