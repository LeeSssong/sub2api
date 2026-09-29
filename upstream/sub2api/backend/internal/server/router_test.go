package server

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestShouldServeEmbeddedFrontend(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{name: "nil config", want: false},
		{name: "default role serves frontend", cfg: &config.Config{}, want: true},
		{name: "all role serves frontend", cfg: &config.Config{Server: config.ServerConfig{ProcessRole: config.ProcessRoleAll}}, want: true},
		{name: "api role serves frontend without singleton jobs", cfg: &config.Config{Server: config.ServerConfig{ProcessRole: config.ProcessRoleAPI}}, want: true},
		{name: "worker role does not serve frontend", cfg: &config.Config{Server: config.ServerConfig{ProcessRole: config.ProcessRoleWorker}}, want: false},
		{name: "gateway runtime does not serve frontend", cfg: &config.Config{Runtime: config.RuntimeConfig{Role: config.RuntimeRoleGateway}}, want: false},
		{name: "api gateway runtime does not serve frontend", cfg: &config.Config{Runtime: config.RuntimeConfig{Role: config.RuntimeRoleGateway}, Server: config.ServerConfig{ProcessRole: config.ProcessRoleAPI}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldServeEmbeddedFrontend(tt.cfg))
		})
	}
}
