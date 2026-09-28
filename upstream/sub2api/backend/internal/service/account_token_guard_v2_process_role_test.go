package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type tokenGuardV2RoleRepo struct {
	AccountTokenGuardV2Repository
	claimed chan struct{}
}

func (r *tokenGuardV2RoleRepo) ClaimDue(context.Context, string, time.Duration, int) ([]AccountTokenGuardV2Record, error) {
	select {
	case r.claimed <- struct{}{}:
	default:
	}
	return nil, nil
}

type tokenGuardV2RoleAdmin struct{ AdminService }

func TestProvideAccountTokenGuardV2HonorsProcessRole(t *testing.T) {
	for _, role := range []config.ProcessRole{config.ProcessRoleAPI, config.ProcessRoleWorker, config.ProcessRoleAll} {
		t.Run(string(role), func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Server.ProcessRole = role
			repo := &tokenGuardV2RoleRepo{claimed: make(chan struct{}, 1)}
			svc := ProvideAccountTokenGuardV2Service(repo, nil, &tokenGuardV2RoleAdmin{}, &OpenAIGatewayService{}, &OpenAIOAuthReauthService{}, cfg)
			defer svc.Stop()
			if role == config.ProcessRoleAPI {
				// Start consumes startOnce synchronously before launching its goroutine.
				// Checking it avoids a timing-based "no claim yet" assertion for API role.
				notStarted := false
				svc.startOnce.Do(func() { notStarted = true; close(svc.doneCh) })
				require.True(t, notStarted, "API providers must never start the singleton probe loop")
				select {
				case <-repo.claimed:
					t.Fatal("API claimed a background probe")
				default:
				}
			} else {
				select {
				case <-repo.claimed:
				case <-time.After(2 * time.Second):
					t.Fatal("worker/all did not start the background claim loop")
				}
			}
			svc.Stop()
			select {
			case <-svc.doneCh:
			default:
				t.Fatal("singleton shutdown did not finish")
			}
		})
	}
}
