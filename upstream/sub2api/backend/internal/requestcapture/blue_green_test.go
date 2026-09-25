package requestcapture

import (
	"context"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestBlueGreenDirectoriesDoNotRecoverLivePeer(t *testing.T) {
	store := newMemoryStore()
	dir := t.TempDir()
	cfg := Config{Enabled: true, QuotaMiB: 10, RetentionDays: 7}
	blue, err := New(store, filepath.Join(dir, "blue"), cfg)
	require.NoError(t, err)
	defer blue.Close()
	running := task(t, blue, "user", 1, false)
	green, err := New(store, filepath.Join(dir, "green"), cfg)
	require.NoError(t, err)
	defer green.Close()
	require.NotEqual(t, blue.InstanceID(), green.InstanceID())
	saved, err := store.Task(context.Background(), blue.InstanceID(), running.ID)
	require.NoError(t, err)
	require.Equal(t, "running", saved.Status, "green startup must not mark blue's live task interrupted")
	_, err = store.Task(context.Background(), green.InstanceID(), running.ID)
	require.Error(t, err, "instance authorization remains isolated")
}
