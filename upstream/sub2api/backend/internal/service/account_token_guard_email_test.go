package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type guardMailCapture struct {
	to, subject, body string
	calls             int
	err               error
}

func (m *guardMailCapture) SendEmail(_ context.Context, to, subject, body string) error {
	m.to = to
	m.subject = subject
	m.body = body
	m.calls++
	return m.err
}

func TestTokenGuardEmailValidationAndRoundTrip(t *testing.T) {
	cfg := defaultAccountTokenGuardConfig()
	require.NoError(t, json.Unmarshal([]byte(`{"email_enabled":true,"email_recipient":"bad\r\nBcc:x@example.com"}`), &cfg))
	require.Error(t, ValidateAccountTokenGuardConfig(cfg))
	require.NoError(t, json.Unmarshal([]byte(`{"email_enabled":true,"email_recipient":""}`), &cfg))
	require.Error(t, ValidateAccountTokenGuardConfig(cfg))
	require.NoError(t, json.Unmarshal([]byte(`{"email_enabled":true,"email_recipient":"ops@example.com"}`), &cfg))
	svc := NewAccountTokenGuardService(&accountOpsSettingsStub{}, nil, nil, nil, nil)
	saved, err := svc.SaveConfig(context.Background(), cfg)
	require.NoError(t, err)
	raw, err := json.Marshal(saved)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"email_enabled":true`)
	require.Contains(t, string(raw), `"email_recipient":"ops@example.com"`)
}

func TestTokenGuardEmailIndependentOfBarkAndEscapesHTML(t *testing.T) {
	mail := &guardMailCapture{}
	svc := NewAccountTokenGuardService(nil, nil, nil, nil, nil)
	svc.email = mail
	cfg := defaultAccountTokenGuardConfig()
	require.NoError(t, json.Unmarshal([]byte(`{"email_enabled":true,"email_recipient":"ops@example.com"}`), &cfg))
	svc.notify(context.Background(), cfg, "repair", "<script>account</script>", true)
	require.Equal(t, 1, mail.calls)
	require.Equal(t, "ops@example.com", mail.to)
	require.NotContains(t, mail.body, "<script>")
	require.Contains(t, mail.body, "&lt;script&gt;")
	svc.notify(context.Background(), cfg, "repair", "failure", false)
	require.Equal(t, 1, mail.calls)
	cfg.EmailEnabled = false
	svc.notify(context.Background(), cfg, "repair", "failure", true)
	require.Equal(t, 1, mail.calls)
}

type guardBlockingMail struct{ barkStarted <-chan struct{} }

func (m guardBlockingMail) SendEmail(ctx context.Context, _, _, _ string) error {
	select {
	case <-m.barkStarted:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type guardNotifyTransport func(*http.Request) (*http.Response, error)

func (f guardNotifyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTokenGuardEmailDoesNotDelayBark(t *testing.T) {
	barkStarted := make(chan struct{})
	svc := NewAccountTokenGuardService(nil, nil, nil, nil, nil)
	svc.email = guardBlockingMail{barkStarted: barkStarted}
	svc.httpClient = &http.Client{Transport: guardNotifyTransport(func(r *http.Request) (*http.Response, error) {
		require.NoError(t, r.Context().Err())
		close(barkStarted)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}
	cfg := defaultAccountTokenGuardConfig()
	cfg.EmailEnabled = true
	cfg.EmailRecipient = "ops@example.com"
	cfg.BarkKey = "test-key"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	svc.notify(ctx, cfg, "repair", "account repaired", true)
	require.NoError(t, ctx.Err(), "Bark should unblock slow SMTP immediately")
}
