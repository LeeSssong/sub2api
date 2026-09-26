package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type qualityProbeAccountRepo struct {
	AccountRepository
	account *Account
}

func (r *qualityProbeAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

type qualityProbeHTTP struct {
	HTTPUpstream
	model string
}

func (h *qualityProbeHTTP) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	var body struct {
		Model string `json:"model"`
	}
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	h.model = body.Model
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"21\"}\n\ndata: {\"type\":\"response.completed\"}\n\n"))}, nil
}
func (h *qualityProbeHTTP) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return h.Do(req, proxy, id, concurrency)
}

func TestQualityRecoveryProbeKeepsRemovedAliasWithoutChangingAccount(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			mapping := map[string]any{"model-c": "model-c"}
			if existing {
				mapping["model-a"] = "manual-target"
			}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-only", "model_mapping": mapping}}
			upstream := &qualityProbeHTTP{}
			svc := &AccountTestService{accountRepo: &qualityProbeAccountRepo{account: account}, httpUpstream: upstream}
			cfg := &PelicanTestConfig{QuestionKind: "candy", Prompt: "question", ReasoningEffort: "high", Quality: &QualityPolicy{ProbeModelMapping: map[string]string{"model-a": "upstream-a"}}}
			result, err := svc.RunPelicanBackground(context.Background(), 1, "model-a", cfg)
			require.NoError(t, err)
			require.Equal(t, "success", result.Status, result.ErrorMessage)
			expected := "upstream-a"
			if existing {
				expected = "manual-target"
			}
			require.Equal(t, expected, upstream.model)
			require.Equal(t, existing, account.IsModelSupported("model-a"), "probe must never re-enable business routing")
			raw, err := json.Marshal(cfg)
			require.NoError(t, err)
			require.NotContains(t, string(raw), "upstream-a")
		})
	}
}
