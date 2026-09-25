package service

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRequestCaptureWorkerDoesNotOpenDatabaseOrFiles(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.ProcessRole = config.ProcessRoleWorker
	m, err := ProvideRequestCaptureManager(nil, nil, cfg)
	require.NoError(t, err)
	require.Nil(t, m)
}
func TestRequestCaptureSlotIsolation(t *testing.T) {
	t.Setenv("SUB2API_CONTAINER_SLOT", "blue")
	blue, err := requestCaptureDirectory("/data")
	require.NoError(t, err)
	t.Setenv("SUB2API_CONTAINER_SLOT", "green")
	green, err := requestCaptureDirectory("/data")
	require.NoError(t, err)
	require.NotEqual(t, blue, green)
	t.Setenv("SUB2API_CONTAINER_SLOT", "../../escape")
	_, err = requestCaptureDirectory("/data")
	require.Error(t, err)
}
