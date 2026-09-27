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
	require.NoError(t, ValidateAccountTokenGuardConfig(cfg))
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

func TestTokenGuardEmailInheritsAccountOpsRecipient(t *testing.T) {
	mail := &guardMailCapture{}
	svc := NewAccountTokenGuardService(&accountOpsSettingsStub{raw: `{"recipient":"shared@example.com"}`}, nil, nil, nil, nil)
	svc.email = mail
	cfg := defaultAccountTokenGuardConfig()
	cfg.EmailEnabled = true
	svc.notify(context.Background(), cfg, "repaired", "account repaired", true)
	require.Equal(t, "shared@example.com", mail.to)
}

func TestTokenGuardEmailAuthTransitionIsNotRepeated(t *testing.T) {
	repo := &tokenGuardTestRepo{states: map[int64]AccountTokenGuardState{}, acquired: true}
	account := guardNativeAccount()
	account.Status = StatusActive
	account.Schedulable = true
	accounts := &tokenGuardTestAccounts{items: []Account{account}}
	svc := NewAccountTokenGuardService(&accountOpsSettingsStub{}, repo, accounts, nil, nil)
	mail := &guardMailCapture{}
	svc.email = mail
	cfg := defaultAccountTokenGuardConfig()
	cfg.Enabled = true
	cfg.EmailEnabled = true
	cfg.EmailRecipient = "ops@example.com"
	require.NoError(t, json.Unmarshal([]byte(`{"notify_on_auth":true}`), &cfg))
	encoded, err := json.Marshal(cfg)
	require.NoError(t, err)
	svc.settings.(*accountOpsSettingsStub).raw = string(encoded)
	svc.config.Store(cfg)
	probeState := AccountTokenGuardProbeAuth
	svc.nativeProbe = func(context.Context, *Account, string) AccountTokenGuardProbeResult {
		return AccountTokenGuardProbeResult{State: probeState}
	}
	_, err = svc.runCycle(context.Background(), true, "")
	require.NoError(t, err)
	require.Equal(t, 1, mail.calls)
	_, err = svc.runCycle(context.Background(), true, "")
	require.NoError(t, err)
	require.Equal(t, 1, mail.calls)
	probeState = AccountTokenGuardProbeOK
	_, err = svc.runCycle(context.Background(), true, "")
	require.NoError(t, err)
	require.Equal(t, 1, mail.calls)
	probeState = AccountTokenGuardProbeAuth
	_, err = svc.runCycle(context.Background(), true, "")
	require.NoError(t, err)
	require.Equal(t, 2, mail.calls)
}
